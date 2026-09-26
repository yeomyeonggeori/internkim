package centralplane

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// SvelteKit negotiates an error's shape on the Accept header and serves an
// HTML page to a caller that asks for nothing (handle_fatal_error).
func TestARecordCallAsksForARefusalItCanRead(t *testing.T) {
	acceptedByTheCall := ""
	acceptedBySignIn := ""
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api/agent/session" {
			acceptedBySignIn = request.Header.Get("Accept")
			writeJSON(writer, map[string]any{
				"memberID":    "member-1",
				"accessToken": "token-1",
				"expiresAt":   4102444800,
			})
			return
		}
		acceptedByTheCall = request.Header.Get("Accept")
		writeJSON(writer, map[string]any{"tool": "attendance_update", "result": map[string]any{"status": "corrected"}})
	}))
	defer server.Close()
	client := New(Settings{
		AppURL:         server.URL,
		HostCredential: func() string { return "agent-key" },
		ProjectURL:     server.URL,
		PublishableKey: "publishable-key",
	})

	answer, errorValue := client.InvokeRecordTool(context.Background(), "member1@example.com", "attendance_update", "invoke", json.RawMessage(`{}`))
	if errorValue != nil {
		t.Fatalf("attendance_update: %v", errorValue)
	}
	if answer.Status != http.StatusOK {
		t.Fatalf("the record answered %d: %s", answer.Status, answer.Body)
	}
	if acceptedByTheCall != "application/json" {
		t.Fatalf("the call accepts %q, so a refusal comes back as a web page", acceptedByTheCall)
	}
	if acceptedBySignIn != "application/json" {
		t.Fatalf("signing in accepts %q, and no request here sets that header itself", acceptedBySignIn)
	}
}
