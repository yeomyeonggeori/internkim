package llmbackend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

type openAICompatClient struct {
	ProviderName string
	BaseURL      string
	ModelName    string
	HTTPClient   *http.Client
}

type openAIRequest struct {
	Model                string                 `json:"model"`
	Messages             []openAIMessage        `json:"messages"`
	Stream               bool                   `json:"stream"`
	ResponseFormat       *openAIJSONSchema      `json:"response_format,omitempty"`
	Tools                []openAITool           `json:"tools,omitempty"`
	ToolChoice           json.RawMessage        `json:"tool_choice,omitempty"`
	ParallelTools        *bool                  `json:"parallel_tool_calls,omitempty"`
	Seed                 *int64                 `json:"seed,omitempty"`
	Temperature          *float64               `json:"temperature,omitempty"`
	NativeToolSchemaLint NativeSchemaLintResult `json:"-"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    any              `json:"content"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
}

type openAIJSONSchema struct {
	Type       string                  `json:"type"`
	JSONSchema openAIJSONSchemaPayload `json:"json_schema"`
}

type openAIJSONSchemaPayload struct {
	Name   string          `json:"name"`
	Strict bool            `json:"strict"`
	Schema json.RawMessage `json:"schema"`
}

type openAIUsage struct {
	PromptTokens        int64 `json:"prompt_tokens"`
	CompletionTokens    int64 `json:"completion_tokens"`
	TotalTokens         int64 `json:"total_tokens"`
	PromptTokensDetails struct {
		CachedTokens     int64 `json:"cached_tokens"`
		CacheWriteTokens int64 `json:"cache_write_tokens"`
	} `json:"prompt_tokens_details"`
	CompletionTokensDetails struct {
		ReasoningTokens int64 `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
	Cost        float64 `json:"cost"`
	CostDetails struct {
		UpstreamInferenceCost float64 `json:"upstream_inference_cost"`
	} `json:"cost_details"`
}

type openAIResponse struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role      string           `json:"role"`
			Content   string           `json:"content"`
			ToolCalls []openAIToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}

type openAIResponseWithUsage struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role      string           `json:"role"`
			Content   string           `json:"content"`
			ToolCalls []openAIToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage openAIUsage `json:"usage"`
}

type openAITool struct {
	Type     string         `json:"type"`
	Function openAIFunction `json:"function"`
}

type openAIFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

type openAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func (client openAICompatClient) chatCompletions(ctx context.Context, request openAIRequest) (string, Usage, error) {
	response, errorValue := client.chatCompletionResponse(ctx, request)
	if errorValue != nil {
		return "", Usage{}, errorValue
	}
	content := response.Choices[0].Message.Content
	if strings.TrimSpace(content) == "" {
		return "", Usage{}, errors.New("response content was empty")
	}
	return content, normalizeUsage(response.Usage), nil
}

func (client openAICompatClient) chatCompletionAction(ctx context.Context, request openAIRequest, toolSet nativeActionToolSet) (string, Usage, error) {
	response, errorValue := client.chatCompletionResponse(ctx, request)
	if errorValue != nil {
		return "", Usage{}, errorValue
	}
	for _, toolCall := range response.Choices[0].Message.ToolCalls {
		if toolCall.Type == "" || toolCall.Type == "function" {
			content, errorValue := nativeActionJSON(toolSet, toolCall.Function.Name, toolCall.Function.Arguments)
			return content, normalizeUsage(response.Usage), errorValue
		}
	}
	return "", Usage{}, errors.New("chat completion response did not include tool_calls")
}

func (client openAICompatClient) chatCompletion(ctx context.Context, request ChatRequest) (ChatResponse, error) {
	modelName := firstNonEmpty(request.Model, client.ModelName)
	chatRequest := openAIChatCompletionRequest(modelName, request)
	response, errorValue := client.chatCompletionResponse(ctx, chatRequest)
	if errorValue != nil {
		return ChatResponse{}, errorValue
	}
	return chatResponseFromOpenAI(client.providerName(), modelName, response), nil
}

const openAIMaxRetryAttempts = 5

func (client openAICompatClient) chatCompletionResponse(ctx context.Context, request openAIRequest) (openAIResponseWithUsage, error) {
	requestDocument, errorValue := json.Marshal(request)
	if errorValue != nil {
		return openAIResponseWithUsage{}, errorValue
	}
	endpoint := strings.TrimRight(client.BaseURL, "/") + "/v1/chat/completions"

	var lastError error
	for attempt := 0; attempt < openAIMaxRetryAttempts; attempt++ {
		if attempt > 0 {
			if errorValue := sleepWithContext(ctx, openAIRetryBackoff(attempt)); errorValue != nil {
				return openAIResponseWithUsage{}, errorValue
			}
		}

		httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestDocument))
		if errorValue != nil {
			return openAIResponseWithUsage{}, errorValue
		}
		httpRequest.Header.Set("Content-Type", "application/json")

		httpResponse, errorValue := client.client().Do(httpRequest)
		if errorValue != nil {
			lastError = errorValue
			continue
		}
		responseDocument, readError := io.ReadAll(httpResponse.Body)
		httpResponse.Body.Close()
		if readError != nil {
			lastError = errors.New("read response: " + readError.Error())
			continue
		}

		if httpResponse.StatusCode >= http.StatusBadRequest {
			responseError := openAIErrorWithNativeSchemaLint(client.providerName(), httpResponse.StatusCode, responseDocument, request.NativeToolSchemaLint)
			if isRetryableUpstreamStatus(httpResponse.StatusCode) {
				lastError = responseError
				continue
			}
			return openAIResponseWithUsage{}, responseError
		}

		var response openAIResponseWithUsage
		if errorValue := json.Unmarshal(responseDocument, &response); errorValue != nil {
			return openAIResponseWithUsage{}, errorValue
		}
		if len(response.Choices) == 0 {
			return openAIResponseWithUsage{}, errors.New("response did not include choices")
		}
		return response, nil
	}
	return openAIResponseWithUsage{}, lastError
}

func isRetryableUpstreamStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode == http.StatusBadGateway ||
		statusCode == http.StatusServiceUnavailable || statusCode == http.StatusGatewayTimeout
}

func openAIRetryBackoff(attempt int) time.Duration {
	backoff := time.Duration(1<<uint(attempt-1)) * time.Second
	if backoff > 16*time.Second {
		backoff = 16 * time.Second
	}
	return backoff
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (client openAICompatClient) pingPath(ctx context.Context, path string) error {
	endpoint := strings.TrimRight(client.BaseURL, "/") + path
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if errorValue != nil {
		return errorValue
	}
	httpResponse, errorValue := client.client().Do(httpRequest)
	if errorValue != nil {
		return errorValue
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return errors.New("backend returned " + httpResponse.Status)
	}
	return nil
}

func (client openAICompatClient) client() *http.Client {
	if client.HTTPClient == nil {
		return http.DefaultClient
	}
	return client.HTTPClient
}

func (client openAICompatClient) providerName() string {
	if strings.TrimSpace(client.ProviderName) == "" {
		return "openai-compatible"
	}
	return strings.TrimSpace(client.ProviderName)
}

func openAIChatRequest(modelName string, messages []Message, schema *StructuredOutputSchema, options GenerationOptions) openAIRequest {
	request := openAIRequest{
		Model:       modelName,
		Messages:    openAIMessages(messages),
		Stream:      false,
		Seed:        options.Seed,
		Temperature: options.Temperature,
	}
	if schema != nil {
		request.ResponseFormat = &openAIJSONSchema{
			Type: "json_schema",
			JSONSchema: openAIJSONSchemaPayload{
				Name:   schema.Name,
				Strict: schema.IsStrictlyEnforced,
				Schema: schema.Document,
			},
		}
	}
	return request
}

func openAIActionToolRequest(modelName string, messages []Message, tools []nativeActionTool, options GenerationOptions, lintResults ...NativeSchemaLintResult) openAIRequest {
	parallelTools := false
	normalizedTools, lintResult := NormalizeNativeActionToolSchemas(tools)
	if len(lintResults) > 0 {
		lintResult = mergeOpenAIActionToolLintResults(lintResults[0], lintResult)
	}
	return openAIRequest{
		Model:                modelName,
		Messages:             openAIMessages(messages),
		Stream:               false,
		Tools:                openAIActionTools(normalizedTools),
		ToolChoice:           json.RawMessage(`"required"`),
		ParallelTools:        &parallelTools,
		Seed:                 options.Seed,
		Temperature:          options.Temperature,
		NativeToolSchemaLint: lintResult,
	}
}

func openAIChatCompletionRequest(modelName string, request ChatRequest) openAIRequest {
	parallelToolCalls := request.ParallelToolCalls
	return openAIRequest{
		Model:         modelName,
		Messages:      openAIChatMessages(request.Messages),
		Stream:        false,
		Tools:         openAIChatTools(request.Tools),
		ToolChoice:    request.ToolChoice,
		ParallelTools: &parallelToolCalls,
	}
}

func mergeOpenAIActionToolLintResults(firstPass NativeSchemaLintResult, requestPass NativeSchemaLintResult) NativeSchemaLintResult {
	result := requestPass
	result.NormalizationsApplied = append(append([]string{}, firstPass.NormalizationsApplied...), requestPass.NormalizationsApplied...)
	result.RemainingViolations = append(append([]string{}, firstPass.RemainingViolations...), requestPass.RemainingViolations...)
	return result
}

func openAIErrorWithNativeSchemaLint(providerName string, httpStatus int, rawBody []byte, lintResult NativeSchemaLintResult) error {
	message := normalizedProviderErrorMessage(providerName, httpStatus, rawBody)
	if lintResult.ToolCount == 0 {
		return errors.New(message)
	}
	return errors.New(strings.TrimSpace(message) + "; " + NativeSchemaLintDiagnostics(lintResult))
}

func openAIMessages(messages []Message) []openAIMessage {
	result := make([]openAIMessage, 0, len(messages))
	for _, message := range messages {
		result = append(result, openAIMessage{
			Role:    message.Role,
			Content: openAIMessageContent(message),
		})
	}
	return result
}

func openAIChatMessages(messages []ChatMessage) []openAIMessage {
	result := make([]openAIMessage, 0, len(messages))
	for _, message := range messages {
		result = append(result, openAIMessage{
			Role:       message.Role,
			Content:    message.Content,
			ToolCallID: message.ToolCallID,
			ToolCalls:  openAIToolCalls(message.ToolCalls),
		})
	}
	return result
}

func openAIMessageContent(message Message) any {
	if len(message.Parts) == 0 {
		return message.Content
	}
	parts := []map[string]any{}
	if strings.TrimSpace(message.Content) != "" {
		parts = append(parts, map[string]any{
			"type": "text",
			"text": message.Content,
		})
	}
	for _, part := range message.Parts {
		switch strings.TrimSpace(part.Type) {
		case "text":
			if strings.TrimSpace(part.Text) != "" {
				parts = append(parts, map[string]any{
					"type": "text",
					"text": part.Text,
				})
			}
		case "image":
			if strings.TrimSpace(part.DataBase64) != "" && strings.TrimSpace(part.MimeType) != "" {
				parts = append(parts, map[string]any{
					"type": "image_url",
					"image_url": map[string]string{
						"url": "data:" + strings.TrimSpace(part.MimeType) + ";base64," + strings.TrimSpace(part.DataBase64),
					},
				})
			}
		}
	}
	if len(parts) == 0 {
		return message.Content
	}
	return parts
}

func normalizeUsage(raw openAIUsage) Usage {
	totalTokens := raw.TotalTokens
	if totalTokens == 0 {
		totalTokens = raw.PromptTokens + raw.CompletionTokens
	}
	return Usage{
		PromptTokens:          raw.PromptTokens,
		CompletionTokens:      raw.CompletionTokens,
		TotalTokens:           totalTokens,
		CachedPromptTokens:    raw.PromptTokensDetails.CachedTokens,
		CacheWriteTokens:      raw.PromptTokensDetails.CacheWriteTokens,
		ReasoningTokens:       raw.CompletionTokensDetails.ReasoningTokens,
		CostUSD:               raw.Cost,
		UpstreamInferenceCost: raw.CostDetails.UpstreamInferenceCost,
	}
}

func openAIActionTools(tools []nativeActionTool) []openAITool {
	result := make([]openAITool, 0, len(tools))
	for _, tool := range tools {
		result = append(result, openAITool{
			Type: "function",
			Function: openAIFunction{
				Name:        tool.FunctionName,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}
	return result
}

func openAIChatTools(tools []ChatTool) []openAITool {
	result := make([]openAITool, 0, len(tools))
	for _, tool := range tools {
		result = append(result, openAITool{
			Type: tool.Type,
			Function: openAIFunction{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  tool.Function.Parameters,
			},
		})
	}
	return result
}

func openAIToolCalls(toolCalls []ChatToolCall) []openAIToolCall {
	result := make([]openAIToolCall, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		result = append(result, openAIToolCall{
			ID:   toolCall.ID,
			Type: toolCall.Type,
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      toolCall.Function.Name,
				Arguments: toolCall.Function.Arguments,
			},
		})
	}
	return result
}

func chatResponseFromOpenAI(providerName string, modelName string, response openAIResponseWithUsage) ChatResponse {
	choice := response.Choices[0]
	role := firstNonEmpty(choice.Message.Role, "assistant")
	return ChatResponse{
		FinishReason: choice.FinishReason,
		Provider:     providerName,
		Model:        modelName,
		Message: ChatResponseMessage{
			Role:      role,
			Content:   choice.Message.Content,
			ToolCalls: chatToolCallsFromOpenAI(choice.Message.ToolCalls),
		},
		Usage: normalizeUsage(response.Usage),
	}
}

func chatToolCallsFromOpenAI(toolCalls []openAIToolCall) []ChatToolCall {
	result := make([]ChatToolCall, 0, len(toolCalls))
	for _, toolCall := range toolCalls {
		result = append(result, ChatToolCall{
			ID:   toolCall.ID,
			Type: toolCall.Type,
			Function: ChatToolCallFunction{
				Name:      toolCall.Function.Name,
				Arguments: toolCall.Function.Arguments,
			},
		})
	}
	return result
}
