package admind

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strings"

	nostr "github.com/nbd-wtf/go-nostr"

	"github.com/lib/pq"

	"gitlab.com/eastriver/internkim/internal/buzzidentity"
	"gitlab.com/eastriver/internkim/internal/buzzimport/mattermostrest"
)

type buzzStrangerMembersReport struct {
	Strangers   int      `json:"strangers"`
	Memberships int      `json:"memberships"`
	Removed     int      `json:"removed"`
	Emails      []string `json:"emails"`
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
	if !service.authorizeInternalOrWebStaffRequest(request) {
		http.Error(responseWriter, "staff access required", http.StatusForbidden)
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
	if !apply {
		return report, nil
	}
	result, errorValue := database.ExecContext(ctx,
		"DELETE FROM channel_members WHERE pubkey = ANY("+asBytea+")",
		pq.Array(strangers),
	)
	if errorValue != nil {
		return buzzStrangerMembersReport{}, errorValue
	}
	affected, _ := result.RowsAffected()
	report.Removed = int(affected)
	return report, nil
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
		pubkey, errorValue := nostr.GetPublicKey(buzzidentity.Secret(seed, email))
		if errorValue != nil {
			return nil, nil, errorValue
		}
		seen[email] = true
		emails = append(emails, email)
		pubkeys = append(pubkeys, pubkey)
	}
	return pubkeys, emails, nil
}
