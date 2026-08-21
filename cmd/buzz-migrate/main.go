// Command buzz-migrate copies a Mattermost workspace's public channels and
// their history into a buzz relay, preserving channel names, authors, threads,
// and original send times. It reads Mattermost over REST and writes messages
// straight into the relay database, since the relay rejects backdated events.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/enescakir/emoji"
	_ "github.com/lib/pq"
	nostr "github.com/nbd-wtf/go-nostr"

	"gitlab.com/eastriver/internkim/internal/buzzidentity"
	"gitlab.com/eastriver/internkim/internal/buzzimport"
	"gitlab.com/eastriver/internkim/internal/buzzimport/assetkeep"
	"gitlab.com/eastriver/internkim/internal/buzzimport/mattermostrest"
	"gitlab.com/eastriver/internkim/internal/buzzimport/media"
	"gitlab.com/eastriver/internkim/internal/buzzimport/relaypublish"
)

const channelWorkers = 6

func main() {
	mattermostBaseURL := flag.String("mattermost-url", "", "Mattermost base URL")
	mattermostTokenPath := flag.String("mattermost-token-path", "", "file holding the Mattermost session token")
	teamName := flag.String("team", "", "Mattermost team name")
	relayURL := flag.String("relay-url", "ws://localhost:3000", "buzz relay websocket URL")
	relayHTTPURL := flag.String("relay-http-url", "http://localhost:3000", "buzz relay HTTP base URL for media upload")
	buzzDatabaseURL := flag.String("buzz-database-url", "", "buzz relay postgres URL")
	buzzAdminCommand := flag.String("buzz-admin", "", "path to the buzz-admin binary")
	viewerPubkey := flag.String("viewer-pubkey", "", "pubkey to add to every imported channel")
	keySeedPath := flag.String("key-seed-path", "", "file holding the seed mixed into per-author key derivation")
	communityHost := flag.String("community-host", "localhost:3000", "buzz community host to import into")
	onlyChannels := flag.String("channels", "", "comma-separated Mattermost channel names to import; empty imports every public channel")
	agentEmail := flag.String("agent-email", "", "messenger address of the agent, whose history is signed with the agent identity")
	orphanRootTitle := flag.String("orphan-root-title", "", "when set, replies whose Mattermost root was not imported are threaded under one synthesized root message carrying this title, per original root; otherwise such replies are skipped")
	fileCacheDir := flag.String("file-cache-dir", "", "directory of {fileID}.{jpg|png} images used as a fallback when the Mattermost server no longer serves a file")
	sinceMillis := flag.Int64("since", 0, "when >0, import only Mattermost posts created after this unix-millis timestamp (incremental sync; skips the wipe)")
	appURL := flag.String("app-url", "", "central plane app URL, which issues the session this import keeps refused files under")
	agentKeyPath := flag.String("agent-key-path", "", "file holding the company's agent key")
	supabaseURL := flag.String("supabase-url", "", "the company's Supabase project URL")
	supabasePublishableKey := flag.String("supabase-publishable-key", "", "the company's Supabase publishable key")
	flag.Parse()

	channelFilter := map[string]bool{}
	for _, name := range strings.Split(*onlyChannels, ",") {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			channelFilter[trimmed] = true
		}
	}

	// The token and the seed are read from files rather than taken as flags:
	// an argument is visible to anyone who can run ps, and the seed alone is
	// enough to sign as any person this import creates.
	mattermostToken := secretFromFile(*mattermostTokenPath, "mattermost-token-path")
	keySeed := secretFromFile(*keySeedPath, "key-seed-path")

	if *mattermostBaseURL == "" || *teamName == "" || *buzzDatabaseURL == "" || *buzzAdminCommand == "" {
		log.Fatal("mattermost-url, team, buzz-database-url and buzz-admin are required")
	}

	keeper := keeperFrom(*appURL, *agentKeyPath, *supabaseURL, *supabasePublishableKey)
	if keeper == nil {
		log.Print("no asset store was given, so a file the messenger refuses will be reported and left out")
	}

	ctx := context.Background()
	client := mattermostrest.Client{BaseURL: strings.TrimRight(*mattermostBaseURL, "/"), Token: mattermostToken}

	teamID, errorValue := client.Team(ctx, *teamName)
	failOn(errorValue, "resolve team")
	users, errorValue := client.Users(ctx)
	failOn(errorValue, "read users")
	authorEmails, authorsByID := mattermostrest.UsersToChannelAuthorEmails(users)

	// The agent is one identity across the import and everything after it, so its
	// history is signed with the key chatd goes on to answer with rather than one
	// derived from whatever address the old messenger gave its bot account.
	agentEmailAddress := strings.ToLower(strings.TrimSpace(*agentEmail))
	authorSecrets := map[string]string{}
	authorPubkeys := map[string]string{}
	for userID, author := range authorsByID {
		secretHex := deriveSecret(keySeed, author.Email)
		if agentEmailAddress != "" && strings.EqualFold(author.Email, agentEmailAddress) {
			secretHex = deriveSecret(keySeed, buzzidentity.AgentSubject)
		}
		authorSecrets[author.Email] = secretHex
		pubkey, errorValue := nostr.GetPublicKey(secretHex)
		failOn(errorValue, "derive pubkey for "+author.Email)
		authorPubkeys[userID] = pubkey
	}

	incremental := *sinceMillis > 0

	bootstrapSecret := deriveSecret(keySeed, buzzidentity.BootstrapSubject)
	bootstrapPubkey, errorValue := nostr.GetPublicKey(bootstrapSecret)
	failOn(errorValue, "derive bootstrap pubkey")
	if !incremental {
		registerRelayMembers(*buzzAdminCommand, append([]string{bootstrapPubkey}, pubkeysOf(authorPubkeys)...))
	}

	buzzDatabase, errorValue := sql.Open("postgres", *buzzDatabaseURL)
	failOn(errorValue, "open buzz database")
	defer buzzDatabase.Close()
	communityID := resolveCommunityID(buzzDatabase, *communityHost)
	injector := buzzimport.ChannelInjector{Database: buzzDatabase, CommunityID: communityID}
	uploader := media.Uploader{HTTPBaseURL: strings.TrimRight(*relayHTTPURL, "/")}

	channels, errorValue := everyChannel(ctx, client, teamID, authorsByID)
	failOn(errorValue, "read channels")

	var publishers *publisherPool
	if !incremental {
		publishers = newPublisherPool(*relayURL)
		defer publishers.closeAll()
		_, errorValue = publishers.as(ctx, bootstrapSecret)
		failOn(errorValue, "connect relay")
	}

	totalImported := 0
	totalSkipped := skipTally{}
	postedEmails := newSharedNames()
	customEmojiCache := newSharedStrings()
	// Channels do not depend on each other; a reply resolves its root inside the
	// channel it was written in. Most of a channel's time is spent waiting on
	// Mattermost, so several move at once.
	var tally sync.Mutex
	var running sync.WaitGroup
	slots := make(chan struct{}, channelWorkers)
	for _, channel := range channels {
		if len(channelFilter) > 0 && !channelFilter[channel.Name] {
			continue
		}
		running.Add(1)
		slots <- struct{}{}
		go func(channel buzzimport.MattermostChannel) {
			defer running.Done()
			defer func() { <-slots }()
			buzzChannelID := deriveChannelID(keySeed, channel.ID)
			if !incremental {
				memberUserIDs, errorValue := client.ChannelMemberUserIDs(ctx, channel.ID)
				failOn(errorValue, "read members for "+channel.Name)
				creatorSecret := creatorSecretFor(channel, memberUserIDs, authorsByID, authorSecrets, bootstrapSecret)
				creator, errorValue := publishers.as(ctx, creatorSecret)
				failOn(errorValue, "connect as the creator of "+channel.Name)
				// An import replays posts by people who have since left the room,
				// and syncChannelMembers can only add who is in it now, so a
				// private channel would reject their history.
				errorValue = creator.CreateChannel(ctx, creatorSecret, buzzChannelID, channelDisplayName(channel, memberUserIDs, authorsByID), channel.Purpose, relayChannelTypeOf(channel), "open")
				if errorValue != nil && !strings.Contains(errorValue.Error(), "already exists") {
					failOn(errorValue, "create channel "+channel.Name)
				}
				failOn(waitForChannelRow(ctx, buzzDatabase, communityID, buzzChannelID), "wait for channel "+channel.Name)
				syncChannelMembers(ctx, creator, creatorSecret, buzzChannelID, memberUserIDs, authorPubkeys, readerPubkeyFor(channel, *viewerPubkey))
			}

			posts, errorValue := client.Posts(ctx, channel.ID, *sinceMillis)
			failOn(errorValue, "read posts for "+channel.Name)
			imported, skipped := importChannelPosts(ctx, importDependencies{
				injector: injector, uploader: uploader, client: client,
				orphanRootTitle: *orphanRootTitle, bootstrapSecret: bootstrapSecret,
				fileCacheDir:     *fileCacheDir,
				customEmojiCache: customEmojiCache,
				keeper:           keeper,
			}, buzzChannelID, posts, authorEmails, authorSecrets, postedEmails)
			tally.Lock()
			totalImported += imported
			totalSkipped.add(skipped)
			tally.Unlock()
			fmt.Printf("%-24s %-40s imported=%d %s\n", channel.Name, buzzChannelID, imported, skipped)
		}(channel)
	}
	running.Wait()
	injectAuthorProfiles(ctx, injector, uploader, client, authorSecrets, authorsByID, postedEmails)
	fmt.Printf("done: %d channels, %d messages imported, %s\n", len(channels), totalImported, totalSkipped)
}

// Two facts outlive the channel that discovers them: which emoji has already
// been uploaded, and who has already posted. Workers share both, and a Go map
// written from two of them at once takes the process down.
type sharedStrings struct {
	guard  sync.Mutex
	byName map[string]string
}

func newSharedStrings() *sharedStrings {
	return &sharedStrings{byName: map[string]string{}}
}

func (shared *sharedStrings) get(name string) (string, bool) {
	shared.guard.Lock()
	defer shared.guard.Unlock()
	value, isKnown := shared.byName[name]
	return value, isKnown
}

func (shared *sharedStrings) put(name string, value string) {
	shared.guard.Lock()
	defer shared.guard.Unlock()
	shared.byName[name] = value
}

type sharedNames struct {
	guard sync.Mutex
	named map[string]bool
}

func newSharedNames() *sharedNames {
	return &sharedNames{named: map[string]bool{}}
}

func (shared *sharedNames) add(name string) {
	shared.guard.Lock()
	defer shared.guard.Unlock()
	shared.named[name] = true
}

func (shared *sharedNames) has(name string) bool {
	shared.guard.Lock()
	defer shared.guard.Unlock()
	return shared.named[name]
}

type importDependencies struct {
	injector         buzzimport.ChannelInjector
	uploader         media.Uploader
	client           mattermostrest.Client
	orphanRootTitle  string
	bootstrapSecret  string
	fileCacheDir     string
	customEmojiCache *sharedStrings
	keeper           *assetkeep.Keeper
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

// A post the import drops is a post the company cannot read afterwards, so the
// count says which wall it hit. "511 skipped" against an empty channel reads the
// same whether nobody could be signed for or every root was missing.
type skipTally struct {
	noAuthor    int
	noRoot      int
	unbuildable int
	refused     int
}

func (tally *skipTally) add(other skipTally) {
	tally.noAuthor += other.noAuthor
	tally.noRoot += other.noRoot
	tally.unbuildable += other.unbuildable
	tally.refused += other.refused
}

func (tally skipTally) String() string {
	return fmt.Sprintf("skipped=%d (noAuthor=%d noRoot=%d unbuildable=%d refused=%d)",
		tally.noAuthor+tally.noRoot+tally.unbuildable+tally.refused,
		tally.noAuthor, tally.noRoot, tally.unbuildable, tally.refused)
}

func importChannelPosts(
	ctx context.Context,
	dependencies importDependencies,
	buzzChannelID string,
	posts []buzzimport.MattermostPost,
	authorEmails map[string]string,
	authorSecrets map[string]string,
	postedEmails *sharedNames,
) (int, skipTally) {
	injector := dependencies.injector
	eventIDByPostID := map[string]string{}
	imported := 0
	skipped := skipTally{}
	for _, post := range posts {
		authorEmail := authorEmails[post.UserID]
		authorSecret := authorSecrets[authorEmail]
		if authorSecret == "" {
			skipped.noAuthor++
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
				skipped.noRoot++
				continue
			}
		}
		event, errorValue := buzzimport.BuildStreamEvent(message)
		if errorValue != nil {
			skipped.unbuildable++
			continue
		}
		if errorValue := injector.InjectMessage(ctx, buzzChannelID, event); errorValue != nil {
			log.Printf("inject failed for post %s: %v", post.ID, errorValue)
			skipped.refused++
			continue
		}
		eventIDByPostID[post.ID] = event.ID
		postedEmails.add(authorEmail)
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
	if cached, isKnown := dependencies.customEmojiCache.get(emojiName); isKnown {
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
	dependencies.customEmojiCache.put(emojiName, url)
	return url
}

func injectAuthorProfiles(ctx context.Context, injector buzzimport.ChannelInjector, uploader media.Uploader, client mattermostrest.Client, authorSecrets map[string]string, authorsByID map[string]mattermostrest.MattermostAuthor, postedEmails *sharedNames) {
	seen := map[string]bool{}
	for userID, author := range authorsByID {
		if seen[author.Email] || !postedEmails.has(author.Email) {
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
		uploadContent, uploadMime := content, mimeType
		if strings.HasPrefix(mimeType, "image/") {
			if stripped, strippedMime, isImage := media.StripMetadata(content, mimeType); isImage {
				uploadContent, uploadMime = stripped, strippedMime
			}
		}
		blob, errorValue := dependencies.uploader.Upload(ctx, authorSecret, uploadContent, uploadMime)
		if errorValue != nil {
			blob, errorValue = keptWhereTheStoreWouldNot(ctx, dependencies.keeper, uploadContent, uploadMime, errorValue)
			if errorValue != nil {
				log.Printf("upload file %s failed: %v", fileID, errorValue)
				continue
			}
		}
		mediaTags = append(mediaTags, blob.Named(fileNameOf(ctx, dependencies, fileID)).IMetaTag())
	}
	return mediaTags
}

// Keeping a refused file is optional: an import against a buzz relay with no
// central plane behind it still runs, and says which files it left out. All
// four settings or none — three of them address nothing on their own.
func keeperFrom(appURL, agentKeyPath, supabaseURL, publishableKey string) *assetkeep.Keeper {
	if appURL == "" && agentKeyPath == "" && supabaseURL == "" && publishableKey == "" {
		return nil
	}
	if appURL == "" || agentKeyPath == "" || supabaseURL == "" || publishableKey == "" {
		log.Fatal("app-url, agent-key-path, supabase-url and supabase-publishable-key are given together or not at all")
	}
	return &assetkeep.Keeper{
		AppURL:         appURL,
		AgentKey:       secretFromFile(agentKeyPath, "agent-key-path"),
		ProjectURL:     supabaseURL,
		PublishableKey: publishableKey,
	}
}

// A file the messenger's store will not carry still belongs to the conversation
// it was sent to. It goes to the company's own bucket instead, and the message
// names it there, so the history is whole even where the store is particular.
// A store that is only busy is not refusing: the same file goes later, and
// keeping a copy of it here would be a second home for bytes that have one.
func keptWhereTheStoreWouldNot(
	ctx context.Context,
	keeper *assetkeep.Keeper,
	content []byte,
	contentType string,
	refusal error,
) (media.Blob, error) {
	var refused media.Refusal
	if !errors.As(refusal, &refused) || !refused.WillRefuseAgain() {
		return media.Blob{}, refusal
	}
	if keeper == nil {
		return media.Blob{}, fmt.Errorf("%w, and no asset store was given to keep it in", refusal)
	}
	kept, errorValue := keeper.Keep(ctx, content, contentType)
	if errorValue != nil {
		return media.Blob{}, fmt.Errorf("%w, and keeping it failed too: %w", refusal, errorValue)
	}
	return media.Blob{
		URL:      kept.Address,
		SHA256:   kept.Digest,
		Size:     kept.SizeBytes,
		MimeType: contentType,
	}, nil
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

type describedMedia struct {
	url      string
	mimeType string
	filename string
}

// An imeta entry is a "name value" string rather than a position, so the tag is
// read by name.
func describeMediaTag(mediaTag []string) describedMedia {
	described := describedMedia{}
	for _, field := range mediaTag {
		name, value, isPair := strings.Cut(field, " ")
		if !isPair {
			continue
		}
		switch name {
		case "url":
			described.url = strings.TrimSpace(value)
		case "m":
			described.mimeType = strings.TrimSpace(value)
		case "filename":
			described.filename = strings.TrimSpace(value)
		}
	}
	return described
}

func labelOr(filename string, fallback string) string {
	if filename == "" {
		return fallback
	}
	return filename
}

func fileNameOf(ctx context.Context, dependencies importDependencies, fileID string) string {
	info, errorValue := dependencies.client.FileInfo(ctx, fileID)
	if errorValue != nil {
		log.Printf("file info for %s failed: %v", fileID, errorValue)
		return ""
	}
	return info.Name
}

func appendMediaMarkdown(text string, mediaTags [][]string) string {
	lines := []string{}
	for _, mediaTag := range mediaTags {
		described := describeMediaTag(mediaTag)
		if described.url == "" {
			continue
		}
		if strings.HasPrefix(described.mimeType, "image/") {
			lines = append(lines, "!["+labelOr(described.filename, "image")+"]("+described.url+")")
			continue
		}
		lines = append(lines, "["+labelOr(described.filename, "file")+"]("+described.url+")")
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
	}
	for _, userID := range memberUserIDs {
		addOne(authorPubkeys[userID])
	}
	addOne(viewerPubkey)
}

// Every add-member is a process, and on a workspace of any size the run spends
// longer starting them than importing. The roster is read once and only the
// people missing from it are added.
func registerRelayMembers(buzzAdminCommand string, pubkeys []string) {
	known := relayMembers(buzzAdminCommand)
	for _, pubkey := range pubkeys {
		if known[strings.ToLower(pubkey)] {
			continue
		}
		command := exec.Command(buzzAdminCommand, "add-member", "--pubkey", pubkey)
		if output, errorValue := command.CombinedOutput(); errorValue != nil && !strings.Contains(string(output), "already") {
			log.Printf("add-member %s: %v (%s)", pubkey, errorValue, strings.TrimSpace(string(output)))
		}
	}
}

func relayMembers(buzzAdminCommand string) map[string]bool {
	known := map[string]bool{}
	output, errorValue := exec.Command(buzzAdminCommand, "list-members").CombinedOutput()
	if errorValue != nil {
		log.Printf("list-members: %v", errorValue)
		return known
	}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if pubkey := strings.ToLower(fields[0]); isPubkeyHex(pubkey) {
			known[pubkey] = true
		}
	}
	return known
}

func isPubkeyHex(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		isHex := (character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')
		if !isHex {
			return false
		}
	}
	return true
}

func pubkeysOf(byUserID map[string]string) []string {
	pubkeys := make([]string, 0, len(byUserID))
	for _, pubkey := range byUserID {
		pubkeys = append(pubkeys, pubkey)
	}
	sort.Strings(pubkeys)
	return pubkeys
}

// Whoever creates a channel becomes its owner and a member of it. In a stream
// that is the importer and harmless; in a direct conversation it is a third
// party in a room meant for two, and the client picking "whoever is not me"
// picks it instead of the person being talked to.
// The relay refuses an event whose author is not the identity the connection
// authenticated as, so creating a channel on someone's behalf means connecting
// as them. A workspace has as many creators as it has people, and each keeps
// one connection for the whole run.
type publisherPool struct {
	relayURL string
	guard    sync.Mutex
	open     map[string]*relaypublish.Publisher
}

func newPublisherPool(relayURL string) *publisherPool {
	return &publisherPool{relayURL: relayURL, open: map[string]*relaypublish.Publisher{}}
}

func (pool *publisherPool) as(ctx context.Context, actorSecretHex string) (*relaypublish.Publisher, error) {
	pool.guard.Lock()
	defer pool.guard.Unlock()
	if known, isOpen := pool.open[actorSecretHex]; isOpen {
		return known, nil
	}
	publisher, errorValue := relaypublish.Connect(ctx, pool.relayURL, actorSecretHex)
	if errorValue != nil {
		return nil, errorValue
	}
	pool.open[actorSecretHex] = publisher
	return publisher, nil
}

func (pool *publisherPool) closeAll() {
	for _, publisher := range pool.open {
		publisher.Close()
	}
}

func creatorSecretFor(
	channel buzzimport.MattermostChannel,
	memberUserIDs []string,
	authorsByID map[string]mattermostrest.MattermostAuthor,
	authorSecrets map[string]string,
	bootstrapSecret string,
) string {
	if !buzzimport.IsConversationChannelType(channel.Type) {
		return bootstrapSecret
	}
	for _, userID := range memberUserIDs {
		author, isKnown := authorsByID[userID]
		if !isKnown {
			continue
		}
		if secret := authorSecrets[author.Email]; secret != "" {
			return secret
		}
	}
	return bootstrapSecret
}

// A relay tells a direct conversation from a channel by its type, and a client
// that reads "stream" shows a room named after whoever is in it instead of the
// person on the other side.
func relayChannelTypeOf(channel buzzimport.MattermostChannel) string {
	if buzzimport.IsConversationChannelType(channel.Type) {
		return "dm"
	}
	return "stream"
}

// Mattermost gives a direct conversation no display name and names it after the
// user ids it joins, so the people in the room are the only name it has.
func channelDisplayName(
	channel buzzimport.MattermostChannel,
	memberUserIDs []string,
	authorsByID map[string]mattermostrest.MattermostAuthor,
) string {
	if displayName := strings.TrimSpace(channel.DisplayName); displayName != "" {
		return displayName
	}
	if names := participantNames(memberUserIDs, authorsByID); len(names) > 0 {
		return strings.Join(names, ", ")
	}
	return channel.Name
}

func participantNames(memberUserIDs []string, authorsByID map[string]mattermostrest.MattermostAuthor) []string {
	names := []string{}
	for _, userID := range memberUserIDs {
		author, isKnown := authorsByID[userID]
		if !isKnown {
			continue
		}
		if name := strings.TrimSpace(author.DisplayName); name != "" {
			names = append(names, name)
			continue
		}
		names = append(names, author.Username)
	}
	sort.Strings(names)
	return names
}

func everyChannel(
	ctx context.Context,
	client mattermostrest.Client,
	teamID string,
	authorsByID map[string]mattermostrest.MattermostAuthor,
) ([]buzzimport.MattermostChannel, error) {
	userIDs := make([]string, 0, len(authorsByID))
	for userID := range authorsByID {
		userIDs = append(userIDs, userID)
	}
	sort.Strings(userIDs)

	public, errorValue := client.PublicChannels(ctx, teamID)
	if errorValue != nil {
		return nil, fmt.Errorf("public channels: %w", errorValue)
	}
	private, errorValue := client.PrivateChannels(ctx, teamID)
	if errorValue != nil {
		return nil, fmt.Errorf("private channels: %w", errorValue)
	}
	direct, errorValue := client.DirectChannels(ctx, teamID, userIDs)
	if errorValue != nil {
		return nil, fmt.Errorf("direct channels: %w", errorValue)
	}

	channels := mattermostrest.ChannelsToImport(public)
	channels = append(channels, mattermostrest.ChannelsToImport(private)...)
	channels = append(channels, mattermostrest.ChannelsToImport(direct)...)
	return channels, nil
}

// Adding the standing reader to a direct conversation would hand someone else's
// private messages to a key that was never in the room.
func readerPubkeyFor(channel buzzimport.MattermostChannel, viewerPubkey string) string {
	if channel.Type == buzzimport.OpenChannelType {
		return viewerPubkey
	}
	return ""
}

func deriveSecret(seed, email string) string {
	return buzzidentity.Secret(seed, email)
}

// resolveCommunityID finds the community by host, falling back to the sole
// community when the host does not match. After a wipe the relay recreates one
// community whose host can differ from the derived public host, and a strict
// host match there would abort the whole import with an empty database.
func resolveCommunityID(database *sql.DB, communityHost string) string {
	var communityID string
	if errorValue := database.QueryRow(`SELECT id FROM communities WHERE host = $1`, communityHost).Scan(&communityID); errorValue == nil {
		return communityID
	}
	var count int
	failOn(database.QueryRow(`SELECT count(*) FROM communities`).Scan(&count), "count communities")
	if count == 1 {
		failOn(database.QueryRow(`SELECT id FROM communities LIMIT 1`).Scan(&communityID), "resolve sole community")
		return communityID
	}
	failOn(fmt.Errorf("community host %q not found among %d communities", communityHost, count), "resolve community")
	return ""
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

func secretFromFile(path string, flagName string) string {
	if strings.TrimSpace(path) == "" {
		log.Fatalf("%s is required", flagName)
	}
	content, errorValue := os.ReadFile(path)
	if errorValue != nil {
		log.Fatalf("read %s: %v", flagName, errorValue)
	}
	secret := strings.TrimSpace(string(content))
	if secret == "" {
		log.Fatalf("%s holds nothing", flagName)
	}
	return secret
}
