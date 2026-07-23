package capabilityd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/internal/llmbackend"
)

const maximumArtifactReviewEvidenceCount = 8
const maximumArtifactReviewImageBytes = 8 * 1024 * 1024

type artifactReviewInput struct {
	ArtifactKind   string                     `json:"artifactKind"`
	Intent         string                     `json:"intent"`
	Rubric         string                     `json:"rubric"`
	Evidence       []artifactReviewEvidence   `json:"evidence"`
	ExpectedText   []artifactReviewTextItem   `json:"expectedText,omitempty"`
	PreviousIssues []artifactReviewIssueInput `json:"previousIssues,omitempty"`
}

type artifactReviewEvidence struct {
	Role     string `json:"role"`
	Path     string `json:"path"`
	MimeType string `json:"mimeType"`
	Label    string `json:"label"`
}

type artifactReviewTextItem struct {
	Target string `json:"target"`
	Text   string `json:"text"`
}

type artifactReviewIssueInput struct {
	Severity     string `json:"severity"`
	Category     string `json:"category"`
	Target       string `json:"target"`
	Message      string `json:"message"`
	SuggestedFix string `json:"suggestedFix"`
}

func (service Service) invokeArtifactReviewTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeArtifactReviewInput(request.Input)
	if errorValue != nil {
		return artifactReviewErrorResponse(request.ToolName, errorValue.Error(), "invalid_input", "input_validation", false), nil
	}
	if service.Configuration.WithDefaults().LocalOnly {
		return artifactReviewErrorResponse(request.ToolName, "remote artifact review is disabled in local-only mode", "local_only", "openrouter_configuration", false), nil
	}
	messages, errorValue := service.artifactReviewMessages(input)
	if errorValue != nil {
		return artifactReviewErrorResponse(request.ToolName, errorValue.Error(), "invalid_evidence", "evidence_loading", false), nil
	}
	response, errorValue := service.completeStructured(ctx, StructuredLLMRequest{
		Model:         service.Configuration.WithDefaults().OpenRouterModel,
		ExecutionMode: "remote",
		Context: llmbackend.RequestContext{
			RequesterPersonID:       request.Context.RequesterPersonID,
			RequesterEmail:          request.Context.RequesterEmail,
			RequesterName:           request.Context.RequesterName,
			RequesterPlatformUserID: request.Context.RequesterPlatformUserID,
			ConversationID:          request.Context.ConversationID,
			Platform:                request.Context.Platform,
		},
		Messages: messages,
		StructuredOutputSchema: StructuredOutputSchema{
			Name:               "artifact_review",
			Document:           artifactReviewOutputSchema(),
			IsStrictlyEnforced: true,
		},
		RequireParameters:     true,
		EnableResponseHealing: true,
	})
	if errorValue != nil {
		return artifactReviewErrorResponse(request.ToolName, errorValue.Error(), "openrouter_review_failed", "llm_review", true), nil
	}
	validatedResponse, errorValue := capabilitySuccessResponseFrom(request.ToolName, "ok", json.RawMessage(response.Content), capabilityResponseOrigin{
		Provider:        "openrouter",
		SelectedBackend: capabilities.LLMBackendRemote,
		Content:         response.Content,
	})
	if errorValue != nil {
		return artifactReviewErrorResponse(request.ToolName, errorValue.Error(), "invalid_review_json", "llm_review", true), nil
	}
	return validatedResponse, nil
}

func decodeArtifactReviewInput(document json.RawMessage) (artifactReviewInput, error) {
	var input artifactReviewInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return artifactReviewInput{}, errorValue
	}
	input.ArtifactKind = strings.TrimSpace(input.ArtifactKind)
	input.Intent = strings.TrimSpace(input.Intent)
	input.Rubric = strings.TrimSpace(input.Rubric)
	if input.ArtifactKind == "" {
		return artifactReviewInput{}, errors.New("artifactKind is required")
	}
	if input.Intent == "" {
		return artifactReviewInput{}, errors.New("intent is required")
	}
	if input.Rubric == "" {
		return artifactReviewInput{}, errors.New("rubric is required")
	}
	if len(input.Evidence) == 0 {
		return artifactReviewInput{}, errors.New("at least one evidence image is required")
	}
	if len(input.Evidence) > maximumArtifactReviewEvidenceCount {
		return artifactReviewInput{}, fmt.Errorf("evidence must contain at most %d images", maximumArtifactReviewEvidenceCount)
	}
	for index := range input.Evidence {
		input.Evidence[index] = cleanArtifactReviewEvidence(input.Evidence[index])
		if input.Evidence[index].Path == "" || input.Evidence[index].MimeType == "" || input.Evidence[index].Label == "" {
			return artifactReviewInput{}, errors.New("each evidence item requires path, mimeType, and label")
		}
		if input.Evidence[index].MimeType != "image/png" && input.Evidence[index].MimeType != "image/jpeg" {
			return artifactReviewInput{}, errors.New("evidence mimeType must be image/png or image/jpeg")
		}
	}
	return input, nil
}

func cleanArtifactReviewEvidence(evidence artifactReviewEvidence) artifactReviewEvidence {
	return artifactReviewEvidence{
		Role:     strings.TrimSpace(evidence.Role),
		Path:     strings.TrimSpace(evidence.Path),
		MimeType: strings.TrimSpace(evidence.MimeType),
		Label:    strings.TrimSpace(evidence.Label),
	}
}

func (service Service) artifactReviewMessages(input artifactReviewInput) ([]LLMMessage, error) {
	parts := []llmbackend.MessagePart{{
		Type: "text",
		Text: artifactReviewPrompt(input),
	}}
	for _, evidence := range input.Evidence {
		part, errorValue := service.artifactReviewImagePart(evidence)
		if errorValue != nil {
			return nil, errorValue
		}
		parts = append(parts, part)
	}
	return []LLMMessage{{
		Role:  "user",
		Parts: parts,
	}}, nil
}

func artifactReviewPrompt(input artifactReviewInput) string {
	prompt := strings.Builder{}
	prompt.WriteString("Review this rendered artifact evidence. Return strict JSON matching the schema.\n")
	prompt.WriteString("Artifact kind: " + input.ArtifactKind + "\n")
	prompt.WriteString("Intent: " + input.Intent + "\n")
	prompt.WriteString("Rubric: " + input.Rubric + "\n")
	if len(input.ExpectedText) > 0 {
		prompt.WriteString("Expected visible text:\n")
		for _, item := range input.ExpectedText {
			prompt.WriteString("- " + item.Target + ": " + item.Text + "\n")
		}
	}
	if len(input.PreviousIssues) > 0 {
		prompt.WriteString("Previous issues to verify:\n")
		for _, issue := range input.PreviousIssues {
			prompt.WriteString("- " + issue.Target + " [" + issue.Severity + "/" + issue.Category + "]: " + issue.Message + "\n")
		}
	}
	prompt.WriteString("Evidence labels:\n")
	for _, evidence := range input.Evidence {
		prompt.WriteString("- " + evidence.Label + " (" + evidence.Role + ")\n")
	}
	prompt.WriteString("Use severity blocking for clipped content, unreadable text, missing expected text, blank renders, broken responsive layout, or obvious template/starter leakage. Use warning for polish issues that should be fixed or explicitly accepted.")
	return prompt.String()
}

func (service Service) artifactReviewImagePart(evidence artifactReviewEvidence) (llmbackend.MessagePart, error) {
	hostPath, _, errorValue := service.resolveFileReadPath(artifactReviewAgentPath(evidence.Path))
	if errorValue != nil {
		return llmbackend.MessagePart{}, errorValue
	}
	information, errorValue := os.Stat(hostPath)
	if errorValue != nil {
		return llmbackend.MessagePart{}, errorValue
	}
	if information.Size() > maximumArtifactReviewImageBytes {
		return llmbackend.MessagePart{}, fmt.Errorf("%s is too large for artifact review", evidence.Path)
	}
	document, errorValue := os.ReadFile(hostPath)
	if errorValue != nil {
		return llmbackend.MessagePart{}, errorValue
	}
	return llmbackend.MessagePart{
		Type:       "image",
		MimeType:   evidence.MimeType,
		DataBase64: base64.StdEncoding.EncodeToString(document),
	}, nil
}

func artifactReviewAgentPath(path string) string {
	trimmedPath := strings.TrimSpace(path)
	if filepath.IsAbs(trimmedPath) {
		return trimmedPath
	}
	return "/workspace/" + strings.TrimPrefix(filepath.ToSlash(filepath.Clean(trimmedPath)), "./")
}

func artifactReviewOutputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"passed":{"type":"boolean"},"issues":{"type":"array","items":{"type":"object","properties":{"severity":{"type":"string","enum":["blocking","warning","info"]},"category":{"type":"string","enum":["textFit","layout","visualHierarchy","contentDensity","templateSmell","responsiveness","renderFidelity"]},"target":{"type":"string"},"message":{"type":"string"},"suggestedFix":{"type":"string"}},"required":["severity","category","target","message","suggestedFix"],"additionalProperties":false}},"acceptedWarnings":{"type":"array","items":{"type":"string"}},"summary":{"type":"string"}},"required":["passed","issues","acceptedWarnings","summary"],"additionalProperties":false}`)
}

func artifactReviewErrorResponse(toolName string, message string, code string, stage string, retryable bool) capabilities.ToolInvokeResponse {
	result, _ := json.Marshal(map[string]any{
		"status":       "error",
		"message":      message,
		"errorCode":    code,
		"failureStage": stage,
		"retryable":    retryable,
		"safeRetry":    retryable,
	})
	return capabilities.ToolInvokeResponse{
		Provider:        "openrouter",
		SelectedBackend: capabilities.LLMBackendRemote,
		ToolName:        toolName,
		Outcome:         capabilities.ToolOutcomeFailed,
		Status:          "error",
		IsError:         true,
		Message:         message,
		ErrorCode:       code,
		FailureStage:    stage,
		Retryable:       retryable,
		SafeRetry:       retryable,
		Result:          result,
	}
}
