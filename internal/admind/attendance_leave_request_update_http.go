package admind

import (
	"net/http"
	"time"
)

func (service *Service) updateAttendanceLeaveRequestResponse(
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
	if input.Revision < 1 {
		writeAttendanceLeaveRequestError(
			responseWriter,
			attendanceLeaveInvalidInputErrorf("leave request revision is required"),
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
		writeAttendanceLeaveRequestError(
			responseWriter,
			attendanceLeaveInvalidInputErrorf("leave type is not active"),
		)
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
	record, errorValue := service.updateAttendanceLeaveRequest(
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
		"request": projectAttendanceLeaveRequest(
			record,
			now,
			service.workspaceTimeZone().location,
		),
	})
}
