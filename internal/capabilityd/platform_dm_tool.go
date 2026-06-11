package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type platformDMFailure struct {
	ErrorCode      string                `json:"errorCode"`
	FailureStage   string                `json:"failureStage"`
	Message        string                `json:"message"`
	Retryable      bool                  `json:"retryable"`
	SafeRetry      bool                  `json:"safeRetry"`
	Candidates     []platformDMRecipient `json:"candidates,omitempty"`
	ApprovedPeople []string              `json:"approvedPeople,omitempty"`
}

type blueclawRecipientResolution struct {
	Status         string                       `json:"status"`
	Recipient      *blueclawRecipientCandidate  `json:"recipient,omitempty"`
	Candidates     []blueclawRecipientCandidate `json:"candidates,omitempty"`
	ApprovedPeople []string                     `json:"approvedPeople,omitempty"`
}

type blueclawRecipientCandidate struct {
	PersonID       string   `json:"personID"`
	DisplayName    string   `json:"displayName"`
	Emails         []string `json:"emails,omitempty"`
	ExternalUserID string   `json:"externalUserID,omitempty"`
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
}

type platformDMRecipient struct {
	PersonID           string   `json:"personID"`
	DisplayName        string   `json:"displayName"`
	Emails             []string `json:"emails"`
	MattermostUserID   string   `json:"mattermostUserID"`
	MattermostUsername string   `json:"mattermostUsername"`
}

func validatePlatformDMSendAuthorization(toolContext capabilities.ToolInvokeContext, recipient platformDMRecipient) string {
	if toolContext.IsScheduledRun || toolContext.IsApprovalContinuation {
		return ""
	}
	if isPlatformDMSelfRecipient(toolContext, recipient) {
		return ""
	}
	return "platform.message.send requires approval for immediate sends; scheduled runs may send without approval"
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
	resolution, errorValue := service.fetchBlueclawRecipientResolution(ctx, personHint)
	if errorValue != nil {
		return platformDMRecipient{}, platformDMFailureForError("recipient_resolve", "identity_unavailable", errorValue, true), true
	}
	switch resolution.Status {
	case "resolved":
		return platformDMRecipientFromResolution(resolution)
	case "unlinked":
		return service.resolveUnlinkedPlatformDMRecipient(ctx, resolution)
	case "ambiguous":
		candidates := platformDMRecipientsFromCandidates(resolution.Candidates)
		message := fmt.Sprintf("recipient %q is ambiguous: %s", personHint, platformDMRecipientList(candidates))
		failure := platformDMStaticFailure("recipient_ambiguous", "recipient_resolve", message)
		failure.Candidates = candidates
		return platformDMRecipient{}, failure, true
	default:
		message := fmt.Sprintf("recipient %q was not found among approved InternKim people; approved people: %s", personHint, strings.Join(resolution.ApprovedPeople, ", "))
		failure := platformDMStaticFailure("recipient_not_found", "recipient_resolve", message)
		failure.ApprovedPeople = resolution.ApprovedPeople
		return platformDMRecipient{}, failure, true
	}
}

func platformDMRecipientFromResolution(resolution blueclawRecipientResolution) (platformDMRecipient, platformDMFailure, bool) {
	if resolution.Recipient == nil {
		return platformDMRecipient{}, platformDMStaticFailure("identity_unavailable", "recipient_resolve", "identity resolution returned no recipient"), true
	}
	return platformDMRecipientFromCandidate(*resolution.Recipient), platformDMFailure{}, false
}

func (service Service) resolveUnlinkedPlatformDMRecipient(ctx context.Context, resolution blueclawRecipientResolution) (platformDMRecipient, platformDMFailure, bool) {
	if resolution.Recipient == nil {
		return platformDMRecipient{}, platformDMStaticFailure("identity_unavailable", "recipient_resolve", "identity resolution returned no recipient"), true
	}
	recipient := platformDMRecipientFromCandidate(*resolution.Recipient)
	for _, email := range recipient.Emails {
		user, errorValue := service.fetchMattermostUserByEmail(ctx, email)
		if errorValue != nil {
			continue
		}
		recipient.MattermostUserID = user.ID
		recipient.MattermostUsername = user.Username
		return recipient, platformDMFailure{}, false
	}
	message := fmt.Sprintf("approved person %q has no active Mattermost account", recipient.DisplayName)
	return platformDMRecipient{}, platformDMStaticFailure("recipient_not_found", "recipient_resolve", message), true
}

func platformDMRecipientFromCandidate(candidate blueclawRecipientCandidate) platformDMRecipient {
	return platformDMRecipient{
		PersonID:         candidate.PersonID,
		DisplayName:      candidate.DisplayName,
		Emails:           candidate.Emails,
		MattermostUserID: candidate.ExternalUserID,
	}
}

func platformDMRecipientsFromCandidates(candidates []blueclawRecipientCandidate) []platformDMRecipient {
	recipients := []platformDMRecipient{}
	for _, candidate := range candidates {
		recipients = append(recipients, platformDMRecipientFromCandidate(candidate))
	}
	return recipients
}

func (service Service) fetchBlueclawRecipientResolution(ctx context.Context, personHint string) (blueclawRecipientResolution, error) {
	endpoint := strings.TrimRight(firstNonEmpty(service.Configuration.BlueclawBaseURL, DefaultConfiguration().BlueclawBaseURL), "/") + "/admin/api/identity/resolve-recipient"
	requestBody, errorValue := json.Marshal(map[string]string{"platform": "mattermost", "hint": personHint})
	if errorValue != nil {
		return blueclawRecipientResolution{}, errorValue
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(requestBody))
	if errorValue != nil {
		return blueclawRecipientResolution{}, errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return blueclawRecipientResolution{}, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return blueclawRecipientResolution{}, fmt.Errorf("recipient resolution failed with status %d", response.StatusCode)
	}
	var resolution blueclawRecipientResolution
	if errorValue := json.NewDecoder(response.Body).Decode(&resolution); errorValue != nil {
		return blueclawRecipientResolution{}, errorValue
	}
	return resolution, nil
}

func (service Service) fetchMattermostUserByEmail(ctx context.Context, email string) (platformDMMattermostUser, error) {
	var user platformDMMattermostUser
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/email/"+url.PathEscape(strings.TrimSpace(email)), nil, &user); errorValue != nil {
		return platformDMMattermostUser{}, errorValue
	}
	if strings.TrimSpace(user.ID) == "" || user.DeleteAt != 0 {
		return platformDMMattermostUser{}, fmt.Errorf("mattermost user for %q is not active", email)
	}
	return user, nil
}

func (service Service) sendPlatformDirectMessage(ctx context.Context, toolContext capabilities.ToolInvokeContext, personHint string, message string) (map[string]string, platformDMFailure, bool) {
	recipient, failure, hasFailure := service.resolvePlatformDMRecipient(ctx, personHint)
	if hasFailure {
		return nil, failure, true
	}
	if errorMessage := validatePlatformDMSendAuthorization(toolContext, recipient); errorMessage != "" {
		return nil, platformDMStaticFailure("approval_required", "authorization", errorMessage), true
	}
	dispatchID, failure, hasFailure := service.sendMattermostDirectMessageWithDispatch(ctx, recipient.MattermostUserID, message)
	if hasFailure {
		return nil, failure, true
	}
	result := map[string]string{
		"platform":           "mattermost",
		"dispatchID":         dispatchID,
		"personID":           recipient.PersonID,
		"mattermostUserID":   recipient.MattermostUserID,
		"mattermostUsername": recipient.MattermostUsername,
	}
	return result, platformDMFailure{}, false
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
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(strings.TrimPrefix(value, "@")))), "")
}

func (service Service) sendMattermostDirectMessageWithDispatch(ctx context.Context, userID string, message string) (string, platformDMFailure, bool) {
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
