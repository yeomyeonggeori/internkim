package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"strings"
)

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
		if !reflect.DeepEqual(action, expectedAction) {
			return false
		}
	}
	return true
}
