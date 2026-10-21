package servicetoken

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestARequestCarriesTheTokenItsServiceIsPointedAt(t *testing.T) {
	tokenPath := filepath.Join(t.TempDir(), "service-token")
	if errorValue := os.WriteFile(tokenPath, []byte("token-of-the-host-services\n"), 0o440); errorValue != nil {
		t.Fatal(errorValue)
	}
	request, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:18090/v1/platform/buzz/message.post", nil)

	if errorValue := Authorize(request, tokenPath); errorValue != nil {
		t.Fatal(errorValue)
	}

	if request.Header.Get("Authorization") != "Bearer token-of-the-host-services" {
		t.Fatalf("the request carried %q", request.Header.Get("Authorization"))
	}
}

func TestATokenThatCannotBeReadStopsTheRequest(t *testing.T) {
	request, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:18090/v1/platform/buzz/message.post", nil)

	if Authorize(request, filepath.Join(t.TempDir(), "missing")) == nil {
		t.Fatal("a token nobody could read was taken as sent")
	}
}

func aTokenFile(t *testing.T) string {
	t.Helper()
	tokenPath := filepath.Join(t.TempDir(), "service-token")
	if errorValue := os.WriteFile(tokenPath, []byte("token-of-the-host-services\n"), 0o440); errorValue != nil {
		t.Fatal(errorValue)
	}
	return tokenPath
}

func TestAClientCarriesTheTokenToTheHostServicesAndNowhereElse(t *testing.T) {
	seen := map[string]string{}
	recordingServer := func(name string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
			seen[name] = request.Header.Get("Authorization")
		}))
	}
	blueclaw := recordingServer("blueclaw")
	defer blueclaw.Close()
	elsewhere := recordingServer("elsewhere")
	defer elsewhere.Close()
	client := Client(&http.Client{}, aTokenFile(t), blueclaw.URL)

	for _, target := range []string{blueclaw.URL + "/admin/api/run/approve", elsewhere.URL + "/v1/chat/completions"} {
		response, errorValue := client.Post(target, "application/json", nil)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		response.Body.Close()
	}

	if seen["blueclaw"] != "Bearer token-of-the-host-services" || seen["elsewhere"] != "" {
		t.Fatalf("the token went to %v", seen)
	}
}

func TestOnlyTheTokenItselfIsTakenAsCarried(t *testing.T) {
	tokenPath := aTokenFile(t)
	for authorization, expected := range map[string]bool{
		"Bearer token-of-the-host-services": true,
		"":                                  false,
		"Bearer ":                           false,
		"Bearer token-of-the-host":          false,
		"token-of-the-host-services":        false,
	} {
		request := httptest.NewRequest(http.MethodPost, "/admin/api/runs/api/approve", nil)
		request.Header.Set("Authorization", authorization)
		if IsCarriedBy(request, tokenPath) != expected {
			t.Fatalf("%q was taken as carried=%v", authorization, !expected)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.Header.Set("Authorization", "Bearer ")
	if IsCarriedBy(request, filepath.Join(t.TempDir(), "missing")) {
		t.Fatal("an empty bearer matched a token nobody could read")
	}
}
