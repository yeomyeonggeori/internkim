package centralplane

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTheFlowMirrorDoesNotAskForCalendarEvents(t *testing.T) {
	asked := ""
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api/agent/session" {
			writeJSON(writer, map[string]any{"memberID": "member-1", "accessToken": "token-1", "expiresAt": 4102444800})
			return
		}
		asked = request.URL.Query().Get("is_event")
		writeJSON(writer, []any{})
	}))
	defer server.Close()
	client := New(Settings{AppURL: server.URL, AgentAPIKey: "agent-key", ProjectURL: server.URL, PublishableKey: "publishable-key"})

	if _, errorValue := client.TasksChangedSince(context.Background(), "mattermost", "owner-account", "", 50); errorValue != nil {
		t.Fatal(errorValue)
	}

	if asked != "eq.false" {
		t.Fatalf("an event carries the device's calendar uid, so a mirror that reads one makes a flow task out of a meeting: is_event=%q", asked)
	}
}
