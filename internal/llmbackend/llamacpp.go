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
	chatRequest := openAIChatRequest(backend.ModelName, request.Messages, &request.StructuredOutputSchema)
	content, errorValue := backend.client().chatCompletions(ctx, chatRequest)
	if errorValue != nil {
		return Response{}, errorValue
	}
	if !ValidateMinimumStructuredOutput(content, request.StructuredOutputSchema.Document) {
		return Response{}, errors.New("llamacpp response did not satisfy structured output schema")
	}
	return Response{
		Provider:        "llamacpp",
		Model:           backend.ModelName,
		Content:         content,
		SelectedBackend: "llamacpp",
		ConstraintMode:  "provider_json_schema",
	}, nil
}

func (backend LlamaCppBackend) CompleteText(ctx context.Context, request TextRequest) (Response, error) {
	chatRequest := openAIChatRequest(backend.ModelName, request.Messages, nil)
	content, errorValue := backend.client().chatCompletions(ctx, chatRequest)
	if errorValue != nil {
		return Response{}, errorValue
	}
	return Response{
		Provider:        "llamacpp",
		Model:           backend.ModelName,
		Content:         content,
		SelectedBackend: "llamacpp",
	}, nil
}

func (backend LlamaCppBackend) client() openAICompatClient {
	return openAICompatClient{
		BaseURL:    backend.BaseURL,
		ModelName:  backend.ModelName,
		HTTPClient: backend.HTTPClient,
	}
}
