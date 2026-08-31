package admind

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"strings"

	nostr "github.com/nbd-wtf/go-nostr"

	"github.com/lib/pq"

	"gitlab.com/eastriver/internkim/internal/buzzidentity"
	"gitlab.com/eastriver/internkim/internal/buzzimport/mattermostrest"
)

// The channel timeline reads "X added by 여명거리, along with ..." from the
// notices the import published, and removing a membership does not unsay one.
// kind 44100 is that notice; its p tags name who was added.
// Every "added by 여명거리" line is the import announcing itself, signed by the
// key that made the channels. Whoever it names, nobody needs to read a
// migration's own bookkeeping.
const noticesTheImportPublished = `kind = 44100 AND pubkey = decode($1, 'hex')`

const noticesNamingAStranger = `kind = 44100 AND EXISTS (
	SELECT 1 FROM jsonb_array_elements(tags) tag
	WHERE tag->>0 = 'p' AND tag->>1 = ANY($1))`

type buzzStrangerMembersReport struct {
	Strangers        int      `json:"strangers"`
	Memberships      int      `json:"memberships"`
	Removed          int      `json:"removed"`
	Profiles         int      `json:"profiles"`
	ProfilesRemoved  int      `json:"profilesRemoved"`
	Community        int      `json:"community"`
	CommunityRemoved int      `json:"communityRemoved"`
	Notices          int      `json:"notices"`
	NoticesRemoved   int      `json:"noticesRemoved"`
	ImportNotices    int      `json:"importNotices"`
	ImportRemoved    int      `json:"importRemoved"`
	UnaccountedGone  int      `json:"unaccountedMembershipsRemoved"`
	Unaccounted      []string `json:"unaccounted"`
	Emails           []string `json:"emails"`
}

// The import made a Buzz identity for every Mattermost account it found, and a
// messenger keeps accounts a probe made, its own admin, and its system bot.
// Their profiles are gone, which only turned the names into bare keys in the
// member lists they are still in. Who is a person of this company is the
// directory's answer; a Mattermost account it does not name is not one.
func (service *Service) handleBuzzStrangerMembers(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.NotFound(responseWriter, request)
		return
	}
	if !service.authorizeInternalOrWebMemberRequest(request) {
		http.Error(responseWriter, "member access required", http.StatusForbidden)
		return
	}
	report, errorValue := service.removeStrangerBuzzMembers(request.Context(), request.URL.Query().Get("apply") == "true")
	if errorValue != nil {
		log.Printf("buzz stranger members failed: %v", errorValue)
		http.Error(responseWriter, "buzz_stranger_members_failed: "+errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, report)
}

func (service *Service) removeStrangerBuzzMembers(ctx context.Context, apply bool) (buzzStrangerMembersReport, error) {
	seed := service.buzzKeySeed()
	databaseURL := strings.TrimSpace(service.Configuration.BuzzDatabaseURL)
	if seed == "" || databaseURL == "" {
		return buzzStrangerMembersReport{}, errors.New("buzz key seed and database url must be configured")
	}
	strangers, emails, errorValue := service.strangerBuzzPubkeys(ctx, seed)
	if errorValue != nil {
		return buzzStrangerMembersReport{}, errorValue
	}
	report := buzzStrangerMembersReport{Strangers: len(strangers), Emails: emails}
	if len(strangers) == 0 {
		return report, nil
	}

	database, errorValue := sql.Open("postgres", databaseURL)
	if errorValue != nil {
		return buzzStrangerMembersReport{}, errorValue
	}
	defer database.Close()

	const asBytea = "ARRAY(SELECT decode(unnest($1::text[]), 'hex'))"
	if errorValue := database.QueryRowContext(ctx,
		"SELECT count(*) FROM channel_members WHERE pubkey = ANY("+asBytea+") AND removed_at IS NULL",
		pq.Array(strangers),
	).Scan(&report.Memberships); errorValue != nil {
		return buzzStrangerMembersReport{}, errorValue
	}
	if errorValue := database.QueryRowContext(ctx,
		"SELECT count(*) FROM events WHERE kind=0 AND pubkey = ANY("+asBytea+")",
		pq.Array(strangers),
	).Scan(&report.Profiles); errorValue != nil {
		return buzzStrangerMembersReport{}, errorValue
	}
	// relay_members keys by the hex text, not the bytes, and it is what a client
	// reads to offer someone in a mention.
	if errorValue := database.QueryRowContext(ctx,
		"SELECT count(*) FROM relay_members WHERE pubkey = ANY($1)",
		pq.Array(strangers),
	).Scan(&report.Community); errorValue != nil {
		return buzzStrangerMembersReport{}, errorValue
	}
	if errorValue := database.QueryRowContext(ctx,
		"SELECT count(*) FROM events WHERE "+noticesNamingAStranger,
		pq.Array(strangers),
	).Scan(&report.Notices); errorValue != nil {
		return buzzStrangerMembersReport{}, errorValue
	}
	bootstrapPubkey, errorValue := nostr.GetPublicKey(buzzidentity.Secret(seed, buzzidentity.BootstrapSubject))
	if errorValue != nil {
		return buzzStrangerMembersReport{}, errorValue
	}
	report.Unaccounted, errorValue = service.membersNobodyAccountsFor(ctx, database, seed, bootstrapPubkey)
	if errorValue != nil {
		return buzzStrangerMembersReport{}, errorValue
	}
	if errorValue := database.QueryRowContext(ctx,
		"SELECT count(*) FROM events WHERE "+noticesTheImportPublished,
		bootstrapPubkey,
	).Scan(&report.ImportNotices); errorValue != nil {
		return buzzStrangerMembersReport{}, errorValue
	}
	if !apply {
		return report, nil
	}

	report.Removed, errorValue = rowsChangedBy(ctx, database,
		"DELETE FROM channel_members WHERE pubkey = ANY("+asBytea+")", pq.Array(strangers))
	if errorValue != nil {
		return buzzStrangerMembersReport{}, errorValue
	}
	report.ProfilesRemoved, errorValue = rowsChangedBy(ctx, database,
		"DELETE FROM events WHERE kind=0 AND pubkey = ANY("+asBytea+")", pq.Array(strangers))
	if errorValue != nil {
		return buzzStrangerMembersReport{}, errorValue
	}
	report.CommunityRemoved, errorValue = rowsChangedBy(ctx, database,
		"DELETE FROM relay_members WHERE pubkey = ANY($1) AND role <> 'owner'", pq.Array(strangers))
	if errorValue != nil {
		return buzzStrangerMembersReport{}, errorValue
	}
	report.NoticesRemoved, errorValue = rowsChangedBy(ctx, database,
		"DELETE FROM events WHERE "+noticesNamingAStranger, pq.Array(strangers))
	if errorValue != nil {
		return buzzStrangerMembersReport{}, errorValue
	}
	report.ImportRemoved, errorValue = rowsChangedBy(ctx, database,
		"DELETE FROM events WHERE "+noticesTheImportPublished, bootstrapPubkey)
	if errorValue != nil {
		return buzzStrangerMembersReport{}, errorValue
	}
	report.UnaccountedGone, errorValue = service.removeUnaccountedMemberships(ctx, database, report.Unaccounted)
	if errorValue != nil {
		return buzzStrangerMembersReport{}, errorValue
	}
	return report, nil
}

// A key the messenger deleted so hard that nothing names it any more still sits
// in the rooms it was added to, where a member list reads it as bare hex. It is
// taken out of every room, and the rooms it was in are told, so a client sees
// the list it will get on its next read.
func (service *Service) removeUnaccountedMemberships(
	ctx context.Context,
	database *sql.DB,
	unaccounted []string,
) (int, error) {
	if len(unaccounted) == 0 {
		return 0, nil
	}
	for _, pubkey := range unaccounted {
		if _, errorValue := hex.DecodeString(pubkey); errorValue != nil {
			return 0, errorValue
		}
	}
	const asBytea = "ARRAY(SELECT decode(unnest($1::text[]), 'hex'))"
	rooms, errorValue := roomsHoldingMembers(ctx, database, unaccounted)
	if errorValue != nil {
		return 0, errorValue
	}
	removed, errorValue := rowsChangedBy(ctx, database,
		"DELETE FROM channel_members WHERE pubkey = ANY("+asBytea+")", pq.Array(unaccounted))
	if errorValue != nil {
		return 0, errorValue
	}
	for _, channelID := range rooms {
		if errorValue := service.tellClientsWhoIsInTheRoom(ctx, database, channelID); errorValue != nil {
			return removed, errorValue
		}
	}
	return removed, nil
}

func roomsHoldingMembers(ctx context.Context, database *sql.DB, pubkeys []string) ([]string, error) {
	const asBytea = "ARRAY(SELECT decode(unnest($1::text[]), 'hex'))"
	rows, errorValue := database.QueryContext(ctx,
		"SELECT DISTINCT channel_id::text FROM channel_members WHERE pubkey = ANY("+asBytea+")", pq.Array(pubkeys))
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	rooms := []string{}
	for rows.Next() {
		var channelID string
		if errorValue := rows.Scan(&channelID); errorValue != nil {
			return nil, errorValue
		}
		rooms = append(rooms, channelID)
	}
	return rooms, rows.Err()
}

// A messenger that forgot an account cannot say who it was, so asking it which
// identities to remove misses exactly the ones it deleted hardest. This asks the
// other way: who is in a channel that nobody the company knows accounts for.
func (service *Service) membersNobodyAccountsFor(ctx context.Context, database *sql.DB, seed, bootstrapPubkey string) ([]string, error) {
	emails, errorValue := service.companyPeopleEmails(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	accounted := map[string]bool{bootstrapPubkey: true}
	agentPubkey, errorValue := nostr.GetPublicKey(buzzidentity.Secret(seed, buzzidentity.AgentSubject))
	if errorValue != nil {
		return nil, errorValue
	}
	accounted[agentPubkey] = true
	for _, email := range emails {
		for _, pubkey := range service.everyKeyHeldBy(ctx, seed, email) {
			accounted[pubkey] = true
		}
	}
	rows, errorValue := database.QueryContext(ctx,
		"SELECT DISTINCT encode(pubkey, 'hex') FROM channel_members WHERE removed_at IS NULL")
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	unaccounted := []string{}
	for rows.Next() {
		var pubkey string
		if errorValue := rows.Scan(&pubkey); errorValue != nil {
			return nil, errorValue
		}
		if !accounted[pubkey] {
			unaccounted = append(unaccounted, pubkey)
		}
	}
	return unaccounted, rows.Err()
}

func rowsChangedBy(ctx context.Context, database *sql.DB, statement string, argument any) (int, error) {
	result, errorValue := database.ExecContext(ctx, statement, argument)
	if errorValue != nil {
		return 0, errorValue
	}
	affected, errorValue := result.RowsAffected()
	if errorValue != nil {
		return 0, errorValue
	}
	return int(affected), nil
}

func (service *Service) strangerBuzzPubkeys(ctx context.Context, seed string) ([]string, []string, error) {
	ourEmails, errorValue := service.companyPeopleEmails(ctx)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	ours := map[string]bool{}
	for _, email := range ourEmails {
		ours[email] = true
	}

	token, errorValue := service.mattermostBotToken()
	if errorValue != nil {
		return nil, nil, errorValue
	}
	client := mattermostrest.Client{
		BaseURL: strings.TrimRight(service.Configuration.MattermostBaseURL, "/"),
		Token:   token,
	}
	users, errorValue := client.Users(ctx)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	_, authorsByID := mattermostrest.UsersToChannelAuthorEmails(users)

	pubkeys := []string{}
	emails := []string{}
	seen := map[string]bool{}
	for _, author := range authorsByID {
		email := strings.ToLower(strings.TrimSpace(author.Email))
		if email == "" || ours[email] || seen[email] {
			continue
		}
		seen[email] = true
		emails = append(emails, email)
		pubkeys = append(pubkeys, service.everyKeyHeldBy(ctx, seed, email)...)
	}
	return pubkeys, emails, nil
}

// A person who was rotated to a fresh identity holds the key of every version
// they have had, and the current one is the version the vault names. Deriving
// only version one calls the person they are today a stranger.
func (service *Service) everyKeyHeldBy(ctx context.Context, seed string, email string) []string {
	version := service.buzzIdentityVersion(service.buzzVaultSubject(ctx, email))
	keys := []string{}
	for held := 1; held <= version; held++ {
		pubkey, errorValue := nostr.GetPublicKey(buzzidentity.Secret(seed, versionedSubject(email, held)))
		if errorValue != nil {
			continue
		}
		keys = append(keys, pubkey)
	}
	return keys
}
