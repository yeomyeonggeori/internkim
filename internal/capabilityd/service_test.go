package capabilityd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func TestConfigurationDefaultsIncludeAdmindBaseURL(t *testing.T) {
	configuration := Configuration{}.WithDefaults()

	if configuration.AdmindBaseURL != DefaultConfiguration().AdmindBaseURL {
		t.Fatalf("expected admind base url default, got %q", configuration.AdmindBaseURL)
	}
}

func TestHTTPClientTimeoutTracksProviderAttemptTimeout(t *testing.T) {
	service := Service{Configuration: Configuration{}.WithDefaults()}

	if service.httpClientTimeout() != 120*time.Second {
		t.Fatalf("expected general HTTP timeout, got %s", service.httpClientTimeout())
	}

	shortTimeoutService := Service{Configuration: Configuration{ProviderAttemptTimeout: 30 * time.Second}.WithDefaults()}
	if shortTimeoutService.httpClientTimeout() != 120*time.Second {
		t.Fatalf("expected minimum http timeout, got %s", shortTimeoutService.httpClientTimeout())
	}
}

func TestProviderHTTPClientHasNoDefaultTimeout(t *testing.T) {
	service := Service{Configuration: Configuration{}.WithDefaults()}
	if service.providerHTTPClient().Timeout != 0 {
		t.Fatalf("expected provider HTTP client without a default timeout, got %s", service.providerHTTPClient().Timeout)
	}
}

func TestHealthReportsConfiguredChatdAsReadyWhenItAnswers(t *testing.T) {
	chatdServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != blueclaw.ChatdHealthPath {
			responseWriter.WriteHeader(http.StatusNotFound)
			return
		}
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer chatdServer.Close()
	service := Service{
		Configuration: Configuration{ChatdEndpoint: chatdServer.URL, ChatdPlatform: "buzz"},
		HTTPClient:    chatdServer.Client(),
	}

	chatd := healthProvider(t, service, "chatd")
	if chatd["configured"] != true || chatd["available"] != true {
		t.Fatalf("expected chatd to be configured and available, got %+v", chatd)
	}
}

func TestHealthReportsConfiguredChatdAsUnreadyWhenItDoesNotAnswer(t *testing.T) {
	chatdServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer chatdServer.Close()
	service := Service{
		Configuration: Configuration{ChatdEndpoint: chatdServer.URL, ChatdPlatform: "buzz"},
		HTTPClient:    chatdServer.Client(),
	}

	chatd := healthProvider(t, service, "chatd")
	if chatd["configured"] != true {
		t.Fatalf("expected chatd to be configured, got %+v", chatd)
	}
	if chatd["available"] != false {
		t.Fatalf("expected chatd to be unavailable, got %+v", chatd)
	}
}

func TestHealthReportsChatdUnconfiguredWithoutAnEndpoint(t *testing.T) {
	service := Service{Configuration: Configuration{}}

	chatd := healthProvider(t, service, "chatd")
	if chatd["configured"] != false {
		t.Fatalf("expected chatd to be reported as unconfigured, got %+v", chatd)
	}
}

func healthProvider(t *testing.T, service Service, providerName string) map[string]any {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	responseRecorder := httptest.NewRecorder()

	service.router().ServeHTTP(responseRecorder, request)

	var response map[string]any
	if errorValue := json.NewDecoder(responseRecorder.Body).Decode(&response); errorValue != nil {
		t.Fatalf("expected health response to decode: %v", errorValue)
	}
	providers, hasProviders := response["providers"].(map[string]any)
	if !hasProviders {
		t.Fatalf("expected providers health section, got %+v", response)
	}
	provider, hasProvider := providers[providerName].(map[string]any)
	if !hasProvider {
		t.Fatalf("expected %s health section, got %+v", providerName, providers)
	}
	return provider
}

func TestToolInvokeDoesNotExposeSecretsForUnconfiguredTool(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "sk-must-not-leak")
	service := Service{Configuration: DefaultConfiguration()}
	request := httptest.NewRequest(http.MethodPost, "/v1/tools/nothing.registered/invoke", strings.NewReader(`{"input":{"query":"hello"}}`))
	responseRecorder := httptest.NewRecorder()

	service.router().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusBadGateway {
		t.Fatalf("expected unconfigured tool to fail safely, got %d", responseRecorder.Code)
	}
	responseBody := responseRecorder.Body.String()
	if strings.Contains(responseBody, "sk-must-not-leak") {
		t.Fatalf("expected tool error to omit secrets, got %q", responseBody)
	}
	if !strings.Contains(responseBody, "capability tool is not configured") {
		t.Fatalf("expected safe tool error, got %q", responseBody)
	}
}
