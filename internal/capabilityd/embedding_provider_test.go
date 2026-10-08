package capabilityd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func embeddingServerRecording(t *testing.T, received *[]map[string]any) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		var document map[string]any
		if errorValue := json.NewDecoder(request.Body).Decode(&document); errorValue != nil {
			t.Error(errorValue)
		}
		*received = append(*received, document)
		io.WriteString(responseWriter, `{"model":"embeddinggemma-2-Q8_0.gguf","data":[{"embedding":[0.1,0.2]}]}`)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestEveryExecutionModeEmbedsOnTheLocalServer(t *testing.T) {
	for _, executionMode := range []string{"", "auto", "remote", "device"} {
		received := []map[string]any{}
		server := embeddingServerRecording(t, &received)
		service := Service{Configuration: Configuration{EmbeddingServerURL: server.URL}}

		response, errorValue := service.createEmbedding(context.Background(), EmbeddingRequest{Input: "hello", Model: "google/embeddinggemma-2", InputType: "query", ExecutionMode: executionMode})
		if errorValue != nil {
			t.Fatalf("execution mode %q: %v", executionMode, errorValue)
		}

		if len(received) != 1 || received[0]["input"] != "task: search result | query: hello" {
			t.Fatalf("execution mode %q: the local server received %v", executionMode, received)
		}
		if response.Model != "google/embeddinggemma-2" || len(response.Embedding) != 2 {
			t.Fatalf("execution mode %q: unexpected response %+v", executionMode, response)
		}
	}
}

func TestLocalOnlyModeStillEmbeds(t *testing.T) {
	received := []map[string]any{}
	server := embeddingServerRecording(t, &received)
	service := Service{Configuration: Configuration{EmbeddingServerURL: server.URL, LocalOnly: true}}

	if _, errorValue := service.createEmbedding(context.Background(), EmbeddingRequest{Input: "hello"}); errorValue != nil {
		t.Fatalf("the embedding server is on this host, so local-only mode has nothing to refuse: %v", errorValue)
	}
}

func TestAnUnknownEmbeddingExecutionModeIsRefused(t *testing.T) {
	received := []map[string]any{}
	server := embeddingServerRecording(t, &received)
	service := Service{Configuration: Configuration{EmbeddingServerURL: server.URL}}

	_, errorValue := service.createEmbedding(context.Background(), EmbeddingRequest{Input: "hello", ExecutionMode: "somewhere"})

	if errorValue == nil || !strings.Contains(errorValue.Error(), "not supported") || len(received) != 0 {
		t.Fatalf("expected a refusal before any request, got %v after %d requests", errorValue, len(received))
	}
}

func TestEmbeddingsAreRefusedForAModelTheHostDoesNotServe(t *testing.T) {
	received := []map[string]any{}
	server := embeddingServerRecording(t, &received)
	service := Service{Configuration: Configuration{EmbeddingServerURL: server.URL}}

	_, errorValue := service.createEmbedding(context.Background(), EmbeddingRequest{Input: "hello", Model: "baai/bge-m3"})

	if errorValue == nil || len(received) != 0 {
		t.Fatalf("expected a refusal, got %v after %d requests", errorValue, len(received))
	}
}
