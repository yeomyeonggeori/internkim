package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAttendanceEventLocationOverridePersistsInSummary(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	if errorValue := service.writeAttendanceLocationsFile([]attendanceLocation{
		{ID: "office", Name: "사무실", Color: "#22c55e", IsDefault: true},
		{ID: "remote", Name: "재택", Color: "#3b82f6"},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	event := insertAttendanceLocationOverrideTestEvent(t, service, false)

	overrideRecorder := httptest.NewRecorder()
	overrideRequest := newAttendanceLocationOverrideRequest(event.ID, "remote", "staff@example.com")
	service.handleAttendance(overrideRecorder, overrideRequest)

	if overrideRecorder.Code != http.StatusOK {
		t.Fatalf("override status = %d body = %s", overrideRecorder.Code, overrideRecorder.Body.String())
	}
	var overrideResponse attendanceEvent
	if errorValue := json.Unmarshal(overrideRecorder.Body.Bytes(), &overrideResponse); errorValue != nil {
		t.Fatal(errorValue)
	}
	if overrideResponse.LocationID != "remote" || overrideResponse.LocationName != "재택" {
		t.Fatalf("override location = %q %q", overrideResponse.LocationID, overrideResponse.LocationName)
	}
	if overrideResponse.ParsedAs == nil || overrideResponse.ParsedAs.LocationID != "office" {
		t.Fatalf("parsedAs = %+v", overrideResponse.ParsedAs)
	}
	if overrideResponse.OverriddenBy != "staff@example.com" || overrideResponse.OverriddenAt == "" {
		t.Fatalf("override metadata = %q %q", overrideResponse.OverriddenBy, overrideResponse.OverriddenAt)
	}

	summaryRecorder := httptest.NewRecorder()
	summaryRequest := httptest.NewRequest(http.MethodGet, "/attendance/api/summary?month=2026-05", nil)
	summaryRequest.RemoteAddr = "203.0.113.10:49152"
	summaryRequest.Header.Set("X-Forwarded-Email", "staff@example.com")
	service.handleAttendance(summaryRecorder, summaryRequest)

	if summaryRecorder.Code != http.StatusOK {
		t.Fatalf("summary status = %d body = %s", summaryRecorder.Code, summaryRecorder.Body.String())
	}
	var summary attendanceSummaryResponse
	if errorValue := json.Unmarshal(summaryRecorder.Body.Bytes(), &summary); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(summary.Events) != 1 {
		t.Fatalf("events = %+v", summary.Events)
	}
	if summary.Events[0].LocationID != "remote" || summary.Events[0].LocationName != "재택" {
		t.Fatalf("summary location = %q %q", summary.Events[0].LocationID, summary.Events[0].LocationName)
	}
	if summary.Events[0].ParsedAs == nil || summary.Events[0].ParsedAs.LocationID != "office" {
		t.Fatalf("summary parsedAs = %+v", summary.Events[0].ParsedAs)
	}
}

func TestAttendanceEventLocationOverrideRejectsOtherUser(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	event := insertAttendanceLocationOverrideTestEvent(t, service, false)

	recorder := httptest.NewRecorder()
	request := newAttendanceLocationOverrideRequest(event.ID, "office", "other@example.com")
	service.handleAttendance(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestAttendanceEventLocationOverrideRejectsCanceledEvent(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	event := insertAttendanceLocationOverrideTestEvent(t, service, true)

	recorder := httptest.NewRecorder()
	request := newAttendanceLocationOverrideRequest(event.ID, "office", "staff@example.com")
	service.handleAttendance(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestAttendanceEventLocationOverrideRejectsStaleCanceledUpdate(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	event := insertAttendanceLocationOverrideTestEvent(t, service, false)
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.ExecContext(context.Background(), `UPDATE attendance_events SET canceled_at = ?, cancel_reason = ? WHERE id = ?`, "2026-05-19T09:05:00Z", "duplicate_click", event.ID); errorValue != nil {
		t.Fatal(errorValue)
	}

	updatedEvent, errorValue := service.updateAttendanceEventLocation(context.Background(), database, event, attendanceLocation{ID: "remote", Name: "재택"}, "staff@example.com", time.Now().UTC())

	if errorValue == nil {
		t.Fatalf("error = nil updated event = %+v", updatedEvent)
	}
	storedEvent, found, errorValue := service.attendanceEventByID(context.Background(), database, event.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found {
		t.Fatalf("event was not found")
	}
	if storedEvent.LocationID != "office" || storedEvent.OverriddenAt != "" {
		t.Fatalf("stored event = %+v", storedEvent)
	}
}

func TestAttendanceEventLocationOverrideUsesLocalRequesterAsActor(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	if errorValue := service.writeAttendanceLocationsFile([]attendanceLocation{
		{ID: "office", Name: "사무실", Color: "#22c55e", IsDefault: true},
		{ID: "remote", Name: "재택", Color: "#3b82f6"},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	event := insertAttendanceLocationOverrideTestEvent(t, service, false)

	recorder := httptest.NewRecorder()
	request := newAttendanceLocationOverrideRequest(event.ID, "remote", "")
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set(flowRequesterEmailHeader, "staff@example.com")
	service.handleAttendance(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var response attendanceEvent
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &response); errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.OverriddenBy != "staff@example.com" {
		t.Fatalf("overridden by = %q", response.OverriddenBy)
	}
}

func TestAttendanceEventLocationOverrideUsesLocalAdminFallbackActor(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	if errorValue := service.writeAttendanceLocationsFile([]attendanceLocation{
		{ID: "office", Name: "사무실", Color: "#22c55e", IsDefault: true},
		{ID: "remote", Name: "재택", Color: "#3b82f6"},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	event := insertAttendanceLocationOverrideTestEvent(t, service, false)

	recorder := httptest.NewRecorder()
	request := newAttendanceLocationOverrideRequest(event.ID, "remote", "")
	request.RemoteAddr = "127.0.0.1:12345"
	service.handleAttendance(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var response attendanceEvent
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &response); errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.OverriddenBy != attendanceOverrideActorLocalAdmin {
		t.Fatalf("overridden by = %q", response.OverriddenBy)
	}
}

func insertAttendanceLocationOverrideTestEvent(t *testing.T, service *Service, canceled bool) attendanceEvent {
	t.Helper()
	ctx := context.Background()
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	event := service.createAttendanceEvent(
		mattermostUserRecord{ID: "user-1", Username: "staff", Email: "staff@example.com", Nickname: "Staff"},
		attendanceKindClockIn,
		time.Date(2026, 5, 19, 9, 0, 0, 0, time.UTC),
		"team-1",
		"attendance-channel",
		"entry-post",
		"result-post",
		attendanceLocation{ID: "office", Name: "사무실"},
	)
	if errorValue := service.insertAttendanceEvent(ctx, database, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	if canceled {
		if _, errorValue := database.ExecContext(ctx, `UPDATE attendance_events SET canceled_at = ?, cancel_reason = ? WHERE id = ?`, "2026-05-19T09:05:00Z", "duplicate_click", event.ID); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	return event
}

func newAttendanceLocationOverrideRequest(eventID string, locationID string, actorEmail string) *http.Request {
	request := httptest.NewRequest(http.MethodPatch, "/attendance/api/events/"+eventID+"/location", strings.NewReader(`{"locationID":"`+locationID+`"}`))
	request.RemoteAddr = "203.0.113.10:49152"
	request.Header.Set("X-Forwarded-Email", actorEmail)
	return request
}
