package llmbackend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

type openAICompatClient struct {
	BaseURL    string
	ModelName  string
	HTTPClient *http.Client
}

type openAIRequest struct {
	Model          string            `json:"model"`
	Messages       []openAIMessage   `json:"messages"`
	Stream         bool              `json:"stream"`
	ResponseFormat *openAIJSONSchema `json:"response_format,omitempty"`
	Tools          []openAITool      `json:"tools,omitempty"`
	ToolChoice     string            `json:"tool_choice,omitempty"`
	ParallelTools  *bool             `json:"parallel_tool_calls,omitempty"`
	Seed           *int64            `json:"seed,omitempty"`
	Temperature    *float64          `json:"temperature,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
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

type openAIResponse struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content   string           `json:"content"`
			ToolCalls []openAIToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
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

func (client openAICompatClient) chatCompletions(ctx context.Context, request openAIRequest) (string, error) {
	response, errorValue := client.chatCompletionResponse(ctx, request)
	if errorValue != nil {
		return "", errorValue
	}
	content := response.Choices[0].Message.Content
	if strings.TrimSpace(content) == "" {
		return "", errors.New("response content was empty")
	}
	return content, nil
}

func (client openAICompatClient) chatCompletionAction(ctx context.Context, request openAIRequest, toolSet nativeActionToolSet) (string, error) {
	response, errorValue := client.chatCompletionResponse(ctx, request)
	if errorValue != nil {
		return "", errorValue
	}
	for _, toolCall := range response.Choices[0].Message.ToolCalls {
		if toolCall.Type == "" || toolCall.Type == "function" {
			return nativeActionJSON(toolSet, toolCall.Function.Name, toolCall.Function.Arguments)
		}
	}
	return "", errors.New("chat completion response did not include tool_calls")
}

func (client openAICompatClient) chatCompletionResponse(ctx context.Context, request openAIRequest) (openAIResponse, error) {
	requestDocument, errorValue := json.Marshal(request)
	if errorValue != nil {
		return openAIResponse{}, errorValue
	}

	endpoint := strings.TrimRight(client.BaseURL, "/") + "/v1/chat/completions"
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestDocument))
	if errorValue != nil {
		return openAIResponse{}, errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, errorValue := client.client().Do(httpRequest)
	if errorValue != nil {
		return openAIResponse{}, errorValue
	}
	defer httpResponse.Body.Close()

	responseDocument, errorValue := io.ReadAll(httpResponse.Body)
	if errorValue != nil {
		return openAIResponse{}, errors.New("read response: " + errorValue.Error())
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return openAIResponse{}, errors.New(string(responseDocument))
	}

	var response openAIResponse
	if errorValue := json.Unmarshal(responseDocument, &response); errorValue != nil {
		return openAIResponse{}, errorValue
	}
	if len(response.Choices) == 0 {
		return openAIResponse{}, errors.New("response did not include choices")
	}
	return response, nil
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

func openAIActionToolRequest(modelName string, messages []Message, tools []nativeActionTool, options GenerationOptions) openAIRequest {
	parallelTools := false
	return openAIRequest{
		Model:         modelName,
		Messages:      openAIMessages(messages),
		Stream:        false,
		Tools:         openAIActionTools(tools),
		ToolChoice:    "required",
		ParallelTools: &parallelTools,
		Seed:          options.Seed,
		Temperature:   options.Temperature,
	}
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
