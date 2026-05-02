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
	KeyPath    string
	BaseURL    string
	ModelName  string
	HTTPClient *http.Client
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
	requestDocument, errorValue := buildOpenRouterStructuredRequest(request, modelName)
	if errorValue != nil {
		return Response{}, errorValue
	}
	content, errorValue := backend.send(ctx, apiKey, requestDocument)
	if errorValue != nil {
		return Response{}, errorValue
	}
	return Response{
		Provider:        "openrouter",
		Model:           modelName,
		Content:         content,
		SelectedBackend: capabilities.LLMBackendRemote,
		ConstraintMode:  ConstraintModeOpenAIJSONSchema,
	}, nil
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
	content, errorValue := backend.send(ctx, apiKey, requestDocument)
	if errorValue != nil {
		return Response{}, errorValue
	}
	return Response{
		Provider:        "openrouter",
		Model:           modelName,
		Content:         content,
		SelectedBackend: capabilities.LLMBackendRemote,
	}, nil
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

func (backend OpenRouterBackend) send(ctx context.Context, apiKey string, requestDocument []byte) (string, error) {
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, backend.BaseURL, bytes.NewReader(requestDocument))
	if errorValue != nil {
		return "", errorValue
	}
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")

	httpResponse, errorValue := backend.HTTPClient.Do(httpRequest)
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

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if errorValue := json.Unmarshal(responseDocument, &parsed); errorValue != nil {
		return "", errorValue
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("openrouter response did not include choices")
	}
	return parsed.Choices[0].Message.Content, nil
}

func buildOpenRouterStructuredRequest(request StructuredRequest, modelName string) ([]byte, error) {
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

func buildOpenRouterTextRequest(request TextRequest, modelName string) ([]byte, error) {
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
