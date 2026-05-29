package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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

	if errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockIn); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockIn); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockOut); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockOut); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(*posts) != 2 || (*posts)[0].Message != "출근" || (*posts)[1].Message != "퇴근" {
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

func TestAttendanceEntryPostUsesSeparateSafeActionIDs(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	props := service.mattermostAttendanceEntryPostProps()
	attachments, ok := props["attachments"].([]mattermostAttachment)
	if !ok || len(attachments) != 1 {
		t.Fatalf("attachments = %+v", props["attachments"])
	}
	actions := attachments[0].Actions
	if len(actions) != 2 {
		t.Fatalf("actions = %+v", actions)
	}
	if actions[0].ID != attendanceClockInAction || actions[0].Name != "출근" {
		t.Fatalf("clock in action = %+v", actions[0])
	}
	if actions[1].ID != attendanceClockOutAction || actions[1].Name != "퇴근" {
		t.Fatalf("clock out action = %+v", actions[1])
	}
}

func TestAttendanceEntryPostUsesLocationNamesForMultipleClockInLocations(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	if errorValue := service.writeAttendanceLocationsFile([]attendanceLocation{
		{ID: "office", Name: "사무실", Color: "#16a34a", IsDefault: true},
		{ID: "home", Name: "재택", Color: "#2563eb"},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}

	actions := service.mattermostAttendanceEntryActions()

	if len(actions) != 3 {
		t.Fatalf("actions = %+v", actions)
	}
	if actions[0].ID != "attendanceClockInoffice" || actions[0].Name != "사무실" {
		t.Fatalf("office action = %+v", actions[0])
	}
	if actions[1].ID != "attendanceClockInhome" || actions[1].Name != "재택" {
		t.Fatalf("home action = %+v", actions[1])
	}
	if actions[2].ID != attendanceClockOutAction || actions[2].Name != "퇴근" {
		t.Fatalf("actions = %+v", actions)
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

	if errorValue := service.recordAttendanceFromMattermost(context.Background(), payload, attendanceKindClockIn); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(*posts) != 1 || (*posts)[0].Message != "출근(사무실)" {
		t.Fatalf("posts = %+v", *posts)
	}
}

func TestAttendanceEntryPostIsBotAuthoredAndPinned(t *testing.T) {
	stateDirectory := t.TempDir()
	service := NewService(Configuration{
		StateDirectory:         stateDirectory,
		MattermostBaseURL:      "http://mattermost.local",
		MattermostBotTokenPath: writeTestFile(t, "bot-token"),
	})
	var createdAsBot bool
	var pinnedAsBot bool
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "http://mattermost.local/api/v4/users/me":
			assertMattermostBearerToken(t, request, "bot-token")
			return jsonResponse(http.StatusOK, `{"id":"bot-1","username":"internkim"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/members" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/members/bot-1/schemeRoles" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/posts?per_page=50":
			return jsonResponse(http.StatusOK, `{"order":["entry-post"],"posts":{"entry-post":{"id":"entry-post","user_id":"admin","message":"출퇴근 기록","props":{"internkim_attendance_entry":true}}}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts/entry-post" && request.Method == http.MethodDelete:
			assertMattermostBearerToken(t, request, "admin-token")
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts" && request.Method == http.MethodPost:
			assertMattermostBearerToken(t, request, "bot-token")
			createdAsBot = true
			return jsonResponse(http.StatusCreated, `{"id":"attendance-entry","user_id":"bot-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts/attendance-entry/pin" && request.Method == http.MethodPost:
			assertMattermostBearerToken(t, request, "bot-token")
			pinnedAsBot = true
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	if errorValue := service.ensureMattermostAttendanceEntryPost(context.Background(), "admin-token", "attendance-channel"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !createdAsBot || !pinnedAsBot {
		t.Fatalf("createdAsBot=%v pinnedAsBot=%v", createdAsBot, pinnedAsBot)
	}
}

func TestAttendanceEntryPostPatchKeepsExistingBotPost(t *testing.T) {
	stateDirectory := t.TempDir()
	service := NewService(Configuration{
		StateDirectory:         stateDirectory,
		MattermostBaseURL:      "http://mattermost.local",
		MattermostBotTokenPath: writeTestFile(t, "bot-token"),
	})
	var patched bool
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "http://mattermost.local/api/v4/users/me":
			assertMattermostBearerToken(t, request, "bot-token")
			return jsonResponse(http.StatusOK, `{"id":"bot-1","username":"internkim"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/members" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/members/bot-1/schemeRoles" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/posts?per_page=50":
			return jsonResponse(http.StatusOK, `{"order":["entry-post"],"posts":{"entry-post":{"id":"entry-post","user_id":"bot-1","message":"출퇴근 기록","is_pinned":true,"props":{"internkim_attendance_entry":true}}}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts/entry-post/patch" && request.Method == http.MethodPut:
			assertMattermostBearerToken(t, request, "bot-token")
			var payload struct {
				Message string         `json:"message"`
				Props   map[string]any `json:"props"`
			}
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload.Message != "출퇴근 기록" || payload.Props[attendanceEntryPostProperty] != true {
				t.Fatalf("payload = %+v", payload)
			}
			patched = true
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	if errorValue := service.ensureMattermostAttendanceEntryPost(context.Background(), "admin-token", "attendance-channel"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !patched {
		t.Fatal("existing bot attendance post was not patched")
	}
	if readTrimmedFile(service.mattermostAttendanceEntryPostIDPath()) != "entry-post" {
		t.Fatalf("entry post id = %q", readTrimmedFile(service.mattermostAttendanceEntryPostIDPath()))
	}
}

func TestAttendanceEntryPostUsesStoredPostOutsideRecentPage(t *testing.T) {
	stateDirectory := t.TempDir()
	service := NewService(Configuration{
		StateDirectory:         stateDirectory,
		MattermostBaseURL:      "http://mattermost.local",
		MattermostBotTokenPath: writeTestFile(t, "bot-token"),
	})
	service.saveMattermostAttendanceEntryPostID("stored-entry")
	var patched bool
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "http://mattermost.local/api/v4/users/me":
			return jsonResponse(http.StatusOK, `{"id":"bot-1","username":"internkim"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/members" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/members/bot-1/schemeRoles" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts/stored-entry" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"id":"stored-entry","user_id":"bot-1","channel_id":"attendance-channel","message":"출퇴근 기록","is_pinned":true,"props":{"internkim_attendance_entry":true}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts/stored-entry/patch" && request.Method == http.MethodPut:
			patched = true
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	if errorValue := service.ensureMattermostAttendanceEntryPost(context.Background(), "admin-token", "attendance-channel"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !patched {
		t.Fatal("stored entry post was not patched")
	}
}

func TestAttendanceChannelKeepsHeaderAndClearsPurpose(t *testing.T) {
	service := NewService(Configuration{MattermostBaseURL: "http://mattermost.local"})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/posts?per_page=100" {
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
		}
		if request.URL.String() != "http://mattermost.local/api/v4/channels/attendance-channel/patch" || request.Method != http.MethodPut {
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		}
		var payload map[string]string
		if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
			t.Fatal(errorValue)
		}
		if payload["display_name"] != attendanceChannelDisplayName {
			t.Fatalf("display name = %q", payload["display_name"])
		}
		if payload["header"] == "" {
			t.Fatal("attendance channel header should keep the link")
		}
		if payload["purpose"] != "" {
			t.Fatalf("attendance channel purpose = %q", payload["purpose"])
		}
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	})}

	if errorValue := service.updateMattermostAttendanceChannelText(context.Background(), "admin-token", "attendance-channel"); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestMattermostPostDeleteRemovesAttendanceEvent(t *testing.T) {
	mattermostServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodDelete || request.URL.Path != "/api/v4/posts/result-post-1" {
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer mattermostServer.Close()
	stateDirectory := t.TempDir()
	service := NewService(Configuration{
		StateDirectory:         stateDirectory,
		AttendanceDatabasePath: filepath.Join(stateDirectory, "attendance.sqlite"),
		MattermostBaseURL:      mattermostServer.URL,
		AdminEmailPath:         writeTestFile(t, "admin@example.com"),
		MattermostBotTokenPath: writeTestFile(t, "bot-token"),
		MattermostTokenPath:    writeTestFile(t, "bot-token"),
	})
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event := service.createAttendanceEvent(
		mattermostUserRecord{ID: "user-1", Username: "staff", Email: "staff@example.com"},
		attendanceKindClockIn,
		time.Now().UTC(),
		"team-1",
		"attendance-channel",
		"action-post-1",
		"result-post-1",
		attendanceLocation{},
	)
	if errorValue := service.insertAttendanceEvent(context.Background(), database, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/v4/posts/result-post-1", nil)
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("delete status = %d", response.Code)
	}
	events, errorValue := service.readAttendanceEvents(context.Background(), time.Now().Format("2006-01"), "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != 0 {
		t.Fatalf("events = %+v", events)
	}
}

func TestMattermostPostDeleteProtectsAttendanceEntryPost(t *testing.T) {
	mattermostServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		t.Fatalf("unexpected Mattermost request %s %s", request.Method, request.URL.Path)
	}))
	defer mattermostServer.Close()
	stateDirectory := t.TempDir()
	service := NewService(Configuration{
		StateDirectory:         stateDirectory,
		MattermostBaseURL:      mattermostServer.URL,
		MattermostBotTokenPath: writeTestFile(t, "bot-token"),
	})
	service.saveMattermostAttendanceEntryPostID("entry-post")

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/v4/posts/entry-post", nil)
	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("delete status = %d", response.Code)
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

	if errorValue := service.toggleAttendanceFromMattermost(context.Background(), payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.toggleAttendanceFromMattermost(context.Background(), payload); errorValue != nil && errorValue != errAttendanceDuplicateIgnored {
		t.Fatal(errorValue)
	}
	if errorValue := service.toggleAttendanceFromMattermost(context.Background(), payload); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(*posts) != 2 || (*posts)[1].Message != "출근 취소" || (*posts)[1].RootID != "entry-post" {
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

type attendanceActionPost struct {
	Message string
	RootID  string
}

func newAttendanceActionTestService(t *testing.T) (*Service, *[]attendanceActionPost) {
	t.Helper()
	posts := []attendanceActionPost{}
	stateDirectory := t.TempDir()
	service := NewService(Configuration{
		StateDirectory:              stateDirectory,
		AttendanceDatabasePath:      filepath.Join(stateDirectory, "attendance.sqlite"),
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: writeTestFile(t, "admin-pass"),
		MattermostBotTokenPath:      writeTestFile(t, "bot-token"),
		AdminEmailPath:              writeTestFile(t, "admin@example.com"),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1" && request.Header.Get("Authorization") == "Bearer admin-token":
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"staff@example.com","username":"staff","nickname":"Staff"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/attendance":
			return jsonResponse(http.StatusOK, `{"id":"attendance-channel"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/patch" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/posts?per_page=100":
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/me" && request.Header.Get("Authorization") == "Bearer bot-token":
			return jsonResponse(http.StatusOK, `{"id":"bot-1","email":"internkim@localhost","username":"internkim"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts/entry-post" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"id":"entry-post","user_id":"bot-1","channel_id":"attendance-channel","is_pinned":true,"message":"출퇴근 기록","props":{"internkim_attendance_entry":true,"attachments":[{"fallback":"출퇴근 기록","text":"출근과 퇴근 버튼을 구분해서 기록합니다.","actions":[{"id":"attendanceClockIn","name":"출근"},{"id":"attendanceClockOut","name":"퇴근"}]}]}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/posts?per_page=50":
			return jsonResponse(http.StatusOK, `{"order":["entry-post"],"posts":{"entry-post":{"id":"entry-post","user_id":"bot-1","is_pinned":true,"message":"출퇴근 기록","props":{"internkim_attendance_entry":true,"attachments":[{"fallback":"출퇴근 기록","text":"출근과 퇴근 버튼을 구분해서 기록합니다.","actions":[{"id":"attendanceClockIn","name":"출근"},{"id":"attendanceClockOut","name":"퇴근"}]}]}}}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts/entry-post/patch" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/members" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/members/bot-1/schemeRoles" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/tokens" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{"token":"user-token"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/me" && request.Header.Get("Authorization") == "Bearer user-token":
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"staff@example.com","username":"staff"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts" && request.Method == http.MethodPost:
			var payload map[string]string
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if request.Header.Get("Authorization") != "Bearer user-token" {
				t.Fatalf("post token = %q", request.Header.Get("Authorization"))
			}
			posts = append(posts, attendanceActionPost{Message: payload["message"], RootID: payload["root_id"]})
			return jsonResponse(http.StatusCreated, `{"id":"attendance-post-1"}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
	return service, &posts
}
