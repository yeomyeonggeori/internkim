package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

func TestAttendanceTogglePostsAsUserAndStoresEvent(t *testing.T) {
	service, messages := newAttendanceActionTestService(t)
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

	if len(*messages) != 1 || (*messages)[0] != "출근" {
		t.Fatalf("messages = %+v", *messages)
	}
	events, errorValue := service.readAttendanceEvents(context.Background(), time.Now().Format("2006-01"), "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != 1 || events[0].Kind != attendanceKindClockIn || events[0].ResultPostID != "attendance-post-1" {
		t.Fatalf("events = %+v", events)
	}
}

func TestAttendanceRepeatedClickCancelsEvent(t *testing.T) {
	service, messages := newAttendanceActionTestService(t)
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

	if len(*messages) != 2 || (*messages)[1] != "출근 취소" {
		t.Fatalf("messages = %+v", *messages)
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

func newAttendanceActionTestService(t *testing.T) (*Service, *[]string) {
	t.Helper()
	messages := []string{}
	stateDirectory := t.TempDir()
	service := NewService(Configuration{
		StateDirectory:              stateDirectory,
		AttendanceDatabasePath:      filepath.Join(stateDirectory, "attendance.sqlite"),
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: writeTestFile(t, "admin-pass"),
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
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/posts?per_page=50":
			return jsonResponse(http.StatusOK, `{"order":["entry-post"],"posts":{"entry-post":{"id":"entry-post","message":"출퇴근 기록","props":{"internkim_attendance_entry":true}}}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts/entry-post/patch" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/members" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
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
			messages = append(messages, payload["message"])
			return jsonResponse(http.StatusCreated, `{"id":"attendance-post-1"}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
	return service, &messages
}
