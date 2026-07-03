package llmbackend

import (
	"context"
	"errors"
	"net/http"
)

type LlamaCppBackend struct {
	BaseURL    string
	ModelName  string
	HTTPClient *http.Client
}

func (backend LlamaCppBackend) Name() string { return "llamacpp" }

func (backend LlamaCppBackend) Ping(ctx context.Context) error {
	return backend.client().pingPath(ctx, "/health")
}

func (backend LlamaCppBackend) CompleteStructured(ctx context.Context, request StructuredRequest) (Response, error) {
	if errorValue := promptExceedsContextWindow(request.Messages, LlamaCppLocalContextWindowTokens); errorValue != nil {
		return Response{}, errorValue
	}
	if response, isHandled, errorValue := backend.completeNativeAction(ctx, request); isHandled {
		if errorValue == nil {
			return response, nil
		}
		fallbackResponse, fallbackError := backend.completeJSONSchema(ctx, request)
		if fallbackError == nil {
			return fallbackResponse, nil
		}
		return Response{}, nativeActionFallbackError(errorValue, fallbackError)
	}
	return backend.completeJSONSchema(ctx, request)
}

func (backend LlamaCppBackend) completeJSONSchema(ctx context.Context, request StructuredRequest) (Response, error) {
	chatRequest := openAIChatRequest(backend.ModelName, request.Messages, &request.StructuredOutputSchema, generationOptionsValue(request.GenerationOptions))
	content, usage, errorValue := backend.client().chatCompletions(ctx, chatRequest)
	if errorValue != nil {
		return Response{}, errorValue
	}
	if !ValidateStructuredJSON(content) {
		return Response{}, errors.New("llamacpp response was not structured JSON")
	}
	return Response{
		Provider:        "llamacpp",
		Model:           backend.ModelName,
		Content:         content,
		SelectedBackend: "llamacpp",
		ConstraintMode:  ConstraintModeLlamaJSONSchema,
		Usage:           usage,
	}, nil
}

func (backend LlamaCppBackend) completeNativeAction(ctx context.Context, request StructuredRequest) (Response, bool, error) {
	toolSet, isActionSchema, errorValue := nativeActionToolsForSchema(request.StructuredOutputSchema)
	if errorValue != nil || !isActionSchema {
		return Response{}, isActionSchema, errorValue
	}
	chatRequest := openAIActionToolRequest(backend.ModelName, request.Messages, toolSet.Tools, generationOptionsValue(request.GenerationOptions))
	content, usage, errorValue := backend.client().chatCompletionAction(ctx, chatRequest, toolSet)
	if errorValue != nil {
		return Response{}, true, errorValue
	}
	return Response{
		Provider:        "llamacpp",
		Model:           backend.ModelName,
		Content:         content,
		SelectedBackend: "llamacpp",
		ConstraintMode:  ConstraintModeNativeToolCall,
		Usage:           usage,
	}, true, nil
}

func (backend LlamaCppBackend) CompleteText(ctx context.Context, request TextRequest) (Response, error) {
	if errorValue := promptExceedsContextWindow(request.Messages, LlamaCppLocalContextWindowTokens); errorValue != nil {
		return Response{}, errorValue
	}
	chatRequest := openAIChatRequest(backend.ModelName, request.Messages, nil, GenerationOptions{})
	content, usage, errorValue := backend.client().chatCompletions(ctx, chatRequest)
	if errorValue != nil {
		return Response{}, errorValue
	}
	return Response{
		Provider:        "llamacpp",
		Model:           backend.ModelName,
		Content:         content,
		SelectedBackend: "llamacpp",
		Usage:           usage,
	}, nil
}

func (backend LlamaCppBackend) CompleteChat(ctx context.Context, request ChatRequest) (ChatResponse, error) {
	return backend.client().chatCompletion(ctx, request)
}

func (backend LlamaCppBackend) client() openAICompatClient {
	return openAICompatClient{
		ProviderName: "llamacpp",
		BaseURL:      backend.BaseURL,
		ModelName:    backend.ModelName,
		HTTPClient:   backend.HTTPClient,
	}
}
