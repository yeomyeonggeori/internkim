package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAttendanceSummaryScopesHiddenTeamViewToActor(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	occurredAt := time.Date(2026, 5, 19, 9, 0, 0, 0, time.UTC)
	insertAttendanceSummaryTestEvent(t, service, database, mattermostUserRecord{
		ID:       "user-1",
		Username: "staff",
		Email:    "staff@example.com",
		Nickname: "Staff",
	}, attendanceKindClockIn, occurredAt)
	insertAttendanceSummaryTestEvent(t, service, database, mattermostUserRecord{
		ID:       "user-2",
		Username: "other",
		Email:    "other@example.com",
		Nickname: "Other",
	}, attendanceKindClockIn, occurredAt)
	if errorValue := service.writeAttendanceTeamViewVisibleToAll(ctx, false); errorValue != nil {
		t.Fatal(errorValue)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/attendance/api/summary?month=2026-05&email=other@example.com", nil)
	request.Header.Set("X-Forwarded-Email", "staff@example.com")

	service.handleAttendance(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var response attendanceSummaryResponse
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &response); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.TeamViewBlocked {
		t.Fatalf("expected hidden team view to be blocked")
	}
	if len(response.Events) != 1 {
		t.Fatalf("events = %+v", response.Events)
	}
	if response.Events[0].Email != "staff@example.com" {
		t.Fatalf("event email = %q", response.Events[0].Email)
	}
}

func insertAttendanceSummaryTestEvent(t *testing.T, service *Service, database *sql.DB, userRecord mattermostUserRecord, kind string, occurredAt time.Time) {
	t.Helper()
	event := service.createAttendanceEvent(
		userRecord,
		kind,
		occurredAt,
		"team-1",
		"attendance-channel",
		"action-post-"+userRecord.ID,
		"result-post-"+userRecord.ID,
		attendanceLocation{},
	)
	if errorValue := service.insertAttendanceEvent(context.Background(), database, event); errorValue != nil {
		t.Fatal(errorValue)
	}
}
