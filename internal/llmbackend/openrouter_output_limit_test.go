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
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestOpenRouterTruncatedStructuredOutputUsesConfiguredFallback(t *testing.T) {
	truncatedContent := []struct {
		name    string
		content string
	}{
		{name: "empty", content: ""},
		{name: "prose", content: "I need more space to answer"},
		{name: "valid JSON", content: `{"answer":"partial"}`},
	}

	for _, testCase := range truncatedContent {
		t.Run(testCase.name, func(t *testing.T) {
			secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
			if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
				t.Fatal(errorValue)
			}
			schema := json.RawMessage(`{"type":"object","properties":{"answer":{"type":"string"}},"required":["answer"],"additionalProperties":false}`)
			messages := []Message{{Role: "user", Content: "route this request"}}
			requestDocuments := []map[string]any{}
			backend := OpenRouterBackend{
				KeyPath:            secretPath,
				BaseURL:            "https://openrouter.test/api/v1/chat/completions",
				ModelName:          "primary-model",
				FallbackModelNames: []string{"fallback-model"},
				HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					var requestDocument map[string]any
					if errorValue := json.NewDecoder(request.Body).Decode(&requestDocument); errorValue != nil {
						t.Fatalf("expected request body: %v", errorValue)
					}
					requestDocuments = append(requestDocuments, requestDocument)
					content := testCase.content
					if len(requestDocuments) == 2 {
						content = `{"answer":"fallback"}`
					}
					encodedContent, errorValue := json.Marshal(content)
					if errorValue != nil {
						t.Fatalf("expected response content to encode: %v", errorValue)
					}
					completionTokens := 1600
					if len(requestDocuments) == 2 {
						completionTokens = 4
					}
					body := `{"choices":[{"finish_reason":"length","message":{"content":` + string(encodedContent) + `}}],"usage":{"prompt_tokens":20,"completion_tokens":` + strconv.Itoa(completionTokens) + `}}`
					if len(requestDocuments) == 2 {
						body = `{"choices":[{"finish_reason":"stop","message":{"content":` + string(encodedContent) + `}}],"usage":{"prompt_tokens":20,"completion_tokens":4}}`
					}
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
				})},
			}
			response, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
				Model:                  "primary-model",
				Messages:               messages,
				StructuredOutputSchema: StructuredOutputSchema{Name: "router", Document: schema, IsStrictlyEnforced: true},
				GenerationOptions:      &GenerationOptions{MaxTokens: intPointer(1600)},
			})
			if errorValue != nil {
				t.Fatalf("expected configured fallback to succeed: %v", errorValue)
			}
			if response.Model != "fallback-model" || !response.UsedFallback || !strings.Contains(response.FallbackReason, "finish_reason=length") || !strings.Contains(response.FallbackReason, "completion_tokens=1600") {
				t.Fatalf("expected truncated fallback metadata, got %+v", response)
			}
			if len(requestDocuments) != 2 || requestDocuments[0]["model"] != "primary-model" || requestDocuments[1]["model"] != "fallback-model" {
				t.Fatalf("expected one request per model, got %+v", requestDocuments)
			}
			assertStructuredRequestPreserved(t, requestDocuments[0], requestDocuments[1], 1600)
		})
	}
}

func TestOpenRouterTruncatedStructuredOutputWithoutFallbackReturnsExplicitError(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	backend := OpenRouterBackend{
		KeyPath:   secretPath,
		BaseURL:   "https://openrouter.test/api/v1/chat/completions",
		ModelName: "primary-model",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"length","message":{"content":"partial"}}],"usage":{"completion_tokens":1600}}`)), Header: make(http.Header)}, nil
		})},
	}

	_, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
		StructuredOutputSchema: StructuredOutputSchema{Name: "router", Document: json.RawMessage(`{"type":"object"}`)},
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "finish_reason=length") || !strings.Contains(errorValue.Error(), "completion_tokens=1600") {
		t.Fatalf("expected explicit truncation error, got %v", errorValue)
	}
}

func TestOpenRouterTruncatedStructuredOutputPreservesDeadlineAndStopsAfterCancellation(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	responseContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	deadline := time.Now().Add(time.Minute)
	responseContext, cancelDeadline := context.WithDeadline(responseContext, deadline)
	defer cancelDeadline()
	requestCount := 0
	deadlinePreserved := false
	backend := OpenRouterBackend{
		KeyPath:            secretPath,
		BaseURL:            "https://openrouter.test/api/v1/chat/completions",
		ModelName:          "primary-model",
		FallbackModelNames: []string{"fallback-model"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestCount++
			_, deadlinePreserved = request.Context().Deadline()
			cancel()
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"length","message":{"content":"partial"}}],"usage":{"completion_tokens":1600}}`)), Header: make(http.Header)}, nil
		})},
	}

	_, errorValue := backend.CompleteStructured(responseContext, StructuredRequest{
		StructuredOutputSchema: StructuredOutputSchema{Name: "router", Document: json.RawMessage(`{"type":"object"}`)},
	})
	if !errors.Is(errorValue, context.Canceled) || !deadlinePreserved || requestCount != 1 {
		t.Fatalf("expected cancellation with preserved deadline and one request, error=%v deadline=%t requests=%d", errorValue, deadlinePreserved, requestCount)
	}
}

func assertStructuredRequestPreserved(t *testing.T, primary, fallback map[string]any, maxTokens int) {
	t.Helper()
	if primary["response_format"] == nil || !reflect.DeepEqual(primary["response_format"], fallback["response_format"]) || !reflect.DeepEqual(primary["messages"], fallback["messages"]) || primary["max_tokens"] != float64(maxTokens) || fallback["max_tokens"] != float64(maxTokens) {
		t.Fatalf("expected schema, messages, and max_tokens to be preserved, primary=%+v fallback=%+v", primary, fallback)
	}
}

func intPointer(value int) *int {
	return &value
}
