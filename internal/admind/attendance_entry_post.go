package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"gitlab.com/eastriver/internkim/internal/mattermostdefaults"
)

func (service *Service) ensureMattermostAttendanceChannel(ctx context.Context, token string, teamID string) (string, error) {
	channel, _ := service.mattermostManagedPublicChannel(attendanceChannelName)
	channelID, errorValue := service.ensureMattermostPublicChannel(ctx, token, teamID, channel.Name, channel.DisplayName)
	if errorValue != nil {
		return "", errorValue
	}
	service.saveMattermostAttendanceChannelID(channelID)
	if errorValue := service.updateMattermostManagedPublicChannelText(ctx, token, channelID, channel); errorValue != nil {
		return "", errorValue
	}
	service.syncMattermostAttendanceEntryPost(ctx, token, channelID)
	return channelID, nil
}

func (service *Service) updateMattermostAttendanceChannelText(ctx context.Context, token string, channelID string) error {
	channel, _ := service.mattermostManagedPublicChannel(attendanceChannelName)
	return service.updateMattermostManagedPublicChannelText(ctx, token, channelID, channel)
}

func (service *Service) syncMattermostAttendanceEntryPost(ctx context.Context, adminToken string, channelID string) {
	if errorValue := service.ensureMattermostAttendanceEntryPost(ctx, adminToken, channelID); errorValue != nil {
		log.Printf("Mattermost Attendance entry post sync failed: %v", errorValue)
	}
}

func (service *Service) ensureMattermostAttendanceEntryPost(ctx context.Context, adminToken string, channelID string) error {
	botToken, errorValue := service.mattermostBotToken()
	if errorValue != nil {
		return errorValue
	}
	botUserID, errorValue := service.mattermostTokenUserID(ctx, botToken)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := service.ensureMattermostBotCanPost(ctx, adminToken, channelID, botUserID); errorValue != nil {
		return errorValue
	}
	expectedProps := service.mattermostAttendanceEntryPostProps()
	postRecord, found := service.mattermostAttendanceEntryPost(ctx, adminToken, channelID)
	if found && strings.TrimSpace(postRecord.UserID) == botUserID {
		service.saveMattermostAttendanceEntryPostID(postRecord.ID)
		if errorValue := service.deleteRecentMattermostAttendanceEntryPostDuplicates(ctx, adminToken, channelID, postRecord.ID); errorValue != nil {
			return errorValue
		}
		if !service.isMattermostAttendanceEntryPostCurrent(postRecord) {
			if errorValue := service.patchMattermostAttendanceEntryPost(ctx, botToken, postRecord.ID, expectedProps); errorValue != nil {
				return errorValue
			}
		}
		if postRecord.IsPinned {
			return nil
		}
		return service.pinMattermostPost(ctx, botToken, postRecord.ID)
	}
	if found && strings.TrimSpace(postRecord.ID) != "" {
		path := "/api/v4/posts/" + url.PathEscape(postRecord.ID)
		if errorValue := service.mattermostRequest(ctx, http.MethodDelete, path, adminToken, nil, nil); errorValue != nil && !isMattermostNotFound(errorValue) {
			return errorValue
		}
	}
	var createdPost mattermostPostRecord
	body := map[string]any{
		"message":    service.adminText().AttendanceEntryMessage,
		"props":      expectedProps,
		"channel_id": channelID,
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/posts", botToken, body, &createdPost); errorValue != nil {
		return errorValue
	}
	service.saveMattermostAttendanceEntryPostID(createdPost.ID)
	return service.pinMattermostPost(ctx, botToken, createdPost.ID)
}

func (service *Service) patchMattermostAttendanceEntryPost(ctx context.Context, botToken string, postID string, props map[string]any) error {
	body := map[string]any{
		"message": service.adminText().AttendanceEntryMessage,
		"props":   props,
	}
	return service.mattermostRequest(ctx, http.MethodPut, "/api/v4/posts/"+url.PathEscape(postID)+"/patch", botToken, body, nil)
}

func (service *Service) mattermostAttendanceEntryPost(ctx context.Context, token string, channelID string) (mattermostPostRecord, bool) {
	if postRecord, found := service.storedMattermostAttendanceEntryPost(ctx, token, channelID); found {
		return postRecord, true
	}
	return service.recentMattermostAttendanceEntryPost(ctx, token, channelID)
}

func (service *Service) storedMattermostAttendanceEntryPost(ctx context.Context, token string, channelID string) (mattermostPostRecord, bool) {
	postID := readTrimmedFile(service.mattermostAttendanceEntryPostIDPath())
	if postID == "" {
		return mattermostPostRecord{}, false
	}
	var postRecord mattermostPostRecord
	path := "/api/v4/posts/" + url.PathEscape(postID)
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, path, token, nil, &postRecord); errorValue != nil {
		return mattermostPostRecord{}, false
	}
	if !isMattermostAttendanceEntryPostRecord(postRecord, channelID) {
		return mattermostPostRecord{}, false
	}
	return postRecord, true
}

func (service *Service) recentMattermostAttendanceEntryPost(ctx context.Context, token string, channelID string) (mattermostPostRecord, bool) {
	posts := service.recentMattermostAttendanceEntryPosts(ctx, token, channelID)
	if len(posts) == 0 {
		return mattermostPostRecord{}, false
	}
	return posts[0], true
}

func (service *Service) recentMattermostAttendanceEntryPosts(ctx context.Context, token string, channelID string) []mattermostPostRecord {
	var response mattermostPostsResponse
	path := "/api/v4/channels/" + url.PathEscape(channelID) + "/posts?per_page=50"
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, path, token, nil, &response); errorValue != nil {
		return nil
	}
	posts := []mattermostPostRecord{}
	for _, postID := range response.Order {
		postRecord := response.Posts[postID]
		if isMattermostAttendanceEntryPostRecord(postRecord, channelID) {
			posts = append(posts, postRecord)
		}
	}
	if len(posts) > 0 {
		return posts
	}
	for _, postRecord := range response.Posts {
		if isMattermostAttendanceEntryPostRecord(postRecord, channelID) {
			posts = append(posts, postRecord)
		}
	}
	return posts
}

func (service *Service) deleteRecentMattermostAttendanceEntryPostDuplicates(ctx context.Context, token string, channelID string, keepPostID string) error {
	for _, postRecord := range service.recentMattermostAttendanceEntryPosts(ctx, token, channelID) {
		if strings.TrimSpace(postRecord.ID) == strings.TrimSpace(keepPostID) {
			continue
		}
		path := "/api/v4/posts/" + url.PathEscape(postRecord.ID)
		if errorValue := service.mattermostRequest(ctx, http.MethodDelete, path, token, nil, nil); errorValue != nil && !isMattermostNotFound(errorValue) {
			return errorValue
		}
	}
	return nil
}

func isMattermostAttendanceEntryPostRecord(postRecord mattermostPostRecord, channelID string) bool {
	if strings.TrimSpace(postRecord.ID) == "" || postRecord.DeleteAt != 0 {
		return false
	}
	if strings.TrimSpace(postRecord.ChannelID) != "" && strings.TrimSpace(postRecord.ChannelID) != strings.TrimSpace(channelID) {
		return false
	}
	if strings.TrimSpace(postRecord.RootID) != "" {
		return false
	}
	return postRecord.Props[attendanceEntryPostProperty] == true
}

func (service *Service) isMattermostAttendanceEntryPostCurrent(postRecord mattermostPostRecord) bool {
	document, errorValue := json.Marshal(postRecord.Props)
	if errorValue != nil {
		return false
	}
	var props struct {
		Attachments []mattermostAttachment `json:"attachments"`
	}
	if errorValue := json.Unmarshal(document, &props); errorValue != nil {
		return false
	}
	if len(props.Attachments) != 1 {
		return false
	}
	attachment := props.Attachments[0]
	text := service.adminText()
	if attachment.Fallback != text.AttendanceEntryMessage || attachment.Text != text.AttendanceEntryText {
		return false
	}
	expectedActions := service.mattermostAttendanceEntryActions()
	if len(attachment.Actions) != len(expectedActions) {
		return false
	}
	actions := attachment.Actions
	for index, action := range actions {
		expectedAction := expectedActions[index]
		if action.ID != expectedAction.ID || action.Name != expectedAction.Name {
			return false
		}
	}
	return true
}

func (service *Service) pinMattermostPost(ctx context.Context, token string, postID string) error {
	if strings.TrimSpace(postID) == "" {
		return fmt.Errorf("Mattermost post ID is empty")
	}
	return service.mattermostRequest(ctx, http.MethodPost, "/api/v4/posts/"+url.PathEscape(postID)+"/pin", token, nil, nil)
}

func (service *Service) mattermostAttendanceEntryPostProps() map[string]any {
	text := service.adminText()
	return map[string]any{
		attendanceEntryPostProperty: true,
		"attachments": []mattermostAttachment{
			{
				Fallback: text.AttendanceEntryMessage,
				Text:     text.AttendanceEntryText,
				Actions:  service.mattermostAttendanceEntryActions(),
			},
		},
	}
}

func (service *Service) mattermostAttendanceEntryActions() []mattermostAction {
	locations, errorValue := service.readAttendanceLocations()
	if errorValue != nil {
		locations = defaultAttendanceLocations()
	}
	text := service.adminText()
	actions := make([]mattermostAction, 0, len(locations)+1)
	if len(locations) == 1 {
		actions = append(actions, service.mattermostAttendanceClockInButton(attendanceClockInAction, text.AttendanceClockIn, locations[0]))
	} else {
		for _, location := range locations {
			actions = append(actions, service.mattermostAttendanceClockInButton(attendanceClockInActionID(location), location.Name, location))
		}
	}
	actions = append(actions, service.mattermostInteractiveButton(attendanceClockOutAction, text.AttendanceClockOut, text.AttendanceClockOutTooltip, "danger"))
	return actions
}

func attendanceClockInActionID(location attendanceLocation) string {
	suffix := sanitizeMattermostActionIDPart(location.ID)
	if suffix == "" {
		return attendanceClockInAction
	}
	return attendanceClockInAction + suffix
}

func sanitizeMattermostActionIDPart(value string) string {
	var builder strings.Builder
	for _, character := range strings.TrimSpace(value) {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' {
			builder.WriteRune(character)
		}
	}
	return builder.String()
}

func (service *Service) mattermostAttendanceClockInButton(actionID string, name string, location attendanceLocation) mattermostAction {
	return service.mattermostInteractiveButtonWithContext(
		actionID,
		name,
		service.adminText().AttendanceClockInTooltip,
		"success",
		mattermostInteractiveContext{Action: attendanceClockInAction, LocationID: location.ID},
	)
}

func (service *Service) mattermostAttendanceLink() string {
	label := mattermostdefaults.PublicChannelLinkLabel(attendanceChannelName, service.workspaceLanguage())
	baseURL := strings.TrimRight(strings.TrimSpace(service.mattermostFlowBaseURL()), "/")
	if baseURL == "" {
		return "[" + label + "](/attendance/)"
	}
	return "[" + label + "](" + baseURL + "/attendance/)"
}
