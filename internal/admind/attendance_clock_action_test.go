package admind

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestAttendanceClockButtonsPostAsUserAndIgnoreInvalidState(t *testing.T) {
	service, posts := newAttendanceActionTestService(t)
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
	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockIn); errorValue != nil {
		t.Fatal(errorValue)
	}
	moveLatestAttendanceEventByDurationForTest(t, service, -time.Hour)
	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockOut); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockOut); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(*posts) != 2 || (*posts)[0].Message != "출근(사무실)" || (*posts)[1].Message != "퇴근" {
		t.Fatalf("posts = %+v", *posts)
	}
	for _, post := range *posts {
		if post.RootID != "entry-post" {
			t.Fatalf("post root = %+v", post)
		}
	}
	events, errorValue := service.readAttendanceEvents(context.Background(), time.Now().Format("2006-01"), "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != 2 || events[0].Kind != attendanceKindClockOut || events[1].Kind != attendanceKindClockIn {
		t.Fatalf("events = %+v", events)
	}
}

func TestAttendanceClockMessagesUseAdminLocale(t *testing.T) {
	service, posts := newAttendanceActionTestService(t)
	setAttendanceActionTestLocale(t, service, "en")
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
	moveLatestAttendanceEventByDurationForTest(t, service, -time.Hour)
	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockOut); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(*posts) != 2 || (*posts)[0].Message != "Clock in(Office)" || (*posts)[1].Message != "Clock out" {
		t.Fatalf("posts = %+v", *posts)
	}
}

func TestAttendanceClockInFromMattermostEndsActiveLeaveEarly(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	location := service.workspaceTimeZone().location
	currentLocalDate := time.Now().In(location)
	localNow := time.Date(
		currentLocalDate.Year(),
		currentLocalDate.Month(),
		currentLocalDate.Day(),
		12,
		0,
		0,
		0,
		location,
	)
	now := localNow.UTC()
	leaveStart := localNow.Add(-30 * time.Minute)
	leaveEnd := localNow.Add(90 * time.Minute)
	employee := attendanceLeaveEmployee{
		Email:  "staff@example.com",
		UserID: "user-1",
	}
	leaveType, found := attendanceLeaveTypeByID(defaultAttendanceLeavePolicy(), "sick")
	if !found {
		t.Fatal("sick leave type is missing")
	}
	record, errorValue := service.createAttendanceLeaveRequest(
		t.Context(),
		employee,
		attendanceLeaveRequestInput{
			LeaveTypeID:   leaveType.ID,
			Unit:          "quarterDay",
			StartDate:     localNow.Format(time.DateOnly),
			PartialPeriod: attendanceLeavePartialPeriodCustom,
			StartTime:     leaveStart.Format("15:04"),
		},
		attendanceLeaveRequestPreview{
			Occurrences: []attendanceLeaveRequestOccurrence{{
				Date:               localNow.Format(time.DateOnly),
				StartTime:          leaveStart.Format("15:04"),
				EndTime:            leaveEnd.Format("15:04"),
				DeductionMilliDays: 250,
			}},
			TotalDeductionMilliDays: 250,
		},
		leaveType,
		defaultAttendanceLeavePolicy(),
		nil,
		now.Add(-time.Hour),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.decideAttendanceLeaveRequest(
		t.Context(),
		record.ID,
		"admin@example.com",
		attendanceLeaveApprovalInput{Action: attendanceLeaveApprovalActionApprove},
		now.Add(-45*time.Minute),
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openAttendanceDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	clockIn := service.createAttendanceEvent(
		mattermostUserRecord{
			ID:       "user-1",
			Username: "staff",
			Email:    "staff@example.com",
		},
		attendanceKindClockIn,
		leaveStart.Add(-time.Hour),
		"team-1",
		"attendance-channel",
		"entry-post",
		"clock-in-post",
		attendanceLocation{ID: "office", Name: "사무실"},
	)
	if errorValue := service.insertAttendanceEvent(t.Context(), database, clockIn); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	database.Close()

	payload := mattermostInteractivePayload{
		UserID:    "user-1",
		PostID:    "entry-post",
		ChannelID: "attendance-channel",
		TeamID:    "team-1",
		Context: mattermostInteractiveContext{
			Action: attendanceClockInAction,
			Token:  service.ensureMattermostInteractiveActionToken(),
		},
	}
	if _, errorValue := service.recordAttendanceFromMattermostAt(
		t.Context(),
		payload,
		attendanceKindClockIn,
		now,
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	activeLeave, errorValue := service.readActiveAttendanceLeave(
		t.Context(),
		"staff@example.com",
		now,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if activeLeave != nil {
		t.Fatalf("active leave = %+v", activeLeave)
	}
	events, errorValue := service.readAttendanceEvents(
		t.Context(),
		localNow.Format("2006-01"),
		"staff@example.com",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	automaticClockOutFound := false
	mattermostClockInCount := 0
	for _, event := range events {
		if event.Source == attendanceSourceApprovedLeave && event.Kind == attendanceKindClockOut {
			automaticClockOutFound = true
		}
		if event.Source == attendanceSourceMattermostButton && event.Kind == attendanceKindClockIn {
			mattermostClockInCount++
		}
	}
	if len(events) != 3 || !automaticClockOutFound || mattermostClockInCount != 2 {
		t.Fatalf("mattermost early return events = %+v", events)
	}
}

func TestAttendanceClockButtonUsesStoredEntryPostID(t *testing.T) {
	service, posts := newAttendanceActionTestService(t)
	service.saveMattermostAttendanceEntryPostID("entry-post")
	payload := mattermostInteractivePayload{
		UserID:    "user-1",
		PostID:    "duplicate-entry-post",
		ChannelID: "attendance-channel",
		TeamID:    "team-1",
		Context:   mattermostInteractiveContext{Action: attendanceClockInAction, Token: service.ensureMattermostInteractiveActionToken()},
	}

	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockIn); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(*posts) != 1 || (*posts)[0].RootID != "entry-post" {
		t.Fatalf("posts = %+v", *posts)
	}
}

func TestAttendanceClockInMultipleLocationsPostsLocationName(t *testing.T) {
	service, posts := newAttendanceActionTestService(t)
	if errorValue := service.writeAttendanceLocationsFile([]attendanceLocation{
		{ID: "office", Name: "사무실", Color: "#16a34a", IsDefault: true},
		{ID: "home", Name: "재택", Color: "#2563eb"},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	payload := mattermostInteractivePayload{
		UserID:    "user-1",
		PostID:    "entry-post",
		ChannelID: "attendance-channel",
		TeamID:    "team-1",
		Context:   mattermostInteractiveContext{Action: attendanceClockInAction, Token: service.ensureMattermostInteractiveActionToken(), LocationID: "office"},
	}

	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockIn); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(*posts) != 1 || (*posts)[0].Message != "출근(사무실)" {
		t.Fatalf("posts = %+v", *posts)
	}
}

func TestAttendanceClockInDifferentLocationCreatesAnotherEvent(t *testing.T) {
	service, posts := newAttendanceActionTestService(t)
	if errorValue := service.writeAttendanceLocationsFile([]attendanceLocation{
		{ID: "office", Name: "사무실", Color: "#16a34a", IsDefault: true},
		{ID: "home", Name: "재택", Color: "#2563eb"},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	payload := mattermostInteractivePayload{
		UserID:    "user-1",
		PostID:    "entry-post",
		ChannelID: "attendance-channel",
		TeamID:    "team-1",
		Context:   mattermostInteractiveContext{Action: attendanceClockInAction, Token: service.ensureMattermostInteractiveActionToken(), LocationID: "office"},
	}

	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockIn); errorValue != nil {
		t.Fatal(errorValue)
	}
	payload.Context.LocationID = "home"
	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockIn); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(*posts) != 2 || !(*posts)[0].Deleted || (*posts)[1].Message != "출근(재택)" {
		t.Fatalf("posts = %+v", *posts)
	}
	events, errorValue := service.readAttendanceEvents(context.Background(), time.Now().Format("2006-01"), "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != 2 ||
		events[0].Kind != attendanceKindClockIn ||
		events[0].LocationName != "재택" ||
		events[1].Kind != attendanceKindClockIn ||
		events[1].LocationName != "사무실" {
		t.Fatalf("events = %+v", events)
	}
}

func TestAttendanceClockInButtonCreatesEventAfterClockOut(t *testing.T) {
	service, posts := newAttendanceActionTestService(t)
	if errorValue := service.writeAttendanceLocationsFile([]attendanceLocation{
		{ID: "office", Name: "사무실", Color: "#16a34a", IsDefault: true},
		{ID: "home", Name: "재택", Color: "#2563eb"},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	payload := mattermostInteractivePayload{
		UserID:    "user-1",
		PostID:    "entry-post",
		ChannelID: "attendance-channel",
		TeamID:    "team-1",
		Context:   mattermostInteractiveContext{Action: attendanceClockInAction, Token: service.ensureMattermostInteractiveActionToken(), LocationID: "home"},
	}

	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockIn); errorValue != nil {
		t.Fatal(errorValue)
	}
	moveLatestAttendanceEventByDurationForTest(t, service, -time.Hour)
	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockOut); errorValue != nil {
		t.Fatal(errorValue)
	}
	moveLatestAttendanceEventByDurationForTest(t, service, -time.Hour)
	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockIn); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(*posts) != 3 ||
		!(*posts)[0].Deleted ||
		(*posts)[1].Message != "퇴근" ||
		(*posts)[2].Message != "출근(재택)" {
		t.Fatalf("posts = %+v", *posts)
	}
	events, errorValue := service.readAttendanceEvents(context.Background(), time.Now().Format("2006-01"), "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != 3 || events[0].Kind != attendanceKindClockIn || events[0].LocationName != "재택" {
		t.Fatalf("events = %+v", events)
	}
}

func TestAttendanceImmediateClockOutCancelsAccidentalClockIn(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	payload := mattermostInteractivePayload{
		UserID:    "user-1",
		PostID:    "entry-post",
		ChannelID: "attendance-channel",
		TeamID:    "team-1",
		Context:   mattermostInteractiveContext{Action: attendanceClockInAction, Token: service.ensureMattermostInteractiveActionToken(), LocationID: "office"},
	}

	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockIn); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockOut); errorValue != nil {
		t.Fatal(errorValue)
	}

	events, errorValue := service.readAttendanceEvents(context.Background(), time.Now().Format("2006-01"), "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	activeEvents := activeAttendanceEventsForTest(events)
	if len(activeEvents) != 0 {
		t.Fatalf("active events = %+v all events = %+v", activeEvents, events)
	}
	if len(events) != 1 || events[0].Kind != attendanceKindClockIn || events[0].CancelReason != attendanceAccidentalShortSegmentCancelReason {
		t.Fatalf("events = %+v", events)
	}
}

func TestAttendanceSameLocationResumeCancelsAccidentalClockOut(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	payload := mattermostInteractivePayload{
		UserID:    "user-1",
		PostID:    "entry-post",
		ChannelID: "attendance-channel",
		TeamID:    "team-1",
		Context:   mattermostInteractiveContext{Action: attendanceClockInAction, Token: service.ensureMattermostInteractiveActionToken(), LocationID: "office"},
	}

	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockIn); errorValue != nil {
		t.Fatal(errorValue)
	}
	moveLatestAttendanceEventByDurationForTest(t, service, -time.Hour)
	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockOut); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockIn); errorValue != nil {
		t.Fatal(errorValue)
	}

	events, errorValue := service.readAttendanceEvents(context.Background(), time.Now().Format("2006-01"), "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	activeEvents := activeAttendanceEventsForTest(events)
	if len(activeEvents) != 1 || activeEvents[0].Kind != attendanceKindClockIn || activeEvents[0].LocationID != "office" {
		t.Fatalf("active events = %+v all events = %+v", activeEvents, events)
	}
	if len(events) != 2 || events[0].Kind != attendanceKindClockOut || events[0].CancelReason != attendanceSameLocationResumeCancelReason {
		t.Fatalf("events = %+v", events)
	}
}

func TestAttendanceDifferentLocationResumeKeepsSeparateSegment(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	if errorValue := service.writeAttendanceLocationsFile([]attendanceLocation{
		{ID: "office", Name: "사무실", Color: "#16a34a", IsDefault: true},
		{ID: "home", Name: "재택", Color: "#2563eb"},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	payload := mattermostInteractivePayload{
		UserID:    "user-1",
		PostID:    "entry-post",
		ChannelID: "attendance-channel",
		TeamID:    "team-1",
		Context:   mattermostInteractiveContext{Action: attendanceClockInAction, Token: service.ensureMattermostInteractiveActionToken(), LocationID: "office"},
	}

	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockIn); errorValue != nil {
		t.Fatal(errorValue)
	}
	moveLatestAttendanceEventByDurationForTest(t, service, -time.Hour)
	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockOut); errorValue != nil {
		t.Fatal(errorValue)
	}
	payload.Context.LocationID = "home"
	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockIn); errorValue != nil {
		t.Fatal(errorValue)
	}

	events, errorValue := service.readAttendanceEvents(context.Background(), time.Now().Format("2006-01"), "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	activeEvents := activeAttendanceEventsForTest(events)
	if len(activeEvents) != 3 || activeEvents[0].LocationID != "home" || activeEvents[1].Kind != attendanceKindClockOut || activeEvents[2].LocationID != "office" {
		t.Fatalf("active events = %+v all events = %+v", activeEvents, events)
	}
}

func TestAttendanceClockInButtonCreatesEventWhenExistingResultPostIsMissing(t *testing.T) {
	service, posts := newAttendanceActionTestService(t)
	insertGhostClockInEventForTest(t, service, "missing-post")
	payload := mattermostInteractivePayload{
		UserID:    "user-1",
		PostID:    "entry-post",
		ChannelID: "attendance-channel",
		TeamID:    "team-1",
		Context:   mattermostInteractiveContext{Action: attendanceClockInAction, Token: service.ensureMattermostInteractiveActionToken(), LocationID: "office"},
	}

	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockIn); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(*posts) != 1 || (*posts)[0].Message != "출근(사무실)" || (*posts)[0].RootID != "entry-post" {
		t.Fatalf("posts = %+v", *posts)
	}
	events, errorValue := service.readAttendanceEvents(context.Background(), time.Now().Format("2006-01"), "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != 2 || events[0].Kind != attendanceKindClockIn || events[0].ResultPostID != "attendance-post-1" {
		t.Fatalf("events = %+v", events)
	}
}

func TestAttendanceClockInButtonCreatesEventWhenExistingResultPostIsInStaleRoot(t *testing.T) {
	service, posts := newAttendanceActionTestService(t)
	*posts = append(*posts, attendanceActionPost{ID: "old-post", Message: "출근(사무실)", RootID: "old-entry-post"})
	insertGhostClockInEventForTest(t, service, "old-post")
	payload := mattermostInteractivePayload{
		UserID:    "user-1",
		PostID:    "entry-post",
		ChannelID: "attendance-channel",
		TeamID:    "team-1",
		Context:   mattermostInteractiveContext{Action: attendanceClockInAction, Token: service.ensureMattermostInteractiveActionToken(), LocationID: "office"},
	}

	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockIn); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(*posts) != 2 ||
		!(*posts)[0].Deleted ||
		(*posts)[1].Message != "출근(사무실)" ||
		(*posts)[1].RootID != "entry-post" {
		t.Fatalf("posts = %+v", *posts)
	}
	events, errorValue := service.readAttendanceEvents(context.Background(), time.Now().Format("2006-01"), "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != 2 || events[0].Kind != attendanceKindClockIn || events[0].ResultPostID != "attendance-post-2" {
		t.Fatalf("events = %+v", events)
	}
}

func TestAttendanceRepeatedClickCancelsEvent(t *testing.T) {
	service, posts := newAttendanceActionTestService(t)
	payload := mattermostInteractivePayload{
		UserID:    "user-1",
		PostID:    "entry-post",
		ChannelID: "attendance-channel",
		TeamID:    "team-1",
		Context:   mattermostInteractiveContext{Action: "attendance.toggle", Token: service.ensureMattermostInteractiveActionToken()},
	}

	if _, errorValue := service.toggleAttendanceFromMattermost(context.Background(), payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.toggleAttendanceFromMattermost(context.Background(), payload); errorValue != nil && errorValue != errAttendanceDuplicateIgnored {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.toggleAttendanceFromMattermost(context.Background(), payload); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(*posts) != 2 || (*posts)[1].Message != "출근(사무실) 취소" || (*posts)[1].RootID != "entry-post" {
		t.Fatalf("posts = %+v", *posts)
	}
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	event, found, errorValue := service.latestActiveAttendanceEvent(context.Background(), database, "user-1")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if found {
		t.Fatalf("expected no active event, got %+v", event)
	}
}

func TestAttendanceCancelMessageUsesAdminLocale(t *testing.T) {
	service, posts := newAttendanceActionTestService(t)
	setAttendanceActionTestLocale(t, service, "en")
	payload := mattermostInteractivePayload{
		UserID:    "user-1",
		PostID:    "entry-post",
		ChannelID: "attendance-channel",
		TeamID:    "team-1",
		Context:   mattermostInteractiveContext{Action: "attendance.toggle", Token: service.ensureMattermostInteractiveActionToken()},
	}

	if _, errorValue := service.toggleAttendanceFromMattermost(context.Background(), payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.toggleAttendanceFromMattermost(context.Background(), payload); errorValue != nil && errorValue != errAttendanceDuplicateIgnored {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.toggleAttendanceFromMattermost(context.Background(), payload); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(*posts) != 2 || (*posts)[1].Message != "Clock in(Office) canceled" || (*posts)[1].RootID != "entry-post" {
		t.Fatalf("posts = %+v", *posts)
	}
}

func setAttendanceActionTestLocale(t *testing.T, service *Service, locale string) {
	t.Helper()
	if errorValue := os.WriteFile(service.adminLocalePath(), []byte(locale), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func moveLatestAttendanceEventByDurationForTest(t *testing.T, service *Service, duration time.Duration) {
	t.Helper()
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	location, _ := service.workspaceTimeLocation()
	localTime := time.Now().In(location).Add(duration)
	if _, errorValue := database.ExecContext(context.Background(), `
UPDATE attendance_events
SET local_date = ?, occurred_at = ?, local_time = ?
WHERE id = (SELECT id FROM attendance_events ORDER BY occurred_at DESC LIMIT 1)`,
		localTime.Format("2006-01-02"),
		localTime.UTC().Format(time.RFC3339Nano),
		localTime.Format("15:04:05"),
	); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func activeAttendanceEventsForTest(events []attendanceEvent) []attendanceEvent {
	activeEvents := []attendanceEvent{}
	for _, event := range events {
		if event.CanceledAt == "" {
			activeEvents = append(activeEvents, event)
		}
	}
	return activeEvents
}
