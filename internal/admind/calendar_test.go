package admind

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)




func newCalendarTestService(t *testing.T) *Service {
	t.Helper()
	rootPath := t.TempDir()
	adminUIPath := filepath.Join(rootPath, "admin-ui")
	if errorValue := os.MkdirAll(adminUIPath, 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(adminUIPath, "index.html"), []byte("<script></script>"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	service := NewService(Configuration{
		StateDirectory:       filepath.Join(rootPath, "state", "admin"),
		CompanionJobPath:     filepath.Join(rootPath, "state", "companion-jobs.json"),
		CalendarDatabasePath: filepath.Join(rootPath, "state", "calendar.sqlite"),
		TaskDatabasePath:     filepath.Join(rootPath, "state", "flow.sqlite"),
		AdminEmailPath:       writeTestFile(t, "admin@example.com"),
		AdminUIPath:          adminUIPath,
	})
	return service
}

func newCalendarMattermostTestService(t *testing.T, transport roundTripFunc) *Service {
	t.Helper()
	service := newCalendarTestService(t)
	service.Configuration.MattermostAdminPasswordPath = writeTestFile(t, "admin-password")
	service.Configuration.MattermostBotTokenPath = writeTestFile(t, "bot-token")
	service.Configuration.MattermostBaseURL = "http://mattermost.local"
	service.HTTPClient = &http.Client{Transport: transport}
	return service
}

type calendarMattermostLogRequests struct {
	createdMessages []string
	updatedMessages []string
	deletedPostIDs  []string
	createTokens    []string
	updateTokens    []string
}

func mattermostCalendarLogTestResponse(t *testing.T, request *http.Request) (*http.Response, bool) {
	switch {
	case request.Method == http.MethodGet && request.URL.Path == "/api/v4/users":
		return jsonResponse(http.StatusOK, `[{"id":"user-1","username":"dongha","nickname":"샘플","email":"dongha@example.com"}]`, nil), true
	case request.Method == http.MethodGet && request.URL.Path == "/api/v4/users/me":
		return jsonResponse(http.StatusOK, `{"id":"bot-1"}`, nil), true
	case request.Method == http.MethodGet && request.URL.Path == "/api/v4/teams/name/internkim":
		return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), true
	case request.Method == http.MethodGet && request.URL.Path == "/api/v4/teams/team-1/channels/name/calendar":
		return jsonResponse(http.StatusOK, `{"id":"calendar-channel"}`, nil), true
	case request.Method == http.MethodPut && request.URL.Path == "/api/v4/channels/calendar-channel/patch":
		return jsonResponse(http.StatusOK, `{}`, nil), true
	case request.Method == http.MethodGet && request.URL.Path == "/api/v4/channels/calendar-channel/posts":
		return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), true
	case request.Method == http.MethodPut && request.URL.Path == "/api/v4/channels/calendar-channel/moderations/patch":
		return jsonResponse(http.StatusOK, `{}`, nil), true
	case request.Method == http.MethodPost && request.URL.Path == "/api/v4/channels/calendar-channel/members":
		return jsonResponse(http.StatusCreated, `{}`, nil), true
	case request.Method == http.MethodPut && request.URL.Path == "/api/v4/channels/calendar-channel/members/bot-1/schemeRoles":
		return jsonResponse(http.StatusOK, `{}`, nil), true
	case request.Method == http.MethodPost && request.URL.Path == "/api/v4/posts" && mattermostPostChannelID(t, request) == "calendar-channel":
		return jsonResponse(http.StatusCreated, `{"id":"calendar-post-1"}`, nil), true
	case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/api/v4/users/username/"):
		return jsonResponse(http.StatusOK, `{"id":"bot-1","username":"internkim"}`, nil), true
	default:
		return nil, false
	}
}

func mattermostCalendarLogLifecycleResponse(t *testing.T, request *http.Request, requests *calendarMattermostLogRequests) (*http.Response, error) {
	switch {
	case request.Method == http.MethodPost && request.URL.Path == "/api/v4/users/login":
		return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
	case request.Method == http.MethodGet && request.URL.Path == "/api/v4/users":
		return jsonResponse(http.StatusOK, `[{"id":"user-1","username":"member2","nickname":"김예시","email":"member2@example.com"}]`, nil), nil
	case request.Method == http.MethodGet && request.URL.Path == "/api/v4/users/me":
		return jsonResponse(http.StatusOK, `{"id":"bot-1"}`, nil), nil
	case request.Method == http.MethodGet && request.URL.Path == "/api/v4/teams/name/internkim":
		return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
	case request.Method == http.MethodGet && request.URL.Path == "/api/v4/teams/team-1/channels/name/calendar":
		return jsonResponse(http.StatusOK, `{"id":"calendar-channel"}`, nil), nil
	case request.Method == http.MethodPut && request.URL.Path == "/api/v4/channels/calendar-channel/patch":
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	case request.Method == http.MethodGet && request.URL.Path == "/api/v4/channels/calendar-channel/posts":
		return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
	case request.Method == http.MethodPut && request.URL.Path == "/api/v4/channels/calendar-channel/moderations/patch":
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	case request.Method == http.MethodPost && request.URL.Path == "/api/v4/channels/calendar-channel/members":
		return jsonResponse(http.StatusCreated, `{}`, nil), nil
	case request.Method == http.MethodPut && request.URL.Path == "/api/v4/channels/calendar-channel/members/bot-1/schemeRoles":
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	case request.Method == http.MethodGet && request.URL.Path == "/api/v4/posts/calendar-post-1":
		return jsonResponse(http.StatusOK, `{"id":"calendar-post-1","user_id":"bot-1"}`, nil), nil
	case request.Method == http.MethodPost && request.URL.Path == "/api/v4/posts":
		requests.createTokens = append(requests.createTokens, request.Header.Get("Authorization"))
		requests.createdMessages = append(requests.createdMessages, mattermostPostMessage(t, request))
		return jsonResponse(http.StatusCreated, `{"id":"calendar-post-1"}`, nil), nil
	case request.Method == http.MethodPut && request.URL.Path == "/api/v4/posts/calendar-post-1/patch":
		requests.updateTokens = append(requests.updateTokens, request.Header.Get("Authorization"))
		requests.updatedMessages = append(requests.updatedMessages, mattermostPostMessage(t, request))
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	case request.Method == http.MethodDelete && request.URL.Path == "/api/v4/posts/calendar-post-1":
		requests.deletedPostIDs = append(requests.deletedPostIDs, "calendar-post-1")
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/api/v4/users/username/"):
		return jsonResponse(http.StatusOK, `{"id":"bot-1","username":"internkim"}`, nil), nil
	case request.Method == http.MethodPost && request.URL.Path == "/api/v4/channels/direct":
		return jsonResponse(http.StatusCreated, `{"id":"direct-channel-1"}`, nil), nil
	case request.Method == http.MethodPut && strings.HasSuffix(request.URL.Path, "/preferences"):
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	default:
		t.Fatalf("unexpected Mattermost request %s %s", request.Method, request.URL.String())
		return nil, nil
	}
}

func mattermostPostChannelID(t *testing.T, request *http.Request) string {
	t.Helper()
	var payload map[string]any
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request.Body = io.NopCloser(strings.NewReader(string(document)))
	channelID, _ := payload["channel_id"].(string)
	return channelID
}
