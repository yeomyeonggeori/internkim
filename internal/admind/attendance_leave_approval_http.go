package admind

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func (service *Service) writeAttendanceLeaveApprovalInbox(
	responseWriter http.ResponseWriter,
	request *http.Request,
) {
	if !service.canManageAttendance(request) {
		writeAttendanceLeaveRequestError(responseWriter, errAttendanceLeaveAccessDenied)
		return
	}
	inbox, errorValue := service.readAttendanceLeaveApprovalInbox(request.Context())
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	responseWriter.Header().Set("Cache-Control", "private, no-store")
	service.writeJSON(responseWriter, inbox)
}

func (service *Service) writeAttendanceLeaveApprovalDecision(
	responseWriter http.ResponseWriter,
	request *http.Request,
	requestID string,
) {
	if !service.canManageAttendance(request) {
		writeAttendanceLeaveRequestError(responseWriter, errAttendanceLeaveAccessDenied)
		return
	}
	var input attendanceLeaveApprovalInput
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if errorValue := decoder.Decode(&input); errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, attendanceLeaveInvalidInputError(errorValue))
		return
	}
	if errorValue := ensureAttendanceLeaveRequestJSONEnd(decoder); errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, attendanceLeaveInvalidInputError(errorValue))
		return
	}
	service.attendanceLeavePolicyMutationMutex.Lock()
	defer service.attendanceLeavePolicyMutationMutex.Unlock()
	view, errorValue := service.decideAttendanceLeaveRequest(
		request.Context(),
		requestID,
		service.webStaffActorEmail(request),
		input,
		time.Now(),
	)
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	responseWriter.Header().Set("Cache-Control", "private, no-store")
	service.writeJSON(responseWriter, map[string]attendanceLeaveApprovalRequestView{
		"request": view,
	})
}

func attendanceLeaveApprovalRequestID(path string) (string, bool) {
	prefix := "/leave-approvals/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	requestID := strings.TrimPrefix(path, prefix)
	if requestID == "" || strings.Contains(requestID, "/") {
		return "", false
	}
	return requestID, true
}
