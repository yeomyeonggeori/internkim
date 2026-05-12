package capabilityd

import (
	"context"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/identity"
)

type mattermostHistoryPost struct {
	ID       string `json:"id"`
	UserID   string `json:"user_id"`
	Message  string `json:"message"`
	Type     string `json:"type"`
	CreateAt int64  `json:"create_at"`
}

func (service Service) enrichMattermostEvent(ctx context.Context, event platformInboundEvent) platformInboundEvent {
	handle, errorValue := decodePlatformHandle(event.Context.HistoryCursor)
	if errorValue != nil {
		return event
	}
	event.Context = service.mattermostContext(ctx, handle, 20)
	event.Context.Sender = service.mattermostSender(ctx, event.SenderID)
	event.Context.ReceivedAt = time.Now().UTC().Format(time.RFC3339)
	return event
}

func (service Service) mattermostContext(ctx context.Context, handle platformHandle, limit int) platformEventContext {
	limit = normalizedHistoryLimit(limit)
	posts, errorValue := service.mattermostHistoryPosts(ctx, handle, limit+1)
	if errorValue != nil {
		return platformEventContext{HistoryCursor: mustEncodePlatformHandle(handle)}
	}

	previousPosts := postsBeforeMessage(posts, handle.MessageID)
	hasMoreBefore := len(previousPosts) > limit
	if hasMoreBefore {
		previousPosts = previousPosts[len(previousPosts)-limit:]
	}

	messages := make([]platformContextMessage, 0, len(previousPosts))
	senderByUserID := map[string]platformContextSender{}
	for _, post := range previousPosts {
		if strings.TrimSpace(post.Message) == "" || strings.TrimSpace(post.Type) != "" {
			continue
		}
		senderInfo, hasSender := senderByUserID[post.UserID]
		if !hasSender {
			senderInfo = service.mattermostSender(ctx, post.UserID)
			senderByUserID[post.UserID] = senderInfo
		}
		messages = append(messages, platformContextMessage{
			Speaker:            senderInfo.Name,
			SpeakerCallingName: senderInfo.CallingName,
			SpeakerHandle:      senderInfo.Handle,
			Text:               post.Message,
		})
	}

	return platformEventContext{
		Messages:      messages,
		HasMoreBefore: hasMoreBefore,
		HistoryCursor: mustEncodePlatformHandle(handle),
	}
}

func (service Service) mattermostHistoryPosts(ctx context.Context, handle platformHandle, limit int) ([]mattermostHistoryPost, error) {
	var response struct {
		Order []string                         `json:"order"`
		Posts map[string]mattermostHistoryPost `json:"posts"`
	}
	path := "/api/v4/channels/" + url.PathEscape(handle.ChannelID) + "/posts?per_page=" + strconv.Itoa(normalizedHistoryLimit(limit))
	if strings.TrimSpace(handle.RootID) != "" {
		path = "/api/v4/posts/" + url.PathEscape(handle.RootID) + "/thread"
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, path, nil, &response); errorValue != nil {
		return nil, errorValue
	}

	posts := make([]mattermostHistoryPost, 0, len(response.Posts))
	for _, post := range response.Posts {
		posts = append(posts, post)
	}
	sort.SliceStable(posts, func(leftIndex int, rightIndex int) bool {
		return posts[leftIndex].CreateAt < posts[rightIndex].CreateAt
	})
	return posts, nil
}

func postsBeforeMessage(posts []mattermostHistoryPost, messageID string) []mattermostHistoryPost {
	for index, post := range posts {
		if post.ID == messageID {
			return posts[:index]
		}
	}
	filteredPosts := make([]mattermostHistoryPost, 0, len(posts))
	for _, post := range posts {
		if post.ID != messageID {
			filteredPosts = append(filteredPosts, post)
		}
	}
	return filteredPosts
}

func (service Service) mattermostSpeaker(ctx context.Context, userID string) string {
	return service.mattermostSender(ctx, userID).Name
}

func (service Service) mattermostSender(ctx context.Context, userID string) platformContextSender {
	if strings.TrimSpace(userID) == "" {
		return platformContextSender{Platform: "mattermost", Name: "unknown"}
	}
	var response struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		Username  string `json:"username"`
		Nickname  string `json:"nickname"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/"+url.PathEscape(userID), nil, &response)
	if errorValue != nil {
		return platformContextSender{
			Platform: "mattermost",
			SenderID: userID,
			UserID:   userID,
			Name:     userID,
		}
	}
	senderID := firstNonEmpty(response.ID, userID)
	canonicalName := firstNonEmpty(
		strings.TrimSpace(response.Nickname),
		strings.TrimSpace(response.FirstName),
		response.Username,
		senderID,
	)
	return platformContextSender{
		Platform:    "mattermost",
		SenderID:    senderID,
		UserID:      senderID,
		Handle:      response.Username,
		Email:       response.Email,
		Name:        canonicalName,
		CallingName: identity.CallingName(canonicalName),
	}
}

func mustEncodePlatformHandle(handle platformHandle) string {
	value, errorValue := encodePlatformHandle(handle)
	if errorValue != nil {
		return ""
	}
	return value
}
