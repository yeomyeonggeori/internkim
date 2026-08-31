package admind

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// A circle channel that is not there says nothing about who belongs to the
// circle. Creating it to answer the question makes the answer "nobody", and the
// sync then takes the circle away from every person who had it.
func TestCircleEmailsSkipsACircleWithNoChannelRatherThanCreatingOne(t *testing.T) {
	createdChannels := []string{}
	service := NewService(Configuration{MattermostBaseURL: "http://mattermost.local"})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		path := request.URL.Path
		switch {
		case path == "/admin/api/policy":
			return jsonResponse(http.StatusOK, `{"circleSync":{"mattermostPrivateChannels":[{"circleID":"c-level","channelName":"circle-c-level"},{"circleID":"representative","channelName":"circle-representative"}]}}`, nil), nil
		case request.Method == http.MethodPost && path == "/api/v4/channels":
			createdChannels = append(createdChannels, path)
			return jsonResponse(http.StatusCreated, `{"id":"channel-new"}`, nil), nil
		case strings.HasSuffix(path, "/channels/name/circle-c-level"):
			return jsonResponse(http.StatusOK, `{"id":"channel-c-level"}`, nil), nil
		case strings.Contains(path, "/channels/name/"):
			return jsonResponse(http.StatusNotFound, `{"id":"store.sql_channel.get_by_name.missing.app_error"}`, nil), nil
		case path == "/api/v4/channels/channel-c-level/members":
			return jsonResponse(http.StatusOK, `[{"user_id":"user-1"}]`, nil), nil
		case path == "/api/v4/users/user-1":
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"member@example.com"}`, nil), nil
		}
		t.Fatalf("unexpected request %s %s", request.Method, path)
		return nil, nil
	})}

	circleEmailsByID, errorValue := service.mattermostCircleEmails(context.Background(), "token", "team-1")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(createdChannels) != 0 {
		t.Errorf("reading circle membership created %v", createdChannels)
	}
	if _, isPresent := circleEmailsByID["representative"]; isPresent {
		t.Error("a circle with no channel was reported as having no members, which takes it away from everyone")
	}
	if !circleEmailsByID["c-level"]["member@example.com"] {
		t.Errorf("c-level emails = %v", circleEmailsByID["c-level"])
	}
}

func TestAPersonKeepsACircleWhoseChannelIsGone(t *testing.T) {
	person := map[string]any{
		"emails":  []any{"member@example.com"},
		"circles": []any{"member", "representative", "c-level"},
	}
	circleEmailsByID := map[string]map[string]bool{"c-level": {"member@example.com": true}}

	circles := mattermostSyncedPersonCircles(person, circleEmailsByID)

	if !containsCircle(circles, "representative") {
		t.Errorf("circles = %v, representative was dropped because its channel was not read", circles)
	}
	if !containsCircle(circles, "c-level") {
		t.Errorf("circles = %v", circles)
	}
}

func containsCircle(circles []string, circleID string) bool {
	for _, circle := range circles {
		if circle == circleID {
			return true
		}
	}
	return false
}
