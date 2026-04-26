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

	response, errorValue := service.completeStructured(context.Background(), llmRequest{
		ExecutionMode: "local",
		Messages:      []message{{Role: "user", Content: "hello"}},
		StructuredOutputSchema: schemaRequest{
			Name:     "plain_text_response",
			Document: `{"type":"object","properties":{"content":{"type":"string"}},"required":["content"],"additionalProperties":false}`,
		},
	})
	if errorValue != nil {
		t.Fatalf("expected local completion to succeed: %v", errorValue)
	}

	responseMap, isMap := response.(map[string]string)
	if !isMap {
		t.Fatalf("expected map response, got %T", response)
	}
	if responseMap["selectedBackend"] != "cpu" {
		t.Fatalf("expected cpu backend, got %q", responseMap["selectedBackend"])
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

	_, errorValue := service.completeStructured(context.Background(), llmRequest{
		ExecutionMode: "local",
		StructuredOutputSchema: schemaRequest{
			Name:     "plain_text_response",
			Document: `{"type":"object","properties":{"content":{"type":"string"}},"required":["content"],"additionalProperties":false}`,
		},
	})
	if errorValue == nil {
		t.Fatalf("expected invalid structured output to fail")
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
