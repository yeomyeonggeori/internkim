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

type platformMessageDeliveryTarget struct {
	Type        string `json:"type"`
	PersonHint  string `json:"personHint"`
	ChannelID   string `json:"channelID"`
	ChannelName string `json:"channelName"`
}

type platformMessageSearchInput struct {
	Scope          string                        `json:"scope"`
	DeliveryTarget platformMessageDeliveryTarget `json:"deliveryTarget"`
	AuthoredBy     string                        `json:"authoredBy"`
	Queries        []string                      `json:"queries"`
	Limit          int                           `json:"limit"`
	Cursor         string                        `json:"cursor"`
}

type platformMessageSendInput struct {
	DeliveryTarget platformMessageDeliveryTarget `json:"deliveryTarget"`
	Message        string                        `json:"message"`
	Pin            bool                          `json:"pin"`
	Reason         string                        `json:"reason"`
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
	case "platform.message.context":
		if response, isDenied := service.authorizePlatformMessageTool(ctx, request, false); isDenied {
			return response, nil
		}
		return service.invokeMattermostContextInspect(ctx, request)
	case "platform.message.search":
		if response, isDenied := service.authorizePlatformMessageTool(ctx, request, false); isDenied {
			return response, nil
		}
		return service.invokePlatformMessageSearch(ctx, request)
	case "platform.message.send":
		return service.invokePlatformMessageSend(ctx, request)
	case "platform.message.update":
		if response, isDenied := service.authorizePlatformMessageTool(ctx, request, true); isDenied {
			return response, nil
		}
		return service.invokePlatformMessageUpdate(ctx, request)
	case "platform.message.delete":
		if response, isDenied := service.authorizePlatformMessageTool(ctx, request, true); isDenied {
			return response, nil
		}
		return service.invokePlatformMessageDelete(ctx, request)
	default:
		return capabilities.ToolInvokeResponse{}, fmt.Errorf("platform message tool is not configured: %s", request.ToolName)
	}
}

func (service Service) authorizePlatformMessageTool(ctx context.Context, request capabilities.ToolInvokeRequest, requiresApproval bool) (capabilities.ToolInvokeResponse, bool) {
	if !service.mattermostRequesterHasCircle(ctx, request.Context, mattermostToolStaffCircle) {
		message := request.ToolName + " requires staff access"
		return mattermostToolDeniedResponse(request.ToolName, mattermostToolStaticFailure(capabilities.CapabilityNotAllowed, "authorization", message)), true
	}
	if requiresApproval && !request.Context.IsApprovalContinuation && !request.Context.IsScheduledRun {
		message := request.ToolName + " requires approval before execution"
		return mattermostToolDeniedResponse(request.ToolName, mattermostToolStaticFailure("approval_required", "authorization", message)), true
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
		return service.invokePlatformMessageDirectSend(ctx, request, input)
	}
	if response, isDenied := service.authorizePlatformMessageTool(ctx, request, true); isDenied {
		return response, nil
	}
	channelID, rootID, failure, hasFailure := service.resolvePlatformMessageSendTarget(ctx, request.Context, input.DeliveryTarget)
	if hasFailure {
		return mattermostToolErrorResponse(request.ToolName, failure), nil
	}
	result, failure, hasFailure := service.createPlatformMessagePost(ctx, channelID, rootID, input.Message, input.Pin)
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
	return mattermostToolSuccessResponse(request.ToolName, "sent", result), nil
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
		failure := mattermostToolStaticFailure("invalid_input", "input_decode", "messageIDs is required; use platform.message.search first to find message IDs")
		return mattermostToolErrorResponse(request.ToolName, failure), nil
	}
	if len(input.MessageIDs) > mattermostPostSearchPageLimit {
		failure := mattermostToolStaticFailure("too_many_message_ids", "input_decode", "platform.message.delete accepts at most 25 messageIDs per call; delete one search page at a time")
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
	input.DeliveryTarget = normalizePlatformMessageDeliveryTarget(input.DeliveryTarget)
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
		return platformMessageSendInput{}, fmt.Errorf("platform.message.send input is required")
	}
	var input platformMessageSendInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return platformMessageSendInput{}, errorValue
	}
	input.Message = strings.TrimSpace(input.Message)
	input.Reason = strings.TrimSpace(input.Reason)
	input.DeliveryTarget = normalizePlatformMessageDeliveryTarget(input.DeliveryTarget)
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
		return platformMessageUpdateInput{}, fmt.Errorf("platform.message.update input is required")
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
		return platformMessageDeleteInput{}, fmt.Errorf("platform.message.delete input is required")
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
			return fmt.Errorf("platform.message.delete accepts only messageIDs; use platform.message.search for criteria")
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
	target.ChannelID = strings.TrimSpace(target.ChannelID)
	target.ChannelName = strings.TrimSpace(target.ChannelName)
	return target
}

func validatePlatformMessageDeliveryTarget(target platformMessageDeliveryTarget) error {
	switch target.Type {
	case "directMessage":
		if target.PersonHint == "" {
			return fmt.Errorf("deliveryTarget.personHint is required for directMessage")
		}
	case "channel":
		return validateMattermostChannelReference(target.ChannelID, target.ChannelName)
	case "currentThread", "currentChannel":
		return nil
	default:
		return fmt.Errorf("deliveryTarget.type must be directMessage, currentThread, currentChannel, or channel")
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
	if input.Scope != "" {
		return input.Scope
	}
	switch input.DeliveryTarget.Type {
	case "directMessage":
		return "directMessage"
	case "channel":
		return "channel"
	case "currentThread":
		return "currentThread"
	case "currentChannel":
		return "currentChannel"
	default:
		return ""
	}
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
		return channel.ID, "", mattermostToolFailure{}, false
	default:
		failure := mattermostToolStaticFailure("invalid_target", "input_decode", "delivery target cannot be used for channel posting")
		return "", "", failure, true
	}
}

func (service Service) createPlatformMessagePost(ctx context.Context, channelID string, rootID string, message string, pin bool) (map[string]any, mattermostToolFailure, bool) {
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
