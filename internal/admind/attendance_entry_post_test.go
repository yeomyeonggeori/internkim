package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

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

func TestAttendanceEntryPostPatchRefreshesStaleActionContext(t *testing.T) {
	stateDirectory := t.TempDir()
	service := NewService(Configuration{
		StateDirectory:         stateDirectory,
		MattermostBaseURL:      "http://mattermost.local",
		MattermostBotTokenPath: writeTestFile(t, "bot-token"),
		ListenAddress:          "127.0.0.1:9000",
	})
	expectedToken := service.ensureMattermostInteractiveActionToken()
	staleProps := service.mattermostAttendanceEntryPostProps()
	attachments, ok := staleProps["attachments"].([]mattermostAttachment)
	if !ok || len(attachments) != 1 {
		t.Fatalf("attachments = %+v", staleProps["attachments"])
	}
	if len(attachments[0].Actions) == 0 {
		t.Fatalf("actions = %+v", attachments[0].Actions)
	}
	attachments[0].Actions[0].Integration.Context.Token = "stale-token"
	staleProps["attachments"] = attachments
	stalePropsDocument, errorValue := json.Marshal(staleProps)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
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
			return jsonResponse(http.StatusOK, `{"order":["entry-post"],"posts":{"entry-post":{"id":"entry-post","user_id":"bot-1","channel_id":"attendance-channel","message":"출퇴근 기록","is_pinned":true,"props":`+string(stalePropsDocument)+`}}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts/entry-post/patch" && request.Method == http.MethodPut:
			assertMattermostBearerToken(t, request, "bot-token")
			var payload struct {
				Props struct {
					Attachments []mattermostAttachment `json:"attachments"`
				} `json:"props"`
			}
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if len(payload.Props.Attachments) != 1 || len(payload.Props.Attachments[0].Actions) == 0 {
				t.Fatalf("patch attachments = %+v", payload.Props.Attachments)
			}
			actualToken := payload.Props.Attachments[0].Actions[0].Integration.Context.Token
			if actualToken != expectedToken || actualToken == "stale-token" {
				t.Fatalf("patch action token = %q, expected %q", actualToken, expectedToken)
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
		t.Fatal("stale attendance action context was not patched")
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
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/posts?per_page=50":
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
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
