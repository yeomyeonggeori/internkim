package llmbackend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"time"
)

const (
	EmbeddingGemmaModelName   = "embeddinggemma"
	DefaultEmbeddingModelName = "baai/bge-m3"
)

type EmbeddingRequest struct {
	Input            any    `json:"input"`
	Model            string `json:"model,omitempty"`
	Provider         string `json:"provider,omitempty"`
	ExecutionMode    string `json:"executionMode,omitempty"`
	Task             string `json:"task,omitempty"`
	InputType        string `json:"inputType,omitempty"`
	Title            string `json:"title,omitempty"`
	OutputDimensions int    `json:"outputDimensions,omitempty"`
}

type EmbeddingResponse struct {
	Provider        string      `json:"provider"`
	Model           string      `json:"model"`
	SelectedBackend string      `json:"selectedBackend"`
	Embedding       []float64   `json:"embedding,omitempty"`
	Embeddings      [][]float64 `json:"embeddings,omitempty"`
}

type EmbeddingCreator interface {
	CreateEmbedding(context.Context, EmbeddingRequest) (EmbeddingResponse, error)
}

type EmbeddingProvider interface {
	EmbeddingCreator
}

type EmbeddingBackend interface {
	EmbeddingProvider
	Name() string
	Ping(context.Context) error
}

type AutoEmbeddingProvider struct {
	Providers      []EmbeddingProvider
	AttemptTimeout time.Duration
}

func (provider AutoEmbeddingProvider) CreateEmbedding(ctx context.Context, request EmbeddingRequest) (EmbeddingResponse, error) {
	return createWithEmbeddingProviderChain(provider.Providers, func(candidate EmbeddingProvider) (EmbeddingResponse, error) {
		attemptContext, cancel := providerAttemptContext(ctx, provider.AttemptTimeout)
		defer cancel()
		return candidate.CreateEmbedding(attemptContext, request)
	})
}

func createWithEmbeddingProviderChain(providers []EmbeddingProvider, create func(EmbeddingProvider) (EmbeddingResponse, error)) (EmbeddingResponse, error) {
	attempts := make([]string, 0, len(providers))
	for index, candidate := range providers {
		if candidate == nil {
			continue
		}
		response, errorValue := create(candidate)
		if errorValue == nil {
			return response, nil
		}
		attempts = append(attempts, embeddingProviderFailure(candidate, errorValue))
		if index < len(providers)-1 {
			logEmbeddingFallback(errorValue)
		}
	}
	if len(attempts) == 0 {
		return EmbeddingResponse{}, errors.New("no embedding provider is available")
	}
	return EmbeddingResponse{}, errors.New("embedding provider attempts failed: " + strings.Join(attempts, "; "))
}

func embeddingProviderFailure(provider EmbeddingProvider, errorValue error) string {
	if namedProvider, ok := provider.(interface{ Name() string }); ok {
		return namedProvider.Name() + ": " + errorValue.Error()
	}
	return errorValue.Error()
}

func logEmbeddingFallback(errorValue error) {
	if errorValue != nil {
		log.Printf("embedding provider failed; trying next provider: %v", errorValue)
	}
}

func normalizeEmbeddingInputs(input any) ([]string, bool) {
	switch value := input.(type) {
	case []any:
		inputs := make([]string, 0, len(value))
		for _, item := range value {
			inputs = append(inputs, embeddingInputString(item))
		}
		return inputs, true
	case []string:
		return append([]string{}, value...), true
	default:
		return []string{embeddingInputString(input)}, false
	}
}

func embeddingInputString(input any) string {
	if value, ok := input.(string); ok {
		return value
	}
	document, errorValue := json.Marshal(input)
	if errorValue != nil {
		return fmt.Sprint(input)
	}
	return string(document)
}

func prepareEmbeddingInputs(inputs []string, request EmbeddingRequest, modelName string, isBatch bool) []string {
	if !strings.EqualFold(strings.TrimSpace(modelName), EmbeddingGemmaModelName) {
		return inputs
	}
	promptedInputs := make([]string, 0, len(inputs))
	for _, input := range inputs {
		promptedInputs = append(promptedInputs, applyEmbeddingGemmaPrompt(input, request, isBatch))
	}
	return promptedInputs
}

func applyEmbeddingGemmaPrompt(input string, request EmbeddingRequest, isBatch bool) string {
	trimmedInput := strings.TrimSpace(input)
	if hasEmbeddingGemmaPrompt(trimmedInput) {
		return trimmedInput
	}
	if embeddingInputType(request, isBatch) == "document" {
		title := firstNonEmpty(request.Title, "none")
		return "title: " + title + " | text: " + trimmedInput
	}
	return "task: " + embeddingTaskDescription(request.Task) + " | query: " + trimmedInput
}

func hasEmbeddingGemmaPrompt(input string) bool {
	normalized := strings.ToLower(strings.TrimSpace(input))
	return strings.HasPrefix(normalized, "task: ") || strings.HasPrefix(normalized, "title: ")
}

func embeddingInputType(request EmbeddingRequest, isBatch bool) string {
	normalized := strings.ToLower(strings.TrimSpace(request.InputType))
	switch normalized {
	case "document", "doc":
		return "document"
	case "query":
		return "query"
	default:
		if isBatch {
			return "document"
		}
		return "query"
	}
}

func embeddingTaskDescription(task string) string {
	switch strings.ToLower(strings.TrimSpace(task)) {
	case "", "retrieval", "search":
		return "search result"
	case "question_answering", "question-answering", "question answering", "qa":
		return "question answering"
	case "fact_checking", "fact-checking", "fact checking", "fact_verification", "fact verification":
		return "fact checking"
	case "classification":
		return "classification"
	case "clustering":
		return "clustering"
	case "semantic_similarity", "semantic-similarity", "sentence_similarity", "sentence similarity", "sts":
		return "sentence similarity"
	case "code_retrieval", "code-retrieval", "code retrieval":
		return "code retrieval"
	default:
		return strings.TrimSpace(task)
	}
}

func finalizeEmbeddingResponse(response EmbeddingResponse, request EmbeddingRequest) EmbeddingResponse {
	if request.OutputDimensions <= 0 {
		return response
	}
	if len(response.Embedding) > 0 {
		response.Embedding = truncateAndNormalizeEmbedding(response.Embedding, request.OutputDimensions)
	}
	if len(response.Embeddings) > 0 {
		embeddings := make([][]float64, 0, len(response.Embeddings))
		for _, embedding := range response.Embeddings {
			embeddings = append(embeddings, truncateAndNormalizeEmbedding(embedding, request.OutputDimensions))
		}
		response.Embeddings = embeddings
	}
	return response
}

func truncateAndNormalizeEmbedding(embedding []float64, dimensions int) []float64 {
	if dimensions <= 0 || dimensions > len(embedding) {
		dimensions = len(embedding)
	}
	truncatedEmbedding := append([]float64{}, embedding[:dimensions]...)
	var squaredSum float64
	for _, value := range truncatedEmbedding {
		squaredSum += value * value
	}
	if squaredSum == 0 {
		return truncatedEmbedding
	}
	scale := math.Sqrt(squaredSum)
	for index, value := range truncatedEmbedding {
		truncatedEmbedding[index] = value / scale
	}
	return truncatedEmbedding
}
