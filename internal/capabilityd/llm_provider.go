package capabilityd

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/llmbackend"
	"github.com/yeomyeonggeori/internkim/internal/modelladder"
	"github.com/yeomyeonggeori/internkim/internal/runtime/locallm"
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

	LiteRTProvider    = llmbackend.LiteRTProvider
	OllamaBackend     = llmbackend.OllamaBackend
	OpenRouterBackend = llmbackend.OpenRouterBackend
	LlamaCppBackend   = llmbackend.LlamaCppBackend
	MLXBackend        = llmbackend.MLXBackend

	AutoProvider = llmbackend.AutoProvider
)

type providerAvailability struct {
	Configured bool   `json:"configured"`
	Available  bool   `json:"available"`
	Reason     string `json:"reason,omitempty"`
}

func (service Service) completeStructured(ctx context.Context, request StructuredLLMRequest) (LLMResponse, error) {
	request.Model = service.llmRequestModel(request.Model)
	request.ReasoningEffort = reasoningEffortForTier(request.ReasoningEffort, request.ModelTier)
	provider, errorValue := service.providerForExecutionMode(request.ExecutionMode, request.Provider, request.Accelerator)
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	return provider.CompleteStructured(ctx, request)
}

func (service Service) completeText(ctx context.Context, request TextLLMRequest) (LLMResponse, error) {
	request.Model = service.llmRequestModel(request.Model)
	request.ReasoningEffort = reasoningEffortForTier(request.ReasoningEffort, request.ModelTier)
	provider, errorValue := service.providerForExecutionMode(request.ExecutionMode, request.Provider, request.Accelerator)
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	return provider.CompleteText(ctx, request)
}

func (service Service) completeChat(ctx context.Context, request ChatLLMRequest) (ChatLLMResponse, error) {
	request.Model = service.llmRequestModel(request.Model)
	request.ReasoningEffort = reasoningEffortForTier(request.ReasoningEffort, request.ModelTier)
	provider, errorValue := service.providerForExecutionMode(request.ExecutionMode, request.Provider, request.Accelerator)
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

func (service Service) providerForExecutionMode(executionMode, providerName, accelerator string) (LLMProvider, error) {
	remoteProvider := service.openRouterBackend()
	localProviderSet := service.localProviderSet(providerName, accelerator, false)
	switch strings.ToLower(firstNonEmpty(executionMode, "auto")) {
	case "device":
		return localProviderSet.Provider, nil
	case "remote":
		if service.Configuration.LocalOnly {
			return nil, errors.New("remote llm execution is disabled by local-only mode")
		}
		return remoteProvider, nil
	case "auto":
		if service.Configuration.ForceOpenRouterModel {
			if service.Configuration.LocalOnly {
				return nil, errors.New("forced OpenRouter model cannot run in local-only mode")
			}
			return remoteProvider, nil
		}
		localProviderSet := service.localProviderSet(providerName, accelerator, true)
		return AutoProvider{
			Providers:               service.automaticLLMProviders(localProviderSet.Provider, remoteProvider),
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
		HTTPClient:              service.providerHTTPClient(),
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

func firstProviderOrder(values []string, fallback []string) []string {
	if len(values) > 0 {
		return append([]string{}, values...)
	}
	return append([]string{}, fallback...)
}

func (service Service) automaticLLMProviders(localProvider LLMProvider, remoteProvider LLMProvider) []LLMProvider {
	if service.Configuration.LocalOnly {
		return []LLMProvider{localProvider}
	}
	if service.localInferenceMode() == "device" {
		return []LLMProvider{localProvider, remoteProvider}
	}
	return []LLMProvider{remoteProvider}
}

func (service Service) localInferenceMode() string {
	return strings.ToLower(strings.TrimSpace(service.Configuration.LocalInferenceMode))
}

func (service Service) providerHealth(ctx context.Context) map[string]providerAvailability {
	return map[string]providerAvailability{
		"chatd":  service.chatdProviderHealth(ctx),
		"litert": service.liteRTProviderHealth(ctx),
	}
}

func (service Service) liteRTProviderHealth(ctx context.Context) providerAvailability {
	if !service.localBackendIsConfigured("litert") {
		return providerAvailability{Reason: "litert is not in the configured local backend order"}
	}
	providerSet := service.localProviderSet("litert", "", false)
	if len(providerSet.Backends) == 0 {
		return providerAvailability{Reason: "litert provider is not configured"}
	}
	errorValue := providerSet.Backends[0].Ping(ctx)
	if errorValue == nil {
		return providerAvailability{Configured: true, Available: true}
	}
	return providerAvailability{Configured: true, Reason: providerUnavailableReason(errorValue)}
}

func (service Service) localBackendIsConfigured(providerName string) bool {
	for _, configuredName := range firstProviderOrder(service.Configuration.LocalBackendOrder, llmbackend.DefaultDeviceLocalProviderOrder) {
		if strings.EqualFold(strings.TrimSpace(configuredName), providerName) {
			return true
		}
	}
	return false
}

func providerUnavailableReason(errorValue error) string {
	reason := llmbackend.ProviderUnavailableReason(errorValue)
	if reason == "" {
		return errorValue.Error()
	}
	return reason
}
