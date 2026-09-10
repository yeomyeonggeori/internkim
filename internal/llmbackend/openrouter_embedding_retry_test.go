package llmbackend

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeOpenRouterKeyFile(t *testing.T) string {
	t.Helper()
	keyPath := filepath.Join(t.TempDir(), "openrouter.key")
	if errorValue := os.WriteFile(keyPath, []byte("sk-or-v1-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return keyPath
}

func TestOpenRouterEmbeddingRetriesOnUpstream429(t *testing.T) {
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		attempts++
		if attempts < 3 {
			responseWriter.Header().Set("Retry-After", "0")
			responseWriter.WriteHeader(http.StatusTooManyRequests)
			_, _ = responseWriter.Write([]byte(`{"error":{"message":"rate limited"}}`))
			return
		}
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte(`{"model":"m","data":[{"embedding":[0.6,0.8]}]}`))
	}))
	defer server.Close()

	backend := OpenRouterEmbeddingBackend{KeyPath: writeOpenRouterKeyFile(t), BaseURL: server.URL, ModelName: "m"}
	response, errorValue := backend.CreateEmbedding(context.Background(), EmbeddingRequest{Input: "hello"})
	if errorValue != nil {
		t.Fatalf("expected success after retries, got %v", errorValue)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts (2 retries), got %d", attempts)
	}
	if len(response.Embedding) != 2 {
		t.Fatalf("expected the embedding from the final attempt, got %v", response.Embedding)
	}
}

func TestOpenRouterEmbeddingDoesNotRetryClientError(t *testing.T) {
	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		attempts++
		responseWriter.WriteHeader(http.StatusBadRequest)
		_, _ = responseWriter.Write([]byte(`{"error":{"message":"bad request"}}`))
	}))
	defer server.Close()

	backend := OpenRouterEmbeddingBackend{KeyPath: writeOpenRouterKeyFile(t), BaseURL: server.URL, ModelName: "m"}
	_, errorValue := backend.CreateEmbedding(context.Background(), EmbeddingRequest{Input: "hello"})
	if errorValue == nil {
		t.Fatal("expected error on 400")
	}
	if attempts != 1 {
		t.Fatalf("expected no retry on 400, got %d attempts", attempts)
	}
}

func TestEmbeddingRetryDelayHonoursRetryAfter(t *testing.T) {
	throttled := retryableEmbeddingError{cause: http.ErrHandlerTimeout, retryAfter: 3 * time.Second}
	if delay := embeddingRetryDelay(1, throttled); delay != 3*time.Second {
		t.Fatalf("expected Retry-After to win, got %v", delay)
	}
	tooLong := retryableEmbeddingError{cause: http.ErrHandlerTimeout, retryAfter: 10 * time.Minute}
	if delay := embeddingRetryDelay(1, tooLong); delay != maxEmbeddingRetryAfter {
		t.Fatalf("expected Retry-After to be capped, got %v", delay)
	}
	if delay := embeddingRetryDelay(2, retryableEmbeddingError{cause: http.ErrHandlerTimeout}); delay != 2*time.Second {
		t.Fatalf("expected exponential backoff without Retry-After, got %v", delay)
	}
}
