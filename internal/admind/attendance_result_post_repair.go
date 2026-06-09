package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type attendanceResultPostRepairRequest struct {
	ResultPostID string `json:"resultPostID"`
}

type attendanceResultPostRepairResponse struct {
	Event                attendanceEvent `json:"event"`
	OriginalResultPostID string          `json:"originalResultPostID"`
	ResultPostID         string          `json:"resultPostID"`
	ActionPostID         string          `json:"actionPostID"`
}

func (service *Service) writeAttendanceResultPostRepair(responseWriter http.ResponseWriter, request *http.Request) {
	if !isLocalRequest(request) && !service.isAuthorized(request) {
		http.Error(responseWriter, "admin access required", http.StatusForbidden)
		return
	}
	var body attendanceResultPostRepairRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&body); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	result, errorValue := service.moveAttendanceResultPostToEntryThread(request.Context(), body.ResultPostID)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, result)
}

func (service *Service) moveAttendanceResultPostToEntryThread(ctx context.Context, resultPostID string) (attendanceResultPostRepairResponse, error) {
	trimmedPostID := strings.TrimSpace(resultPostID)
	if trimmedPostID == "" {
		return attendanceResultPostRepairResponse{}, fmt.Errorf("resultPostID is required")
	}
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return attendanceResultPostRepairResponse{}, errorValue
	}
	defer database.Close()
	event, found, errorValue := service.attendanceEventByResultPostID(ctx, database, trimmedPostID)
	if errorValue != nil {
		return attendanceResultPostRepairResponse{}, errorValue
	}
	if !found {
		return attendanceResultPostRepairResponse{}, fmt.Errorf("attendance event for result post %s was not found", trimmedPostID)
	}
	return service.moveAttendanceEventResultPostToEntryThread(ctx, database, event, trimmedPostID)
}

func (service *Service) moveAttendanceEventResultPostToEntryThread(ctx context.Context, database *sql.DB, event attendanceEvent, originalPostID string) (attendanceResultPostRepairResponse, error) {
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return attendanceResultPostRepairResponse{}, errorValue
	}
	postRecord, found, errorValue := service.mattermostPostByID(ctx, adminToken, originalPostID)
	if errorValue != nil {
		return attendanceResultPostRepairResponse{}, errorValue
	}
	if !found {
		return attendanceResultPostRepairResponse{}, fmt.Errorf("Mattermost post %s was not found", originalPostID)
	}
	entryPost, found, errorValue := service.currentMattermostAttendanceEntryPost(ctx, adminToken, event.ChannelID)
	if errorValue != nil {
		return attendanceResultPostRepairResponse{}, errorValue
	}
	if !found {
		return attendanceResultPostRepairResponse{}, fmt.Errorf("Mattermost attendance entry post was not found")
	}
	if strings.TrimSpace(postRecord.RootID) == strings.TrimSpace(entryPost.ID) {
		return service.updateAttendanceEventResultPostReference(ctx, database, event, originalPostID, entryPost.ID, originalPostID)
	}
	if strings.TrimSpace(postRecord.RootID) != "" {
		return attendanceResultPostRepairResponse{}, fmt.Errorf("Mattermost post %s is already in another thread", originalPostID)
	}
	userToken, errorValue := service.ensureMattermostUserAccessToken(ctx, adminToken, event.MattermostUserID)
	if errorValue != nil {
		return attendanceResultPostRepairResponse{}, errorValue
	}
	message := firstNonEmpty(postRecord.Message, service.attendanceMessageForEvent(event, false))
	replacementPostID, errorValue := service.postMattermostUserAttendanceMessage(ctx, userToken, event.ChannelID, entryPost.ID, message)
	if errorValue != nil {
		return attendanceResultPostRepairResponse{}, errorValue
	}
	if errorValue := service.deleteMattermostAttendanceResultPost(ctx, adminToken, originalPostID); errorValue != nil {
		return attendanceResultPostRepairResponse{}, errorValue
	}
	return service.updateAttendanceEventResultPostReference(ctx, database, event, originalPostID, entryPost.ID, replacementPostID)
}

func (service *Service) currentMattermostAttendanceEntryPost(ctx context.Context, adminToken string, channelID string) (mattermostPostRecord, bool, error) {
	if errorValue := service.ensureMattermostAttendanceEntryPost(ctx, adminToken, channelID); errorValue != nil {
		return mattermostPostRecord{}, false, errorValue
	}
	postRecord, found := service.mattermostAttendanceEntryPost(ctx, adminToken, channelID)
	return postRecord, found, nil
}

func (service *Service) updateAttendanceEventResultPostReference(ctx context.Context, database *sql.DB, event attendanceEvent, originalPostID string, actionPostID string, resultPostID string) (attendanceResultPostRepairResponse, error) {
	if errorValue := service.updateAttendanceEventPostIDs(ctx, database, event.ID, actionPostID, resultPostID); errorValue != nil {
		return attendanceResultPostRepairResponse{}, errorValue
	}
	event.ActionPostID = strings.TrimSpace(actionPostID)
	event.ResultPostID = strings.TrimSpace(resultPostID)
	return attendanceResultPostRepairResponse{
		Event:                event,
		OriginalResultPostID: strings.TrimSpace(originalPostID),
		ResultPostID:         strings.TrimSpace(resultPostID),
		ActionPostID:         strings.TrimSpace(actionPostID),
	}, nil
}
