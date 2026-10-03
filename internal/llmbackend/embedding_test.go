package llmbackend

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddingInputsRemainUnchangedForOtherModels(t *testing.T) {
	inputs := []string{"hello"}
	preparedInputs := prepareEmbeddingInputs(inputs, EmbeddingRequest{InputType: "query"}, "baai/bge-m3", false)

	if len(preparedInputs) != 1 || preparedInputs[0] != "hello" {
		t.Fatalf("expected unchanged input, got %+v", preparedInputs)
	}
}

func TestQwen3EmbeddingQueriesCarryTheRetrievalInstruction(t *testing.T) {
	preparedInputs := prepareEmbeddingInputs([]string{"who runs payroll"}, EmbeddingRequest{InputType: "query"}, "qwen/qwen3-embedding-8b", false)

	expected := "Instruct: Given a question about a person or their work, retrieve the memory facts that answer it\nQuery: who runs payroll"
	if len(preparedInputs) != 1 || preparedInputs[0] != expected {
		t.Fatalf("expected the query instruction prefix, got %+v", preparedInputs)
	}
}

func TestQwen3EmbeddingDocumentsAreSentAsIs(t *testing.T) {
	inputs := []string{"이샘플 runs payroll", "박예시 prefers morning meetings"}
	preparedInputs := prepareEmbeddingInputs(inputs, EmbeddingRequest{InputType: "document"}, "Qwen/Qwen3-Embedding-8B", true)

	if len(preparedInputs) != 2 || preparedInputs[0] != inputs[0] || preparedInputs[1] != inputs[1] {
		t.Fatalf("expected documents unchanged, got %+v", preparedInputs)
	}
}

func TestQwen3EmbeddingKeepsAnInstructionTheCallerAlreadyWrote(t *testing.T) {
	input := "Instruct: Retrieve passages\nQuery: who runs payroll"
	preparedInputs := prepareEmbeddingInputs([]string{input}, EmbeddingRequest{InputType: "query"}, "qwen/qwen3-embedding-8b", false)

	if len(preparedInputs) != 1 || preparedInputs[0] != input {
		t.Fatalf("expected the caller's instruction kept, got %+v", preparedInputs)
	}
}

func TestOpenRouterEmbeddingBackendKeepsCanonicalModelName(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-key")
	if errorValue := os.WriteFile(secretPath, []byte("test-key"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	backend := OpenRouterEmbeddingBackend{
		KeyPath:   secretPath,
		BaseURL:   "https://openrouter.test/embeddings",
		ModelName: "baai/bge-m3",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"model":"parasail-bge-m3","data":[{"embedding":[0.1,0.2]}]}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	response, errorValue := backend.CreateEmbedding(context.Background(), EmbeddingRequest{Input: "hello"})
	if errorValue != nil {
		t.Fatalf("expected embedding to succeed: %v", errorValue)
	}
	if response.Model != "baai/bge-m3" {
		t.Fatalf("expected canonical model name, got %q", response.Model)
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
