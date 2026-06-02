package admind

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
)

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

func (service *Service) pinMattermostPost(ctx context.Context, token string, postID string) error {
	if strings.TrimSpace(postID) == "" {
		return fmt.Errorf("Mattermost post ID is empty")
	}
	return service.mattermostRequest(ctx, http.MethodPost, "/api/v4/posts/"+url.PathEscape(postID)+"/pin", token, nil, nil)
}
