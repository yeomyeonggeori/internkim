package admind

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type attendanceClockRequest struct {
	Kind               string `json:"kind"`
	LocationID         string `json:"locationID"`
	ConfirmEarlyReturn bool   `json:"confirmEarlyReturn"`
}

var errAttendanceDuplicateIgnored = errors.New("attendance duplicate ignored")

func (service *Service) handleAttendanceToggleAction(responseWriter http.ResponseWriter, request *http.Request, payload mattermostInteractivePayload) {
	_, errorValue := service.toggleAttendanceFromMattermost(request.Context(), payload)
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
	now := time.Now().UTC()
	activeLeave, errorValue := service.prepareAttendanceLeaveClock(ctx, actorEmail, body, now)
	if errorValue != nil {
		if errors.Is(errorValue, errAttendanceLeaveEarlyReturnConfirmationRequired) ||
			errors.Is(errorValue, errAttendanceLeaveClockOutAlreadyApplied) {
			http.Error(responseWriter, errorValue.Error(), http.StatusConflict)
			return
		}
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
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
	channelID, errorValue := service.mattermostAttendanceActionChannelID(ctx, adminToken, teamRecord.ID, "")
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if errorValue := service.ensureMattermostAttendanceEntryPost(ctx, adminToken, channelID); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	actionPostID := readTrimmedFile(service.mattermostAttendanceEntryPostIDPath())
	if actionPostID == "" {
		http.Error(responseWriter, "attendance entry post id required", http.StatusInternalServerError)
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
	result := attendanceActionResult{}
	if kind == "" {
		result, errorValue = service.applyAttendanceToggle(ctx, userRecord, userToken, teamRecord.ID, channelID, actionPostID)
	} else {
		result, errorValue = service.applyAttendanceAction(ctx, userRecord, userToken, kind, teamRecord.ID, channelID, actionPostID, locationID)
	}
	if errorValue != nil && !errors.Is(errorValue, errAttendanceDuplicateIgnored) {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if errorValue := service.completeAttendanceLeaveClock(
		ctx,
		activeLeave,
		actorEmail,
		body,
		now,
	); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, map[string]any{"ok": true, "status": result.Status, "resultPostID": result.ResultPostID})
}

func (service *Service) handleAttendanceClockAction(responseWriter http.ResponseWriter, request *http.Request, payload mattermostInteractivePayload, kind string) {
	_, errorValue := service.recordAttendanceFromMattermost(request.Context(), payload, kind)
	if errorValue == nil {
		service.writeMattermostInteractiveSuccess(responseWriter)
		return
	}
	service.writeMattermostInteractiveError(responseWriter, errorValue.Error())
}

func (service *Service) toggleAttendanceFromMattermost(ctx context.Context, payload mattermostInteractivePayload) (attendanceActionResult, error) {
	return service.recordAttendanceFromMattermost(ctx, payload, "")
}

func (service *Service) recordAttendanceFromMattermost(ctx context.Context, payload mattermostInteractivePayload, requestedKind string) (attendanceActionResult, error) {
	return service.recordAttendanceFromMattermostAt(ctx, payload, requestedKind, time.Now().UTC())
}

func (service *Service) recordAttendanceFromMattermostAt(
	ctx context.Context,
	payload mattermostInteractivePayload,
	requestedKind string,
	now time.Time,
) (attendanceActionResult, error) {
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	userRecord, found, errorValue := service.findMattermostUserByID(ctx, adminToken, payload.UserID)
	if errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	if !found {
		return attendanceActionResult{}, fmt.Errorf("Mattermost user %s was not found", payload.UserID)
	}
	leaveClockRequest := attendanceClockRequest{Kind: requestedKind}
	activeLeave, errorValue := service.reconcileApprovedLeaveClockOut(
		ctx,
		userRecord.Email,
		now,
	)
	if errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	if activeLeave != nil {
		if requestedKind == attendanceKindClockOut {
			return attendanceActionResult{}, errAttendanceLeaveClockOutAlreadyApplied
		}
		leaveClockRequest.Kind = attendanceKindClockIn
		leaveClockRequest.ConfirmEarlyReturn = true
	}
	teamRecord, errorValue := service.ensureMattermostTeam(ctx, adminToken)
	if errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	channelID, errorValue := service.mattermostAttendanceActionChannelID(ctx, adminToken, teamRecord.ID, payload.ChannelID)
	if errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	if errorValue := service.ensureMattermostChannelMembership(ctx, adminToken, channelID, userRecord.ID); errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	userToken, errorValue := service.ensureMattermostUserAccessToken(ctx, adminToken, userRecord.ID)
	if errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	teamID := firstNonEmpty(payload.TeamID, teamRecord.ID)
	actionPostID := firstNonEmpty(readTrimmedFile(service.mattermostAttendanceEntryPostIDPath()), payload.PostID)
	result := attendanceActionResult{}
	if requestedKind == "" {
		result, errorValue = service.applyAttendanceToggle(ctx, userRecord, userToken, teamID, channelID, actionPostID)
	} else {
		result, errorValue = service.applyAttendanceAction(ctx, userRecord, userToken, requestedKind, teamID, channelID, actionPostID, payload.Context.LocationID)
	}
	if errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	if errorValue := service.completeAttendanceLeaveClock(
		ctx,
		activeLeave,
		userRecord.Email,
		leaveClockRequest,
		now,
	); errorValue != nil {
		return attendanceActionResult{}, errorValue
	}
	return result, nil
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
