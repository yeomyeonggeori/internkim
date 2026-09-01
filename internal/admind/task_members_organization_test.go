package admind

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTaskMembersOnlyIncludeOrganizationChartPeople(t *testing.T) {
	service := newLocalUsersTestService(t)
	if errorValue := service.writeOrganizationProfiles(context.Background(), []organizationProfile{
		{
			MemberID: "user-1",
			Email:    "ada@example.com",
			Status:   memberStatusActive,
		},
		{
			MemberID: "user-3",
			Email:    "hidden@example.com",
			Status:   memberStatusDeparted,
		},
		{
			MemberID: "user-4",
			Email:    "resigned@example.com",
			Status:   memberStatusDeparted,
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
					{"personID":"agent","displayName":"김인턴","emails":["bot@example.com"],"circles":["member"],"isAdmin":false}
				],
				"circles":[{"circleID":"member","displayName":"Member"}]
			}`, nil), nil
		case strings.Contains(request.URL.String(), "/api/agent/key"):
			return jsonResponse(http.StatusNotFound, `{}`, nil), nil
		case request.URL.String() == "https://api.example.test/api/users?fleet_id=dc719d8e":
			return jsonResponse(http.StatusOK, `{"records":[
				{"memberID":"user-ada","email":"ada@example.com","name":"Ada Kim","role":"member"},
				{"memberID":"user-grace","email":"grace@example.com","name":"Grace Lee","role":"member"},
				{"memberID":"user-hidden","email":"hidden@example.com","name":"Hidden Lee","role":"member"},
				{"memberID":"user-resigned","email":"resigned@example.com","name":"Resigned Park","role":"member"}
			]}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	request := httptest.NewRequest(http.MethodGet, "/flow/api/state", nil)
	request.Header.Set("X-Forwarded-Email", "grace@example.com")
	emails := map[string]bool{}
	for _, member := range service.taskMembers(request) {
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
