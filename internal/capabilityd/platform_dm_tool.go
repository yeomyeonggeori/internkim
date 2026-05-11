package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type platformDMSendInput struct {
	RecipientHint string `json:"recipientHint"`
	Message       string `json:"message"`
	Platform      string `json:"platform"`
	Reason        string `json:"reason"`
}

type platformDMInspectInput struct {
	RecipientHint string `json:"recipientHint"`
	Platform      string `json:"platform"`
}

type platformDMFailure struct {
	ErrorCode    string `json:"errorCode"`
	FailureStage string `json:"failureStage"`
	Message      string `json:"message"`
	Retryable    bool   `json:"retryable"`
	SafeRetry    bool   `json:"safeRetry"`
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

type platformDMInspectSnapshot struct {
	IsTokenConfigured    bool
	PolicyDocument       platformDMPolicyDocument
	PolicyError          error
	MattermostUsers      []platformDMMattermostUser
	MattermostUsersError error
	BotUser              platformDMMattermostUser
	BotUserError         error
}

func (service Service) invokePlatformDMSend(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodePlatformDMSendInput(request.Input)
	if errorValue != nil {
		return platformDMErrorResponse(request.ToolName, platformDMStaticFailure("invalid_input", "input_decode", errorValue.Error())), nil
	}
	recipient, failure, hasFailure := service.resolvePlatformDMRecipient(ctx, input.RecipientHint)
	if hasFailure {
		return platformDMErrorResponse(request.ToolName, failure), nil
	}
	if errorMessage := validatePlatformDMSendAuthorization(request.Context, recipient); errorMessage != "" {
		return platformDMDeniedResponse(request.ToolName, platformDMStaticFailure("approval_required", "authorization", errorMessage)), nil
	}
	dispatchID, failure, hasFailure := service.sendMattermostDirectMessageWithDispatch(ctx, recipient.MattermostUserID, input.Message)
	if hasFailure {
		return platformDMErrorResponse(request.ToolName, failure), nil
	}
	result := map[string]string{
		"platform":           "mattermost",
		"dispatchID":         dispatchID,
		"personID":           recipient.PersonID,
		"mattermostUserID":   recipient.MattermostUserID,
		"mattermostUsername": recipient.MattermostUsername,
	}
	resultDocument, _ := json.Marshal(result)
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        request.ToolName,
		Status:          "ok",
		Content:         "direct message sent",
		Result:          resultDocument,
	}, nil
}

func (service Service) invokePlatformDMInspect(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodePlatformDMInspectInput(request.Input)
	if errorValue != nil {
		return platformDMErrorResponse(request.ToolName, platformDMStaticFailure("invalid_input", "input_decode", errorValue.Error())), nil
	}
	snapshot := service.platformDMInspectSnapshot(ctx)
	candidates, candidateFailure, hasCandidateFailure := platformDMInspectCandidates(input.RecipientHint, snapshot)
	diagnosis := platformDMDiagnosis(snapshot)
	result := map[string]any{
		"platform":       "mattermost",
		"recipientHint":  input.RecipientHint,
		"candidateCount": len(candidates),
		"candidates":     candidates,
		"diagnosis":      diagnosis,
	}
	resultDocument, _ := json.Marshal(result)
	if hasCandidateFailure {
		return platformDMErrorResponse(request.ToolName, candidateFailure), nil
	}
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        request.ToolName,
		Status:          "ok",
		Content:         platformDMInspectSummary(input.RecipientHint, candidates, diagnosis),
		Result:          resultDocument,
	}, nil
}

func decodePlatformDMSendInput(document json.RawMessage) (platformDMSendInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return platformDMSendInput{}, fmt.Errorf("platform.dm.send input is required")
	}
	var input platformDMSendInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return platformDMSendInput{}, errorValue
	}
	input.RecipientHint = strings.TrimSpace(input.RecipientHint)
	input.Message = strings.TrimSpace(input.Message)
	input.Platform = firstNonEmpty(strings.TrimSpace(input.Platform), "mattermost")
	input.Reason = strings.TrimSpace(input.Reason)
	if input.RecipientHint == "" {
		return platformDMSendInput{}, fmt.Errorf("recipientHint is required")
	}
	if input.Message == "" {
		return platformDMSendInput{}, fmt.Errorf("message is required")
	}
	if input.Platform != "mattermost" {
		return platformDMSendInput{}, fmt.Errorf("platform %q is not supported", input.Platform)
	}
	return input, nil
}

func decodePlatformDMInspectInput(document json.RawMessage) (platformDMInspectInput, error) {
	var input platformDMInspectInput
	if len(bytes.TrimSpace(document)) > 0 {
		if errorValue := json.Unmarshal(document, &input); errorValue != nil {
			return platformDMInspectInput{}, errorValue
		}
	}
	input.RecipientHint = strings.TrimSpace(input.RecipientHint)
	input.Platform = firstNonEmpty(strings.TrimSpace(input.Platform), "mattermost")
	if input.Platform != "mattermost" {
		return platformDMInspectInput{}, fmt.Errorf("platform %q is not supported", input.Platform)
	}
	return input, nil
}

func validatePlatformDMSendAuthorization(toolContext capabilities.ToolInvokeContext, recipient platformDMRecipient) string {
	if toolContext.IsScheduledRun || toolContext.IsApprovalContinuation {
		return ""
	}
	if isPlatformDMSelfRecipient(toolContext, recipient) {
		return ""
	}
	return "platform.dm.send requires approval for immediate sends; scheduled runs may send without approval"
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

func (service Service) resolvePlatformDMRecipient(ctx context.Context, recipientHint string) (platformDMRecipient, platformDMFailure, bool) {
	candidates, failure, hasFailure := service.resolvePlatformDMCandidates(ctx, recipientHint)
	if hasFailure {
		return platformDMRecipient{}, failure, true
	}
	switch len(candidates) {
	case 0:
		message := fmt.Sprintf("recipient %q was not found among approved InternKim people with active Mattermost accounts", recipientHint)
		return platformDMRecipient{}, platformDMStaticFailure("recipient_not_found", "recipient_resolve", message), true
	case 1:
		return candidates[0], platformDMFailure{}, false
	default:
		message := fmt.Sprintf("recipient %q is ambiguous: %s", recipientHint, platformDMRecipientList(candidates))
		return platformDMRecipient{}, platformDMStaticFailure("recipient_ambiguous", "recipient_resolve", message), true
	}
}

func (service Service) resolvePlatformDMCandidates(ctx context.Context, recipientHint string) ([]platformDMRecipient, platformDMFailure, bool) {
	policyDocument, errorValue := service.fetchPlatformDMPolicy(ctx)
	if errorValue != nil {
		return nil, platformDMFailureForError("mattermost_lookup", "mattermost_unavailable", errorValue, true), true
	}
	mattermostUsers, errorValue := service.listPlatformDMMattermostUsers(ctx)
	if errorValue != nil {
		return nil, platformDMFailureForError("mattermost_lookup", "mattermost_unavailable", errorValue, true), true
	}
	return matchingPlatformDMRecipients(recipientHint, policyDocument.People, mattermostUsers), platformDMFailure{}, false
}

func (service Service) platformDMInspectSnapshot(ctx context.Context) platformDMInspectSnapshot {
	policyDocument, policyError := service.fetchPlatformDMPolicy(ctx)
	mattermostUsers, mattermostUsersError := service.listPlatformDMMattermostUsers(ctx)
	botUser, botUserError := service.resolveMattermostBotUser(ctx)
	return platformDMInspectSnapshot{
		IsTokenConfigured:    readSecretValue(service.Configuration.MattermostTokenPath) != "",
		PolicyDocument:       policyDocument,
		PolicyError:          policyError,
		MattermostUsers:      mattermostUsers,
		MattermostUsersError: mattermostUsersError,
		BotUser:              botUser,
		BotUserError:         botUserError,
	}
}

func platformDMInspectCandidates(recipientHint string, snapshot platformDMInspectSnapshot) ([]platformDMRecipient, platformDMFailure, bool) {
	if strings.TrimSpace(recipientHint) == "" {
		return []platformDMRecipient{}, platformDMFailure{}, false
	}
	if snapshot.PolicyError != nil {
		return nil, platformDMFailureForError("mattermost_lookup", "mattermost_unavailable", snapshot.PolicyError, true), true
	}
	if snapshot.MattermostUsersError != nil {
		return nil, platformDMFailureForError("mattermost_lookup", "mattermost_unavailable", snapshot.MattermostUsersError, true), true
	}
	return matchingPlatformDMRecipients(recipientHint, snapshot.PolicyDocument.People, snapshot.MattermostUsers), platformDMFailure{}, false
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

func matchingPlatformDMRecipients(recipientHint string, people []platformDMPolicyPerson, users []platformDMMattermostUser) []platformDMRecipient {
	candidates := platformDMRecipientsForApprovedPeople(people, users)
	matches := []platformDMRecipient{}
	for _, candidate := range candidates {
		if platformDMRecipientMatches(recipientHint, candidate) {
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

func platformDMRecipientMatches(recipientHint string, recipient platformDMRecipient) bool {
	hint := normalizePlatformDMMatchValue(strings.TrimPrefix(recipientHint, "@"))
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

func platformDMInspectSummary(recipientHint string, candidates []platformDMRecipient, diagnosis map[string]any) string {
	parts := []string{platformDMDiagnosisSummary(diagnosis)}
	if strings.TrimSpace(recipientHint) == "" {
		return strings.Join(parts, " ")
	}
	if len(candidates) == 0 {
		parts = append(parts, fmt.Sprintf("No active approved Mattermost recipient matched %q.", recipientHint))
		return strings.Join(parts, " ")
	}
	parts = append(parts, fmt.Sprintf("Found %d active approved Mattermost recipient candidate(s): %s.", len(candidates), platformDMRecipientList(candidates)))
	return strings.Join(parts, " ")
}

func platformDMDiagnosis(snapshot platformDMInspectSnapshot) map[string]any {
	checks := []map[string]any{}
	checks = append(checks, platformDMCheck("mattermost_bot_token", snapshot.IsTokenConfigured, platformDMTokenSummary(snapshot.IsTokenConfigured)))
	checks = append(checks, platformDMPolicyCheck(snapshot))
	checks = append(checks, platformDMMattermostBotCheck(snapshot))
	checks = append(checks, platformDMMattermostUserListCheck(snapshot))
	return map[string]any{
		"platform": "mattermost",
		"ok":       platformDMChecksAreOK(checks),
		"checks":   checks,
	}
}

func platformDMPolicyCheck(snapshot platformDMInspectSnapshot) map[string]any {
	if snapshot.PolicyError != nil {
		return platformDMFailedCheck("blueclaw_policy", snapshot.PolicyError)
	}
	return map[string]any{"name": "blueclaw_policy", "ok": true, "summary": fmt.Sprintf("%d approved people loaded", len(snapshot.PolicyDocument.People))}
}

func platformDMMattermostBotCheck(snapshot platformDMInspectSnapshot) map[string]any {
	if snapshot.BotUserError != nil {
		return platformDMFailedCheck("mattermost_bot_user", snapshot.BotUserError)
	}
	return map[string]any{"name": "mattermost_bot_user", "ok": true, "summary": "bot user resolved as " + firstNonEmpty(snapshot.BotUser.Username, snapshot.BotUser.ID)}
}

func platformDMMattermostUserListCheck(snapshot platformDMInspectSnapshot) map[string]any {
	if snapshot.MattermostUsersError != nil {
		return platformDMFailedCheck("mattermost_user_list", snapshot.MattermostUsersError)
	}
	return map[string]any{"name": "mattermost_user_list", "ok": true, "summary": fmt.Sprintf("%d active users loaded", len(snapshot.MattermostUsers))}
}

func platformDMCheck(name string, isOK bool, summary string) map[string]any {
	return map[string]any{"name": name, "ok": isOK, "summary": summary}
}

func platformDMTokenSummary(isConfigured bool) string {
	if isConfigured {
		return "Mattermost bot token is configured"
	}
	return "Mattermost bot token is not configured"
}

func platformDMFailedCheck(name string, errorValue error) map[string]any {
	return map[string]any{"name": name, "ok": false, "summary": safePlatformDMError(errorValue)}
}

func platformDMChecksAreOK(checks []map[string]any) bool {
	for _, check := range checks {
		isOK, _ := check["ok"].(bool)
		if !isOK {
			return false
		}
	}
	return true
}

func platformDMDiagnosisSummary(result map[string]any) string {
	checks, _ := result["checks"].([]map[string]any)
	failedChecks := []string{}
	for _, check := range checks {
		isOK, _ := check["ok"].(bool)
		if isOK {
			continue
		}
		name, _ := check["name"].(string)
		summary, _ := check["summary"].(string)
		failedChecks = append(failedChecks, name+": "+summary)
	}
	if len(failedChecks) == 0 {
		return "Mattermost DM diagnostics passed."
	}
	return "Mattermost DM diagnostics found issues: " + strings.Join(failedChecks, "; ")
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
		return "", platformDMFailureForError("mattermost_lookup", "mattermost_unavailable", errorValue, true), true
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
