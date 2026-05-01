package llmbackend

import (
	"context"
	"encoding/json"
	"errors"
)

type LiteRTBackend struct {
	ModelPath   string
	WrapperPath string
	Variant     string
	RunCommand  func(context.Context, string, []string, []byte) ([]byte, error)
}

type litertWrapperRequest struct {
	ModelPath              string                  `json:"modelPath"`
	Backend                string                  `json:"backend"`
	Mode                   string                  `json:"mode"`
	Messages               []Message               `json:"messages"`
	StructuredOutputSchema *StructuredOutputSchema `json:"structuredOutputSchema,omitempty"`
}

type litertWrapperResponse struct {
	Content string `json:"content"`
}

const litertModelLabel = "gemma-4-E4B-it-litert-lm"

func (backend LiteRTBackend) Name() string { return "litert-" + backend.Variant }

func (backend LiteRTBackend) Ping(context.Context) error {
	if backend.WrapperPath == "" || backend.ModelPath == "" {
		return errors.New("litert backend is not configured")
	}
	if backend.RunCommand == nil {
		return errors.New("litert backend has no run command")
	}
	return nil
}

func (backend LiteRTBackend) CompleteStructured(ctx context.Context, request StructuredRequest) (Response, error) {
	document, errorValue := json.Marshal(litertWrapperRequest{
		ModelPath:              backend.ModelPath,
		Backend:                backend.Variant,
		Mode:                   "structured",
		Messages:               request.Messages,
		StructuredOutputSchema: &request.StructuredOutputSchema,
	})
	if errorValue != nil {
		return Response{}, errorValue
	}

	output, errorValue := backend.RunCommand(ctx, backend.WrapperPath, nil, document)
	if errorValue != nil {
		return Response{}, errorValue
	}

	var response litertWrapperResponse
	if errorValue := json.Unmarshal(output, &response); errorValue != nil {
		return Response{}, errorValue
	}
	if !ValidateMinimumStructuredOutput(response.Content, request.StructuredOutputSchema.Document) {
		return Response{}, errors.New("litert response did not satisfy structured output schema")
	}
	return Response{
		Provider:        "litert",
		Model:           litertModelLabel,
		Content:         response.Content,
		SelectedBackend: backend.Variant,
		ConstraintMode:  "prompt_validation",
	}, nil
}

func (backend LiteRTBackend) CompleteText(ctx context.Context, request TextRequest) (Response, error) {
	document, errorValue := json.Marshal(litertWrapperRequest{
		ModelPath: backend.ModelPath,
		Backend:   backend.Variant,
		Mode:      "text",
		Messages:  request.Messages,
	})
	if errorValue != nil {
		return Response{}, errorValue
	}

	output, errorValue := backend.RunCommand(ctx, backend.WrapperPath, nil, document)
	if errorValue != nil {
		return Response{}, errorValue
	}

	var response litertWrapperResponse
	if errorValue := json.Unmarshal(output, &response); errorValue != nil {
		return Response{}, errorValue
	}
	return Response{
		Provider:        "litert",
		Model:           litertModelLabel,
		Content:         response.Content,
		SelectedBackend: backend.Variant,
	}, nil
}
