package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	nostr "github.com/nbd-wtf/go-nostr"

	"gitlab.com/eastriver/internkim/internal/buzzidentity"
)

type buzzIdentityKeyReport struct {
	Version     int    `json:"version"`
	Pubkey      string `json:"pubkey"`
	IsCurrent   bool   `json:"isCurrent"`
	HasProfile  bool   `json:"hasProfile"`
	ProfileName string `json:"profileName,omitempty"`
	HasPicture  bool   `json:"hasPicture"`
	Memberships int    `json:"memberships"`
	Messages    int    `json:"messages"`
}

type buzzIdentityPersonReport struct {
	Email      string                  `json:"email"`
	Version    int                     `json:"version"`
	Identities []buzzIdentityKeyReport `json:"identities"`
}

type buzzIdentityMemberReport struct {
	Pubkey    string `json:"pubkey"`
	Owner     string `json:"owner,omitempty"`
	Version   int    `json:"version,omitempty"`
	IsCurrent bool   `json:"isCurrent"`
}

type buzzIdentityChannelReport struct {
	ChannelID   string                     `json:"channelID"`
	Name        string                     `json:"name,omitempty"`
	ChannelType string                     `json:"channelType"`
	Members     []buzzIdentityMemberReport `json:"members"`
}

type buzzIdentityReport struct {
	People   []buzzIdentityPersonReport  `json:"people"`
	Channels []buzzIdentityChannelReport `json:"channels"`
}

// The report answers, for every identity this device can derive, whether the
// relay still carries its profile and where its keys sit in channel
// memberships. A reset rotates a person to a fresh key but the rooms and the
// profile of the old one stay where they were, and this is the one place that
// lays the whole ledger side by side before anything is repaired.
func (service *Service) handleBuzzIdentityReport(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.NotFound(responseWriter, request)
		return
	}
	if !service.authorizeInternalOrWebStaffRequest(request) {
		http.Error(responseWriter, "staff access required", http.StatusForbidden)
		return
	}
	report, errorValue := service.buzzIdentityLedger(request.Context())
	if errorValue != nil {
		log.Printf("buzz identity report failed: %v", errorValue)
		http.Error(responseWriter, "buzz_identity_report_failed: "+errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, report)
}

type buzzIdentityLedgerRows struct {
	profileByPubkey    map[string]buzzProfileContent
	membershipByPubkey map[string]int
	messagesByPubkey   map[string]int
	channels           []buzzLedgerChannel
}

type buzzProfileContent struct {
	Name    string `json:"name"`
	Display string `json:"display_name"`
	Picture string `json:"picture"`
}

type buzzLedgerChannel struct {
	id          string
	name        string
	channelType string
	members     []string
}

type buzzKeyOwner struct {
	owner     string
	version   int
	isCurrent bool
}

func (service *Service) buzzIdentityLedger(ctx context.Context) (buzzIdentityReport, error) {
	seed := service.buzzKeySeed()
	databaseURL := strings.TrimSpace(service.Configuration.BuzzDatabaseURL)
	if seed == "" || databaseURL == "" {
		return buzzIdentityReport{}, errors.New("buzz key seed and database url must be configured")
	}
	emails, errorValue := service.companyPeopleEmails(ctx)
	if errorValue != nil {
		return buzzIdentityReport{}, errorValue
	}
	versionOf := func(email string) int {
		return service.buzzIdentityVersion(service.buzzVaultSubject(ctx, email))
	}
	owners, errorValue := buzzKeyOwners(seed, emails, versionOf)
	if errorValue != nil {
		return buzzIdentityReport{}, errorValue
	}

	database, errorValue := sql.Open("postgres", databaseURL)
	if errorValue != nil {
		return buzzIdentityReport{}, errorValue
	}
	defer database.Close()
	rows, errorValue := readBuzzIdentityLedgerRows(ctx, database)
	if errorValue != nil {
		return buzzIdentityReport{}, errorValue
	}
	return assembleBuzzIdentityReport(emails, versionOf, seed, owners, rows)
}

func buzzKeyOwners(seed string, emails []string, versionOf func(string) int) (map[string]buzzKeyOwner, error) {
	owners := map[string]buzzKeyOwner{}
	claim := func(subject string, owner buzzKeyOwner) error {
		pubkey, errorValue := nostr.GetPublicKey(buzzidentity.Secret(seed, subject))
		if errorValue != nil {
			return errorValue
		}
		owners[pubkey] = owner
		return nil
	}
	if errorValue := claim(buzzidentity.BootstrapSubject, buzzKeyOwner{owner: "bootstrap", isCurrent: true}); errorValue != nil {
		return nil, errorValue
	}
	if errorValue := claim(buzzidentity.AgentSubject, buzzKeyOwner{owner: "agent", isCurrent: true}); errorValue != nil {
		return nil, errorValue
	}
	for _, email := range emails {
		current := versionOf(email)
		for version := 1; version <= current; version++ {
			owner := buzzKeyOwner{owner: email, version: version, isCurrent: version == current}
			if errorValue := claim(versionedSubject(email, version), owner); errorValue != nil {
				return nil, errorValue
			}
		}
	}
	return owners, nil
}

func readBuzzIdentityLedgerRows(ctx context.Context, database *sql.DB) (buzzIdentityLedgerRows, error) {
	ledger := buzzIdentityLedgerRows{
		profileByPubkey:    map[string]buzzProfileContent{},
		membershipByPubkey: map[string]int{},
		messagesByPubkey:   map[string]int{},
	}
	profileRows, errorValue := database.QueryContext(ctx,
		"SELECT DISTINCT ON (pubkey) encode(pubkey,'hex'), content FROM events WHERE kind = 0 ORDER BY pubkey, created_at DESC")
	if errorValue != nil {
		return ledger, errorValue
	}
	defer profileRows.Close()
	for profileRows.Next() {
		var pubkey, content string
		if errorValue := profileRows.Scan(&pubkey, &content); errorValue != nil {
			return ledger, errorValue
		}
		var profile buzzProfileContent
		if errorValue := json.Unmarshal([]byte(content), &profile); errorValue != nil {
			continue
		}
		ledger.profileByPubkey[pubkey] = profile
	}
	if errorValue := profileRows.Err(); errorValue != nil {
		return ledger, errorValue
	}

	if errorValue := countsByPubkey(ctx, database,
		"SELECT encode(pubkey,'hex'), count(*) FROM channel_members WHERE removed_at IS NULL GROUP BY 1",
		ledger.membershipByPubkey); errorValue != nil {
		return ledger, errorValue
	}
	if errorValue := countsByPubkey(ctx, database,
		"SELECT encode(pubkey,'hex'), count(*) FROM events WHERE kind = 9 GROUP BY 1",
		ledger.messagesByPubkey); errorValue != nil {
		return ledger, errorValue
	}

	channelRows, errorValue := database.QueryContext(ctx, `
		SELECT c.id::text, c.name, c.channel_type, encode(m.pubkey,'hex')
		FROM channels c JOIN channel_members m ON m.channel_id = c.id
		WHERE c.deleted_at IS NULL AND m.removed_at IS NULL
		ORDER BY c.id`)
	if errorValue != nil {
		return ledger, errorValue
	}
	defer channelRows.Close()
	channelsByID := map[string]*buzzLedgerChannel{}
	order := []string{}
	for channelRows.Next() {
		var id, name, channelType, pubkey string
		if errorValue := channelRows.Scan(&id, &name, &channelType, &pubkey); errorValue != nil {
			return ledger, errorValue
		}
		channel := channelsByID[id]
		if channel == nil {
			channel = &buzzLedgerChannel{id: id, name: name, channelType: channelType}
			channelsByID[id] = channel
			order = append(order, id)
		}
		channel.members = append(channel.members, pubkey)
	}
	if errorValue := channelRows.Err(); errorValue != nil {
		return ledger, errorValue
	}
	for _, id := range order {
		ledger.channels = append(ledger.channels, *channelsByID[id])
	}
	return ledger, nil
}

func countsByPubkey(ctx context.Context, database *sql.DB, statement string, into map[string]int) error {
	rows, errorValue := database.QueryContext(ctx, statement)
	if errorValue != nil {
		return errorValue
	}
	defer rows.Close()
	for rows.Next() {
		var pubkey string
		var count int
		if errorValue := rows.Scan(&pubkey, &count); errorValue != nil {
			return errorValue
		}
		into[pubkey] = count
	}
	return rows.Err()
}

func assembleBuzzIdentityReport(
	emails []string,
	versionOf func(string) int,
	seed string,
	owners map[string]buzzKeyOwner,
	rows buzzIdentityLedgerRows,
) (buzzIdentityReport, error) {
	report := buzzIdentityReport{People: []buzzIdentityPersonReport{}, Channels: []buzzIdentityChannelReport{}}
	for _, email := range emails {
		current := versionOf(email)
		person := buzzIdentityPersonReport{Email: email, Version: current}
		for version := 1; version <= current; version++ {
			pubkey, errorValue := nostr.GetPublicKey(buzzidentity.Secret(seed, versionedSubject(email, version)))
			if errorValue != nil {
				return report, errorValue
			}
			profile, hasProfile := rows.profileByPubkey[pubkey]
			person.Identities = append(person.Identities, buzzIdentityKeyReport{
				Version:     version,
				Pubkey:      pubkey,
				IsCurrent:   version == current,
				HasProfile:  hasProfile,
				ProfileName: firstNonEmpty(profile.Display, profile.Name),
				HasPicture:  profile.Picture != "",
				Memberships: rows.membershipByPubkey[pubkey],
				Messages:    rows.messagesByPubkey[pubkey],
			})
		}
		report.People = append(report.People, person)
	}

	for _, channel := range rows.channels {
		classified := buzzIdentityChannelReport{ChannelID: channel.id, Name: channel.name, ChannelType: channel.channelType}
		holdsStaleMember := false
		for _, pubkey := range channel.members {
			owner, isKnown := owners[pubkey]
			member := buzzIdentityMemberReport{Pubkey: pubkey}
			if isKnown {
				member.Owner = owner.owner
				member.Version = owner.version
				member.IsCurrent = owner.isCurrent
			}
			if !isKnown || !owner.isCurrent {
				holdsStaleMember = true
			}
			classified.Members = append(classified.Members, member)
		}
		if channel.channelType == "dm" || holdsStaleMember {
			report.Channels = append(report.Channels, classified)
		}
	}
	return report, nil
}
