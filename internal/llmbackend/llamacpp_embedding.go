package llmbackend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
}

type llamaCppEmbeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
}

func (backend LlamaCppEmbeddingBackend) Name() string { return "llamacpp" }

func (backend LlamaCppEmbeddingBackend) Ping(ctx context.Context) error {
	return openAICompatClient{
		BaseURL:    backend.BaseURL,
		ModelName:  backend.ModelName,
		HTTPClient: backend.HTTPClient,
	}.pingPath(ctx, "/health")
}

func (backend LlamaCppEmbeddingBackend) CreateEmbedding(ctx context.Context, request EmbeddingRequest) (EmbeddingResponse, error) {
	inputs, isBatch := normalizeEmbeddingInputs(request.Input)
	modelName := firstNonEmpty(request.Model, backend.ModelName, DefaultEmbeddingModelName)
	response, errorValue := backend.create(ctx, modelName, prepareEmbeddingInputs(inputs, request, modelName, isBatch))
	if errorValue != nil {
		return EmbeddingResponse{}, errorValue
	}
	response.Provider = "llamacpp"
	response.Model = firstNonEmpty(response.Model, modelName)
	response.SelectedBackend = "llamacpp"
	if !isBatch {
		if len(response.Embeddings) == 0 {
			return EmbeddingResponse{}, errors.New("llama.cpp embedding response did not include data")
		}
		response.Embedding = response.Embeddings[0]
		response.Embeddings = nil
	}
	return finalizeEmbeddingResponse(response, request), nil
}

func (backend LlamaCppEmbeddingBackend) create(ctx context.Context, modelName string, inputs []string) (EmbeddingResponse, error) {
	requestDocument, errorValue := json.Marshal(llamaCppEmbeddingRequest{
		Model:          modelName,
		Input:          embeddingRequestInput(inputs),
		EncodingFormat: "float",
	})
	if errorValue != nil {
		return EmbeddingResponse{}, errorValue
	}
	endpoint := strings.TrimRight(backend.BaseURL, "/") + "/v1/embeddings"
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestDocument))
	if errorValue != nil {
		return EmbeddingResponse{}, errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, errorValue := backend.client().Do(httpRequest)
	if errorValue != nil {
		return EmbeddingResponse{}, errorValue
	}
	defer httpResponse.Body.Close()

	responseDocument, errorValue := io.ReadAll(httpResponse.Body)
	if errorValue != nil {
		return EmbeddingResponse{}, errors.New("read llama.cpp embedding response: " + errorValue.Error())
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return EmbeddingResponse{}, normalizeProviderError("llamacpp", httpResponse.StatusCode, responseDocument)
	}

	var parsedResponse llamaCppEmbeddingResponse
	if errorValue := json.Unmarshal(responseDocument, &parsedResponse); errorValue != nil {
		return EmbeddingResponse{}, errorValue
	}
	if len(parsedResponse.Data) == 0 {
		return EmbeddingResponse{}, errors.New("llama.cpp embedding response did not include data")
	}
	embeddings := make([][]float64, 0, len(parsedResponse.Data))
	for _, item := range parsedResponse.Data {
		embeddings = append(embeddings, item.Embedding)
	}
	return EmbeddingResponse{
		Model:      parsedResponse.Model,
		Embeddings: embeddings,
	}, nil
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
