package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

type chatdMessagePostAttachment struct {
	DevicePath  string `json:"devicePath"`
	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	SizeBytes   int64  `json:"sizeBytes,omitempty"`
}

type chatdMessagePostRequest struct {
	ThreadID    string                       `json:"threadID,omitempty"`
	ChannelID   string                       `json:"channelID,omitempty"`
	ChannelName string                       `json:"channelName,omitempty"`
	Message     string                       `json:"message"`
	Attachments []chatdMessagePostAttachment `json:"attachments,omitempty"`
}

type chatdMessagePostResponse struct {
	MessageID string `json:"messageID"`
	ChannelID string `json:"channelID,omitempty"`
}

// A company runs one messenger and this device is configured with its name. A
// request says which conversation it arrived from, and a request that carries
// no conversation says nothing at all; neither is a choice of where a message
// is delivered, so neither is asked.
func (service Service) companyMessenger() string {
	return strings.TrimSpace(service.Configuration.ChatdPlatform)
}

func (service Service) chatdServesTheMessenger() bool {
	return service.companyMessenger() != "" && strings.TrimSpace(service.Configuration.ChatdEndpoint) != ""
}

func (service Service) invokeChatdPlatformMessageSend(ctx context.Context, request capabilities.ToolInvokeRequest, input platformMessageSendInput) (capabilities.ToolInvokeResponse, error) {
	if input.DeliveryTarget.Type == "directMessage" {
		if strings.TrimSpace(input.DeliveryTarget.PersonHint) != "" || len(input.DeliveryTarget.PersonHints) > 0 {
			return service.invokeChatdDirectMessageSend(ctx, request, input)
		}
		failure := platformToolStaticFailure("unsupported_target", "platform_route",
			"targetType=directMessage without personHint answers the requester, and on "+service.companyMessenger()+" that is this conversation; reply with targetType=currentChannel or currentThread")
		return platformToolErrorResponse(request.ToolName, failure), nil
	}
	if input.Pin {
		failure := platformToolStaticFailure("invalid_input", "platform_route",
			"pin is not supported on "+service.companyMessenger()+"; send without pin")
		return platformToolErrorResponse(request.ToolName, failure), nil
	}
	attachmentFiles, failure, hasFailure := service.resolvePlatformMessageAttachments(request, input.Attachments)
	if hasFailure {
		return platformToolErrorResponse(request.ToolName, failure), nil
	}
	postRequest, failure, hasFailure := chatdMessagePostTarget(request.Context, input.DeliveryTarget)
	if hasFailure {
		return platformToolErrorResponse(request.ToolName, failure), nil
	}
	postRequest.Message = strings.TrimSpace(input.Message)
	postRequest.Attachments = chatdMessagePostAttachments(attachmentFiles)
	var response chatdMessagePostResponse
	if errorValue := service.chatdRequest(ctx, "message.post", postRequest, &response); errorValue != nil {
		return platformToolErrorResponse(request.ToolName, platformToolFailureForError("post_create", "platform_unavailable", errorValue)), nil
	}
	if strings.TrimSpace(response.MessageID) == "" {
		return platformToolErrorResponse(request.ToolName, platformToolStaticFailure("post_create_failed", "post_create", "chatd did not return a message ID")), nil
	}
	return platformToolSuccessResponse(request.ToolName, "sent", platformMessageSendResult{
		MessageIDs:     []string{response.MessageID},
		DeliveryStatus: "sent",
	})
}

func chatdMessagePostTarget(toolContext capabilities.ToolInvokeContext, target platformMessageDeliveryTarget) (chatdMessagePostRequest, platformToolFailure, bool) {
	switch target.Type {
	case "currentThread":
		replyTargetID := strings.TrimSpace(toolContext.ReplyTargetID)
		if replyTargetID == "" {
			return chatdMessagePostRequest{}, platformToolStaticFailure("thread_not_available", "context", "current platform thread is not available"), true
		}
		return chatdMessagePostRequest{ThreadID: replyTargetID}, platformToolFailure{}, false
	case "currentChannel":
		channelID := strings.TrimSpace(toolContext.ChannelID)
		if channelID == "" {
			return chatdMessagePostRequest{}, platformToolStaticFailure("channel_not_available", "context", "current platform channel is not available"), true
		}
		return chatdMessagePostRequest{ChannelID: channelID}, platformToolFailure{}, false
	case "channel":
		channelID := strings.TrimSpace(target.ChannelID)
		// The id a model passes is whatever id it last saw, and in a direct
		// conversation that is the conversation's own. A channel post aimed at
		// the room the request came from is not a channel post; asking for the
		// channel's name is what keeps the copy out of the DM.
		if channelID != "" && channelID == strings.TrimSpace(toolContext.ChannelID) &&
			strings.EqualFold(strings.TrimSpace(toolContext.ConversationType), "direct") {
			return chatdMessagePostRequest{}, platformToolStaticFailure("invalid_target", "input_decode",
				"channelID "+channelID+" is this direct conversation, not a channel; name the channel with channelName, or use targetType=currentChannel to reply here"), true
		}
		return chatdMessagePostRequest{
			ChannelID:   channelID,
			ChannelName: strings.TrimSpace(target.ChannelName),
		}, platformToolFailure{}, false
	default:
		return chatdMessagePostRequest{}, platformToolStaticFailure("invalid_target", "input_decode", "delivery target cannot be used for channel posting"), true
	}
}

func chatdMessagePostAttachments(files []platformFile) []chatdMessagePostAttachment {
	attachments := make([]chatdMessagePostAttachment, 0, len(files))
	for _, file := range files {
		attachments = append(attachments, chatdMessagePostAttachment{
			DevicePath:  file.DevicePath,
			Filename:    file.Filename,
			ContentType: file.ContentType,
			SizeBytes:   file.SizeBytes,
		})
	}
	return attachments
}

func (service Service) chatdProviderHealth(ctx context.Context) providerAvailability {
	if !service.chatdServesTheMessenger() {
		return providerAvailability{Reason: "chatd endpoint is not configured"}
	}
	if errorValue := service.pingChatd(ctx); errorValue != nil {
		return providerAvailability{Configured: true, Reason: errorValue.Error()}
	}
	return providerAvailability{Configured: true, Available: true}
}

func (service Service) pingChatd(ctx context.Context) error {
	endpoint := strings.TrimRight(strings.TrimSpace(service.Configuration.ChatdEndpoint), "/")
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+blueclaw.ChatdHealthPath, nil)
	if errorValue != nil {
		return errorValue
	}
	response, errorValue := service.httpClient().Do(httpRequest)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("chatd health answered %d", response.StatusCode)
	}
	return nil
}

func (service Service) chatdRequest(ctx context.Context, capabilityName string, requestBody any, responseValue any) error {
	endpoint := strings.TrimRight(strings.TrimSpace(service.Configuration.ChatdEndpoint), "/")
	if endpoint == "" {
		return errors.New("chatd endpoint is not configured")
	}
	document, errorValue := json.Marshal(requestBody)
	if errorValue != nil {
		return errorValue
	}
	requestURL := endpoint + "/v1/platform/" + url.PathEscape(service.companyMessenger()) + "/" + capabilityName
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(document))
	if errorValue != nil {
		return errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	client := service.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	response, errorValue := client.Do(httpRequest)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	body, errorValue := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if errorValue != nil {
		return errorValue
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("chatd %s answered %d: %s", capabilityName, response.StatusCode, strings.TrimSpace(string(body)))
	}
	if responseValue == nil {
		return nil
	}
	return json.Unmarshal(body, responseValue)
}

// chatd deletes a message as the person who wrote it, which it has been able to
// do since the messenger screen learned to. Only the route from here was
// missing, so the agent was told deletion did not exist on this platform.
func (service Service) invokeChatdPlatformMessageDelete(ctx context.Context, request capabilities.ToolInvokeRequest, messageIDs []string) (capabilities.ToolInvokeResponse, error) {
	replyTargetID := strings.TrimSpace(request.Context.ReplyTargetID)
	if replyTargetID == "" {
		replyTargetID = strings.TrimSpace(request.Context.ConversationID)
	}
	if replyTargetID == "" {
		failure := platformToolStaticFailure("invalid_input", "platform_route",
			"a message is deleted in the conversation it belongs to, and this call names none")
		return platformToolErrorResponse(request.ToolName, failure), nil
	}
	deleted := []string{}
	for _, messageID := range messageIDs {
		trimmedMessageID := strings.TrimSpace(messageID)
		if trimmedMessageID == "" {
			continue
		}
		var response map[string]any
		requestBody := map[string]any{
			"replyTargetID":      replyTargetID,
			"messageID":          trimmedMessageID,
			"requesterPubkeyHex": strings.TrimSpace(request.Context.RequesterPlatformUserID),
		}
		if errorValue := service.chatdRequest(ctx, "message_delete", requestBody, &response); errorValue != nil {
			failure := platformToolStaticFailure("message_delete_failed", "platform_delete", trimmedMessageID+": "+errorValue.Error())
			return platformToolErrorResponse(request.ToolName, failure), nil
		}
		deleted = append(deleted, trimmedMessageID)
	}
	return platformToolSuccessResponse(request.ToolName, "deleted", platformMessageDeleteResult{
		MessageIDs:     deleted,
		DeliveryStatus: "deleted",
	})
}

// An edit names a quoted span of the message it changes. The current text comes
// from the platform's own record of that exact message, the span is applied to
// it, and the whole result is sent, because that is the only edit the platform
// itself has.
func (service Service) invokeChatdPlatformMessageUpdate(ctx context.Context, request capabilities.ToolInvokeRequest, input platformMessageUpdateInput) (capabilities.ToolInvokeResponse, error) {
	messageID := strings.TrimSpace(input.MessageID)
	if input.OldText == nil && len(input.Attachments) == 0 {
		return platformToolErrorResponse(request.ToolName, platformToolStaticFailure("invalid_input", "input_decode",
			"pass oldText with the exact current span and newText with its replacement, or attachments to add files without changing the text")), nil
	}
	attachmentFiles, failure, hasFailure := service.resolvePlatformMessageAttachments(request, input.Attachments)
	if hasFailure {
		return platformToolErrorResponse(request.ToolName, failure), nil
	}
	replyTargetID := strings.TrimSpace(request.Context.ReplyTargetID)
	if replyTargetID == "" {
		replyTargetID = strings.TrimSpace(request.Context.ConversationID)
	}
	currentText, failure, hasFailure := service.chatdCurrentMessageText(ctx, messageID)
	if hasFailure {
		return platformToolErrorResponse(request.ToolName, failure), nil
	}
	editedText := currentText
	if input.OldText != nil {
		matchCount := strings.Count(currentText, *input.OldText)
		if matchCount != 1 {
			return platformToolErrorResponse(request.ToolName, platformMessageEditMatchFailure(matchCount, currentText)), nil
		}
		editedText = strings.Replace(currentText, *input.OldText, *input.NewText, 1)
	}
	requestBody := map[string]any{
		"replyTargetID":      replyTargetID,
		"messageID":          messageID,
		"message":            editedText,
		"requesterPubkeyHex": strings.TrimSpace(request.Context.RequesterPlatformUserID),
	}
	if len(attachmentFiles) > 0 {
		requestBody["attachments"] = chatdMessagePostAttachments(attachmentFiles)
	}
	var response map[string]any
	if errorValue := service.chatdRequest(ctx, "message.edit", requestBody, &response); errorValue != nil {
		return platformToolErrorResponse(request.ToolName, platformToolFailureForError("message_update", "platform_unavailable", errorValue)), nil
	}
	return platformToolSuccessResponse(request.ToolName, "updated", map[string]any{
		"messageID":      messageID,
		"deliveryStatus": "updated",
		"messageUpdated": true,
	})
}

// The record an edit reads must be the record message_search answered from: an
// ID search crosses channels and applies later edits, so a post found in one
// channel stays editable from the conversation the request came from.
func (service Service) chatdCurrentMessageText(ctx context.Context, messageID string) (string, platformToolFailure, bool) {
	searchRequest := chatdMessageSearchRequest{MessageIDs: []string{messageID}, Queries: []string{}}
	var response chatdMessageSearchResponse
	if errorValue := service.chatdRequest(ctx, "message.search", searchRequest, &response); errorValue != nil {
		return "", platformToolFailureForError("message_lookup", "platform_unavailable", errorValue), true
	}
	for _, candidate := range response.Candidates {
		if strings.TrimSpace(candidate.MessageID) == messageID {
			return candidate.Text, platformToolFailure{}, false
		}
	}
	return "", platformToolStaticFailure("not_found", "message_lookup",
		"message "+messageID+" does not exist on this platform or was deleted"), true
}
