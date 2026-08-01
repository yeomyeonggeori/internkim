package admind

import (
	"encoding/json"
	"net/http"
	"time"
)

func (service *Service) writeAttendanceLeaveManagement(
	responseWriter http.ResponseWriter,
	request *http.Request,
) {
	if !service.canManageAttendance(request) {
		writeAttendanceLeaveRequestError(responseWriter, errAttendanceLeaveAccessDenied)
		return
	}
	response, errorValue := service.readAttendanceLeaveManagement(
		request,
		request.URL.Query().Get("email"),
		time.Now(),
	)
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	responseWriter.Header().Set("Cache-Control", "private, no-store")
	service.writeJSON(responseWriter, response)
}

func (service *Service) writeAttendanceLeaveManagementAdjustment(
	responseWriter http.ResponseWriter,
	request *http.Request,
) {
	if !service.canManageAttendance(request) {
		writeAttendanceLeaveRequestError(responseWriter, errAttendanceLeaveAccessDenied)
		return
	}
	var input attendanceLeaveManagementAdjustmentInput
	if errorValue := decodeAttendanceLeaveManagementJSON(request, &input); errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	balance, errorValue := service.adjustManagedAttendanceLeave(
		request.Context(),
		input,
		time.Now(),
	)
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	responseWriter.Header().Set("Cache-Control", "private, no-store")
	service.writeJSON(responseWriter, map[string]attendanceLeaveBalance{"balance": balance})
}

func (service *Service) writeAttendanceLeaveManagementPastLeave(
	responseWriter http.ResponseWriter,
	request *http.Request,
) {
	if !service.canManageAttendance(request) {
		writeAttendanceLeaveRequestError(responseWriter, errAttendanceLeaveAccessDenied)
		return
	}
	var input attendanceLeaveManagementPastLeaveInput
	if errorValue := decodeAttendanceLeaveManagementJSON(request, &input); errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	view, errorValue := service.createManagedPastAttendanceLeave(
		request.Context(),
		input,
		service.webStaffActorEmail(request),
		time.Now(),
	)
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	responseWriter.Header().Set("Cache-Control", "private, no-store")
	service.writeJSON(responseWriter, map[string]attendanceLeaveApprovalRequestView{"request": view})
}

func (service *Service) writeAttendanceLeaveManagementCancellation(
	responseWriter http.ResponseWriter,
	request *http.Request,
	requestID string,
) {
	if !service.canManageAttendance(request) {
		writeAttendanceLeaveRequestError(responseWriter, errAttendanceLeaveAccessDenied)
		return
	}
	var input attendanceLeaveManagementCancelInput
	if errorValue := decodeAttendanceLeaveManagementJSON(request, &input); errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	if errorValue := service.cancelManagedAttendanceLeave(
		request.Context(),
		requestID,
		input.EmployeeEmail,
		service.webStaffActorEmail(request),
		time.Now(),
	); errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	responseWriter.Header().Set("Cache-Control", "private, no-store")
	service.writeJSON(responseWriter, map[string]bool{"ok": true})
}

func (service *Service) writeAttendanceLeaveManagementTimeCorrection(
	responseWriter http.ResponseWriter,
	request *http.Request,
	requestID string,
) {
	if !service.canManageAttendance(request) {
		writeAttendanceLeaveRequestError(responseWriter, errAttendanceLeaveAccessDenied)
		return
	}
	var input attendanceLeaveManagementTimeInput
	if errorValue := decodeAttendanceLeaveManagementJSON(request, &input); errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	if errorValue := service.correctManagedAttendanceLeaveTime(
		request.Context(),
		requestID,
		input,
		service.webStaffActorEmail(request),
		time.Now(),
	); errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	responseWriter.Header().Set("Cache-Control", "private, no-store")
	service.writeJSON(responseWriter, map[string]bool{"ok": true})
}

func decodeAttendanceLeaveManagementJSON(request *http.Request, destination any) error {
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if errorValue := decoder.Decode(destination); errorValue != nil {
		return attendanceLeaveInvalidInputError(errorValue)
	}
	if errorValue := ensureAttendanceLeaveRequestJSONEnd(decoder); errorValue != nil {
		return attendanceLeaveInvalidInputError(errorValue)
	}
	return nil
}
