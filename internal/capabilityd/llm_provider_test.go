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
	}, "openrouter/model")
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
	if document["model"] != "openrouter/model" {
		t.Fatalf("expected explicit remote model, got %q", document["model"])
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

func TestOpenRouterProviderUsesDefaultModelForLocalAlias(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	var receivedDocument map[string]any
	provider := OpenRouterProvider{
		Configuration: Configuration{
			OpenRouterKeyPath: "missing",
			OpenRouterBaseURL: "https://example.test/chat",
			OpenRouterModel:   "google/default-remote",
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if errorValue := json.NewDecoder(request.Body).Decode(&receivedDocument); errorValue != nil {
				t.Fatalf("expected request body: %v", errorValue)
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
		Model: "local/gemma-4-E4B-it-litert-lm",
		StructuredOutputSchema: StructuredOutputSchema{
			Name:     "reply",
			Document: json.RawMessage(`{"type":"object","required":["reply"]}`),
		},
	})
	if errorValue != nil {
		t.Fatalf("expected remote completion: %v", errorValue)
	}
	if receivedDocument["model"] != "google/default-remote" {
		t.Fatalf("expected provider default remote model, got %q", receivedDocument["model"])
	}
	if response.Model != "google/default-remote" {
		t.Fatalf("expected response model to match selected remote model, got %q", response.Model)
	}
}

func TestOpenRouterProviderUsesDefaultModelForProviderNeutralSentinel(t *testing.T) {
	provider := OpenRouterProvider{
		Configuration: Configuration{OpenRouterModel: "google/default-remote"},
	}

	for _, modelName := range []string{"", "default", "DEFAULT"} {
		if selectedModelName := provider.remoteModelName(modelName); selectedModelName != "google/default-remote" {
			t.Fatalf("expected default remote model for %q, got %q", modelName, selectedModelName)
		}
	}
}

func TestOpenRouterProviderReportsResponseReadError(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	provider := OpenRouterProvider{
		Configuration: Configuration{
			OpenRouterKeyPath: secretPath,
			OpenRouterBaseURL: "https://example.test/chat",
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       failingReadCloser{errorValue: context.DeadlineExceeded},
				Header:     make(http.Header),
			}, nil
		})},
	}

	_, errorValue := provider.CompleteStructured(context.Background(), StructuredLLMRequest{})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "read openrouter response") {
		t.Fatalf("expected read error, got %v", errorValue)
	}
}

func TestOpenRouterProviderRejectsEmptySuccessBody(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	provider := OpenRouterProvider{
		Configuration: Configuration{
			OpenRouterKeyPath: secretPath,
			OpenRouterBaseURL: "https://example.test/chat",
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		})},
	}

	_, errorValue := provider.CompleteStructured(context.Background(), StructuredLLMRequest{})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "response body was empty") {
		t.Fatalf("expected empty body error, got %v", errorValue)
	}
}

func TestDefaultProviderAttemptTimeoutAllowsRemoteStructuredResponses(t *testing.T) {
	if DefaultConfiguration().ProviderAttemptTimeout < 90*time.Second {
		t.Fatalf("expected remote provider attempt timeout to allow structured responses, got %s", DefaultConfiguration().ProviderAttemptTimeout)
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

type failingReadCloser struct {
	errorValue error
}

func (reader failingReadCloser) Read([]byte) (int, error) {
	return 0, reader.errorValue
}

func (reader failingReadCloser) Close() error {
	return nil
}

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
