package admind

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAttendanceEventOverrideRejectsFutureTime(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	if errorValue := service.writeWorkspaceSettingsFile(workspaceSettings{Language: workspaceLanguageKorean, TimeZone: "Asia/Seoul"}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeAttendanceLocationsFile([]attendanceLocation{
		{ID: "office", Name: "사무실", Color: "#16a34a", IsDefault: true},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	now := time.Now().In(location)
	database, errorValue := service.openAttendanceDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	userRecord := mattermostUserRecord{ID: "user-1", Username: "staff", Email: "staff@example.com", Nickname: "Staff"}
	event := service.createAttendanceEvent(
		userRecord,
		attendanceKindClockOut,
		now.Add(-time.Hour).UTC(),
		"team-1",
		"attendance-channel",
		"entry-post",
		"result-post",
		service.attendanceLocationByID("office"),
	)
	if errorValue := service.insertAttendanceEvent(t.Context(), database, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	futureTime := now.Add(24 * time.Hour)
	payload := fmt.Sprintf(
		`{"localDate":%q,"localTime":%q,"locationID":"office","reason":"future time regression"}`,
		futureTime.Format("2006-01-02"),
		futureTime.Format("15:04"),
	)
	recorder := httptest.NewRecorder()
	request := newLocalAttendanceRequest(http.MethodPatch, "/attendance/api/events/"+event.ID, strings.NewReader(payload))
	request.Header.Set("X-Forwarded-Email", "staff@example.com")

	service.handleAttendance(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "attendance event time cannot be in the future") {
		t.Fatalf("body = %s", recorder.Body.String())
	}
	database, errorValue = service.openAttendanceDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var overrideCount int
	if errorValue := database.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM attendance_event_overrides WHERE event_id = ?", event.ID).Scan(&overrideCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if overrideCount != 0 {
		t.Fatalf("override count = %d", overrideCount)
	}
}

func TestAttendanceEventOverrideUsesCurrentMinuteBoundary(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	if errorValue := service.writeWorkspaceSettingsFile(workspaceSettings{Language: workspaceLanguageKorean, TimeZone: "Asia/Seoul"}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeAttendanceLocationsFile([]attendanceLocation{
		{ID: "office", Name: "사무실", Color: "#16a34a", IsDefault: true},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	editedAt := time.Date(2026, time.July, 14, 15, 0, 30, 0, location).UTC()
	database, errorValue := service.openAttendanceDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	userRecord := mattermostUserRecord{ID: "user-1", Username: "staff", Email: "staff@example.com", Nickname: "Staff"}
	event := service.createAttendanceEvent(
		userRecord,
		attendanceKindClockOut,
		editedAt.Add(-time.Hour),
		"team-1",
		"attendance-channel",
		"entry-post",
		"result-post",
		service.attendanceLocationByID("office"),
	)

	tests := []struct {
		name      string
		localTime string
		isFuture  bool
	}{
		{name: "current minute", localTime: "15:00", isFuture: false},
		{name: "next minute", localTime: "15:01", isFuture: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload := attendanceEventOverrideRequest{
				LocalDate:  "2026-07-14",
				LocalTime:  test.localTime,
				LocationID: "office",
				Reason:     "minute boundary regression",
			}
			override, errorValue := service.createAttendanceEventOverride(
				t.Context(),
				database,
				event,
				payload,
				"staff@example.com",
				editedAt,
			)
			if test.isFuture {
				if errorValue == nil || !strings.Contains(errorValue.Error(), "attendance event time cannot be in the future") {
					t.Fatalf("error = %v", errorValue)
				}
				return
			}
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if override.OverrideLocalTime != "15:00:00" {
				t.Fatalf("override local time = %q", override.OverrideLocalTime)
			}
		})
	}
}
