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

const platformMessageBroadcastRecipientLimit = 50

type platformMessageDeliveryTarget struct {
	Type        string   `json:"type"`
	PersonHint  string   `json:"personHint"`
	PersonHints []string `json:"personHints"`
	ChannelID   string   `json:"channelID"`
	ChannelName string   `json:"channelName"`
}

type platformMessageSearchInput struct {
	Scope       string   `json:"scope"`
	ChannelID   string   `json:"channelID"`
	ChannelName string   `json:"channelName"`
	PersonHint  string   `json:"personHint"`
	AuthoredBy  string   `json:"authoredBy"`
	Queries     []string `json:"queries"`
	Limit       int      `json:"limit"`
	Cursor      string   `json:"cursor"`

	DeliveryTarget platformMessageDeliveryTarget `json:"-"`
}

type platformMessageSendInput struct {
	TargetType  string   `json:"targetType"`
	Message     string   `json:"message"`
	ChannelID   string   `json:"channelID"`
	ChannelName string   `json:"channelName"`
	PersonHint  string   `json:"personHint"`
	PersonHints []string `json:"personHints"`
	Pin         bool     `json:"pin"`
	Reason      string   `json:"reason"`

	DeliveryTarget platformMessageDeliveryTarget `json:"-"`
}

type platformMessageUpdateInput struct {
	MessageID string  `json:"messageID"`
	Message   *string `json:"message"`
	IsPinned  *bool   `json:"isPinned"`
}

type platformMessageDeleteInput struct {
	MessageIDs []string `json:"messageIDs"`
}

func (service Service) invokePlatformMessageTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	switch request.ToolName {
	case "message.context":
		if response, isDenied := service.authorizePlatformMessageTool(ctx, request); isDenied {
			return response, nil
		}
		return service.invokeMattermostContextInspect(ctx, request)
	case "message.search":
		if response, isDenied := service.authorizePlatformMessageTool(ctx, request); isDenied {
			return response, nil
		}
		return service.invokePlatformMessageSearch(ctx, request)
	case "message.send":
		return service.invokePlatformMessageSend(ctx, request)
	case "message.update":
		if response, isDenied := service.authorizePlatformMessageTool(ctx, request); isDenied {
			return response, nil
		}
		return service.invokePlatformMessageUpdate(ctx, request)
	case "message.delete":
		if response, isDenied := service.authorizePlatformMessageTool(ctx, request); isDenied {
			return response, nil
		}
		return service.invokePlatformMessageDelete(ctx, request)
	default:
		return capabilities.ToolInvokeResponse{}, fmt.Errorf("platform message tool is not configured: %s", request.ToolName)
	}
}

func (service Service) authorizePlatformMessageTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, bool) {
	if !service.mattermostRequesterHasCircle(ctx, request.Context, mattermostToolStaffCircle) {
		message := request.ToolName + " requires staff access"
		return mattermostToolDeniedResponse(request.ToolName, mattermostToolStaticFailure(capabilities.CapabilityNotAllowed, "authorization", message)), true
	}
	return capabilities.ToolInvokeResponse{}, false
}

func (service Service) invokePlatformMessageSearch(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodePlatformMessageSearchInput(request.Input)
	if errorValue != nil {
		return mattermostToolErrorResponse(request.ToolName, mattermostToolStaticFailure("invalid_input", "input_decode", errorValue.Error())), nil
	}
	mattermostInput := mattermostPostSearchInput{
		Scope:       platformMessageSearchScope(input),
		ChannelID:   input.DeliveryTarget.ChannelID,
		ChannelName: input.DeliveryTarget.ChannelName,
		PersonHint:  input.DeliveryTarget.PersonHint,
		AuthoredBy:  platformMessageSearchAuthor(input.AuthoredBy),
		Queries:     input.Queries,
		Limit:       input.Limit,
		Cursor:      input.Cursor,
	}
	return service.invokeMattermostPostSearch(ctx, capabilities.ToolInvokeRequest{
		ToolName: request.ToolName,
		Input:    mustMarshalPlatformMessageInput(mattermostInput),
		Context:  request.Context,
	})
}

func (service Service) invokePlatformMessageSend(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodePlatformMessageSendInput(request.Input)
	if errorValue != nil {
		return mattermostToolErrorResponse(request.ToolName, mattermostToolStaticFailure("invalid_input", "input_decode", errorValue.Error())), nil
	}
	if input.DeliveryTarget.Type == "directMessage" {
		if len(input.DeliveryTarget.PersonHints) > 0 {
			return service.invokePlatformMessageDirectBroadcast(ctx, request, input)
		}
		return service.invokePlatformMessageDirectSend(ctx, request, input)
	}
	if response, isDenied := service.authorizePlatformMessageTool(ctx, request); isDenied {
		return response, nil
	}
	channelID, rootID, failure, hasFailure := service.resolvePlatformMessageSendTarget(ctx, request.Context, input.DeliveryTarget)
	if hasFailure {
		return mattermostToolErrorResponse(request.ToolName, failure), nil
	}
	result, failure, hasFailure := service.createPlatformMessagePost(ctx, channelID, rootID, input.Message, input.Pin, request.IdempotencyKey)
	if hasFailure {
		return mattermostToolErrorResponse(request.ToolName, failure), nil
	}
	return mattermostToolSuccessResponse(request.ToolName, "sent", result), nil
}

func (service Service) invokePlatformMessageDirectSend(ctx context.Context, request capabilities.ToolInvokeRequest, input platformMessageSendInput) (capabilities.ToolInvokeResponse, error) {
	recipient, failure, hasFailure := service.resolvePlatformDMRecipient(ctx, input.DeliveryTarget.PersonHint)
	if hasFailure {
		return platformDMErrorResponse(request.ToolName, failure), nil
	}
	dispatchID, failure, hasFailure := service.sendMattermostDirectMessageWithDispatch(ctx, recipient.MattermostUserID, input.Message, request.IdempotencyKey)
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
	return mattermostToolSuccessResponse(request.ToolName, "sent", result), nil
}

type platformMessageBroadcastResult struct {
	PersonHint         string `json:"personHint"`
	PersonID           string `json:"personID,omitempty"`
	DisplayName        string `json:"displayName,omitempty"`
	MattermostUsername string `json:"mattermostUsername,omitempty"`
	DispatchID         string `json:"dispatchID,omitempty"`
	Status             string `json:"status"`
	ErrorCode          string `json:"errorCode,omitempty"`
	Message            string `json:"message,omitempty"`
}

func (service Service) invokePlatformMessageDirectBroadcast(ctx context.Context, request capabilities.ToolInvokeRequest, input platformMessageSendInput) (capabilities.ToolInvokeResponse, error) {
	results := make([]platformMessageBroadcastResult, 0, len(input.DeliveryTarget.PersonHints))
	sentCount := 0
	for _, personHint := range input.DeliveryTarget.PersonHints {
		result := service.broadcastDirectMessageToHint(ctx, request, personHint, input.Message)
		if result.Status == "sent" {
			sentCount++
		}
		results = append(results, result)
	}
	rollup := map[string]any{
		"platform":    "mattermost",
		"results":     results,
		"sentCount":   sentCount,
		"failedCount": len(results) - sentCount,
	}
	if sentCount == 0 {
		response := platformDMErrorResponse(request.ToolName, platformDMStaticFailure("broadcast_all_failed", "message_send", "every recipient delivery failed; see results"))
		response.Result, _ = json.Marshal(rollup)
		return response, nil
	}
	return mattermostToolSuccessResponse(request.ToolName, "sent", rollup), nil
}

func (service Service) broadcastDirectMessageToHint(ctx context.Context, request capabilities.ToolInvokeRequest, personHint string, message string) platformMessageBroadcastResult {
	recipient, failure, hasFailure := service.resolvePlatformDMRecipient(ctx, personHint)
	if hasFailure {
		return platformMessageBroadcastResult{PersonHint: personHint, Status: "failed", ErrorCode: failure.ErrorCode, Message: failure.Message}
	}
	idempotencyKey := platformMessageBroadcastIdempotencyKey(request.IdempotencyKey, recipient.MattermostUserID)
	dispatchID, failure, hasFailure := service.sendMattermostDirectMessageWithDispatch(ctx, recipient.MattermostUserID, message, idempotencyKey)
	if hasFailure {
		return platformMessageBroadcastResult{PersonHint: personHint, PersonID: recipient.PersonID, DisplayName: recipient.DisplayName, Status: "failed", ErrorCode: failure.ErrorCode, Message: failure.Message}
	}
	return platformMessageBroadcastResult{
		PersonHint:         personHint,
		PersonID:           recipient.PersonID,
		DisplayName:        recipient.DisplayName,
		MattermostUsername: recipient.MattermostUsername,
		DispatchID:         dispatchID,
		Status:             "sent",
	}
}

func platformMessageBroadcastIdempotencyKey(baseKey string, recipientUserID string) string {
	trimmedBaseKey := strings.TrimSpace(baseKey)
	if trimmedBaseKey == "" {
		return ""
	}
	return trimmedBaseKey + ":" + strings.TrimSpace(recipientUserID)
}

func (service Service) invokePlatformMessageUpdate(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodePlatformMessageUpdateInput(request.Input)
	if errorValue != nil {
		return mattermostToolErrorResponse(request.ToolName, mattermostToolStaticFailure("invalid_input", "input_decode", errorValue.Error())), nil
	}
	mattermostInput := mattermostPostUpdateInput{
		PostID:   input.MessageID,
		Message:  input.Message,
		IsPinned: input.IsPinned,
	}
	return service.invokeMattermostPostUpdate(ctx, capabilities.ToolInvokeRequest{
		ToolName: request.ToolName,
		Input:    mustMarshalPlatformMessageInput(mattermostInput),
		Context:  request.Context,
	})
}

func (service Service) invokePlatformMessageDelete(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodePlatformMessageDeleteInput(request.Input)
	if errorValue != nil {
		return mattermostToolErrorResponse(request.ToolName, mattermostToolStaticFailure("invalid_input", "input_decode", errorValue.Error())), nil
	}
	if len(input.MessageIDs) == 0 {
		failure := mattermostToolStaticFailure("invalid_input", "input_decode", "messageIDs is required; use message.search first to find message IDs")
		return mattermostToolErrorResponse(request.ToolName, failure), nil
	}
	if len(input.MessageIDs) > mattermostPostSearchPageLimit {
		failure := mattermostToolStaticFailure("too_many_message_ids", "input_decode", "message.delete accepts at most 25 messageIDs per call; delete one search page at a time")
		return mattermostToolErrorResponse(request.ToolName, failure), nil
	}
	mattermostInput := mattermostPostDeleteInput{PostIDs: input.MessageIDs}
	return service.invokeMattermostPostDelete(ctx, capabilities.ToolInvokeRequest{
		ToolName: request.ToolName,
		Input:    mustMarshalPlatformMessageInput(mattermostInput),
		Context:  request.Context,
	})
}

func decodePlatformMessageSearchInput(document json.RawMessage) (platformMessageSearchInput, error) {
	var input platformMessageSearchInput
	if len(bytes.TrimSpace(document)) > 0 {
		if errorValue := json.Unmarshal(document, &input); errorValue != nil {
			return platformMessageSearchInput{}, errorValue
		}
	}
	input.Scope = strings.TrimSpace(input.Scope)
	input.AuthoredBy = strings.TrimSpace(input.AuthoredBy)
	input.Queries = normalizePlatformMessageSearchQueries(input.Queries)
	input.Cursor = strings.TrimSpace(input.Cursor)
	input.DeliveryTarget = normalizePlatformMessageDeliveryTarget(platformMessageDeliveryTarget{
		ChannelID:   input.ChannelID,
		ChannelName: input.ChannelName,
		PersonHint:  input.PersonHint,
	})
	if !isValidPlatformMessageScope(input.Scope) {
		return platformMessageSearchInput{}, fmt.Errorf("scope must be currentThread, currentChannel, directMessage, or channel")
	}
	if !isValidPlatformMessageAuthor(input.AuthoredBy) {
		return platformMessageSearchInput{}, fmt.Errorf("authoredBy must be assistant, requester, or anyone")
	}
	return input, nil
}

func normalizePlatformMessageSearchQueries(values []string) []string {
	queries := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		query := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
		if query == "" {
			continue
		}
		key := strings.ToLower(query)
		if seen[key] {
			continue
		}
		seen[key] = true
		queries = append(queries, query)
	}
	return queries
}

func decodePlatformMessageSendInput(document json.RawMessage) (platformMessageSendInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return platformMessageSendInput{}, fmt.Errorf("message.send input is required")
	}
	var input platformMessageSendInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return platformMessageSendInput{}, errorValue
	}
	input.Message = strings.TrimSpace(input.Message)
	input.Reason = strings.TrimSpace(input.Reason)
	input.DeliveryTarget = normalizePlatformMessageDeliveryTarget(platformMessageDeliveryTarget{
		Type:        input.TargetType,
		PersonHint:  input.PersonHint,
		PersonHints: input.PersonHints,
		ChannelID:   input.ChannelID,
		ChannelName: input.ChannelName,
	})
	if input.Message == "" {
		return platformMessageSendInput{}, fmt.Errorf("message is required")
	}
	if errorValue := validatePlatformMessageDeliveryTarget(input.DeliveryTarget); errorValue != nil {
		return platformMessageSendInput{}, errorValue
	}
	return input, nil
}

func decodePlatformMessageUpdateInput(document json.RawMessage) (platformMessageUpdateInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return platformMessageUpdateInput{}, fmt.Errorf("message.update input is required")
	}
	var input platformMessageUpdateInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return platformMessageUpdateInput{}, errorValue
	}
	input.MessageID = strings.TrimSpace(input.MessageID)
	if input.MessageID == "" {
		return platformMessageUpdateInput{}, fmt.Errorf("messageID is required")
	}
	if input.Message == nil && input.IsPinned == nil {
		return platformMessageUpdateInput{}, fmt.Errorf("message or isPinned is required")
	}
	if input.Message != nil && strings.TrimSpace(*input.Message) == "" {
		return platformMessageUpdateInput{}, fmt.Errorf("message cannot be empty")
	}
	return input, nil
}

func decodePlatformMessageDeleteInput(document json.RawMessage) (platformMessageDeleteInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return platformMessageDeleteInput{}, fmt.Errorf("message.delete input is required")
	}
	if errorValue := rejectUnexpectedPlatformMessageDeleteFields(document); errorValue != nil {
		return platformMessageDeleteInput{}, errorValue
	}
	var input platformMessageDeleteInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return platformMessageDeleteInput{}, errorValue
	}
	input.MessageIDs = uniqueTrimmedMattermostPostIDs(input.MessageIDs)
	if len(input.MessageIDs) == 0 {
		return platformMessageDeleteInput{}, fmt.Errorf("messageIDs is required")
	}
	return input, nil
}

func rejectUnexpectedPlatformMessageDeleteFields(document json.RawMessage) error {
	var input map[string]json.RawMessage
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return errorValue
	}
	for fieldName := range input {
		if fieldName != "messageIDs" {
			return fmt.Errorf("message.delete accepts only messageIDs; use message.search for criteria")
		}
	}
	return nil
}

func deletableMattermostCandidateIDs(candidates []mattermostPostSearchCandidate) []string {
	postIDs := []string{}
	for _, candidate := range candidates {
		if candidate.Deletable {
			postIDs = append(postIDs, candidate.PostID)
		}
	}
	return postIDs
}

func normalizePlatformMessageDeliveryTarget(target platformMessageDeliveryTarget) platformMessageDeliveryTarget {
	target.Type = strings.TrimSpace(target.Type)
	target.PersonHint = strings.TrimSpace(target.PersonHint)
	target.PersonHints = uniqueTrimmedPlatformMessageHints(target.PersonHints)
	target.ChannelID = strings.TrimSpace(target.ChannelID)
	target.ChannelName = strings.TrimSpace(target.ChannelName)
	return target
}

func uniqueTrimmedPlatformMessageHints(hints []string) []string {
	uniqueHints := []string{}
	seenHint := map[string]bool{}
	for _, hint := range hints {
		trimmedHint := strings.TrimSpace(hint)
		key := strings.ToLower(trimmedHint)
		if trimmedHint == "" || seenHint[key] {
			continue
		}
		seenHint[key] = true
		uniqueHints = append(uniqueHints, trimmedHint)
	}
	return uniqueHints
}

func validatePlatformMessageDeliveryTarget(target platformMessageDeliveryTarget) error {
	switch target.Type {
	case "directMessage":
		if target.PersonHint == "" && len(target.PersonHints) == 0 {
			return fmt.Errorf("targetType=directMessage requires personHint or personHints")
		}
		if len(target.PersonHints) > platformMessageBroadcastRecipientLimit {
			return fmt.Errorf("personHints accepts at most %d recipients per call; narrow the list", platformMessageBroadcastRecipientLimit)
		}
	case "channel":
		if target.ChannelID == "" && target.ChannelName == "" {
			return fmt.Errorf("targetType=channel requires channelName or channelID")
		}
	case "currentThread", "currentChannel":
		return nil
	default:
		return fmt.Errorf("targetType must be directMessage, currentThread, currentChannel, or channel")
	}
	return nil
}

func isValidPlatformMessageScope(scope string) bool {
	switch strings.TrimSpace(scope) {
	case "", "currentThread", "currentChannel", "directMessage", "channel":
		return true
	default:
		return false
	}
}

func isValidPlatformMessageAuthor(author string) bool {
	switch strings.TrimSpace(author) {
	case "", "assistant", "requester", "anyone":
		return true
	default:
		return false
	}
}

func platformMessageSearchScope(input platformMessageSearchInput) string {
	return input.Scope
}

func platformMessageSearchAuthor(author string) string {
	switch strings.TrimSpace(author) {
	case "assistant":
		return "internkim"
	case "requester":
		return "requester"
	default:
		return strings.TrimSpace(author)
	}
}

func (service Service) resolvePlatformMessageSendTarget(ctx context.Context, toolContext capabilities.ToolInvokeContext, target platformMessageDeliveryTarget) (string, string, mattermostToolFailure, bool) {
	switch target.Type {
	case "currentThread":
		handle, failure, hasFailure := requiredMattermostHandleFromContext(toolContext)
		if hasFailure {
			return "", "", failure, true
		}
		channelID := firstNonEmpty(handle.ChannelID, toolContext.ChannelID)
		if strings.TrimSpace(channelID) == "" || strings.TrimSpace(handle.RootID) == "" {
			failure := mattermostToolStaticFailure("thread_not_available", "context", "current platform thread is not available")
			return "", "", failure, true
		}
		return channelID, handle.RootID, mattermostToolFailure{}, false
	case "currentChannel":
		channelID := strings.TrimSpace(toolContext.ChannelID)
		if channelID == "" {
			failure := mattermostToolStaticFailure("channel_not_available", "context", "current platform channel is not available")
			return "", "", failure, true
		}
		return channelID, "", mattermostToolFailure{}, false
	case "channel":
		channel, failure, hasFailure := service.resolveMattermostToolChannel(ctx, target.ChannelID, target.ChannelName)
		if hasFailure {
			return "", "", failure, true
		}
		if !service.requesterMayAccessChannel(ctx, toolContext, channel.ID) {
			return "", "", channelAccessDeniedFailure(firstNonEmpty(channel.DisplayName, channel.Name, channel.ID)), true
		}
		return channel.ID, "", mattermostToolFailure{}, false
	default:
		failure := mattermostToolStaticFailure("invalid_target", "input_decode", "delivery target cannot be used for channel posting")
		return "", "", failure, true
	}
}

func (service Service) createPlatformMessagePost(ctx context.Context, channelID string, rootID string, message string, pin bool, idempotencyKey string) (map[string]any, mattermostToolFailure, bool) {
	var postResponse struct {
		ID string `json:"id"`
	}
	body := map[string]any{
		"channel_id": strings.TrimSpace(channelID),
		"message":    strings.TrimSpace(message),
		"props":      map[string]any{"internkim_platform_message_post": true},
	}
	if strings.TrimSpace(rootID) != "" {
		body["root_id"] = strings.TrimSpace(rootID)
	}
	if strings.TrimSpace(idempotencyKey) != "" {
		if botUser, errorValue := service.resolveMattermostBotUser(ctx); errorValue == nil {
			if pendingPostID := mattermostPendingPostID(botUser.ID, idempotencyKey); pendingPostID != "" {
				body["pending_post_id"] = pendingPostID
			}
		}
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/posts", body, &postResponse); errorValue != nil {
		return nil, mattermostToolFailureForError("post_create", "mattermost_unavailable", errorValue), true
	}
	if strings.TrimSpace(postResponse.ID) == "" {
		return nil, mattermostToolStaticFailure("post_create_failed", "post_create", "mattermost did not return a post ID"), true
	}
	if pin {
		if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/posts/"+url.PathEscape(postResponse.ID)+"/pin", nil, nil); errorValue != nil {
			return nil, mattermostToolFailureForError("post_pin", "mattermost_unavailable", errorValue), true
		}
	}
	result := map[string]any{"dispatchID": postResponse.ID, "channelID": channelID, "rootPostID": rootID, "isPinned": pin}
	return result, mattermostToolFailure{}, false
}

func mustMarshalPlatformMessageInput(value any) json.RawMessage {
	document, _ := json.Marshal(value)
	return document
}
