package admind

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	nostr "github.com/nbd-wtf/go-nostr"

	"github.com/yeomyeonggeori/internkim/internal/buzzidentity"
	"github.com/yeomyeonggeori/internkim/internal/buzzimport"
)

const editMessageKind = 40003

type oldLinkRewriteReport struct {
	Messages  int      `json:"messages"`
	Rewritten int      `json:"rewritten"`
	Unsigned  int      `json:"unsigned"`
	Failed    []string `json:"failed"`
}

func (service *Service) handleBuzzRewriteOldLinks(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.NotFound(responseWriter, request)
		return
	}
	if !service.authorizeInternalOrWebMemberRequest(request) {
		http.Error(responseWriter, "member access required", http.StatusForbidden)
		return
	}
	report, errorValue := service.rewriteOldLinksInBuzz(request.Context(), request.URL.Query().Get("apply") == "true")
	if errorValue != nil {
		log.Printf("buzz link rewrite failed: %v", errorValue)
		http.Error(responseWriter, "buzz_link_rewrite_failed: "+errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, report)
}

// Each message keeps its own author: an edit signed by anyone else is somebody
// putting words in their mouth. Every key this company has is derived from one
// seed, so the author of a message it holds can always sign the correction to it.
func (service *Service) secretsByPubkey(ctx context.Context, seed string) (map[string]string, error) {
	emails, errorValue := service.companyPeopleEmails(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	secrets := map[string]string{}
	remember := func(secret string) {
		pubkey, errorValue := nostr.GetPublicKey(secret)
		if errorValue == nil {
			secrets[pubkey] = secret
		}
	}
	for _, email := range emails {
		secret, errorValue := service.personBuzzSecret(ctx, email)
		if errorValue == nil {
			remember(secret)
		}
	}
	remember(buzzidentity.Secret(seed, buzzidentity.BootstrapSubject))
	remember(buzzidentity.Secret(seed, buzzidentity.AgentSubject))
	return secrets, nil
}

func (service *Service) rewriteOldLinksInBuzz(ctx context.Context, apply bool) (oldLinkRewriteReport, error) {
	seed := service.buzzKeySeed()
	appURL := strings.TrimSpace(service.Configuration.CentralPlaneAppURL)
	deviceHost := deviceHostOf(service.Configuration.DeviceURLPath)
	if seed == "" || appURL == "" || deviceHost == "" {
		return oldLinkRewriteReport{}, errors.New("the seed, the relay, the company address and this device's host are all needed")
	}
	translation := linkTranslation{
		appURL:           appURL,
		deviceHost:       deviceHost,
		recordCalendarID: service.linkedCalendarEventID,
		recordTaskID:     service.linkedTaskID,
	}

	database, errorValue := service.buzzDatabase()
	if errorValue != nil {
		return oldLinkRewriteReport{}, errorValue
	}

	var communityID string
	if errorValue := database.QueryRowContext(ctx, `SELECT id FROM communities LIMIT 1`).Scan(&communityID); errorValue != nil {
		return oldLinkRewriteReport{}, errorValue
	}

	carried, errorValue := messagesNamingTheDevice(ctx, database, deviceHost)
	if errorValue != nil {
		return oldLinkRewriteReport{}, errorValue
	}

	secrets, errorValue := service.secretsByPubkey(ctx, seed)
	if errorValue != nil {
		return oldLinkRewriteReport{}, errorValue
	}

	report := oldLinkRewriteReport{Messages: len(carried)}
	injector := buzzimport.ChannelInjector{Database: database, CommunityID: communityID}
	for _, one := range carried {
		secret, isOurs := secrets[one.pubkey]
		if !isOurs {
			report.Unsigned++
			continue
		}
		rewritten := translation.rewrite(one.content)
		if rewritten == one.content {
			continue
		}
		if !apply {
			report.Rewritten++
			continue
		}
		event := nostr.Event{
			CreatedAt: nostr.Now(),
			Kind:      editMessageKind,
			Tags:      nostr.Tags{nostr.Tag{"h", one.channelID}, nostr.Tag{"e", one.eventID}},
			Content:   rewritten,
		}
		if errorValue := event.Sign(secret); errorValue != nil {
			report.Failed = append(report.Failed, one.eventID+": "+errorValue.Error())
			continue
		}
		if errorValue := injector.InjectEdit(ctx, one.channelID, event); errorValue != nil {
			report.Failed = append(report.Failed, one.eventID+": "+errorValue.Error())
			continue
		}
		report.Rewritten++
	}
	return report, nil
}

type carriedLink struct{ eventID, pubkey, channelID, content string }

func messagesNamingTheDevice(ctx context.Context, database *sql.DB, deviceHost string) ([]carriedLink, error) {
	rows, errorValue := database.QueryContext(ctx, `
		SELECT encode(id, 'hex'), encode(pubkey, 'hex'), channel_id::text, content
		FROM events WHERE kind = 9 AND content LIKE '%' || $1 || '%'`, deviceHost)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	carried := []carriedLink{}
	for rows.Next() {
		var one carriedLink
		if errorValue := rows.Scan(&one.eventID, &one.pubkey, &one.channelID, &one.content); errorValue != nil {
			return nil, errorValue
		}
		carried = append(carried, one)
	}
	return carried, rows.Err()
}

func deviceHostOf(deviceURLPath string) string {
	contents, errorValue := os.ReadFile(deviceURLPath)
	if errorValue != nil {
		return ""
	}
	host := strings.TrimSpace(string(contents))
	if index := strings.Index(host, "://"); index >= 0 {
		host = host[index+3:]
	}
	return strings.TrimSuffix(strings.Split(host, "/")[0], ".")
}
