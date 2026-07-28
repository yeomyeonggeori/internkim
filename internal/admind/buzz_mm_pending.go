package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type buzzMirrorPost struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	UserID    string `json:"user_id"`
	Message   string `json:"message"`
	RootID    string `json:"root_id"`
	Type      string `json:"type"`
	CreateAt  int64  `json:"create_at"`
	UpdateAt  int64  `json:"update_at"`
	DeleteAt  int64  `json:"delete_at"`
}

type buzzMirrorPostsPage struct {
	Order []string                        `json:"order"`
	Posts map[string]buzzMirrorPost `json:"posts"`
}

type buzzMMPendingItem struct {
	ExternalID        string `json:"externalId"`
	ExternalChannelID string `json:"externalChannelId"`
	BuzzChannelID     string `json:"buzzChannelId"`
	Text              string `json:"text"`
	UpdatedAt         int64  `json:"updatedAt"`
}

type buzzMMPendingResponse struct {
	Items  []buzzMMPendingItem `json:"items"`
	Cursor int64               `json:"cursor"`
}

// handleBuzzMMPending returns the caller's Mattermost posts, in mirrored
// channels, that are newer than the cursor and not yet mirrored to Buzz. The
// browser signs each with the person's own key and publishes it to Buzz, then
// records the mapping — so MM-authored messages reach Buzz client-signed, never
// signed server-side on their behalf.
func (service *Service) handleBuzzMMPending(responseWriter http.ResponseWriter, request *http.Request) {
	actorEmail := service.webActorEmail(request)
	if actorEmail == "" {
		http.Error(responseWriter, "unauthorized", http.StatusUnauthorized)
		return
	}
	since, _ := strconv.ParseInt(request.URL.Query().Get("since"), 10, 64)
	token, errorValue := service.mattermostAdminToken(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, "mattermost_unavailable", http.StatusBadGateway)
		return
	}
	userRecord, found, errorValue := service.findMattermostUserByEmail(request.Context(), token, actorEmail)
	if errorValue != nil || !found {
		service.writeJSON(responseWriter, buzzMMPendingResponse{Items: []buzzMMPendingItem{}, Cursor: since})
		return
	}
	database, errorValue := service.openBridgeMapDatabase(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, "bridge_map_unavailable", http.StatusInternalServerError)
		return
	}
	defer database.Close()
	channels, errorValue := service.listBridgeChannels(request.Context(), database, "mattermost")
	if errorValue != nil {
		http.Error(responseWriter, "bridge_map_read_failed", http.StatusInternalServerError)
		return
	}

	items := []buzzMMPendingItem{}
	cursor := since
	for _, channel := range channels {
		posts, errorValue := service.fetchMattermostChannelPostsSince(request.Context(), token, channel.ExternalChannelID, since)
		if errorValue != nil {
			continue
		}
		for _, post := range posts {
			if post.UserID != userRecord.ID || post.DeleteAt != 0 || post.UpdateAt <= since || strings.HasPrefix(post.Type, "system_") {
				continue
			}
			if _, mapped, _ := service.bridgeMessageByExternal(request.Context(), database, "mattermost", post.ID); mapped {
				continue
			}
			items = append(items, buzzMMPendingItem{
				ExternalID:        post.ID,
				ExternalChannelID: channel.ExternalChannelID,
				BuzzChannelID:     channel.BuzzChannelID,
				Text:              post.Message,
				UpdatedAt:         post.UpdateAt,
			})
			if post.UpdateAt > cursor {
				cursor = post.UpdateAt
			}
		}
	}
	service.writeJSON(responseWriter, buzzMMPendingResponse{Items: items, Cursor: cursor})
}

func (service *Service) fetchMattermostChannelPostsSince(ctx context.Context, token string, channelID string, since int64) ([]buzzMirrorPost, error) {
	path := "/api/v4/channels/" + url.PathEscape(channelID) + "/posts?since=" + strconv.FormatInt(since, 10)
	var page buzzMirrorPostsPage
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, path, token, nil, &page); errorValue != nil {
		return nil, errorValue
	}
	posts := make([]buzzMirrorPost, 0, len(page.Order))
	for _, id := range page.Order {
		if post, ok := page.Posts[id]; ok {
			posts = append(posts, post)
		}
	}
	return posts, nil
}

type buzzMMMirroredRequest struct {
	ExternalID        string `json:"externalId"`
	ExternalChannelID string `json:"externalChannelId"`
	BuzzEventID       string `json:"buzzEventId"`
}

// handleBuzzMMMirrored records that a Mattermost post was mirrored to a Buzz
// event, so it is not mirrored again and the Buzz->MM fan-out never echoes it
// back to Mattermost.
func (service *Service) handleBuzzMMMirrored(responseWriter http.ResponseWriter, request *http.Request) {
	if service.webActorEmail(request) == "" {
		http.Error(responseWriter, "unauthorized", http.StatusUnauthorized)
		return
	}
	var payload buzzMMMirroredRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, "invalid request", http.StatusBadRequest)
		return
	}
	if payload.ExternalID == "" || payload.BuzzEventID == "" {
		http.Error(responseWriter, "externalId and buzzEventId are required", http.StatusBadRequest)
		return
	}
	database, errorValue := service.openBridgeMapDatabase(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, "bridge_map_unavailable", http.StatusInternalServerError)
		return
	}
	defer database.Close()
	mapping := bridgeMessageMapping{
		BuzzEventID:       payload.BuzzEventID,
		Platform:          "mattermost",
		ExternalID:        payload.ExternalID,
		ExternalChannelID: payload.ExternalChannelID,
	}
	if errorValue := service.recordBridgeMessage(request.Context(), database, mapping); errorValue != nil {
		http.Error(responseWriter, "bridge_map_write_failed", http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, map[string]bool{"recorded": true})
}
