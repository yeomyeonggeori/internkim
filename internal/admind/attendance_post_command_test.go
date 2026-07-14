package admind

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAttendanceClockButtonsKeepOnlyLatestClockInAndClockOutPosts(t *testing.T) {
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
	moveLatestAttendanceEventByDurationForTest(t, service, -time.Hour)
	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockOut); errorValue != nil {
		t.Fatal(errorValue)
	}
	moveLatestAttendanceEventByDurationForTest(t, service, -time.Hour)
	if _, errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockIn); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(*posts) != 3 {
		t.Fatalf("posts = %+v", *posts)
	}
	if !(*posts)[0].Deleted || (*posts)[1].Deleted || (*posts)[2].Deleted {
		t.Fatalf("deleted posts = %+v", *posts)
	}
	events, errorValue := service.readAttendanceEvents(context.Background(), time.Now().Format("2006-01"), "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != 3 {
		t.Fatalf("events = %+v", events)
	}
}

func TestAttendanceEntryCommentCreatesAttendanceEvent(t *testing.T) {
	service, posts := newAttendanceActionTestService(t)
	service.saveMattermostAttendanceChannelID("attendance-channel")
	service.saveMattermostAttendanceEntryPostID("entry-post")
	request := httptest.NewRequest(http.MethodPost, "/api/v4/posts", strings.NewReader(`{"channel_id":"attendance-channel","root_id":"entry-post","message":"출근"}`))
	request.Header.Set("Cookie", "MMAUTHTOKEN=session-token")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	if len(*posts) != 1 ||
		(*posts)[0].Deleted ||
		(*posts)[0].Message != "출근(사무실)" ||
		(*posts)[0].RootID != "entry-post" {
		t.Fatalf("posts = %+v", *posts)
	}
	events, errorValue := service.readAttendanceEvents(context.Background(), time.Now().Format("2006-01"), "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != 1 || events[0].Kind != attendanceKindClockIn || events[0].LocationName != "사무실" {
		t.Fatalf("events = %+v", events)
	}
}

func TestAttendanceEntryCommentClockInFormsUseSameAction(t *testing.T) {
	cases := []struct {
		name                 string
		message              string
		expectedResultPost   string
		expectedLocationID   string
		expectedLocationName string
	}{
		{name: "kind only", message: "출근", expectedResultPost: "출근(사무실)", expectedLocationID: "office", expectedLocationName: "사무실"},
		{name: "kind with location", message: "출근(사무실)", expectedResultPost: "출근(사무실)", expectedLocationID: "office", expectedLocationName: "사무실"},
		{name: "default location only", message: "사무실", expectedResultPost: "출근(사무실)", expectedLocationID: "office", expectedLocationName: "사무실"},
		{name: "custom location only", message: "재택", expectedResultPost: "출근(재택)", expectedLocationID: "home", expectedLocationName: "재택"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service, posts := newAttendanceActionTestService(t)
			service.saveMattermostAttendanceChannelID("attendance-channel")
			service.saveMattermostAttendanceEntryPostID("entry-post")
			if errorValue := service.writeAttendanceLocationsFile([]attendanceLocation{
				{ID: "office", Name: "사무실", Color: "#16a34a", IsDefault: true},
				{ID: "home", Name: "재택", Color: "#2563eb"},
			}); errorValue != nil {
				t.Fatal(errorValue)
			}
			request := httptest.NewRequest(http.MethodPost, "/api/v4/posts", strings.NewReader(`{"channel_id":"attendance-channel","root_id":"entry-post","message":"`+testCase.message+`"}`))
			request.Header.Set("Cookie", "MMAUTHTOKEN=session-token")
			response := httptest.NewRecorder()

			service.router().ServeHTTP(response, request)

			if response.Code != http.StatusCreated {
				t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
			}
			if len(*posts) != 1 ||
				(*posts)[0].Deleted ||
				(*posts)[0].Message != testCase.expectedResultPost ||
				(*posts)[0].RootID != "entry-post" {
				t.Fatalf("posts = %+v", *posts)
			}
			events, errorValue := service.readAttendanceEvents(context.Background(), time.Now().Format("2006-01"), "")
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if len(events) != 1 ||
				events[0].Kind != attendanceKindClockIn ||
				events[0].LocationID != testCase.expectedLocationID ||
				events[0].LocationName != testCase.expectedLocationName {
				t.Fatalf("events = %+v", events)
			}
		})
	}
}

func TestAttendanceChannelPostCreatesAttendanceEvent(t *testing.T) {
	service, posts := newAttendanceActionTestService(t)
	service.saveMattermostAttendanceChannelID("attendance-channel")
	service.saveMattermostAttendanceEntryPostID("entry-post")
	if errorValue := service.writeAttendanceLocationsFile([]attendanceLocation{
		{ID: "office", Name: "사무실", Color: "#16a34a", IsDefault: true},
		{ID: "home", Name: "재택", Color: "#2563eb"},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v4/posts", strings.NewReader(`{"channel_id":"attendance-channel","message":"출근(재택)"}`))
	request.Header.Set("Cookie", "MMAUTHTOKEN=session-token")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	if len(*posts) != 2 ||
		!(*posts)[0].Deleted ||
		(*posts)[1].Message != "출근(재택)" ||
		(*posts)[1].RootID != "entry-post" {
		t.Fatalf("posts = %+v", *posts)
	}
	events, errorValue := service.readAttendanceEvents(context.Background(), time.Now().Format("2006-01"), "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != 1 || events[0].Kind != attendanceKindClockIn || events[0].LocationName != "재택" {
		t.Fatalf("events = %+v", events)
	}
}

func TestAttendanceChannelPostCreatesEventWhenExistingResultPostIsMissing(t *testing.T) {
	service, posts := newAttendanceActionTestService(t)
	service.saveMattermostAttendanceChannelID("attendance-channel")
	service.saveMattermostAttendanceEntryPostID("entry-post")
	insertGhostClockInEventForTest(t, service, "missing-post")
	request := httptest.NewRequest(http.MethodPost, "/api/v4/posts", strings.NewReader(`{"channel_id":"attendance-channel","message":"출근"}`))
	request.Header.Set("Cookie", "MMAUTHTOKEN=session-token")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
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

func TestAttendanceEntryCommentDeletesDuplicateCommand(t *testing.T) {
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
	request := httptest.NewRequest(http.MethodPost, "/api/v4/posts", strings.NewReader(`{"channel_id":"attendance-channel","root_id":"entry-post","message":"출근"}`))
	request.Header.Set("Cookie", "MMAUTHTOKEN=session-token")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	if len(*posts) != 2 ||
		(*posts)[0].Deleted ||
		!(*posts)[1].Deleted ||
		(*posts)[1].Message != "출근" {
		t.Fatalf("posts = %+v", *posts)
	}
	events, errorValue := service.readAttendanceEvents(context.Background(), time.Now().Format("2006-01"), "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != 1 || events[0].Kind != attendanceKindClockIn {
		t.Fatalf("events = %+v", events)
	}
}

func TestAttendanceEntryCommentAcceptsEnglishAliases(t *testing.T) {
	service, posts := newAttendanceActionTestService(t)
	service.saveMattermostAttendanceChannelID("attendance-channel")
	service.saveMattermostAttendanceEntryPostID("entry-post")
	for _, message := range []string{"Clock in", "Clock out"} {
		request := httptest.NewRequest(http.MethodPost, "/api/v4/posts", strings.NewReader(`{"channel_id":"attendance-channel","root_id":"entry-post","message":"`+message+`"}`))
		request.Header.Set("Cookie", "MMAUTHTOKEN=session-token")
		response := httptest.NewRecorder()
		service.router().ServeHTTP(response, request)
		if response.Code != http.StatusCreated {
			t.Fatalf("%s status = %d body = %s", message, response.Code, response.Body.String())
		}
		if message == "Clock in" {
			moveLatestAttendanceEventByDurationForTest(t, service, -time.Hour)
		}
	}

	if len(*posts) != 2 ||
		(*posts)[0].Deleted ||
		(*posts)[0].Message != "출근(사무실)" ||
		(*posts)[1].Deleted ||
		(*posts)[1].Message != "퇴근" {
		t.Fatalf("posts = %+v", *posts)
	}
}

func TestAttendanceEntryCommentImmediateClockOutCancelsAccidentalClockIn(t *testing.T) {
	service, posts := newAttendanceActionTestService(t)
	service.saveMattermostAttendanceChannelID("attendance-channel")
	service.saveMattermostAttendanceEntryPostID("entry-post")

	postAttendanceEntryCommentForTest(t, service, "출근")
	postAttendanceEntryCommentForTest(t, service, "퇴근")

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
	if len(*posts) != 3 || !(*posts)[1].Deleted || (*posts)[2].RootID != "entry-post" {
		t.Fatalf("posts = %+v", *posts)
	}
}

func TestAttendanceEntryCommentSameLocationResumeCancelsAccidentalClockOut(t *testing.T) {
	service, posts := newAttendanceActionTestService(t)
	service.saveMattermostAttendanceChannelID("attendance-channel")
	service.saveMattermostAttendanceEntryPostID("entry-post")

	postAttendanceEntryCommentForTest(t, service, "출근")
	moveLatestAttendanceEventByDurationForTest(t, service, -time.Hour)
	postAttendanceEntryCommentForTest(t, service, "퇴근")
	postAttendanceEntryCommentForTest(t, service, "출근")

	events, errorValue := service.readAttendanceEvents(context.Background(), time.Now().Format("2006-01"), "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	activeEvents := activeAttendanceEventsForTest(events)
	if len(activeEvents) != 1 || activeEvents[0].Kind != attendanceKindClockIn || activeEvents[0].LocationID != "office" {
		t.Fatalf("active events = %+v all events = %+v", activeEvents, events)
	}
	clockOutEvent, found := findAttendanceEventByKind(events, attendanceKindClockOut)
	if !found || clockOutEvent.CancelReason != attendanceSameLocationResumeCancelReason {
		t.Fatalf("events = %+v", events)
	}
	if len(*posts) != 4 || !(*posts)[2].Deleted || (*posts)[3].RootID != "entry-post" {
		t.Fatalf("posts = %+v", *posts)
	}
}

func TestAttendanceEntryCommentOvernightClockOutUsesActualEventDate(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	service.saveMattermostAttendanceChannelID("attendance-channel")
	service.saveMattermostAttendanceEntryPostID("entry-post")
	location, _ := service.workspaceTimeLocation()
	expectedWorkDate := time.Now().In(location).AddDate(0, 0, -1).Format("2006-01-02")
	expectedClockOutDate := time.Now().In(location).Format("2006-01-02")

	postAttendanceEntryCommentForTest(t, service, "출근")
	moveLatestAttendanceEventToYesterday(t, service)
	postAttendanceEntryCommentForTest(t, service, "퇴근")

	events, errorValue := service.readAttendanceEvents(context.Background(), expectedWorkDate[:7], "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	clockOutEvent, found := findAttendanceEventByKind(events, attendanceKindClockOut)
	if !found || clockOutEvent.LocalDate != expectedClockOutDate {
		t.Fatalf("clock out event = %+v all events = %+v", clockOutEvent, events)
	}
}

func TestAttendanceEntryCommentUpdatesCurrentEventTime(t *testing.T) {
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
	request := httptest.NewRequest(http.MethodPost, "/api/v4/posts", strings.NewReader(`{"channel_id":"attendance-channel","root_id":"entry-post","message":"9:00"}`))
	request.Header.Set("Cookie", "MMAUTHTOKEN=session-token")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	if len(*posts) != 2 || (*posts)[0].Message != "출근(사무실) 09:00" || !(*posts)[1].Deleted {
		t.Fatalf("posts = %+v", *posts)
	}
	events, errorValue := service.readAttendanceEvents(context.Background(), time.Now().Format("2006-01"), "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != 1 || events[0].LocalTime != "09:00:00" {
		t.Fatalf("events = %+v", events)
	}
}

func TestAttendanceEntryCommentUpdatesClockOutTime(t *testing.T) {
	service, posts := newAttendanceActionTestService(t)
	service.saveMattermostAttendanceChannelID("attendance-channel")
	service.saveMattermostAttendanceEntryPostID("entry-post")
	location, _ := service.workspaceTimeLocation()
	eventDate := time.Now().In(location)
	clockInTime := time.Date(eventDate.Year(), eventDate.Month(), 15, 9, 0, 0, 0, location)
	clockOutTime := time.Date(eventDate.Year(), eventDate.Month(), 15, 18, 0, 0, 0, location)
	userRecord := mattermostUserRecord{ID: "user-1", Username: "staff", Email: "staff@example.com"}
	clockInEvent := service.createAttendanceEvent(userRecord, attendanceKindClockIn, clockInTime.UTC(), "team-1", "attendance-channel", "entry-post", "attendance-post-1", service.attendanceLocationByID("office"))
	clockOutEvent := service.createAttendanceEvent(userRecord, attendanceKindClockOut, clockOutTime.UTC(), "team-1", "attendance-channel", "entry-post", "attendance-post-2", attendanceLocation{})
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if errorValue := service.insertAttendanceEvent(context.Background(), database, clockInEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.insertAttendanceEvent(context.Background(), database, clockOutEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	*posts = append(*posts, attendanceActionPost{ID: "attendance-post-2", Message: "퇴근", RootID: "entry-post"})
	request := httptest.NewRequest(http.MethodPost, "/api/v4/posts", strings.NewReader(`{"channel_id":"attendance-channel","root_id":"entry-post","message":"18:30"}`))
	request.Header.Set("Cookie", "MMAUTHTOKEN=session-token")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	if len(*posts) != 2 || (*posts)[0].Message != "퇴근 18:30" || !(*posts)[1].Deleted {
		t.Fatalf("posts = %+v", *posts)
	}
	events, errorValue := service.readAttendanceEvents(context.Background(), eventDate.Format("2006-01"), "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	updatedClockOutEvent, found := findAttendanceEventByKind(events, attendanceKindClockOut)
	if len(events) != 2 || !found || updatedClockOutEvent.LocalTime != "18:30:00" {
		t.Fatalf("events = %+v", events)
	}
}

func TestAttendanceTimeUpdateUsesPreviousDayWhenTimeIsAfterCommandPost(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	if errorValue := service.writeWorkspaceSettingsFile(workspaceSettings{Language: workspaceLanguageKorean, TimeZone: "Asia/Seoul"}); errorValue != nil {
		t.Fatal(errorValue)
	}
	location, _ := time.LoadLocation("Asia/Seoul")
	event := attendanceEvent{LocalDate: "2026-06-10"}
	commandPostCreatedAt := time.Date(2026, 6, 10, 0, 30, 0, 0, location).UTC()

	localTime, errorValue := service.attendanceLocalTimeForEvent(event, "23:30", commandPostCreatedAt)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if localTime.Format("2006-01-02 15:04") != "2026-06-09 23:30" {
		t.Fatalf("local time = %s", localTime.Format("2006-01-02 15:04"))
	}
}

func findAttendanceEventByKind(events []attendanceEvent, kind string) (attendanceEvent, bool) {
	for _, event := range events {
		if event.Kind == kind {
			return event, true
		}
	}
	return attendanceEvent{}, false
}

func postAttendanceEntryCommentForTest(t *testing.T, service *Service, message string) {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/v4/posts", strings.NewReader(`{"channel_id":"attendance-channel","root_id":"entry-post","message":"`+message+`"}`))
	request.Header.Set("Cookie", "MMAUTHTOKEN=session-token")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("%s status = %d body = %s", message, response.Code, response.Body.String())
	}
}

func TestAttendanceEntryCommentDeletesUnknownMessage(t *testing.T) {
	service, posts := newAttendanceActionTestService(t)
	service.saveMattermostAttendanceChannelID("attendance-channel")
	service.saveMattermostAttendanceEntryPostID("entry-post")
	request := httptest.NewRequest(http.MethodPost, "/api/v4/posts", strings.NewReader(`{"channel_id":"attendance-channel","root_id":"entry-post","message":"hello"}`))
	request.Header.Set("Cookie", "MMAUTHTOKEN=session-token")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	if len(*posts) != 1 || !(*posts)[0].Deleted {
		t.Fatalf("posts = %+v", *posts)
	}
}

func TestAttendanceCleanupKeepsLatestClockInAndClockOutPosts(t *testing.T) {
	stateDirectory := t.TempDir()
	service := NewService(Configuration{
		StateDirectory:         stateDirectory,
		AttendanceDatabasePath: filepath.Join(stateDirectory, "attendance.sqlite"),
		MattermostBaseURL:      "http://mattermost.local",
	})
	deletedPostIDs := []string{}
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodDelete && strings.HasPrefix(request.URL.Path, "/api/v4/posts/") {
			deletedPostIDs = append(deletedPostIDs, strings.TrimPrefix(request.URL.Path, "/api/v4/posts/"))
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	baseTime := time.Now().UTC().Add(-4 * time.Hour)
	firstUser := mattermostUserRecord{ID: "user-1", Username: "staff", Email: "staff@example.com"}
	secondUser := mattermostUserRecord{ID: "user-2", Username: "other", Email: "other@example.com"}
	events := []attendanceEvent{
		service.createAttendanceEvent(firstUser, attendanceKindClockIn, baseTime, "team-1", "attendance-channel", "entry-post", "old-in", attendanceLocation{}),
		service.createAttendanceEvent(firstUser, attendanceKindClockOut, baseTime.Add(time.Hour), "team-1", "attendance-channel", "entry-post", "old-out", attendanceLocation{}),
		service.createAttendanceEvent(firstUser, attendanceKindClockIn, baseTime.Add(2*time.Hour), "team-1", "attendance-channel", "entry-post", "new-in", attendanceLocation{}),
		service.createAttendanceEvent(firstUser, attendanceKindClockOut, baseTime.Add(3*time.Hour), "team-1", "attendance-channel", "entry-post", "new-out", attendanceLocation{}),
		service.createAttendanceEvent(secondUser, attendanceKindClockIn, baseTime.Add(30*time.Minute), "team-1", "attendance-channel", "entry-post", "other-in", attendanceLocation{}),
		service.createAttendanceEvent(secondUser, attendanceKindClockOut, baseTime.Add(90*time.Minute), "team-1", "attendance-channel", "entry-post", "other-out", attendanceLocation{}),
	}
	for _, event := range events {
		if errorValue := service.insertAttendanceEvent(context.Background(), database, event); errorValue != nil {
			t.Fatal(errorValue)
		}
	}

	if errorValue := service.cleanupMattermostAttendanceResultPosts(context.Background(), "admin-token"); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(deletedPostIDs) != 2 || !containsString(deletedPostIDs, "old-in") || !containsString(deletedPostIDs, "old-out") {
		t.Fatalf("deleted post ids = %+v", deletedPostIDs)
	}
	for _, postID := range []string{"new-in", "new-out", "other-in", "other-out"} {
		if containsString(deletedPostIDs, postID) {
			t.Fatalf("deleted latest per-user post %q: %+v", postID, deletedPostIDs)
		}
	}
	remainingEvents, errorValue := service.readAttendanceEvents(context.Background(), time.Now().Format("2006-01"), "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(remainingEvents) != 6 {
		t.Fatalf("events = %+v", remainingEvents)
	}
}
