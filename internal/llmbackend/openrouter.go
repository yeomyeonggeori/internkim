package llmbackend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type OpenRouterBackend struct {
	KeyPath             string
	BaseURL             string
	ModelName           string
	GatewaySecretPath   string
	GatewaySecretHeader string
	HTTPClient          *http.Client
}

func (backend OpenRouterBackend) Name() string { return "openrouter" }

func (backend OpenRouterBackend) Ping(ctx context.Context) error {
	apiKey := readOpenRouterKey(backend.KeyPath)
	if apiKey == "" {
		return errors.New("openrouter api key is not configured")
	}
	if isPlaceholderOpenRouterKey(apiKey) {
		return errors.New("openrouter api key is a simulation placeholder")
	}
	return nil
}

func (backend OpenRouterBackend) CompleteStructured(ctx context.Context, request StructuredRequest) (Response, error) {
	apiKey, errorValue := backend.resolveAPIKey()
	if errorValue != nil {
		return Response{}, errorValue
	}
	modelName := backend.resolveModelName(request.Model)
	if response, isHandled, errorValue := backend.completeNativeAction(ctx, apiKey, request, modelName); isHandled {
		return response, errorValue
	}
	requestDocument, errorValue := buildOpenRouterStructuredRequest(request, modelName)
	if errorValue != nil {
		return Response{}, errorValue
	}
	content, usage, errorValue := backend.send(ctx, apiKey, requestDocument)
	if errorValue != nil {
		return Response{}, errorValue
	}
	return Response{
		Provider:        "openrouter",
		Model:           modelName,
		Content:         content,
		SelectedBackend: capabilities.LLMBackendRemote,
		ConstraintMode:  ConstraintModeOpenAIJSONSchema,
		Usage:           usage,
	}, nil
}

func (backend OpenRouterBackend) completeNativeAction(ctx context.Context, apiKey string, request StructuredRequest, modelName string) (Response, bool, error) {
	toolSet, isActionSchema, errorValue := nativeActionToolsForSchema(request.StructuredOutputSchema)
	if errorValue != nil || !isActionSchema {
		return Response{}, isActionSchema, errorValue
	}
	requestDocument, lintResult, errorValue := buildOpenRouterChatActionRequest(request, modelName, toolSet.Tools, toolSet.NativeSchemaLint)
	if errorValue != nil {
		return Response{}, true, errorValue
	}
	content, usage, errorValue := backend.sendChatAction(ctx, apiKey, requestDocument, toolSet, lintResult)
	if errorValue != nil {
		return Response{}, true, errorValue
	}
	return Response{
		Provider:        "openrouter",
		Model:           modelName,
		Content:         content,
		SelectedBackend: capabilities.LLMBackendRemote,
		ConstraintMode:  ConstraintModeNativeToolCall,
		Usage:           usage,
	}, true, nil
}

func (backend OpenRouterBackend) CompleteText(ctx context.Context, request TextRequest) (Response, error) {
	apiKey, errorValue := backend.resolveAPIKey()
	if errorValue != nil {
		return Response{}, errorValue
	}
	modelName := backend.resolveModelName(request.Model)
	requestDocument, errorValue := buildOpenRouterTextRequest(request, modelName)
	if errorValue != nil {
		return Response{}, errorValue
	}
	content, usage, errorValue := backend.send(ctx, apiKey, requestDocument)
	if errorValue != nil {
		return Response{}, errorValue
	}
	return Response{
		Provider:        "openrouter",
		Model:           modelName,
		Content:         content,
		SelectedBackend: capabilities.LLMBackendRemote,
		Usage:           usage,
	}, nil
}

func (backend OpenRouterBackend) CompleteChat(ctx context.Context, request ChatRequest) (ChatResponse, error) {
	apiKey, errorValue := backend.resolveAPIKey()
	if errorValue != nil {
		return ChatResponse{}, errorValue
	}
	modelName := backend.resolveModelName(request.Model)
	requestDocument, errorValue := json.Marshal(openAIChatCompletionRequest(modelName, request))
	if errorValue != nil {
		return ChatResponse{}, errorValue
	}
	response, errorValue := backend.sendChatCompletion(ctx, apiKey, requestDocument)
	if errorValue != nil {
		return ChatResponse{}, errorValue
	}
	return chatResponseFromOpenAI("openrouter", modelName, response), nil
}

func (backend OpenRouterBackend) resolveAPIKey() (string, error) {
	apiKey := readOpenRouterKey(backend.KeyPath)
	if apiKey == "" {
		return "", errors.New("openrouter api key is not configured")
	}
	if isPlaceholderOpenRouterKey(apiKey) {
		return "", errors.New("openrouter api key is a simulation placeholder; set OPENROUTER_API_KEY or rerun setup --only openrouter --force")
	}
	return apiKey, nil
}

func (backend OpenRouterBackend) resolveModelName(requestedModel string) string {
	normalized := strings.TrimSpace(requestedModel)
	if normalized == "" || strings.EqualFold(normalized, "default") || isLocalModelReference(normalized) {
		return strings.TrimSpace(backend.ModelName)
	}
	return normalized
}

func (backend OpenRouterBackend) send(ctx context.Context, apiKey string, requestDocument []byte) (string, Usage, error) {
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, backend.BaseURL, bytes.NewReader(requestDocument))
	if errorValue != nil {
		return "", Usage{}, errorValue
	}
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	backend.setGatewaySecretHeader(httpRequest)

	httpResponse, errorValue := backend.HTTPClient.Do(httpRequest)
	if errorValue != nil {
		return "", Usage{}, errorValue
	}
	defer httpResponse.Body.Close()

	responseDocument, errorValue := io.ReadAll(httpResponse.Body)
	if errorValue != nil {
		return "", Usage{}, errors.New("read openrouter response: " + errorValue.Error())
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return "", Usage{}, normalizeProviderError("openrouter", httpResponse.StatusCode, responseDocument)
	}
	if len(bytes.TrimSpace(responseDocument)) == 0 {
		return "", Usage{}, errors.New("openrouter response body was empty")
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage openAIUsage `json:"usage"`
	}
	if errorValue := json.Unmarshal(responseDocument, &parsed); errorValue != nil {
		return "", Usage{}, errorValue
	}
	if len(parsed.Choices) == 0 {
		return "", Usage{}, errors.New("openrouter response did not include choices")
	}
	return parsed.Choices[0].Message.Content, normalizeUsage(parsed.Usage), nil
}

func (backend OpenRouterBackend) sendChatAction(ctx context.Context, apiKey string, requestDocument []byte, toolSet nativeActionToolSet, lintResult NativeSchemaLintResult) (string, Usage, error) {
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, backend.BaseURL, bytes.NewReader(requestDocument))
	if errorValue != nil {
		return "", Usage{}, errorValue
	}
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	backend.setGatewaySecretHeader(httpRequest)

	httpResponse, errorValue := backend.HTTPClient.Do(httpRequest)
	if errorValue != nil {
		return "", Usage{}, errorValue
	}
	defer httpResponse.Body.Close()

	responseDocument, errorValue := io.ReadAll(httpResponse.Body)
	if errorValue != nil {
		return "", Usage{}, errors.New("read openrouter response: " + errorValue.Error())
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return "", Usage{}, openAIErrorWithNativeSchemaLint("openrouter", httpResponse.StatusCode, responseDocument, lintResult)
	}

	var response openAIResponseWithUsage
	if errorValue := json.Unmarshal(responseDocument, &response); errorValue != nil {
		return "", Usage{}, errorValue
	}
	if len(response.Choices) == 0 {
		return "", Usage{}, errors.New("openrouter response did not include choices")
	}
	for _, toolCall := range response.Choices[0].Message.ToolCalls {
		if toolCall.Type == "" || toolCall.Type == "function" {
			content, errorValue := nativeActionJSON(toolSet, toolCall.Function.Name, toolCall.Function.Arguments)
			return content, normalizeUsage(response.Usage), errorValue
		}
	}
	return "", Usage{}, errors.New("openrouter chat completion response did not include tool_calls")
}

func (backend OpenRouterBackend) sendChatCompletion(ctx context.Context, apiKey string, requestDocument []byte) (openAIResponseWithUsage, error) {
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, backend.BaseURL, bytes.NewReader(requestDocument))
	if errorValue != nil {
		return openAIResponseWithUsage{}, errorValue
	}
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	backend.setGatewaySecretHeader(httpRequest)

	httpResponse, errorValue := backend.HTTPClient.Do(httpRequest)
	if errorValue != nil {
		return openAIResponseWithUsage{}, errorValue
	}
	defer httpResponse.Body.Close()

	responseDocument, errorValue := io.ReadAll(httpResponse.Body)
	if errorValue != nil {
		return openAIResponseWithUsage{}, errors.New("read openrouter response: " + errorValue.Error())
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return openAIResponseWithUsage{}, normalizeProviderError("openrouter", httpResponse.StatusCode, responseDocument)
	}

	var response openAIResponseWithUsage
	if errorValue := json.Unmarshal(responseDocument, &response); errorValue != nil {
		return openAIResponseWithUsage{}, errorValue
	}
	if len(response.Choices) == 0 {
		return openAIResponseWithUsage{}, errors.New("openrouter response did not include choices")
	}
	return response, nil
}

func (backend OpenRouterBackend) setGatewaySecretHeader(request *http.Request) {
	gatewaySecret := strings.TrimSpace(readOpenRouterKey(backend.GatewaySecretPath))
	if gatewaySecret == "" {
		return
	}
	headerName := strings.TrimSpace(backend.GatewaySecretHeader)
	if headerName == "" {
		headerName = "X-InternKim-Gateway-Secret"
	}
	request.Header.Set(headerName, gatewaySecret)
}

func buildOpenRouterStructuredRequest(request StructuredRequest, modelName string) ([]byte, error) {
	document := map[string]any{
		"model":    modelName,
		"messages": openAIMessages(request.Messages),
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
	addGenerationOptions(document, request.GenerationOptions)
	return json.Marshal(document)
}

func buildOpenRouterChatActionRequest(request StructuredRequest, modelName string, tools []nativeActionTool, lintResults ...NativeSchemaLintResult) ([]byte, NativeSchemaLintResult, error) {
	document := openAIActionToolRequest(modelName, request.Messages, tools, generationOptionsValue(request.GenerationOptions), lintResults...)
	content, errorValue := json.Marshal(document)
	return content, document.NativeToolSchemaLint, errorValue
}

func buildOpenRouterTextRequest(request TextRequest, modelName string) ([]byte, error) {
	document := map[string]any{
		"model":    modelName,
		"messages": openAIMessages(request.Messages),
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

func addGenerationOptions(document map[string]any, options *GenerationOptions) {
	if options == nil {
		return
	}
	if options.Seed != nil {
		document["seed"] = *options.Seed
	}
	if options.Temperature != nil {
		document["temperature"] = *options.Temperature
	}
}

func generationOptionsValue(options *GenerationOptions) GenerationOptions {
	if options == nil {
		return GenerationOptions{}
	}
	return *options
}

func readOpenRouterKey(path string) string {
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return ""
	}
	value := strings.TrimSpace(string(document))
	return strings.TrimPrefix(value, "OPENROUTER_API_KEY=")
}

func isPlaceholderOpenRouterKey(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(normalized, "internkim-simulation-openrouter-api-key") ||
		strings.Contains(normalized, "simulation-openrouter")
}

func isLocalModelReference(modelName string) bool {
	normalized := strings.ToLower(strings.TrimSpace(modelName))
	return strings.HasPrefix(normalized, "local/")
}
