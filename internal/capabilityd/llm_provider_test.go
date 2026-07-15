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

	"gitlab.com/eastriver/internkim/internal/llmbackend"
)

func newOpenRouterBackend(secretPath, baseURL, modelName string, transport http.RoundTripper) OpenRouterBackend {
	return OpenRouterBackend{
		KeyPath:    secretPath,
		BaseURL:    baseURL,
		ModelName:  modelName,
		HTTPClient: &http.Client{Transport: transport},
	}
}

func setLiteRTConstrainedRunnerPath(t *testing.T, path string) {
	t.Helper()
	previousPath := llmbackend.LiteRTConstrainedRunnerBinaryPath
	llmbackend.LiteRTConstrainedRunnerBinaryPath = path
	t.Cleanup(func() {
		llmbackend.LiteRTConstrainedRunnerBinaryPath = previousPath
	})
}

func createLiteRTConstrainedRunner(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "internkim-litert-constrained")
	if errorValue := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	return path
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

func TestLiteRTProviderSendsJSONSchemaDocumentToWrapper(t *testing.T) {
	setLiteRTConstrainedRunnerPath(t, createLiteRTConstrainedRunner(t))
	var wrapperDocument map[string]any
	backend := LiteRTProvider{
		ModelPath:  DefaultConfiguration().LiteRTModelPath,
		RunnerPath: DefaultConfiguration().LocalLLMRunnerPath,
		Variant:    "gpu",
		RunCommand: func(ctx context.Context, executablePath string, arguments []string, standardInput []byte) ([]byte, error) {
			_ = ctx
			_ = executablePath
			_ = arguments
			if errorValue := json.Unmarshal(standardInput, &wrapperDocument); errorValue != nil {
				t.Fatalf("expected wrapper document: %v", errorValue)
			}
			return []byte(`{"content":"{\"reply\":\"ok\"}","constraintMode":"litert_llguidance_json_schema"}`), nil
		},
	}

	_, errorValue := backend.CompleteStructured(context.Background(), StructuredLLMRequest{
		StructuredOutputSchema: StructuredOutputSchema{
			Name:     "reply",
			Document: json.RawMessage(`{"type":"object","properties":{"reply":{"type":"string"}},"required":["reply"]}`),
		},
	})
	if errorValue != nil {
		t.Fatalf("expected LiteRT completion: %v", errorValue)
	}

	constraint := wrapperDocument["constrainedDecoding"].(map[string]any)
	if constraint["type"] != "json_schema" {
		t.Fatalf("expected JSON schema constraint, got %+v", constraint)
	}
	schema := constraint["jsonSchema"].(map[string]any)
	document := schema["document"].(map[string]any)
	if document["type"] != "object" {
		t.Fatalf("expected schema document object, got %+v", document)
	}
}

func TestLiteRTProviderCompleteTextSendsTextModeToWrapper(t *testing.T) {
	var wrapperDocument map[string]any
	backend := LiteRTProvider{
		ModelPath:  DefaultConfiguration().LiteRTModelPath,
		RunnerPath: DefaultConfiguration().LocalLLMRunnerPath,
		Variant:    "gpu",
		RunCommand: func(ctx context.Context, executablePath string, arguments []string, standardInput []byte) ([]byte, error) {
			_ = ctx
			_ = executablePath
			_ = arguments
			if errorValue := json.Unmarshal(standardInput, &wrapperDocument); errorValue != nil {
				t.Fatalf("expected wrapper document: %v", errorValue)
			}
			return []byte(`{"content":"plain local reply"}`), nil
		},
	}

	response, errorValue := backend.CompleteText(context.Background(), TextLLMRequest{
		Messages: []LLMMessage{{Role: "user", Content: "hello"}},
	})
	if errorValue != nil {
		t.Fatalf("expected LiteRT text completion: %v", errorValue)
	}
	if wrapperDocument["mode"] != "text" {
		t.Fatalf("expected text mode, got %+v", wrapperDocument)
	}
	if _, isFound := wrapperDocument["constrainedDecoding"]; isFound {
		t.Fatalf("expected text request not to include a schema document, got %+v", wrapperDocument)
	}
	if response.Content != "plain local reply" {
		t.Fatalf("expected plain local reply, got %q", response.Content)
	}
}

func TestLocalBackendsHonorsRequestedLiteRTVariant(t *testing.T) {
	service := Service{
		Configuration: DefaultConfiguration(),
	}
	providerSet := service.localProviderSet("litert", "cpu", false)
	if len(providerSet.Backends) != 1 {
		t.Fatalf("expected single cpu backend, got %d", len(providerSet.Backends))
	}
	litertBackend, isLiteRT := providerSet.Backends[0].(LiteRTProvider)
	if !isLiteRT {
		t.Fatalf("expected LiteRT backend, got %T", providerSet.Backends[0])
	}
	if litertBackend.Variant != "cpu" {
		t.Fatalf("expected cpu variant, got %q", litertBackend.Variant)
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

func TestLocalProviderUsesExplicitOllamaProvider(t *testing.T) {
	service := Service{
		Configuration: Configuration{
			OllamaBaseURL:              "https://ollama.test",
			OllamaModel:                "gemma3:1b",
			LocalBackendOrder:          []string{"ollama"},
			ProviderAttemptTimeout:     time.Second,
			LocalLLMRunnerPath:         "/missing-local-llm-runner",
			LiteRTModelPath:            "/missing-litert-model",
			OpenRouterKeyPath:          "missing",
			OpenRouterBaseURL:          "https://openrouter.test",
			CompanionBaseURL:           "",
			DeviceBrowserPath:          "chromium",
			AgentBrowserPath:           "agent-browser",
			CompanionFileDirectory:     t.TempDir(),
			SocketPath:                 filepath.Join(t.TempDir(), "capability.sock"),
			MattermostTokenPath:        "missing",
			SlackTokenPath:             "missing",
			SlackAppTokenPath:          "missing",
			SignalJSONRPCURLPath:       "missing",
			SignalAccountPath:          "missing",
			SocketGroupName:            "blueclaw",
			OpenRouterEmbeddingBaseURL: "https://embedding.test",
			OpenRouterEmbeddingModel:   "embedding",
		},
		RunCommand: func(ctx context.Context, executablePath string, arguments []string, standardInput []byte) ([]byte, error) {
			_ = ctx
			_ = executablePath
			_ = arguments
			_ = standardInput
			return nil, os.ErrNotExist
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.String() != "https://ollama.test/api/chat" {
				t.Fatalf("unexpected local provider URL: %s", request.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"message":{"role":"assistant","content":"ok from ollama"}}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	response, errorValue := service.completeText(context.Background(), TextLLMRequest{
		ExecutionMode: "device",
		Provider:      "ollama",
		Messages:      []LLMMessage{{Role: "user", Content: "Reply with ok."}},
	})
	if errorValue != nil {
		t.Fatalf("expected explicit Ollama provider: %v", errorValue)
	}
	if response.Provider != "ollama" || response.SelectedBackend != "ollama" || response.Content != "ok from ollama" {
		t.Fatalf("expected Ollama response, got %+v", response)
	}
}

func TestLocalProviderDoesNotUseOllamaByDefault(t *testing.T) {
	ollamaCalled := false
	service := Service{
		Configuration: Configuration{
			OllamaBaseURL:          "https://ollama.test",
			ProviderAttemptTimeout: time.Second,
			LocalLLMRunnerPath:     "/missing-local-llm-runner",
			LiteRTModelPath:        "/missing-litert-model",
		},
		RunCommand: func(ctx context.Context, executablePath string, arguments []string, standardInput []byte) ([]byte, error) {
			_ = ctx
			_ = executablePath
			_ = arguments
			_ = standardInput
			return nil, os.ErrNotExist
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.Host == "ollama.test" {
				ollamaCalled = true
			}
			return nil, os.ErrNotExist
		})},
	}

	_, errorValue := service.completeText(context.Background(), TextLLMRequest{
		ExecutionMode: "device",
		Messages:      []LLMMessage{{Role: "user", Content: "Reply with ok."}},
	})
	if errorValue == nil {
		t.Fatal("expected LiteRT failure")
	}
	if ollamaCalled {
		t.Fatal("expected local mode not to call Ollama without explicit opt-in")
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

	provider, errorValue := service.providerForExecutionMode(context.Background(), "llm.structured", "auto", "", "")
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
