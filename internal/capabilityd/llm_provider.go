package capabilityd

import (
	"context"
	"errors"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/llmbackend"
	"github.com/yeomyeonggeori/internkim/internal/modelladder"
)

type (
	LLMMessage             = llmbackend.Message
	ChatLLMMessage         = llmbackend.ChatMessage
	StructuredOutputSchema = llmbackend.StructuredOutputSchema
	StructuredLLMRequest   = llmbackend.StructuredRequest
	TextLLMRequest         = llmbackend.TextRequest
	ChatLLMRequest         = llmbackend.ChatRequest
	LLMResponse            = llmbackend.Response
	ChatLLMResponse        = llmbackend.ChatResponse

	LLMProvider           = llmbackend.Provider
	StructuredLLMProvider = llmbackend.StructuredCompleter
	TextLLMProvider       = llmbackend.TextCompleter
	ChatLLMProvider       = llmbackend.ChatCompleter

	OpenRouterBackend = llmbackend.OpenRouterBackend
	AutoProvider      = llmbackend.AutoProvider
)

type providerAvailability struct {
	Configured bool   `json:"configured"`
	Available  bool   `json:"available"`
	Reason     string `json:"reason,omitempty"`
}

func (service Service) completeStructured(ctx context.Context, request StructuredLLMRequest) (LLMResponse, error) {
	request.Model = service.llmRequestModel(request.Model)
	request.ReasoningEffort = reasoningEffortForTier(request.ReasoningEffort, request.ModelTier)
	provider, errorValue := service.providerForExecutionMode(request.ExecutionMode)
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	return provider.CompleteStructured(ctx, request)
}

func (service Service) completeText(ctx context.Context, request TextLLMRequest) (LLMResponse, error) {
	request.Model = service.llmRequestModel(request.Model)
	request.ReasoningEffort = reasoningEffortForTier(request.ReasoningEffort, request.ModelTier)
	provider, errorValue := service.providerForExecutionMode(request.ExecutionMode)
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	return provider.CompleteText(ctx, request)
}

func (service Service) completeChat(ctx context.Context, request ChatLLMRequest) (ChatLLMResponse, error) {
	request.Model = service.llmRequestModel(request.Model)
	request.ReasoningEffort = reasoningEffortForTier(request.ReasoningEffort, request.ModelTier)
	provider, errorValue := service.providerForExecutionMode(request.ExecutionMode)
	if errorValue != nil {
		return ChatLLMResponse{}, errorValue
	}
	chatProvider, isChatProvider := provider.(ChatLLMProvider)
	if !isChatProvider {
		return ChatLLMResponse{}, errors.New("selected llm provider does not support native chat completions")
	}
	return chatProvider.CompleteChat(ctx, request)
}

func reasoningEffortForTier(requestedEffort string, modelTier string) string {
	if requested := strings.TrimSpace(requestedEffort); requested != "" {
		return requested
	}
	return modelladder.ReasoningEffort(modelTier)
}

func (service Service) llmRequestModel(requestModel string) string {
	if !service.Configuration.ForceOpenRouterModel {
		return requestModel
	}
	return firstNonEmpty(service.Configuration.OpenRouterModel, requestModel)
}

func (service Service) providerForExecutionMode(executionMode string) (LLMProvider, error) {
	switch strings.ToLower(firstNonEmpty(executionMode, "auto")) {
	case "device":
		return nil, errors.New("device llm execution is not supported: there is no local model")
	case "remote":
		if service.Configuration.LocalOnly {
			return nil, errors.New("remote llm execution is disabled by local-only mode")
		}
		return service.openRouterBackend(), nil
	case "auto":
		if service.Configuration.LocalOnly {
			return nil, errors.New("local-only mode has no local model to run")
		}
		if service.Configuration.ForceOpenRouterModel {
			return service.openRouterBackend(), nil
		}
		return llmbackend.AutoProvider{
			Providers:               []LLMProvider{service.openRouterBackend()},
			AttemptTimeout:          service.Configuration.ProviderAttemptTimeout,
			AllowStructuredFallback: true,
		}, nil
	default:
		return nil, errors.New("llm execution mode is not supported")
	}
}

func (service Service) openRouterBackend() OpenRouterBackend {
	return OpenRouterBackend{
		KeyPath:             service.Configuration.OpenRouterKeyPath,
		BaseURL:             service.Configuration.OpenRouterBaseURL,
		ModelName:           firstNonEmpty(service.Configuration.OpenRouterModel, DefaultConfiguration().OpenRouterModel),
		FallbackModelNames:  service.openRouterActionFallbackModelNames(),
		GatewaySecretPath:   service.Configuration.OpenRouterGatewaySecretPath,
		GatewaySecretHeader: service.Configuration.OpenRouterGatewaySecretHeader,
		ProviderSort:        modelladder.ProviderSort,
		HTTPClient:          service.providerHTTPClient(),
	}
}

func (service Service) openRouterActionFallbackModelNames() []string {
	if service.Configuration.ForceOpenRouterModel {
		return nil
	}
	return append([]string{}, llmbackend.DefaultOpenRouterActionFallbackModels...)
}

func (service Service) providerHealth(ctx context.Context) map[string]providerAvailability {
	return map[string]providerAvailability{
		"chatd": service.chatdProviderHealth(ctx),
	}
}
