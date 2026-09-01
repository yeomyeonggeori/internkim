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
