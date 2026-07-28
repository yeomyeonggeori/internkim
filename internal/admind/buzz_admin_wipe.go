package admind

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	nostr "github.com/nbd-wtf/go-nostr"
)

const (
	buzzStreamMessageKind = 9
	buzzDeleteMessageKind = 9005
)

type buzzAdminWipeRequest struct {
	Email string `json:"email"`
}

type buzzAdminWipeResponse struct {
	Email             string `json:"email"`
	BuzzDeleted       int    `json:"buzzDeleted"`
	MattermostDeleted int    `json:"mattermostDeleted"`
}

// handleBuzzAdminWipe deletes every message a person authored, on Buzz and on
// Mattermost, as an explicit, confirmed admin action — for offboarding or a
// deletion request. It does not mint a new identity (the deterministic scheme
// would re-derive the same key); rotating to a fresh key needs per-user key
// versioning, which is a separate change.
func (service *Service) handleBuzzAdminWipe(responseWriter http.ResponseWriter, request *http.Request) {
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
	var payload buzzAdminWipeRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, "invalid request", http.StatusBadRequest)
		return
	}
	email := strings.ToLower(strings.TrimSpace(payload.Email))
	if email == "" {
		http.Error(responseWriter, "email is required", http.StatusBadRequest)
		return
	}

	secretHex, errorValue := service.personBuzzSecret(request.Context(), email)
	if errors.Is(errorValue, errBuzzKeySeedMissing) {
		http.Error(responseWriter, "buzz key seed is not configured", http.StatusNotImplemented)
		return
	}
	if errorValue != nil {
		http.Error(responseWriter, "buzz_wipe_failed", http.StatusInternalServerError)
		return
	}
	buzzDeleted, errorValue := service.wipeBuzzMessages(request.Context(), secretHex)
	if errorValue != nil {
		http.Error(responseWriter, "buzz_wipe_failed", http.StatusBadGateway)
		return
	}
	mattermostDeleted := service.wipeMattermostMessages(request.Context(), email)

	service.writeJSON(responseWriter, buzzAdminWipeResponse{
		Email:             email,
		BuzzDeleted:       buzzDeleted,
		MattermostDeleted: mattermostDeleted,
	})
}

func (service *Service) wipeBuzzMessages(ctx context.Context, secretHex string) (int, error) {
	relayURL := strings.TrimSpace(service.Configuration.BuzzRelayURL)
	if relayURL == "" {
		return 0, errors.New("buzz relay URL is not configured")
	}
	pubkey, errorValue := nostr.GetPublicKey(secretHex)
	if errorValue != nil {
		return 0, errorValue
	}
	relay, errorValue := nostr.RelayConnect(ctx, relayURL)
	if errorValue != nil {
		return 0, errorValue
	}
	defer relay.Close()
	time.Sleep(700 * time.Millisecond)
	_ = relay.Auth(ctx, func(event *nostr.Event) error { return event.Sign(secretHex) })

	events, errorValue := relay.QuerySync(ctx, nostr.Filter{
		Kinds:   []int{buzzStreamMessageKind},
		Authors: []string{pubkey},
	})
	if errorValue != nil {
		return 0, errorValue
	}
	deleted := 0
	for _, event := range events {
		deleteEvent := nostr.Event{
			CreatedAt: nostr.Now(),
			Kind:      buzzDeleteMessageKind,
			Tags:      nostr.Tags{nostr.Tag{"h", channelIDOfEvent(event)}, nostr.Tag{"e", event.ID}},
		}
		if errorValue := deleteEvent.Sign(secretHex); errorValue != nil {
			continue
		}
		if errorValue := relay.Publish(ctx, deleteEvent); errorValue == nil {
			deleted++
		}
	}
	return deleted, nil
}

func channelIDOfEvent(event *nostr.Event) string {
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "h" {
			return tag[1]
		}
	}
	return ""
}

func (service *Service) wipeMattermostMessages(ctx context.Context, email string) int {
	token, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return 0
	}
	userRecord, found, errorValue := service.findMattermostUserByEmail(ctx, token, email)
	if errorValue != nil || !found {
		return 0
	}
	database, errorValue := service.openBridgeMapDatabase(ctx)
	if errorValue != nil {
		return 0
	}
	defer database.Close()
	channels, errorValue := service.listBridgeChannels(ctx, database, "mattermost")
	if errorValue != nil {
		return 0
	}
	deleted := 0
	for _, channel := range channels {
		posts, errorValue := service.fetchMattermostChannelPostsSince(ctx, token, channel.ExternalChannelID, 0)
		if errorValue != nil {
			continue
		}
		for _, post := range posts {
			if post.UserID != userRecord.ID || post.DeleteAt != 0 {
				continue
			}
			if errorValue := service.mattermostRequest(ctx, http.MethodDelete, "/api/v4/posts/"+post.ID, token, nil, nil); errorValue == nil {
				deleted++
			}
		}
	}
	return deleted
}
