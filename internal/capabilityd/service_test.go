package capabilityd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalStructuredCompletionUsesCPUAfterGPUFailure(t *testing.T) {
	var backends []string
	service := Service{
		Configuration: DefaultConfiguration(),
		RunCommand: func(ctx context.Context, executablePath string, arguments []string, standardInput []byte) ([]byte, error) {
			_ = ctx
			_ = executablePath
			_ = arguments
			document := string(standardInput)
			if strings.Contains(document, `"backend":"gpu"`) {
				backends = append(backends, "gpu")
				return nil, errors.New("gpu unavailable")
			}
			if strings.Contains(document, `"backend":"cpu"`) {
				backends = append(backends, "cpu")
				return []byte(`{"content":"{\"content\":\"ok\"}"}`), nil
			}
			t.Fatalf("unexpected wrapper request: %s", document)
			return nil, nil
		},
	}

	response, errorValue := service.completeStructured(context.Background(), StructuredLLMRequest{
		ExecutionMode: "local",
		Messages:      []LLMMessage{{Role: "user", Content: "hello"}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:     "plain_text_response",
			Document: json.RawMessage(`{"type":"object","properties":{"content":{"type":"string"}},"required":["content"],"additionalProperties":false}`),
		},
	})
	if errorValue != nil {
		t.Fatalf("expected local completion to succeed: %v", errorValue)
	}

	if response.SelectedBackend != "cpu" {
		t.Fatalf("expected cpu backend, got %q", response.SelectedBackend)
	}
	if response.ConstraintMode != "prompt_validation" {
		t.Fatalf("expected prompt validation constraint mode, got %q", response.ConstraintMode)
	}
	if strings.Join(backends, ",") != "gpu,cpu" {
		t.Fatalf("expected gpu then cpu, got %v", backends)
	}
}

func TestLocalStructuredCompletionRejectsInvalidStructuredOutput(t *testing.T) {
	service := Service{
		Configuration: DefaultConfiguration(),
		RunCommand: func(context.Context, string, []string, []byte) ([]byte, error) {
			return []byte(`{"content":"{\"message\":\"wrong\"}"}`), nil
		},
	}

	_, errorValue := service.completeStructured(context.Background(), StructuredLLMRequest{
		ExecutionMode: "local",
		StructuredOutputSchema: StructuredOutputSchema{
			Name:     "plain_text_response",
			Document: json.RawMessage(`{"type":"object","properties":{"content":{"type":"string"}},"required":["content"],"additionalProperties":false}`),
		},
	})
	if errorValue == nil {
		t.Fatalf("expected invalid structured output to fail")
	}
}

func TestTextCompletionReturnsPlainContent(t *testing.T) {
	service := Service{
		Configuration: DefaultConfiguration(),
		RunCommand: func(context.Context, string, []string, []byte) ([]byte, error) {
			return []byte(`{"content":"{\"content\":\"plain reply\"}"}`), nil
		},
	}

	response, errorValue := service.completeText(context.Background(), TextLLMRequest{
		ExecutionMode: "local",
		Messages:      []LLMMessage{{Role: "user", Content: "hello"}},
	})
	if errorValue != nil {
		t.Fatalf("expected text completion to succeed: %v", errorValue)
	}
	if response.Content != "plain reply" {
		t.Fatalf("expected plain text content, got %q", response.Content)
	}
}

func TestStructuredEndpointRemainsCompatible(t *testing.T) {
	service := Service{
		Configuration: DefaultConfiguration(),
		RunCommand: func(context.Context, string, []string, []byte) ([]byte, error) {
			return []byte(`{"content":"{\"reply\":\"hello\"}"}`), nil
		},
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/llm/structured", strings.NewReader(`{
		"model":"local/gemma",
		"executionMode":"local",
		"messages":[{"role":"user","content":"hello"}],
		"structuredOutputSchema":{
			"name":"reply",
			"document":{"type":"object","properties":{"reply":{"type":"string"}},"required":["reply"],"additionalProperties":false},
			"isStrictlyEnforced":true
		}
	}`))
	responseRecorder := httptest.NewRecorder()

	service.router().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected structured endpoint success, got %d: %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	var response LLMResponse
	if errorValue := json.NewDecoder(responseRecorder.Body).Decode(&response); errorValue != nil {
		t.Fatalf("expected response to decode: %v", errorValue)
	}
	if response.Content != `{"reply":"hello"}` {
		t.Fatalf("expected structured content, got %q", response.Content)
	}
	if response.ConstraintMode != "prompt_validation" {
		t.Fatalf("expected prompt validation mode, got %q", response.ConstraintMode)
	}
}

func TestTextEndpointReturnsPlainContent(t *testing.T) {
	service := Service{
		Configuration: DefaultConfiguration(),
		RunCommand: func(context.Context, string, []string, []byte) ([]byte, error) {
			return []byte(`{"content":"{\"content\":\"plain endpoint reply\"}"}`), nil
		},
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/llm/text", strings.NewReader(`{
		"executionMode":"local",
		"messages":[{"role":"user","content":"hello"}]
	}`))
	responseRecorder := httptest.NewRecorder()

	service.router().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected text endpoint success, got %d: %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	var response LLMResponse
	if errorValue := json.NewDecoder(responseRecorder.Body).Decode(&response); errorValue != nil {
		t.Fatalf("expected response to decode: %v", errorValue)
	}
	if response.Content != "plain endpoint reply" {
		t.Fatalf("expected plain response content, got %q", response.Content)
	}
}

func TestToolInvokeDoesNotExposeSecretsForUnconfiguredTool(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "sk-must-not-leak")
	service := Service{Configuration: DefaultConfiguration()}
	request := httptest.NewRequest(http.MethodPost, "/v1/tools/google.search/invoke", strings.NewReader(`{"input":{"query":"hello"}}`))
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

func TestEmbeddingCreateUsesOpenRouterSecretWithoutReturningIt(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	service := Service{Configuration: Configuration{
		OpenRouterKeyPath:          secretPath,
		OpenRouterEmbeddingBaseURL: "https://example.test/embeddings",
		OpenRouterEmbeddingModel:   "embedding-model",
	}}
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer sk-test" {
			t.Fatalf("unexpected authorization header: %q", request.Header.Get("Authorization"))
		}
		var document map[string]any
		if errorValue := json.NewDecoder(request.Body).Decode(&document); errorValue != nil {
			t.Fatal(errorValue)
		}
		if document["model"] != "embedding-model" {
			t.Fatalf("unexpected model: %v", document["model"])
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"model":"embedding-model","data":[{"embedding":[0.1,0.2]}]}`)),
			Header:     make(http.Header),
		}, nil
	})}
	response, errorValue := service.createEmbedding(context.Background(), embeddingRequest{Input: "hello"})
	if errorValue != nil {
		t.Fatalf("expected embedding creation to succeed: %v", errorValue)
	}

	responseDocument, errorValue := json.Marshal(response)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Contains(string(responseDocument), "sk-test") {
		t.Fatal("expected embedding response to omit the API key")
	}
	if !strings.Contains(string(responseDocument), `"embedding":[0.1,0.2]`) {
		t.Fatalf("unexpected embedding response: %s", responseDocument)
	}
}
