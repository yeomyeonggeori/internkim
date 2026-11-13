package capabilityd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
	"github.com/yeomyeonggeori/internkim/internal/personname"
)

type platformDMFailure struct {
	ErrorCode    string        `json:"errorCode"`
	FailureStage string        `json:"failureStage"`
	Message      string        `json:"message"`
	Retryable    bool          `json:"retryable"`
	SafeRetry    bool          `json:"safeRetry"`
	Candidates   []personShown `json:"candidates,omitempty"`
}

type personShown struct {
	PersonID string   `json:"personID"`
	Name     string   `json:"name"`
	Emails   []string `json:"emails,omitempty"`
}

func shownPerson(person directoryPerson, responseLanguage string) personShown {
	return personShown{PersonID: strings.TrimSpace(person.MemberID), Name: personname.Render(person.Name, responseLanguage)}
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
		Content:         string(resultDocument),
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
