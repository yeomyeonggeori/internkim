package admind

import (
	"context"
	"net/http"
	"testing"
)

func TestAddMattermostNameFieldsSplitsKoreanName(t *testing.T) {
	body := map[string]string{}
	addMattermostNameFields(body, "김민수")

	if body["first_name"] != "민수" {
		t.Errorf("first_name = %q; want %q", body["first_name"], "민수")
	}
	if body["last_name"] != "김" {
		t.Errorf("last_name = %q; want %q", body["last_name"], "김")
	}
	if body["nickname"] != "김민수" {
		t.Errorf("nickname = %q; want %q", body["nickname"], "김민수")
	}
}

func TestAddMattermostNameFieldsSplitsEnglishName(t *testing.T) {
	body := map[string]string{}
	addMattermostNameFields(body, "Ada Lovelace")

	if body["first_name"] != "Ada" {
		t.Errorf("first_name = %q; want %q", body["first_name"], "Ada")
	}
	if body["last_name"] != "Lovelace" {
		t.Errorf("last_name = %q; want %q", body["last_name"], "Lovelace")
	}
	if body["nickname"] != "Ada" {
		t.Errorf("nickname = %q; want %q", body["nickname"], "Ada")
	}
}

func TestAddMattermostNameFieldsSingleTokenOmitsLastName(t *testing.T) {
	body := map[string]string{}
	addMattermostNameFields(body, "Madonna")

	if body["first_name"] != "Madonna" {
		t.Errorf("first_name = %q; want %q", body["first_name"], "Madonna")
	}
	if _, hasLastName := body["last_name"]; hasLastName {
		t.Errorf("last_name should be absent for single-token name; got %q", body["last_name"])
	}
	if body["nickname"] != "Madonna" {
		t.Errorf("nickname = %q; want %q", body["nickname"], "Madonna")
	}
}

func TestAddMattermostNameFieldsEmptyNameWritesNothing(t *testing.T) {
	body := map[string]string{}
	addMattermostNameFields(body, "   ")

	if len(body) != 0 {
		t.Errorf("empty name should write no fields; got %#v", body)
	}
}

func TestAllowedMattermostUsersFallsBackToActiveUsersWhenDeviceAuthIsAbsent(t *testing.T) {
	service := NewService(Configuration{MattermostBaseURL: "http://mattermost.local", FleetIDPath: t.TempDir() + "/missing-fleet-id", FleetSecretPath: t.TempDir() + "/missing-fleet-secret"})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "http://mattermost.local/api/v4/users?per_page=200" {
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		}
		return jsonResponse(http.StatusOK, `[
			{"id":"admin","username":"admin","delete_at":0},
			{"id":"agent-1","username":"internkim01","delete_at":0,"is_bot":true},
			{"id":"admin-1","username":"admin01","delete_at":0},
			{"id":"deleted-1","username":"deleted01","delete_at":10}
		]`, nil), nil
	})}

	users, errorValue := service.allowedMattermostUsers(context.Background(), "admin-token")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(users) != 1 || users[0].ID != "admin-1" {
		t.Fatalf("expected active tenant users without device auth, got %+v", users)
	}
}

func TestProtectedMattermostUsersIncludeEveryBot(t *testing.T) {
	if !isProtectedMattermostUser(mattermostUserRecord{Username: "custom-agent", IsBot: true}) {
		t.Fatal("expected bot account to be protected from people projections")
	}
	if isProtectedMattermostUser(mattermostUserRecord{Username: "admin15"}) {
		t.Fatal("expected human tenant admin to remain visible")
	}
}

func TestDefaultChannelMembershipsRunWithoutDeviceAuth(t *testing.T) {
	service := NewService(Configuration{MattermostBaseURL: "http://mattermost.local", FleetIDPath: t.TempDir() + "/missing-fleet-id", FleetSecretPath: t.TempDir() + "/missing-fleet-secret"})
	joinedChannelsByUserID := map[string]map[string]bool{}
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.String() == "http://mattermost.local/api/v4/users?in_team=team-1&per_page=200":
			return jsonResponse(http.StatusOK, `[
				{"id":"agent-1","username":"internkim01","delete_at":0},
				{"id":"admin-1","username":"admin01","delete_at":0}
			]`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/teams/team-1/members" && request.Method == http.MethodPost:
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case isMattermostDefaultChannelMemberRequest(request):
			userID := mattermostChannelMemberUserID(t, request)
			channelID := request.URL.Path[len("/api/v4/channels/") : len(request.URL.Path)-len("/members")]
			if joinedChannelsByUserID[userID] == nil {
				joinedChannelsByUserID[userID] = map[string]bool{}
			}
			joinedChannelsByUserID[userID][channelID] = true
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/flow-channel/posts?per_page=100":
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/calendar-channel/posts?per_page=100":
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
		case request.URL.String() == "http://mattermost.local/api/v4/channels/attendance-channel/posts?per_page=100":
			return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})}

	channelIDs := []string{"flow-channel", "calendar-channel", "attendance-channel", "town-square-channel", "off-topic-channel"}
	if errorValue := service.ensureMattermostDefaultChannelMemberships(context.Background(), "admin-token", "team-1", channelIDs); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, userID := range []string{"agent-1", "admin-1"} {
		for _, channelID := range channelIDs {
			if !joinedChannelsByUserID[userID][channelID] {
				t.Fatalf("user %s was not joined to %s: %#v", userID, channelID, joinedChannelsByUserID)
			}
		}
	}
}

func isMattermostDefaultChannelMemberRequest(request *http.Request) bool {
	return request.Method == http.MethodPost &&
		len(request.URL.Path) > len("/api/v4/channels//members") &&
		request.URL.Path[:len("/api/v4/channels/")] == "/api/v4/channels/" &&
		request.URL.Path[len(request.URL.Path)-len("/members"):] == "/members"
}

func TestMattermostSyncedPersonCirclesUsesStaffAndChannelMembership(t *testing.T) {
	person := map[string]any{
		"emails": []any{"minsu@example.com", "other@example.com"},
	}
	circles := mattermostSyncedPersonCircles(person, map[string]map[string]bool{
		"finance":        {"minsu@example.com": true},
		"representative": {"someone@example.com": true},
	})

	if !containsMattermostTestString(circles, "staff") || !containsMattermostTestString(circles, "finance") {
		t.Fatalf("expected staff and finance circles, got %+v", circles)
	}
	if containsMattermostTestString(circles, "representative") {
		t.Fatalf("expected non-member representative circle omitted, got %+v", circles)
	}
}

func TestMattermostSyncedPersonCirclesKeepsAdminFromPolicy(t *testing.T) {
	person := map[string]any{
		"emails":  []any{"owner@example.com"},
		"isAdmin": true,
	}
	circles := mattermostSyncedPersonCircles(person, map[string]map[string]bool{})

	if !containsMattermostTestString(circles, "staff") || !containsMattermostTestString(circles, "admin") {
		t.Fatalf("expected staff and admin circles, got %+v", circles)
	}
}

func TestMattermostSyncedPersonCirclesPreservesUnmanagedCircles(t *testing.T) {
	person := map[string]any{
		"emails":  []any{"owner@example.com"},
		"circles": []any{"staff", "lab", "c-level"},
	}
	circles := mattermostSyncedPersonCircles(person, map[string]map[string]bool{
		"c-level": {"someone@example.com": true},
	})

	if !containsMattermostTestString(circles, "staff") || !containsMattermostTestString(circles, "lab") {
		t.Fatalf("expected staff and unmanaged lab circles, got %+v", circles)
	}
	if containsMattermostTestString(circles, "c-level") {
		t.Fatalf("expected managed c-level to come only from Mattermost membership, got %+v", circles)
	}
}

func TestMattermostCircleChannelDefinitionsFromPolicy(t *testing.T) {
	circleChannels := mattermostCircleChannelDefinitionsFromPolicy(map[string]any{
		"circleSync": map[string]any{
			"mattermostPrivateChannels": []any{
				map[string]any{"circleID": "HR-Compensation", "channelName": "Circle-HR-Compensation"},
			},
		},
	})

	if len(circleChannels) != 1 {
		t.Fatalf("expected one circle channel, got %+v", circleChannels)
	}
	if circleChannels[0].CircleID != "hr-compensation" || circleChannels[0].ChannelName != "circle-hr-compensation" {
		t.Fatalf("expected normalized circle channel, got %+v", circleChannels)
	}
}

func containsMattermostTestString(values []string, expectedValue string) bool {
	for _, value := range values {
		if value == expectedValue {
			return true
		}
	}
	return false
}
