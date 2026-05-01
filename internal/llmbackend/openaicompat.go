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
	Model          string             `json:"model"`
	Messages       []Message          `json:"messages"`
	Stream         bool               `json:"stream"`
	ResponseFormat *openAIJSONSchema  `json:"response_format,omitempty"`
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
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (client openAICompatClient) chatCompletions(ctx context.Context, request openAIRequest) (string, error) {
	requestDocument, errorValue := json.Marshal(request)
	if errorValue != nil {
		return "", errorValue
	}

	endpoint := strings.TrimRight(client.BaseURL, "/") + "/v1/chat/completions"
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestDocument))
	if errorValue != nil {
		return "", errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, errorValue := client.client().Do(httpRequest)
	if errorValue != nil {
		return "", errorValue
	}
	defer httpResponse.Body.Close()

	responseDocument, errorValue := io.ReadAll(httpResponse.Body)
	if errorValue != nil {
		return "", errors.New("read response: " + errorValue.Error())
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return "", errors.New(string(responseDocument))
	}

	var response openAIResponse
	if errorValue := json.Unmarshal(responseDocument, &response); errorValue != nil {
		return "", errorValue
	}
	if len(response.Choices) == 0 {
		return "", errors.New("response did not include choices")
	}
	content := response.Choices[0].Message.Content
	if strings.TrimSpace(content) == "" {
		return "", errors.New("response content was empty")
	}
	return content, nil
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

func openAIChatRequest(modelName string, messages []Message, schema *StructuredOutputSchema) openAIRequest {
	request := openAIRequest{
		Model:    modelName,
		Messages: messages,
		Stream:   false,
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
