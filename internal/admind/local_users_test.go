package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLocalUsersRequiresResolvedMutationIdentity(t *testing.T) {
	service := newLocalUsersTestService(t)
	externalRequestMade := false
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		externalRequestMade = true
		return nil, errors.New("unexpected external request")
	})}

	_, _, errorValue := service.applyLocalUserMutation(context.Background(), adminUserMutation{
		Email:  "member@example.com",
		Handle: "member-user",
		Role:   "admin",
	}, false)

	if errorValue == nil || errorValue.Error() != "resolved userID required for local user mutation" {
		t.Fatalf("error = %v", errorValue)
	}
	if externalRequestMade {
		t.Fatal("external request was made before mutation identity resolution")
	}
}

func TestLocalListUsers(t *testing.T) {
	service := newLocalUsersTestService(t)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users?in_team=team-1&per_page=200":
			return jsonResponse(http.StatusOK, `[
				{"id":"user-1","email":"admin@example.com","username":"admin-user","nickname":"Admin User","roles":"system_user","delete_at":0},
				{"id":"user-2","email":"member@example.com","username":"member-user","nickname":"Member User","roles":"system_user","delete_at":0},
				{"id":"deleted-1","email":"deleted@example.com","username":"deleted-user","roles":"system_user","delete_at":10}
			]`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members":
			return jsonResponse(http.StatusOK, `[
				{"user_id":"user-1","roles":"team_user team_admin"},
				{"user_id":"user-2","roles":"team_user"}
			]`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://blueclaw.local/admin/api/policy":
			return jsonResponse(http.StatusOK, localUsersPolicyDocument(), nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	responseRecorder := httptest.NewRecorder()
	service.localListUsers(responseRecorder, httptest.NewRequest(http.MethodGet, "/admin/api/users", nil))

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	var response pagesUsersResponse
	if errorValue := json.Unmarshal(responseRecorder.Body.Bytes(), &response); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(response.Records) != 2 {
		t.Fatalf("records = %d; want 2", len(response.Records))
	}
	if response.Records[0].Role != "admin" || response.Records[1].Role != "member" {
		t.Fatalf("roles = %q, %q; want admin, member", response.Records[0].Role, response.Records[1].Role)
	}
	if response.Records[1].Note != "Existing member note" {
		t.Fatalf("member note = %q; want Existing member note", response.Records[1].Note)
	}
	expectedImage := calendarParticipantImagePath(stableFlowID("member@example.com"))
	if response.Records[1].Image != expectedImage {
		t.Fatalf("member image = %q; want %q", response.Records[1].Image, expectedImage)
	}
}

func TestLocalUpsertUser(t *testing.T) {
	service := newLocalUsersTestService(t)
	mattermostCreated := false
	blueclawSaved := false
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == "blueclaw.local" {
			return localUsersBlueclawUpsertResponse(t, request, &blueclawSaved)
		}
		return localUsersMattermostUpsertResponse(t, request, &mattermostCreated)
	})}

	requestBody := strings.NewReader(`{"email":"new@example.com","handle":"new-user","name":"New User","role":"admin","circles":[],"note":"  Local note  "}`)
	responseRecorder := httptest.NewRecorder()
	service.localUpsertUser(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users", requestBody))

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if !mattermostCreated {
		t.Fatal("Mattermost user create call did not fire")
	}
	if !blueclawSaved {
		t.Fatal("Blueclaw policy save call did not fire")
	}
}

func TestLocalUpsertUsersBatchRejectsDuplicateNormalizedEmailsBeforeExternalRequests(t *testing.T) {
	service := newLocalUsersTestService(t)
	externalRequestMade := false
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		externalRequestMade = true
		return nil, errors.New("unexpected external request")
	})}

	requestBody := strings.NewReader(`{"users":[
		{"email":"Member@Example.com","handle":"member-one","name":"Member One","role":"member"},
		{"email":" member@example.COM ","handle":"member-two","name":"Member Two","role":"member"}
	]}`)
	responseRecorder := httptest.NewRecorder()
	service.localUpsertUsersBatch(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users/batch", requestBody))

	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d; body = %s", responseRecorder.Code, http.StatusBadRequest, responseRecorder.Body.String())
	}
	if externalRequestMade {
		t.Fatal("external request was made before duplicate email validation")
	}
}

func TestNormalizeAdminUserPayloadPreservesOperationsAdminRole(t *testing.T) {
	payload, _, errorValue := normalizeAdminUserPayload(adminUserMutation{
		Email: "operator@example.com",
		Role:  "operationsAdmin",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if payload.Role != "operationsAdmin" {
		t.Fatalf("role = %q, want operationsAdmin", payload.Role)
	}
}

func TestLocalRemoveUser(t *testing.T) {
	service := newLocalUsersTestService(t)
	deactivatedUser := false
	removedPerson := false
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/email/member@example.com":
			return jsonResponse(http.StatusOK, `{"id":"user-2","email":"member@example.com","username":"member-user","roles":"system_user","delete_at":0}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users?in_team=team-1&per_page=200":
			return jsonResponse(http.StatusOK, `[
				{"id":"user-1","email":"admin@example.com","username":"admin-user","roles":"system_admin system_user","delete_at":0},
				{"id":"user-2","email":"member@example.com","username":"member-user","roles":"system_user","delete_at":0}
			]`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members":
			return jsonResponse(http.StatusOK, `[
				{"user_id":"user-1","roles":"team_user team_admin"},
				{"user_id":"user-2","roles":"team_user"}
			]`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/user-2":
			return jsonResponse(http.StatusOK, `{"id":"user-2","email":"member@example.com","username":"member-user","roles":"system_user","delete_at":0}`, nil), nil
		case request.Method == http.MethodDelete && request.URL.String() == "http://mattermost.local/api/v4/users/user-2":
			deactivatedUser = true
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://blueclaw.local/admin/api/policy":
			return jsonResponse(http.StatusOK, `{"people":[],"circles":[]}`, nil), nil
		case request.Method == http.MethodDelete && request.URL.String() == "http://blueclaw.local/admin/api/people?email=member%40example.com":
			removedPerson = true
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	responseRecorder := httptest.NewRecorder()
	service.localRemoveUser(responseRecorder, httptest.NewRequest(http.MethodDelete, "/admin/api/users/member@example.com", nil), "member@example.com")

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if !deactivatedUser || !removedPerson {
		t.Fatalf("deactivated = %t, removed = %t; want true, true", deactivatedUser, removedPerson)
	}
}

func TestLocalRemoveUserLastAdmin(t *testing.T) {
	service := newLocalUsersTestService(t)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/email/admin@example.com":
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"admin@example.com","username":"admin-user","roles":"system_admin system_user","delete_at":0}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users?in_team=team-1&per_page=200":
			return jsonResponse(http.StatusOK, `[{"id":"user-1","email":"admin@example.com","username":"admin-user","roles":"system_admin system_user","delete_at":0}]`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members":
			return jsonResponse(http.StatusOK, `[{"user_id":"user-1","roles":"team_user team_admin"}]`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	responseRecorder := httptest.NewRecorder()
	service.localRemoveUser(responseRecorder, httptest.NewRequest(http.MethodDelete, "/admin/api/users/admin@example.com", nil), "admin@example.com")

	if responseRecorder.Code != http.StatusConflict {
		t.Fatalf("status = %d; want %d; body = %s", responseRecorder.Code, http.StatusConflict, responseRecorder.Body.String())
	}
}

func newLocalUsersTestService(t *testing.T) *Service {
	t.Helper()
	service := NewService(Configuration{
		BlueclawBaseURL:             "http://blueclaw.local",
		MattermostBaseURL:           "http://mattermost.local",
		MattermostAdminPasswordPath: writeTestFile(t, "admin-pass"),
		FleetIDPath:                 t.TempDir() + "/missing-fleet-id",
		FleetSecretPath:             t.TempDir() + "/missing-fleet-secret",
		StateDirectory:              t.TempDir(),
		MattermostBotTokenPath:      t.TempDir() + "/missing-bot-token",
		MattermostTeamName:          "internkim",
	})
	service.RunCommand = func(context.Context, string, ...string) ([]byte, error) {
		return nil, nil
	}
	return service
}

func localUsersMattermostUpsertResponse(t *testing.T, request *http.Request, mattermostCreated *bool) (*http.Response, error) {
	t.Helper()
	switch {
	case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users/login":
		return jsonResponse(http.StatusOK, `{}`, http.Header{"Token": []string{"admin-token"}}), nil
	case request.Method == http.MethodPut && request.URL.String() == "http://mattermost.local/api/v4/config/patch":
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/email/new@example.com":
		return jsonResponse(http.StatusNotFound, `{}`, nil), nil
	case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users":
		body := readLocalUsersTestBody(t, request)
		if !strings.Contains(body, `"email":"new@example.com"`) || !strings.Contains(body, `"username":"new-user"`) {
			t.Fatalf("unexpected create body %s", body)
		}
		*mattermostCreated = true
		return jsonResponse(http.StatusCreated, `{"id":"user-new","email":"new@example.com","username":"new-user","nickname":"New User","roles":"system_user","delete_at":0}`, nil), nil
	case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
		return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
	case request.Method == http.MethodGet && strings.Contains(request.URL.String(), "/api/v4/teams/team-1/channels/name/"):
		channelName := request.URL.Path[strings.LastIndex(request.URL.Path, "/")+1:]
		return jsonResponse(http.StatusOK, `{"id":"`+channelName+`"}`, nil), nil
	case request.Method == http.MethodPut && strings.Contains(request.URL.String(), "/api/v4/channels/"):
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	case request.Method == http.MethodGet && strings.Contains(request.URL.String(), "/posts?per_page="):
		return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
	case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members":
		return jsonResponse(http.StatusCreated, `{}`, nil), nil
	case request.Method == http.MethodPost && strings.Contains(request.URL.String(), "/api/v4/channels/") && strings.HasSuffix(request.URL.Path, "/members"):
		return jsonResponse(http.StatusCreated, `{}`, nil), nil
	case request.Method == http.MethodPut && request.URL.String() == "http://mattermost.local/api/v4/users/user-new/roles":
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	case request.Method == http.MethodPut && request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members/user-new/schemeRoles":
		body := readLocalUsersTestBody(t, request)
		if !strings.Contains(body, `"scheme_admin":true`) {
			t.Fatalf("expected team admin scheme role, got %s", body)
		}
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/username/internkim":
		return jsonResponse(http.StatusNotFound, `{}`, nil), nil
	default:
		t.Fatalf("unexpected Mattermost request %s %s", request.Method, request.URL.String())
		return nil, nil
	}
}

func localUsersBlueclawUpsertResponse(t *testing.T, request *http.Request, blueclawSaved *bool) (*http.Response, error) {
	t.Helper()
	switch {
	case request.Method == http.MethodGet && request.URL.String() == "http://blueclaw.local/admin/api/policy":
		return jsonResponse(http.StatusOK, localUsersPolicyDocument(), nil), nil
	case request.Method == http.MethodPost && request.URL.String() == "http://blueclaw.local/admin/api/people/invite":
		body := readLocalUsersTestBody(t, request)
		if !strings.Contains(body, `"email":"new@example.com"`) {
			t.Fatalf("unexpected invite body %s", body)
		}
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	case request.Method == http.MethodPost && request.URL.String() == "http://blueclaw.local/admin/api/policy/save":
		body := readLocalUsersTestBody(t, request)
		if !strings.Contains(body, `"emails":["new@example.com"]`) || !strings.Contains(body, `"isAdmin":true`) || !strings.Contains(body, `"note":"Local note"`) {
			t.Fatalf("unexpected policy save body %s", body)
		}
		*blueclawSaved = true
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	default:
		t.Fatalf("unexpected Blueclaw request %s %s", request.Method, request.URL.String())
		return nil, nil
	}
}

func readLocalUsersTestBody(t *testing.T, request *http.Request) string {
	t.Helper()
	document, errorValue := io.ReadAll(request.Body)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request.Body = io.NopCloser(bytes.NewReader(document))
	return string(document)
}

func localUsersPolicyDocument() string {
	return `{
		"people":[
			{"personID":"user-admin","displayName":"Admin User","emails":["admin@example.com"],"circles":["staff","admin"],"isAdmin":true},
			{"personID":"user-member","displayName":"Member User","emails":["member@example.com"],"circles":["staff"],"isAdmin":false,"note":"Existing member note"},
			{"personID":"user-new","displayName":"New User","emails":["new@example.com"],"circles":["staff"],"isAdmin":false}
		],
		"circles":[{"circleID":"staff","displayName":"Staff"}],
		"circleSync":{"mattermostPrivateChannels":[{"circleID":"staff","channelName":"circle-staff"}]}
	}`
}
