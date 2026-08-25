package admind

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func localOrganizationMattermostResponse(t *testing.T, request *http.Request) (*http.Response, bool) {
	t.Helper()
	switch request.URL.String() {
	case "http://mattermost.local/api/v4/users/login":
		return jsonResponse(http.StatusOK, `{}`, http.Header{"Token": []string{"admin-token"}}), true
	case "http://mattermost.local/api/v4/teams/name/internkim":
		return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), true
	case "http://mattermost.local/api/v4/users?in_team=team-1&per_page=200":
		return jsonResponse(http.StatusOK, `[{"id":"user-2","email":"member@example.com","username":"member-user","nickname":"Member User","roles":"system_user","delete_at":0}]`, nil), true
	case "http://mattermost.local/api/v4/teams/team-1/members":
		return jsonResponse(http.StatusOK, `[{"user_id":"user-2","roles":"team_user"}]`, nil), true
	case "http://blueclaw.local/admin/api/policy":
		return jsonResponse(http.StatusOK, localUsersPolicyDocument(), nil), true
	}
	return nil, false
}

func TestOrganizationProfileUpdateReachesTheCompanyDirectory(t *testing.T) {
	service := newLocalUsersTestService(t)
	seatPeopleInACompanyDirectoryForTest(t, service)
	var offered []map[string]any
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if response, isHandled := localOrganizationMattermostResponse(t, request); isHandled {
			return response, nil
		}
		if isCompanyDirectoryRequest(request) && request.Method == http.MethodPatch {
			var payload struct {
				Profiles []map[string]any `json:"profiles"`
			}
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			offered = payload.Profiles
			return jsonResponse(http.StatusOK, `{"written":["member@example.com"]}`, nil), nil
		}
		if isCompanyDirectoryRequest(request) {
			return companyDirectoryResponse(t, request)
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}

	requestBody := strings.NewReader(`{"profiles":[{"memberID":"user-member","email":"member@example.com","jobTitle":"Designer","groupID":"design"}]}`)
	responseRecorder := httptest.NewRecorder()
	service.localUpdateOrgProfiles(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users/org-profiles", requestBody))

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if len(offered) != 1 {
		t.Fatalf("the directory was offered %#v; a profile edit that never reaches it leaves two org charts", offered)
	}
	if offered[0]["email"] != "member@example.com" || offered[0]["jobTitle"] != "Designer" {
		t.Fatalf("directory profile = %#v", offered[0])
	}
}

func TestOrganizationProfileUpdateStaysLocalWithoutADirectory(t *testing.T) {
	service := newLocalUsersTestService(t)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if response, isHandled := localOrganizationMattermostResponse(t, request); isHandled {
			return response, nil
		}
		if isCompanyDirectoryRequest(request) {
			t.Fatal("a device with no company directory must not call one")
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}

	requestBody := strings.NewReader(`{"profiles":[{"memberID":"user-member","email":"member@example.com","jobTitle":"Designer"}]}`)
	responseRecorder := httptest.NewRecorder()
	service.localUpdateOrgProfiles(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users/org-profiles", requestBody))

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
}
