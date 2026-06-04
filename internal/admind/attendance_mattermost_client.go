package admind

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (service *Service) ensureMattermostChannelMembership(ctx context.Context, token string, channelID string, userID string) error {
	body := map[string]string{"user_id": userID}
	errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/channels/"+url.PathEscape(channelID)+"/members", token, body, nil)
	if errorValue == nil {
		return nil
	}
	if !isMattermostBadRequest(errorValue) {
		return errorValue
	}
	return service.ensureMattermostChannelMembershipExists(ctx, token, channelID, userID, errorValue)
}

func (service *Service) ensureMattermostChannelMembershipExists(ctx context.Context, token string, channelID string, userID string, cause error) error {
	path := "/api/v4/channels/" + url.PathEscape(channelID) + "/members/" + url.PathEscape(userID)
	var member mattermostChannelMemberRecord
	errorValue := service.mattermostRequest(ctx, http.MethodGet, path, token, nil, &member)
	if errorValue == nil && strings.TrimSpace(member.UserID) == strings.TrimSpace(userID) {
		return nil
	}
	if errorValue != nil {
		return fmt.Errorf("Mattermost channel membership was not confirmed after join failed: %w", cause)
	}
	return fmt.Errorf("Mattermost channel membership returned unexpected user %q after join failed: %w", member.UserID, cause)
}

func (service *Service) postMattermostUserAttendanceMessage(ctx context.Context, userToken string, channelID string, rootID string, message string) (string, error) {
	body := map[string]string{
		"channel_id": channelID,
		"message":    message,
	}
	if strings.TrimSpace(rootID) != "" {
		body["root_id"] = strings.TrimSpace(rootID)
	}
	var response struct {
		ID string `json:"id"`
	}
	errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/posts", userToken, body, &response)
	return strings.TrimSpace(response.ID), errorValue
}

func (service *Service) patchMattermostAttendanceResultPost(ctx context.Context, userToken string, postID string, message string) error {
	if strings.TrimSpace(postID) == "" {
		return fmt.Errorf("Mattermost attendance result post ID is empty")
	}
	body := map[string]string{"message": message}
	return service.mattermostRequest(ctx, http.MethodPut, "/api/v4/posts/"+url.PathEscape(postID)+"/patch", userToken, body, nil)
}

func (service *Service) deleteMattermostAttendanceResultPost(ctx context.Context, adminToken string, postID string) error {
	trimmedPostID := strings.TrimSpace(postID)
	if trimmedPostID == "" {
		return nil
	}
	errorValue := service.mattermostRequest(ctx, http.MethodDelete, "/api/v4/posts/"+url.PathEscape(trimmedPostID), adminToken, nil, nil)
	if errorValue != nil && !isMattermostNotFound(errorValue) {
		return errorValue
	}
	return nil
}
