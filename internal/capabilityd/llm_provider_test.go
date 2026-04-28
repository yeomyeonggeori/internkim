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
	"time"
)

func TestOpenRouterRequestPreservesStructuredSchema(t *testing.T) {
	requestDocument, errorValue := buildOpenRouterRequest(StructuredLLMRequest{
		Model: "openrouter/model",
		Messages: []LLMMessage{
			{Role: "user", Content: "hello"},
		},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:               "reply",
			Document:           json.RawMessage(`{"type":"object","properties":{"reply":{"type":"string"}},"required":["reply"],"additionalProperties":false}`),
			IsStrictlyEnforced: true,
		},
		RequireParameters:     true,
		EnableResponseHealing: true,
	})
	if errorValue != nil {
		t.Fatalf("expected OpenRouter request: %v", errorValue)
	}

	var document map[string]any
	if errorValue := json.Unmarshal(requestDocument, &document); errorValue != nil {
		t.Fatalf("expected request to decode: %v", errorValue)
	}
	responseFormat := document["response_format"].(map[string]any)
	jsonSchema := responseFormat["json_schema"].(map[string]any)
	schema := jsonSchema["schema"].(map[string]any)
	required := schema["required"].([]any)
	if required[0] != "reply" {
		t.Fatalf("expected schema to be preserved, got %+v", schema)
	}
	if jsonSchema["strict"] != true {
		t.Fatalf("expected strict schema, got %+v", jsonSchema)
	}
}

func TestOpenRouterProviderReturnsProviderConstraintMode(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	provider := OpenRouterProvider{
		Configuration: Configuration{
			OpenRouterKeyPath: "missing",
			OpenRouterBaseURL: "https://example.test/chat",
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Header.Get("Authorization") != "Bearer sk-test" {
				t.Fatalf("unexpected authorization header: %q", request.Header.Get("Authorization"))
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"{\"reply\":\"ok\"}"}}]}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}
	provider.Configuration.OpenRouterKeyPath = secretPath

	response, errorValue := provider.CompleteStructured(context.Background(), StructuredLLMRequest{
		Model: "openrouter/model",
		StructuredOutputSchema: StructuredOutputSchema{
			Name:     "reply",
			Document: json.RawMessage(`{"type":"object","required":["reply"]}`),
		},
	})
	if errorValue != nil {
		t.Fatalf("expected remote completion: %v", errorValue)
	}
	if response.ConstraintMode != "provider_json_schema" {
		t.Fatalf("expected provider json schema mode, got %q", response.ConstraintMode)
	}
}

func TestLiteRTProviderSendsJSONSchemaDocumentToWrapper(t *testing.T) {
	var wrapperDocument map[string]any
	provider := LiteRTProvider{
		Configuration: DefaultConfiguration(),
		RunCommand: func(ctx context.Context, executablePath string, arguments []string, standardInput []byte) ([]byte, error) {
			_ = ctx
			_ = executablePath
			_ = arguments
			if errorValue := json.Unmarshal(standardInput, &wrapperDocument); errorValue != nil {
				t.Fatalf("expected wrapper document: %v", errorValue)
			}
			return []byte(`{"content":"{\"reply\":\"ok\"}"}`), nil
		},
	}

	_, errorValue := provider.CompleteStructured(context.Background(), StructuredLLMRequest{
		StructuredOutputSchema: StructuredOutputSchema{
			Name:     "reply",
			Document: json.RawMessage(`{"type":"object","properties":{"reply":{"type":"string"}},"required":["reply"]}`),
		},
	})
	if errorValue != nil {
		t.Fatalf("expected LiteRT completion: %v", errorValue)
	}

	schema := wrapperDocument["structuredOutputSchema"].(map[string]any)
	document := schema["document"].(map[string]any)
	if document["type"] != "object" {
		t.Fatalf("expected schema document object, got %+v", document)
	}
}

func TestAutoProviderFallsBackToRemote(t *testing.T) {
	autoProvider := AutoProvider{
		Providers: []LLMProvider{
			staticLLMProvider{errorValue: errTestProviderUnavailable},
			staticLLMProvider{response: LLMResponse{
				Provider:        "openrouter",
				Content:         `{"reply":"ok"}`,
				SelectedBackend: "remote",
			}},
		},
	}

	response, errorValue := autoProvider.CompleteStructured(context.Background(), StructuredLLMRequest{})
	if errorValue != nil {
		t.Fatalf("expected auto fallback: %v", errorValue)
	}
	if response.SelectedBackend != "remote" {
		t.Fatalf("expected remote backend, got %q", response.SelectedBackend)
	}
}

func TestAutoProviderAttemptTimeoutFallsBackToRemote(t *testing.T) {
	autoProvider := AutoProvider{
		AttemptTimeout: time.Millisecond,
		Providers: []LLMProvider{
			blockingLLMProvider{},
			staticLLMProvider{response: LLMResponse{
				Provider:        "openrouter",
				Content:         `{"reply":"ok"}`,
				SelectedBackend: "remote",
			}},
		},
	}

	response, errorValue := autoProvider.CompleteStructured(context.Background(), StructuredLLMRequest{})
	if errorValue != nil {
		t.Fatalf("expected timeout fallback: %v", errorValue)
	}
	if response.SelectedBackend != "remote" {
		t.Fatalf("expected remote backend, got %q", response.SelectedBackend)
	}
}

var errTestProviderUnavailable = os.ErrNotExist

type staticLLMProvider struct {
	response   LLMResponse
	errorValue error
}

type blockingLLMProvider struct{}

func (provider blockingLLMProvider) CompleteStructured(ctx context.Context, request StructuredLLMRequest) (LLMResponse, error) {
	_ = request
	<-ctx.Done()
	return LLMResponse{}, ctx.Err()
}

func (provider blockingLLMProvider) CompleteText(ctx context.Context, request TextLLMRequest) (LLMResponse, error) {
	_ = request
	<-ctx.Done()
	return LLMResponse{}, ctx.Err()
}

func (provider staticLLMProvider) CompleteStructured(context.Context, StructuredLLMRequest) (LLMResponse, error) {
	return provider.response, provider.errorValue
}

func (provider staticLLMProvider) CompleteText(context.Context, TextLLMRequest) (LLMResponse, error) {
	return provider.response, provider.errorValue
}
