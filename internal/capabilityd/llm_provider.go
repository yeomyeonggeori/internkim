package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/anthropic-lab/internkim/internal/capabilities"
)

type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type StructuredOutputSchema struct {
	Name               string          `json:"name"`
	Document           json.RawMessage `json:"document"`
	IsStrictlyEnforced bool            `json:"isStrictlyEnforced"`
}

type StructuredLLMRequest struct {
	Model                  string                 `json:"model"`
	ExecutionMode          string                 `json:"executionMode"`
	Messages               []LLMMessage           `json:"messages"`
	StructuredOutputSchema StructuredOutputSchema `json:"structuredOutputSchema"`
	RequireParameters      bool                   `json:"requireParameters"`
	EnableResponseHealing  bool                   `json:"enableResponseHealing"`
}

type TextLLMRequest struct {
	Model                 string       `json:"model"`
	ExecutionMode         string       `json:"executionMode"`
	Messages              []LLMMessage `json:"messages"`
	RequireParameters     bool         `json:"requireParameters"`
	EnableResponseHealing bool         `json:"enableResponseHealing"`
}

type LLMResponse struct {
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	Content         string `json:"content"`
	SelectedBackend string `json:"selectedBackend"`
	ConstraintMode  string `json:"constraintMode,omitempty"`
}

type StructuredLLMProvider interface {
	CompleteStructured(context.Context, StructuredLLMRequest) (LLMResponse, error)
}

type TextLLMProvider interface {
	CompleteText(context.Context, TextLLMRequest) (LLMResponse, error)
}

type LLMProvider interface {
	StructuredLLMProvider
	TextLLMProvider
}

type LiteRTProvider struct {
	Configuration Configuration
	RunCommand    func(context.Context, string, []string, []byte) ([]byte, error)
}

type OpenRouterProvider struct {
	Configuration Configuration
	HTTPClient    *http.Client
}

type AutoProvider struct {
	Providers      []LLMProvider
	AttemptTimeout time.Duration
}

type litertRequest struct {
	ModelPath              string                  `json:"modelPath"`
	Backend                string                  `json:"backend"`
	Mode                   string                  `json:"mode"`
	Messages               []LLMMessage            `json:"messages"`
	StructuredOutputSchema *StructuredOutputSchema `json:"structuredOutputSchema,omitempty"`
}

type litertResponse struct {
	Content string `json:"content"`
}

func (service Service) completeStructured(ctx context.Context, request StructuredLLMRequest) (LLMResponse, error) {
	provider, errorValue := service.providerForExecutionMode(request.ExecutionMode)
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	return provider.CompleteStructured(ctx, request)
}

func (service Service) completeText(ctx context.Context, request TextLLMRequest) (LLMResponse, error) {
	provider, errorValue := service.providerForExecutionMode(request.ExecutionMode)
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	return provider.CompleteText(ctx, request)
}

func (service Service) providerForExecutionMode(executionMode string) (LLMProvider, error) {
	localProvider := LiteRTProvider{
		Configuration: service.Configuration,
		RunCommand:    service.runCommand,
	}
	companionProvider := service.companionProvider()
	remoteProvider := OpenRouterProvider{
		Configuration: service.Configuration,
		HTTPClient:    service.httpClient(),
	}
	switch strings.ToLower(firstNonEmpty(executionMode, "auto")) {
	case "local":
		return localProvider, nil
	case "companion", "user_desktop":
		return companionProvider, nil
	case "remote":
		if service.Configuration.LocalOnly {
			return nil, errors.New("remote llm execution is disabled by local-only mode")
		}
		return remoteProvider, nil
	case "auto":
		return AutoProvider{Providers: service.automaticLLMProviders(localProvider, companionProvider, remoteProvider), AttemptTimeout: service.Configuration.ProviderAttemptTimeout}, nil
	default:
		return nil, errors.New("llm execution mode is not supported")
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

func (provider AutoProvider) CompleteStructured(ctx context.Context, request StructuredLLMRequest) (LLMResponse, error) {
	return completeWithProviderChain(provider.Providers, func(candidate LLMProvider) (LLMResponse, error) {
		attemptContext, cancel := context.WithTimeout(ctx, provider.attemptTimeout())
		defer cancel()
		return candidate.CompleteStructured(attemptContext, request)
	})
}

func (provider AutoProvider) CompleteText(ctx context.Context, request TextLLMRequest) (LLMResponse, error) {
	return completeWithProviderChain(provider.Providers, func(candidate LLMProvider) (LLMResponse, error) {
		attemptContext, cancel := context.WithTimeout(ctx, provider.attemptTimeout())
		defer cancel()
		return candidate.CompleteText(attemptContext, request)
	})
}

func (provider AutoProvider) attemptTimeout() time.Duration {
	if provider.AttemptTimeout <= 0 {
		return DefaultConfiguration().ProviderAttemptTimeout
	}
	return provider.AttemptTimeout
}

func completeWithProviderChain(providers []LLMProvider, complete func(LLMProvider) (LLMResponse, error)) (LLMResponse, error) {
	var lastError error
	for index, candidate := range providers {
		if candidate == nil {
			continue
		}
		response, errorValue := complete(candidate)
		if errorValue == nil {
			return response, nil
		}
		lastError = errorValue
		if index == 0 {
			logLLMFallback(errorValue)
		}
	}
	if lastError == nil {
		lastError = errors.New("no llm provider is available")
	}
	return LLMResponse{}, lastError
}

func (provider LiteRTProvider) CompleteStructured(ctx context.Context, request StructuredLLMRequest) (LLMResponse, error) {
	var lastError error
	for _, backend := range []string{"gpu", "cpu"} {
		response, errorValue := provider.completeWithBackend(ctx, request, backend)
		if errorValue == nil {
			return response, nil
		}
		lastError = errorValue
	}
	if lastError == nil {
		lastError = errors.New("litert backend list is empty")
	}
	return LLMResponse{}, lastError
}

func (provider LiteRTProvider) completeWithBackend(ctx context.Context, request StructuredLLMRequest, backend string) (LLMResponse, error) {
	modelPath := firstNonEmpty(provider.Configuration.LiteRTModelPath, DefaultConfiguration().LiteRTModelPath)
	wrapperPath := firstNonEmpty(provider.Configuration.LiteRTWrapperPath, DefaultConfiguration().LiteRTWrapperPath)
	document, errorValue := json.Marshal(litertRequest{
		ModelPath:              modelPath,
		Backend:                backend,
		Mode:                   "structured",
		Messages:               request.Messages,
		StructuredOutputSchema: &request.StructuredOutputSchema,
	})
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}

	output, errorValue := provider.RunCommand(ctx, wrapperPath, nil, document)
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}

	var response litertResponse
	if errorValue := json.Unmarshal(output, &response); errorValue != nil {
		return LLMResponse{}, errorValue
	}
	if !validateMinimumStructuredOutput(response.Content, request.StructuredOutputSchema.Document) {
		return LLMResponse{}, errors.New("litert response did not satisfy structured output schema")
	}
	return LLMResponse{
		Provider:        "litert",
		Model:           "gemma-4-E4B-it-litert-lm",
		Content:         response.Content,
		SelectedBackend: backend,
		ConstraintMode:  "prompt_validation",
	}, nil
}

func (provider LiteRTProvider) CompleteText(ctx context.Context, request TextLLMRequest) (LLMResponse, error) {
	var lastError error
	for _, backend := range []string{"gpu", "cpu"} {
		response, errorValue := provider.completeTextWithBackend(ctx, request, backend)
		if errorValue == nil {
			return response, nil
		}
		lastError = errorValue
	}
	if lastError == nil {
		lastError = errors.New("litert backend list is empty")
	}
	return LLMResponse{}, lastError
}

func (provider LiteRTProvider) completeTextWithBackend(ctx context.Context, request TextLLMRequest, backend string) (LLMResponse, error) {
	modelPath := firstNonEmpty(provider.Configuration.LiteRTModelPath, DefaultConfiguration().LiteRTModelPath)
	wrapperPath := firstNonEmpty(provider.Configuration.LiteRTWrapperPath, DefaultConfiguration().LiteRTWrapperPath)
	document, errorValue := json.Marshal(litertRequest{
		ModelPath: modelPath,
		Backend:   backend,
		Mode:      "text",
		Messages:  request.Messages,
	})
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}

	output, errorValue := provider.RunCommand(ctx, wrapperPath, nil, document)
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}

	var response litertResponse
	if errorValue := json.Unmarshal(output, &response); errorValue != nil {
		return LLMResponse{}, errorValue
	}
	return LLMResponse{
		Provider:        "litert",
		Model:           "gemma-4-E4B-it-litert-lm",
		Content:         response.Content,
		SelectedBackend: backend,
	}, nil
}

func (provider OpenRouterProvider) CompleteStructured(ctx context.Context, request StructuredLLMRequest) (LLMResponse, error) {
	apiKey := readSecretValue(provider.Configuration.OpenRouterKeyPath)
	if apiKey == "" {
		return LLMResponse{}, errors.New("openrouter api key is not configured")
	}
	if isPlaceholderOpenRouterKey(apiKey) {
		return LLMResponse{}, errors.New("openrouter api key is a simulation placeholder; set OPENROUTER_API_KEY or rerun setup --only openrouter --force")
	}

	modelName := provider.remoteModelName(request.Model)
	requestDocument, errorValue := buildOpenRouterRequest(request, modelName)
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	content, errorValue := provider.sendOpenRouterRequest(ctx, apiKey, requestDocument)
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	return LLMResponse{
		Provider:        "openrouter",
		Model:           modelName,
		Content:         content,
		SelectedBackend: capabilities.LLMBackendRemote,
		ConstraintMode:  "provider_json_schema",
	}, nil
}

func (provider OpenRouterProvider) CompleteText(ctx context.Context, request TextLLMRequest) (LLMResponse, error) {
	apiKey := readSecretValue(provider.Configuration.OpenRouterKeyPath)
	if apiKey == "" {
		return LLMResponse{}, errors.New("openrouter api key is not configured")
	}
	if isPlaceholderOpenRouterKey(apiKey) {
		return LLMResponse{}, errors.New("openrouter api key is a simulation placeholder; set OPENROUTER_API_KEY or rerun setup --only openrouter --force")
	}

	modelName := provider.remoteModelName(request.Model)
	requestDocument, errorValue := buildOpenRouterTextRequest(request, modelName)
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	content, errorValue := provider.sendOpenRouterRequest(ctx, apiKey, requestDocument)
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	return LLMResponse{
		Provider:        "openrouter",
		Model:           modelName,
		Content:         content,
		SelectedBackend: capabilities.LLMBackendRemote,
	}, nil
}

func buildOpenRouterRequest(request StructuredLLMRequest, modelName string) ([]byte, error) {
	document := map[string]any{
		"model":    modelName,
		"messages": request.Messages,
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   request.StructuredOutputSchema.Name,
				"strict": request.StructuredOutputSchema.IsStrictlyEnforced,
				"schema": request.StructuredOutputSchema.Document,
			},
		},
		"stream": false,
	}
	if request.RequireParameters {
		document["provider"] = map[string]bool{"require_parameters": true}
	}
	if request.EnableResponseHealing {
		document["plugins"] = []map[string]string{{"id": "response-healing"}}
	}
	return json.Marshal(document)
}

func buildOpenRouterTextRequest(request TextLLMRequest, modelName string) ([]byte, error) {
	document := map[string]any{
		"model":    modelName,
		"messages": request.Messages,
		"stream":   false,
	}
	if request.RequireParameters {
		document["provider"] = map[string]bool{"require_parameters": true}
	}
	if request.EnableResponseHealing {
		document["plugins"] = []map[string]string{{"id": "response-healing"}}
	}
	return json.Marshal(document)
}

func (provider OpenRouterProvider) sendOpenRouterRequest(ctx context.Context, apiKey string, requestDocument []byte) (string, error) {
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, provider.Configuration.OpenRouterBaseURL, bytes.NewReader(requestDocument))
	if errorValue != nil {
		return "", errorValue
	}
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, errorValue := provider.HTTPClient.Do(httpRequest)
	if errorValue != nil {
		return "", errorValue
	}
	defer httpResponse.Body.Close()

	responseDocument, errorValue := io.ReadAll(httpResponse.Body)
	if errorValue != nil {
		return "", errors.New("read openrouter response: " + errorValue.Error())
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return "", errors.New(string(responseDocument))
	}
	if len(bytes.TrimSpace(responseDocument)) == 0 {
		return "", errors.New("openrouter response body was empty")
	}

	var parsedResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if errorValue := json.Unmarshal(responseDocument, &parsedResponse); errorValue != nil {
		return "", errorValue
	}
	if len(parsedResponse.Choices) == 0 {
		return "", errors.New("openrouter response did not include choices")
	}
	return parsedResponse.Choices[0].Message.Content, nil
}

func validateMinimumStructuredOutput(content string, schemaDocument json.RawMessage) bool {
	var parsedContent any
	if json.Unmarshal([]byte(content), &parsedContent) != nil {
		return false
	}
	if len(bytes.TrimSpace(schemaDocument)) == 0 {
		return true
	}

	var schema struct {
		Required []string `json:"required"`
	}
	if json.Unmarshal(schemaDocument, &schema) != nil {
		return true
	}
	contentMap, isMap := parsedContent.(map[string]any)
	if !isMap {
		return len(schema.Required) == 0
	}
	for _, requiredKey := range schema.Required {
		if _, isFound := contentMap[requiredKey]; !isFound {
			return false
		}
	}
	return true
}

func (provider OpenRouterProvider) remoteModelName(modelName string) string {
	normalizedModelName := strings.TrimSpace(modelName)
	if normalizedModelName == "" || strings.EqualFold(normalizedModelName, "default") || isLocalModelReference(normalizedModelName) {
		return firstNonEmpty(provider.Configuration.OpenRouterModel, DefaultConfiguration().OpenRouterModel)
	}
	return normalizedModelName
}

func isLocalModelReference(modelName string) bool {
	normalizedModelName := strings.ToLower(strings.TrimSpace(modelName))
	return strings.HasPrefix(normalizedModelName, "local/") ||
		strings.HasSuffix(normalizedModelName, ".litertlm") ||
		strings.Contains(normalizedModelName, "litert-lm")
}

func logLLMFallback(errorValue error) {
	if errorValue != nil {
		log.Printf("llm provider failed; trying next provider: %v", errorValue)
	}
}
