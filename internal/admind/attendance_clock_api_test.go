package admind

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAttendanceClockRequiresActorEmail(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/attendance/api/clock", strings.NewReader(`{}`))
	service.handleAttendance(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestAttendanceClockTogglesClockIn(t *testing.T) {
	service, messages := newAttendanceActionTestService(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/attendance/api/clock", strings.NewReader(`{}`))
	request.Header.Set("X-Forwarded-Email", "staff@example.com")
	service.handleAttendance(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if len(*messages) != 1 {
		t.Fatalf("expected 1 mattermost message, got %d: %v", len(*messages), *messages)
	}
	if !strings.Contains((*messages)[0].Message, "출근") {
		t.Fatalf("expected clock-in message, got %q", (*messages)[0])
	}
}

func TestAttendanceClockAcceptsExplicitClockOut(t *testing.T) {
	service, messages := newAttendanceActionTestService(t)
	clockInRecorder := httptest.NewRecorder()
	clockInRequest := httptest.NewRequest(http.MethodPost, "/attendance/api/clock", strings.NewReader(`{"kind":"clock_in"}`))
	clockInRequest.Header.Set("X-Forwarded-Email", "staff@example.com")
	service.handleAttendance(clockInRecorder, clockInRequest)
	if clockInRecorder.Code != http.StatusOK {
		t.Fatalf("clock-in status = %d body = %s", clockInRecorder.Code, clockInRecorder.Body.String())
	}
	clockOutRecorder := httptest.NewRecorder()
	clockOutRequest := httptest.NewRequest(http.MethodPost, "/attendance/api/clock", strings.NewReader(`{"kind":"clock_out"}`))
	clockOutRequest.Header.Set("X-Forwarded-Email", "staff@example.com")
	service.handleAttendance(clockOutRecorder, clockOutRequest)
	if clockOutRecorder.Code != http.StatusOK {
		t.Fatalf("clock-out status = %d body = %s", clockOutRecorder.Code, clockOutRecorder.Body.String())
	}
	if len(*messages) != 2 {
		t.Fatalf("expected 2 mattermost messages, got %d: %v", len(*messages), *messages)
	}
	if !strings.Contains((*messages)[1].Message, "퇴근") {
		t.Fatalf("expected clock-out message, got %q", (*messages)[1])
	}
}
