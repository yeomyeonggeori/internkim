package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestAttendanceSummaryIncludesRegisteredMembersWithoutEvents(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	service.Configuration.APIBaseURL = "http://admin.local"
	service.Configuration.FleetIDPath = writeTestFile(t, "fleet-1")
	service.Configuration.FleetSecretPath = writeTestFile(t, "secret-1")
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodGet && request.URL.String() == "http://admin.local/api/users?fleet_id=fleet-1":
			return jsonResponse(http.StatusOK, `{"records":[{"email":"kim@example.com","name":"김철수","mattermostUsername":"kim","status":"active"},{"email":"park@example.com","name":"박지민","mattermostUsername":"park","status":"active"}]}`, nil), nil
		case request.Method == http.MethodGet && request.URL.Path == "/admin/api/policy":
			return jsonResponse(http.StatusOK, `{"people":[]}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/attendance/api/summary?month=2026-06", nil)
	request.Header.Set("X-Forwarded-Email", "kim@example.com")

	service.handleAttendance(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var response attendanceSummaryResponse
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &response); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(response.Members) != 2 {
		t.Fatalf("members = %+v", response.Members)
	}
	if response.Members[0].Email != "kim@example.com" || response.Members[1].Email != "park@example.com" {
		t.Fatalf("members = %+v", response.Members)
	}
	if len(response.Events) != 0 {
		t.Fatalf("events = %+v", response.Events)
	}
}

func TestAttendanceSummaryFallsBackWhenRegisteredMembersLookupFails(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	service.Configuration.APIBaseURL = "http://admin.local"
	service.Configuration.FleetIDPath = writeTestFile(t, "fleet-1")
	service.Configuration.FleetSecretPath = writeTestFile(t, "secret-1")
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodGet && request.URL.String() == "http://admin.local/api/users?fleet_id=fleet-1":
			return jsonResponse(http.StatusBadGateway, `{"error":"upstream unavailable"}`, nil), nil
		case request.Method == http.MethodGet && request.URL.Path == "/admin/api/policy":
			return jsonResponse(http.StatusOK, `{"people":[{"displayName":"김철수","emails":["kim@example.com"]}]}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/attendance/api/summary?month=2026-06", nil)
	request.Header.Set("X-Forwarded-Email", "kim@example.com")

	service.handleAttendance(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var response attendanceSummaryResponse
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &response); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(response.Members) != 0 {
		t.Fatalf("members = %+v", response.Members)
	}
	if !strings.Contains(recorder.Body.String(), `"members":[]`) {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestAttendanceSummaryLeavesMembersEmptyWithoutRegisteredMembersCredentials(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/admin/api/policy":
			return jsonResponse(http.StatusOK, `{"people":[{"displayName":"김철수","emails":["kim@example.com"]}]}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/attendance/api/summary?month=2026-06", nil)
	request.Header.Set("X-Forwarded-Email", "kim@example.com")

	service.handleAttendance(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var response attendanceSummaryResponse
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &response); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(response.Members) != 0 {
		t.Fatalf("members = %+v", response.Members)
	}
	if !strings.Contains(recorder.Body.String(), `"members":[]`) {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestAttendanceStatusIgnoresFutureEvents(t *testing.T) {
	now := time.Date(2026, 6, 10, 8, 0, 0, 0, time.UTC)
	events := []attendanceEvent{
		{Kind: attendanceKindClockIn, LocalDate: "2026-06-10", OccurredAt: now.Add(15 * time.Hour).Format(time.RFC3339Nano)},
		{Kind: attendanceKindClockOut, LocalDate: "2026-06-10", OccurredAt: now.Add(-30 * time.Minute).Format(time.RFC3339Nano)},
	}

	status := attendanceStatusForEvents(events, "2026-06-10", now)

	if status != "clocked_out" {
		t.Fatalf("status = %q", status)
	}
}

func TestAttendanceClockInIgnoresFutureClockInState(t *testing.T) {
	service, posts := newAttendanceActionTestService(t)
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	now := time.Now().UTC()
	userRecord := mattermostUserRecord{ID: "user-1", Username: "staff", Email: "staff@example.com", Nickname: "Staff"}
	futureEvent := service.createAttendanceEvent(userRecord, attendanceKindClockIn, now.Add(12*time.Hour), "team-1", "attendance-channel", "entry-post", "future-post", service.attendanceLocationByID("office"))
	location, _ := service.workspaceTimeLocation()
	futureEvent.LocalDate = now.In(location).Format("2006-01-02")
	if errorValue := service.insertAttendanceEvent(context.Background(), database, futureEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
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

func TestRepairFutureAttendanceEventsMovesEventToPreviousDay(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	if errorValue := service.writeWorkspaceSettingsFile(workspaceSettings{Language: workspaceLanguageKorean, TimeZone: "Asia/Seoul"}); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	location, _ := time.LoadLocation("Asia/Seoul")
	userRecord := mattermostUserRecord{ID: "user-1", Username: "lee", Email: "lee@example.com", Nickname: "이샘플"}
	futureTime := time.Date(2026, 6, 10, 23, 30, 0, 0, location)
	event := service.createAttendanceEvent(userRecord, attendanceKindClockIn, futureTime.UTC(), "team-1", "attendance-channel", "entry-post", "result-post", service.attendanceLocationByID("office"))
	if errorValue := service.insertAttendanceEvent(context.Background(), database, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	now := time.Date(2026, 6, 10, 8, 30, 0, 0, location).UTC()
	if errorValue := service.repairFutureAttendanceEvents(context.Background(), now); errorValue != nil {
		t.Fatal(errorValue)
	}

	events, errorValue := service.readAttendanceEvents(context.Background(), "2026-06", "lee@example.com")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != 1 || events[0].LocalDate != "2026-06-09" || events[0].LocalTime != "23:30:00" {
		t.Fatalf("events = %+v", events)
	}
}

func TestAttendanceEventOverridePreservesOriginalAndProjectsSummary(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	if errorValue := service.writeWorkspaceSettingsFile(workspaceSettings{Language: workspaceLanguageKorean, TimeZone: "Asia/Seoul"}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeAttendanceLocationsFile([]attendanceLocation{
		{ID: "office", Name: "사무실", Color: "#16a34a", IsDefault: true},
		{ID: "home", Name: "재택", Color: "#3b82f6"},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	userRecord := mattermostUserRecord{ID: "user-1", Username: "staff", Email: "staff@example.com", Nickname: "Staff"}
	event := service.createAttendanceEvent(
		userRecord,
		attendanceKindClockIn,
		time.Date(2026, 6, 18, 13, 44, 0, 0, location).UTC(),
		"team-1",
		"attendance-channel",
		"entry-post",
		"result-post",
		service.attendanceLocationByID("office"),
	)
	if errorValue := service.insertAttendanceEvent(context.Background(), database, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/attendance/api/events/"+event.ID, strings.NewReader(`{"localDate":"2026-06-18","localTime":"14:05","locationID":"home","reason":"메시지 인식 수정"}`))
	request.Header.Set("X-Forwarded-Email", "staff@example.com")

	service.handleAttendance(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	summaryRecorder := httptest.NewRecorder()
	summaryRequest := httptest.NewRequest(http.MethodGet, "/attendance/api/summary?month=2026-06", nil)
	summaryRequest.Header.Set("X-Forwarded-Email", "staff@example.com")
	service.handleAttendance(summaryRecorder, summaryRequest)
	if summaryRecorder.Code != http.StatusOK {
		t.Fatalf("summary status = %d body = %s", summaryRecorder.Code, summaryRecorder.Body.String())
	}
	var response attendanceSummaryResponse
	if errorValue := json.Unmarshal(summaryRecorder.Body.Bytes(), &response); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(response.Events) != 1 {
		t.Fatalf("events = %+v", response.Events)
	}
	editedEvent := response.Events[0]
	if editedEvent.LocalTime != "14:05:00" || editedEvent.LocationID != "home" || editedEvent.LocationName != "재택" {
		t.Fatalf("edited event = %+v", editedEvent)
	}
	if editedEvent.OriginalLocalTime != "13:44:00" || editedEvent.OriginalLocationID != "office" || editedEvent.OriginalLocationName != "사무실" {
		t.Fatalf("original event fields = %+v", editedEvent)
	}
	if editedEvent.OverriddenBy != "staff@example.com" || editedEvent.OverriddenAt == "" || editedEvent.OverrideReason != "메시지 인식 수정" {
		t.Fatalf("override metadata = %+v", editedEvent)
	}
	if len(editedEvent.OverrideHistory) != 1 || editedEvent.OverrideHistory[0].OriginalLocalTime != "13:44:00" || editedEvent.OverrideHistory[0].OverrideLocalTime != "14:05:00" {
		t.Fatalf("override history = %+v", editedEvent.OverrideHistory)
	}
}

func TestAttendanceEventOverrideRequiresOwnerOrAdmin(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	if errorValue := service.writeWorkspaceSettingsFile(workspaceSettings{Language: workspaceLanguageKorean, TimeZone: "Asia/Seoul"}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeAttendanceLocationsFile([]attendanceLocation{
		{ID: "office", Name: "사무실", Color: "#16a34a", IsDefault: true},
		{ID: "home", Name: "재택", Color: "#3b82f6"},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	userRecord := mattermostUserRecord{ID: "user-1", Username: "staff", Email: "staff@example.com", Nickname: "Staff"}
	event := service.createAttendanceEvent(
		userRecord,
		attendanceKindClockIn,
		time.Date(2026, 6, 18, 13, 44, 0, 0, location).UTC(),
		"team-1",
		"attendance-channel",
		"entry-post",
		"result-post",
		service.attendanceLocationByID("office"),
	)
	if errorValue := service.insertAttendanceEvent(context.Background(), database, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	staffRecorder := patchAttendanceEventOverrideForTest(service, event.ID, "other@example.com")
	if staffRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected non-owner staff status 403, got %d: %s", staffRecorder.Code, staffRecorder.Body.String())
	}

	adminRecorder := patchAttendanceEventOverrideForTest(service, event.ID, "admin@example.com")
	if adminRecorder.Code != http.StatusOK {
		t.Fatalf("expected admin status 200, got %d: %s", adminRecorder.Code, adminRecorder.Body.String())
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

func patchAttendanceEventOverrideForTest(service *Service, eventID string, actorEmail string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/attendance/api/events/"+eventID, strings.NewReader(`{"localDate":"2026-06-18","localTime":"14:05","locationID":"home","reason":"메시지 인식 수정"}`))
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-Email", actorEmail)
	service.handleAttendance(recorder, request)
	return recorder
}
