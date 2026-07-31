package admind

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	nostr "github.com/nbd-wtf/go-nostr"
	"gitlab.com/eastriver/internkim/internal/buzzidentity"
	"gitlab.com/eastriver/internkim/internal/buzzimport"
	"gitlab.com/eastriver/internkim/internal/buzzimport/mattermostrest"

	_ "github.com/lib/pq"
)

const buzzOrphanRootMarker = "이전 대화"

type buzzRepairChannelResult struct {
	Name           string   `json:"name"`
	Relinked       int      `json:"relinked"`
	RewrittenRoots int      `json:"rewrittenRoots"`
	DeletedRoots   int      `json:"deletedRoots"`
	KeptGenuine    int      `json:"keptGenuine"`
	KeptSample     []string `json:"keptSample,omitempty"`
}

type buzzRepairResponse struct {
	Apply          bool                      `json:"apply"`
	Relinked       int                       `json:"relinked"`
	RewrittenRoots int                       `json:"rewrittenRoots"`
	DeletedRoots   int                       `json:"deletedRoots"`
	KeptGenuine    int                       `json:"keptGenuine"`
	Channels       []buzzRepairChannelResult `json:"channels"`
}

// handleBuzzRepairOrphans re-links replies that an incremental import stranded
// under a synthetic "이전 대화" root back onto their real imported root, then
// deletes the now-empty synthetic root. It never wipes and never touches
// membership, so it cannot cause an access outage. It runs in-process so it
// ships through the admind OTA component, independent of any LAN binary push.
func (service *Service) handleBuzzRepairOrphans(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.NotFound(responseWriter, request)
		return
	}
	if !service.authorizeInternalOrWebStaffRequest(request) {
		http.Error(responseWriter, "staff access required", http.StatusForbidden)
		return
	}
	apply := request.URL.Query().Get("apply") == "true"
	report, errorValue := service.repairBuzzOrphanRoots(request.Context(), apply)
	if errorValue != nil {
		log.Printf("buzz repair failed (apply=%v): %v", apply, errorValue)
		http.Error(responseWriter, "buzz_repair_failed: "+errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, report)
}

func (service *Service) repairBuzzOrphanRoots(ctx context.Context, apply bool) (buzzRepairResponse, error) {
	seed := service.buzzKeySeed()
	databaseURL := strings.TrimSpace(service.Configuration.BuzzDatabaseURL)
	if seed == "" || databaseURL == "" {
		return buzzRepairResponse{}, errors.New("buzz key seed and database url must be configured")
	}
	token, errorValue := service.mattermostBotToken()
	if errorValue != nil {
		return buzzRepairResponse{}, errorValue
	}

	client := mattermostrest.Client{
		BaseURL: strings.TrimRight(service.Configuration.MattermostBaseURL, "/"),
		Token:   token,
	}
	teamID, errorValue := client.Team(ctx, service.Configuration.MattermostTeamName)
	if errorValue != nil {
		return buzzRepairResponse{}, errorValue
	}
	users, errorValue := client.Users(ctx)
	if errorValue != nil {
		return buzzRepairResponse{}, errorValue
	}
	_, authorsByID := mattermostrest.UsersToChannelAuthorEmails(users)
	authorPubkeys := map[string]string{}
	for userID, author := range authorsByID {
		pubkey, errorValue := nostr.GetPublicKey(buzzidentity.Secret(seed, author.Email))
		if errorValue != nil {
			return buzzRepairResponse{}, errorValue
		}
		authorPubkeys[userID] = pubkey
	}
	bootstrapPubkey, errorValue := nostr.GetPublicKey(buzzidentity.Secret(seed, buzzidentity.BootstrapSubject))
	if errorValue != nil {
		return buzzRepairResponse{}, errorValue
	}

	channels, errorValue := client.PublicChannels(ctx, teamID)
	if errorValue != nil {
		return buzzRepairResponse{}, errorValue
	}

	database, errorValue := sql.Open("postgres", databaseURL)
	if errorValue != nil {
		return buzzRepairResponse{}, errorValue
	}
	defer database.Close()

	var communityID string
	if errorValue := database.QueryRowContext(ctx, `SELECT id FROM communities LIMIT 1`).Scan(&communityID); errorValue != nil {
		return buzzRepairResponse{}, fmt.Errorf("resolve community: %w", errorValue)
	}

	report := buzzRepairResponse{Apply: apply}
	for _, restChannel := range channels {
		channel := restChannel.ToImport()
		posts, errorValue := client.Posts(ctx, channel.ID, 0)
		if errorValue != nil {
			return buzzRepairResponse{}, fmt.Errorf("read posts for %s: %w", channel.Name, errorValue)
		}
		buzzChannelID := buzzidentity.ChannelID(seed, channel.ID)
		result := repairBuzzChannelOrphans(ctx, database, client, communityID, buzzChannelID, bootstrapPubkey, posts, authorPubkeys, apply)
		if result.Relinked == 0 && result.RewrittenRoots == 0 && result.DeletedRoots == 0 && result.KeptGenuine == 0 {
			continue
		}
		result.Name = channel.Name
		report.Channels = append(report.Channels, result)
		report.Relinked += result.Relinked
		report.RewrittenRoots += result.RewrittenRoots
		report.DeletedRoots += result.DeletedRoots
		report.KeptGenuine += result.KeptGenuine
	}
	return report, nil
}

func repairBuzzChannelOrphans(ctx context.Context, database *sql.DB, client mattermostrest.Client, communityID, buzzChannelID, bootstrapPubkey string, posts []buzzimport.MattermostPost, authorPubkeys map[string]string, apply bool) buzzRepairChannelResult {
	result := buzzRepairChannelResult{}
	mmPostByID := map[string]buzzimport.MattermostPost{}
	for _, post := range posts {
		mmPostByID[post.ID] = post
	}
	rootPostCache := map[string]buzzimport.MattermostPost{}
	buzzIDByKey := map[string]string{}
	keyByBuzzID := map[string]string{}
	rows, errorValue := database.QueryContext(ctx, `SELECT encode(id,'hex'), encode(pubkey,'hex'), extract(epoch from created_at)::bigint FROM events WHERE community_id=$1 AND channel_id=$2 AND kind=$3`, communityID, buzzChannelID, buzzStreamMessageKind)
	if errorValue != nil {
		log.Printf("buzz repair: query events failed for %s: %v", buzzChannelID, errorValue)
		return result
	}
	for rows.Next() {
		var idHex, pubHex string
		var second int64
		if errorValue := rows.Scan(&idHex, &pubHex, &second); errorValue != nil {
			continue
		}
		key := fmt.Sprintf("%s:%d", pubHex, second)
		buzzIDByKey[key] = idHex
		keyByBuzzID[idHex] = key
	}
	rows.Close()

	mmIDByKey := map[string]string{}
	for _, post := range posts {
		if pubkey := authorPubkeys[post.UserID]; pubkey != "" {
			mmIDByKey[fmt.Sprintf("%s:%d", pubkey, post.CreatedAt.Unix())] = post.ID
		}
	}

	mmRootPostForReply := func(replyBuzzID string) (buzzimport.MattermostPost, bool) {
		mmID := mmIDByKey[keyByBuzzID[replyBuzzID]]
		if mmID == "" {
			return buzzimport.MattermostPost{}, false
		}
		rootID := strings.TrimSpace(mmPostByID[mmID].RootID)
		if rootID == "" {
			return buzzimport.MattermostPost{}, false
		}
		if rootPost, present := mmPostByID[rootID]; present {
			return rootPost, true
		}
		if rootPost, cached := rootPostCache[rootID]; cached {
			return rootPost, rootPost.ID != ""
		}
		rootPost, found, errorValue := client.Post(ctx, rootID)
		if errorValue != nil {
			log.Printf("buzz repair: fetch root post %s failed: %v", rootID, errorValue)
			return buzzimport.MattermostPost{}, false
		}
		if !found || strings.TrimSpace(rootPost.Message) == "" {
			rootPost = buzzimport.MattermostPost{}
		}
		rootPostCache[rootID] = rootPost
		return rootPost, rootPost.ID != ""
	}
	buzzIDForRootPost := func(rootPost buzzimport.MattermostPost) string {
		return buzzIDByKey[fmt.Sprintf("%s:%d", authorPubkeys[rootPost.UserID], rootPost.CreatedAt.Unix())]
	}

	synthRoots := []string{}
	synthRows, errorValue := database.QueryContext(ctx, `SELECT encode(id,'hex') FROM events WHERE community_id=$1 AND channel_id=$2 AND content=$3 AND pubkey=decode($4,'hex')`, communityID, buzzChannelID, buzzOrphanRootMarker, bootstrapPubkey)
	if errorValue != nil {
		log.Printf("buzz repair: query synthetic roots failed for %s: %v", buzzChannelID, errorValue)
		return result
	}
	for synthRows.Next() {
		var idHex string
		if errorValue := synthRows.Scan(&idHex); errorValue == nil {
			synthRoots = append(synthRoots, idHex)
		}
	}
	synthRows.Close()

	for _, synthRoot := range synthRoots {
		replies := buzzOrphanReplyEventIDs(ctx, database, communityID, buzzChannelID, synthRoot)
		if len(replies) == 0 {
			if apply {
				buzzOrphanDeleteEvent(ctx, database, communityID, synthRoot)
			}
			result.DeletedRoots++
			continue
		}
		existingRealRoot := ""
		var realRootPost buzzimport.MattermostPost
		hasRealRootPost := false
		for _, reply := range replies {
			rootPost, resolved := mmRootPostForReply(reply)
			if !resolved || strings.TrimSpace(rootPost.ID) == "" {
				continue
			}
			realRootPost = rootPost
			hasRealRootPost = true
			if candidate := buzzIDForRootPost(rootPost); candidate != "" && candidate != synthRoot {
				existingRealRoot = candidate
				break
			}
		}
		switch {
		case existingRealRoot != "":
			if apply {
				buzzOrphanRelinkReplies(ctx, database, communityID, synthRoot, existingRealRoot, replies)
				buzzOrphanDeleteEvent(ctx, database, communityID, synthRoot)
				buzzOrphanRecomputeDescendants(ctx, database, communityID, existingRealRoot)
			}
			result.Relinked += len(replies)
			result.DeletedRoots++
		case hasRealRootPost:
			if apply {
				buzzOrphanRewriteRoot(ctx, database, communityID, synthRoot, realRootPost)
			}
			result.RewrittenRoots++
		default:
			if len(result.KeptSample) < 4 {
				result.KeptSample = append(result.KeptSample, buzzOrphanKeptSample(ctx, database, communityID, bootstrapPubkey, replies))
			}
			result.KeptGenuine++
		}
	}
	return result
}

func buzzOrphanReplyEventIDs(ctx context.Context, database *sql.DB, communityID, buzzChannelID, rootHex string) []string {
	rows, errorValue := database.QueryContext(ctx, `SELECT encode(id,'hex') FROM events WHERE community_id=$1 AND channel_id=$2 AND id<>decode($3,'hex') AND tags::text LIKE '%"' || $3 || '"%'`, communityID, buzzChannelID, rootHex)
	if errorValue != nil {
		return nil
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var idHex string
		if errorValue := rows.Scan(&idHex); errorValue == nil {
			ids = append(ids, idHex)
		}
	}
	return ids
}

func buzzOrphanKeptSample(ctx context.Context, database *sql.DB, communityID, bootstrapPubkey string, replies []string) string {
	if len(replies) == 0 {
		return "(no replies)"
	}
	var content, pubHex string
	if errorValue := database.QueryRowContext(ctx, `SELECT left(content,60), encode(pubkey,'hex') FROM events WHERE community_id=$1 AND id=decode($2,'hex')`, communityID, replies[0]).Scan(&content, &pubHex); errorValue != nil {
		return "(scan failed: " + errorValue.Error() + ")"
	}
	author := "human/" + pubHex[:8]
	if pubHex == bootstrapPubkey {
		author = "bootstrap-bot"
	}
	return fmt.Sprintf("replies=%d author=%s first=%q", len(replies), author, content)
}

func buzzOrphanRewriteRoot(ctx context.Context, database *sql.DB, communityID, synthRoot string, rootPost buzzimport.MattermostPost) {
	if _, errorValue := database.ExecContext(ctx, `UPDATE events SET content=$1, created_at=$2 WHERE community_id=$3 AND id=decode($4,'hex')`, rootPost.Message, rootPost.CreatedAt.UTC(), communityID, synthRoot); errorValue != nil {
		log.Printf("buzz repair: rewrite root %s (content+time) failed: %v; retrying content only", synthRoot[:12], errorValue)
		if _, errorValue := database.ExecContext(ctx, `UPDATE events SET content=$1 WHERE community_id=$2 AND id=decode($3,'hex')`, rootPost.Message, communityID, synthRoot); errorValue != nil {
			log.Printf("buzz repair: rewrite root %s content failed: %v", synthRoot[:12], errorValue)
		}
	}
}

func buzzOrphanRelinkReplies(ctx context.Context, database *sql.DB, communityID, synthRoot, realRoot string, replies []string) {
	for _, reply := range replies {
		if _, errorValue := database.ExecContext(ctx, `UPDATE events SET tags = replace(tags::text, $1, $2)::jsonb WHERE community_id=$3 AND id=decode($4,'hex')`, `"`+synthRoot+`"`, `"`+realRoot+`"`, communityID, reply); errorValue != nil {
			log.Printf("buzz repair: relink tags %s failed: %v", reply[:12], errorValue)
		}
		if _, errorValue := database.ExecContext(ctx, `UPDATE thread_metadata SET root_event_id=decode($1,'hex'), parent_event_id=decode($1,'hex') WHERE community_id=$2 AND event_id=decode($3,'hex')`, realRoot, communityID, reply); errorValue != nil {
			log.Printf("buzz repair: relink metadata %s failed: %v", reply[:12], errorValue)
		}
	}
}

func buzzOrphanDeleteEvent(ctx context.Context, database *sql.DB, communityID, eventHex string) {
	database.ExecContext(ctx, `DELETE FROM event_mentions WHERE community_id=$1 AND event_id=decode($2,'hex')`, communityID, eventHex)
	database.ExecContext(ctx, `DELETE FROM thread_metadata WHERE community_id=$1 AND event_id=decode($2,'hex')`, communityID, eventHex)
	database.ExecContext(ctx, `DELETE FROM events WHERE community_id=$1 AND id=decode($2,'hex')`, communityID, eventHex)
}

func buzzOrphanRecomputeDescendants(ctx context.Context, database *sql.DB, communityID, rootHex string) {
	database.ExecContext(ctx, `UPDATE thread_metadata SET descendant_count=(SELECT count(*) FROM thread_metadata t WHERE t.community_id=$1 AND t.root_event_id=decode($2,'hex')), reply_count=(SELECT count(*) FROM thread_metadata t WHERE t.community_id=$1 AND t.parent_event_id=decode($2,'hex')) WHERE community_id=$1 AND event_id=decode($2,'hex')`, communityID, rootHex)
}
