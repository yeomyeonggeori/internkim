package capabilityd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

type platformDMFailure struct {
	ErrorCode    string                `json:"errorCode"`
	FailureStage string                `json:"failureStage"`
	Message      string                `json:"message"`
	Retryable    bool                  `json:"retryable"`
	SafeRetry    bool                  `json:"safeRetry"`
	Candidates   []platformDMRecipient `json:"candidates,omitempty"`
}

type platformDMRecipient struct {
	PersonID       string   `json:"personID"`
	DisplayName    string   `json:"displayName"`
	Emails         []string `json:"emails"`
	ExternalUserID string   `json:"externalUserID"`
	Username       string   `json:"username"`
	Mention        string   `json:"mention,omitempty"`
}

func platformDMRecipientNotFoundFailure(personHint string) platformDMFailure {
	message := fmt.Sprintf("recipient %q was not found among approved internkim people with an active messenger account", personHint)
	return platformDMStaticFailure("recipient_not_found", "recipient_resolve", message)
}

func platformDMUnavailableFailure(errorValue error) platformDMFailure {
	failure := platformDMFailureForError("recipient_lookup", "directory_unavailable", errorValue, true)
	failure.Retryable = true
	failure.SafeRetry = true
	return failure
}

func normalizedPlatformDMEmails(emails []string) []string {
	normalizedEmails := []string{}
	seenEmail := map[string]bool{}
	for _, email := range emails {
		normalizedEmail := normalizePlatformDMMatchValue(email)
		if normalizedEmail == "" || seenEmail[normalizedEmail] {
			continue
		}
		seenEmail[normalizedEmail] = true
		normalizedEmails = append(normalizedEmails, normalizedEmail)
	}
	return normalizedEmails
}

func platformDMRecipientList(recipients []platformDMRecipient) string {
	parts := []string{}
	for _, recipient := range recipients {
		parts = append(parts, firstNonEmpty(recipient.DisplayName, recipient.PersonID)+" <"+strings.Join(recipient.Emails, ",")+">")
	}
	return strings.Join(parts, "; ")
}

func normalizePlatformDMMatchValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func platformDMErrorResponse(toolName string, failure platformDMFailure) capabilities.ToolInvokeResponse {
	resultDocument, _ := json.Marshal(failure)
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        toolName,
		Outcome:         capabilities.ToolOutcomeFailed,
		Status:          "error",
		Content:         failure.Message,
		IsError:         true,
		Message:         failure.Message,
		ErrorCode:       failure.ErrorCode,
		FailureStage:    failure.FailureStage,
		Retryable:       failure.Retryable,
		SafeRetry:       failure.SafeRetry,
		Result:          resultDocument,
	}
}

func platformDMStaticFailure(errorCode string, failureStage string, message string) platformDMFailure {
	return platformDMFailure{
		ErrorCode:    errorCode,
		FailureStage: failureStage,
		Message:      strings.TrimSpace(message),
	}
}

func platformDMFailureForError(failureStage string, errorCode string, errorValue error, canRetrySafely bool) platformDMFailure {
	failure := platformDMStaticFailure(errorCode, failureStage, errorValue.Error())
	if isPlatformDMTransientError(errorValue) {
		failure.Retryable = true
		failure.SafeRetry = canRetrySafely
	}
	return failure
}

func isPlatformDMTransientError(errorValue error) bool {
	message := strings.ToLower(errorValue.Error())
	transientFragments := []string{
		"timeout",
		"context deadline exceeded",
		"connection reset",
		"connection refused",
		"temporary",
		"eof",
		"status 500",
		"status 502",
		"status 503",
		"status 504",
		"http 500",
		"http 502",
		"http 503",
		"http 504",
	}
	for _, fragment := range transientFragments {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
}
