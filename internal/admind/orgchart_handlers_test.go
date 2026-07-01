package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOrgchartProfileHandlerPersistsMetadata(t *testing.T) {
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
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
	requestBody := strings.NewReader(`{"profiles":[{"userID":"user-member","email":"member@example.com","jobTitle":"Designer","group":"design","positionLevel":3,"projectIDs":["brand"],"employmentStatus":"resigned","isOrgchartVisible":false}]}`)
	responseRecorder := httptest.NewRecorder()

	service.localUpdateOrgProfiles(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users/org-profiles", requestBody))

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	profilesByEmail, errorValue := service.readOrgchartProfilesByEmail(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	profile := profilesByEmail["member@example.com"]
	if profile.JobTitle != "Designer" || profile.PrimaryGroupID != "design" {
		t.Fatalf("profile = %#v", profile)
	}
	if profile.PositionLevel != 0 || len(profile.ProjectIDs) != 0 || profile.TeamRole != "" {
		t.Fatalf("unsupported profile fields = %#v; want ignored", profile)
	}
	if profile.EmploymentStatus != orgchartEmploymentStatusActive || !profile.IsOrgchartVisible {
		t.Fatalf("profile defaults = %#v; want active visible defaults", profile)
	}
}

func TestOrgchartProfileHandlerPreservesOmittedMetadata(t *testing.T) {
	service := newLocalUsersTestService(t)
	if errorValue := service.writeOrgchartProfiles(context.Background(), []orgchartProfile{{
		UserID:            "user-member",
		Email:             "member@example.com",
		JobTitle:          "Product Manager",
		PositionLevel:     2,
		PrimaryGroupID:    "product",
		GroupIDs:          []string{"product", "growth"},
		SupervisorID:      "user-admin",
		ProjectIDs:        []string{"new-business"},
		TeamRole:          "제품 일정 관리",
		EmploymentStatus:  orgchartEmploymentStatusLeave,
		IsOrgchartVisible: false,
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
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
	requestBody := strings.NewReader(`{"profiles":[{"userID":"user-member","email":"member@example.com","jobTitle":"Lead PM","group":"product","supervisorID":"user-ceo"}]}`)
	responseRecorder := httptest.NewRecorder()

	service.localUpdateOrgProfiles(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users/org-profiles", requestBody))

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	profilesByEmail, errorValue := service.readOrgchartProfilesByEmail(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	profile := profilesByEmail["member@example.com"]
	if profile.JobTitle != "Lead PM" || profile.SupervisorID != "user-ceo" {
		t.Fatalf("profile editable fields = %#v", profile)
	}
	if profile.PositionLevel != 2 || profile.EmploymentStatus != orgchartEmploymentStatusLeave || profile.IsOrgchartVisible {
		t.Fatalf("profile preserved scalar fields = %#v", profile)
	}
	if strings.Join(profile.GroupIDs, ",") != "product,growth" {
		t.Fatalf("group ids = %#v; want product and growth", profile.GroupIDs)
	}
	if strings.Join(profile.ProjectIDs, ",") != "new-business" {
		t.Fatalf("project ids = %#v; want new-business", profile.ProjectIDs)
	}
	if profile.TeamRole != "제품 일정 관리" {
		t.Fatalf("team role = %q; want preserved team role", profile.TeamRole)
	}
}

func TestOrgchartProfileHandlerRejectsSelfSupervisor(t *testing.T) {
	service := newLocalUsersTestService(t)
	service.HTTPClient = orgchartProfileValidationTestHTTPClient(t)
	if errorValue := service.writeOrgchartProfiles(context.Background(), []orgchartProfile{{
		UserID:            "user-member",
		Email:             "member@example.com",
		SupervisorID:      "user-admin",
		EmploymentStatus:  orgchartEmploymentStatusActive,
		IsOrgchartVisible: true,
	}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	requestBody := strings.NewReader(`{"profiles":[{"userID":"user-member","email":"member@example.com","supervisorID":"user-member"}]}`)
	responseRecorder := httptest.NewRecorder()

	service.localUpdateOrgProfiles(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users/org-profiles", requestBody))

	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d; body = %s", responseRecorder.Code, http.StatusBadRequest, responseRecorder.Body.String())
	}
	profilesByEmail, errorValue := service.readOrgchartProfilesByEmail(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if profilesByEmail["member@example.com"].SupervisorID != "user-admin" {
		t.Fatalf("profile = %#v; want existing supervisor preserved", profilesByEmail["member@example.com"])
	}
}

func TestOrgchartProfileHandlerRejectsSupervisorCycleInRequestBatch(t *testing.T) {
	service := newLocalUsersTestService(t)
	service.HTTPClient = orgchartProfileValidationTestHTTPClient(t)
	requestBody := strings.NewReader(`{"profiles":[{"userID":"user-a","email":"a@example.com","supervisorID":"user-b"},{"userID":"user-b","email":"b@example.com","supervisorID":"user-a"}]}`)
	responseRecorder := httptest.NewRecorder()

	service.localUpdateOrgProfiles(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users/org-profiles", requestBody))

	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d; body = %s", responseRecorder.Code, http.StatusBadRequest, responseRecorder.Body.String())
	}
	profiles, errorValue := service.readOrgchartProfiles(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(profiles) != 0 {
		t.Fatalf("profiles = %#v; want none saved after rejected cycle", profiles)
	}
}

func TestOrgchartProfileHandlerRejectsSupervisorCycleWithExistingProfiles(t *testing.T) {
	service := newLocalUsersTestService(t)
	service.HTTPClient = orgchartProfileValidationTestHTTPClient(t)
	if errorValue := service.writeOrgchartProfiles(context.Background(), []orgchartProfile{
		{
			UserID:            "user-a",
			Email:             "a@example.com",
			SupervisorID:      "",
			EmploymentStatus:  orgchartEmploymentStatusActive,
			IsOrgchartVisible: true,
		},
		{
			UserID:            "user-b",
			Email:             "b@example.com",
			SupervisorID:      "user-a",
			EmploymentStatus:  orgchartEmploymentStatusActive,
			IsOrgchartVisible: true,
		},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	requestBody := strings.NewReader(`{"profiles":[{"userID":"user-a","email":"a@example.com","supervisorID":"user-b"}]}`)
	responseRecorder := httptest.NewRecorder()

	service.localUpdateOrgProfiles(responseRecorder, httptest.NewRequest(http.MethodPost, "/admin/api/users/org-profiles", requestBody))

	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d; body = %s", responseRecorder.Code, http.StatusBadRequest, responseRecorder.Body.String())
	}
	profilesByEmail, errorValue := service.readOrgchartProfilesByEmail(context.Background())
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

func orgchartProfileValidationTestHTTPClient(t *testing.T) *http.Client {
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
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
}

func TestOrgchartGroupHandlerPersistsGroups(t *testing.T) {
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
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}
	requestBody := strings.NewReader(`{"groups":[{"id":"product","name":"제품"},{"id":"growth","name":"성장"}]}`)
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
}
