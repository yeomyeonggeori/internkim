package capabilityd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWebSearchUsesOpenRouterAutoServerTool(t *testing.T) {
	secretPath := writeOpenRouterSecretForWebToolTest(t, "sk-web")
	var requestDocument map[string]any
	service := Service{
		Configuration: Configuration{
			OpenRouterKeyPath:    secretPath,
			OpenRouterWebBaseURL: "https://openrouter.test/chat",
			OpenRouterModel:      "openrouter/search-model",
		}.WithDefaults(),
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Header.Get("Authorization") != "Bearer sk-web" {
				t.Fatalf("unexpected authorization header: %q", request.Header.Get("Authorization"))
			}
			if errorValue := json.NewDecoder(request.Body).Decode(&requestDocument); errorValue != nil {
				t.Fatalf("expected request body: %v", errorValue)
			}
			return jsonResponseBody(`{"choices":[{"message":{"content":"{\"provider\":\"openrouter\",\"remoteLLMInvolved\":true,\"compatibility\":\"openrouter_server_tool_auto\",\"query\":\"internkim\",\"answer\":\"result\",\"results\":[]}"}}]}`), nil
		})},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "web.search", strings.NewReader(`{"input":{"query":"internkim","limit":3}}`))
	if errorValue != nil {
		t.Fatalf("expected web search: %v", errorValue)
	}

	tool := requestDocument["tools"].([]any)[0].(map[string]any)
	parameters := tool["parameters"].(map[string]any)
	if tool["type"] != "openrouter:web_search" || parameters["engine"] != "auto" || parameters["max_results"] != float64(3) {
		t.Fatalf("unexpected web search tool: %+v", tool)
	}
	if response.Provider != "openrouter" || response.SelectedBackend != "remote" || response.IsError {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestWebFetchRejectsPrivateURLBeforeProviderCall(t *testing.T) {
	secretPath := writeOpenRouterSecretForWebToolTest(t, "sk-web")
	wasCalled := false
	service := Service{
		Configuration: Configuration{
			OpenRouterKeyPath:    secretPath,
			OpenRouterWebBaseURL: "https://openrouter.test/chat",
			OpenRouterModel:      "openrouter/search-model",
		}.WithDefaults(),
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			wasCalled = true
			return jsonResponseBody(`{}`), nil
		})},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "web.fetch", strings.NewReader(`{"input":{"urls":["http://127.0.0.1:8080"]}}`))
	if errorValue != nil {
		t.Fatalf("expected structured tool error response: %v", errorValue)
	}
	if !response.IsError || wasCalled {
		t.Fatalf("expected private URL rejection before provider call, response=%+v called=%v", response, wasCalled)
	}
}

func TestWebToolLocalOnlyBlocksOpenRouter(t *testing.T) {
	secretPath := writeOpenRouterSecretForWebToolTest(t, "sk-web")
	service := Service{Configuration: Configuration{OpenRouterKeyPath: secretPath, LocalOnly: true}.WithDefaults()}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "web.search", strings.NewReader(`{"input":{"query":"internkim"}}`))
	if errorValue != nil {
		t.Fatalf("expected structured tool error response: %v", errorValue)
	}
	if !response.IsError || response.ErrorCode != "local_only" {
		t.Fatalf("expected local-only denial, got %+v", response)
	}
}

func TestWebToolMissingOpenRouterKeyDoesNotLeakSecrets(t *testing.T) {
	service := Service{Configuration: Configuration{OpenRouterKeyPath: filepath.Join(t.TempDir(), "missing")}.WithDefaults()}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "web.search", strings.NewReader(`{"input":{"query":"internkim"}}`))
	if errorValue != nil {
		t.Fatalf("expected structured tool error response: %v", errorValue)
	}
	if !response.IsError || strings.Contains(string(response.Result), "sk-") {
		t.Fatalf("expected safe missing key response, got %+v", response)
	}
}

func writeOpenRouterSecretForWebToolTest(t *testing.T, value string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(path, []byte(value), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return path
}

func jsonResponseBody(document string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(document)),
		Header:     make(http.Header),
	}
}
