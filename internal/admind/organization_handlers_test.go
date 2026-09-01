package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOrganizationProfileHandlerPersistsMetadata(t *testing.T) {
	service := newLocalUsersTestService(t)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users?in_team=team-1&per_page=200":
			return jsonResponse(http.StatusOK, `[{"id":"user-2","email":"member@example.com","username":"member-user","nickname":"Member User","roles":"system_user","delete_at":0}]`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members":
			return jsonResponse(http.StatusOK, `[{"user_id":"user-2","roles":"team_user"}]`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://blueclaw.local/admin/api/policy":
			return jsonResponse(http.StatusOK, localUsersPolicyDocument(), nil), nil
		case strings.Contains(request.URL.String(), "/api/agent/key"):
			return jsonResponse(http.StatusNotFound, `{}`, nil), nil
		case request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e":
			return jsonResponse(http.StatusOK, `{"records":[]}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
	requestBody := strings.NewReader(`{"profiles":[{"memberID":"user-member","email":"member@example.com","jobTitle":"Designer","groupID":"design","positionLevel":3,"projectIDs":["brand"],"status":"departed","isOrganizationVisible":false}]}`)
	responseRecorder := httptest.NewRecorder()

	service.localUpdateOrgProfiles(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users/org-profiles", requestBody))

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	profilesByEmail, errorValue := service.readOrganizationProfilesByEmail(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	profile := profilesByEmail["member@example.com"]
	if profile.JobTitle != "Designer" || profile.GroupID != "design" {
		t.Fatalf("profile = %#v", profile)
	}
}

func TestOrganizationProfileHandlerPreservesOmittedMetadata(t *testing.T) {
	service := newLocalUsersTestService(t)
	if errorValue := service.writeOrganizationProfiles(context.Background(), []organizationProfile{{
		MemberID:     "user-member",
		Email:        "member@example.com",
		JobTitle:     "Product Manager",
		GroupID:      "product",
		SupervisorID: "user-admin",
	}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users?in_team=team-1&per_page=200":
			return jsonResponse(http.StatusOK, `[{"id":"user-2","email":"member@example.com","username":"member-user","nickname":"Member User","roles":"system_user","delete_at":0}]`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members":
			return jsonResponse(http.StatusOK, `[{"user_id":"user-2","roles":"team_user"}]`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://blueclaw.local/admin/api/policy":
			return jsonResponse(http.StatusOK, localUsersPolicyDocument(), nil), nil
		case strings.Contains(request.URL.String(), "/api/agent/key"):
			return jsonResponse(http.StatusNotFound, `{}`, nil), nil
		case request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e":
			return jsonResponse(http.StatusOK, `{"records":[]}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
	requestBody := strings.NewReader(`{"profiles":[{"memberID":"user-member","email":"member@example.com","jobTitle":"Lead PM","groupID":"product","supervisorID":"user-ceo"}]}`)
	responseRecorder := httptest.NewRecorder()

	service.localUpdateOrgProfiles(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users/org-profiles", requestBody))

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	profilesByEmail, errorValue := service.readOrganizationProfilesByEmail(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	profile := profilesByEmail["member@example.com"]
	if profile.JobTitle != "Lead PM" || profile.SupervisorID != "user-ceo" {
		t.Fatalf("profile editable fields = %#v", profile)
	}
}

func TestOrganizationProfileHandlerRejectsSelfSupervisor(t *testing.T) {
	service := newLocalUsersTestService(t)
	service.HTTPClient = organizationProfileValidationTestHTTPClient(t)
	if errorValue := service.writeOrganizationProfiles(context.Background(), []organizationProfile{{
		MemberID:     "user-member",
		Email:        "member@example.com",
		SupervisorID: "user-admin",
		Status:       memberStatusActive,
	}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	requestBody := strings.NewReader(`{"profiles":[{"memberID":"user-member","email":"member@example.com","supervisorID":"user-member"}]}`)
	responseRecorder := httptest.NewRecorder()

	service.localUpdateOrgProfiles(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users/org-profiles", requestBody))

	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d; body = %s", responseRecorder.Code, http.StatusBadRequest, responseRecorder.Body.String())
	}
	profilesByEmail, errorValue := service.readOrganizationProfilesByEmail(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if profilesByEmail["member@example.com"].SupervisorID != "user-admin" {
		t.Fatalf("profile = %#v; want existing supervisor preserved", profilesByEmail["member@example.com"])
	}
}

func TestOrganizationProfileHandlerRejectsSupervisorCycleInRequestBatch(t *testing.T) {
	service := newLocalUsersTestService(t)
	service.HTTPClient = organizationProfileValidationTestHTTPClient(t)
	requestBody := strings.NewReader(`{"profiles":[{"memberID":"user-a","email":"a@example.com","supervisorID":"user-b"},{"memberID":"user-b","email":"b@example.com","supervisorID":"user-a"}]}`)
	responseRecorder := httptest.NewRecorder()

	service.localUpdateOrgProfiles(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users/org-profiles", requestBody))

	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d; body = %s", responseRecorder.Code, http.StatusBadRequest, responseRecorder.Body.String())
	}
	profiles, errorValue := service.readOrganizationProfiles(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(profiles) != 0 {
		t.Fatalf("profiles = %#v; want none saved after rejected cycle", profiles)
	}
}

func TestOrganizationProfileHandlerRejectsSupervisorCycleWithExistingProfiles(t *testing.T) {
	service := newLocalUsersTestService(t)
	service.HTTPClient = organizationProfileValidationTestHTTPClient(t)
	if errorValue := service.writeOrganizationProfiles(context.Background(), []organizationProfile{
		{
			MemberID:     "user-a",
			Email:        "a@example.com",
			SupervisorID: "",
			Status:       memberStatusActive,
		},
		{
			MemberID:     "user-b",
			Email:        "b@example.com",
			SupervisorID: "user-a",
			Status:       memberStatusActive,
		},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	requestBody := strings.NewReader(`{"profiles":[{"memberID":"user-a","email":"a@example.com","supervisorID":"user-b"}]}`)
	responseRecorder := httptest.NewRecorder()

	service.localUpdateOrgProfiles(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users/org-profiles", requestBody))

	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d; body = %s", responseRecorder.Code, http.StatusBadRequest, responseRecorder.Body.String())
	}
	profilesByEmail, errorValue := service.readOrganizationProfilesByEmail(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if profilesByEmail["a@example.com"].SupervisorID != "" {
		t.Fatalf("profile = %#v; want existing supervisor preserved", profilesByEmail["a@example.com"])
	}
	if profilesByEmail["b@example.com"].SupervisorID != "user-a" {
		t.Fatalf("profile = %#v; want existing supervisor preserved", profilesByEmail["b@example.com"])
	}
}

func organizationProfileValidationTestHTTPClient(t *testing.T) *http.Client {
	t.Helper()
	return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users?in_team=team-1&per_page=200":
			return jsonResponse(http.StatusOK, `[]`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members":
			return jsonResponse(http.StatusOK, `[]`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://blueclaw.local/admin/api/policy":
			return jsonResponse(http.StatusOK, localUsersPolicyDocument(), nil), nil
		case strings.Contains(request.URL.String(), "/api/agent/key"):
			return jsonResponse(http.StatusNotFound, `{}`, nil), nil
		case request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e":
			return jsonResponse(http.StatusOK, `{"records":[]}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
}

func TestOrganizationGroupHandlerPersistsGroups(t *testing.T) {
	service := newLocalUsersTestService(t)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users?in_team=team-1&per_page=200":
			return jsonResponse(http.StatusOK, `[{"id":"user-2","email":"member@example.com","username":"member-user","nickname":"Member User","roles":"system_user","delete_at":0}]`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members":
			return jsonResponse(http.StatusOK, `[{"user_id":"user-2","roles":"team_user"}]`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://blueclaw.local/admin/api/policy":
			return jsonResponse(http.StatusOK, localUsersPolicyDocument(), nil), nil
		case strings.Contains(request.URL.String(), "/api/agent/key"):
			return jsonResponse(http.StatusNotFound, `{}`, nil), nil
		case request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e":
			return jsonResponse(http.StatusOK, `{"records":[]}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
	requestBody := strings.NewReader(`{"groups":[{"id":"product","name":"제품"},{"id":"growth","name":"성장","parentID":"product"}]}`)
	responseRecorder := httptest.NewRecorder()

	service.localSetOrgGroups(responseRecorder, httptest.NewRequest(http.MethodPut, "/admin/api/org-groups", requestBody))

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	var response pagesUsersResponse
	if errorValue := json.Unmarshal(responseRecorder.Body.Bytes(), &response); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(response.AvailableGroups) != 2 || response.AvailableGroups[0].ID != "product" || response.AvailableGroups[1].ID != "growth" {
		t.Fatalf("available groups = %#v; want product and growth", response.AvailableGroups)
	}
	if response.AvailableGroups[1].ParentID != "product" {
		t.Fatalf("growth parent ID = %q; want product", response.AvailableGroups[1].ParentID)
	}
}

func TestOrganizationGroupHandlerRejectsHierarchyCycle(t *testing.T) {
	service := newLocalUsersTestService(t)
	requestBody := strings.NewReader(`{"groups":[{"id":"product","name":"제품","parentID":"growth"},{"id":"growth","name":"성장","parentID":"product"}]}`)
	responseRecorder := httptest.NewRecorder()

	service.localSetOrgGroups(responseRecorder, httptest.NewRequest(http.MethodPut, "/admin/api/org-groups", requestBody))

	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d; body = %s", responseRecorder.Code, http.StatusBadRequest, responseRecorder.Body.String())
	}
}
