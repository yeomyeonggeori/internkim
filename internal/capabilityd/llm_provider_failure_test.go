package capabilityd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAChatItsProviderEndedWithAnErrorAnswersAsAFailedCall(t *testing.T) {
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(responseWriter, `data: {"provider":"Wafer","choices":[{"delta":{"role":"assistant","content":""},"finish_reason":"error"}]}`+"\n\ndata: [DONE]\n\n")
	}))
	t.Cleanup(upstreamServer.Close)
	keyPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(keyPath, []byte("test-key"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	configuration := DefaultConfiguration()
	configuration.OpenRouterKeyPath = keyPath
	configuration.OpenRouterBaseURL = upstreamServer.URL + "/api/v1/chat/completions"
	configuration.OpenRouterModel = "a-model-its-provider-fails"
	configuration.ForceOpenRouterModel = true
	service := Service{Configuration: configuration, HTTPClient: upstreamServer.Client()}

	responseRecorder := askLLM(t, service, "/v1/llm/chat", `{"executionMode":"remote","messages":[{"role":"user","content":"안녕"}]}`)

	if responseRecorder.Code != http.StatusBadGateway {
		t.Fatalf("a caller falls back to its next model only on a failed call, got %d: %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if !strings.Contains(responseRecorder.Body.String(), "Wafer") {
		t.Fatalf("the failure names the provider that failed: %s", responseRecorder.Body.String())
	}
}
