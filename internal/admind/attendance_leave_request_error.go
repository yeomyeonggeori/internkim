package admind

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

const (
	attendanceLeaveErrorInvalidInput        = "invalidInput"
	attendanceLeaveErrorLeaveConflict       = "leaveConflict"
	attendanceLeaveErrorWorkConflict        = "workConflict"
	attendanceLeaveErrorInsufficientBalance = "insufficientBalance"
	attendanceLeaveErrorInvalidAttachment   = "invalidAttachment"
	attendanceLeaveErrorRequestNotFound     = "requestNotFound"
	attendanceLeaveErrorInvalidStatus       = "invalidStatus"
	attendanceLeaveErrorAccessDenied        = "accessDenied"
	attendanceLeaveErrorInternal            = "internal"
)

var errAttendanceLeaveInvalidInput = errors.New("invalid leave request input")
var errAttendanceLeaveRequestTooLarge = errors.New("leave request payload is too large")
var errAttendanceLeaveMethodNotAllowed = errors.New("leave request method is not allowed")
var errAttendanceLeaveWorkConflict = errors.New("leave request conflicts with confirmed work")
var errAttendanceLeaveInvalidAttachment = errors.New("invalid leave request attachment")
var errAttendanceLeaveAccessDenied = errors.New("attendance leave access required")

type attendanceLeaveErrorResponse struct {
	Code  string `json:"code"`
	Error string `json:"error"`
}

func attendanceLeaveInvalidInputError(errorValue error) error {
	if errorValue == nil {
		return errAttendanceLeaveInvalidInput
	}
	return fmt.Errorf("%w: %v", errAttendanceLeaveInvalidInput, errorValue)
}

func attendanceLeaveInvalidInputErrorf(format string, values ...any) error {
	return attendanceLeaveInvalidInputError(fmt.Errorf(format, values...))
}

func attendanceLeaveInvalidAttachmentError(errorValue error) error {
	if errorValue == nil {
		return errAttendanceLeaveInvalidAttachment
	}
	return fmt.Errorf("%w: %v", errAttendanceLeaveInvalidAttachment, errorValue)
}

func attendanceLeaveErrorCodeAndStatus(errorValue error) (string, int) {
	switch {
	case errors.Is(errorValue, errAttendanceLeaveRequestTooLarge):
		return attendanceLeaveErrorInvalidAttachment, http.StatusRequestEntityTooLarge
	case errors.Is(errorValue, errAttendanceLeaveMethodNotAllowed):
		return attendanceLeaveErrorInvalidInput, http.StatusMethodNotAllowed
	case errors.Is(errorValue, errAttendanceLeaveInvalidInput):
		return attendanceLeaveErrorInvalidInput, http.StatusBadRequest
	case errors.Is(errorValue, errAttendanceLeaveRequestConflict):
		return attendanceLeaveErrorLeaveConflict, http.StatusConflict
	case errors.Is(errorValue, errAttendanceLeaveWorkConflict):
		return attendanceLeaveErrorWorkConflict, http.StatusConflict
	case errors.Is(errorValue, errAttendanceLeaveInsufficientBalance):
		return attendanceLeaveErrorInsufficientBalance, http.StatusConflict
	case errors.Is(errorValue, errAttendanceLeaveInvalidAttachment),
		errors.Is(errorValue, errAttendanceLeaveRequestAttachmentLimit):
		return attendanceLeaveErrorInvalidAttachment, http.StatusBadRequest
	case errors.Is(errorValue, errAttendanceLeaveRequestNotFound):
		return attendanceLeaveErrorRequestNotFound, http.StatusNotFound
	case errors.Is(errorValue, errAttendanceLeaveRequestCannotCancel),
		errors.Is(errorValue, errAttendanceLeaveRequestAlreadyStarted),
		errors.Is(errorValue, errAttendanceLeaveRequestCannotResubmit),
		errors.Is(errorValue, errAttendanceLeaveRequestCannotUpdate),
		errors.Is(errorValue, errAttendanceLeaveApprovalConflict):
		return attendanceLeaveErrorInvalidStatus, http.StatusConflict
	case errors.Is(errorValue, errAttendanceLeaveAccessDenied):
		return attendanceLeaveErrorAccessDenied, http.StatusForbidden
	default:
		return attendanceLeaveErrorInternal, http.StatusInternalServerError
	}
}

func writeAttendanceLeaveRequestError(
	responseWriter http.ResponseWriter,
	errorValue error,
) {
	code, status := attendanceLeaveErrorCodeAndStatus(errorValue)
	responseWriter.Header().Set("Cache-Control", "private, no-store")
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(status)
	message := errorValue.Error()
	if code == attendanceLeaveErrorInternal {
		slog.Error(
			"attendance leave request failed",
			"code",
			code,
			"error",
			errorValue,
		)
		message = "internal leave request error"
	}
	_ = json.NewEncoder(responseWriter).Encode(attendanceLeaveErrorResponse{
		Code:  code,
		Error: message,
	})
}
