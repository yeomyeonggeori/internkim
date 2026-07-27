package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalListUsersMergesOrganizationMetadata(t *testing.T) {
	service := newLocalUsersTestService(t)
	ctx := context.Background()
	if errorValue := service.writeOrganizationGroups(ctx, []orgGroupRecord{{ID: "product", Name: "제품"}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeOrganizationProfiles(ctx, []organizationProfile{{
		UserID:            "user-member",
		Email:             "member@example.com",
		JobTitle:          "Product Manager",
		PositionLevel:     2,
		PrimaryGroupID:    "product",
		GroupIDs:          []string{"product"},
		SupervisorID:      "user-admin",
		ProjectIDs:        []string{"new-business"},
		TeamRole:          "제품 일정 관리",
		EmploymentStatus:  organizationEmploymentStatusActive,
		IsOrganizationVisible: true,
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
			return jsonResponse(http.StatusOK, `[
				{"id":"user-1","email":"admin@example.com","username":"admin-user","nickname":"Admin User","roles":"system_user","delete_at":0},
				{"id":"user-2","email":"member@example.com","username":"member-user","nickname":"Member User","roles":"system_user","delete_at":0}
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
	if len(response.AvailableGroups) != 1 || response.AvailableGroups[0].ID != "product" {
		t.Fatalf("available groups = %#v; want product", response.AvailableGroups)
	}
	member := response.Records[1]
	if member.JobTitle != "Product Manager" || member.PrimaryGroupID != "product" || member.SupervisorID != "user-admin" {
		t.Fatalf("member org fields = %#v", member)
	}
	if member.PositionLevel != 0 || len(member.ProjectIDs) != 0 || member.TeamRole != "" || member.EmploymentStatus != "" || member.IsOrganizationVisible {
		t.Fatalf("unsupported response fields = %#v; want omitted metadata", member)
	}
}

func TestLocalListUsersUsesEmptyOrganizationGroupsOverBlueclawGroups(t *testing.T) {
	service := newLocalUsersTestService(t)
	if errorValue := service.writeOrganizationGroups(context.Background(), []orgGroupRecord{}); errorValue != nil {
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
			return jsonResponse(http.StatusOK, localUsersPolicyDocumentWithOrgGroups(), nil), nil
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
	if len(response.AvailableGroups) != 0 {
		t.Fatalf("available groups = %#v; want empty organization groups", response.AvailableGroups)
	}
}

func TestLocalUsersResponseBodyFallsBackWhenOrganizationMetadataFails(t *testing.T) {
	service := newLocalUsersTestService(t)
	service.Configuration.StateDirectory = writeTestFile(t, "not a directory")
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet && request.URL.String() == "http://blueclaw.local/admin/api/policy" {
			return jsonResponse(http.StatusOK, localUsersPolicyDocument(), nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}

	responseBody, errorValue := service.localUsersResponseBody(context.Background(), pagesUsersResponse{Records: []adminUserMutation{{
		UserID: "user-member",
		Email:  "member@example.com",
		Name:   "Member User",
		Role:   "member",
	}}})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var response pagesUsersResponse
	if errorValue := json.Unmarshal(responseBody, &response); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(response.Records) != 1 || response.Records[0].Email != "member@example.com" {
		t.Fatalf("records = %#v; want original response", response.Records)
	}
}

func TestLocalListUsersStartsWithoutLegacyBlueclawGroups(t *testing.T) {
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
			return jsonResponse(http.StatusOK, localUsersPolicyDocumentWithOrgGroups(), nil), nil
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
	if len(response.AvailableGroups) != 0 {
		t.Fatalf("available groups = %#v; want no user-created groups", response.AvailableGroups)
	}
	groups, errorValue := service.readOrganizationGroups(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(groups) != 0 {
		t.Fatalf("stored groups = %#v; want no imported legacy groups", groups)
	}
}

func localUsersPolicyDocumentWithOrgGroups() string {
	return `{
		"people":[
			{"personID":"user-member","displayName":"Member User","emails":["member@example.com"],"circles":["staff"],"isAdmin":false}
		],
		"circles":[{"circleID":"staff","displayName":"Staff"}],
		"orgGroups":[{"id":"legacy","name":"Legacy"}],
		"circleSync":{"mattermostPrivateChannels":[{"circleID":"staff","channelName":"circle-staff"}]}
	}`
}
