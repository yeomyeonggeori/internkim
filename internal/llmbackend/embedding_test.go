package llmbackend

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLlamaCppEmbeddingBackendUsesEmbeddingGemmaPrompt(t *testing.T) {
	backend := LlamaCppEmbeddingBackend{
		BaseURL:   "https://llamacpp.test",
		ModelName: EmbeddingGemmaModelName,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.Path != "/v1/embeddings" {
				t.Fatalf("unexpected path: %s", request.URL.Path)
			}
			var document map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&document); errorValue != nil {
				t.Fatal(errorValue)
			}
			if document["model"] != EmbeddingGemmaModelName {
				t.Fatalf("unexpected model: %v", document["model"])
			}
			if document["input"] != "task: search result | query: hello" {
				t.Fatalf("unexpected input prompt: %v", document["input"])
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"model":"embeddinggemma","data":[{"embedding":[0.3,0.4]}]}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	response, errorValue := backend.CreateEmbedding(context.Background(), EmbeddingRequest{Input: "hello"})
	if errorValue != nil {
		t.Fatalf("expected embedding to succeed: %v", errorValue)
	}
	if response.Provider != "llamacpp" || response.Model != EmbeddingGemmaModelName {
		t.Fatalf("unexpected provider response: %+v", response)
	}
	if len(response.Embedding) != 2 || response.Embedding[0] != 0.3 || len(response.Embeddings) != 0 {
		t.Fatalf("unexpected embedding response: %+v", response)
	}
}

func TestLlamaCppEmbeddingBackendPromptsBatchAsDocuments(t *testing.T) {
	backend := LlamaCppEmbeddingBackend{
		BaseURL:   "https://llamacpp.test",
		ModelName: EmbeddingGemmaModelName,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			var document map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&document); errorValue != nil {
				t.Fatal(errorValue)
			}
			inputs, ok := document["input"].([]any)
			if !ok || len(inputs) != 2 {
				t.Fatalf("expected batch input, got: %v", document["input"])
			}
			if inputs[0] != "title: none | text: alpha" || inputs[1] != "title: none | text: beta" {
				t.Fatalf("unexpected document prompts: %v", inputs)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"model":"embeddinggemma","data":[{"embedding":[1,0]},{"embedding":[0,1]}]}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	response, errorValue := backend.CreateEmbedding(context.Background(), EmbeddingRequest{Input: []any{"alpha", "beta"}})
	if errorValue != nil {
		t.Fatalf("expected embedding to succeed: %v", errorValue)
	}
	if len(response.Embeddings) != 2 || len(response.Embedding) != 0 {
		t.Fatalf("unexpected batch response: %+v", response)
	}
}

func TestEmbeddingInputsRemainUnchangedForOtherModels(t *testing.T) {
	inputs := []string{"hello"}
	preparedInputs := prepareEmbeddingInputs(inputs, EmbeddingRequest{InputType: "query"}, "baai/bge-m3", false)

	if len(preparedInputs) != 1 || preparedInputs[0] != "hello" {
		t.Fatalf("expected unchanged input, got %+v", preparedInputs)
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
	response := finalizeEmbeddingResponse(EmbeddingResponse{
		Embedding: []float64{3, 4, 100},
	}, EmbeddingRequest{OutputDimensions: 2})

	if len(response.Embedding) != 2 {
		t.Fatalf("expected truncated embedding, got %+v", response.Embedding)
	}
	if response.Embedding[0] != 0.6 || response.Embedding[1] != 0.8 {
		t.Fatalf("expected normalized embedding, got %+v", response.Embedding)
	}
}
