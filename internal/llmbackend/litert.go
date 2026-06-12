package llmbackend

import (
	"context"
	"encoding/json"
	"errors"
	"os"
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
const defaultLiteRTConstrainedRunnerBinaryPath = "/usr/local/bin/internkim-litert-constrained"

var LiteRTConstrainedRunnerBinaryPath = defaultLiteRTConstrainedRunnerBinaryPath

var errProviderUnavailable = errors.New("llm provider unavailable")

type providerUnavailableError struct {
	provider string
	reason   string
	cause    error
}

func (errorValue providerUnavailableError) Error() string {
	if errorValue.cause == nil {
		return errorValue.provider + " unavailable: " + errorValue.reason
	}
	return errorValue.provider + " unavailable: " + errorValue.reason + ": " + errorValue.cause.Error()
}

func (errorValue providerUnavailableError) Unwrap() error {
	return errorValue.cause
}

func (errorValue providerUnavailableError) Is(target error) bool {
	return target == errProviderUnavailable
}

func IsProviderUnavailable(errorValue error) bool {
	return errors.Is(errorValue, errProviderUnavailable)
}

func ProviderUnavailableReason(errorValue error) string {
	var unavailableError providerUnavailableError
	if !errors.As(errorValue, &unavailableError) {
		return ""
	}
	return unavailableError.reason
}

func (provider LiteRTProvider) Name() string { return "litert-" + provider.Variant }

func (provider LiteRTProvider) Ping(context.Context) error {
	if provider.RunnerPath == "" || provider.ModelPath == "" {
		return errors.New("litert provider is not configured")
	}
	if provider.RunCommand == nil {
		return errors.New("litert provider has no run command")
	}
	return liteRTConstrainedRunnerAvailabilityError()
}

func (provider LiteRTProvider) CompleteStructured(ctx context.Context, request StructuredRequest) (Response, error) {
	if errorValue := provider.Ping(ctx); errorValue != nil {
		return Response{}, errorValue
	}

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

func liteRTConstrainedRunnerAvailabilityError() error {
	fileInfo, errorValue := os.Stat(LiteRTConstrainedRunnerBinaryPath)
	if errorValue != nil {
		return providerUnavailableError{
			provider: "litert",
			reason:   "constrained runner not installed",
			cause:    errorValue,
		}
	}
	if fileInfo.IsDir() {
		return providerUnavailableError{
			provider: "litert",
			reason:   "constrained runner not installed",
			cause:    errors.New("path is a directory"),
		}
	}
	return nil
}

func providerUnavailableFailure(errorValue error) (string, bool) {
	var unavailableError providerUnavailableError
	if !errors.As(errorValue, &unavailableError) {
		return "", false
	}
	return unavailableError.provider + ": " + unavailableError.reason, true
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
