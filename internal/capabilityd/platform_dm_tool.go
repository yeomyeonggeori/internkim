package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
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
	PersonID           string   `json:"personID"`
	DisplayName        string   `json:"displayName"`
	Emails             []string `json:"emails"`
	MattermostUserID   string   `json:"mattermostUserID"`
	MattermostUsername string   `json:"mattermostUsername"`
	Mention            string   `json:"mention,omitempty"`
}

type platformDMResolvedRecipient struct {
	PersonID       string   `json:"personID"`
	DisplayName    string   `json:"displayName"`
	Emails         []string `json:"emails"`
	ExternalUserID string   `json:"externalUserID"`
	Username       string   `json:"username"`
}

type platformDMRecipientResolution struct {
	Status     string                        `json:"status"`
	Recipient  *platformDMResolvedRecipient  `json:"recipient,omitempty"`
	Candidates []platformDMResolvedRecipient `json:"candidates,omitempty"`
}

func isPlatformDMSelfRecipient(toolContext capabilities.ToolInvokeContext, recipient platformDMRecipient) bool {
	if strings.TrimSpace(toolContext.RequesterPersonID) != "" && strings.TrimSpace(toolContext.RequesterPersonID) == recipient.PersonID {
		return true
	}
	if strings.TrimSpace(toolContext.RequesterPlatformUserID) != "" && strings.TrimSpace(toolContext.RequesterPlatformUserID) == recipient.MattermostUserID {
		return true
	}
	return false
}

// Who the hint names is settled against the company directory, and only then is
// this platform asked which account is theirs. Deciding who somebody is from the
// accounts this agent happens to have seen makes a colleague who has not written
// to it yet into a stranger.
func (service Service) resolvePlatformDMRecipient(ctx context.Context, personHint string, responseLanguage string) (platformDMRecipient, platformDMFailure, bool) {
	named, directoryFailure, hasDirectoryFailure := service.namedDirectoryPerson(ctx, personHint, responseLanguage)
	if hasDirectoryFailure {
		return platformDMRecipient{}, directoryFailure, true
	}
	resolution, errorValue := service.fetchPlatformDMRecipientResolution(ctx, named.Email)
	if errorValue != nil {
		return platformDMRecipient{}, platformDMUnavailableFailure(errorValue), true
	}
	switch resolution.Status {
	case "resolved":
		recipient := platformDMRecipientFromResolution(resolution.Recipient)
		if strings.TrimSpace(recipient.MattermostUserID) == "" {
			return platformDMRecipient{}, platformDMRecipientNotFoundFailure(personHint), true
		}
		return recipient, platformDMFailure{}, false
	case "ambiguous":
		candidates := platformDMRecipientsFromResolution(resolution.Candidates)
		message := fmt.Sprintf("recipient %q is ambiguous: %s", personHint, platformDMRecipientList(candidates))
		failure := platformDMStaticFailure("recipient_ambiguous", "recipient_resolve", message)
		failure.Candidates = candidates
		return platformDMRecipient{}, failure, true
	case "not_found", "unlinked":
		return platformDMRecipient{}, platformDMRecipientNotFoundFailure(personHint), true
	default:
		errorValue := fmt.Errorf("recipient resolve returned unsupported status %q", resolution.Status)
		return platformDMRecipient{}, platformDMUnavailableFailure(errorValue), true
	}
}

func (service Service) fetchPlatformDMRecipientResolution(ctx context.Context, personHint string) (platformDMRecipientResolution, error) {
	endpoint := strings.TrimRight(firstNonEmpty(service.Configuration.BlueclawBaseURL, DefaultConfiguration().BlueclawBaseURL), "/") + "/admin/api/identity/resolve-recipient"
	requestBody, errorValue := json.Marshal(map[string]string{"platform": service.companyMessenger(), "hint": personHint})
	if errorValue != nil {
		return platformDMRecipientResolution{}, errorValue
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestBody))
	if errorValue != nil {
		return platformDMRecipientResolution{}, errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return platformDMRecipientResolution{}, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return platformDMRecipientResolution{}, fmt.Errorf("recipient resolve failed with status %d", response.StatusCode)
	}
	var resolution platformDMRecipientResolution
	if errorValue := json.NewDecoder(response.Body).Decode(&resolution); errorValue != nil {
		return platformDMRecipientResolution{}, errorValue
	}
	return resolution, nil
}

func platformDMRecipientFromResolution(recipient *platformDMResolvedRecipient) platformDMRecipient {
	if recipient == nil {
		return platformDMRecipient{}
	}
	return platformDMRecipient{
		PersonID:           strings.TrimSpace(recipient.PersonID),
		DisplayName:        strings.TrimSpace(recipient.DisplayName),
		Emails:             normalizedPlatformDMEmails(recipient.Emails),
		MattermostUserID:   strings.TrimSpace(recipient.ExternalUserID),
		MattermostUsername: strings.TrimSpace(recipient.Username),
		Mention:            platformMentionForUsername(recipient.Username),
	}
}

func platformDMRecipientsFromResolution(recipients []platformDMResolvedRecipient) []platformDMRecipient {
	resolvedRecipients := []platformDMRecipient{}
	for recipientIndex := range recipients {
		recipient := platformDMRecipientFromResolution(&recipients[recipientIndex])
		resolvedRecipients = append(resolvedRecipients, recipient)
	}
	return resolvedRecipients
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

func safePlatformDMError(errorValue error) string {
	if errorValue == nil {
		return ""
	}
	return strings.TrimSpace(errorValue.Error())
}

func normalizePlatformDMMatchValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func platformDMDeniedResponse(toolName string, failure platformDMFailure) capabilities.ToolInvokeResponse {
	resultDocument, _ := json.Marshal(failure)
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        toolName,
		Outcome:         capabilities.ToolOutcomeDenied,
		Status:          "denied",
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

func platformMentionForUsername(username string) string {
	trimmedUsername := strings.TrimSpace(strings.TrimPrefix(username, "@"))
	if trimmedUsername == "" {
		return ""
	}
	return "@" + trimmedUsername
}
