package admind

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func (service *Service) writeAttendanceLeaveRequestPreview(responseWriter http.ResponseWriter, request *http.Request) {
	var input attendanceLeaveRequestInput
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
	if strings.TrimSpace(input.Reason) != "" || strings.TrimSpace(input.Response) != "" {
		writeAttendanceLeaveRequestError(
			responseWriter,
			attendanceLeaveInvalidInputErrorf("leave request preview must not include reason or response"),
		)
		return
	}
	preview, errorValue := service.previewAttendanceLeaveRequest(request.Context(), input, time.Now())
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	responseWriter.Header().Set("Cache-Control", "private, no-store")
	service.writeJSON(responseWriter, preview)
}

func (service *Service) writeAttendanceLeaveRequest(responseWriter http.ResponseWriter, request *http.Request) {
	input, uploads, errorValue := readAttendanceLeaveRequestMultipartInput(responseWriter, request)
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	input = normalizeAttendanceLeaveRequestInput(input)
	if input.Response != "" {
		writeAttendanceLeaveRequestError(
			responseWriter,
			attendanceLeaveInvalidInputErrorf("new leave request must not include a response"),
		)
		return
	}
	service.attendanceLeavePolicyMutationMutex.Lock()
	defer service.attendanceLeavePolicyMutationMutex.Unlock()
	now := time.Now()
	preview, errorValue := service.previewAttendanceLeaveRequest(request.Context(), input, now)
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	policy, errorValue := service.readAttendanceLeavePolicy(request.Context())
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	leaveType, found := attendanceLeaveTypeByID(policy, input.LeaveTypeID)
	if !found || !leaveType.IsActive {
		writeAttendanceLeaveRequestError(responseWriter, attendanceLeaveInvalidInputErrorf("leave type is not active"))
		return
	}
	leaveType = attendanceLeaveTypeForRequest(policy, leaveType)
	employee, errorValue := service.attendanceLeaveEmployeeWithHireDateForRequest(
		request,
		service.webStaffActorEmail(request),
	)
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	if errorValue := ensureAttendanceLeaveEmployeeCanUseType(employee, leaveType); errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	attachments, errorValue := service.storeAttendanceLeaveRequestAttachments(uploads)
	keepAttachments := false
	defer func() {
		if !keepAttachments {
			cleanupFailedAttendanceLeaveRequestAttachments(
				request.Context(),
				service.attendanceLeaveAttachmentDirectory(),
				attachments,
			)
		}
	}()
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	if errorValue := service.synchronizeAttendanceLeaveAccruals(
		request.Context(),
		employee,
		policy,
		now,
	); errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	record, errorValue := service.createAttendanceLeaveRequest(
		request.Context(),
		employee,
		input,
		preview,
		leaveType,
		policy,
		attachments,
		now,
	)
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	keepAttachments = true
	responseWriter.Header().Set("Cache-Control", "private, no-store")
	service.writeJSON(responseWriter, map[string]attendanceLeaveRequestView{
		"request": projectAttendanceLeaveRequest(record, now, service.workspaceTimeZone().location),
	})
}

func (service *Service) writeAttendanceLeaveDashboard(responseWriter http.ResponseWriter, request *http.Request) {
	employee, errorValue := service.attendanceLeaveEmployeeWithHireDateForRequest(
		request,
		service.webStaffActorEmail(request),
	)
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	dashboard, errorValue := service.readAttendanceLeaveDashboard(request.Context(), employee, time.Now())
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	responseWriter.Header().Set("Cache-Control", "private, no-store")
	service.writeJSON(responseWriter, dashboard)
}

func (service *Service) cancelAttendanceLeaveRequestResponse(
	responseWriter http.ResponseWriter,
	request *http.Request,
	requestID string,
) {
	employee := service.attendanceLeaveEmployeeForRequest(
		request,
		service.webStaffActorEmail(request),
	)
	errorValue := service.cancelAttendanceLeaveRequest(request.Context(), requestID, employee, time.Now())
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	responseWriter.Header().Set("Cache-Control", "private, no-store")
	service.writeJSON(responseWriter, map[string]bool{"ok": true})
}

func (service *Service) resubmitAttendanceLeaveRequestResponse(
	responseWriter http.ResponseWriter,
	request *http.Request,
	requestID string,
) {
	input, uploads, errorValue := readAttendanceLeaveRequestMultipartInput(responseWriter, request)
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	input = normalizeAttendanceLeaveRequestInput(input)
	service.attendanceLeavePolicyMutationMutex.Lock()
	defer service.attendanceLeavePolicyMutationMutex.Unlock()
	now := time.Now()
	preview, errorValue := service.previewAttendanceLeaveRequest(request.Context(), input, now)
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	policy, errorValue := service.readAttendanceLeavePolicy(request.Context())
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	leaveType, found := attendanceLeaveTypeByID(policy, input.LeaveTypeID)
	if !found || !leaveType.IsActive {
		writeAttendanceLeaveRequestError(responseWriter, attendanceLeaveInvalidInputErrorf("leave type is not active"))
		return
	}
	leaveType = attendanceLeaveTypeForRequest(policy, leaveType)
	employee, errorValue := service.attendanceLeaveEmployeeWithHireDateForRequest(
		request,
		service.webStaffActorEmail(request),
	)
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	if errorValue := ensureAttendanceLeaveEmployeeCanUseType(employee, leaveType); errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	attachments, errorValue := service.storeAttendanceLeaveRequestAttachments(uploads)
	keepAttachments := false
	defer func() {
		if !keepAttachments {
			cleanupFailedAttendanceLeaveRequestAttachments(
				request.Context(),
				service.attendanceLeaveAttachmentDirectory(),
				attachments,
			)
		}
	}()
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	if errorValue := service.synchronizeAttendanceLeaveAccruals(
		request.Context(),
		employee,
		policy,
		now,
	); errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	record, errorValue := service.resubmitAttendanceLeaveRequest(
		request.Context(),
		requestID,
		employee,
		input,
		preview,
		leaveType,
		policy,
		attachments,
		now,
	)
	if errorValue != nil {
		writeAttendanceLeaveRequestError(responseWriter, errorValue)
		return
	}
	keepAttachments = true
	responseWriter.Header().Set("Cache-Control", "private, no-store")
	service.writeJSON(responseWriter, map[string]attendanceLeaveRequestView{
		"request": projectAttendanceLeaveRequest(record, now, service.workspaceTimeZone().location),
	})
}

func attendanceLeaveRequestActionID(path string, action string) (string, bool) {
	prefix := "/leave-requests/"
	suffix := "/" + action
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}
	requestID := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	if requestID == "" || strings.Contains(requestID, "/") {
		return "", false
	}
	return requestID, true
}
