package centralplane

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMuteConversationAsksTheRecordForThatMemberAndConversation(t *testing.T) {
	var asked map[string]string
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path = request.URL.Path
		_ = json.NewDecoder(request.Body).Decode(&asked)
		writer.Write([]byte(`{"muted":true}`))
	}))
	defer server.Close()
	client := New(Settings{AppURL: server.URL, HostCredential: func() string { return "key" }, ProjectURL: server.URL, PublishableKey: "publishable"})

	if errorValue := client.MuteConversation(context.Background(), "member-1", "channel-1"); errorValue != nil {
		t.Fatal(errorValue)
	}

	if path != conversationMutePath || asked["memberID"] != "member-1" || asked["conversationID"] != "channel-1" {
		t.Fatalf("asked %s with %v", path, asked)
	}
}

func TestMuteConversationReportsARefusal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	client := New(Settings{AppURL: server.URL, HostCredential: func() string { return "key" }, ProjectURL: server.URL, PublishableKey: "publishable"})

	if errorValue := client.MuteConversation(context.Background(), "member-1", "channel-1"); errorValue == nil {
		t.Fatal("a refused mute should be reported")
	}
}
