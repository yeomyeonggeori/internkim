package admind

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func decodeAttendanceLeaveRequestIDForTest(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
) string {
	t.Helper()
	if recorder.Code != http.StatusOK {
		t.Fatalf("create status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var created struct {
		Request struct {
			ID string `json:"id"`
		} `json:"request"`
	}
	if errorValue := json.NewDecoder(recorder.Body).Decode(&created); errorValue != nil {
		t.Fatal(errorValue)
	}
	if created.Request.ID == "" {
		t.Fatal("missing request id")
	}
	return created.Request.ID
}

func assertAttendanceLeaveErrorResponse(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	expectedStatus int,
	expectedCode string,
) attendanceLeaveErrorResponse {
	t.Helper()
	if recorder.Code != expectedStatus {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var response attendanceLeaveErrorResponse
	if errorValue := json.NewDecoder(recorder.Body).Decode(&response); errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Code != expectedCode || response.Error == "" {
		t.Fatalf("response = %+v", response)
	}
	if !strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content type = %q", recorder.Header().Get("Content-Type"))
	}
	if recorder.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("cache control = %q", recorder.Header().Get("Cache-Control"))
	}
	return response
}

func setAttendanceLeaveRequestStatusForTest(
	t *testing.T,
	service *Service,
	requestID string,
	status string,
) {
	t.Helper()
	database, errorValue := service.openAttendanceDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.ExecContext(t.Context(), `
UPDATE attendance_leave_requests
SET status = ?
WHERE id = ?`,
		status,
		requestID,
	); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func readAttendanceLeaveDashboardForTest(
	t *testing.T,
	service *Service,
	actorEmail string,
) attendanceLeaveDashboard {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/attendance/api/leave", nil)
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-Email", actorEmail)
	recorder := httptest.NewRecorder()
	service.handleAttendance(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var dashboard attendanceLeaveDashboard
	if errorValue := json.NewDecoder(recorder.Body).Decode(&dashboard); errorValue != nil {
		t.Fatal(errorValue)
	}
	return dashboard
}

func performAttendanceLeaveMultipartRequest(
	t *testing.T,
	service *Service,
	method string,
	target string,
	actorEmail string,
	requestJSON string,
) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	field, errorValue := writer.CreateFormField("request")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := field.Write([]byte(requestJSON)); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := writer.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(method, target, &body)
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-Email", actorEmail)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	service.handleAttendance(recorder, request)
	return recorder
}

func performAttendanceLeaveMultipartRequestWithAttachment(
	t *testing.T,
	service *Service,
	target string,
	actorEmail string,
	requestJSON string,
	fileName string,
	content []byte,
) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	field, errorValue := writer.CreateFormField("request")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := field.Write([]byte(requestJSON)); errorValue != nil {
		t.Fatal(errorValue)
	}
	fileField, errorValue := writer.CreateFormFile("attachments", fileName)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := fileField.Write(content); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := writer.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPost, target, &body)
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-Email", actorEmail)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	service.handleAttendance(recorder, request)
	return recorder
}
