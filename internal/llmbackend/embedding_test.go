package llmbackend

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestEmbeddingInputsRemainUnchangedForOtherModels(t *testing.T) {
	inputs := []string{"hello"}
	preparedInputs := prepareEmbeddingInputs(inputs, EmbeddingRequest{InputType: "query"}, "baai/bge-m3")

	if len(preparedInputs) != 1 || preparedInputs[0] != "hello" {
		t.Fatalf("expected unchanged input, got %+v", preparedInputs)
	}
}

func TestEveryEmbeddingGemmaNameGetsThePrompts(t *testing.T) {
	for _, modelName := range []string{"embeddinggemma", "google/embeddinggemma-2", "Google/EmbeddingGemma-300m", "ggml-org/embeddinggemma-2-GGUF"} {
		preparedInputs := prepareEmbeddingInputs([]string{"who runs payroll"}, EmbeddingRequest{InputType: "query"}, modelName)

		if len(preparedInputs) != 1 || preparedInputs[0] != "task: search result | query: who runs payroll" {
			t.Fatalf("%s: expected the query prompt, got %+v", modelName, preparedInputs)
		}
	}
}

func TestEmbeddingGemmaDocumentsCarryTheirTitle(t *testing.T) {
	titled := prepareEmbeddingInputs([]string{"이샘플 runs payroll"}, EmbeddingRequest{InputType: "document", Title: "payroll"}, "google/embeddinggemma-2")
	untitled := prepareEmbeddingInputs([]string{"이샘플 runs payroll"}, EmbeddingRequest{InputType: "document"}, "google/embeddinggemma-2")

	if titled[0] != "title: payroll | text: 이샘플 runs payroll" {
		t.Fatalf("expected the titled document prompt, got %q", titled[0])
	}
	if untitled[0] != "title: none | text: 이샘플 runs payroll" {
		t.Fatalf("expected the untitled document prompt, got %q", untitled[0])
	}
}

func llamaCppServerAnswering(t *testing.T, received *[]map[string]any) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/embeddings" {
			t.Errorf("unexpected path %s", request.URL.Path)
		}
		var document map[string]any
		if errorValue := json.NewDecoder(request.Body).Decode(&document); errorValue != nil {
			t.Error(errorValue)
		}
		*received = append(*received, document)
		io.WriteString(responseWriter, `{"model":"embeddinggemma-2-Q8_0.gguf","data":[{"embedding":[3,4,100]},{"embedding":[6,8,100]}]}`)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestLlamaCppEmbeddingSendsTheQueryPromptForAQuery(t *testing.T) {
	received := []map[string]any{}
	server := llamaCppServerAnswering(t, &received)
	backend := LlamaCppEmbeddingBackend{BaseURL: server.URL, ModelName: "google/embeddinggemma-2"}

	response, errorValue := backend.CreateEmbedding(context.Background(), EmbeddingRequest{Input: "who runs payroll", Model: "google/embeddinggemma-2", InputType: "query"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if received[0]["input"] != "task: search result | query: who runs payroll" {
		t.Fatalf("expected the query prompt on the wire, got %v", received[0]["input"])
	}
	if response.Model != "google/embeddinggemma-2" || len(response.Embedding) != 3 || response.Embeddings != nil {
		t.Fatalf("unexpected response %+v", response)
	}
}

func TestLlamaCppEmbeddingSendsDocumentPromptsForDocuments(t *testing.T) {
	received := []map[string]any{}
	server := llamaCppServerAnswering(t, &received)
	backend := LlamaCppEmbeddingBackend{BaseURL: server.URL}

	response, errorValue := backend.CreateEmbedding(context.Background(), EmbeddingRequest{Input: []string{"a", "b"}, Model: "default", InputType: "document"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	inputs, isList := received[0]["input"].([]any)
	if !isList || inputs[0] != "title: none | text: a" || inputs[1] != "title: none | text: b" {
		t.Fatalf("expected document prompts on the wire, got %v", received[0]["input"])
	}
	if response.Model != DefaultEmbeddingModelName || len(response.Embeddings) != 2 {
		t.Fatalf("unexpected response %+v", response)
	}
}

func TestLlamaCppEmbeddingTruncatesAndNormalizesToTheRequestedDimensions(t *testing.T) {
	received := []map[string]any{}
	server := llamaCppServerAnswering(t, &received)
	backend := LlamaCppEmbeddingBackend{BaseURL: server.URL}

	response, errorValue := backend.CreateEmbedding(context.Background(), EmbeddingRequest{Input: "hello", OutputDimensions: 2})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if response.Embedding[0] != 0.6 || response.Embedding[1] != 0.8 {
		t.Fatalf("expected the normalized prefix, got %+v", response.Embedding)
	}
	if received[0]["dimensions"] != float64(2) {
		t.Fatalf("expected the requested width on the wire, got %v", received[0]["dimensions"])
	}
}

func TestLlamaCppEmbeddingRefusesAModelItDoesNotServe(t *testing.T) {
	received := []map[string]any{}
	server := llamaCppServerAnswering(t, &received)

	_, errorValue := LlamaCppEmbeddingBackend{BaseURL: server.URL}.CreateEmbedding(context.Background(), EmbeddingRequest{Input: "hello", Model: "baai/bge-m3"})

	if errorValue == nil || !strings.Contains(errorValue.Error(), "baai/bge-m3") || len(received) != 0 {
		t.Fatalf("expected a refusal naming the model before any request, got %v after %d requests", errorValue, len(received))
	}
}

func TestLlamaCppEmbeddingWaitsOutAServerThatIsStillLoadingItsModel(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if calls.Add(1) == 1 {
			http.Error(responseWriter, `{"error":{"message":"Loading model"}}`, http.StatusServiceUnavailable)
			return
		}
		io.WriteString(responseWriter, `{"data":[{"embedding":[1,0]}]}`)
	}))
	defer server.Close()

	response, errorValue := LlamaCppEmbeddingBackend{BaseURL: server.URL}.CreateEmbedding(context.Background(), EmbeddingRequest{Input: "hello"})

	if errorValue != nil || len(response.Embedding) != 2 || calls.Load() != 2 {
		t.Fatalf("expected one retry then an embedding, got %+v after %d calls: %v", response, calls.Load(), errorValue)
	}
}

func TestLlamaCppEmbeddingDoesNotRetryAClientError(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		http.Error(responseWriter, `{"error":{"message":"input is too large"}}`, http.StatusBadRequest)
	}))
	defer server.Close()

	_, errorValue := LlamaCppEmbeddingBackend{BaseURL: server.URL}.CreateEmbedding(context.Background(), EmbeddingRequest{Input: "hello"})

	if errorValue == nil || !strings.Contains(errorValue.Error(), "input is too large") || calls.Load() != 1 {
		t.Fatalf("expected one call and the server's reason, got %d calls: %v", calls.Load(), errorValue)
	}
}

func TestEmbeddingOutputDimensionsNormalizeTruncatedVector(t *testing.T) {
	response, errorValue := finalizeEmbeddingResponse(EmbeddingResponse{
		Embedding: []float64{3, 4, 100},
	}, EmbeddingRequest{OutputDimensions: 2})

	if errorValue != nil || len(response.Embedding) != 2 {
		t.Fatalf("expected truncated embedding, got %+v", response.Embedding)
	}
	if response.Embedding[0] != 0.6 || response.Embedding[1] != 0.8 {
		t.Fatalf("expected normalized embedding, got %+v", response.Embedding)
	}
}

func TestUntypedEmbeddingInputIsSentAsWritten(t *testing.T) {
	preparedInputs := prepareEmbeddingInputs([]string{"who runs payroll"}, EmbeddingRequest{}, "google/embeddinggemma-2")

	if preparedInputs[0] != "who runs payroll" {
		t.Fatalf("expected an input with no type to go out unchanged, got %q", preparedInputs[0])
	}
}
