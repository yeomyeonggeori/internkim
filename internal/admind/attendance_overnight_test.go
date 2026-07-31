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
	moveLatestAttendanceEventToYesterdayAt(t, service, 21, 50)
}

func moveLatestAttendanceEventToYesterdayAt(t *testing.T, service *Service, hour int, minute int) {
	t.Helper()
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	location, _ := service.workspaceTimeLocation()
	yesterday := time.Now().In(location).AddDate(0, 0, -1)
	occurredAt := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), hour, minute, 0, 0, location)
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
	location, _ := service.workspaceTimeLocation()
	expectedWorkDate := time.Now().In(location).AddDate(0, 0, -1).Format("2006-01-02")
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
	events, errorValue := service.readAttendanceEvents(context.Background(), expectedWorkDate[:7], "")
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
	location, _ := service.workspaceTimeLocation()
	expectedWorkDate := time.Now().In(location).AddDate(0, 0, -1).Format("2006-01-02")
	expectedClockOutDate := time.Now().In(location).Format("2006-01-02")
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

	events, errorValue := service.readAttendanceEvents(context.Background(), expectedWorkDate[:7], "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) == 0 {
		t.Fatalf("expected overnight events for %s", expectedWorkDate[:7])
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
	if latestEvent.LocalDate != expectedClockOutDate {
		t.Fatalf("expected clock-out event date %s, got %+v", expectedClockOutDate, latestEvent)
	}
}

func TestAttendanceOvernightClockOutAppearsInBothMonthsAcrossBoundaries(t *testing.T) {
	cases := []struct {
		name         string
		clockIn      time.Time
		clockOut     time.Time
		workMonth    string
		nextMonth    string
		clockOutDate string
	}{
		{
			name:         "month end",
			clockIn:      time.Date(2026, 6, 30, 21, 0, 0, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
			clockOut:     time.Date(2026, 7, 1, 4, 0, 0, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
			workMonth:    "2026-06",
			nextMonth:    "2026-07",
			clockOutDate: "2026-07-01",
		},
		{
			name:         "year end",
			clockIn:      time.Date(2026, 12, 31, 21, 0, 0, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
			clockOut:     time.Date(2027, 1, 1, 4, 0, 0, 0, time.FixedZone("Asia/Seoul", 9*60*60)),
			workMonth:    "2026-12",
			nextMonth:    "2027-01",
			clockOutDate: "2027-01-01",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service, _ := newAttendanceActionTestService(t)
			if errorValue := service.writeWorkspaceSettingsFile(workspaceSettings{Language: workspaceLanguageKorean, TimeZone: "Asia/Seoul"}); errorValue != nil {
				t.Fatal(errorValue)
			}
			database, errorValue := service.openAttendanceDatabase(context.Background())
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			defer database.Close()
			userRecord := mattermostUserRecord{ID: "user-1", Username: "staff", Email: "staff@example.com", Nickname: "Staff"}

			if _, errorValue := service.createAttendanceEventForKind(context.Background(), database, userRecord, "user-token", attendanceKindClockIn, "team-1", "attendance-channel", "entry-post", testCase.clockIn.UTC(), service.attendanceLocationByID("office")); errorValue != nil {
				t.Fatal(errorValue)
			}
			if _, errorValue := service.createAttendanceEventForKind(context.Background(), database, userRecord, "user-token", attendanceKindClockOut, "team-1", "attendance-channel", "entry-post", testCase.clockOut.UTC(), attendanceLocation{}); errorValue != nil {
				t.Fatal(errorValue)
			}

			workMonthEvents, errorValue := service.readAttendanceEvents(context.Background(), testCase.workMonth, "")
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			nextMonthEvents, errorValue := service.readAttendanceEvents(context.Background(), testCase.nextMonth, "")
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if len(workMonthEvents) != 2 || len(nextMonthEvents) != 2 {
				t.Fatalf("work month events = %+v next month events = %+v", workMonthEvents, nextMonthEvents)
			}
			clockOutEvent, found := findAttendanceEventByKind(workMonthEvents, attendanceKindClockOut)
			if !found || clockOutEvent.LocalDate != testCase.clockOutDate {
				t.Fatalf("clock out event = %+v", clockOutEvent)
			}
		})
	}
}

func TestAttendanceClockInStillAllowedAfterForgottenClockOut(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	service.saveMattermostAttendanceChannelID("attendance-channel")
	service.saveMattermostAttendanceEntryPostID("entry-post")
	location, _ := service.workspaceTimeLocation()
	forgottenWorkMonth := time.Now().In(location).AddDate(0, 0, -1).Format("2006-01")
	currentWorkMonth := time.Now().In(location).Format("2006-01")
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
	moveLatestAttendanceEventToYesterdayAt(t, service, 9, 50)

	request := httptest.NewRequest(http.MethodPost, "/api/v4/posts", strings.NewReader(`{"channel_id":"attendance-channel","root_id":"entry-post","message":"출근"}`))
	request.Header.Set("Cookie", "MMAUTHTOKEN=session-token")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	events, errorValue := service.readAttendanceEvents(context.Background(), forgottenWorkMonth, "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if currentWorkMonth != forgottenWorkMonth {
		currentMonthEvents, errorValue := service.readAttendanceEvents(context.Background(), currentWorkMonth, "")
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		events = append(events, currentMonthEvents...)
	}
	clockInIDs := map[string]struct{}{}
	for _, event := range events {
		if event.Kind == attendanceKindClockIn && event.CanceledAt == "" {
			clockInIDs[event.ID] = struct{}{}
		}
	}
	if len(clockInIDs) != 2 {
		t.Fatalf("expected morning clock-in to stay allowed, events = %+v", events)
	}
}

func insertAttendanceClockInAt(t *testing.T, service *Service, occurredAt time.Time) {
	t.Helper()
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	location, timeZoneName := service.workspaceTimeLocation()
	localTime := occurredAt.In(location)
	event := attendanceEvent{
		ID:               randomHex(16),
		MattermostUserID: "user-1",
		Kind:             attendanceKindClockIn,
		OccurredAt:       occurredAt.UTC().Format(time.RFC3339Nano),
		LocalDate:        localTime.Format("2006-01-02"),
		LocalTime:        localTime.Format("15:04:05"),
		TimeZoneAtEvent:  timeZoneName,
		Source:           attendanceSourceMattermostButton,
		LocationID:       "office",
		LocationName:     "사무실",
	}
	if errorValue := service.insertAttendanceEvent(context.Background(), database, event); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestOvernightShiftKeepsBlockingAnotherClockIn(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	location, _ := service.workspaceTimeLocation()
	now := time.Date(2026, 7, 30, 0, 20, 0, 0, location).UTC()
	insertAttendanceClockInAt(t, service, time.Date(2026, 7, 29, 21, 22, 0, 0, location).UTC())
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()

	event, found, errorValue := service.latestAttendanceActionEvent(context.Background(), database, "user-1", attendanceKindClockIn, now)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found || event.Kind != attendanceKindClockIn {
		t.Fatalf("expected the open overnight clock-in to be found, found = %v event = %+v", found, event)
	}
	if !shouldIgnoreAttendanceAction(attendanceKindClockIn, event, found) {
		t.Fatal("expected another clock-in during the overnight shift to be ignored")
	}
}

func TestForgottenClockOutStopsBlockingTheNextClockIn(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	location, _ := service.workspaceTimeLocation()
	now := time.Date(2026, 7, 30, 9, 0, 0, 0, location).UTC()
	insertAttendanceClockInAt(t, service, time.Date(2026, 7, 29, 9, 30, 0, 0, location).UTC())
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()

	_, found, errorValue := service.latestAttendanceActionEvent(context.Background(), database, "user-1", attendanceKindClockIn, now)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if found {
		t.Fatal("expected a stale clock-in from the previous day to stop blocking a new clock-in")
	}
}
