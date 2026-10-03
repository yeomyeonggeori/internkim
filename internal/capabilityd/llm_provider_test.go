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

	"github.com/yeomyeonggeori/internkim/internal/llmbackend"
)

func newOpenRouterBackend(secretPath, baseURL, modelName string, transport http.RoundTripper) OpenRouterBackend {
	return OpenRouterBackend{
		KeyPath:    secretPath,
		BaseURL:    baseURL,
		ModelName:  modelName,
		HTTPClient: &http.Client{Transport: transport},
	}
}

func TestOpenRouterBackendReturnsProviderConstraintMode(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	backend := newOpenRouterBackend(secretPath, "https://example.test/chat", "", roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer sk-test" {
			t.Fatalf("unexpected authorization header: %q", request.Header.Get("Authorization"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"{\"reply\":\"ok\"}"}}]}`)),
			Header:     make(http.Header),
		}, nil
	}))

	response, errorValue := backend.CompleteStructured(context.Background(), StructuredLLMRequest{
		Model: "openrouter/model",
		StructuredOutputSchema: StructuredOutputSchema{
			Name:     "reply",
			Document: json.RawMessage(`{"type":"object","required":["reply"]}`),
		},
	})
	if errorValue != nil {
		t.Fatalf("expected remote completion: %v", errorValue)
	}
	if response.ConstraintMode != llmbackend.ConstraintModeOpenAIJSONSchema {
		t.Fatalf("expected OpenAI JSON schema mode, got %q", response.ConstraintMode)
	}
}

func TestOpenRouterBackendUsesDefaultModelForLocalAlias(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	var receivedDocument map[string]any
	backend := newOpenRouterBackend(secretPath, "https://example.test/chat", "google/default-remote", roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if errorValue := json.NewDecoder(request.Body).Decode(&receivedDocument); errorValue != nil {
			t.Fatalf("expected request body: %v", errorValue)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"{\"reply\":\"ok\"}"}}]}`)),
			Header:     make(http.Header),
		}, nil
	}))

	response, errorValue := backend.CompleteStructured(context.Background(), StructuredLLMRequest{
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

func TestOpenRouterBackendReportsResponseReadError(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	backend := newOpenRouterBackend(secretPath, "https://example.test/chat", "", roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       failingReadCloser{errorValue: context.DeadlineExceeded},
			Header:     make(http.Header),
		}, nil
	}))

	_, errorValue := backend.CompleteStructured(context.Background(), StructuredLLMRequest{})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "read openrouter response") {
		t.Fatalf("expected read error, got %v", errorValue)
	}
}

func TestOpenRouterBackendRejectsEmptySuccessBody(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	backend := newOpenRouterBackend(secretPath, "https://example.test/chat", "", roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	}))

	_, errorValue := backend.CompleteStructured(context.Background(), StructuredLLMRequest{})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "response body was empty") {
		t.Fatalf("expected empty body error, got %v", errorValue)
	}
}

func TestOpenRouterBackendCompleteTextDoesNotRequestStructuredOutput(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	var receivedDocument map[string]any
	backend := newOpenRouterBackend(secretPath, "https://example.test/chat", "google/default-remote", roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if errorValue := json.NewDecoder(request.Body).Decode(&receivedDocument); errorValue != nil {
			t.Fatalf("expected request body: %v", errorValue)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"plain reply"}}]}`)),
			Header:     make(http.Header),
		}, nil
	}))

	response, errorValue := backend.CompleteText(context.Background(), TextLLMRequest{
		Model:             "local/gemma-4-E4B-it-litert-lm",
		RequireParameters: true,
		Messages:          []LLMMessage{{Role: "user", Content: "hello"}},
	})
	if errorValue != nil {
		t.Fatalf("expected text completion: %v", errorValue)
	}
	if _, isFound := receivedDocument["response_format"]; isFound {
		t.Fatalf("expected text request to omit response_format, got %+v", receivedDocument)
	}
	if response.Content != "plain reply" {
		t.Fatalf("expected plain text content, got %q", response.Content)
	}
	if response.ConstraintMode != "" {
		t.Fatalf("expected text response to omit constraint mode, got %q", response.ConstraintMode)
	}
}

func TestDefaultProviderAttemptHasNoArbitraryTimeout(t *testing.T) {
	if DefaultConfiguration().ProviderAttemptTimeout != 0 {
		t.Fatalf("expected no default provider attempt timeout, got %s", DefaultConfiguration().ProviderAttemptTimeout)
	}
}

func TestAutoProviderFallsBackToRemote(t *testing.T) {
	autoProvider := AutoProvider{
		AllowStructuredFallback: true,
		Providers: []llmbackend.Provider{
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
		AttemptTimeout:          time.Millisecond,
		AllowStructuredFallback: true,
		Providers: []llmbackend.Provider{
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

func TestLLMRequestModelPreservesRequestedModelByDefault(t *testing.T) {
	service := Service{Configuration: Configuration{OpenRouterModel: "configured-model"}}

	if modelName := service.llmRequestModel("requested-model"); modelName != "requested-model" {
		t.Fatalf("expected requested model to be preserved, got %q", modelName)
	}
}

func TestLLMRequestModelCanForceConfiguredOpenRouterModel(t *testing.T) {
	service := Service{Configuration: Configuration{OpenRouterModel: "configured-model", ForceOpenRouterModel: true}}

	if modelName := service.llmRequestModel("requested-model"); modelName != "configured-model" {
		t.Fatalf("expected configured model to override requested model, got %q", modelName)
	}
}

func TestForceOpenRouterModelDisablesActionFallbackModels(t *testing.T) {
	service := Service{Configuration: Configuration{ForceOpenRouterModel: true}}

	if fallbackModelNames := service.openRouterBackend().FallbackModelNames; len(fallbackModelNames) != 0 {
		t.Fatalf("expected forced OpenRouter model to disable fallback models, got %+v", fallbackModelNames)
	}
}

func TestForceOpenRouterModelUsesRemoteProviderForAutoMode(t *testing.T) {
	service := Service{Configuration: Configuration{ForceOpenRouterModel: true}}

	provider, errorValue := service.providerForExecutionMode("auto")
	if errorValue != nil {
		t.Fatalf("expected forced OpenRouter auto provider: %v", errorValue)
	}
	if _, isOpenRouter := provider.(OpenRouterBackend); !isOpenRouter {
		t.Fatalf("expected OpenRouter provider, got %T", provider)
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
