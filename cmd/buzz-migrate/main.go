// Command buzz-migrate copies a Mattermost workspace's public channels and
// their history into a buzz relay, preserving channel names, authors, threads,
// and original send times. It reads Mattermost over REST and writes messages
// straight into the relay database, since the relay rejects backdated events.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/enescakir/emoji"
	_ "github.com/lib/pq"
	nostr "github.com/nbd-wtf/go-nostr"

	"gitlab.com/eastriver/internkim/internal/buzzidentity"
	"gitlab.com/eastriver/internkim/internal/buzzimport"
	"gitlab.com/eastriver/internkim/internal/buzzimport/mattermostrest"
	"gitlab.com/eastriver/internkim/internal/buzzimport/media"
	"gitlab.com/eastriver/internkim/internal/buzzimport/relaypublish"
)

func main() {
	mattermostBaseURL := flag.String("mattermost-url", "", "Mattermost base URL")
	mattermostToken := flag.String("mattermost-token", "", "Mattermost session token")
	teamName := flag.String("team", "", "Mattermost team name")
	relayURL := flag.String("relay-url", "ws://localhost:3000", "buzz relay websocket URL")
	relayHTTPURL := flag.String("relay-http-url", "http://localhost:3000", "buzz relay HTTP base URL for media upload")
	buzzDatabaseURL := flag.String("buzz-database-url", "", "buzz relay postgres URL")
	buzzAdminCommand := flag.String("buzz-admin", "", "path to the buzz-admin binary")
	viewerPubkey := flag.String("viewer-pubkey", "", "pubkey to add to every imported channel")
	keySeed := flag.String("key-seed", "", "seed mixed into per-author key derivation")
	communityHost := flag.String("community-host", "localhost:3000", "buzz community host to import into")
	onlyChannels := flag.String("channels", "", "comma-separated Mattermost channel names to import; empty imports every public channel")
	orphanRootTitle := flag.String("orphan-root-title", "", "when set, replies whose Mattermost root was not imported are threaded under one synthesized root message carrying this title, per original root; otherwise such replies are skipped")
	fileCacheDir := flag.String("file-cache-dir", "", "directory of {fileID}.{jpg|png} images used as a fallback when the Mattermost server no longer serves a file")
	sinceMillis := flag.Int64("since", 0, "when >0, import only Mattermost posts created after this unix-millis timestamp (incremental sync; skips the wipe)")
	flag.Parse()

	channelFilter := map[string]bool{}
	for _, name := range strings.Split(*onlyChannels, ",") {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			channelFilter[trimmed] = true
		}
	}

	if *mattermostBaseURL == "" || *mattermostToken == "" || *teamName == "" || *buzzDatabaseURL == "" || *buzzAdminCommand == "" || *keySeed == "" {
		log.Fatal("mattermost-url, mattermost-token, team, buzz-database-url, buzz-admin and key-seed are required")
	}

	ctx := context.Background()
	client := mattermostrest.Client{BaseURL: strings.TrimRight(*mattermostBaseURL, "/"), Token: *mattermostToken}

	teamID, errorValue := client.Team(ctx, *teamName)
	failOn(errorValue, "resolve team")
	users, errorValue := client.Users(ctx)
	failOn(errorValue, "read users")
	authorEmails, authorsByID := mattermostrest.UsersToChannelAuthorEmails(users)

	authorSecrets := map[string]string{}
	authorPubkeys := map[string]string{}
	for userID, author := range authorsByID {
		secretHex := deriveSecret(*keySeed, author.Email)
		authorSecrets[author.Email] = secretHex
		pubkey, errorValue := nostr.GetPublicKey(secretHex)
		failOn(errorValue, "derive pubkey for "+author.Email)
		authorPubkeys[userID] = pubkey
	}

	incremental := *sinceMillis > 0

	bootstrapSecret := deriveSecret(*keySeed, buzzidentity.BootstrapSubject)
	bootstrapPubkey, errorValue := nostr.GetPublicKey(bootstrapSecret)
	failOn(errorValue, "derive bootstrap pubkey")
	if !incremental {
		registerRelayMember(*buzzAdminCommand, bootstrapPubkey)
		for _, pubkey := range authorPubkeys {
			registerRelayMember(*buzzAdminCommand, pubkey)
		}
	}

	buzzDatabase, errorValue := sql.Open("postgres", *buzzDatabaseURL)
	failOn(errorValue, "open buzz database")
	defer buzzDatabase.Close()
	var communityID string
	failOn(buzzDatabase.QueryRow(`SELECT id FROM communities WHERE host = $1`, *communityHost).Scan(&communityID), "resolve community")
	injector := buzzimport.ChannelInjector{Database: buzzDatabase, CommunityID: communityID}
	uploader := media.Uploader{HTTPBaseURL: strings.TrimRight(*relayHTTPURL, "/")}

	channels, errorValue := client.PublicChannels(ctx, teamID)
	failOn(errorValue, "read channels")

	var publisher *relaypublish.Publisher
	if !incremental {
		publisher, errorValue = relaypublish.Connect(ctx, *relayURL, bootstrapSecret)
		failOn(errorValue, "connect relay")
		defer publisher.Close()
	}

	totalImported := 0
	totalSkipped := 0
	postedEmails := map[string]bool{}
	customEmojiCache := map[string]string{}
	for _, restChannel := range channels {
		channel := restChannel.ToImport()
		if len(channelFilter) > 0 && !channelFilter[channel.Name] {
			continue
		}
		buzzChannelID := deriveChannelID(*keySeed, channel.ID)
		if !incremental {
			errorValue := publisher.CreateChannel(ctx, bootstrapSecret, buzzChannelID, channelDisplayName(channel), channel.Purpose)
			if errorValue != nil && !strings.Contains(errorValue.Error(), "already exists") {
				failOn(errorValue, "create channel "+channel.Name)
			}
			failOn(waitForChannelRow(ctx, buzzDatabase, communityID, buzzChannelID), "wait for channel "+channel.Name)
			memberUserIDs, errorValue := client.ChannelMemberUserIDs(ctx, channel.ID)
			failOn(errorValue, "read members for "+channel.Name)
			syncChannelMembers(ctx, publisher, bootstrapSecret, buzzChannelID, memberUserIDs, authorPubkeys, *viewerPubkey)
		}

		posts, errorValue := client.Posts(ctx, channel.ID, *sinceMillis)
		failOn(errorValue, "read posts for "+channel.Name)
		imported, skipped := importChannelPosts(ctx, importDependencies{
			injector: injector, uploader: uploader, client: client,
			orphanRootTitle: *orphanRootTitle, bootstrapSecret: bootstrapSecret,
			fileCacheDir:     *fileCacheDir,
			customEmojiCache: customEmojiCache,
		}, buzzChannelID, posts, authorEmails, authorSecrets, postedEmails)
		totalImported += imported
		totalSkipped += skipped
		fmt.Printf("%-24s %-40s imported=%d skipped=%d\n", channel.Name, buzzChannelID, imported, skipped)
	}
	injectAuthorProfiles(ctx, injector, uploader, client, authorSecrets, authorsByID, postedEmails)
	fmt.Printf("done: %d channels, %d messages imported, %d skipped\n", len(channels), totalImported, totalSkipped)
}

type importDependencies struct {
	injector         buzzimport.ChannelInjector
	uploader         media.Uploader
	client           mattermostrest.Client
	orphanRootTitle  string
	bootstrapSecret  string
	fileCacheDir     string
	customEmojiCache map[string]string
}

func fetchFileBytes(ctx context.Context, dependencies importDependencies, fileID string) ([]byte, string, error) {
	content, mimeType, errorValue := dependencies.client.FileBytes(ctx, fileID)
	if errorValue == nil {
		return content, mimeType, nil
	}
	if dependencies.fileCacheDir == "" {
		return nil, "", errorValue
	}
	for _, candidate := range []struct{ extension, mime string }{{"jpg", "image/jpeg"}, {"png", "image/png"}} {
		cachePath := filepath.Join(dependencies.fileCacheDir, fileID+"."+candidate.extension)
		if cachedBytes, readError := os.ReadFile(cachePath); readError == nil {
			return cachedBytes, candidate.mime, nil
		}
	}
	return nil, "", errorValue
}

func importChannelPosts(
	ctx context.Context,
	dependencies importDependencies,
	buzzChannelID string,
	posts []buzzimport.MattermostPost,
	authorEmails map[string]string,
	authorSecrets map[string]string,
	postedEmails map[string]bool,
) (int, int) {
	injector := dependencies.injector
	eventIDByPostID := map[string]string{}
	imported := 0
	skipped := 0
	for _, post := range posts {
		authorEmail := authorEmails[post.UserID]
		authorSecret := authorSecrets[authorEmail]
		if authorSecret == "" {
			skipped++
			continue
		}
		mediaTags := uploadPostAttachments(ctx, dependencies, authorSecret, post)
		message := buzzimport.ImportedMessage{
			ChannelID:       buzzChannelID,
			AuthorSecretHex: authorSecret,
			Text:            appendMediaMarkdown(post.Message, mediaTags),
			SentAt:          post.CreatedAt,
			MediaTags:       mediaTags,
		}
		if rootPostID := strings.TrimSpace(post.RootID); rootPostID != "" {
			rootEventID, isKnown := eventIDByPostID[rootPostID]
			if !isKnown && dependencies.orphanRootTitle != "" {
				rootEventID, isKnown = ensureSyntheticRoot(ctx, dependencies, buzzChannelID, rootPostID, post.CreatedAt, eventIDByPostID)
			}
			if isKnown {
				message.RootEventID = rootEventID
			} else {
				skipped++
				continue
			}
		}
		event, errorValue := buzzimport.BuildStreamEvent(message)
		if errorValue != nil {
			skipped++
			continue
		}
		if errorValue := injector.InjectMessage(ctx, buzzChannelID, event); errorValue != nil {
			log.Printf("inject failed for post %s: %v", post.ID, errorValue)
			skipped++
			continue
		}
		eventIDByPostID[post.ID] = event.ID
		postedEmails[authorEmail] = true
		imported++
		if post.HasReactions {
			importPostReactions(ctx, dependencies, buzzChannelID, event.ID, post.ID, authorEmails, authorSecrets)
		}
	}
	return imported, skipped
}

func importPostReactions(ctx context.Context, dependencies importDependencies, buzzChannelID, targetEventID, postID string, authorEmails, authorSecrets map[string]string) {
	reactions, errorValue := dependencies.client.Reactions(ctx, postID)
	if errorValue != nil {
		log.Printf("fetch reactions for %s failed: %v", postID, errorValue)
		return
	}
	for _, reaction := range reactions {
		authorSecret := authorSecrets[authorEmails[reaction.UserID]]
		if authorSecret == "" {
			continue
		}
		importedReaction := buzzimport.ImportedReaction{
			ChannelID:       buzzChannelID,
			AuthorSecretHex: authorSecret,
			TargetEventID:   targetEventID,
			CreatedAt:       time.UnixMilli(reaction.CreateAt).UTC(),
		}
		if unicodeEmoji := emoji.Parse(":" + reaction.EmojiName + ":"); !strings.Contains(unicodeEmoji, ":") {
			importedReaction.Emoji = unicodeEmoji
		} else if emojiURL := dependencies.customEmojiURL(ctx, authorSecret, reaction.EmojiName); emojiURL != "" {
			importedReaction.CustomShortcode = reaction.EmojiName
			importedReaction.CustomEmojiURL = emojiURL
		} else {
			continue
		}
		event, errorValue := buzzimport.BuildReactionEvent(importedReaction)
		if errorValue != nil {
			continue
		}
		if errorValue := dependencies.injector.InjectReaction(ctx, buzzChannelID, event); errorValue != nil {
			log.Printf("inject reaction failed: %v", errorValue)
		}
	}
}

func (dependencies importDependencies) customEmojiURL(ctx context.Context, authorSecret, emojiName string) string {
	if cached, isKnown := dependencies.customEmojiCache[emojiName]; isKnown {
		return cached
	}
	url := ""
	content, mimeType, errorValue := dependencies.client.EmojiImage(ctx, emojiName)
	if errorValue == nil && len(content) > 0 {
		stripped, strippedMime, isImage := media.StripMetadata(content, mimeType)
		if isImage {
			if blob, uploadError := dependencies.uploader.Upload(ctx, authorSecret, stripped, strippedMime); uploadError == nil {
				url = blob.URL
			}
		}
	}
	dependencies.customEmojiCache[emojiName] = url
	return url
}

func injectAuthorProfiles(ctx context.Context, injector buzzimport.ChannelInjector, uploader media.Uploader, client mattermostrest.Client, authorSecrets map[string]string, authorsByID map[string]mattermostrest.MattermostAuthor, postedEmails map[string]bool) {
	seen := map[string]bool{}
	for userID, author := range authorsByID {
		if seen[author.Email] || !postedEmails[author.Email] {
			continue
		}
		seen[author.Email] = true
		pictureURL := uploadUserAvatar(ctx, uploader, client, authorSecrets[author.Email], userID)
		event, errorValue := buzzimport.BuildProfileEvent(authorSecrets[author.Email], author.DisplayName, pictureURL)
		if errorValue != nil {
			log.Printf("build profile for %s failed: %v", author.Email, errorValue)
			continue
		}
		if errorValue := injector.InjectProfile(ctx, event); errorValue != nil {
			log.Printf("inject profile for %s failed: %v", author.Email, errorValue)
		}
	}
}

func uploadPostAttachments(ctx context.Context, dependencies importDependencies, authorSecret string, post buzzimport.MattermostPost) [][]string {
	mediaTags := [][]string{}
	for _, fileID := range post.FileIDs {
		content, mimeType, errorValue := fetchFileBytes(ctx, dependencies, fileID)
		if errorValue != nil {
			log.Printf("fetch file %s failed: %v", fileID, errorValue)
			continue
		}
		if !strings.HasPrefix(mimeType, "image/") {
			continue
		}
		strippedContent, strippedMime, isImage := media.StripMetadata(content, mimeType)
		if !isImage {
			continue
		}
		blob, errorValue := dependencies.uploader.Upload(ctx, authorSecret, strippedContent, strippedMime)
		if errorValue != nil {
			log.Printf("upload file %s failed: %v", fileID, errorValue)
			continue
		}
		mediaTags = append(mediaTags, blob.IMetaTag())
	}
	return mediaTags
}

func ensureSyntheticRoot(ctx context.Context, dependencies importDependencies, buzzChannelID, rootPostID string, replyTime time.Time, eventIDByPostID map[string]string) (string, bool) {
	if existing, isKnown := eventIDByPostID[rootPostID]; isKnown {
		return existing, true
	}
	rootMessage := buzzimport.ImportedMessage{
		ChannelID:       buzzChannelID,
		AuthorSecretHex: dependencies.bootstrapSecret,
		Text:            dependencies.orphanRootTitle,
		SentAt:          replyTime.Add(-time.Second),
	}
	event, errorValue := buzzimport.BuildStreamEvent(rootMessage)
	if errorValue != nil {
		return "", false
	}
	if errorValue := dependencies.injector.InjectMessage(ctx, buzzChannelID, event); errorValue != nil {
		log.Printf("inject synthetic root for %s failed: %v", rootPostID, errorValue)
		return "", false
	}
	eventIDByPostID[rootPostID] = event.ID
	return event.ID, true
}

func appendMediaMarkdown(text string, mediaTags [][]string) string {
	lines := []string{}
	for _, mediaTag := range mediaTags {
		mediaURL := ""
		for _, field := range mediaTag {
			if strings.HasPrefix(field, "url ") {
				mediaURL = strings.TrimSpace(strings.TrimPrefix(field, "url "))
			}
		}
		if mediaURL != "" {
			lines = append(lines, "![image]("+mediaURL+")")
		}
	}
	if len(lines) == 0 {
		return text
	}
	if strings.TrimSpace(text) == "" {
		return strings.Join(lines, "\n")
	}
	return text + "\n" + strings.Join(lines, "\n")
}

func uploadUserAvatar(ctx context.Context, uploader media.Uploader, client mattermostrest.Client, authorSecret string, userID string) string {
	content, mimeType, errorValue := client.UserImage(ctx, userID)
	if errorValue != nil || len(content) == 0 {
		return ""
	}
	strippedContent, strippedMime, isImage := media.StripMetadata(content, mimeType)
	if !isImage {
		return ""
	}
	blob, errorValue := uploader.Upload(ctx, authorSecret, strippedContent, strippedMime)
	if errorValue != nil {
		log.Printf("upload avatar for %s failed: %v", userID, errorValue)
		return ""
	}
	return blob.URL
}

func syncChannelMembers(ctx context.Context, publisher *relaypublish.Publisher, actorSecret, buzzChannelID string, memberUserIDs []string, authorPubkeys map[string]string, viewerPubkey string) {
	added := map[string]bool{}
	addOne := func(pubkey string) {
		pubkey = strings.ToLower(strings.TrimSpace(pubkey))
		if pubkey == "" || added[pubkey] {
			return
		}
		added[pubkey] = true
		if errorValue := publisher.AddMember(ctx, actorSecret, buzzChannelID, pubkey); errorValue != nil {
			log.Printf("add member %s failed: %v", pubkey, errorValue)
		}
		time.Sleep(120 * time.Millisecond)
	}
	for _, userID := range memberUserIDs {
		addOne(authorPubkeys[userID])
	}
	addOne(viewerPubkey)
}

func registerRelayMember(buzzAdminCommand string, pubkey string) {
	command := exec.Command(buzzAdminCommand, "add-member", "--pubkey", pubkey)
	if output, errorValue := command.CombinedOutput(); errorValue != nil && !strings.Contains(string(output), "already") {
		log.Printf("add-member %s: %v (%s)", pubkey, errorValue, strings.TrimSpace(string(output)))
	}
}

func channelDisplayName(channel buzzimport.MattermostChannel) string {
	if displayName := strings.TrimSpace(channel.DisplayName); displayName != "" {
		return displayName
	}
	return channel.Name
}

func deriveSecret(seed, email string) string {
	return buzzidentity.Secret(seed, email)
}

func deriveChannelID(seed, mattermostChannelID string) string {
	return buzzidentity.ChannelID(seed, mattermostChannelID)
}

func waitForChannelRow(ctx context.Context, database *sql.DB, communityID, channelID string) error {
	for attempt := 0; attempt < 240; attempt++ {
		var exists bool
		errorValue := database.QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM channels WHERE community_id = $1 AND id = $2)`,
			communityID, channelID,
		).Scan(&exists)
		if errorValue != nil {
			return errorValue
		}
		if exists {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("channel %s never appeared in the relay database", channelID)
}

func failOn(errorValue error, context string) {
	if errorValue != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", context, errorValue)
		os.Exit(1)
	}
}
