package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

const platformMessageBroadcastRecipientLimit = 50
const platformMessageAttachmentLimit = 5

type platformMessageDeliveryTarget struct {
	Type        string   `json:"type"`
	PersonHint  string   `json:"personHint"`
	PersonHints []string `json:"personHints"`
	ChannelID   string   `json:"channelID"`
	ChannelName string   `json:"channelName"`
}

type platformMessageSearchInput struct {
	Scope       string   `json:"scope"`
	MessageIDs  []string `json:"messageIDs"`
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
	Attachments []string `json:"attachments"`
	Reason      string   `json:"reason"`

	DeliveryTarget platformMessageDeliveryTarget `json:"-"`
}

type platformMessageUpdateInput struct {
	MessageID   string   `json:"messageID"`
	OldText     *string  `json:"oldText"`
	NewText     *string  `json:"newText"`
	IsPinned    *bool    `json:"isPinned"`
	Attachments []string `json:"attachments"`
}

type platformMessageDeleteInput struct {
	MessageIDs []string `json:"messageIDs"`
}

type platformMessageContextResult struct {
	Platform                string `json:"platform"`
	ConversationID          string `json:"conversationID"`
	ConversationType        string `json:"conversationType"`
	ChannelID               string `json:"channelID"`
	ChannelName             string `json:"channelName"`
	ReplyTargetID           string `json:"replyTargetID"`
	RootMessageID           string `json:"rootMessageID"`
	CurrentMessageID        string `json:"currentMessageID"`
	RequesterPersonID       string `json:"requesterPersonID"`
	RequesterPlatformUserID string `json:"requesterPlatformUserID"`
	BotUserID               string `json:"botUserID"`
	BotUsername             string `json:"botUsername"`
}

type platformMessageSearchCandidateResult struct {
	MessageID       string `json:"messageID"`
	Text            string `json:"text,omitempty"`
	ChannelID       string `json:"channelID"`
	RootMessageID   string `json:"rootMessageID,omitempty"`
	UserID          string `json:"userID"`
	AuthoredBy      string `json:"authoredBy"`
	CreatedAt       int64  `json:"createdAt"`
	Preview         string `json:"preview"`
	Editable        bool   `json:"editable,omitempty"`
	Deletable       bool   `json:"deletable"`
	ProtectedReason string `json:"protectedReason,omitempty"`
}

type platformMessageSearchResult struct {
	Scope      string                                 `json:"scope"`
	Queries    []string                               `json:"queries"`
	AuthoredBy string                                 `json:"authoredBy"`
	MessageIDs []string                               `json:"messageIDs"`
	Candidates []platformMessageSearchCandidateResult `json:"candidates"`
	NextCursor string                                 `json:"nextCursor,omitempty"`
	HasMore    bool                                   `json:"hasMore"`
}

type platformMessageFailureResult struct {
	PersonHint string `json:"personHint,omitempty"`
	MessageID  string `json:"messageID,omitempty"`
	ErrorCode  string `json:"errorCode"`
	Message    string `json:"message"`
}

type platformMessageSendResult struct {
	MessageIDs     []string                       `json:"messageIDs"`
	DeliveryStatus string                         `json:"deliveryStatus"`
	Failures       []platformMessageFailureResult `json:"failures,omitempty"`
}

type platformMessageDeleteResult struct {
	MessageIDs     []string                       `json:"messageIDs"`
	DeliveryStatus string                         `json:"deliveryStatus"`
	Failures       []platformMessageFailureResult `json:"failures,omitempty"`
}

func (service Service) invokePlatformMessageTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	switch request.ToolName {
	case "message_context":
		if response, isDenied := service.authorizePlatformMessageTool(ctx, request); isDenied {
			return response, nil
		}
		return service.invokeChatdPlatformMessageContext(ctx, request)
	case "message_search":
		if response, isDenied := service.authorizePlatformMessageTool(ctx, request); isDenied {
			return response, nil
		}
		return service.invokePlatformMessageSearch(ctx, request)
	case "message_send":
		return service.invokePlatformMessageSend(ctx, request)
	case "message_update":
		if response, isDenied := service.authorizePlatformMessageTool(ctx, request); isDenied {
			return response, nil
		}
		return service.invokePlatformMessageUpdate(ctx, request)
	case "message_delete":
		if response, isDenied := service.authorizePlatformMessageTool(ctx, request); isDenied {
			return response, nil
		}
		return service.invokePlatformMessageDelete(ctx, request)
	default:
		return capabilities.ToolInvokeResponse{}, fmt.Errorf("platform message tool is not configured: %s", request.ToolName)
	}
}

func (service Service) authorizePlatformMessageTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, bool) {
	if !service.requesterIsACompanyMember(ctx, request.Context) {
		message := request.ToolName + " requires member access"
		return platformToolDeniedResponse(request.ToolName, platformToolStaticFailure(capabilities.CapabilityNotAllowed, "authorization", message)), true
	}
	return capabilities.ToolInvokeResponse{}, false
}

func (service Service) invokePlatformMessageSearch(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodePlatformMessageSearchInput(request.Input)
	if errorValue != nil {
		return platformToolErrorResponse(request.ToolName, platformToolStaticFailure("invalid_input", "input_decode", errorValue.Error())), nil
	}
	return service.invokeChatdPlatformMessageSearch(ctx, request, input)
}

func (service Service) invokePlatformMessageSend(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodePlatformMessageSendInput(request.Input)
	if errorValue != nil {
		return platformToolErrorResponse(request.ToolName, platformToolStaticFailure("invalid_input", "input_decode", errorValue.Error())), nil
	}
	return service.invokeChatdPlatformMessageSend(ctx, request, input)
}

// The files were read by the person sending them, where their identity exists,
// and arrive as content. A message that names a file nobody carried is refused
// rather than served by opening the workspace as root.
func (service Service) resolvePlatformMessageAttachments(request capabilities.ToolInvokeRequest, paths []string) ([]platformFile, platformToolFailure, bool) {
	if len(paths) == 0 {
		return nil, platformToolFailure{}, false
	}
	carriedFiles := request.Transport.WorkspaceFiles
	if len(carriedFiles) != len(paths) {
		return nil, platformToolStaticFailure("attachment_not_carried", "attachment_resolve",
			"these files were named but not carried; the caller reads them as the person who asked and sends their content"), true
	}
	attachmentFiles := []platformFile{}
	for _, carried := range carriedFiles {
		attachmentFile, errorValue := service.materializeInlinePlatformFile(service.Configuration.WithDefaults(), platformFileSpec{
			DevicePath:    carried.WorkspacePath,
			Filename:      carried.Filename,
			ContentBase64: carried.ContentBase64,
		})
		if errorValue != nil {
			return nil, platformToolStaticFailure("attachment_unavailable", "attachment_resolve", carried.WorkspacePath+": "+errorValue.Error()), true
		}
		attachmentFiles = append(attachmentFiles, attachmentFile)
	}
	return attachmentFiles, platformToolFailure{}, false
}

type platformMessageBroadcastResult struct {
	PersonHint  string `json:"personHint"`
	PersonID    string `json:"personID,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	DispatchID  string `json:"dispatchID,omitempty"`
	Status      string `json:"status"`
	ErrorCode   string `json:"errorCode,omitempty"`
	Message     string `json:"message,omitempty"`
}

func canonicalPlatformMessageBroadcastResult(results []platformMessageBroadcastResult) ([]string, []platformMessageFailureResult) {
	messageIDs := []string{}
	seenMessageIDs := map[string]bool{}
	failures := []platformMessageFailureResult{}
	for _, result := range results {
		messageID := strings.TrimSpace(result.DispatchID)
		if result.Status == "sent" && messageID != "" {
			if !seenMessageIDs[messageID] {
				messageIDs = append(messageIDs, messageID)
				seenMessageIDs[messageID] = true
			}
			continue
		}
		failures = append(failures, platformMessageFailureResult{
			PersonHint: result.PersonHint,
			ErrorCode:  result.ErrorCode,
			Message:    result.Message,
		})
	}
	return messageIDs, failures
}

func (service Service) invokePlatformMessageUpdate(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodePlatformMessageUpdateInput(request.Input)
	if errorValue != nil {
		return platformToolErrorResponse(request.ToolName, platformToolStaticFailure("invalid_input", "input_decode", errorValue.Error())), nil
	}
	return service.invokeChatdPlatformMessageUpdate(ctx, request, input)
}

func (service Service) invokePlatformMessageDelete(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodePlatformMessageDeleteInput(request.Input)
	if errorValue != nil {
		return platformToolErrorResponse(request.ToolName, platformToolStaticFailure("invalid_input", "input_decode", errorValue.Error())), nil
	}
	if len(input.MessageIDs) == 0 {
		failure := platformToolStaticFailure("invalid_input", "input_decode", "messageIDs is required; use message_search first to find message IDs")
		return platformToolErrorResponse(request.ToolName, failure), nil
	}
	return service.invokeChatdPlatformMessageDelete(ctx, request, input.MessageIDs)
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
	input.MessageIDs = uniqueTrimmedPlatformMessageIDs(input.MessageIDs)
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
		return platformMessageSendInput{}, fmt.Errorf("message_send input is required")
	}
	var input platformMessageSendInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return platformMessageSendInput{}, errorValue
	}
	input.Message = strings.TrimSpace(input.Message)
	input.Reason = strings.TrimSpace(input.Reason)
	input.Attachments = uniqueTrimmedPlatformMessageHints(input.Attachments)
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
	if len(input.Attachments) > platformMessageAttachmentLimit {
		return platformMessageSendInput{}, fmt.Errorf("attachments accepts at most %d workspace paths per call", platformMessageAttachmentLimit)
	}
	if errorValue := validatePlatformMessageDeliveryTarget(input.DeliveryTarget); errorValue != nil {
		return platformMessageSendInput{}, errorValue
	}
	return input, nil
}

func decodePlatformMessageUpdateInput(document json.RawMessage) (platformMessageUpdateInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return platformMessageUpdateInput{}, fmt.Errorf("message_update input is required")
	}
	var input platformMessageUpdateInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return platformMessageUpdateInput{}, errorValue
	}
	input.MessageID = strings.TrimSpace(input.MessageID)
	input.Attachments = uniqueTrimmedPlatformMessageHints(input.Attachments)
	if input.MessageID == "" {
		return platformMessageUpdateInput{}, fmt.Errorf("messageID is required")
	}
	if (input.OldText == nil) != (input.NewText == nil) {
		return platformMessageUpdateInput{}, fmt.Errorf("oldText and newText must be given together")
	}
	if input.OldText == nil && input.IsPinned == nil && len(input.Attachments) == 0 {
		return platformMessageUpdateInput{}, fmt.Errorf("oldText with newText, isPinned, or attachments is required")
	}
	if input.OldText != nil && strings.TrimSpace(*input.OldText) == "" {
		return platformMessageUpdateInput{}, fmt.Errorf("oldText cannot be empty")
	}
	if len(input.Attachments) > platformMessageAttachmentLimit {
		return platformMessageUpdateInput{}, fmt.Errorf("attachments accepts at most %d workspace paths per call", platformMessageAttachmentLimit)
	}
	return input, nil
}

func decodePlatformMessageDeleteInput(document json.RawMessage) (platformMessageDeleteInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return platformMessageDeleteInput{}, fmt.Errorf("message_delete input is required")
	}
	if errorValue := rejectUnexpectedPlatformMessageDeleteFields(document); errorValue != nil {
		return platformMessageDeleteInput{}, errorValue
	}
	var input platformMessageDeleteInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return platformMessageDeleteInput{}, errorValue
	}
	input.MessageIDs = uniqueTrimmedPlatformMessageIDs(input.MessageIDs)
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
			return fmt.Errorf("message_delete accepts only messageIDs; use message_search for criteria")
		}
	}
	return nil
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
