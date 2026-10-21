package capabilityd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestEveryChatdCallCarriesTheServiceToken(t *testing.T) {
	tokenPath := filepath.Join(t.TempDir(), "service-token")
	if errorValue := os.WriteFile(tokenPath, []byte("token-of-the-host-services\n"), 0o440); errorValue != nil {
		t.Fatal(errorValue)
	}
	authorizations := []string{}
	chatd := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		authorizations = append(authorizations, request.Header.Get("Authorization"))
		_, _ = responseWriter.Write([]byte(`{}`))
	}))
	t.Cleanup(chatd.Close)
	service := Service{Configuration: Configuration{ChatdEndpoint: chatd.URL, ChatdPlatform: "buzz", ServiceTokenPath: tokenPath}}

	if errorValue := service.chatdRequest(context.Background(), "message.post", map[string]string{"message": "안녕하세요"}, nil); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(authorizations) != 1 || authorizations[0] != "Bearer token-of-the-host-services" {
		t.Fatalf("chatd was asked with %q", authorizations)
	}
}
