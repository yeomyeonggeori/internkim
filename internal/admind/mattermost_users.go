package admind

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"gitlab.com/eastriver/internkim/internal/buzzimport/mattermostadmin"
	"gitlab.com/eastriver/internkim/internal/mattermostdefaults"
)

type adminCircleRecord struct {
	CircleID            string `json:"circleID"`
	DisplayName         string `json:"displayName"`
	IsMattermostManaged bool   `json:"isMattermostManaged,omitempty"`
}

type mattermostPostRecord struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id"`
	ChannelID string         `json:"channel_id"`
	RootID    string         `json:"root_id"`
	Message   string         `json:"message"`
	Type      string         `json:"type"`
	CreateAt  int64          `json:"create_at"`
	DeleteAt  int64          `json:"delete_at"`
	IsPinned  bool           `json:"is_pinned"`
	Props     map[string]any `json:"props"`
}

type mattermostPostsResponse struct {
	Order []string                        `json:"order"`
	Posts map[string]mattermostPostRecord `json:"posts"`
}

type mattermostPreferenceRecord struct {
	UserID   string `json:"user_id"`
	Category string `json:"category"`
	Name     string `json:"name"`
	Value    string `json:"value"`
}

func (service *Service) ensureMattermostPrivateChannel(ctx context.Context, token string, teamID string, channelName string) (string, error) {
	channelID, errorValue := service.mattermostChannelIDByName(ctx, token, teamID, channelName)
	if errorValue == nil || channelID != "" {
		if channelID != "" {
			if updateError := service.updateMattermostPrivateChannelDisplayName(ctx, token, channelID, channelName); updateError != nil {
				return "", updateError
			}
		}
		return channelID, errorValue
	}
	if !mattermostadmin.IsNotFound(errorValue) {
		return "", errorValue
	}
	body := map[string]string{
		"team_id":      teamID,
		"name":         channelName,
		"display_name": mattermostdefaults.CircleChannelDisplayName(channelName),
		"type":         "P",
	}
	var channelRecord mattermostadmin.ChannelRecord
	if errorValue := service.mattermostAdmin().Request(ctx, http.MethodPost, "/api/v4/channels", token, body, &channelRecord); errorValue != nil {
		return "", errorValue
	}
	return channelRecord.ID, nil
}

func (service *Service) updateMattermostPrivateChannelDisplayName(ctx context.Context, token string, channelID string, channelName string) error {
	body := map[string]string{"display_name": mattermostdefaults.CircleChannelDisplayName(channelName)}
	if errorValue := service.mattermostAdmin().Request(ctx, http.MethodPut, "/api/v4/channels/"+url.PathEscape(channelID)+"/patch", token, body, nil); errorValue != nil {
		return errorValue
	}
	return service.cleanupMattermostManagedChannelSystemPosts(ctx, token, channelID)
}

func (service *Service) cleanupMattermostManagedChannelSystemPosts(ctx context.Context, token string, channelID string) error {
	for _, postRecord := range service.mattermostTaskPosts(ctx, token, channelID, 100) {
		if !isMattermostManagedChannelSystemPost(postRecord) || strings.TrimSpace(postRecord.ID) == "" {
			continue
		}
		if errorValue := service.mattermostAdmin().Request(ctx, http.MethodDelete, "/api/v4/posts/"+url.PathEscape(postRecord.ID), token, nil, nil); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func (service *Service) mattermostTaskPosts(ctx context.Context, token string, channelID string, limit int) []mattermostPostRecord {
	var response mattermostPostsResponse
	path := "/api/v4/channels/" + url.PathEscape(channelID) + "/posts?per_page=" + strconv.Itoa(limit)
	if errorValue := service.mattermostAdmin().Request(ctx, http.MethodGet, path, token, nil, &response); errorValue != nil {
		return nil
	}
	posts := make([]mattermostPostRecord, 0, len(response.Posts))
	for _, postID := range response.Order {
		if postRecord, found := response.Posts[postID]; found {
			posts = append(posts, postRecord)
		}
	}
	if len(posts) > 0 {
		return posts
	}
	for _, postRecord := range response.Posts {
		posts = append(posts, postRecord)
	}
	return posts
}

func isMattermostManagedChannelSystemPost(post mattermostPostRecord) bool {
	switch strings.TrimSpace(post.Type) {
	case "system_add_to_channel", "system_displayname_change", "system_header_change", "system_join_channel", "system_purpose_change":
		return true
	default:
		return false
	}
}

func (service *Service) syncMattermostUserCircleMemberships(ctx context.Context, record adminUserMutation) error {
	token, errorValue := service.mattermostAdmin().AdminToken(ctx)
	if errorValue != nil {
		return errorValue
	}
	teamRecord, errorValue := service.mattermostAdmin().EnsureTeam(ctx, token)
	if errorValue != nil {
		return errorValue
	}
	userID := strings.TrimSpace(record.MattermostUserID)
	if userID == "" {
		userRecord, found, errorValue := service.mattermostAdmin().FindUserByEmail(ctx, token, record.Email)
		if errorValue != nil {
			return errorValue
		}
		if !found {
			return nil
		}
		userID = userRecord.ID
	}
	selectedCircles := map[string]bool{}
	for _, circleID := range normalizeAdminUserCircles(record.Circles, record.Role) {
		selectedCircles[circleID] = true
	}
	circleChannels, errorValue := service.mattermostCircleChannelDefinitions(ctx)
	if errorValue != nil {
		return errorValue
	}
	for _, circleChannel := range circleChannels {
		channelID, errorValue := service.ensureMattermostPrivateChannel(ctx, token, teamRecord.ID, circleChannel.ChannelName)
		if errorValue != nil {
			return errorValue
		}
		if selectedCircles[circleChannel.CircleID] {
			if errorValue := service.ensureMattermostChannelMember(ctx, token, channelID, userID); errorValue != nil {
				return errorValue
			}
			continue
		}
		if errorValue := service.removeMattermostChannelMember(ctx, token, channelID, userID); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func (service *Service) removeMattermostChannelMember(ctx context.Context, token string, channelID string, userID string) error {
	if strings.TrimSpace(channelID) == "" || strings.TrimSpace(userID) == "" {
		return nil
	}
	path := "/api/v4/channels/" + url.PathEscape(channelID) + "/members/" + url.PathEscape(userID)
	if errorValue := service.mattermostAdmin().Request(ctx, http.MethodDelete, path, token, nil, nil); errorValue != nil && !mattermostadmin.IsNotFound(errorValue) {
		return errorValue
	}
	return nil
}

type mattermostCircleChannelDefinition struct {
	CircleID    string
	ChannelName string
}

func (service *Service) mattermostCircleChannelDefinitions(ctx context.Context) ([]mattermostCircleChannelDefinition, error) {
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		return nil, errorValue
	}
	circleChannels := mattermostCircleChannelDefinitionsFromPolicy(policyDocument)
	if len(circleChannels) == 0 {
		return defaultMattermostCircleChannelDefinitions(), nil
	}
	return circleChannels, nil
}

func mattermostCircleChannelDefinitionsFromPolicy(policyDocument map[string]any) []mattermostCircleChannelDefinition {
	circleSync, _ := policyDocument["circleSync"].(map[string]any)
	channelValues, _ := circleSync["mattermostPrivateChannels"].([]any)
	circleChannels := []mattermostCircleChannelDefinition{}
	for _, value := range channelValues {
		channel, isChannel := value.(map[string]any)
		if !isChannel {
			continue
		}
		circleChannel := mattermostCircleChannelDefinition{
			CircleID:    strings.ToLower(strings.TrimSpace(policyString(channel["circleID"]))),
			ChannelName: strings.ToLower(strings.TrimSpace(policyString(channel["channelName"]))),
		}
		if circleChannel.CircleID != "" && circleChannel.ChannelName != "" {
			circleChannels = append(circleChannels, circleChannel)
		}
	}
	return circleChannels
}

func policyString(value any) string {
	stringValue, _ := value.(string)
	return stringValue
}

func defaultMattermostCircleChannelDefinitions() []mattermostCircleChannelDefinition {
	return []mattermostCircleChannelDefinition{
		{CircleID: "c-level", ChannelName: "circle-c-level"},
		{CircleID: "representative", ChannelName: "circle-representative"},
		{CircleID: "admin", ChannelName: "circle-admin"},
		{CircleID: "hr", ChannelName: "circle-hr"},
	}
}

func (service *Service) mattermostChannelIDByName(ctx context.Context, token string, teamID string, channelName string) (string, error) {
	var channelRecord mattermostadmin.ChannelRecord
	path := "/api/v4/teams/" + url.PathEscape(teamID) + "/channels/name/" + url.PathEscape(channelName)
	errorValue := service.mattermostAdmin().Request(ctx, http.MethodGet, path, token, nil, &channelRecord)
	return channelRecord.ID, errorValue
}

func (service *Service) ensureMattermostChannelMember(ctx context.Context, token string, channelID string, userID string) error {
	if strings.TrimSpace(channelID) == "" || strings.TrimSpace(userID) == "" {
		return nil
	}
	channelMember := map[string]string{"user_id": strings.TrimSpace(userID)}
	path := "/api/v4/channels/" + url.PathEscape(channelID) + "/members"
	if errorValue := service.mattermostAdmin().Request(ctx, http.MethodPost, path, token, channelMember, nil); errorValue != nil && !mattermostadmin.IsBadRequest(errorValue) {
		return errorValue
	}
	return nil
}
