package admind

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAttendanceLeaveRequestErrorsUseClosedJSONContract(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	tests := []struct {
		name       string
		method     string
		target     string
		actorEmail string
		status     int
		code       string
	}{
		{
			name:       "unknown leave request path",
			method:     http.MethodPost,
			target:     "/attendance/api/leave-requests/missing-action",
			actorEmail: "staff@example.com",
			status:     http.StatusNotFound,
			code:       attendanceLeaveErrorRequestNotFound,
		},
		{
			name:       "malformed leave dashboard path",
			method:     http.MethodGet,
			target:     "/attendance/api/leave/",
			actorEmail: "staff@example.com",
			status:     http.StatusNotFound,
			code:       attendanceLeaveErrorRequestNotFound,
		},
		{
			name:   "unauthorized leave dashboard",
			method: http.MethodGet,
			target: "/attendance/api/leave",
			status: http.StatusForbidden,
			code:   attendanceLeaveErrorAccessDenied,
		},
		{
			name:   "local request without employee identity",
			method: http.MethodGet,
			target: "/attendance/api/leave",
			status: http.StatusForbidden,
			code:   attendanceLeaveErrorAccessDenied,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.target, nil)
			request.RemoteAddr = "203.0.113.10:1234"
			if test.name == "local request without employee identity" {
				request.RemoteAddr = "127.0.0.1:1234"
			}
			if test.actorEmail != "" {
				request.Header.Set("X-Forwarded-Email", test.actorEmail)
			}
			recorder := httptest.NewRecorder()

			service.handleAttendance(recorder, request)

			assertAttendanceLeaveErrorResponse(t, recorder, test.status, test.code)
		})
	}
}

func TestAttendanceLeaveRequestKnownRoutesRejectWrongMethod(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	tests := []struct {
		name          string
		method        string
		target        string
		allowedMethod string
	}{
		{
			name:          "dashboard",
			method:        http.MethodPost,
			target:        "/attendance/api/leave",
			allowedMethod: http.MethodGet,
		},
		{
			name:          "preview",
			method:        http.MethodGet,
			target:        "/attendance/api/leave-requests/preview",
			allowedMethod: http.MethodPost,
		},
		{
			name:          "create",
			method:        http.MethodGet,
			target:        "/attendance/api/leave-requests",
			allowedMethod: http.MethodPost,
		},
		{
			name:          "resubmit",
			method:        http.MethodGet,
			target:        "/attendance/api/leave-requests/request-1/resubmit",
			allowedMethod: http.MethodPost,
		},
		{
			name:          "cancel",
			method:        http.MethodGet,
			target:        "/attendance/api/leave-requests/request-1/cancel",
			allowedMethod: http.MethodPost,
		},
		{
			name:          "attachment download",
			method:        http.MethodPost,
			target:        "/attendance/api/leave-requests/request-1/attachments/attachment-1",
			allowedMethod: http.MethodGet,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.target, nil)
			request.RemoteAddr = "203.0.113.10:1234"
			request.Header.Set("X-Forwarded-Email", "staff@example.com")
			recorder := httptest.NewRecorder()

			service.handleAttendance(recorder, request)

			assertAttendanceLeaveErrorResponse(
				t,
				recorder,
				http.StatusMethodNotAllowed,
				attendanceLeaveErrorInvalidInput,
			)
			if recorder.Header().Get("Allow") != test.allowedMethod {
				t.Fatalf("allow = %q", recorder.Header().Get("Allow"))
			}
		})
	}
}

func TestAttendanceLeaveRequestMultipartLimitUsesTypedError(t *testing.T) {
	errorValue := attendanceLeaveRequestMultipartError(
		&http.MaxBytesError{Limit: attendanceLeaveRequestMaximumMultipartBytes},
	)
	code, status := attendanceLeaveErrorCodeAndStatus(errorValue)
	if code != attendanceLeaveErrorInvalidAttachment || status != http.StatusRequestEntityTooLarge {
		t.Fatalf("code = %q status = %d error = %v", code, status, errorValue)
	}
}

func TestAttendanceLeaveRequestInternalErrorDoesNotExposeDetails(t *testing.T) {
	recorder := httptest.NewRecorder()

	writeAttendanceLeaveRequestError(
		recorder,
		errors.New("open /private/leave.sqlite: permission denied"),
	)

	response := assertAttendanceLeaveErrorResponse(
		t,
		recorder,
		http.StatusInternalServerError,
		attendanceLeaveErrorInternal,
	)
	if response.Error != "internal leave request error" {
		t.Fatalf("error = %q", response.Error)
	}
}

func TestAttendanceLeaveRequestMissingCancelTargetReturnsRequestNotFound(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	request := httptest.NewRequest(
		http.MethodPost,
		"/attendance/api/leave-requests/missing/cancel",
		nil,
	)
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-Email", "staff@example.com")
	recorder := httptest.NewRecorder()

	service.handleAttendance(recorder, request)

	assertAttendanceLeaveErrorResponse(
		t,
		recorder,
		http.StatusNotFound,
		attendanceLeaveErrorRequestNotFound,
	)
}
