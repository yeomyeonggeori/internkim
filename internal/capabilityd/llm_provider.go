package capabilityd

import (
	"context"
	"errors"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/llmbackend"
	"gitlab.com/eastriver/internkim/internal/runtime/locallm"
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

	LiteRTProvider    = llmbackend.LiteRTProvider
	OllamaBackend     = llmbackend.OllamaBackend
	OpenRouterBackend = llmbackend.OpenRouterBackend
	LlamaCppBackend   = llmbackend.LlamaCppBackend
	MLXBackend        = llmbackend.MLXBackend

	AutoProvider = llmbackend.AutoProvider
)

func (service Service) completeStructured(ctx context.Context, request StructuredLLMRequest) (LLMResponse, error) {
	provider, errorValue := service.providerForExecutionMode(request.ExecutionMode, request.Provider, request.Accelerator)
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	return provider.CompleteStructured(ctx, request)
}

func (service Service) completeText(ctx context.Context, request TextLLMRequest) (LLMResponse, error) {
	provider, errorValue := service.providerForExecutionMode(request.ExecutionMode, request.Provider, request.Accelerator)
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	return provider.CompleteText(ctx, request)
}

func (service Service) providerForExecutionMode(executionMode, providerName, accelerator string) (LLMProvider, error) {
	companionProvider := service.companionProvider()
	remoteProvider := service.openRouterBackend()
	localProviderSet := service.localProviderSet(providerName, accelerator, false)
	switch strings.ToLower(firstNonEmpty(executionMode, "auto")) {
	case "device":
		return localProviderSet.Provider, nil
	case "companion":
		return companionProvider, nil
	case "remote":
		if service.Configuration.LocalOnly {
			return nil, errors.New("remote llm execution is disabled by local-only mode")
		}
		return remoteProvider, nil
	case "auto":
		localProviderSet := service.localProviderSet(providerName, accelerator, true)
		return AutoProvider{
			Providers:               service.automaticLLMProviders(localProviderSet.Provider, companionProvider, remoteProvider),
			AttemptTimeout:          service.Configuration.ProviderAttemptTimeout,
			AllowStructuredFallback: true,
		}, nil
	default:
		return nil, errors.New("llm execution mode is not supported")
	}
}

func (service Service) localProviderSet(providerName, accelerator string, allowStructuredFallback bool) llmbackend.LocalProviderSet {
	defaultConfiguration := DefaultConfiguration()
	return llmbackend.BuildLocalProviderSet(llmbackend.LocalProviderConfig{
		ProviderOrder:           firstProviderOrder(service.Configuration.LocalBackendOrder, llmbackend.DefaultDeviceLocalProviderOrder),
		ProviderName:            providerName,
		Accelerator:             accelerator,
		AttemptTimeout:          service.Configuration.ProviderAttemptTimeout,
		AllowStructuredFallback: allowStructuredFallback,
		HTTPClient:              service.httpClient(),
		RunCommand:              service.runCommand,
		LlamaCppServiceName:     locallm.LlamaCppServiceName,
		LlamaCppStartTimeout:    30 * time.Second,
		LlamaCppPollInterval:    500 * time.Millisecond,
		LiteRTModelPath:         firstNonEmpty(service.Configuration.LiteRTModelPath, defaultConfiguration.LiteRTModelPath),
		LiteRTRunnerPath:        firstNonEmpty(service.Configuration.LocalLLMRunnerPath, defaultConfiguration.LocalLLMRunnerPath),
		OllamaBaseURL:           firstNonEmpty(service.Configuration.OllamaBaseURL, defaultConfiguration.OllamaBaseURL),
		OllamaModel:             firstNonEmpty(service.Configuration.OllamaModel, defaultConfiguration.OllamaModel),
		LlamaCppBaseURL:         firstNonEmpty(service.Configuration.LlamaCppBaseURL, defaultConfiguration.LlamaCppBaseURL),
		LlamaCppModel:           firstNonEmpty(service.Configuration.LlamaCppModel, defaultConfiguration.LlamaCppModel),
	})
}

func (service Service) openRouterBackend() OpenRouterBackend {
	return OpenRouterBackend{
		KeyPath:    service.Configuration.OpenRouterKeyPath,
		BaseURL:    service.Configuration.OpenRouterBaseURL,
		ModelName:  firstNonEmpty(service.Configuration.OpenRouterModel, DefaultConfiguration().OpenRouterModel),
		HTTPClient: service.httpClient(),
	}
}

func firstProviderOrder(values []string, fallback []string) []string {
	if len(values) > 0 {
		return append([]string{}, values...)
	}
	return append([]string{}, fallback...)
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
