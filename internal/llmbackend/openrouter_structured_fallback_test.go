package llmbackend

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestOpenRouterStructuredFailureFallsBackToConfiguredProvider(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	requestDocuments := []map[string]any{}
	backend := OpenRouterBackend{
		KeyPath:   secretPath,
		BaseURL:   "https://openrouter.test/api/v1/chat/completions",
		ModelName: "z-ai/glm-5.3-flash",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			var requestDocument map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&requestDocument); errorValue != nil {
				t.Fatalf("expected request body: %v", errorValue)
			}
			requestDocuments = append(requestDocuments, requestDocument)
			content := "not json"
			if len(requestDocuments) == 2 {
				content = ""
			}
			if len(requestDocuments) == 3 {
				content = `{"answer":"fallback"}`
			}
			encodedContent, errorValue := json.Marshal(content)
			if errorValue != nil {
				t.Fatalf("expected response content to encode: %v", errorValue)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"stop","message":{"content":` + string(encodedContent) + `}}]}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}
	backend.FallbackModelNames = []string{"fallback-model"}
	response, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
		Model:    "z-ai/glm-5.3-flash",
		Messages: []Message{{Role: "user", Content: "route this request"}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:               "bluecollar_turn_router",
			Document:           json.RawMessage(`{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"],"additionalProperties":false}`),
			IsStrictlyEnforced: true,
		},
	})

	if errorValue != nil {
		t.Fatalf("expected configured fallback to succeed: %v", errorValue)
	}
	if response.Provider != "openrouter" || response.Model != "fallback-model" {
		t.Fatalf("expected fallback provider response, got %+v", response)
	}
	if len(requestDocuments) != 3 {
		t.Fatalf("expected schema, prompted, and configured fallback requests, got %d", len(requestDocuments))
	}
	responseFormat, isResponseFormat := requestDocuments[0]["response_format"].(map[string]any)
	if !isResponseFormat {
		t.Fatalf("expected schema constrained first request, got %+v", requestDocuments[0])
	}
	jsonSchema := responseFormat["json_schema"].(map[string]any)
	if jsonSchema["name"] != "bluecollar_turn_router" || jsonSchema["strict"] != true {
		t.Fatalf("expected router schema metadata to be preserved, got %+v", jsonSchema)
	}
	if !reflect.DeepEqual(jsonSchema["schema"], map[string]any{
		"type":                 "object",
		"properties":           map[string]any{"answer": map[string]any{"type": "string"}},
		"required":             []any{"answer"},
		"additionalProperties": false,
	}) {
		t.Fatalf("expected router schema to be preserved, got %+v", jsonSchema["schema"])
	}
	fallbackResponseFormat, isFallbackResponseFormat := requestDocuments[2]["response_format"].(map[string]any)
	if !isFallbackResponseFormat || !reflect.DeepEqual(fallbackResponseFormat["json_schema"].(map[string]any)["schema"], jsonSchema["schema"]) {
		t.Fatalf("expected fallback request to preserve router schema, got %+v", requestDocuments[2])
	}
}

func TestOpenRouterStructuredFailureWithoutConfiguredFallbackReturnsError(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	backend := OpenRouterBackend{
		KeyPath:   secretPath,
		BaseURL:   "https://openrouter.test/api/v1/chat/completions",
		ModelName: "z-ai/glm-5.3-flash",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"stop","message":{"content":"not json"}}]}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	_, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
		StructuredOutputSchema: StructuredOutputSchema{Name: "router", Document: json.RawMessage(`{"type":"object"}`)},
	})
	if errorValue == nil {
		t.Fatal("expected structured failure without configured fallback")
	}
}

func TestOpenRouterStructuredFailureWithCanceledContextStopsRetry(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	responseContext, cancel := context.WithCancel(context.Background())
	requestCount := 0
	backend := OpenRouterBackend{
		KeyPath:   secretPath,
		BaseURL:   "https://openrouter.test/api/v1/chat/completions",
		ModelName: "z-ai/glm-5.3-flash",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestCount++
			cancel()
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"stop","message":{"content":"not json"}}]}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	_, errorValue := backend.CompleteStructured(responseContext, StructuredRequest{
		StructuredOutputSchema: StructuredOutputSchema{Name: "router", Document: json.RawMessage(`{"type":"object"}`)},
	})
	if !errors.Is(errorValue, context.Canceled) {
		t.Fatalf("expected canceled context error, got %v", errorValue)
	}
	if requestCount != 1 {
		t.Fatalf("expected canceled context to stop prompted retry, got %d requests", requestCount)
	}
}
