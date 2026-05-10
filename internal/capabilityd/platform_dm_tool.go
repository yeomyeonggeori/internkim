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

func (service Service) invokePlatformDMSend(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodePlatformDMSendInput(request.Input)
	if errorValue != nil {
		return platformDMErrorResponse(request.ToolName, errorValue.Error()), nil
	}
	recipient, errorValue := service.resolvePlatformDMRecipient(ctx, input)
	if errorValue != nil {
		return platformDMErrorResponse(request.ToolName, errorValue.Error()), nil
	}
	if errorMessage := validatePlatformDMSendAuthorization(request.Context, recipient); errorMessage != "" {
		return platformDMDeniedResponse(request.ToolName, errorMessage), nil
	}
	dispatchID, errorValue := service.sendMattermostDirectMessageWithDispatch(ctx, recipient.MattermostUserID, input.Message)
	if errorValue != nil {
		return platformDMErrorResponse(request.ToolName, errorValue.Error()), nil
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

func (service Service) resolvePlatformDMRecipient(ctx context.Context, input platformDMSendInput) (platformDMRecipient, error) {
	policyDocument, errorValue := service.fetchPlatformDMPolicy(ctx)
	if errorValue != nil {
		return platformDMRecipient{}, errorValue
	}
	mattermostUsers, errorValue := service.listPlatformDMMattermostUsers(ctx)
	if errorValue != nil {
		return platformDMRecipient{}, errorValue
	}
	candidates := matchingPlatformDMRecipients(input.RecipientHint, policyDocument.People, mattermostUsers)
	switch len(candidates) {
	case 0:
		return platformDMRecipient{}, fmt.Errorf("recipient %q was not found among approved InternKim people with active Mattermost accounts", input.RecipientHint)
	case 1:
		return candidates[0], nil
	default:
		return platformDMRecipient{}, fmt.Errorf("recipient %q is ambiguous: %s", input.RecipientHint, platformDMRecipientList(candidates))
	}
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

func normalizePlatformDMMatchValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func (service Service) sendMattermostDirectMessageWithDispatch(ctx context.Context, userID string, message string) (string, error) {
	botUser, errorValue := service.resolveMattermostBotUser(ctx)
	if errorValue != nil {
		return "", errorValue
	}
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return "", fmt.Errorf("mattermost user ID is required")
	}
	if normalizedUserID == botUser.ID {
		return "", fmt.Errorf("cannot send a direct message to the InternKim bot user")
	}
	channelID, errorValue := service.createMattermostDirectChannel(ctx, botUser.ID, normalizedUserID)
	if errorValue != nil {
		return "", errorValue
	}
	var response struct {
		ID string `json:"id"`
	}
	body := map[string]string{"channel_id": channelID, "message": message}
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/posts", body, &response); errorValue != nil {
		return "", errorValue
	}
	if strings.TrimSpace(response.ID) == "" {
		return "", fmt.Errorf("mattermost direct message did not return a post ID")
	}
	return response.ID, nil
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

func platformDMDeniedResponse(toolName string, message string) capabilities.ToolInvokeResponse {
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        toolName,
		Status:          "denied",
		Content:         message,
		IsError:         true,
	}
}

func platformDMErrorResponse(toolName string, message string) capabilities.ToolInvokeResponse {
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        toolName,
		Status:          "error",
		Content:         message,
		IsError:         true,
	}
}
