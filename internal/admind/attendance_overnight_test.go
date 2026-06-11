package admind

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func moveLatestAttendanceEventToYesterday(t *testing.T, service *Service) {
	t.Helper()
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	location, _ := service.workspaceTimeLocation()
	yesterday := time.Now().In(location).AddDate(0, 0, -1)
	occurredAt := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 21, 50, 0, 0, location)
	if _, errorValue := database.ExecContext(context.Background(), `
UPDATE attendance_events
SET local_date = ?, occurred_at = ?, local_time = ?
WHERE id = (SELECT id FROM attendance_events ORDER BY occurred_at DESC LIMIT 1)`,
		yesterday.Format("2006-01-02"),
		occurredAt.UTC().Format(time.RFC3339Nano),
		occurredAt.Format("15:04:05"),
	); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestAttendanceChannelPostAllowsOvernightClockOut(t *testing.T) {
	service, posts := newAttendanceActionTestService(t)
	service.saveMattermostAttendanceChannelID("attendance-channel")
	service.saveMattermostAttendanceEntryPostID("entry-post")
	payload := mattermostInteractivePayload{
		UserID:    "user-1",
		PostID:    "entry-post",
		ChannelID: "attendance-channel",
		TeamID:    "team-1",
		Context:   mattermostInteractiveContext{Action: attendanceClockInAction, Token: service.ensureMattermostInteractiveActionToken()},
	}
	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockIn); errorValue != nil {
		t.Fatal(errorValue)
	}
	moveLatestAttendanceEventToYesterday(t, service)

	request := httptest.NewRequest(http.MethodPost, "/api/v4/posts", strings.NewReader(`{"channel_id":"attendance-channel","root_id":"entry-post","message":"퇴근"}`))
	request.Header.Set("Cookie", "MMAUTHTOKEN=session-token")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	events, errorValue := service.readAttendanceEvents(context.Background(), time.Now().Format("2006-01"), "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	clockOutCount := 0
	for _, event := range events {
		if event.Kind == attendanceKindClockOut && event.CanceledAt == "" {
			clockOutCount++
		}
	}
	if clockOutCount != 1 {
		t.Fatalf("expected overnight clock-out to be recorded, events = %+v posts = %+v", events, *posts)
	}
}

func TestAttendanceToggleAfterOvernightClockInProducesClockOut(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	payload := mattermostInteractivePayload{
		UserID:    "user-1",
		PostID:    "entry-post",
		ChannelID: "attendance-channel",
		TeamID:    "team-1",
		Context:   mattermostInteractiveContext{Action: attendanceClockInAction, Token: service.ensureMattermostInteractiveActionToken()},
	}
	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockIn); errorValue != nil {
		t.Fatal(errorValue)
	}
	moveLatestAttendanceEventToYesterday(t, service)

	if _, errorValue := service.toggleAttendanceFromMattermost(context.Background(), payload); errorValue != nil {
		t.Fatal(errorValue)
	}

	events, errorValue := service.readAttendanceEvents(context.Background(), time.Now().Format("2006-01"), "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	latestEvent := events[0]
	for _, event := range events {
		if event.OccurredAt > latestEvent.OccurredAt {
			latestEvent = event
		}
	}
	if latestEvent.Kind != attendanceKindClockOut {
		t.Fatalf("expected toggle to clock out after overnight clock-in, events = %+v", events)
	}
}

func TestAttendanceClockInStillAllowedAfterForgottenClockOut(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	service.saveMattermostAttendanceChannelID("attendance-channel")
	service.saveMattermostAttendanceEntryPostID("entry-post")
	payload := mattermostInteractivePayload{
		UserID:    "user-1",
		PostID:    "entry-post",
		ChannelID: "attendance-channel",
		TeamID:    "team-1",
		Context:   mattermostInteractiveContext{Action: attendanceClockInAction, Token: service.ensureMattermostInteractiveActionToken()},
	}
	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockIn); errorValue != nil {
		t.Fatal(errorValue)
	}
	moveLatestAttendanceEventToYesterday(t, service)

	request := httptest.NewRequest(http.MethodPost, "/api/v4/posts", strings.NewReader(`{"channel_id":"attendance-channel","root_id":"entry-post","message":"출근"}`))
	request.Header.Set("Cookie", "MMAUTHTOKEN=session-token")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	events, errorValue := service.readAttendanceEvents(context.Background(), time.Now().Format("2006-01"), "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	clockInCount := 0
	for _, event := range events {
		if event.Kind == attendanceKindClockIn && event.CanceledAt == "" {
			clockInCount++
		}
	}
	if clockInCount != 2 {
		t.Fatalf("expected morning clock-in to stay allowed, events = %+v", events)
	}
}
