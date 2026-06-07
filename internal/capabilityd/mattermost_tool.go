package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type mattermostChannelUpdateInput struct {
	ChannelID    string    `json:"channelID"`
	ChannelName  string    `json:"channelName"`
	Header       *string   `json:"header"`
	DisplayName  *string   `json:"displayName"`
	InviteeHints *[]string `json:"inviteeHints"`
}

type mattermostPostSearchInput struct {
	Scope       string `json:"scope"`
	ChannelID   string `json:"channelID"`
	ChannelName string `json:"channelName"`
	PersonHint  string `json:"personHint"`
	RootPostID  string `json:"rootPostID"`
	AuthoredBy  string `json:"authoredBy"`
	Query       string `json:"query"`
	Limit       int    `json:"limit"`
}

type mattermostPostUpdateInput struct {
	PostID   string  `json:"postID"`
	Message  *string `json:"message"`
	IsPinned *bool   `json:"isPinned"`
}

type mattermostPostDeleteInput struct {
	PostIDs []string `json:"postIDs"`
}

type mattermostToolChannel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Header      string `json:"header"`
	TeamID      string `json:"team_id"`
	Type        string `json:"type"`
}

type mattermostToolPost struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id"`
	ChannelID string         `json:"channel_id"`
	Message   string         `json:"message"`
	RootID    string         `json:"root_id"`
	Type      string         `json:"type"`
	CreateAt  int64          `json:"create_at"`
	UpdateAt  int64          `json:"update_at"`
	DeleteAt  int64          `json:"delete_at"`
	IsPinned  bool           `json:"is_pinned"`
	Props     map[string]any `json:"props"`
}

type mattermostToolPostsResponse struct {
	Order []string                      `json:"order"`
	Posts map[string]mattermostToolPost `json:"posts"`
}

type mattermostToolPolicyDocument struct {
	People []mattermostToolPolicyPerson `json:"people"`
}

type mattermostToolPolicyPerson struct {
	PersonID    string   `json:"personID"`
	DisplayName string   `json:"displayName"`
	Emails      []string `json:"emails"`
	Circles     []string `json:"circles"`
	IsAdmin     bool     `json:"isAdmin"`
}

type mattermostToolFailure struct {
	ErrorCode    string `json:"errorCode"`
	FailureStage string `json:"failureStage"`
	Message      string `json:"message"`
}

type mattermostPostDeleteFailure struct {
	PostID       string `json:"postID"`
	ErrorCode    string `json:"errorCode"`
	FailureStage string `json:"failureStage"`
	Message      string `json:"message"`
}

type mattermostPostSearchCandidate struct {
	PostID          string `json:"postID"`
	ChannelID       string `json:"channelID"`
	RootID          string `json:"rootID"`
	UserID          string `json:"userID"`
	AuthoredBy      string `json:"authoredBy"`
	CreateAt        int64  `json:"createAt"`
	Preview         string `json:"preview"`
	Deletable       bool   `json:"deletable"`
	ProtectedReason string `json:"protectedReason,omitempty"`
}

const (
	mattermostToolStaffCircle = "staff"
	mattermostToolAdminCircle = "admin"
)

func isMattermostTool(toolName string) bool {
	switch toolName {
	case "mattermost.channel.update":
		return true
	default:
		return false
	}
}

func (service Service) invokeMattermostTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if response, isDenied := service.authorizeMattermostTool(ctx, request); isDenied {
		return response, nil
	}
	switch request.ToolName {
	case "mattermost.channel.update":
		return service.invokeMattermostChannelUpdate(ctx, request)
	default:
		return capabilities.ToolInvokeResponse{}, fmt.Errorf("mattermost tool is not configured: %s", request.ToolName)
	}
}

func (service Service) authorizeMattermostTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, bool) {
	requiredCircle := mattermostRequiredCircle(request.ToolName)
	if !service.mattermostRequesterHasCircle(ctx, request.Context, requiredCircle) {
		message := "mattermost tool requires " + requiredCircle + " access"
		return mattermostToolDeniedResponse(request.ToolName, mattermostToolStaticFailure(capabilities.CapabilityNotAllowed, "authorization", message)), true
	}
	if mattermostToolRequiresApproval(request.ToolName) && !request.Context.IsApprovalContinuation {
		message := request.ToolName + " requires approval before execution"
		return mattermostToolDeniedResponse(request.ToolName, mattermostToolStaticFailure("approval_required", "authorization", message)), true
	}
	return capabilities.ToolInvokeResponse{}, false
}

func mattermostRequiredCircle(toolName string) string {
	if toolName == "mattermost.channel.update" {
		return mattermostToolAdminCircle
	}
	return mattermostToolStaffCircle
}

func mattermostToolRequiresApproval(toolName string) bool {
	switch toolName {
	case "mattermost.channel.update":
		return true
	default:
		return false
	}
}

func (service Service) invokeMattermostContextInspect(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if !strings.EqualFold(strings.TrimSpace(request.Context.Platform), "mattermost") {
		failure := mattermostToolStaticFailure("platform_unavailable", "context", "current task is not running in Mattermost")
		return mattermostToolErrorResponse(request.ToolName, failure), nil
	}
	botUser, errorValue := service.resolveMattermostBotUser(ctx)
	if errorValue != nil {
		return mattermostToolErrorResponse(request.ToolName, mattermostToolFailureForError("bot_lookup", "mattermost_unavailable", errorValue)), nil
	}
	handle, _, _ := mattermostHandleFromContext(request.Context)
	result := map[string]any{
		"platform":                "mattermost",
		"conversationID":          request.Context.ConversationID,
		"conversationType":        request.Context.ConversationType,
		"channelID":               firstNonEmpty(strings.TrimSpace(request.Context.ChannelID), strings.TrimSpace(handle.ChannelID)),
		"channelName":             request.Context.ChannelName,
		"replyTargetID":           request.Context.ReplyTargetID,
		"rootPostID":              handle.RootID,
		"currentPostID":           handle.MessageID,
		"requesterPersonID":       request.Context.RequesterPersonID,
		"requesterPlatformUserID": request.Context.RequesterPlatformUserID,
		"botUserID":               botUser.ID,
		"botUsername":             botUser.Username,
	}
	return mattermostToolSuccessResponse(request.ToolName, "ok", result), nil
}

func (service Service) invokeMattermostPostSearch(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeMattermostPostSearchInput(request.Input)
	if errorValue != nil {
		return mattermostToolErrorResponse(request.ToolName, mattermostToolStaticFailure("invalid_input", "input_decode", errorValue.Error())), nil
	}
	searchHandle, channel, failure, hasFailure := service.resolveMattermostPostSearchHandle(ctx, request.Context, input)
	if hasFailure {
		return mattermostToolErrorResponse(request.ToolName, failure), nil
	}
	posts, failure, hasFailure := service.searchMattermostPosts(ctx, searchHandle, normalizedMattermostToolSearchLimit(input.Limit))
	if hasFailure {
		return mattermostToolErrorResponse(request.ToolName, failure), nil
	}
	candidates := service.mattermostPostSearchCandidates(ctx, posts, input, request.Context)
	result := map[string]any{
		"scope":          normalizedMattermostPostSearchScope(input.Scope, request.Context),
		"channel":        channel,
		"rootPostID":     searchHandle.RootID,
		"query":          input.Query,
		"authoredBy":     input.AuthoredBy,
		"candidateCount": len(candidates),
		"candidates":     candidates,
	}
	return mattermostToolSuccessResponse(request.ToolName, "ok", result), nil
}

func (service Service) invokeMattermostPostUpdate(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeMattermostPostUpdateInput(request.Input)
	if errorValue != nil {
		return mattermostToolErrorResponse(request.ToolName, mattermostToolStaticFailure("invalid_input", "input_decode", errorValue.Error())), nil
	}
	post, failure, hasFailure := service.mattermostToolPost(ctx, input.PostID)
	if hasFailure {
		return mattermostToolErrorResponse(request.ToolName, failure), nil
	}
	if failure, isBlocked := service.validateMattermostPostUpdate(ctx, post, input); isBlocked {
		return mattermostToolDeniedResponse(request.ToolName, failure), nil
	}
	if input.Message != nil {
		body := map[string]any{"message": strings.TrimSpace(*input.Message)}
		path := "/api/v4/posts/" + url.PathEscape(input.PostID) + "/patch"
		if errorValue := service.mattermostRequest(ctx, http.MethodPut, path, body, nil); errorValue != nil {
			return mattermostToolErrorResponse(request.ToolName, mattermostToolFailureForError("post_update", "mattermost_unavailable", errorValue)), nil
		}
	}
	if input.IsPinned != nil {
		if errorValue := service.updateMattermostPostPinState(ctx, input.PostID, *input.IsPinned); errorValue != nil {
			return mattermostToolErrorResponse(request.ToolName, mattermostToolFailureForError("post_pin", "mattermost_unavailable", errorValue)), nil
		}
	}
	result := map[string]any{"postID": input.PostID, "isPinned": input.IsPinned, "messageUpdated": input.Message != nil}
	return mattermostToolSuccessResponse(request.ToolName, "updated", result), nil
}

func (service Service) invokeMattermostPostDelete(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeMattermostPostDeleteInput(request.Input)
	if errorValue != nil {
		return mattermostToolErrorResponse(request.ToolName, mattermostToolStaticFailure("invalid_input", "input_decode", errorValue.Error())), nil
	}
	deletedPostIDs := []string{}
	failedPosts := []mattermostPostDeleteFailure{}
	for _, postID := range input.PostIDs {
		if failure, hasFailure := service.deleteMattermostPost(ctx, postID); hasFailure {
			failedPosts = append(failedPosts, mattermostPostDeleteFailure{
				PostID:       postID,
				ErrorCode:    failure.ErrorCode,
				FailureStage: failure.FailureStage,
				Message:      failure.Message,
			})
			continue
		}
		deletedPostIDs = append(deletedPostIDs, postID)
	}
	if len(deletedPostIDs) == 0 {
		result := map[string]any{"deletedCount": 0, "failedCount": len(failedPosts), "failedPosts": failedPosts}
		resultDocument, _ := json.Marshal(result)
		failure := mattermostToolStaticFailure("post_delete_not_completed", "post_delete", "no Mattermost posts were deleted")
		response := mattermostToolErrorResponse(request.ToolName, failure)
		response.Result = resultDocument
		return response, nil
	}
	result := map[string]any{
		"deletedPostIDs": deletedPostIDs,
		"deletedCount":   len(deletedPostIDs),
		"failedPosts":    failedPosts,
		"failedCount":    len(failedPosts),
	}
	return mattermostToolSuccessResponse(request.ToolName, "deleted", result), nil
}

func (service Service) invokeMattermostChannelUpdate(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeMattermostChannelUpdateInput(request.Input)
	if errorValue != nil {
		return mattermostToolErrorResponse(request.ToolName, mattermostToolStaticFailure("invalid_input", "input_decode", errorValue.Error())), nil
	}
	channel, failure, hasFailure := service.resolveMattermostToolChannel(ctx, input.ChannelID, input.ChannelName)
	if hasFailure {
		return mattermostToolErrorResponse(request.ToolName, failure), nil
	}
	if failure, isBlocked := validateMattermostChannelUpdate(channel, input); isBlocked {
		return mattermostToolDeniedResponse(request.ToolName, failure), nil
	}
	if input.Header != nil || input.DisplayName != nil {
		body := map[string]string{}
		if input.Header != nil {
			body["header"] = strings.TrimSpace(*input.Header)
		}
		if input.DisplayName != nil {
			body["display_name"] = strings.TrimSpace(*input.DisplayName)
		}
		if errorValue := service.mattermostRequest(ctx, http.MethodPut, "/api/v4/channels/"+url.PathEscape(channel.ID)+"/patch", body, nil); errorValue != nil {
			return mattermostToolErrorResponse(request.ToolName, mattermostToolFailureForError("channel_update", "mattermost_unavailable", errorValue)), nil
		}
	}
	invitedUsers, failure, hasFailure := service.inviteMattermostChannelUsers(ctx, channel.ID, mattermostToolInviteeHints(input))
	if hasFailure {
		return mattermostToolErrorResponse(request.ToolName, failure), nil
	}
	result := map[string]any{"channelID": channel.ID, "updated": true, "invitedUsers": invitedUsers}
	return mattermostToolSuccessResponse(request.ToolName, "updated", result), nil
}

func decodeMattermostChannelUpdateInput(document json.RawMessage) (mattermostChannelUpdateInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return mattermostChannelUpdateInput{}, fmt.Errorf("mattermost.channel.update input is required")
	}
	var input mattermostChannelUpdateInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return mattermostChannelUpdateInput{}, errorValue
	}
	input.ChannelID = strings.TrimSpace(input.ChannelID)
	input.ChannelName = strings.TrimSpace(input.ChannelName)
	if errorValue := validateMattermostChannelReference(input.ChannelID, input.ChannelName); errorValue != nil {
		return mattermostChannelUpdateInput{}, errorValue
	}
	if input.Header == nil && input.DisplayName == nil && len(mattermostToolInviteeHints(input)) == 0 {
		return mattermostChannelUpdateInput{}, fmt.Errorf("header, displayName, or inviteeHints is required")
	}
	return input, nil
}

func decodeMattermostPostSearchInput(document json.RawMessage) (mattermostPostSearchInput, error) {
	var input mattermostPostSearchInput
	if len(bytes.TrimSpace(document)) > 0 {
		if errorValue := json.Unmarshal(document, &input); errorValue != nil {
			return mattermostPostSearchInput{}, errorValue
		}
	}
	input.Scope = strings.TrimSpace(input.Scope)
	input.ChannelID = strings.TrimSpace(input.ChannelID)
	input.ChannelName = strings.TrimSpace(input.ChannelName)
	input.PersonHint = strings.TrimSpace(input.PersonHint)
	input.RootPostID = strings.TrimSpace(input.RootPostID)
	input.AuthoredBy = strings.TrimSpace(input.AuthoredBy)
	input.Query = strings.TrimSpace(input.Query)
	if !isValidMattermostPostSearchScope(input.Scope) {
		return mattermostPostSearchInput{}, fmt.Errorf("scope must be currentThread, currentChannel, directMessage, or channel")
	}
	if !isValidMattermostPostSearchAuthor(input.AuthoredBy) {
		return mattermostPostSearchInput{}, fmt.Errorf("authoredBy must be internkim, requester, or anyone")
	}
	return input, nil
}

func decodeMattermostPostUpdateInput(document json.RawMessage) (mattermostPostUpdateInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return mattermostPostUpdateInput{}, fmt.Errorf("platform.message.update input is required")
	}
	var input mattermostPostUpdateInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return mattermostPostUpdateInput{}, errorValue
	}
	input.PostID = strings.TrimSpace(input.PostID)
	if input.PostID == "" {
		return mattermostPostUpdateInput{}, fmt.Errorf("postID is required")
	}
	if input.Message == nil && input.IsPinned == nil {
		return mattermostPostUpdateInput{}, fmt.Errorf("message or isPinned is required")
	}
	if input.Message != nil && strings.TrimSpace(*input.Message) == "" {
		return mattermostPostUpdateInput{}, fmt.Errorf("message cannot be empty")
	}
	return input, nil
}

func decodeMattermostPostDeleteInput(document json.RawMessage) (mattermostPostDeleteInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return mattermostPostDeleteInput{}, fmt.Errorf("platform.message.delete input is required")
	}
	var input mattermostPostDeleteInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return mattermostPostDeleteInput{}, errorValue
	}
	input.PostIDs = uniqueTrimmedMattermostPostIDs(input.PostIDs)
	if len(input.PostIDs) == 0 {
		return mattermostPostDeleteInput{}, fmt.Errorf("postIDs is required")
	}
	return input, nil
}

func isValidMattermostPostSearchScope(scope string) bool {
	switch strings.TrimSpace(scope) {
	case "", "currentThread", "currentChannel", "directMessage", "channel":
		return true
	default:
		return false
	}
}

func isValidMattermostPostSearchAuthor(author string) bool {
	switch strings.TrimSpace(author) {
	case "", "internkim", "requester", "anyone":
		return true
	default:
		return false
	}
}

func uniqueTrimmedMattermostPostIDs(postIDs []string) []string {
	result := []string{}
	seenPostID := map[string]bool{}
	for _, postID := range postIDs {
		postID = strings.TrimSpace(postID)
		if postID == "" || seenPostID[postID] {
			continue
		}
		seenPostID[postID] = true
		result = append(result, postID)
	}
	return result
}

func validateMattermostChannelReference(channelID string, channelName string) error {
	if strings.TrimSpace(channelID) == "" && strings.TrimSpace(channelName) == "" {
		return fmt.Errorf("channelID or channelName is required")
	}
	if strings.TrimSpace(channelID) != "" && strings.TrimSpace(channelName) != "" {
		return fmt.Errorf("use either channelID or channelName, not both")
	}
	return nil
}

func (service Service) resolveMattermostToolChannel(ctx context.Context, channelID string, channelName string) (mattermostToolChannel, mattermostToolFailure, bool) {
	if strings.TrimSpace(channelID) != "" {
		var channel mattermostToolChannel
		if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/channels/"+url.PathEscape(channelID), nil, &channel); errorValue != nil {
			return mattermostToolChannel{}, mattermostToolFailureForError("channel_lookup", "mattermost_unavailable", errorValue), true
		}
		if strings.TrimSpace(channel.ID) == "" {
			return mattermostToolChannel{}, mattermostToolStaticFailure("channel_not_found", "channel_lookup", "mattermost channel was not found"), true
		}
		return channel, mattermostToolFailure{}, false
	}
	channels, errorValue := service.joinedMattermostToolChannels(ctx)
	if errorValue != nil {
		return mattermostToolChannel{}, mattermostToolFailureForError("channel_lookup", "mattermost_unavailable", errorValue), true
	}
	matches := matchingMattermostToolChannels(channelName, channels)
	switch len(matches) {
	case 0:
		return mattermostToolChannel{}, mattermostToolStaticFailure("channel_not_found", "channel_lookup", "mattermost channel was not found"), true
	case 1:
		return matches[0], mattermostToolFailure{}, false
	default:
		return mattermostToolChannel{}, mattermostToolStaticFailure("channel_ambiguous", "channel_lookup", "mattermost channel name is ambiguous"), true
	}
}

func (service Service) joinedMattermostToolChannels(ctx context.Context) ([]mattermostToolChannel, error) {
	var channels []mattermostToolChannel
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/me/channels?per_page=200", nil, &channels); errorValue != nil {
		return nil, errorValue
	}
	return channels, nil
}

func matchingMattermostToolChannels(channelName string, channels []mattermostToolChannel) []mattermostToolChannel {
	normalizedName := normalizePlatformDMMatchValue(channelName)
	matches := []mattermostToolChannel{}
	for _, channel := range channels {
		if normalizePlatformDMMatchValue(channel.Name) == normalizedName || normalizePlatformDMMatchValue(channel.DisplayName) == normalizedName {
			matches = append(matches, channel)
		}
	}
	sort.Slice(matches, func(leftIndex int, rightIndex int) bool {
		return matches[leftIndex].ID < matches[rightIndex].ID
	})
	return matches
}

func (service Service) resolveMattermostPostSearchHandle(ctx context.Context, toolContext capabilities.ToolInvokeContext, input mattermostPostSearchInput) (platformHandle, map[string]string, mattermostToolFailure, bool) {
	scope := normalizedMattermostPostSearchScope(input.Scope, toolContext)
	switch scope {
	case "currentThread":
		return service.resolveCurrentMattermostThreadHandle(toolContext, input)
	case "currentChannel":
		return service.resolveCurrentMattermostChannelHandle(toolContext, input)
	case "directMessage":
		return service.resolveMattermostDirectMessageSearchHandle(ctx, toolContext, input)
	case "channel":
		return service.resolveMattermostChannelSearchHandle(ctx, input)
	default:
		failure := mattermostToolStaticFailure("invalid_scope", "input_decode", "Mattermost search scope is not supported")
		return platformHandle{}, nil, failure, true
	}
}

func normalizedMattermostPostSearchScope(scope string, toolContext capabilities.ToolInvokeContext) string {
	scope = strings.TrimSpace(scope)
	if scope != "" {
		return scope
	}
	if isMattermostDirectConversation(toolContext) {
		return "directMessage"
	}
	handle, _, hasHandle := mattermostHandleFromContext(toolContext)
	if hasHandle && strings.TrimSpace(handle.RootID) != "" {
		return "currentThread"
	}
	return "currentChannel"
}

func (service Service) resolveCurrentMattermostThreadHandle(toolContext capabilities.ToolInvokeContext, input mattermostPostSearchInput) (platformHandle, map[string]string, mattermostToolFailure, bool) {
	handle, failure, hasFailure := requiredMattermostHandleFromContext(toolContext)
	if hasFailure {
		return platformHandle{}, nil, failure, true
	}
	rootPostID := firstNonEmpty(input.RootPostID, handle.RootID)
	if strings.TrimSpace(rootPostID) == "" {
		failure := mattermostToolStaticFailure("thread_not_available", "context", "current Mattermost context does not have a thread root")
		return platformHandle{}, nil, failure, true
	}
	handle.RootID = rootPostID
	channelID := firstNonEmpty(handle.ChannelID, toolContext.ChannelID)
	handle.ChannelID = channelID
	channel := map[string]string{"id": channelID, "name": toolContext.ChannelName, "type": toolContext.ConversationType}
	return handle, channel, mattermostToolFailure{}, false
}

func (service Service) resolveCurrentMattermostChannelHandle(toolContext capabilities.ToolInvokeContext, input mattermostPostSearchInput) (platformHandle, map[string]string, mattermostToolFailure, bool) {
	channelID := firstNonEmpty(input.ChannelID, toolContext.ChannelID)
	if strings.TrimSpace(channelID) == "" {
		failure := mattermostToolStaticFailure("channel_not_available", "context", "current Mattermost channel is not available")
		return platformHandle{}, nil, failure, true
	}
	handle := platformHandle{Platform: "mattermost", ConversationID: toolContext.ConversationID, ChannelID: channelID, ChannelType: toolContext.ConversationType}
	channel := map[string]string{"id": channelID, "name": toolContext.ChannelName, "type": toolContext.ConversationType}
	return handle, channel, mattermostToolFailure{}, false
}

func (service Service) resolveMattermostDirectMessageSearchHandle(ctx context.Context, toolContext capabilities.ToolInvokeContext, input mattermostPostSearchInput) (platformHandle, map[string]string, mattermostToolFailure, bool) {
	if strings.TrimSpace(input.PersonHint) == "" && isMattermostDirectConversation(toolContext) {
		return service.resolveCurrentMattermostChannelHandle(toolContext, input)
	}
	recipient, failure, hasFailure := service.resolvePlatformDMRecipient(ctx, input.PersonHint)
	if hasFailure {
		return platformHandle{}, nil, failureFromPlatformDMFailure(failure), true
	}
	channel, toolFailure, hasToolFailure := service.mattermostDirectChannelForRecipient(ctx, recipient)
	if hasToolFailure {
		return platformHandle{}, nil, toolFailure, true
	}
	handle := platformHandle{Platform: "mattermost", ConversationID: namespacedConversationID("D", channel.ID), ChannelID: channel.ID, ChannelType: "D"}
	channelResult := map[string]string{"id": channel.ID, "name": channel.Name, "displayName": channel.DisplayName, "type": "D"}
	return handle, channelResult, mattermostToolFailure{}, false
}

func (service Service) resolveMattermostChannelSearchHandle(ctx context.Context, input mattermostPostSearchInput) (platformHandle, map[string]string, mattermostToolFailure, bool) {
	channel, failure, hasFailure := service.resolveMattermostToolChannel(ctx, input.ChannelID, input.ChannelName)
	if hasFailure {
		return platformHandle{}, nil, failure, true
	}
	handle := platformHandle{Platform: "mattermost", ConversationID: namespacedConversationID(channel.Type, channel.ID), ChannelID: channel.ID, ChannelType: channel.Type}
	channelResult := map[string]string{"id": channel.ID, "name": channel.Name, "displayName": channel.DisplayName, "type": channel.Type}
	return handle, channelResult, mattermostToolFailure{}, false
}

func (service Service) mattermostDirectChannelForRecipient(ctx context.Context, recipient platformDMRecipient) (mattermostToolChannel, mattermostToolFailure, bool) {
	botUser, errorValue := service.resolveMattermostBotUser(ctx)
	if errorValue != nil {
		return mattermostToolChannel{}, mattermostToolFailureForError("bot_lookup", "mattermost_unavailable", errorValue), true
	}
	channels, errorValue := service.joinedMattermostToolChannels(ctx)
	if errorValue != nil {
		return mattermostToolChannel{}, mattermostToolFailureForError("channel_lookup", "mattermost_unavailable", errorValue), true
	}
	for _, channel := range channels {
		if !strings.EqualFold(strings.TrimSpace(channel.Type), "D") {
			continue
		}
		normalizedName := normalizePlatformDMMatchValue(channel.Name)
		if strings.Contains(normalizedName, normalizePlatformDMMatchValue(botUser.ID)) && strings.Contains(normalizedName, normalizePlatformDMMatchValue(recipient.MattermostUserID)) {
			return channel, mattermostToolFailure{}, false
		}
	}
	message := "Mattermost direct message channel with " + firstNonEmpty(recipient.DisplayName, recipient.MattermostUsername, recipient.PersonID) + " was not found"
	return mattermostToolChannel{}, mattermostToolStaticFailure("direct_message_not_found", "channel_lookup", message), true
}

func isMattermostDirectConversation(toolContext capabilities.ToolInvokeContext) bool {
	return strings.EqualFold(strings.TrimSpace(toolContext.ConversationType), "D")
}

func requiredMattermostHandleFromContext(toolContext capabilities.ToolInvokeContext) (platformHandle, mattermostToolFailure, bool) {
	handle, errorValue, hasHandle := mattermostHandleFromContext(toolContext)
	if !hasHandle {
		return platformHandle{}, mattermostToolStaticFailure("mattermost_context_missing", "context", "current Mattermost context is not available"), true
	}
	if errorValue != nil {
		return platformHandle{}, mattermostToolFailureForError("context", "invalid_reply_target", errorValue), true
	}
	return handle, mattermostToolFailure{}, false
}

func mattermostHandleFromContext(toolContext capabilities.ToolInvokeContext) (platformHandle, error, bool) {
	if strings.TrimSpace(toolContext.ReplyTargetID) == "" {
		return platformHandle{}, nil, false
	}
	handle, errorValue := decodePlatformHandle(toolContext.ReplyTargetID)
	return handle, errorValue, true
}

func (service Service) searchMattermostPosts(ctx context.Context, handle platformHandle, limit int) ([]mattermostToolPost, mattermostToolFailure, bool) {
	var response mattermostToolPostsResponse
	path := "/api/v4/channels/" + url.PathEscape(handle.ChannelID) + "/posts?page=0&per_page=" + strconv.Itoa(limit)
	if strings.TrimSpace(handle.RootID) != "" {
		path = "/api/v4/posts/" + url.PathEscape(handle.RootID) + "/thread"
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, path, nil, &response); errorValue != nil {
		return nil, mattermostToolFailureForError("post_search", "mattermost_unavailable", errorValue), true
	}
	posts := mattermostPostsFromResponse(response, limit)
	return posts, mattermostToolFailure{}, false
}

func mattermostPostsFromResponse(response mattermostToolPostsResponse, limit int) []mattermostToolPost {
	posts := []mattermostToolPost{}
	for _, postID := range response.Order {
		post, exists := response.Posts[postID]
		if !exists || strings.TrimSpace(post.Type) != "" || post.DeleteAt > 0 {
			continue
		}
		posts = append(posts, post)
		if len(posts) >= limit {
			break
		}
	}
	return posts
}

func (service Service) mattermostPostSearchCandidates(ctx context.Context, posts []mattermostToolPost, input mattermostPostSearchInput, toolContext capabilities.ToolInvokeContext) []mattermostPostSearchCandidate {
	candidates := []mattermostPostSearchCandidate{}
	botUser, _ := service.resolveMattermostBotUser(ctx)
	for _, post := range posts {
		if !mattermostPostMatchesSearchAuthor(post, botUser.ID, toolContext.RequesterPlatformUserID, input.AuthoredBy) {
			continue
		}
		candidates = append(candidates, service.mattermostPostSearchCandidate(ctx, post, botUser.ID))
	}
	return candidates
}

func (service Service) mattermostPostSearchCandidate(ctx context.Context, post mattermostToolPost, botUserID string) mattermostPostSearchCandidate {
	failure, isBlocked := service.validateMattermostPostDeleteWithBotUserID(post, botUserID)
	candidate := mattermostPostSearchCandidate{
		PostID:     post.ID,
		ChannelID:  post.ChannelID,
		RootID:     post.RootID,
		UserID:     post.UserID,
		AuthoredBy: mattermostPostAuthorLabel(post, botUserID),
		CreateAt:   post.CreateAt,
		Preview:    clippedMattermostPostPreview(post.Message),
		Deletable:  !isBlocked,
	}
	if isBlocked {
		candidate.ProtectedReason = failure.Message
	}
	return candidate
}

func mattermostPostMatchesSearchAuthor(post mattermostToolPost, botUserID string, requesterPlatformUserID string, authoredBy string) bool {
	switch strings.TrimSpace(authoredBy) {
	case "", "anyone":
		return true
	case "internkim":
		return strings.TrimSpace(botUserID) != "" && strings.TrimSpace(post.UserID) == strings.TrimSpace(botUserID)
	case "requester":
		return strings.TrimSpace(requesterPlatformUserID) != "" && strings.TrimSpace(post.UserID) == strings.TrimSpace(requesterPlatformUserID)
	default:
		return true
	}
}

func mattermostPostAuthorLabel(post mattermostToolPost, botUserID string) string {
	if strings.TrimSpace(botUserID) != "" && strings.TrimSpace(post.UserID) == strings.TrimSpace(botUserID) {
		return "internkim"
	}
	return "user"
}

func clippedMattermostPostPreview(message string) string {
	message = strings.Join(strings.Fields(strings.TrimSpace(message)), " ")
	if len([]rune(message)) <= 240 {
		return message
	}
	runes := []rune(message)
	return string(runes[:240])
}

func normalizedMattermostToolSearchLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func (service Service) mattermostToolPost(ctx context.Context, postID string) (mattermostToolPost, mattermostToolFailure, bool) {
	var post mattermostToolPost
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/posts/"+url.PathEscape(postID), nil, &post); errorValue != nil {
		return mattermostToolPost{}, mattermostToolFailureForError("post_lookup", "mattermost_unavailable", errorValue), true
	}
	if strings.TrimSpace(post.ID) == "" {
		return mattermostToolPost{}, mattermostToolStaticFailure("post_not_found", "post_lookup", "mattermost post was not found"), true
	}
	return post, mattermostToolFailure{}, false
}

func (service Service) validateMattermostPostUpdate(ctx context.Context, post mattermostToolPost, input mattermostPostUpdateInput) (mattermostToolFailure, bool) {
	if isProtectedMattermostToolPost(post) {
		return mattermostToolStaticFailure("protected_post", "guardrail", "automated InternKim Flow, calendar, and attendance posts cannot be changed"), true
	}
	if input.Message == nil {
		return mattermostToolFailure{}, false
	}
	if service.isMattermostToolBotPost(ctx, post) {
		return mattermostToolFailure{}, false
	}
	return mattermostToolStaticFailure("not_bot_post", "authorization", "message updates are allowed only for InternKim bot posts"), true
}

func (service Service) validateMattermostPostDelete(ctx context.Context, post mattermostToolPost) (mattermostToolFailure, bool) {
	botUser, errorValue := service.resolveMattermostBotUser(ctx)
	if errorValue != nil {
		return mattermostToolFailureForError("bot_lookup", "mattermost_unavailable", errorValue), true
	}
	return service.validateMattermostPostDeleteWithBotUserID(post, botUser.ID)
}

func (service Service) validateMattermostPostDeleteWithBotUserID(post mattermostToolPost, botUserID string) (mattermostToolFailure, bool) {
	if isProtectedMattermostToolPost(post) {
		return mattermostToolStaticFailure("protected_post", "guardrail", "automated InternKim Flow, calendar, and attendance posts cannot be deleted"), true
	}
	if strings.TrimSpace(botUserID) != "" && strings.TrimSpace(post.UserID) == strings.TrimSpace(botUserID) {
		return mattermostToolFailure{}, false
	}
	return mattermostToolStaticFailure("not_bot_post", "authorization", "post deletion is allowed only for InternKim bot posts"), true
}

func (service Service) deleteMattermostPost(ctx context.Context, postID string) (mattermostToolFailure, bool) {
	post, failure, hasFailure := service.mattermostToolPost(ctx, postID)
	if hasFailure {
		return failure, true
	}
	if failure, isBlocked := service.validateMattermostPostDelete(ctx, post); isBlocked {
		return failure, true
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodDelete, "/api/v4/posts/"+url.PathEscape(postID), nil, nil); errorValue != nil {
		return mattermostToolFailureForError("post_delete", "mattermost_unavailable", errorValue), true
	}
	return mattermostToolFailure{}, false
}

func (service Service) isMattermostToolBotPost(ctx context.Context, post mattermostToolPost) bool {
	botUser, errorValue := service.resolveMattermostBotUser(ctx)
	return errorValue == nil && strings.TrimSpace(botUser.ID) != "" && strings.TrimSpace(post.UserID) == strings.TrimSpace(botUser.ID)
}

func (service Service) updateMattermostPostPinState(ctx context.Context, postID string, isPinned bool) error {
	action := "pin"
	if !isPinned {
		action = "unpin"
	}
	return service.mattermostRequest(ctx, http.MethodPost, "/api/v4/posts/"+url.PathEscape(postID)+"/"+action, nil, nil)
}

func validateMattermostChannelUpdate(channel mattermostToolChannel, input mattermostChannelUpdateInput) (mattermostToolFailure, bool) {
	if input.Header != nil && mattermostChannelHasManagedOpenLink(channel, *input.Header) {
		return mattermostToolStaticFailure("protected_channel_header", "guardrail", "managed Mattermost channel headers cannot be changed"), true
	}
	if input.DisplayName != nil && isDefaultMattermostToolChannel(channel.Name) {
		return mattermostToolStaticFailure("protected_channel_name", "guardrail", "default Mattermost channel names cannot be changed"), true
	}
	return mattermostToolFailure{}, false
}

func mattermostChannelHasManagedOpenLink(channel mattermostToolChannel, nextHeader string) bool {
	return containsMattermostManagedOpenLink(channel.Header) || containsMattermostManagedOpenLink(nextHeader)
}

func containsMattermostManagedOpenLink(value string) bool {
	normalizedValue := strings.ToLower(strings.TrimSpace(value))
	labels := []string{"업무 열기", "캘린더 열기", "출결 열기", "open flow", "open calendar", "open attendance"}
	for _, label := range labels {
		if strings.Contains(normalizedValue, strings.ToLower(label)) {
			return true
		}
	}
	return false
}

func isDefaultMattermostToolChannel(channelName string) bool {
	switch strings.ToLower(strings.TrimSpace(channelName)) {
	case "town-square", "off-topic", "flow", "calendar", "attendance", "announcements":
		return true
	default:
		return false
	}
}

func isProtectedMattermostToolPost(post mattermostToolPost) bool {
	protectedProperties := []string{
		"internkim_flow_entry",
		"internkim_attendance_entry",
		"internkim_calendar_event",
		"internkim_calendar_notification",
	}
	for _, property := range protectedProperties {
		if value, exists := post.Props[property]; exists && value != nil {
			return true
		}
	}
	return false
}

func (service Service) inviteMattermostChannelUsers(ctx context.Context, channelID string, inviteeHints []string) ([]map[string]string, mattermostToolFailure, bool) {
	if len(inviteeHints) == 0 {
		return []map[string]string{}, mattermostToolFailure{}, false
	}
	recipients := []platformDMRecipient{}
	for _, inviteeHint := range inviteeHints {
		recipient, failure, hasFailure := service.resolvePlatformDMRecipient(ctx, inviteeHint)
		if hasFailure {
			return nil, failureFromPlatformDMFailure(failure), true
		}
		recipients = append(recipients, recipient)
	}
	invitedUsers := []map[string]string{}
	for _, recipient := range recipients {
		body := map[string]string{"user_id": recipient.MattermostUserID}
		errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/channels/"+url.PathEscape(channelID)+"/members", body, nil)
		if errorValue != nil && !isMattermostToolAlreadyMemberError(errorValue) {
			return nil, mattermostToolFailureForError("channel_invite", "mattermost_unavailable", errorValue), true
		}
		invitedUsers = append(invitedUsers, map[string]string{
			"personID":           recipient.PersonID,
			"mattermostUserID":   recipient.MattermostUserID,
			"mattermostUsername": recipient.MattermostUsername,
		})
	}
	return invitedUsers, mattermostToolFailure{}, false
}

func isMattermostToolAlreadyMemberError(errorValue error) bool {
	message := strings.ToLower(errorValue.Error())
	return strings.Contains(message, "already")
}

func mattermostToolInviteeHints(input mattermostChannelUpdateInput) []string {
	if input.InviteeHints == nil {
		return []string{}
	}
	values := []string{}
	seenValue := map[string]bool{}
	for _, value := range *input.InviteeHints {
		trimmedValue := strings.TrimSpace(value)
		normalizedValue := normalizePlatformDMMatchValue(trimmedValue)
		if normalizedValue == "" || seenValue[normalizedValue] {
			continue
		}
		seenValue[normalizedValue] = true
		values = append(values, trimmedValue)
	}
	return values
}

func (service Service) mattermostRequesterHasCircle(ctx context.Context, toolContext capabilities.ToolInvokeContext, requiredCircle string) bool {
	policyDocument, errorValue := service.fetchMattermostToolPolicy(ctx)
	if errorValue != nil {
		return false
	}
	for _, person := range policyDocument.People {
		if !mattermostToolPersonMatchesContext(person, toolContext) {
			continue
		}
		return mattermostToolPersonHasCircle(person, requiredCircle)
	}
	return false
}

func (service Service) fetchMattermostToolPolicy(ctx context.Context) (mattermostToolPolicyDocument, error) {
	endpoint := strings.TrimRight(firstNonEmpty(service.Configuration.BlueclawBaseURL, DefaultConfiguration().BlueclawBaseURL), "/") + "/admin/api/policy"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if errorValue != nil {
		return mattermostToolPolicyDocument{}, errorValue
	}
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return mattermostToolPolicyDocument{}, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return mattermostToolPolicyDocument{}, fmt.Errorf("policy lookup failed with status %d", response.StatusCode)
	}
	var policyDocument mattermostToolPolicyDocument
	if errorValue := json.NewDecoder(response.Body).Decode(&policyDocument); errorValue != nil {
		return mattermostToolPolicyDocument{}, errorValue
	}
	return policyDocument, nil
}

func mattermostToolPersonMatchesContext(person mattermostToolPolicyPerson, toolContext capabilities.ToolInvokeContext) bool {
	if strings.TrimSpace(toolContext.RequesterPersonID) != "" && strings.TrimSpace(person.PersonID) == strings.TrimSpace(toolContext.RequesterPersonID) {
		return true
	}
	requesterEmail := normalizePlatformDMMatchValue(toolContext.RequesterEmail)
	if requesterEmail == "" {
		return false
	}
	for _, email := range person.Emails {
		if normalizePlatformDMMatchValue(email) == requesterEmail {
			return true
		}
	}
	return false
}

func mattermostToolPersonHasCircle(person mattermostToolPolicyPerson, requiredCircle string) bool {
	requiredCircle = strings.ToLower(strings.TrimSpace(requiredCircle))
	if requiredCircle == "" {
		return false
	}
	if requiredCircle == mattermostToolStaffCircle {
		return true
	}
	if requiredCircle == mattermostToolAdminCircle && person.IsAdmin {
		return true
	}
	for _, circle := range person.Circles {
		if strings.ToLower(strings.TrimSpace(circle)) == requiredCircle {
			return true
		}
	}
	return false
}

func normalizedMattermostToolPage(page int) int {
	if page < 0 {
		return 0
	}
	return page
}

func normalizedMattermostToolPerPage(perPage int) int {
	if perPage <= 0 {
		return 20
	}
	if perPage > 100 {
		return 100
	}
	return perPage
}

func mattermostToolSuccessResponse(toolName string, status string, result any) capabilities.ToolInvokeResponse {
	resultDocument, _ := json.Marshal(result)
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        toolName,
		Status:          status,
		Result:          resultDocument,
	}
}

func mattermostToolDeniedResponse(toolName string, failure mattermostToolFailure) capabilities.ToolInvokeResponse {
	response := mattermostToolErrorResponse(toolName, failure)
	response.Status = "denied"
	return response
}

func mattermostToolErrorResponse(toolName string, failure mattermostToolFailure) capabilities.ToolInvokeResponse {
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
		Result:          resultDocument,
	}
}

func mattermostToolStaticFailure(errorCode string, failureStage string, message string) mattermostToolFailure {
	return mattermostToolFailure{
		ErrorCode:    strings.TrimSpace(errorCode),
		FailureStage: strings.TrimSpace(failureStage),
		Message:      strings.TrimSpace(message),
	}
}

func mattermostToolFailureForError(failureStage string, errorCode string, errorValue error) mattermostToolFailure {
	return mattermostToolStaticFailure(errorCode, failureStage, errorValue.Error())
}

func failureFromPlatformDMFailure(failure platformDMFailure) mattermostToolFailure {
	return mattermostToolFailure{
		ErrorCode:    failure.ErrorCode,
		FailureStage: failure.FailureStage,
		Message:      failure.Message,
	}
}
