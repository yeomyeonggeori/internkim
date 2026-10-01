package admind

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/llmbackend"
)

const (
	llmFailureEvidenceEventName            = "llm.failure_evidence"
	llmFailureEvidenceUnavailableEventName = "llm.failure_evidence.unavailable"
)

func inlineLLMFailureEvidence(workspacePath string, detail map[string]any, isViewerAdmin bool) {
	if !isViewerAdmin {
		return
	}
	failureReason, hasFailureReason := taskFailureReason(detail)
	if !hasFailureReason {
		return
	}
	evidenceIdentifier, hasEvidence := llmFailureEvidenceIdentifier(failureReason)
	if !hasEvidence {
		return
	}
	event := readLLMFailureEvidenceEvent(workspacePath, evidenceIdentifier)
	taskEvents, _ := detail["taskEvents"].([]any)
	detail["taskEvents"] = append(taskEvents, event)
}

func taskFailureReason(detail map[string]any) (string, bool) {
	taskRun, isTaskRun := detail["taskRun"].(map[string]any)
	if !isTaskRun {
		return "", false
	}
	failureReason, isFailureReason := taskRun["failureReason"].(string)
	return failureReason, isFailureReason
}

func llmFailureEvidenceIdentifier(failureReason string) (string, bool) {
	_, evidenceReference, hasMarker := strings.Cut(failureReason, llmbackend.FailureEvidenceMarker)
	if !hasMarker {
		return "", false
	}
	evidenceIdentifier, _, hasTerminator := strings.Cut(evidenceReference, ";")
	if !hasTerminator || evidenceIdentifier == "" {
		return "", false
	}
	return evidenceIdentifier, true
}

func readLLMFailureEvidenceEvent(workspacePath string, evidenceIdentifier string) map[string]any {
	document, errorValue := llmbackend.ReadFailureEvidence(workspacePath, evidenceIdentifier)
	if errorValue != nil {
		return unavailableLLMFailureEvidenceEvent(evidenceIdentifier, errorValue)
	}
	var evidence struct {
		CreatedAt string `json:"createdAt"`
	}
	if errorValue := json.Unmarshal(document, &evidence); errorValue != nil || evidence.CreatedAt == "" {
		if errorValue == nil {
			errorValue = fmt.Errorf("createdAt is missing")
		}
		return unavailableLLMFailureEvidenceEvent(evidenceIdentifier, errorValue)
	}
	return map[string]any{
		"name":      llmFailureEvidenceEventName,
		"body":      string(document),
		"createdAt": evidence.CreatedAt,
	}
}

func unavailableLLMFailureEvidenceEvent(evidenceIdentifier string, errorValue error) map[string]any {
	body, _ := json.Marshal(map[string]string{
		"evidenceID": evidenceIdentifier,
		"error":      errorValue.Error(),
	})
	return map[string]any{
		"name": llmFailureEvidenceUnavailableEventName,
		"body": string(body),
	}
}
