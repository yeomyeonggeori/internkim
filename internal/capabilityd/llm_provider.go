package capabilityd

import (
	"context"
	"errors"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/internal/llmbackend"
	"gitlab.com/eastriver/internkim/internal/runtime/locallm"
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
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

func (service Service) completeStructured(ctx context.Context, request StructuredLLMRequest) (LLMResponse, error) {
	request.Model = service.llmRequestModel(request.Model)
	provider, errorValue := service.providerForExecutionMode(ctx, "llm.structured", request.ExecutionMode, request.Provider, request.Accelerator)
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	return provider.CompleteStructured(ctx, request)
}

func (service Service) completeText(ctx context.Context, request TextLLMRequest) (LLMResponse, error) {
	request.Model = service.llmRequestModel(request.Model)
	provider, errorValue := service.providerForExecutionMode(ctx, "llm.text", request.ExecutionMode, request.Provider, request.Accelerator)
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	return provider.CompleteText(ctx, request)
}

func (service Service) completeChat(ctx context.Context, request ChatLLMRequest) (ChatLLMResponse, error) {
	request.Model = service.llmRequestModel(request.Model)
	provider, errorValue := service.providerForExecutionMode(ctx, "llm.chat", request.ExecutionMode, request.Provider, request.Accelerator)
	if errorValue != nil {
		return ChatLLMResponse{}, errorValue
	}
	chatProvider, isChatProvider := provider.(ChatLLMProvider)
	if !isChatProvider {
		return ChatLLMResponse{}, errors.New("selected llm provider does not support native chat completions")
	}
	return chatProvider.CompleteChat(ctx, request)
}

func (service Service) llmRequestModel(requestModel string) string {
	if !service.Configuration.ForceOpenRouterModel {
		return requestModel
	}
	return firstNonEmpty(service.Configuration.OpenRouterModel, requestModel)
}

func (service Service) providerForExecutionMode(ctx context.Context, toolName, executionMode, providerName, accelerator string) (LLMProvider, error) {
	companionProvider := service.companionInferenceProvider()
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
		if service.Configuration.ForceOpenRouterModel {
			if service.Configuration.LocalOnly {
				return nil, errors.New("forced OpenRouter model cannot run in local-only mode")
			}
			return remoteProvider, nil
		}
		localProviderSet := service.localProviderSet(providerName, accelerator, true)
		autoCompanionProvider := service.companionLLMProviderForAuto(ctx, toolName)
		return AutoProvider{
			Providers:               service.automaticLLMProviders(localProviderSet.Provider, autoCompanionProvider, remoteProvider),
			AttemptTimeout:          service.Configuration.ProviderAttemptTimeout,
			AllowStructuredFallback: true,
		}, nil
	default:
		return nil, errors.New("llm execution mode is not supported")
	}
}

func (service Service) companionLLMProviderForAuto(ctx context.Context, toolName string) LLMProvider {
	if strings.TrimSpace(service.Configuration.CompanionBaseURL) == "" {
		return nil
	}
	availabilityContext, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	descriptors, errorValue := service.companionProvider().capabilities(availabilityContext)
	if errorValue != nil {
		return nil
	}
	if !hasCapabilityDescriptor(descriptors, toolName) {
		return nil
	}
	return service.companionInferenceProvider()
}

func hasCapabilityDescriptor(descriptors []capabilities.Descriptor, toolName string) bool {
	for _, descriptor := range descriptors {
		if descriptor.Name == toolName {
			return true
		}
	}
	return false
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

func (service Service) automaticLLMProviders(localProvider LLMProvider, companionProvider LLMProvider, remoteProvider LLMProvider) []LLMProvider {
	switch service.localInferenceMode() {
	case "device":
		if service.Configuration.LocalOnly {
			return []LLMProvider{localProvider, companionProvider}
		}
		return []LLMProvider{localProvider, remoteProvider, companionProvider}
	case "companion_preferred":
		if service.Configuration.LocalOnly {
			return []LLMProvider{companionProvider, localProvider}
		}
		return []LLMProvider{companionProvider, remoteProvider, localProvider}
	case "companion_only":
		return []LLMProvider{companionProvider}
	case "remote":
		if service.Configuration.LocalOnly {
			return []LLMProvider{companionProvider, localProvider}
		}
		return []LLMProvider{remoteProvider, companionProvider}
	}
	if service.Configuration.LocalOnly {
		return []LLMProvider{companionProvider, localProvider}
	}
	if service.Configuration.PreferCompanionLLM {
		return []LLMProvider{companionProvider, remoteProvider, localProvider}
	}
	return []LLMProvider{remoteProvider, companionProvider, localProvider}
}

func (service Service) localInferenceMode() string {
	normalizedMode := strings.ToLower(strings.TrimSpace(service.Configuration.LocalInferenceMode))
	if normalizedMode != "" {
		return normalizedMode
	}
	if service.Configuration.PreferCompanionLLM {
		return "companion_preferred"
	}
	return ""
}

func (service Service) providerHealth(ctx context.Context) map[string]providerAvailability {
	return map[string]providerAvailability{
		"litert": service.liteRTProviderHealth(ctx),
	}
}

func (service Service) liteRTProviderHealth(ctx context.Context) providerAvailability {
	providerSet := service.localProviderSet("litert", "", false)
	if len(providerSet.Backends) == 0 {
		return providerAvailability{Available: false, Reason: "litert provider is not configured"}
	}
	errorValue := providerSet.Backends[0].Ping(ctx)
	if errorValue == nil {
		return providerAvailability{Available: true}
	}
	reason := llmbackend.ProviderUnavailableReason(errorValue)
	if reason == "" {
		reason = errorValue.Error()
	}
	return providerAvailability{Available: false, Reason: reason}
}
