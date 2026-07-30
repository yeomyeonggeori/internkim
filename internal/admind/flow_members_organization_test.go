package admind

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFlowMembersOnlyIncludeOrganizationChartPeople(t *testing.T) {
	service := newLocalUsersTestService(t)
	if errorValue := service.writeOrganizationProfiles(context.Background(), []organizationProfile{
		{
			UserID:                "user-1",
			Email:                 "ada@example.com",
			EmploymentStatus:      organizationEmploymentStatusActive,
			IsOrganizationVisible: true,
		},
		{
			UserID:                "user-3",
			Email:                 "hidden@example.com",
			EmploymentStatus:      organizationEmploymentStatusActive,
			IsOrganizationVisible: false,
		},
		{
			UserID:                "user-4",
			Email:                 "resigned@example.com",
			EmploymentStatus:      organizationEmploymentStatusResigned,
			IsOrganizationVisible: true,
		},
	}); errorValue != nil {
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
				{"id":"user-1","email":"ada@example.com","username":"ada","nickname":"Ada Kim","roles":"system_user","delete_at":0},
				{"id":"user-2","email":"grace@example.com","username":"grace","nickname":"Grace Lee","roles":"system_user","delete_at":0},
				{"id":"user-3","email":"hidden@example.com","username":"hidden","nickname":"Hidden Lee","roles":"system_user","delete_at":0},
				{"id":"user-4","email":"resigned@example.com","username":"resigned","nickname":"Resigned Park","roles":"system_user","delete_at":0}
			]`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members":
			return jsonResponse(http.StatusOK, `[
				{"user_id":"user-1","roles":"team_user"},
				{"user_id":"user-2","roles":"team_user"},
				{"user_id":"user-3","roles":"team_user"},
				{"user_id":"user-4","roles":"team_user"}
			]`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://blueclaw.local/admin/api/policy":
			return jsonResponse(http.StatusOK, `{
				"people":[
					{"personID":"agent","displayName":"김인턴","emails":["bot@example.com"],"circles":["staff"],"isAdmin":false}
				],
				"circles":[{"circleID":"staff","displayName":"Staff"}]
			}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	request := httptest.NewRequest(http.MethodGet, "/flow/api/state", nil)
	request.Header.Set("X-Forwarded-Email", "grace@example.com")
	emails := map[string]bool{}
	for _, member := range service.flowMembers(request) {
		emails[member.Email] = true
	}

	for _, expectedEmail := range []string{"ada@example.com", "grace@example.com"} {
		if !emails[expectedEmail] {
			t.Fatalf("missing %s in %#v", expectedEmail, emails)
		}
	}
	for _, unexpectedEmail := range []string{"hidden@example.com", "resigned@example.com", "bot@example.com"} {
		if emails[unexpectedEmail] {
			t.Fatalf("kept %s outside the organization chart: %#v", unexpectedEmail, emails)
		}
	}
}
