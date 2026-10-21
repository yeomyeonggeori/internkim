package admind

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestAdmindAsksChatdWithTheServiceToken(t *testing.T) {
	tokenPath := filepath.Join(t.TempDir(), "service-token")
	if errorValue := os.WriteFile(tokenPath, []byte("token-of-the-host-services\n"), 0o440); errorValue != nil {
		t.Fatal(errorValue)
	}
	authorizations := []string{}
	chatd := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		authorizations = append(authorizations, request.Header.Get("Authorization"))
		_, _ = responseWriter.Write([]byte(`{}`))
	}))
	defer chatd.Close()
	service := NewService(Configuration{ChatdEndpoint: chatd.URL, ChatdPlatform: "buzz", ServiceTokenPath: tokenPath})

	if errorValue := service.chatdPlatformRequest(context.Background(), "dm.ensure", map[string]string{}, nil); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(authorizations) != 1 || authorizations[0] != "Bearer token-of-the-host-services" {
		t.Fatalf("chatd was asked with %q", authorizations)
	}
}
