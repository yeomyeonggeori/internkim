//go:build llmeval

package llmbackend

import (
	"bytes"
	"context"
	"io"
	"math"
	"net/http"
	"testing"
)

type recordingTransport struct{ bodies [][]byte }

func (transport *recordingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	body, errorValue := io.ReadAll(request.Body)
	if errorValue != nil {
		return nil, errorValue
	}
	transport.bodies = append(transport.bodies, body)
	request.Body = io.NopCloser(bytes.NewReader(body))
	return http.DefaultTransport.RoundTrip(request)
}

func cosineSimilarity(left []float64, right []float64) float64 {
	var dot, leftNorm, rightNorm float64
	for index := range left {
		dot += left[index] * right[index]
		leftNorm += left[index] * left[index]
		rightNorm += right[index] * right[index]
	}
	return dot / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm))
}

func TestLiveEmbeddingGemmaServerEmbedsWithThePromptsAndWidthTheHostExpects(t *testing.T) {
	serverURL := testEnvValue("INTERNKIM_EMBEDDING_SERVER_URL", "")
	if serverURL == "" {
		t.Skip("INTERNKIM_EMBEDDING_SERVER_URL names no running llama-server")
	}
	transport := &recordingTransport{}
	backend := LlamaCppEmbeddingBackend{BaseURL: serverURL, ModelName: DefaultEmbeddingModelName, HTTPClient: &http.Client{Transport: transport}}

	query, errorValue := backend.CreateEmbedding(context.Background(), EmbeddingRequest{Input: "who is responsible for payroll", Model: DefaultEmbeddingModelName, OutputDimensions: 768})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	documents, errorValue := backend.CreateEmbedding(context.Background(), EmbeddingRequest{
		Input: []string{"이샘플 runs payroll for the whole company every month.", "The office plants are watered on Fridays."},
		Model: DefaultEmbeddingModelName, InputType: "document", OutputDimensions: 768,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(query.Embedding) != 768 || len(documents.Embeddings) != 2 || len(documents.Embeddings[0]) != 768 {
		t.Fatalf("expected 768-dimensional vectors, got %d and %d", len(query.Embedding), len(documents.Embeddings[0]))
	}
	related, unrelated := cosineSimilarity(query.Embedding, documents.Embeddings[0]), cosineSimilarity(query.Embedding, documents.Embeddings[1])
	t.Logf("query to related document %.4f, to unrelated document %.4f", related, unrelated)
	if related <= unrelated {
		t.Fatalf("the related document scored %.4f and the unrelated one %.4f", related, unrelated)
	}
	if !bytes.Contains(transport.bodies[0], []byte(`"task: search result | query: who is responsible for payroll"`)) {
		t.Fatalf("the query went out without its prompt: %s", transport.bodies[0])
	}
	if !bytes.Contains(transport.bodies[1], []byte(`"title: none | text: 이샘플 runs payroll`)) {
		t.Fatalf("the documents went out without their prompt: %s", transport.bodies[1])
	}
	withoutPrompt, errorValue := backend.CreateEmbedding(context.Background(), EmbeddingRequest{Input: "task: search result | query: who is responsible for payroll", Model: DefaultEmbeddingModelName})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if agreement := cosineSimilarity(query.Embedding, withoutPrompt.Embedding); agreement < 0.9999 {
		t.Fatalf("the prompted query and the same text written out by hand differ: cosine %.6f", agreement)
	}
}
