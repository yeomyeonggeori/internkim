package admind

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type attendanceActionPost struct {
	ID      string
	Message string
	RootID  string
	Deleted bool
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
		case request.URL.String() == "http://mattermost.local/api/v4/users/email/staff@example.com" && request.Header.Get("Authorization") == "Bearer admin-token":
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
		case request.URL.String() == "http://mattermost.local/api/v4/users/me" && strings.Contains(request.Header.Get("Cookie"), "MMAUTHTOKEN=session-token"):
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"staff@example.com","username":"staff","nickname":"Staff"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/posts" && request.Method == http.MethodPost:
			var payload map[string]string
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			postIDPrefix := "command-post-"
			if request.Header.Get("Authorization") == "Bearer user-token" {
				postIDPrefix = "attendance-post-"
			}
			postID := postIDPrefix + strconv.Itoa(len(posts)+1)
			posts = append(posts, attendanceActionPost{ID: postID, Message: payload["message"], RootID: payload["root_id"]})
			return jsonResponse(http.StatusCreated, `{"id":"`+postID+`","channel_id":"`+payload["channel_id"]+`","root_id":"`+payload["root_id"]+`","message":"`+payload["message"]+`"}`, nil), nil
		case strings.HasPrefix(request.URL.String(), "http://mattermost.local/api/v4/posts/") && strings.HasSuffix(request.URL.String(), "/patch") && request.Method == http.MethodPut:
			if request.Header.Get("Authorization") != "Bearer user-token" {
				t.Fatalf("patch token = %q", request.Header.Get("Authorization"))
			}
			postID := strings.TrimSuffix(strings.TrimPrefix(request.URL.Path, "/api/v4/posts/"), "/patch")
			var payload map[string]string
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			for index := range posts {
				if posts[index].ID == postID {
					posts[index].Message = payload["message"]
					return jsonResponse(http.StatusOK, `{}`, nil), nil
				}
			}
			return jsonResponse(http.StatusNotFound, `{}`, nil), nil
		case strings.HasPrefix(request.URL.String(), "http://mattermost.local/api/v4/posts/") && request.Method == http.MethodDelete:
			if request.Header.Get("Authorization") != "Bearer admin-token" {
				t.Fatalf("delete token = %q", request.Header.Get("Authorization"))
			}
			postID := strings.TrimPrefix(request.URL.Path, "/api/v4/posts/")
			for index := range posts {
				if posts[index].ID == postID {
					posts[index].Deleted = true
					return jsonResponse(http.StatusOK, `{}`, nil), nil
				}
			}
			return jsonResponse(http.StatusNotFound, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
	return service, &posts
}
