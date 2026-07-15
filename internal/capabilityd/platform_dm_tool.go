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

type platformDMMattermostUser struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Nickname    string `json:"nickname"`
	DeleteAt    int64  `json:"delete_at"`
	IsBot       bool   `json:"is_bot"`
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

func (service Service) resolvePlatformDMRecipient(ctx context.Context, personHint string) (platformDMRecipient, platformDMFailure, bool) {
	resolution, errorValue := service.fetchPlatformDMRecipientResolution(ctx, personHint)
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
	requestBody, errorValue := json.Marshal(map[string]string{"platform": "mattermost", "hint": personHint})
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
		Mention:            mattermostMentionForUsername(recipient.Username),
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
	message := fmt.Sprintf("recipient %q was not found among approved InternKim people with active Mattermost accounts", personHint)
	return platformDMStaticFailure("recipient_not_found", "recipient_resolve", message)
}

func platformDMUnavailableFailure(errorValue error) platformDMFailure {
	failure := platformDMFailureForError("mattermost_lookup", "mattermost_unavailable", errorValue, true)
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

func mattermostMentionForUsername(username string) string {
	trimmedUsername := strings.TrimSpace(strings.TrimPrefix(username, "@"))
	if trimmedUsername == "" {
		return ""
	}
	return "@" + trimmedUsername
}

func normalizePlatformDMMatchValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func mattermostPendingPostID(botUserID string, idempotencyKey string) string {
	trimmedKey := strings.TrimSpace(idempotencyKey)
	trimmedBotUserID := strings.TrimSpace(botUserID)
	if trimmedKey == "" || trimmedBotUserID == "" {
		return ""
	}
	return trimmedBotUserID + ":" + trimmedKey
}

func (service Service) sendMattermostDirectMessageWithDispatch(ctx context.Context, userID string, message string, idempotencyKey string) (string, platformDMFailure, bool) {
	botUser, errorValue := service.resolveMattermostBotUser(ctx)
	if errorValue != nil {
		return "", platformDMFailureForError("mattermost_lookup", "mattermost_unavailable", errorValue, true), true
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return "", platformDMStaticFailure("recipient_not_found", "recipient_resolve", "mattermost user ID is required"), true
	}
	if normalizedUserID == botUser.ID {
		return "", platformDMStaticFailure("recipient_ambiguous", "recipient_resolve", "cannot send a direct message to the InternKim bot user"), true
	}
	channelID, errorValue := service.createMattermostDirectChannel(ctx, botUser.ID, normalizedUserID)
	if errorValue != nil {
		return "", platformDMFailureForError("direct_channel_create", "direct_channel_create_failed", errorValue, true), true
	}
	var response struct {
		ID string `json:"id"`
	}
	body := map[string]string{"channel_id": channelID, "message": message}
	if pendingPostID := mattermostPendingPostID(botUser.ID, idempotencyKey); pendingPostID != "" {
		body["pending_post_id"] = pendingPostID
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/posts", body, &response); errorValue != nil {
		return "", platformDMFailureForError("message_send", "send_failed", errorValue, false), true
	}
	if strings.TrimSpace(response.ID) == "" {
		return "", platformDMStaticFailure("send_failed", "message_send", "mattermost direct message did not return a post ID"), true
	}
	return response.ID, platformDMFailure{}, false
}

func (service Service) resolveMattermostBotUser(ctx context.Context) (platformDMMattermostUser, error) {
	var botUser platformDMMattermostUser
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/me", nil, &botUser); errorValue != nil {
		return platformDMMattermostUser{}, errorValue
	}
	if strings.TrimSpace(botUser.ID) == "" {
		return platformDMMattermostUser{}, fmt.Errorf("mattermost bot user is not available")
	}
	if !botUser.IsBot {
		return platformDMMattermostUser{}, fmt.Errorf("mattermost token user is not a bot")
	}
	return botUser, nil
}

func (service Service) createMattermostDirectChannel(ctx context.Context, botUserID string, recipientUserID string) (string, error) {
	var channelRecord struct {
		ID string `json:"id"`
	}
	body := []string{strings.TrimSpace(recipientUserID), strings.TrimSpace(botUserID)}
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/channels/direct", body, &channelRecord); errorValue != nil {
		return "", errorValue
	}
	if strings.TrimSpace(channelRecord.ID) == "" {
		return "", fmt.Errorf("mattermost direct channel was not created")
	}
	return channelRecord.ID, nil
}

func platformDMDeniedResponse(toolName string, failure platformDMFailure) capabilities.ToolInvokeResponse {
	resultDocument, _ := json.Marshal(failure)
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        toolName,
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
