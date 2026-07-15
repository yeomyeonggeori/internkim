package llmbackend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type OpenRouterEmbeddingBackend struct {
	KeyPath             string
	BaseURL             string
	ModelName           string
	GatewaySecretPath   string
	GatewaySecretHeader string
	HTTPClient          *http.Client
}

type openRouterEmbeddingRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"`
}

type openRouterEmbeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
}

func (backend OpenRouterEmbeddingBackend) Name() string { return "openrouter" }

func (backend OpenRouterEmbeddingBackend) Ping(ctx context.Context) error {
	_ = ctx
	_, errorValue := backend.resolveAPIKey()
	return errorValue
}

func (backend OpenRouterEmbeddingBackend) CreateEmbedding(ctx context.Context, request EmbeddingRequest) (EmbeddingResponse, error) {
	apiKey, errorValue := backend.resolveAPIKey()
	if errorValue != nil {
		return EmbeddingResponse{}, errorValue
	}
	inputs, isBatch := normalizeEmbeddingInputs(request.Input)
	modelName := backend.resolveModelName(request.Model)
	requestDocument, errorValue := json.Marshal(openRouterEmbeddingRequest{
		Model: modelName,
		Input: embeddingRequestInput(prepareEmbeddingInputs(inputs, request, modelName, isBatch)),
	})
	if errorValue != nil {
		return EmbeddingResponse{}, errorValue
	}
	response, errorValue := backend.send(ctx, apiKey, requestDocument)
	if errorValue != nil {
		return EmbeddingResponse{}, errorValue
	}
	response.Provider = "openrouter"
	response.Model = modelName
	response.SelectedBackend = capabilities.LLMBackendRemote
	if !isBatch {
		if len(response.Embeddings) == 0 {
			return EmbeddingResponse{}, errors.New("openrouter embedding response did not include data")
		}
		response.Embedding = response.Embeddings[0]
		response.Embeddings = nil
	}
	return finalizeEmbeddingResponse(response, request), nil
}

func (backend OpenRouterEmbeddingBackend) resolveAPIKey() (string, error) {
	apiKey := readOpenRouterKey(backend.KeyPath)
	if apiKey == "" {
		return "", errors.New("openrouter api key is not configured")
	}
	if isPlaceholderOpenRouterKey(apiKey) {
		return "", errors.New("openrouter api key is a simulation placeholder; set OPENROUTER_API_KEY or rerun setup --only openrouter --force")
	}
	return apiKey, nil
}

func (backend OpenRouterEmbeddingBackend) resolveModelName(requestedModel string) string {
	normalized := strings.TrimSpace(requestedModel)
	if normalized == "" || strings.EqualFold(normalized, "default") || isLocalModelReference(normalized) {
		return strings.TrimSpace(backend.ModelName)
	}
	return normalized
}

func (backend OpenRouterEmbeddingBackend) send(ctx context.Context, apiKey string, requestDocument []byte) (EmbeddingResponse, error) {
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, backend.BaseURL, bytes.NewReader(requestDocument))
	if errorValue != nil {
		return EmbeddingResponse{}, errorValue
	}
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	if gatewaySecret := readOpenRouterKey(backend.GatewaySecretPath); strings.TrimSpace(gatewaySecret) != "" {
		httpRequest.Header.Set(firstNonEmpty(backend.GatewaySecretHeader, "X-InternKim-Gateway-Secret"), strings.TrimSpace(gatewaySecret))
	}

	httpResponse, errorValue := backend.client().Do(httpRequest)
	if errorValue != nil {
		return EmbeddingResponse{}, errorValue
	}
	defer httpResponse.Body.Close()

	responseDocument, errorValue := io.ReadAll(httpResponse.Body)
	if errorValue != nil {
		return EmbeddingResponse{}, errors.New("read openrouter embedding response: " + errorValue.Error())
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return EmbeddingResponse{}, normalizeProviderError("openrouter", httpResponse.StatusCode, responseDocument)
	}

	var parsedResponse openRouterEmbeddingResponse
	if errorValue := json.Unmarshal(responseDocument, &parsedResponse); errorValue != nil {
		return EmbeddingResponse{}, errorValue
	}
	if len(parsedResponse.Data) == 0 {
		return EmbeddingResponse{}, errors.New("openrouter embedding response did not include data")
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

func (backend OpenRouterEmbeddingBackend) client() *http.Client {
	if backend.HTTPClient == nil {
		return http.DefaultClient
	}
	return backend.HTTPClient
}
