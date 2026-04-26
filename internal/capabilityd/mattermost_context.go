package capabilityd

import (
	"context"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
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
	speakerByUserID := map[string]string{}
	for _, post := range previousPosts {
		if strings.TrimSpace(post.Message) == "" || strings.TrimSpace(post.Type) != "" {
			continue
		}
		speaker := speakerByUserID[post.UserID]
		if speaker == "" {
			speaker = service.mattermostSpeaker(ctx, post.UserID)
			speakerByUserID[post.UserID] = speaker
		}
		messages = append(messages, platformContextMessage{Speaker: speaker, Text: post.Message})
	}

	contextValue := platformEventContext{Messages: messages, HasMoreBefore: hasMoreBefore}
	if hasMoreBefore {
		contextValue.HistoryCursor = mustEncodePlatformHandle(handle)
	}
	return contextValue
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
	if strings.TrimSpace(userID) == "" {
		return "unknown"
	}
	var response struct {
		Username  string `json:"username"`
		Nickname  string `json:"nickname"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/"+url.PathEscape(userID), nil, &response)
	if errorValue != nil {
		return userID
	}
	return firstNonEmpty(strings.TrimSpace(response.FirstName+" "+response.LastName), response.Nickname, response.Username, userID)
}

func mustEncodePlatformHandle(handle platformHandle) string {
	value, errorValue := encodePlatformHandle(handle)
	if errorValue != nil {
		return ""
	}
	return value
}
