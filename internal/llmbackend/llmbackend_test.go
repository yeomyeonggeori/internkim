package llmbackend

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (transport roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestOpenRouterStructuredRequestPreservesSchema(t *testing.T) {
	requestDocument, errorValue := buildOpenRouterStructuredRequest(StructuredRequest{
		Model: "openrouter/model",
		Messages: []Message{
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
		t.Fatalf("expected request document: %v", errorValue)
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

func TestOpenRouterBackendResolvesDefaultModel(t *testing.T) {
	backend := OpenRouterBackend{ModelName: "google/default-remote"}
	for _, modelName := range []string{"", "default", "DEFAULT", "local/anything", "model.litertlm"} {
		if resolvedModelName := backend.resolveModelName(modelName); resolvedModelName != "google/default-remote" {
			t.Fatalf("expected default remote model for %q, got %q", modelName, resolvedModelName)
		}
	}
}

func TestOllamaBackendStructuredOutputSendsFormatField(t *testing.T) {
	var receivedDocument map[string]any
	backend := OllamaBackend{
		BaseURL:   "https://ollama.test",
		ModelName: "gemma3:1b",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if errorValue := json.NewDecoder(request.Body).Decode(&receivedDocument); errorValue != nil {
				t.Fatalf("expected request body: %v", errorValue)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"message":{"role":"assistant","content":"{\"reply\":\"ok\"}"}}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	response, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:     "reply",
			Document: json.RawMessage(`{"type":"object","required":["reply"]}`),
		},
	})
	if errorValue != nil {
		t.Fatalf("expected ollama completion: %v", errorValue)
	}
	if _, isFound := receivedDocument["format"]; !isFound {
		t.Fatalf("expected format field in request, got %+v", receivedDocument)
	}
	if response.ConstraintMode != "provider_json_schema" {
		t.Fatalf("expected provider json schema mode, got %q", response.ConstraintMode)
	}
}

func TestLlamaCppBackendStructuredOutputUsesResponseFormat(t *testing.T) {
	var receivedDocument map[string]any
	backend := LlamaCppBackend{
		BaseURL:   "https://llamacpp.test",
		ModelName: "default",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.Path != "/v1/chat/completions" {
				t.Fatalf("unexpected path: %s", request.URL.Path)
			}
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

	_, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:     "reply",
			Document: json.RawMessage(`{"type":"object","required":["reply"]}`),
		},
	})
	if errorValue != nil {
		t.Fatalf("expected llamacpp completion: %v", errorValue)
	}
	responseFormat, isFound := receivedDocument["response_format"].(map[string]any)
	if !isFound {
		t.Fatalf("expected response_format field, got %+v", receivedDocument)
	}
	if responseFormat["type"] != "json_schema" {
		t.Fatalf("expected json_schema response_format, got %+v", responseFormat)
	}
}

func TestMLXBackendStructuredOutputUsesResponseFormat(t *testing.T) {
	backend := MLXBackend{
		BaseURL:   "https://mlx.test",
		ModelName: "default",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.Path != "/v1/chat/completions" {
				t.Fatalf("unexpected path: %s", request.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"{\"reply\":\"ok\"}"}}]}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	response, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:     "reply",
			Document: json.RawMessage(`{"type":"object","required":["reply"]}`),
		},
	})
	if errorValue != nil {
		t.Fatalf("expected mlx completion: %v", errorValue)
	}
	if response.Provider != "mlx" {
		t.Fatalf("expected mlx provider, got %q", response.Provider)
	}
}

func TestStructuredOutputValidationRejectsMissingRequired(t *testing.T) {
	backend := OllamaBackend{
		BaseURL:   "https://ollama.test",
		ModelName: "gemma3:1b",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			_ = request
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"message":{"role":"assistant","content":"{\"other\":\"value\"}"}}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	_, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
		StructuredOutputSchema: StructuredOutputSchema{
			Name:     "reply",
			Document: json.RawMessage(`{"type":"object","required":["reply"]}`),
		},
	})
	if errorValue == nil {
		t.Fatal("expected validation failure for missing required field")
	}
}

func TestAutoProviderReportsAggregateErrorWhenAllFail(t *testing.T) {
	auto := AutoProvider{
		Providers: []Provider{
			staticProvider{errorValue: errors.New("one")},
			staticProvider{errorValue: errors.New("two")},
		},
	}
	_, errorValue := auto.CompleteText(context.Background(), TextRequest{})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "two") {
		t.Fatalf("expected last error from chain, got %v", errorValue)
	}
}

func TestAutoProviderReportsNoProviderError(t *testing.T) {
	auto := AutoProvider{Providers: nil}
	_, errorValue := auto.CompleteText(context.Background(), TextRequest{})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "no llm provider") {
		t.Fatalf("expected no provider error, got %v", errorValue)
	}
}

func TestOpenRouterPingDetectsPlaceholderKey(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "key")
	if errorValue := os.WriteFile(secretPath, []byte("internkim-simulation-openrouter-api-key"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	backend := OpenRouterBackend{KeyPath: secretPath}
	if errorValue := backend.Ping(context.Background()); errorValue == nil {
		t.Fatal("expected placeholder key to fail ping")
	}
}

func TestOllamaStreamTextEmitsTokens(t *testing.T) {
	backend := OllamaBackend{
		BaseURL:   "https://ollama.test",
		ModelName: "gemma3:1b",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			body := strings.Join([]string{
				`{"message":{"role":"assistant","content":"hel"},"done":false}`,
				`{"message":{"role":"assistant","content":"lo"},"done":false}`,
				`{"message":{"role":"assistant","content":""},"done":true}`,
			}, "\n")
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	tokens := []string{}
	errorValue := backend.StreamText(context.Background(), TextRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	}, func(token string) {
		tokens = append(tokens, token)
	})
	if errorValue != nil {
		t.Fatalf("expected stream success: %v", errorValue)
	}
	if strings.Join(tokens, "") != "hello" {
		t.Fatalf("expected hello, got %v", tokens)
	}
}

type staticProvider struct {
	response   Response
	errorValue error
}

func (provider staticProvider) CompleteStructured(context.Context, StructuredRequest) (Response, error) {
	return provider.response, provider.errorValue
}

func (provider staticProvider) CompleteText(context.Context, TextRequest) (Response, error) {
	return provider.response, provider.errorValue
}
