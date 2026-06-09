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
	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockOut); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(*posts) != 2 || (*posts)[0].Message != "Clock in(Office)" || (*posts)[1].Message != "Clock out" {
		t.Fatalf("posts = %+v", *posts)
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
	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockOut); errorValue != nil {
		t.Fatal(errorValue)
	}
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
