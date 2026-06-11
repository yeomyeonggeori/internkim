package llmbackend

import (
	"context"
	"errors"
	"net/http"
)

type MLXBackend struct {
	BaseURL    string
	ModelName  string
	HTTPClient *http.Client
}

func (backend MLXBackend) Name() string { return "mlx" }

func (backend MLXBackend) Ping(ctx context.Context) error {
	return backend.client().pingPath(ctx, "/v1/models")
}

func (backend MLXBackend) CompleteStructured(ctx context.Context, request StructuredRequest) (Response, error) {
	_ = ctx
	_ = request
	return Response{}, errors.New("mlx does not support verified deterministic structured output")
}

func (backend MLXBackend) CompleteText(ctx context.Context, request TextRequest) (Response, error) {
	chatRequest := openAIChatRequest(backend.ModelName, request.Messages, nil, GenerationOptions{})
	content, usage, errorValue := backend.client().chatCompletions(ctx, chatRequest)
	if errorValue != nil {
		return Response{}, errorValue
	}
	return Response{
		Provider:        "mlx",
		Model:           backend.ModelName,
		Content:         content,
		SelectedBackend: "mlx",
		Usage:           usage,
	}, nil
}

func (backend MLXBackend) client() openAICompatClient {
	return openAICompatClient{
		BaseURL:    backend.BaseURL,
		ModelName:  backend.ModelName,
		HTTPClient: backend.HTTPClient,
	}
}
