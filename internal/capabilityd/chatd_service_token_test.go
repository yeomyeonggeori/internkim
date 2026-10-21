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

func TestCapabilitydCarriesTheServiceTokenToBlueclawAndAdmind(t *testing.T) {
	tokenPath := filepath.Join(t.TempDir(), "service-token")
	if errorValue := os.WriteFile(tokenPath, []byte("token-of-the-host-services\n"), 0o440); errorValue != nil {
		t.Fatal(errorValue)
	}
	authorizations := map[string]string{}
	recording := func(name string) *httptest.Server {
		server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			authorizations[name] = request.Header.Get("Authorization")
			_, _ = responseWriter.Write([]byte(`{"people":[]}`))
		}))
		t.Cleanup(server.Close)
		return server
	}
	blueclaw := recording("blueclaw")
	admind := recording("admind")
	service := Service{Configuration: Configuration{BlueclawBaseURL: blueclaw.URL, AdmindBaseURL: admind.URL, ServiceTokenPath: tokenPath}}

	for _, call := range []func() error{
		func() error {
			request, _ := http.NewRequest(http.MethodPost, blueclaw.URL+"/admin/api/policy", nil)
			_, errorValue := service.httpClient().Do(request)
			return errorValue
		},
		func() error {
			request, _ := http.NewRequest(http.MethodGet, admind.URL+"/admin/api/directory/people", nil)
			_, errorValue := service.httpClient().Do(request)
			return errorValue
		},
	} {
		if errorValue := call(); errorValue != nil {
			t.Fatal(errorValue)
		}
	}

	if authorizations["blueclaw"] != "Bearer token-of-the-host-services" || authorizations["admind"] != "Bearer token-of-the-host-services" {
		t.Fatalf("the services were asked with %v", authorizations)
	}
}
