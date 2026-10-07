package capabilityd

import (
	"context"
	"errors"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
	"github.com/yeomyeonggeori/internkim/internal/llmbackend"
)

type (
	EmbeddingRequest  = llmbackend.EmbeddingRequest
	EmbeddingResponse = llmbackend.EmbeddingResponse
	EmbeddingProvider = llmbackend.EmbeddingProvider
)

func (service Service) createEmbedding(ctx context.Context, request EmbeddingRequest) (EmbeddingResponse, error) {
	if errorValue := requireKnownEmbeddingExecutionMode(request.ExecutionMode); errorValue != nil {
		return EmbeddingResponse{}, errorValue
	}
	return service.localEmbeddingBackend().CreateEmbedding(ctx, request)
}

func requireKnownEmbeddingExecutionMode(executionMode string) error {
	switch strings.ToLower(firstNonEmpty(executionMode, capabilities.ExecutionModeAuto)) {
	case capabilities.ExecutionModeDevice, capabilities.ExecutionModeRemote, capabilities.ExecutionModeAuto:
		return nil
	default:
		return errors.New("embedding execution mode is not supported")
	}
}

func (service Service) localEmbeddingBackend() llmbackend.LlamaCppEmbeddingBackend {
	return llmbackend.LlamaCppEmbeddingBackend{
		BaseURL:    service.Configuration.WithDefaults().EmbeddingServerURL,
		ModelName:  llmbackend.DefaultEmbeddingModelName,
		HTTPClient: service.providerHTTPClient(),
	}
}
