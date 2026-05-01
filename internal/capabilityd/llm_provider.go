package capabilityd

import (
	"context"
	"errors"
	"strings"

	"github.com/anthropic-lab/internkim/internal/llmbackend"
)

type (
	LLMMessage             = llmbackend.Message
	StructuredOutputSchema = llmbackend.StructuredOutputSchema
	StructuredLLMRequest   = llmbackend.StructuredRequest
	TextLLMRequest         = llmbackend.TextRequest
	LLMResponse            = llmbackend.Response

	LLMProvider           = llmbackend.Provider
	StructuredLLMProvider = llmbackend.StructuredCompleter
	TextLLMProvider       = llmbackend.TextCompleter

	LiteRTBackend     = llmbackend.LiteRTBackend
	OllamaBackend     = llmbackend.OllamaBackend
	OpenRouterBackend = llmbackend.OpenRouterBackend
	LlamaCppBackend   = llmbackend.LlamaCppBackend
	MLXBackend        = llmbackend.MLXBackend

	AutoProvider = llmbackend.AutoProvider
)

func (service Service) completeStructured(ctx context.Context, request StructuredLLMRequest) (LLMResponse, error) {
	provider, errorValue := service.providerForExecutionMode(request.ExecutionMode, request.Backend)
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	return provider.CompleteStructured(ctx, request)
}

func (service Service) completeText(ctx context.Context, request TextLLMRequest) (LLMResponse, error) {
	provider, errorValue := service.providerForExecutionMode(request.ExecutionMode, request.Backend)
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	return provider.CompleteText(ctx, request)
}

func (service Service) providerForExecutionMode(executionMode, preferredBackend string) (LLMProvider, error) {
	companionProvider := service.companionProvider()
	remoteProvider := service.openRouterBackend()
	localProviderChain := AutoProvider{
		Providers:      service.localBackends(preferredBackend),
		AttemptTimeout: service.Configuration.ProviderAttemptTimeout,
	}
	switch strings.ToLower(firstNonEmpty(executionMode, "auto")) {
	case "local":
		return localProviderChain, nil
	case "companion", "user_desktop":
		return companionProvider, nil
	case "remote":
		if service.Configuration.LocalOnly {
			return nil, errors.New("remote llm execution is disabled by local-only mode")
		}
		return remoteProvider, nil
	case "auto":
		return AutoProvider{
			Providers:      service.automaticLLMProviders(localProviderChain, companionProvider, remoteProvider),
			AttemptTimeout: service.Configuration.ProviderAttemptTimeout,
		}, nil
	default:
		return nil, errors.New("llm execution mode is not supported")
	}
}

func (service Service) localBackends(preferredBackend string) []LLMProvider {
	order := service.localBackendOrder()
	backends := make([]LLMProvider, 0)
	for _, name := range order {
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "litert":
			for _, variant := range litertVariantsFor(preferredBackend) {
				backends = append(backends, LiteRTBackend{
					ModelPath:   firstNonEmpty(service.Configuration.LiteRTModelPath, DefaultConfiguration().LiteRTModelPath),
					WrapperPath: firstNonEmpty(service.Configuration.LiteRTWrapperPath, DefaultConfiguration().LiteRTWrapperPath),
					Variant:     variant,
					RunCommand:  service.runCommand,
				})
			}
		case "ollama":
			backends = append(backends, service.ollamaBackend())
		}
	}
	return backends
}

func (service Service) localBackendOrder() []string {
	order := service.Configuration.LocalBackendOrder
	if len(order) == 0 {
		order = []string{"litert"}
	}
	if !service.Configuration.EnableOllamaFallback {
		return order
	}
	for _, name := range order {
		if strings.EqualFold(strings.TrimSpace(name), "ollama") {
			return order
		}
	}
	return append(order, "ollama")
}

func (service Service) ollamaBackend() OllamaBackend {
	return OllamaBackend{
		BaseURL:    firstNonEmpty(service.Configuration.OllamaBaseURL, DefaultConfiguration().OllamaBaseURL),
		ModelName:  firstNonEmpty(service.Configuration.OllamaModel, DefaultConfiguration().OllamaModel),
		HTTPClient: service.httpClient(),
	}
}

func (service Service) openRouterBackend() OpenRouterBackend {
	return OpenRouterBackend{
		KeyPath:    service.Configuration.OpenRouterKeyPath,
		BaseURL:    service.Configuration.OpenRouterBaseURL,
		ModelName:  firstNonEmpty(service.Configuration.OpenRouterModel, DefaultConfiguration().OpenRouterModel),
		HTTPClient: service.httpClient(),
	}
}

func (service Service) automaticLLMProviders(localProvider LLMProvider, companionProvider LLMProvider, remoteProvider LLMProvider) []LLMProvider {
	if service.Configuration.LocalOnly {
		return []LLMProvider{companionProvider, localProvider}
	}
	if service.Configuration.PreferCompanionLLM {
		return []LLMProvider{companionProvider, remoteProvider, localProvider}
	}
	return []LLMProvider{remoteProvider, companionProvider, localProvider}
}

func litertVariantsFor(preferredBackend string) []string {
	switch strings.ToLower(strings.TrimSpace(preferredBackend)) {
	case "gpu":
		return []string{"gpu"}
	case "cpu":
		return []string{"cpu"}
	default:
		return []string{"gpu", "cpu"}
	}
}
