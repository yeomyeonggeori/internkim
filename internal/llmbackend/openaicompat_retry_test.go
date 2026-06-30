package llmbackend

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChatCompletionRetriesOnUpstream429(t *testing.T) {
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		attempts++
		if attempts < 3 {
			responseWriter.WriteHeader(http.StatusTooManyRequests)
			_, _ = responseWriter.Write([]byte(`{"error":{"message":"rate limited"}}`))
			return
		}
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{}}`))
	}))
	defer server.Close()

	client := openAICompatClient{ProviderName: "test", BaseURL: server.URL, ModelName: "m"}
	response, errorValue := client.chatCompletion(context.Background(), ChatRequest{
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	})
	if errorValue != nil {
		t.Fatalf("expected success after retries, got %v", errorValue)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts (2 retries), got %d", attempts)
	}
	if response.Message.Content != "ok" {
		t.Fatalf("expected content 'ok', got %q", response.Message.Content)
	}
}

func TestChatCompletionDoesNotRetryClientError(t *testing.T) {
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		attempts++
		responseWriter.WriteHeader(http.StatusBadRequest)
		_, _ = responseWriter.Write([]byte(`{"error":{"message":"bad request"}}`))
	}))
	defer server.Close()

	client := openAICompatClient{ProviderName: "test", BaseURL: server.URL, ModelName: "m"}
	_, errorValue := client.chatCompletion(context.Background(), ChatRequest{
		Messages: []ChatMessage{{Role: "user", Content: "hi"}},
	})
	if errorValue == nil {
		t.Fatal("expected error on 400")
	}
	if attempts != 1 {
		t.Fatalf("expected no retry on 400, got %d attempts", attempts)
	}
}
