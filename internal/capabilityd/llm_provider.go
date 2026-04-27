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

type LiteRTProvider struct {
	Configuration Configuration
	RunCommand    func(context.Context, string, []string, []byte) ([]byte, error)
}

type OpenRouterProvider struct {
	Configuration Configuration
	HTTPClient    *http.Client
}

type AutoProvider struct {
	Local  StructuredLLMProvider
	Remote StructuredLLMProvider
}

type litertRequest struct {
	ModelPath              string                 `json:"modelPath"`
	Backend                string                 `json:"backend"`
	Messages               []LLMMessage           `json:"messages"`
	StructuredOutputSchema StructuredOutputSchema `json:"structuredOutputSchema"`
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
	structuredRequest := StructuredLLMRequest{
		Model:                 request.Model,
		ExecutionMode:         request.ExecutionMode,
		Messages:              request.Messages,
		RequireParameters:     request.RequireParameters,
		EnableResponseHealing: request.EnableResponseHealing,
		StructuredOutputSchema: StructuredOutputSchema{
			Name:               "plain_text_response",
			Document:           json.RawMessage(`{"type":"object","properties":{"content":{"type":"string"}},"required":["content"],"additionalProperties":false}`),
			IsStrictlyEnforced: true,
		},
	}
	structuredResponse, errorValue := service.completeStructured(ctx, structuredRequest)
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	var textDocument struct {
		Content string `json:"content"`
	}
	if errorValue := json.Unmarshal([]byte(structuredResponse.Content), &textDocument); errorValue != nil {
		return LLMResponse{}, errorValue
	}
	structuredResponse.Content = textDocument.Content
	return structuredResponse, nil
}

func (service Service) providerForExecutionMode(executionMode string) (StructuredLLMProvider, error) {
	localProvider := LiteRTProvider{
		Configuration: service.Configuration,
		RunCommand:    service.runCommand,
	}
	remoteProvider := OpenRouterProvider{
		Configuration: service.Configuration,
		HTTPClient:    service.httpClient(),
	}
	switch strings.ToLower(firstNonEmpty(executionMode, "auto")) {
	case "local":
		return localProvider, nil
	case "remote":
		return remoteProvider, nil
	case "auto":
		return AutoProvider{Local: localProvider, Remote: remoteProvider}, nil
	default:
		return nil, errors.New("llm execution mode is not supported")
	}
}

func (provider AutoProvider) CompleteStructured(ctx context.Context, request StructuredLLMRequest) (LLMResponse, error) {
	response, errorValue := provider.Local.CompleteStructured(ctx, request)
	if errorValue == nil {
		return response, nil
	}
	logLocalLLMFallback(errorValue)
	return provider.Remote.CompleteStructured(ctx, request)
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
		Messages:               request.Messages,
		StructuredOutputSchema: request.StructuredOutputSchema,
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

func (provider OpenRouterProvider) CompleteStructured(ctx context.Context, request StructuredLLMRequest) (LLMResponse, error) {
	apiKey := readSecretValue(provider.Configuration.OpenRouterKeyPath)
	if apiKey == "" {
		return LLMResponse{}, errors.New("openrouter api key is not configured")
	}
	if isPlaceholderOpenRouterKey(apiKey) {
		return LLMResponse{}, errors.New("openrouter api key is a simulation placeholder; set OPENROUTER_API_KEY or rerun setup --only openrouter --force")
	}

	requestDocument, errorValue := buildOpenRouterRequest(request)
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, provider.Configuration.OpenRouterBaseURL, bytes.NewReader(requestDocument))
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, errorValue := provider.HTTPClient.Do(httpRequest)
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	defer httpResponse.Body.Close()
	responseDocument, _ := io.ReadAll(httpResponse.Body)
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return LLMResponse{}, errors.New(string(responseDocument))
	}

	var parsedResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if errorValue := json.Unmarshal(responseDocument, &parsedResponse); errorValue != nil {
		return LLMResponse{}, errorValue
	}
	if len(parsedResponse.Choices) == 0 {
		return LLMResponse{}, errors.New("openrouter response did not include choices")
	}
	return LLMResponse{
		Provider:        "openrouter",
		Model:           request.model(),
		Content:         parsedResponse.Choices[0].Message.Content,
		SelectedBackend: "remote",
		ConstraintMode:  "provider_json_schema",
	}, nil
}

func buildOpenRouterRequest(request StructuredLLMRequest) ([]byte, error) {
	document := map[string]any{
		"model":    request.model(),
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

func (request StructuredLLMRequest) model() string {
	return strings.TrimSpace(request.Model)
}

func logLocalLLMFallback(errorValue error) {
	if errorValue != nil {
		log.Printf("litert local capability failed; falling back to remote: %v", errorValue)
	}
}
