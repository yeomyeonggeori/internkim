package capabilityd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
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

type platformDMPolicyDocument struct {
	People []platformDMPolicyPerson `json:"people"`
}

type platformDMPolicyPerson struct {
	PersonID    string   `json:"personID"`
	DisplayName string   `json:"displayName"`
	Emails      []string `json:"emails"`
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
	MattermostAliases  []string `json:"mattermostAliases"`
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
	candidates, failure, hasFailure := service.resolvePlatformDMCandidates(ctx, personHint)
	if hasFailure {
		return platformDMRecipient{}, failure, true
	}
	switch len(candidates) {
	case 0:
		message := fmt.Sprintf("recipient %q was not found among approved InternKim people with active Mattermost accounts", personHint)
		return platformDMRecipient{}, platformDMStaticFailure("recipient_not_found", "recipient_resolve", message), true
	case 1:
		return candidates[0], platformDMFailure{}, false
	default:
		message := fmt.Sprintf("recipient %q is ambiguous: %s", personHint, platformDMRecipientList(candidates))
		failure := platformDMStaticFailure("recipient_ambiguous", "recipient_resolve", message)
		failure.Candidates = candidates
		return platformDMRecipient{}, failure, true
	}
}

func (service Service) resolvePlatformDMCandidates(ctx context.Context, personHint string) ([]platformDMRecipient, platformDMFailure, bool) {
	policyDocument, errorValue := service.fetchPlatformDMPolicy(ctx)
	if errorValue != nil {
		return nil, platformDMFailureForError("mattermost_lookup", "mattermost_unavailable", errorValue, true), true
	}
	mattermostUsers, errorValue := service.listPlatformDMMattermostUsers(ctx)
	if errorValue != nil {
		return nil, platformDMFailureForError("mattermost_lookup", "mattermost_unavailable", errorValue, true), true
	}
	return matchingPlatformDMRecipients(personHint, policyDocument.People, mattermostUsers), platformDMFailure{}, false
}

func (service Service) fetchPlatformDMPolicy(ctx context.Context) (platformDMPolicyDocument, error) {
	endpoint := strings.TrimRight(firstNonEmpty(service.Configuration.BlueclawBaseURL, DefaultConfiguration().BlueclawBaseURL), "/") + "/admin/api/policy"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if errorValue != nil {
		return platformDMPolicyDocument{}, errorValue
	}
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return platformDMPolicyDocument{}, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return platformDMPolicyDocument{}, fmt.Errorf("policy lookup failed with status %d", response.StatusCode)
	}
	var policyDocument platformDMPolicyDocument
	if errorValue := json.NewDecoder(response.Body).Decode(&policyDocument); errorValue != nil {
		return platformDMPolicyDocument{}, errorValue
	}
	return policyDocument, nil
}

func (service Service) listPlatformDMMattermostUsers(ctx context.Context) ([]platformDMMattermostUser, error) {
	var users []platformDMMattermostUser
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users?per_page=200", nil, &users); errorValue != nil {
		return nil, errorValue
	}
	activeUsers := []platformDMMattermostUser{}
	for _, user := range users {
		if strings.TrimSpace(user.ID) == "" || user.DeleteAt != 0 {
			continue
		}
		activeUsers = append(activeUsers, user)
	}
	return activeUsers, nil
}

func matchingPlatformDMRecipients(personHint string, people []platformDMPolicyPerson, users []platformDMMattermostUser) []platformDMRecipient {
	candidates := platformDMRecipientsForApprovedPeople(people, users)
	matches := []platformDMRecipient{}
	for _, candidate := range candidates {
		if platformDMRecipientMatches(personHint, candidate) {
			matches = append(matches, candidate)
		}
	}
	sortPlatformDMRecipients(matches)
	return matches
}

func platformDMRecipientsForApprovedPeople(people []platformDMPolicyPerson, users []platformDMMattermostUser) []platformDMRecipient {
	userByEmail := map[string]platformDMMattermostUser{}
	for _, user := range users {
		normalizedEmail := normalizePlatformDMMatchValue(user.Email)
		if normalizedEmail != "" {
			userByEmail[normalizedEmail] = user
		}
	}
	recipients := []platformDMRecipient{}
	seenRecipient := map[string]bool{}
	for _, person := range people {
		for _, email := range person.Emails {
			user, isFound := userByEmail[normalizePlatformDMMatchValue(email)]
			if !isFound {
				continue
			}
			key := strings.TrimSpace(person.PersonID) + ":" + strings.TrimSpace(user.ID)
			if seenRecipient[key] {
				continue
			}
			seenRecipient[key] = true
			recipients = append(recipients, platformDMRecipient{
				PersonID:           strings.TrimSpace(person.PersonID),
				DisplayName:        strings.TrimSpace(person.DisplayName),
				Emails:             normalizedPlatformDMEmails(person.Emails),
				MattermostUserID:   strings.TrimSpace(user.ID),
				MattermostUsername: strings.TrimSpace(user.Username),
				MattermostAliases:  mattermostUserAliases(user),
			})
		}
	}
	return recipients
}

func platformDMRecipientMatches(personHint string, recipient platformDMRecipient) bool {
	hint := normalizePlatformDMMatchValue(strings.TrimPrefix(personHint, "@"))
	if hint == "" {
		return false
	}
	values := []string{recipient.PersonID, recipient.DisplayName, recipient.MattermostUsername}
	values = append(values, recipient.Emails...)
	values = append(values, recipient.MattermostAliases...)
	for _, value := range values {
		if normalizePlatformDMMatchValue(value) == hint {
			return true
		}
	}
	for _, value := range values {
		normalizedValue := normalizePlatformDMMatchValue(value)
		if normalizedValue != "" && (strings.Contains(normalizedValue, hint) || strings.Contains(hint, normalizedValue)) {
			return true
		}
	}
	return false
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

func mattermostUserAliases(user platformDMMattermostUser) []string {
	return trimPlatformDMMattermostAliases([]string{user.Email, user.Username, user.DisplayName, user.FirstName, user.LastName, user.Nickname})
}

func trimPlatformDMMattermostAliases(values []string) []string {
	aliases := []string{}
	seenAlias := map[string]bool{}
	for _, value := range values {
		normalizedValue := normalizePlatformDMMatchValue(value)
		if normalizedValue == "" || seenAlias[normalizedValue] {
			continue
		}
		seenAlias[normalizedValue] = true
		aliases = append(aliases, normalizedValue)
	}
	return aliases
}

func sortPlatformDMRecipients(recipients []platformDMRecipient) {
	sort.Slice(recipients, func(leftIndex int, rightIndex int) bool {
		left := recipients[leftIndex]
		right := recipients[rightIndex]
		if left.PersonID != right.PersonID {
			return left.PersonID < right.PersonID
		}
		return left.MattermostUserID < right.MattermostUserID
	})
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
