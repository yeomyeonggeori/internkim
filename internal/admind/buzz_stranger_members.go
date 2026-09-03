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
	report.Unaccounted, errorValue = service.membersNobodyAccountsFor(ctx, database, seed)
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

// A key nobody derives is somebody the company never issued: it shows in the
// people list and can be mentioned, and no address reaches it. It goes from the
// rooms, from the community, and from the people list, and the rooms it was in
// are told, so a client sees the list it will get on its next read.
//
// A key that has said something is left alone whatever else is true of it.
// Taking it out would take its messages out of the conversations they are part
// of, and that is a person's judgement, not a sweep's.
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
	unaccounted, errorValue := keysThatSaidNothing(ctx, database, unaccounted)
	if errorValue != nil || len(unaccounted) == 0 {
		return 0, errorValue
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
	if _, errorValue := rowsChangedBy(ctx, database,
		"DELETE FROM relay_members WHERE lower(pubkey) = ANY($1) AND role <> 'owner'",
		pq.Array(unaccounted)); errorValue != nil {
		return removed, errorValue
	}
	if _, errorValue := rowsChangedBy(ctx, database,
		"DELETE FROM events WHERE kind = 0 AND pubkey = ANY("+asBytea+")",
		pq.Array(unaccounted)); errorValue != nil {
		return removed, errorValue
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
// The audit and the sweep read one roster, so a key the sweep would take back
// is exactly the key this reports, and a key it reports is one the sweep takes.
func (service *Service) membersNobodyAccountsFor(ctx context.Context, database *sql.DB, seed string) ([]string, error) {
	accounted, errorValue := service.accountedBuzzPubkeys(ctx, seed)
	if errorValue != nil {
		return nil, errorValue
	}
	// Looking only at the rooms is how a key nobody derives outlived every
	// sweep: a client reads relay_members to list and mention people, so a key
	// that is a member of the community and of no room is a person in the
	// messenger that no pass could see.
	rows, errorValue := database.QueryContext(ctx, `
		SELECT DISTINCT encode(pubkey, 'hex') FROM channel_members WHERE removed_at IS NULL
		UNION
		SELECT DISTINCT lower(pubkey) FROM relay_members WHERE role <> 'owner'`)
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

// The company issues one key per address and derives it, so a key in the
// community that no address derives was never issued here. It arrives through
// doors that let a client bring its own — redeeming a Buzz invite, or a member
// pasting one into settings — and it appears in the people list as somebody who
// can be mentioned and cannot be reached. Waiting for a person to notice is how
// one colleague came to be in the messenger twice.
func (service *Service) showOutWhoeverNobodyNames(ctx context.Context) {
	databaseURL := strings.TrimSpace(service.Configuration.BuzzDatabaseURL)
	seed := service.buzzKeySeed()
	if databaseURL == "" || seed == "" {
		return
	}
	relay, errorValue := sql.Open("postgres", databaseURL)
	if errorValue != nil {
		log.Printf("buzz: the community could not be read: %v", errorValue)
		return
	}
	defer relay.Close()

	unaccounted, errorValue := service.membersNobodyAccountsFor(ctx, relay, seed)
	if errorValue != nil {
		log.Printf("buzz: nobody is shown out, %v", errorValue)
		return
	}
	if len(unaccounted) == 0 {
		return
	}
	removed, errorValue := service.removeUnaccountedMemberships(ctx, relay, unaccounted)
	if errorValue != nil {
		log.Printf("buzz: showing out a key nobody names failed: %v", errorValue)
		return
	}
	if removed > 0 {
		log.Printf("buzz: showed out %d seat(s) held by keys nobody names, of %d found", removed, len(unaccounted))
	}
}

func keysThatSaidNothing(ctx context.Context, database *sql.DB, pubkeys []string) ([]string, error) {
	rows, errorValue := database.QueryContext(ctx,
		"SELECT DISTINCT encode(pubkey, 'hex') FROM events WHERE kind = 9 AND pubkey = ANY(ARRAY(SELECT decode(unnest($1::text[]), 'hex')))",
		pq.Array(pubkeys))
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	spoke := map[string]bool{}
	for rows.Next() {
		var pubkey string
		if errorValue := rows.Scan(&pubkey); errorValue != nil {
			return nil, errorValue
		}
		spoke[pubkey] = true
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	for _, pubkey := range pubkeys {
		if spoke[pubkey] {
			log.Printf("buzz: %s belongs to nobody the company names and has said something, so it stays for a person to judge", pubkey)
		}
	}
	return keysSafeToShowOut(pubkeys, spoke), nil
}

func keysSafeToShowOut(pubkeys []string, spoke map[string]bool) []string {
	silent := []string{}
	for _, pubkey := range pubkeys {
		if spoke[pubkey] {
			continue
		}
		silent = append(silent, pubkey)
	}
	return silent
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

	token, errorValue := service.mattermostAdmin().BotToken()
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
		held, errorValue := service.everyKeyHeldBy(ctx, seed, email)
		if errorValue != nil {
			return nil, nil, errorValue
		}
		pubkeys = append(pubkeys, held...)
	}
	return pubkeys, emails, nil
}

// A person who was rotated to a fresh identity holds the key of every version
// they have had, and the current one is the version the vault names. Deriving
// only version one calls the person they are today a stranger.
func (service *Service) everyKeyHeldBy(ctx context.Context, seed string, email string) ([]string, error) {
	vaultSubject, errorValue := service.buzzVaultSubject(ctx, email)
	if errorValue != nil {
		return nil, errorValue
	}
	version := service.buzzIdentityVersion(vaultSubject)
	keys := []string{}
	for held := 1; held <= version; held++ {
		pubkey, errorValue := nostr.GetPublicKey(buzzidentity.Secret(seed, versionedSubject(email, held)))
		if errorValue != nil {
			continue
		}
		keys = append(keys, pubkey)
	}
	return keys, nil
}
