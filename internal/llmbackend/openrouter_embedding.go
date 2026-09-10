package llmbackend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	var lastError error
	for attempt := 0; attempt < openAIMaxRetryAttempts; attempt++ {
		if attempt > 0 {
			if errorValue := sleepWithContext(ctx, embeddingRetryDelay(attempt, lastError)); errorValue != nil {
				return EmbeddingResponse{}, errorValue
			}
		}
		response, errorValue := backend.sendOnce(ctx, apiKey, requestDocument)
		if errorValue == nil {
			return response, nil
		}
		var throttled retryableEmbeddingError
		if !errors.As(errorValue, &throttled) {
			return EmbeddingResponse{}, errorValue
		}
		lastError = errorValue
	}
	return EmbeddingResponse{}, lastError
}

type retryableEmbeddingError struct {
	cause      error
	retryAfter time.Duration
}

func (retryable retryableEmbeddingError) Error() string { return retryable.cause.Error() }

func (retryable retryableEmbeddingError) Unwrap() error { return retryable.cause }

func embeddingRetryDelay(attempt int, lastError error) time.Duration {
	var throttled retryableEmbeddingError
	if errors.As(lastError, &throttled) && throttled.retryAfter > 0 {
		return min(throttled.retryAfter, maxEmbeddingRetryAfter)
	}
	return openAIRetryBackoff(attempt)
}

const maxEmbeddingRetryAfter = 30 * time.Second

func parseRetryAfterHeader(value string) time.Duration {
	seconds, errorValue := strconv.Atoi(strings.TrimSpace(value))
	if errorValue != nil || seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func (backend OpenRouterEmbeddingBackend) sendOnce(ctx context.Context, apiKey string, requestDocument []byte) (EmbeddingResponse, error) {
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, backend.BaseURL, bytes.NewReader(requestDocument))
	if errorValue != nil {
		return EmbeddingResponse{}, errorValue
	}
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	if gatewaySecret := readOpenRouterKey(backend.GatewaySecretPath); strings.TrimSpace(gatewaySecret) != "" {
		httpRequest.Header.Set(firstNonEmpty(backend.GatewaySecretHeader, "X-INTERNKIM-GATEWAY-SECRET"), strings.TrimSpace(gatewaySecret))
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
		providerError := normalizeProviderError("openrouter", httpResponse.StatusCode, responseDocument)
		if isRetryableUpstreamStatus(httpResponse.StatusCode) {
			return EmbeddingResponse{}, retryableEmbeddingError{cause: providerError, retryAfter: parseRetryAfterHeader(httpResponse.Header.Get("Retry-After"))}
		}
		return EmbeddingResponse{}, providerError
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
