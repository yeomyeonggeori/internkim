package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	nostr "github.com/nbd-wtf/go-nostr"
	"gitlab.com/eastriver/internkim/internal/buzzidentity"
	"gitlab.com/eastriver/internkim/internal/buzzimport"

	_ "github.com/lib/pq"
)

type buzzAdminResetRequest struct {
	Email string `json:"email"`
}

type buzzAdminResetResponse struct {
	Email        string `json:"email"`
	OldVersion   int    `json:"oldVersion"`
	NewVersion   int    `json:"newVersion"`
	Reattributed int    `json:"reattributed"`
}

// handleBuzzAdminReset rotates a person to a fresh identity (A') and re-attributes
// their history to it: every message they authored is re-signed under the new
// key with its original timestamp and injected into the relay, references are
// remapped to the new event ids, and the old identity's messages are deleted.
// For a leaked key — the new key revokes the old while the person keeps their
// history under the same email anchor. The person re-claims A' on next sign-in
// (their sealed copies are cleared).
func (service *Service) handleBuzzAdminReset(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.NotFound(responseWriter, request)
		return
	}
	if !service.authorizeInternalOrWebStaffRequest(request) {
		http.Error(responseWriter, "staff access required", http.StatusForbidden)
		return
	}
	callerEmail := service.webActorEmail(request)
	if !service.isCurrentAdminEmail(request.Context(), callerEmail) && !service.isClaimedAdminEmail(callerEmail) {
		http.Error(responseWriter, "admin access required", http.StatusForbidden)
		return
	}
	var payload buzzAdminResetRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, "invalid request", http.StatusBadRequest)
		return
	}
	email := strings.ToLower(strings.TrimSpace(payload.Email))
	if email == "" {
		http.Error(responseWriter, "email is required", http.StatusBadRequest)
		return
	}

	seed := service.buzzKeySeed()
	if seed == "" {
		http.Error(responseWriter, "buzz key seed is not configured", http.StatusNotImplemented)
		return
	}
	personSubject := service.buzzVaultSubject(request.Context(), email)
	oldVersion := service.buzzIdentityVersion(personSubject)
	oldSecret := buzzKeyForVersion(seed, email, oldVersion)
	newVersion, errorValue := service.bumpBuzzIdentityVersion(personSubject)
	if errorValue != nil {
		http.Error(responseWriter, "buzz_reset_failed", http.StatusInternalServerError)
		return
	}
	newSecret := buzzKeyForVersion(seed, email, newVersion)
	_ = service.deleteBuzzIdentitySecret(personSubject)

	reattributed, errorValue := service.reattributeBuzzMessages(request.Context(), oldSecret, newSecret)
	if errorValue != nil {
		http.Error(responseWriter, "buzz_reset_failed: "+errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, buzzAdminResetResponse{
		Email:        email,
		OldVersion:   oldVersion,
		NewVersion:   newVersion,
		Reattributed: reattributed,
	})
}

func buzzKeyForVersion(seed string, email string, version int) string {
	return buzzidentity.Secret(seed, versionedSubject(email, version))
}

func (service *Service) reattributeBuzzMessages(ctx context.Context, oldSecretHex string, newSecretHex string) (int, error) {
	relayURL := service.buzzRelayEffectiveURL()
	databaseURL := strings.TrimSpace(service.Configuration.BuzzDatabaseURL)
	communityID := strings.TrimSpace(service.Configuration.BuzzCommunityID)
	if relayURL == "" || databaseURL == "" || communityID == "" {
		return 0, errors.New("buzz relay URL, database URL, and community id must be configured")
	}
	oldPubkey, errorValue := nostr.GetPublicKey(oldSecretHex)
	if errorValue != nil {
		return 0, errorValue
	}

	relay, errorValue := nostr.RelayConnect(ctx, relayURL)
	if errorValue != nil {
		return 0, errorValue
	}
	defer relay.Close()
	time.Sleep(700 * time.Millisecond)
	_ = relay.Auth(ctx, func(event *nostr.Event) error { return event.Sign(oldSecretHex) })

	oldEvents, errorValue := relay.QuerySync(ctx, nostr.Filter{
		Kinds:   []int{buzzStreamMessageKind},
		Authors: []string{oldPubkey},
	})
	if errorValue != nil {
		return 0, errorValue
	}
	sort.Slice(oldEvents, func(first, second int) bool {
		return oldEvents[first].CreatedAt < oldEvents[second].CreatedAt
	})

	database, errorValue := sql.Open("postgres", databaseURL)
	if errorValue != nil {
		return 0, errorValue
	}
	defer database.Close()
	injector := buzzimport.ChannelInjector{Database: database, CommunityID: communityID}

	newEventIDByOld := map[string]string{}
	reattributed := 0
	for _, oldEvent := range oldEvents {
		newEvent := nostr.Event{
			CreatedAt: oldEvent.CreatedAt,
			Kind:      oldEvent.Kind,
			Tags:      remapEventReferences(oldEvent.Tags, newEventIDByOld),
			Content:   oldEvent.Content,
		}
		if errorValue := newEvent.Sign(newSecretHex); errorValue != nil {
			continue
		}
		newEventIDByOld[oldEvent.ID] = newEvent.ID
		if errorValue := injector.InjectMessage(ctx, channelIDOfEvent(oldEvent), newEvent); errorValue != nil {
			continue
		}
		deleteEvent := nostr.Event{
			CreatedAt: nostr.Now(),
			Kind:      buzzDeleteMessageKind,
			Tags:      nostr.Tags{nostr.Tag{"h", channelIDOfEvent(oldEvent)}, nostr.Tag{"e", oldEvent.ID}},
		}
		if errorValue := deleteEvent.Sign(oldSecretHex); errorValue == nil {
			_ = relay.Publish(ctx, deleteEvent)
		}
		reattributed++
	}
	return reattributed, nil
}

// remapEventReferences points e-tags at the re-signed event ids so the person's
// own reply threads stay connected under the new identity. References to other
// people's events (not in the map) are left untouched.
func remapEventReferences(tags nostr.Tags, newEventIDByOld map[string]string) nostr.Tags {
	remapped := make(nostr.Tags, len(tags))
	for index, tag := range tags {
		copied := append(nostr.Tag{}, tag...)
		if len(copied) >= 2 && copied[0] == "e" {
			if newID, found := newEventIDByOld[copied[1]]; found {
				copied[1] = newID
			}
		}
		remapped[index] = copied
	}
	return remapped
}
