package capabilityd

import (
	"context"
	"errors"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/internal/llmbackend"
)

type (
	EmbeddingRequest  = llmbackend.EmbeddingRequest
	EmbeddingResponse = llmbackend.EmbeddingResponse
	EmbeddingProvider = llmbackend.EmbeddingProvider
)

func (service Service) createEmbedding(ctx context.Context, request EmbeddingRequest) (EmbeddingResponse, error) {
	provider, errorValue := service.embeddingProviderForExecutionMode(request.ExecutionMode, request.Provider)
	if errorValue != nil {
		return EmbeddingResponse{}, errorValue
	}
	return provider.CreateEmbedding(ctx, request)
}

func (service Service) embeddingProviderForExecutionMode(executionMode, providerName string) (EmbeddingProvider, error) {
	companionProvider := service.companionInferenceProvider()
	remoteProvider := service.openRouterEmbeddingBackend()
	localProviderSet := service.localEmbeddingProviderSet(providerName)
	switch strings.ToLower(firstNonEmpty(executionMode, capabilities.ExecutionModeAuto)) {
	case capabilities.ExecutionModeDevice:
		return localProviderSet.Provider, nil
	case capabilities.ExecutionModeCompanion:
		return companionProvider, nil
	case capabilities.ExecutionModeRemote:
		if service.Configuration.LocalOnly {
			return nil, errors.New("remote embedding execution is disabled by local-only mode")
		}
		return remoteProvider, nil
	case capabilities.ExecutionModeAuto:
		return llmbackend.AutoEmbeddingProvider{
			Providers:      service.automaticEmbeddingProviders(localProviderSet.Provider, companionProvider, remoteProvider),
			AttemptTimeout: service.Configuration.ProviderAttemptTimeout,
		}, nil
	default:
		return nil, errors.New("embedding execution mode is not supported")
	}
}

func (service Service) localEmbeddingProviderSet(providerName string) llmbackend.LocalEmbeddingProviderSet {
	defaultConfiguration := DefaultConfiguration()
	return llmbackend.BuildLocalEmbeddingProviderSet(llmbackend.LocalEmbeddingProviderConfig{
		ProviderOrder:   firstProviderOrder(service.Configuration.EmbeddingProviderOrder, llmbackend.DefaultLocalEmbeddingProviderOrder),
		ProviderName:    providerName,
		AttemptTimeout:  service.Configuration.ProviderAttemptTimeout,
		HTTPClient:      service.providerHTTPClient(),
		LlamaCppBaseURL: firstNonEmpty(service.Configuration.LlamaCppEmbeddingBaseURL, defaultConfiguration.LlamaCppEmbeddingBaseURL),
		LlamaCppModel:   firstNonEmpty(service.Configuration.LlamaCppEmbeddingModel, defaultConfiguration.LlamaCppEmbeddingModel),
	})
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

func (service Service) automaticEmbeddingProviders(localProvider EmbeddingProvider, companionProvider EmbeddingProvider, remoteProvider EmbeddingProvider) []EmbeddingProvider {
	switch service.localInferenceMode() {
	case "device":
		return service.localFirstEmbeddingProviders(localProvider, remoteProvider)
	case "companion_preferred":
		return []EmbeddingProvider{companionProvider}
	case "companion_only":
		return []EmbeddingProvider{companionProvider}
	case "remote":
		if service.Configuration.LocalOnly {
			return []EmbeddingProvider{localProvider}
		}
		return []EmbeddingProvider{remoteProvider}
	}
	return service.localFirstEmbeddingProviders(localProvider, remoteProvider)
}

func (service Service) localFirstEmbeddingProviders(localProvider EmbeddingProvider, remoteProvider EmbeddingProvider) []EmbeddingProvider {
	if service.Configuration.LocalOnly || !service.hasCompatibleRemoteEmbeddingModel() {
		return []EmbeddingProvider{localProvider}
	}
	return []EmbeddingProvider{localProvider, remoteProvider}
}

func (service Service) hasCompatibleRemoteEmbeddingModel() bool {
	defaultConfiguration := DefaultConfiguration()
	localModelName := firstNonEmpty(service.Configuration.LlamaCppEmbeddingModel, defaultConfiguration.LlamaCppEmbeddingModel)
	remoteModelName := firstNonEmpty(service.Configuration.OpenRouterEmbeddingModel, defaultConfiguration.OpenRouterEmbeddingModel)
	return strings.EqualFold(strings.TrimSpace(localModelName), strings.TrimSpace(remoteModelName))
}
