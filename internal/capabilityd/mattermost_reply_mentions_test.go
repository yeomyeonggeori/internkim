package capabilityd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeMattermostReplyMentionsUsesRecipientResolver(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/admin/api/identity/resolve-recipient" {
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		}
		var requestBody map[string]string
		if errorValue := json.NewDecoder(request.Body).Decode(&requestBody); errorValue != nil {
			t.Fatal(errorValue)
		}
		if requestBody["hint"] != "김테스트" {
			t.Fatalf("hint = %+v", requestBody)
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(`{"status":"resolved","recipient":{"personID":"person-rain","displayName":"김테스트","emails":["rain@example.com"],"externalUserID":"user-rain","username":"rain"}}`))
	}))
	defer server.Close()
	service := Service{Configuration: Configuration{BlueclawBaseURL: server.URL}}

	message := service.normalizeMattermostReplyMentions(context.Background(), "참여자: @김테스트")

	if message != "참여자: @rain" {
		t.Fatalf("message = %q", message)
	}
}

func TestNormalizeMattermostReplyMentionsSkipsCodeAndEmail(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		called = true
		t.Fatalf("unexpected request %s", request.URL.String())
	}))
	defer server.Close()
	service := Service{Configuration: Configuration{BlueclawBaseURL: server.URL}}

	codeMessage := service.normalizeMattermostReplyMentions(context.Background(), "`@김테스트`")
	emailMessage := service.normalizeMattermostReplyMentions(context.Background(), "mail rain@example.com")

	if codeMessage != "`@김테스트`" || emailMessage != "mail rain@example.com" || called || strings.Contains(emailMessage, "@rain ") {
		t.Fatalf("code=%q email=%q called=%v", codeMessage, emailMessage, called)
	}
}
