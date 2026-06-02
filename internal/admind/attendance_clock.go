package admind

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type attendanceClockRequest struct {
	Kind       string `json:"kind"`
	LocationID string `json:"locationID"`
}

var errAttendanceDuplicateIgnored = errors.New("attendance duplicate ignored")

func (service *Service) handleAttendanceToggleAction(responseWriter http.ResponseWriter, request *http.Request, payload mattermostInteractivePayload) {
	errorValue := service.toggleAttendanceFromMattermost(request.Context(), payload)
	if errorValue == nil || errors.Is(errorValue, errAttendanceDuplicateIgnored) {
		service.writeMattermostInteractiveSuccess(responseWriter)
		return
	}
	service.writeMattermostInteractiveError(responseWriter, errorValue.Error())
}

func (service *Service) writeAttendanceClock(responseWriter http.ResponseWriter, request *http.Request) {
	actorEmail := strings.ToLower(strings.TrimSpace(service.webStaffActorEmail(request)))
	if actorEmail == "" {
		http.Error(responseWriter, "actor email required", http.StatusForbidden)
		return
	}
	var body attendanceClockRequest
	if request.ContentLength > 0 {
		if errorValue := json.NewDecoder(request.Body).Decode(&body); errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
			return
		}
	}
	kind := strings.TrimSpace(body.Kind)
	if kind != "" && kind != attendanceKindClockIn && kind != attendanceKindClockOut {
		http.Error(responseWriter, "invalid kind", http.StatusBadRequest)
		return
	}
	ctx := request.Context()
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	userRecord, found, errorValue := service.findMattermostUserByEmail(ctx, adminToken, actorEmail)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(responseWriter, "mattermost user not found", http.StatusNotFound)
		return
	}
	teamRecord, errorValue := service.ensureMattermostTeam(ctx, adminToken)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	channelID, errorValue := service.ensureMattermostAttendanceChannel(ctx, adminToken, teamRecord.ID)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if errorValue := service.ensureMattermostChannelMembership(ctx, adminToken, channelID, userRecord.ID); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	userToken, errorValue := service.ensureMattermostUserAccessToken(ctx, adminToken, userRecord.ID)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	locationID := strings.TrimSpace(body.LocationID)
	if kind == "" {
		errorValue = service.applyAttendanceToggle(ctx, userRecord, userToken, teamRecord.ID, channelID, "")
	} else {
		errorValue = service.applyAttendanceAction(ctx, userRecord, userToken, kind, teamRecord.ID, channelID, "", locationID)
	}
	if errorValue != nil && !errors.Is(errorValue, errAttendanceDuplicateIgnored) {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, map[string]any{"ok": true})
}

func (service *Service) handleAttendanceClockAction(responseWriter http.ResponseWriter, request *http.Request, payload mattermostInteractivePayload, kind string) {
	errorValue := service.recordAttendanceFromMattermost(request.Context(), payload, kind)
	if errorValue == nil {
		service.writeMattermostInteractiveSuccess(responseWriter)
		return
	}
	service.writeMattermostInteractiveError(responseWriter, errorValue.Error())
}

func (service *Service) toggleAttendanceFromMattermost(ctx context.Context, payload mattermostInteractivePayload) error {
	return service.recordAttendanceFromMattermost(ctx, payload, "")
}

func (service *Service) recordAttendanceFromMattermost(ctx context.Context, payload mattermostInteractivePayload, requestedKind string) error {
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return errorValue
	}
	userRecord, found, errorValue := service.findMattermostUserByID(ctx, adminToken, payload.UserID)
	if errorValue != nil {
		return errorValue
	}
	if !found {
		return fmt.Errorf("Mattermost user %s was not found", payload.UserID)
	}
	teamRecord, errorValue := service.ensureMattermostTeam(ctx, adminToken)
	if errorValue != nil {
		return errorValue
	}
	channelID, errorValue := service.mattermostAttendanceActionChannelID(ctx, adminToken, teamRecord.ID, payload.ChannelID)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := service.ensureMattermostChannelMembership(ctx, adminToken, channelID, userRecord.ID); errorValue != nil {
		return errorValue
	}
	userToken, errorValue := service.ensureMattermostUserAccessToken(ctx, adminToken, userRecord.ID)
	if errorValue != nil {
		return errorValue
	}
	teamID := firstNonEmpty(payload.TeamID, teamRecord.ID)
	actionPostID := firstNonEmpty(readTrimmedFile(service.mattermostAttendanceEntryPostIDPath()), payload.PostID)
	if requestedKind == "" {
		return service.applyAttendanceToggle(ctx, userRecord, userToken, teamID, channelID, actionPostID)
	}
	return service.applyAttendanceAction(ctx, userRecord, userToken, requestedKind, teamID, channelID, actionPostID, payload.Context.LocationID)
}

func (service *Service) mattermostAttendanceActionChannelID(ctx context.Context, token string, teamID string, payloadChannelID string) (string, error) {
	if channelID := strings.TrimSpace(payloadChannelID); channelID != "" {
		service.saveMattermostAttendanceChannelID(channelID)
		return channelID, nil
	}
	if channelID := readTrimmedFile(service.mattermostAttendanceChannelIDPath()); channelID != "" {
		return channelID, nil
	}
	channelID, errorValue := service.mattermostChannelIDByName(ctx, token, teamID, attendanceChannelName)
	if errorValue != nil {
		return "", errorValue
	}
	service.saveMattermostAttendanceChannelID(channelID)
	return channelID, nil
}
