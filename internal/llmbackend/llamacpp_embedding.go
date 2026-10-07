package llmbackend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type LlamaCppEmbeddingBackend struct {
	BaseURL    string
	ModelName  string
	HTTPClient *http.Client
}

type llamaCppEmbeddingRequest struct {
	Model          string `json:"model"`
	Input          any    `json:"input"`
	EncodingFormat string `json:"encoding_format"`
	Dimensions     int    `json:"dimensions,omitempty"`
}

type llamaCppEmbeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
}

func (backend LlamaCppEmbeddingBackend) CreateEmbedding(ctx context.Context, request EmbeddingRequest) (EmbeddingResponse, error) {
	inputs, isBatch := normalizeEmbeddingInputs(request.Input)
	modelName, errorValue := backend.resolveModelName(request.Model)
	if errorValue != nil {
		return EmbeddingResponse{}, errorValue
	}
	embeddings, errorValue := backend.send(ctx, modelName, request.OutputDimensions, prepareEmbeddingInputs(inputs, request, modelName, isBatch))
	if errorValue != nil {
		return EmbeddingResponse{}, errorValue
	}
	response := EmbeddingResponse{Provider: "llamacpp", Model: modelName, SelectedBackend: "llamacpp"}
	if isBatch {
		response.Embeddings = embeddings
	} else {
		response.Embedding = embeddings[0]
	}
	return finalizeEmbeddingResponse(response, request)
}

func (backend LlamaCppEmbeddingBackend) resolveModelName(requestedModel string) (string, error) {
	servedModel := firstNonEmpty(backend.ModelName, DefaultEmbeddingModelName)
	normalized := strings.TrimSpace(requestedModel)
	if normalized == "" || strings.EqualFold(normalized, "default") || normalized == servedModel {
		return servedModel, nil
	}
	return "", fmt.Errorf("this host embeds with %s only, and %s was requested; vectors from two models are not comparable", servedModel, normalized)
}

func (backend LlamaCppEmbeddingBackend) send(ctx context.Context, modelName string, dimensions int, inputs []string) ([][]float64, error) {
	requestDocument, errorValue := json.Marshal(llamaCppEmbeddingRequest{
		Model:          modelName,
		Input:          embeddingRequestInput(inputs),
		EncodingFormat: "float",
		Dimensions:     dimensions,
	})
	if errorValue != nil {
		return nil, errorValue
	}
	var lastError error
	for attempt := 0; attempt < openAIMaxRetryAttempts; attempt++ {
		if attempt > 0 {
			if errorValue := sleepWithContext(ctx, openAIRetryBackoff(attempt)); errorValue != nil {
				return nil, errorValue
			}
		}
		embeddings, errorValue := backend.sendOnce(ctx, requestDocument)
		if errorValue == nil {
			return embeddings, nil
		}
		var unavailable unavailableEmbeddingServerError
		if !errors.As(errorValue, &unavailable) {
			return nil, errorValue
		}
		lastError = errorValue
	}
	return nil, lastError
}

type unavailableEmbeddingServerError struct{ cause error }

func (unavailable unavailableEmbeddingServerError) Error() string { return unavailable.cause.Error() }

func (unavailable unavailableEmbeddingServerError) Unwrap() error { return unavailable.cause }

func (backend LlamaCppEmbeddingBackend) sendOnce(ctx context.Context, requestDocument []byte) ([][]float64, error) {
	endpoint := strings.TrimRight(backend.BaseURL, "/") + "/v1/embeddings"
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestDocument))
	if errorValue != nil {
		return nil, errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, errorValue := backend.client().Do(httpRequest)
	if errorValue != nil {
		return nil, errorValue
	}
	defer httpResponse.Body.Close()

	responseDocument, errorValue := io.ReadAll(httpResponse.Body)
	if errorValue != nil {
		return nil, errors.New("read llama.cpp embedding response: " + errorValue.Error())
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		providerError := normalizeProviderError("llamacpp", httpResponse.StatusCode, responseDocument)
		if isRetryableUpstreamStatus(httpResponse.StatusCode) {
			return nil, unavailableEmbeddingServerError{cause: providerError}
		}
		return nil, providerError
	}
	return parseLlamaCppEmbeddings(responseDocument)
}

func parseLlamaCppEmbeddings(responseDocument []byte) ([][]float64, error) {
	var parsedResponse llamaCppEmbeddingResponse
	if errorValue := json.Unmarshal(responseDocument, &parsedResponse); errorValue != nil {
		return nil, errorValue
	}
	if len(parsedResponse.Data) == 0 {
		return nil, errors.New("llama.cpp embedding response did not include data")
	}
	embeddings := make([][]float64, 0, len(parsedResponse.Data))
	for _, item := range parsedResponse.Data {
		embeddings = append(embeddings, item.Embedding)
	}
	return embeddings, nil
}

func (backend LlamaCppEmbeddingBackend) client() *http.Client {
	if backend.HTTPClient == nil {
		return http.DefaultClient
	}
	return backend.HTTPClient
}

func embeddingRequestInput(inputs []string) any {
	if len(inputs) == 1 {
		return inputs[0]
	}
	return inputs
}
