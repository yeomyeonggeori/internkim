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
	provider, errorValue := service.embeddingProviderForExecutionMode(request.ExecutionMode)
	if errorValue != nil {
		return EmbeddingResponse{}, errorValue
	}
	return provider.CreateEmbedding(ctx, request)
}

func (service Service) embeddingProviderForExecutionMode(executionMode string) (EmbeddingProvider, error) {
	switch strings.ToLower(firstNonEmpty(executionMode, capabilities.ExecutionModeAuto)) {
	case capabilities.ExecutionModeDevice:
		return nil, errors.New("device embedding execution is not supported: there is no local model")
	case capabilities.ExecutionModeRemote:
		if service.Configuration.LocalOnly {
			return nil, errors.New("remote embedding execution is disabled by local-only mode")
		}
		return service.openRouterEmbeddingBackend(), nil
	case capabilities.ExecutionModeAuto:
		if service.Configuration.LocalOnly {
			return nil, errors.New("local-only mode has no local model to run")
		}
		return llmbackend.AutoEmbeddingProvider{
			Providers:      []EmbeddingProvider{service.openRouterEmbeddingBackend()},
			AttemptTimeout: service.Configuration.ProviderAttemptTimeout,
		}, nil
	default:
		return nil, errors.New("embedding execution mode is not supported")
	}
}

func (service Service) openRouterEmbeddingBackend() llmbackend.OpenRouterEmbeddingBackend {
	return llmbackend.OpenRouterEmbeddingBackend{
		KeyPath:             service.Configuration.OpenRouterKeyPath,
		BaseURL:             service.Configuration.OpenRouterEmbeddingBaseURL,
		ModelName:           firstNonEmpty(service.Configuration.OpenRouterEmbeddingModel, DefaultConfiguration().OpenRouterEmbeddingModel),
		GatewaySecretPath:   service.Configuration.OpenRouterGatewaySecretPath,
		GatewaySecretHeader: service.Configuration.OpenRouterGatewaySecretHeader,
		HTTPClient:          service.providerHTTPClient(),
	}
}
