package admind

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

func (service *Service) ensureMattermostChannelMembership(ctx context.Context, token string, channelID string, userID string) error {
	body := map[string]string{"user_id": userID}
	errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/channels/"+url.PathEscape(channelID)+"/members", token, body, nil)
	if errorValue != nil && !isMattermostBadRequest(errorValue) {
		return errorValue
	}
	return nil
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
