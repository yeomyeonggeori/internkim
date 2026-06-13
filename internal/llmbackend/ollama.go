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

type OllamaBackend struct {
	BaseURL    string
	ModelName  string
	HTTPClient *http.Client
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaRequest struct {
	Model    string          `json:"model"`
	Stream   bool            `json:"stream"`
	Messages []ollamaMessage `json:"messages"`
	Format   json.RawMessage `json:"format,omitempty"`
}

type ollamaResponse struct {
	Message         ollamaMessage `json:"message"`
	Done            bool          `json:"done"`
	PromptEvalCount int64         `json:"prompt_eval_count"`
	EvalCount       int64         `json:"eval_count"`
}

func (backend OllamaBackend) Name() string { return "ollama" }

func (backend OllamaBackend) Ping(ctx context.Context) error {
	endpoint := strings.TrimRight(backend.BaseURL, "/") + "/api/tags"
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if errorValue != nil {
		return errorValue
	}
	httpResponse, errorValue := backend.client().Do(httpRequest)
	if errorValue != nil {
		return errorValue
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return errors.New("ollama returned " + httpResponse.Status)
	}
	return nil
}

func (backend OllamaBackend) CompleteStructured(ctx context.Context, request StructuredRequest) (Response, error) {
	_ = ctx
	_ = request
	return Response{}, errors.New("ollama does not support verified deterministic structured output")
}

func (backend OllamaBackend) CompleteText(ctx context.Context, request TextRequest) (Response, error) {
	content, modelName, usage, errorValue := backend.complete(ctx, request.Messages, nil)
	if errorValue != nil {
		return Response{}, errorValue
	}
	return Response{
		Provider:        "ollama",
		Model:           modelName,
		Content:         content,
		SelectedBackend: "ollama",
		Usage:           usage,
	}, nil
}

func (backend OllamaBackend) complete(ctx context.Context, messages []Message, format json.RawMessage) (string, string, Usage, error) {
	modelName := strings.TrimSpace(backend.ModelName)
	requestDocument, errorValue := json.Marshal(ollamaRequest{
		Model:    modelName,
		Stream:   false,
		Messages: ollamaMessagesFrom(messages),
		Format:   format,
	})
	if errorValue != nil {
		return "", "", Usage{}, errorValue
	}

	endpoint := strings.TrimRight(backend.BaseURL, "/") + "/api/chat"
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestDocument))
	if errorValue != nil {
		return "", "", Usage{}, errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, errorValue := backend.client().Do(httpRequest)
	if errorValue != nil {
		return "", "", Usage{}, errorValue
	}
	defer httpResponse.Body.Close()

	responseDocument, errorValue := io.ReadAll(httpResponse.Body)
	if errorValue != nil {
		return "", "", Usage{}, errors.New("read ollama response: " + errorValue.Error())
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return "", "", Usage{}, normalizeProviderError("ollama", httpResponse.StatusCode, responseDocument)
	}

	var response ollamaResponse
	if errorValue := json.Unmarshal(responseDocument, &response); errorValue != nil {
		return "", "", Usage{}, errorValue
	}
	if strings.TrimSpace(response.Message.Content) == "" {
		return "", "", Usage{}, errors.New("ollama response content was empty")
	}
	usage := Usage{
		PromptTokens:     response.PromptEvalCount,
		CompletionTokens: response.EvalCount,
		TotalTokens:      response.PromptEvalCount + response.EvalCount,
	}
	return response.Message.Content, modelName, usage, nil
}

func (backend OllamaBackend) StreamText(ctx context.Context, request TextRequest, emit func(token string)) error {
	modelName := strings.TrimSpace(backend.ModelName)
	requestDocument, errorValue := json.Marshal(ollamaRequest{
		Model:    modelName,
		Stream:   true,
		Messages: ollamaMessagesFrom(request.Messages),
	})
	if errorValue != nil {
		return errorValue
	}

	endpoint := strings.TrimRight(backend.BaseURL, "/") + "/api/chat"
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestDocument))
	if errorValue != nil {
		return errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, errorValue := backend.client().Do(httpRequest)
	if errorValue != nil {
		return errorValue
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(httpResponse.Body)
		return normalizeProviderError("ollama", httpResponse.StatusCode, body)
	}

	decoder := json.NewDecoder(httpResponse.Body)
	for decoder.More() {
		var chunk ollamaResponse
		if errorValue := decoder.Decode(&chunk); errorValue != nil {
			return errorValue
		}
		if chunk.Message.Content != "" {
			emit(chunk.Message.Content)
		}
		if chunk.Done {
			return nil
		}
	}
	return nil
}

func (backend OllamaBackend) client() *http.Client {
	if backend.HTTPClient == nil {
		return http.DefaultClient
	}
	return backend.HTTPClient
}

func ollamaMessagesFrom(messages []Message) []ollamaMessage {
	converted := make([]ollamaMessage, 0, len(messages))
	for _, message := range messages {
		converted = append(converted, ollamaMessage{
			Role:    message.Role,
			Content: textOnlyMessageContent(message),
		})
	}
	return converted
}

func textOnlyMessageContent(message Message) string {
	parts := []string{}
	if strings.TrimSpace(message.Content) != "" {
		parts = append(parts, message.Content)
	}
	for _, part := range message.Parts {
		switch strings.TrimSpace(part.Type) {
		case "text":
			if strings.TrimSpace(part.Text) != "" {
				parts = append(parts, part.Text)
			}
		case "image":
			parts = append(parts, "[image input omitted by this backend]")
		}
	}
	return strings.Join(parts, "\n")
}
