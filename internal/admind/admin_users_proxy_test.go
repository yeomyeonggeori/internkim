package admind

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdminUserSavePatchesMattermostIdentityByStoredID(t *testing.T) {
	var pagesPayload map[string]any
	var mattermostPatch map[string]string
	service := newAdminUsersProxyTestService(t, func(request *http.Request) (*http.Response, error) {
		if response, isHandled := adminUsersProxyCommonMattermostResponse(t, request); isHandled {
			return response, nil
		}
		switch {
		case request.URL.String() == "https://api.intern.kim/api/users?fleet_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[{"userID":"user-member","handle":"oldhandle","name":"Old Name","email":"member@example.com","role":"member","mattermostUserID":"user-1","mattermostUsername":"oldhandle"},{"email":"admin@example.com","role":"admin"}]}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"member@example.com","username":"oldhandle","roles":"system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/patch" && request.Method == http.MethodPut:
			if errorValue := json.NewDecoder(request.Body).Decode(&mattermostPatch); errorValue != nil {
				t.Fatal(errorValue)
			}
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"member@example.com","username":"newhandle"}`, nil), nil
		case request.URL.String() == "https://api.intern.kim/api/users" && request.Method == http.MethodPost:
			if errorValue := json.NewDecoder(request.Body).Decode(&pagesPayload); errorValue != nil {
				t.Fatal(errorValue)
			}
			return jsonResponse(http.StatusOK, `{"records":[{"userID":"user-member","handle":"newhandle","name":"New Name","email":"member@example.com","role":"member","mattermostUserID":"user-1","mattermostUsername":"newhandle"}]}`, nil), nil
		case isBlueclawInviteRequest(t, request, "member@example.com"):
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})

	request := httptest.NewRequest(http.MethodPost, "/admin/api/users", strings.NewReader(`{"userID":"user-member","handle":"newhandle","name":"New Name","email":"member@example.com","role":"member","mattermostUserID":"user-1","mattermostUsername":"oldhandle","note":"Needs HR compensation follow-up"}`))
	request.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("user save status = %d body = %s", response.Code, response.Body.String())
	}
	if mattermostPatch["username"] != "newhandle" || mattermostPatch["first_name"] != "New" || mattermostPatch["last_name"] != "Name" || mattermostPatch["nickname"] != "New" {
		t.Fatalf("mattermost patch = %#v", mattermostPatch)
	}
	if pagesPayload["userID"] != "user-member" || pagesPayload["handle"] != "newhandle" || pagesPayload["name"] != "New Name" {
		t.Fatalf("pages payload = %#v", pagesPayload)
	}
	if pagesPayload["note"] != "Needs HR compensation follow-up" {
		t.Fatalf("pages note payload = %#v", pagesPayload)
	}
}

func TestAdminUserSaveWritesBlueclawNote(t *testing.T) {
	var savedPerson map[string]any
	service := newAdminUsersProxyTestService(t, func(request *http.Request) (*http.Response, error) {
		if response, isHandled := adminUsersProxyCommonMattermostResponse(t, request); isHandled {
			return response, nil
		}
		switch {
		case request.URL.String() == "https://api.intern.kim/api/users?fleet_id=dc719d8e" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"records":[{"userID":"user-member","handle":"member","name":"Member User","email":"member@example.com","role":"member","mattermostUserID":"user-1","mattermostUsername":"member"},{"email":"admin@example.com","role":"admin"}]}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1" && request.Method == http.MethodGet:
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"member@example.com","username":"member","roles":"system_user"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/patch" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"member@example.com","username":"member"}`, nil), nil
		case request.URL.String() == "https://api.intern.kim/api/users" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusOK, `{"records":[{"userID":"user-member","handle":"member","name":"Member User","email":"member@example.com","role":"member","mattermostUserID":"user-1","mattermostUsername":"member"}]}`, nil), nil
		case isBlueclawInviteRequest(t, request, "member@example.com"):
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case isBlueclawPolicyGet(request):
			return jsonResponse(http.StatusOK, `{"people":[{"personID":"user-member","displayName":"Member User","emails":["member@example.com"],"circles":["staff"],"isAdmin":false,"note":"Existing note"}],"channels":[],"circleSync":{"mattermostPrivateChannels":[{"circleID":"staff","channelName":"circle-staff"}]},"retention":{"rawEventDays":60}}`, nil), nil
		case request.Method == http.MethodPost && request.URL.String() == "http://127.0.0.1:8080/admin/api/policy/save":
			var policyDocument map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&policyDocument); errorValue != nil {
				t.Fatal(errorValue)
			}
			people, _ := policyDocument["people"].([]any)
			if len(people) != 1 {
				t.Fatalf("Blueclaw people = %#v", policyDocument["people"])
			}
			savedPerson, _ = people[0].(map[string]any)
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/circle-staff":
			return jsonResponse(http.StatusOK, `{"id":"circle-staff-channel"}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/circle-staff-channel/patch" && request.Method == http.MethodPut:
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/circle-staff-channel/posts?per_page=100":
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/circle-staff-channel/members" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})

	requestBody := `{"userID":"user-member","handle":"member","name":"Member User","email":"member@example.com","role":"member","mattermostUserID":"user-1","mattermostUsername":"member","circles":["staff"],"note":"Needs HR compensation follow-up"}`
	request := httptest.NewRequest(http.MethodPost, "/admin/api/users", strings.NewReader(requestBody))
	request.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("user save status = %d body = %s", response.Code, response.Body.String())
	}
	if savedPerson["note"] != "Needs HR compensation follow-up" {
		t.Fatalf("Blueclaw note = %#v; person = %#v", savedPerson["note"], savedPerson)
	}
}

func newAdminUsersProxyTestService(t *testing.T, transport roundTripFunc) *Service {
	t.Helper()
	deviceDirectory := t.TempDir()
	fleetIDPath := filepath.Join(deviceDirectory, "fleet-id")
	fleetSecretPath := filepath.Join(deviceDirectory, "fleet-secret")
	writeFile(t, fleetIDPath, "dc719d8e")
	writeFile(t, fleetSecretPath, "secret-value")
	writeFile(t, filepath.Join(deviceDirectory, "admin-email"), "admin@example.com")
	service := NewService(Configuration{
		APIBaseURL:                  "https://api.intern.kim",
		MattermostBaseURL:           "http://mattermost.local",
		BlueclawBaseURL:             "http://127.0.0.1:8080",
		MattermostAdminPasswordPath: writeTestFile(t, "admin-password"),
		AdminEmailPath:              writeTestFile(t, "admin@example.com"),
		FleetIDPath:                 fleetIDPath,
		FleetSecretPath:             fleetSecretPath,
		StateDirectory:              t.TempDir(),
		CompanionJobPath:            filepath.Join(t.TempDir(), "jobs.json"),
		AdminUIPath:                 t.TempDir(),
	})
	service.HTTPClient = &http.Client{Transport: transport}
	return service
}

func adminUsersProxyCommonMattermostResponse(t *testing.T, request *http.Request) (*http.Response, bool) {
	t.Helper()
	switch {
	case request.URL.String() == "http://mattermost.local/api/v4/users/login":
		return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), true
	case request.URL.String() == "http://mattermost.local/api/v4/users/username/admin":
		return jsonResponse(http.StatusOK, `{"id":"admin","email":"admin@localhost","username":"admin","roles":"system_admin system_user"}`, nil), true
	case request.URL.String() == "http://mattermost.local/api/v4/users/admin/roles":
		return jsonResponse(http.StatusOK, `{}`, nil), true
	case isMattermostConnectCommandSetupRequest(request):
		return mattermostConnectCommandSetupResponse(t, request), true
	case request.URL.String() == "http://mattermost.local/api/v4/config/patch" && request.Method == http.MethodPut:
		assertMattermostNicknameDisplayPatch(t, request)
		return jsonResponse(http.StatusOK, `{}`, nil), true
	case request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
		return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), true
	case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members":
		return jsonResponse(http.StatusCreated, `{}`, nil), true
	case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/channels/name/town-square":
		return jsonResponse(http.StatusOK, `{"id":"channel-1"}`, nil), true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/channel-1/members":
		return jsonResponse(http.StatusCreated, `{}`, nil), true
	case isMattermostFlowSetupRequest(request):
		return mattermostFlowSetupResponse(t, request), true
	case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/roles":
		return jsonResponse(http.StatusOK, `{}`, nil), true
	case request.URL.String() == "http://mattermost.local/api/v4/users/username/internkim":
		return jsonResponse(http.StatusOK, `{"id":"bot-1","email":"internkim@localhost","username":"internkim","roles":"system_user"}`, nil), true
	case request.URL.String() == "http://mattermost.local/api/v4/channels/direct" && request.Method == http.MethodPost:
		return jsonResponse(http.StatusCreated, `{"id":"dm-1"}`, nil), true
	case request.URL.String() == "http://mattermost.local/api/v4/users/user-1/preferences" && request.Method == http.MethodPut:
		assertBotDirectChannelShown(t, request, "user-1", "bot-1")
		return jsonResponse(http.StatusOK, `{}`, nil), true
	default:
		return nil, false
	}
}
