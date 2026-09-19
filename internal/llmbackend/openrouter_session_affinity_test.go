package llmbackend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type capturedOpenRouterCall struct {
	SessionHeader string
	Body          map[string]any
}

func openRouterBackendCapturing(t *testing.T, capture *capturedOpenRouterCall) OpenRouterBackend {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		capture.SessionHeader = request.Header.Get("X-Session-Id")
		if errorValue := json.NewDecoder(request.Body).Decode(&capture.Body); errorValue != nil {
			t.Errorf("the request body could not be read: %v", errorValue)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.Write([]byte(`{"provider":"a-provider","choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"{\"answer\":\"ok\"}","tool_calls":[{"id":"call-1","type":"function","function":{"name":"continue__file_write","arguments":"{\"path\":\"a.txt\",\"content\":\"hello\"}"}}]}}]}`))
	}))
	t.Cleanup(server.Close)
	keyPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(keyPath, []byte("sk-a-test-key"), 0o600); errorValue != nil {
		t.Fatalf("expected a key fixture: %v", errorValue)
	}
	return OpenRouterBackend{KeyPath: keyPath, BaseURL: server.URL, ModelName: "a-model", HTTPClient: server.Client()}
}

func everyOpenRouterTransport(t *testing.T, sessionID string) map[string]func(OpenRouterBackend) error {
	t.Helper()
	messages := []Message{{Role: "system", Content: "you help"}, {Role: "user", Content: "안녕"}}
	plainSchema := StructuredOutputSchema{Name: "answer", Document: json.RawMessage(`{"type":"object","properties":{"answer":{"type":"string"}}}`)}
	actionSchema := StructuredOutputSchema{
		Name: "bluecollar_agent_turn_action",
		Document: testActionSchemaForDescriptors(t, []capabilities.Descriptor{{
			Name:        "file_write",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}`),
		}}),
	}
	return map[string]func(OpenRouterBackend) error{
		"native action": func(backend OpenRouterBackend) error {
			_, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{SessionID: sessionID, Messages: messages, StructuredOutputSchema: actionSchema})
			return errorValue
		},
		"streamed text": func(backend OpenRouterBackend) error {
			_, errorValue := backend.CompleteText(context.Background(), TextRequest{SessionID: sessionID, Messages: messages})
			return errorValue
		},
		"healed text": func(backend OpenRouterBackend) error {
			_, errorValue := backend.CompleteText(context.Background(), TextRequest{SessionID: sessionID, Messages: messages, EnableResponseHealing: true})
			return errorValue
		},
		"structured": func(backend OpenRouterBackend) error {
			_, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{SessionID: sessionID, Messages: messages, StructuredOutputSchema: plainSchema})
			return errorValue
		},
		"chat": func(backend OpenRouterBackend) error {
			_, errorValue := backend.CompleteChat(context.Background(), ChatRequest{SessionID: sessionID, Messages: []ChatMessage{{Role: "user", Content: "안녕"}}})
			return errorValue
		},
	}
}

// A cache lives on one upstream provider's servers, so turn N+1 of a run only
// hits it when it asks the same way turn N did. The run id is what says the two
// belong together, and every shape of request has to carry it or the turns that
// go out the other shapes land somewhere else.
func TestEveryOpenRouterTransportCarriesTheRunsSessionID(t *testing.T) {
	const taskRunID = "task-run-7f3a91"
	for name, ask := range everyOpenRouterTransport(t, taskRunID) {
		capture := &capturedOpenRouterCall{}
		if errorValue := ask(openRouterBackendCapturing(t, capture)); errorValue != nil {
			t.Fatalf("%s: %v", name, errorValue)
		}
		if capture.SessionHeader != taskRunID {
			t.Errorf("%s asked without the run's session header: %q", name, capture.SessionHeader)
		}
		if capture.Body["prompt_cache_key"] != taskRunID {
			t.Errorf("%s asked without the run's prompt cache key: %v", name, capture.Body["prompt_cache_key"])
		}
	}
}

func TestACallWithoutARunSendsNoAffinity(t *testing.T) {
	for name, ask := range everyOpenRouterTransport(t, "") {
		capture := &capturedOpenRouterCall{}
		if errorValue := ask(openRouterBackendCapturing(t, capture)); errorValue != nil {
			t.Fatalf("%s: %v", name, errorValue)
		}
		if capture.SessionHeader != "" {
			t.Errorf("%s invented a session header: %q", name, capture.SessionHeader)
		}
		if _, isPresent := capture.Body["prompt_cache_key"]; isPresent {
			t.Errorf("%s invented a prompt cache key: %v", name, capture.Body["prompt_cache_key"])
		}
	}
}
