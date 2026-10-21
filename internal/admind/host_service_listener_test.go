package admind

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func admindHoldingAServiceToken(t *testing.T) (*Service, http.Handler, *[]string) {
	t.Helper()
	tokenPath := filepath.Join(t.TempDir(), "service-token")
	if errorValue := os.WriteFile(tokenPath, []byte("token-of-the-host-services\n"), 0o440); errorValue != nil {
		t.Fatal(errorValue)
	}
	service := NewService(Configuration{ServiceTokenPath: tokenPath})
	reached := []string{}
	handler := service.answeringOnlyTheHostServices(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		reached = append(reached, request.URL.Path)
	}))
	return service, handler, &reached
}

func TestTheAdminPortRefusesAnyoneWithoutTheServiceTokenWhateverEmailTheyForward(t *testing.T) {
	_, handler, reached := admindHoldingAServiceToken(t)
	for _, path := range []string{"/runs/api/approve", "/admin/api/buzz/signing-secret", "/admin/api/directory/direct-message", "/admin/api/directory/people"} {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"taskRunID":"run-1","decision":"approve","pubkeyHex":"`+strings.Repeat("2", 64)+`"}`))
		request.RemoteAddr = "127.0.0.1:41234"
		request.Header.Set("X-Forwarded-Email", "requester@example.com")
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("a loopback caller with a forwarded email and no token answered %d on %s, want 401", recorder.Code, path)
		}
	}
	if len(*reached) != 0 {
		t.Fatalf("a caller without the token reached %v", *reached)
	}
}

func TestAHostServiceWithTheTokenAndAReadinessProbeWithoutOneGetThrough(t *testing.T) {
	_, handler, reached := admindHoldingAServiceToken(t)
	authorized := httptest.NewRequest(http.MethodPost, "/admin/api/directory/people", nil)
	authorized.Header.Set("Authorization", "Bearer token-of-the-host-services")

	handler.ServeHTTP(httptest.NewRecorder(), authorized)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/admin/api/health", nil))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/admin/api/health/roster", nil))

	if strings.Join(*reached, " ") != "/admin/api/directory/people /admin/api/health /admin/api/health/roster" {
		t.Fatalf("reached %v", *reached)
	}
}

func TestAdmindCarriesTheTokenToBlueclaw(t *testing.T) {
	tokenPath := filepath.Join(t.TempDir(), "service-token")
	if errorValue := os.WriteFile(tokenPath, []byte("token-of-the-host-services\n"), 0o440); errorValue != nil {
		t.Fatal(errorValue)
	}
	authorizations := []string{}
	blueclaw := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		authorizations = append(authorizations, request.Header.Get("Authorization"))
		_, _ = responseWriter.Write([]byte(`{}`))
	}))
	defer blueclaw.Close()
	service := NewService(Configuration{BlueclawBaseURL: blueclaw.URL, ServiceTokenPath: tokenPath})

	if errorValue := service.blueclawJSONRequest(t.Context(), http.MethodGet, "/admin/api/policy", nil, nil); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(authorizations) != 1 || authorizations[0] != "Bearer token-of-the-host-services" {
		t.Fatalf("blueclaw was asked with %q", authorizations)
	}
}
