package llmbackend

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/modelladder"
)

const (
	embeddingGemmaModelNamePrefix = "embeddinggemma"
	DefaultEmbeddingModelName     = modelladder.EmbeddingModel
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

func prepareEmbeddingInputs(inputs []string, request EmbeddingRequest, modelName string) []string {
	inputType := embeddingInputType(request)
	if !isEmbeddingGemmaModel(modelName) || inputType == "" {
		return inputs
	}
	return mapEmbeddingInputs(inputs, func(input string) string {
		return applyEmbeddingGemmaPrompt(input, request, inputType)
	})
}

func isEmbeddingGemmaModel(modelName string) bool {
	lastSegment := modelName[strings.LastIndex(modelName, "/")+1:]
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(lastSegment)), embeddingGemmaModelNamePrefix)
}

func mapEmbeddingInputs(inputs []string, transform func(string) string) []string {
	transformedInputs := make([]string, 0, len(inputs))
	for _, input := range inputs {
		transformedInputs = append(transformedInputs, transform(input))
	}
	return transformedInputs
}

func applyEmbeddingGemmaPrompt(input string, request EmbeddingRequest, inputType string) string {
	trimmedInput := strings.TrimSpace(input)
	if hasEmbeddingGemmaPrompt(trimmedInput) {
		return trimmedInput
	}
	if inputType == "document" {
		title := firstNonEmpty(request.Title, "none")
		return "title: " + title + " | text: " + trimmedInput
	}
	return "task: " + embeddingTaskDescription(request.Task) + " | query: " + trimmedInput
}

func hasEmbeddingGemmaPrompt(input string) bool {
	normalized := strings.ToLower(strings.TrimSpace(input))
	return strings.HasPrefix(normalized, "task: ") || strings.HasPrefix(normalized, "title: ")
}

func embeddingInputType(request EmbeddingRequest) string {
	switch strings.ToLower(strings.TrimSpace(request.InputType)) {
	case "document", "doc":
		return "document"
	case "query":
		return "query"
	default:
		return ""
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

func finalizeEmbeddingResponse(response EmbeddingResponse, request EmbeddingRequest) (EmbeddingResponse, error) {
	if request.OutputDimensions <= 0 {
		return response, nil
	}
	if shortest := shortestEmbeddingLength(response); shortest < request.OutputDimensions {
		return EmbeddingResponse{}, fmt.Errorf("%s answered %s with %d-dimensional embeddings; %d were requested", response.Provider, response.Model, shortest, request.OutputDimensions)
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
	return response, nil
}

func shortestEmbeddingLength(response EmbeddingResponse) int {
	shortest := len(response.Embedding)
	for _, embedding := range response.Embeddings {
		if shortest == 0 || len(embedding) < shortest {
			shortest = len(embedding)
		}
	}
	return shortest
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
