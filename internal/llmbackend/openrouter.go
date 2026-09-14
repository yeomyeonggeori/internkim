package llmbackend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/internal/modelladder"
)

type OpenRouterBackend struct {
	KeyPath             string
	BaseURL             string
	ModelName           string
	FallbackModelNames  []string
	GatewaySecretPath   string
	GatewaySecretHeader string
	ProviderOrder       []string
	ProviderSort        string
	HTTPClient          *http.Client
}

const DefaultActionModelName = modelladder.PrimaryModel

var DefaultOpenRouterActionFallbackModels = modelladder.DegradedModels

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
	if isActionTurnStructuredRequest(request) {
		return backend.completeActionStructured(ctx, apiKey, request, backend.actionModelNames(modelName))
	}
	return backend.completeStructuredModels(ctx, apiKey, request, backend.actionModelNames(modelName))
}

func (backend OpenRouterBackend) completeStructuredModels(ctx context.Context, apiKey string, request StructuredRequest, modelNames []string) (Response, error) {
	attemptErrors := []error{}
	for _, modelName := range modelNames {
		if errorValue := ctx.Err(); errorValue != nil {
			return Response{}, errors.Join(append(attemptErrors, errorValue)...)
		}
		response, errorValue := backend.completeStructuredModel(ctx, apiKey, request, modelName)
		if errorValue == nil {
			if len(attemptErrors) > 0 {
				response.UsedFallback = true
				response.FallbackReason = joinedErrors(attemptErrors)
			}
			return response, nil
		}
		log.Printf("structured completion attempt failed: model=%s error=%v", modelName, truncatedAttemptErrorText(errorValue))
		attemptErrors = append(attemptErrors, fmt.Errorf("%s: %w", modelName, errorValue))
		if contextError := ctx.Err(); contextError != nil {
			return Response{}, errors.Join(append(attemptErrors, contextError)...)
		}
	}
	return Response{}, errors.Join(attemptErrors...)
}

func (backend OpenRouterBackend) completeStructuredModel(ctx context.Context, apiKey string, request StructuredRequest, modelName string) (Response, error) {
	if prefersPromptedStructuredJSON(modelName) {
		return backend.completePromptedJSON(ctx, apiKey, request, modelName)
	}
	response, errorValue := backend.completeJSONSchema(ctx, apiKey, request, modelName)
	if errorValue == nil {
		return response, nil
	}
	if isStructuredOutputLimitError(errorValue) {
		return response, errorValue
	}
	if contextError := ctx.Err(); contextError != nil {
		return Response{}, errors.Join(errorValue, contextError)
	}
	promptedResponse, promptedError := backend.completePromptedJSON(ctx, apiKey, request, modelName)
	if promptedError == nil {
		return promptedResponse, nil
	}
	if isStructuredOutputLimitError(promptedError) {
		return promptedResponse, errors.Join(errorValue, promptedError)
	}
	// A model that answers with nothing has not refused the schema, it has
	// dropped the turn: the same request succeeds on the next attempt. Giving up
	// here hands the turn to whatever stands next in the chain, which on a device
	// is a model small enough to invent its answer.
	if isEmptyStructuredContentError(errorValue) && isEmptyStructuredContentError(promptedError) {
		retriedResponse, retryError := backend.completeJSONSchema(ctx, apiKey, request, modelName)
		if retryError == nil {
			log.Printf("structured completion recovered after an empty answer: model=%s schemaName=%s", modelName, request.StructuredOutputSchema.Name)
			return retriedResponse, nil
		}
	}
	return Response{}, errors.New("json schema completion failed: " + errorValue.Error() + "; prompted json fallback failed: " + promptedError.Error())
}

func (backend OpenRouterBackend) completeActionStructured(ctx context.Context, apiKey string, request StructuredRequest, modelNames []string) (Response, error) {
	if len(modelNames) == 1 && prefersPromptedStructuredJSON(modelNames[0]) {
		return backend.completePromptedJSON(ctx, apiKey, request, modelNames[0])
	}
	nativeErrors := []error{}
	for _, modelName := range modelNames {
		response, isHandled, errorValue := backend.completeNativeAction(ctx, apiKey, request, modelName)
		if !isHandled {
			return backend.completeJSONSchema(ctx, apiKey, request, modelName)
		}
		if errorValue == nil {
			log.Printf("action structured completion succeeded: mode=native model=%s", modelName)
			return response, nil
		}
		log.Printf("native action attempt failed; trying next action model: model=%s error=%v", modelName, truncatedAttemptErrorText(errorValue))
		nativeErrors = append(nativeErrors, modelAttemptError(modelName, errorValue))
	}
	fallbackErrors := []error{}
	for _, modelName := range modelNames {
		response, errorValue := backend.completeJSONSchema(ctx, apiKey, request, modelName)
		if errorValue == nil {
			log.Printf("action structured completion succeeded: mode=json-schema model=%s", modelName)
			return response, nil
		}
		log.Printf("json schema action attempt failed; trying next action model: model=%s error=%v", modelName, truncatedAttemptErrorText(errorValue))
		fallbackErrors = append(fallbackErrors, modelAttemptError(modelName, errorValue))
	}
	promptedErrors := []error{}
	for _, modelName := range modelNames {
		response, errorValue := backend.completePromptedJSON(ctx, apiKey, request, modelName)
		if errorValue == nil {
			log.Printf("action structured completion succeeded: mode=prompted-json model=%s", modelName)
			return response, nil
		}
		log.Printf("prompted json action attempt failed; trying next action model: model=%s error=%v", modelName, truncatedAttemptErrorText(errorValue))
		promptedErrors = append(promptedErrors, modelAttemptError(modelName, errorValue))
	}
	return Response{}, nativeActionModelFallbackError(nativeErrors, fallbackErrors, promptedErrors)
}

func (backend OpenRouterBackend) completeNativeAction(ctx context.Context, apiKey string, request StructuredRequest, modelName string) (Response, bool, error) {
	toolSet, isActionSchema, errorValue := nativeActionToolsForSchema(request.StructuredOutputSchema)
	if errorValue != nil || !isActionSchema {
		return Response{}, isActionSchema, errorValue
	}
	document := backend.chatActionDocument(request, modelName, toolSet.Tools, toolSet.NativeSchemaLint)
	response, servingNote, errorValue := backend.streamWithOneRetry(ctx, apiKey, modelName, func(ignoredProviders []string) ([]byte, error) {
		document.Provider = backend.providerRoutingIgnoring(request.RequireParameters, ignoredProviders)
		return json.Marshal(document)
	})
	if errorValue != nil {
		return Response{}, true, withNativeSchemaLint(errorValue, document.NativeToolSchemaLint)
	}
	completion, errorValue := nativeActionCompletionFromStream(response, toolSet)
	if errorValue != nil {
		return Response{}, true, errorValue
	}
	return Response{
		Provider:         "openrouter",
		UpstreamProvider: completion.UpstreamProvider,
		Model:            modelName,
		Content:          completion.Content,
		SelectedBackend:  capabilities.LLMBackendRemote,
		ConstraintMode:   ConstraintModeNativeToolCall,
		Usage:            completion.Usage,
		UsedFallback:     servingNote != "",
		FallbackReason:   servingNote,
	}, true, nil
}

func withNativeSchemaLint(errorValue error, lintResult NativeSchemaLintResult) error {
	var slowServing slowServingError
	if lintResult.ToolCount == 0 || errors.As(errorValue, &slowServing) || errors.Is(errorValue, context.Canceled) || errors.Is(errorValue, context.DeadlineExceeded) {
		return errorValue
	}
	return errors.New(strings.TrimSpace(errorValue.Error()) + "; " + NativeSchemaLintDiagnostics(lintResult))
}

func nativeActionCompletionFromStream(response openAIResponseWithUsage, toolSet nativeActionToolSet) (nativeActionCompletion, error) {
	if len(response.Choices) == 0 {
		return nativeActionCompletion{}, errors.New("openrouter response did not include choices")
	}
	for _, toolCall := range response.Choices[0].Message.ToolCalls {
		if toolCall.Type == "" || toolCall.Type == "function" {
			content, errorValue := nativeActionJSON(toolSet, toolCall.Function.Name, toolCall.Function.Arguments)
			return nativeActionCompletion{Content: content, UpstreamProvider: response.Provider, Usage: normalizeUsage(response.Usage)}, errorValue
		}
	}
	return nativeActionCompletion{}, errors.New("openrouter chat completion response did not include tool_calls")
}

func (backend OpenRouterBackend) streamWithOneRetry(ctx context.Context, apiKey string, modelName string, buildRequest func(ignoredProviders []string) ([]byte, error)) (openAIResponseWithUsage, string, error) {
	requestDocument, errorValue := buildRequest(sharedServingRecord.ignoredProviders(modelName))
	if errorValue != nil {
		return openAIResponseWithUsage{}, "", errorValue
	}
	response, errorValue := backend.streamCompletion(ctx, apiKey, requestDocument, modelName)
	var slowServing slowServingError
	if !errors.As(errorValue, &slowServing) || ctx.Err() != nil {
		return response, "", errorValue
	}
	log.Printf("openrouter serving cut, asking again without it: model=%s %s", modelName, slowServing.Error())
	requestDocument, errorValue = buildRequest(sharedServingRecord.ignoredProviders(modelName))
	if errorValue != nil {
		return openAIResponseWithUsage{}, "", errorValue
	}
	response, errorValue = backend.streamCompletion(ctx, apiKey, requestDocument, modelName)
	return response, slowServing.Error(), errorValue
}

func (backend OpenRouterBackend) completeJSONSchema(ctx context.Context, apiKey string, request StructuredRequest, modelName string) (Response, error) {
	requestDocument, errorValue := backend.buildStructuredRequest(request, modelName)
	if errorValue != nil {
		return Response{}, errorValue
	}
	return backend.completeStructuredDocument(ctx, apiKey, requestDocument, modelName, ConstraintModeOpenAIJSONSchema)
}

func (backend OpenRouterBackend) completePromptedJSON(ctx context.Context, apiKey string, request StructuredRequest, modelName string) (Response, error) {
	requestDocument, errorValue := backend.buildPromptedStructuredRequest(request, modelName)
	if errorValue != nil {
		return Response{}, errorValue
	}
	return backend.completeStructuredDocument(ctx, apiKey, requestDocument, modelName, ConstraintModePromptedJSON)
}

func (backend OpenRouterBackend) completeStructuredDocument(ctx context.Context, apiKey string, requestDocument []byte, modelName string, constraintMode string) (Response, error) {
	completion, errorValue := backend.send(ctx, apiKey, requestDocument)
	if errorValue != nil {
		return Response{}, errorValue
	}
	if completion.FinishReason == "length" {
		return Response{}, structuredOutputLimitError{ModelName: modelName, Usage: completion.Usage}
	}
	content := normalizeStructuredJSONContent(completion.Content)
	if errorValue := validateStructuredJSONContent(content); errorValue != nil {
		return Response{}, errorValue
	}
	return Response{
		Provider:         "openrouter",
		UpstreamProvider: completion.UpstreamProvider,
		Model:            modelName,
		Content:          content,
		SelectedBackend:  capabilities.LLMBackendRemote,
		ConstraintMode:   constraintMode,
		Usage:            completion.Usage,
	}, nil
}

func (backend OpenRouterBackend) CompleteText(ctx context.Context, request TextRequest) (Response, error) {
	apiKey, errorValue := backend.resolveAPIKey()
	if errorValue != nil {
		return Response{}, errorValue
	}
	modelName := backend.resolveModelName(request.Model)
	requestDocument, errorValue := backend.buildTextRequest(request, modelName)
	if errorValue != nil {
		return Response{}, errorValue
	}
	completion, errorValue := backend.send(ctx, apiKey, requestDocument)
	if errorValue != nil {
		return Response{}, errorValue
	}
	return Response{
		Provider:         "openrouter",
		UpstreamProvider: completion.UpstreamProvider,
		Model:            modelName,
		Content:          completion.Content,
		SelectedBackend:  capabilities.LLMBackendRemote,
		Usage:            completion.Usage,
	}, nil
}

func (backend OpenRouterBackend) CompleteChat(ctx context.Context, request ChatRequest) (ChatResponse, error) {
	apiKey, errorValue := backend.resolveAPIKey()
	if errorValue != nil {
		return ChatResponse{}, errorValue
	}
	modelName := backend.resolveModelName(request.Model)
	chatRequest := openAIChatCompletionRequest(modelName, request)
	chatRequest.Reasoning = openAIReasoningFor(request.ReasoningEffort)
	chatRequest.Stream = true
	chatRequest.Usage = &openAIUsageOptions{Include: true}
	response, servingNote, errorValue := backend.streamWithOneRetry(ctx, apiKey, modelName, func(ignoredProviders []string) ([]byte, error) {
		chatRequest.Provider = backend.providerRoutingIgnoring(false, ignoredProviders)
		return json.Marshal(chatRequest)
	})
	if errorValue != nil {
		return ChatResponse{}, errorValue
	}
	if len(response.Choices) == 0 {
		return ChatResponse{}, errors.New("openrouter response did not include choices")
	}
	chatResponse := chatResponseFromOpenAI("openrouter", modelName, response)
	chatResponse.UsedFallback = servingNote != ""
	chatResponse.FallbackReason = servingNote
	return chatResponse, nil
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

func (backend OpenRouterBackend) actionModelNames(primaryModelName string) []string {
	candidates := []string{primaryModelName}
	candidates = append(candidates, backend.FallbackModelNames...)
	modelNames := uniqueModelNames(candidates)
	if len(modelNames) == 0 {
		return []string{strings.TrimSpace(primaryModelName)}
	}
	return modelNames
}

func uniqueModelNames(modelNames []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, modelName := range modelNames {
		normalized := strings.TrimSpace(modelName)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		result = append(result, normalized)
	}
	return result
}

func prefersPromptedStructuredJSON(modelName string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(modelName)), ":free")
}

type openRouterCompletion struct {
	Content          string
	FinishReason     string
	UpstreamProvider string
	Usage            Usage
}

type nativeActionCompletion struct {
	Content          string
	UpstreamProvider string
	Usage            Usage
}

type structuredOutputLimitError struct {
	ModelName string
	Usage     Usage
}

func (failure structuredOutputLimitError) Error() string {
	return fmt.Sprintf("structured response exceeded completion limit: model=%s finish_reason=length completion_tokens=%d reasoning_tokens=%d", failure.ModelName, failure.Usage.CompletionTokens, failure.Usage.ReasoningTokens)
}

func isStructuredOutputLimitError(errorValue error) bool {
	var failure structuredOutputLimitError
	return errors.As(errorValue, &failure)
}

func (backend OpenRouterBackend) send(ctx context.Context, apiKey string, requestDocument []byte) (openRouterCompletion, error) {
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, backend.BaseURL, bytes.NewReader(requestDocument))
	if errorValue != nil {
		return openRouterCompletion{}, errorValue
	}
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	backend.setGatewaySecretHeader(httpRequest)

	httpResponse, errorValue := backend.HTTPClient.Do(httpRequest)
	if errorValue != nil {
		return openRouterCompletion{}, errorValue
	}
	defer httpResponse.Body.Close()

	responseDocument, errorValue := io.ReadAll(httpResponse.Body)
	if errorValue != nil {
		return openRouterCompletion{}, errors.New("read openrouter response: " + errorValue.Error())
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return openRouterCompletion{}, normalizeProviderError("openrouter", httpResponse.StatusCode, responseDocument)
	}
	if len(bytes.TrimSpace(responseDocument)) == 0 {
		return openRouterCompletion{}, errors.New("openrouter response body was empty")
	}

	var parsed openAIResponseWithUsage
	if errorValue := json.Unmarshal(responseDocument, &parsed); errorValue != nil {
		return openRouterCompletion{}, errorValue
	}
	if len(parsed.Choices) == 0 {
		return openRouterCompletion{}, errors.New("openrouter response did not include choices")
	}
	return openRouterCompletion{
		Content:          parsed.Choices[0].Message.Content,
		FinishReason:     parsed.Choices[0].FinishReason,
		UpstreamProvider: parsed.Provider,
		Usage:            normalizeUsage(parsed.Usage),
	}, nil
}

func (backend OpenRouterBackend) setGatewaySecretHeader(request *http.Request) {
	gatewaySecret := strings.TrimSpace(readOpenRouterKey(backend.GatewaySecretPath))
	if gatewaySecret == "" {
		return
	}
	headerName := strings.TrimSpace(backend.GatewaySecretHeader)
	if headerName == "" {
		headerName = "X-INTERNKIM-GATEWAY-SECRET"
	}
	request.Header.Set(headerName, gatewaySecret)
}

// https://openrouter.ai/docs/features/provider-routing
func (backend OpenRouterBackend) providerRouting(requireParameters bool) map[string]any {
	providerSort := strings.TrimSpace(backend.ProviderSort)
	if providerSort == "" && len(backend.ProviderOrder) == 0 && !requireParameters {
		return nil
	}
	routing := map[string]any{"allow_fallbacks": true}
	if len(backend.ProviderOrder) > 0 {
		routing["order"] = append([]string{}, backend.ProviderOrder...)
	}
	if providerSort != "" {
		routing["sort"] = providerSort
	}
	if requireParameters {
		routing["require_parameters"] = true
	}
	return routing
}

func (backend OpenRouterBackend) providerRoutingIgnoring(requireParameters bool, ignoredProviders []string) map[string]any {
	routing := backend.providerRouting(requireParameters)
	if len(ignoredProviders) == 0 {
		return routing
	}
	if routing == nil {
		routing = map[string]any{"allow_fallbacks": true}
	}
	routing["ignore"] = append([]string{}, ignoredProviders...)
	return routing
}

func (backend OpenRouterBackend) applyServingPreferences(document map[string]any, requireParameters bool, reasoningEffort string) {
	if routing := backend.providerRouting(requireParameters); routing != nil {
		document["provider"] = routing
	}
	if reasoning := openAIReasoningFor(reasoningEffort); reasoning != nil {
		document["reasoning"] = reasoning
	}
}

func (backend OpenRouterBackend) buildStructuredRequest(request StructuredRequest, modelName string) ([]byte, error) {
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
	backend.applyServingPreferences(document, request.RequireParameters, request.ReasoningEffort)
	if request.EnableResponseHealing {
		document["plugins"] = []map[string]string{{"id": "response-healing"}}
	}
	addGenerationOptions(document, request.GenerationOptions)
	return json.Marshal(document)
}

func (backend OpenRouterBackend) buildPromptedStructuredRequest(request StructuredRequest, modelName string) ([]byte, error) {
	messages := append([]openAIMessage{}, openAIMessages(request.Messages)...)
	messages = append(messages, openAIMessage{
		Role:    "user",
		Content: promptedStructuredOutputInstruction(request.StructuredOutputSchema),
	})
	document := map[string]any{
		"model":    modelName,
		"messages": messages,
		"stream":   false,
	}
	backend.applyServingPreferences(document, request.RequireParameters, request.ReasoningEffort)
	if request.EnableResponseHealing {
		document["plugins"] = []map[string]string{{"id": "response-healing"}}
	}
	addGenerationOptions(document, request.GenerationOptions)
	return json.Marshal(document)
}

func promptedStructuredOutputInstruction(schema StructuredOutputSchema) string {
	schemaName := strings.TrimSpace(schema.Name)
	if schemaName == "" {
		schemaName = "structured_output"
	}
	schemaDocument := strings.TrimSpace(string(schema.Document))
	if schemaDocument == "" {
		return "Return only valid JSON for " + schemaName + ". Do not use Markdown or explanatory text."
	}
	return "Return only one valid JSON object for " + schemaName + ". Do not use Markdown or explanatory text. The JSON must satisfy this schema:\n" + schemaDocument
}

func normalizeStructuredJSONContent(content string) string {
	trimmedContent := strings.TrimSpace(content)
	if json.Valid([]byte(trimmedContent)) {
		return trimmedContent
	}
	fencedContent, hasFencedContent := fencedJSONContent(trimmedContent)
	if hasFencedContent && json.Valid([]byte(fencedContent)) {
		return fencedContent
	}
	return content
}

const emptyStructuredContentMessage = "structured response content was empty"

func isEmptyStructuredContentError(errorValue error) bool {
	return errorValue != nil && strings.Contains(errorValue.Error(), emptyStructuredContentMessage)
}

func validateStructuredJSONContent(content string) error {
	trimmedContent := strings.TrimSpace(content)
	if trimmedContent == "" {
		return errors.New(emptyStructuredContentMessage)
	}
	if !json.Valid([]byte(trimmedContent)) {
		return errors.New("structured response content was not valid json")
	}
	return nil
}

func fencedJSONContent(content string) (string, bool) {
	if !strings.HasPrefix(content, "```") {
		return "", false
	}
	lines := strings.Split(content, "\n")
	if len(lines) < 3 {
		return "", false
	}
	if !strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
		return "", false
	}
	return strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n")), true
}

func (backend OpenRouterBackend) chatActionDocument(request StructuredRequest, modelName string, tools []nativeActionTool, lintResults ...NativeSchemaLintResult) openAIRequest {
	document := openAIActionToolRequest(modelName, request.Messages, tools, generationOptionsValue(request.GenerationOptions), lintResults...)
	document.Provider = backend.providerRouting(request.RequireParameters)
	document.Reasoning = openAIReasoningFor(request.ReasoningEffort)
	document.Stream = true
	document.Usage = &openAIUsageOptions{Include: true}
	return document
}

func (backend OpenRouterBackend) buildChatActionRequest(request StructuredRequest, modelName string, tools []nativeActionTool, lintResults ...NativeSchemaLintResult) ([]byte, NativeSchemaLintResult, error) {
	document := backend.chatActionDocument(request, modelName, tools, lintResults...)
	content, errorValue := json.Marshal(document)
	return content, document.NativeToolSchemaLint, errorValue
}

func (backend OpenRouterBackend) buildTextRequest(request TextRequest, modelName string) ([]byte, error) {
	document := map[string]any{
		"model":    modelName,
		"messages": openAIMessages(request.Messages),
		"stream":   false,
	}
	backend.applyServingPreferences(document, request.RequireParameters, request.ReasoningEffort)
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
	if options.MaxTokens != nil {
		document["max_tokens"] = *options.MaxTokens
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

func (backend OpenRouterBackend) ContextWindowTokensForModel(modelName string) int64 {
	if tokens := sharedOpenRouterModelCatalog.contextWindowTokens(backend.modelsURL(), backend.HTTPClient, backend.resolveModelName(modelName)); tokens > 0 {
		return tokens
	}
	return DefaultContextWindowTokens
}

func (backend OpenRouterBackend) modelsURL() string {
	baseURL := strings.TrimSpace(backend.BaseURL)
	if index := strings.Index(baseURL, "/chat/completions"); index > 0 {
		return baseURL[:index] + "/models"
	}
	return "https://openrouter.ai/api/v1/models"
}
