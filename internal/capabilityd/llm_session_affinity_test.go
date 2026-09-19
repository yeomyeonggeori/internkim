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

type capturedUpstreamCall struct {
	SessionHeader string
	Body          string
}

func upstreamCapturingService(t *testing.T, capture *capturedUpstreamCall) Service {
	t.Helper()
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		document, errorValue := io.ReadAll(request.Body)
		if errorValue != nil {
			t.Error(errorValue)
			return
		}
		capture.SessionHeader = request.Header.Get("X-Session-Id")
		capture.Body = string(document)
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(responseWriter, `{"provider":"a-provider","choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"{\"answer\":\"ok\"}"}}]}`)
	}))
	t.Cleanup(upstreamServer.Close)

	keyPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(keyPath, []byte("test-key"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	configuration := DefaultConfiguration()
	configuration.OpenRouterKeyPath = keyPath
	configuration.OpenRouterBaseURL = upstreamServer.URL + "/api/v1/chat/completions"
	configuration.OpenRouterModel = "a-model"
	configuration.ForceOpenRouterModel = true
	return Service{Configuration: configuration, HTTPClient: upstreamServer.Client()}
}

func askLLM(t *testing.T, service Service, path string, requestBody string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(requestBody))
	responseRecorder := httptest.NewRecorder()
	service.router().ServeHTTP(responseRecorder, request)
	return responseRecorder
}

// The run identity is the caller's to supply: capabilityd cannot tell which
// turns belong together, so a caller that leaves sessionID out gets today's
// behavior and one that fills it gets its run pinned to one upstream cache.
func TestSessionIDReachesTheUpstreamAffinityKey(t *testing.T) {
	const taskRunID = "task-run-19c4"
	requestBodies := map[string]string{
		"/v1/llm/text":       `{"executionMode":"remote","sessionID":"` + taskRunID + `","messages":[{"role":"user","content":"안녕"}]}`,
		"/v1/llm/structured": `{"executionMode":"remote","sessionID":"` + taskRunID + `","messages":[{"role":"user","content":"안녕"}],"structuredOutputSchema":{"name":"answer","document":{"type":"object"},"isStrictlyEnforced":false}}`,
		"/v1/llm/chat":       `{"executionMode":"remote","sessionID":"` + taskRunID + `","messages":[{"role":"user","content":"안녕"}]}`,
	}
	for path, requestBody := range requestBodies {
		capture := &capturedUpstreamCall{}
		responseRecorder := askLLM(t, upstreamCapturingService(t, capture), path, requestBody)
		if responseRecorder.Code != http.StatusOK {
			t.Fatalf("%s answered %d: %s", path, responseRecorder.Code, responseRecorder.Body.String())
		}
		if capture.SessionHeader != taskRunID {
			t.Errorf("%s lost the run's session header: %q", path, capture.SessionHeader)
		}
		if !strings.Contains(capture.Body, `"prompt_cache_key":"`+taskRunID+`"`) {
			t.Errorf("%s lost the run's prompt cache key: %s", path, capture.Body)
		}
	}
}
