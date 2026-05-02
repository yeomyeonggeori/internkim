package llmbackend

import (
	"context"
	"encoding/json"
	"errors"
)

type LiteRTProvider struct {
	ModelPath  string
	RunnerPath string
	Variant    string
	RunCommand func(context.Context, string, []string, []byte) ([]byte, error)
}

type localLLMRunnerRequest struct {
	Provider            string               `json:"provider"`
	ModelPath           string               `json:"modelPath"`
	Accelerator         string               `json:"accelerator"`
	Mode                string               `json:"mode"`
	Messages            []Message            `json:"messages"`
	ConstrainedDecoding *constrainedDecoding `json:"constrainedDecoding,omitempty"`
}

type constrainedDecoding struct {
	Type       string                 `json:"type"`
	JSONSchema StructuredOutputSchema `json:"jsonSchema"`
}

type localLLMRunnerResponse struct {
	Content        string `json:"content"`
	ConstraintMode string `json:"constraintMode,omitempty"`
}

const litertModelLabel = "gemma-4-E4B-it-litert-lm"

func (provider LiteRTProvider) Name() string { return "litert-" + provider.Variant }

func (provider LiteRTProvider) Ping(context.Context) error {
	if provider.RunnerPath == "" || provider.ModelPath == "" {
		return errors.New("litert provider is not configured")
	}
	if provider.RunCommand == nil {
		return errors.New("litert provider has no run command")
	}
	return nil
}

func (provider LiteRTProvider) CompleteStructured(ctx context.Context, request StructuredRequest) (Response, error) {
	document, errorValue := json.Marshal(localLLMRunnerRequest{
		Provider:    "litert",
		ModelPath:   provider.ModelPath,
		Accelerator: provider.Variant,
		Mode:        "structured",
		Messages:    request.Messages,
		ConstrainedDecoding: &constrainedDecoding{
			Type:       "json_schema",
			JSONSchema: request.StructuredOutputSchema,
		},
	})
	if errorValue != nil {
		return Response{}, errorValue
	}

	output, errorValue := provider.RunCommand(ctx, provider.RunnerPath, nil, document)
	if errorValue != nil {
		return Response{}, errorValue
	}

	var response localLLMRunnerResponse
	if errorValue := json.Unmarshal(output, &response); errorValue != nil {
		return Response{}, errorValue
	}
	if !ValidateStructuredJSON(response.Content) {
		return Response{}, errors.New("litert response was not structured JSON")
	}
	constraintMode := response.ConstraintMode
	if constraintMode == "" {
		constraintMode = ConstraintModeLiteRTLLGuidanceJSONSchema
	}
	return Response{
		Provider:        "litert",
		Model:           litertModelLabel,
		Content:         response.Content,
		SelectedBackend: provider.Variant,
		ConstraintMode:  constraintMode,
	}, nil
}

func (provider LiteRTProvider) CompleteText(ctx context.Context, request TextRequest) (Response, error) {
	document, errorValue := json.Marshal(localLLMRunnerRequest{
		Provider:    "litert",
		ModelPath:   provider.ModelPath,
		Accelerator: provider.Variant,
		Mode:        "text",
		Messages:    request.Messages,
	})
	if errorValue != nil {
		return Response{}, errorValue
	}

	output, errorValue := provider.RunCommand(ctx, provider.RunnerPath, nil, document)
	if errorValue != nil {
		return Response{}, errorValue
	}

	var response localLLMRunnerResponse
	if errorValue := json.Unmarshal(output, &response); errorValue != nil {
		return Response{}, errorValue
	}
	return Response{
		Provider:        "litert",
		Model:           litertModelLabel,
		Content:         response.Content,
		SelectedBackend: provider.Variant,
	}, nil
}
