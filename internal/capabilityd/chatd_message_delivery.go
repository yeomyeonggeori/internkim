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

func (service Service) chatdServesPlatform(platform string) bool {
	configured := strings.TrimSpace(service.Configuration.ChatdPlatform)
	return configured != "" &&
		strings.TrimSpace(service.Configuration.ChatdEndpoint) != "" &&
		strings.EqualFold(strings.TrimSpace(platform), configured)
}

func (service Service) invokeChatdPlatformMessageSend(ctx context.Context, request capabilities.ToolInvokeRequest, input platformMessageSendInput) (capabilities.ToolInvokeResponse, error) {
	if input.DeliveryTarget.Type == "directMessage" {
		failure := mattermostToolStaticFailure("unsupported_target", "platform_route",
			"targetType=directMessage is not yet routed for platform "+request.Context.Platform+"; reply in the current conversation with targetType=currentChannel or currentThread")
		return mattermostToolErrorResponse(request.ToolName, failure), nil
	}
	if input.Pin {
		failure := mattermostToolStaticFailure("invalid_input", "platform_route",
			"pin is not supported on platform "+request.Context.Platform+"; send without pin")
		return mattermostToolErrorResponse(request.ToolName, failure), nil
	}
	attachmentFiles, failure, hasFailure := service.resolvePlatformMessageAttachments(request, input.Attachments)
	if hasFailure {
		return mattermostToolErrorResponse(request.ToolName, failure), nil
	}
	postRequest, failure, hasFailure := chatdMessagePostTarget(request.Context, input.DeliveryTarget)
	if hasFailure {
		return mattermostToolErrorResponse(request.ToolName, failure), nil
	}
	postRequest.Message = strings.TrimSpace(input.Message)
	postRequest.Attachments = chatdMessagePostAttachments(attachmentFiles)
	var response chatdMessagePostResponse
	if errorValue := service.chatdPlatformRequest(ctx, request.Context.Platform, "message.post", postRequest, &response); errorValue != nil {
		return mattermostToolErrorResponse(request.ToolName, mattermostToolFailureForError("post_create", "platform_unavailable", errorValue)), nil
	}
	if strings.TrimSpace(response.MessageID) == "" {
		return mattermostToolErrorResponse(request.ToolName, mattermostToolStaticFailure("post_create_failed", "post_create", "chatd did not return a message ID")), nil
	}
	return mattermostToolSuccessResponse(request.ToolName, "sent", platformMessageSendResult{
		MessageIDs:     []string{response.MessageID},
		DeliveryStatus: "sent",
	})
}

func chatdMessagePostTarget(toolContext capabilities.ToolInvokeContext, target platformMessageDeliveryTarget) (chatdMessagePostRequest, mattermostToolFailure, bool) {
	switch target.Type {
	case "currentThread":
		replyTargetID := strings.TrimSpace(toolContext.ReplyTargetID)
		if replyTargetID == "" {
			return chatdMessagePostRequest{}, mattermostToolStaticFailure("thread_not_available", "context", "current platform thread is not available"), true
		}
		return chatdMessagePostRequest{ThreadID: replyTargetID}, mattermostToolFailure{}, false
	case "currentChannel":
		channelID := strings.TrimSpace(toolContext.ChannelID)
		if channelID == "" {
			return chatdMessagePostRequest{}, mattermostToolStaticFailure("channel_not_available", "context", "current platform channel is not available"), true
		}
		return chatdMessagePostRequest{ChannelID: channelID}, mattermostToolFailure{}, false
	case "channel":
		channelID := strings.TrimSpace(target.ChannelID)
		// The id a model passes is whatever id it last saw, and in a direct
		// conversation that is the conversation's own. A channel post aimed at
		// the room the request came from is not a channel post; asking for the
		// channel's name is what keeps the copy out of the DM.
		if channelID != "" && channelID == strings.TrimSpace(toolContext.ChannelID) &&
			strings.EqualFold(strings.TrimSpace(toolContext.ConversationType), "direct") {
			return chatdMessagePostRequest{}, mattermostToolStaticFailure("invalid_target", "input_decode",
				"channelID "+channelID+" is this direct conversation, not a channel; name the channel with channelName, or use targetType=currentChannel to reply here"), true
		}
		return chatdMessagePostRequest{
			ChannelID:   channelID,
			ChannelName: strings.TrimSpace(target.ChannelName),
		}, mattermostToolFailure{}, false
	default:
		return chatdMessagePostRequest{}, mattermostToolStaticFailure("invalid_target", "input_decode", "delivery target cannot be used for channel posting"), true
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

func (service Service) chatdUnroutedToolFailure(toolName string, platform string) mattermostToolFailure {
	return mattermostToolStaticFailure("unsupported_platform", "platform_route",
		toolName+" is not yet available for platform "+platform+"; message_send can post to the current conversation or a named channel")
}

func (service Service) chatdPlatformRequest(ctx context.Context, platform string, capabilityName string, requestBody any, responseValue any) error {
	endpoint := strings.TrimRight(strings.TrimSpace(service.Configuration.ChatdEndpoint), "/")
	if endpoint == "" {
		return errors.New("chatd endpoint is not configured")
	}
	document, errorValue := json.Marshal(requestBody)
	if errorValue != nil {
		return errorValue
	}
	requestURL := endpoint + "/v1/platform/" + url.PathEscape(strings.TrimSpace(platform)) + "/" + capabilityName
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
		failure := mattermostToolStaticFailure("invalid_input", "platform_route",
			"a message is deleted in the conversation it belongs to, and this call names none")
		return mattermostToolErrorResponse(request.ToolName, failure), nil
	}
	deleted := []string{}
	for _, messageID := range messageIDs {
		trimmedMessageID := strings.TrimSpace(messageID)
		if trimmedMessageID == "" {
			continue
		}
		var response map[string]any
		requestBody := map[string]any{"replyTargetID": replyTargetID, "messageID": trimmedMessageID}
		if errorValue := service.chatdPlatformRequest(ctx, request.Context.Platform, "message_delete", requestBody, &response); errorValue != nil {
			failure := mattermostToolStaticFailure("message_delete_failed", "platform_delete", trimmedMessageID+": "+errorValue.Error())
			return mattermostToolErrorResponse(request.ToolName, failure), nil
		}
		deleted = append(deleted, trimmedMessageID)
	}
	return mattermostToolSuccessResponse(request.ToolName, "deleted", platformMessageDeleteResult{
		MessageIDs:     deleted,
		DeliveryStatus: "deleted",
	})
}

type chatdHistoryFetchRequest struct {
	ThreadID    string `json:"threadID,omitempty"`
	ChannelID   string `json:"channelID,omitempty"`
	ChannelName string `json:"channelName,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

type chatdHistoryMessage struct {
	ID           string `json:"id"`
	ThreadRootID string `json:"threadRootId,omitempty"`
	Speaker      string `json:"speaker"`
	SenderID     string `json:"senderId,omitempty"`
	Text         string `json:"text"`
	SentAt       string `json:"sentAt,omitempty"`
	IsBot        bool   `json:"isBot,omitempty"`
}

type chatdHistoryFetchResponse struct {
	Messages  []chatdHistoryMessage `json:"messages"`
	ChannelID string                `json:"channelID,omitempty"`
}

// An edit names a quoted span of the message it changes, the same contract the
// Mattermost path holds. The current text comes from the conversation's own
// record, the span is applied to it, and the whole result is sent, because that
// is the only edit the platform itself has.
func (service Service) invokeChatdPlatformMessageUpdate(ctx context.Context, request capabilities.ToolInvokeRequest, input platformMessageUpdateInput) (capabilities.ToolInvokeResponse, error) {
	messageID := strings.TrimSpace(input.MessageID)
	if input.NewText == nil || input.OldText == nil {
		return mattermostToolErrorResponse(request.ToolName, mattermostToolStaticFailure("invalid_input", "input_decode",
			"pass oldText with the exact current span and newText with its replacement")), nil
	}
	replyTargetID := strings.TrimSpace(request.Context.ReplyTargetID)
	if replyTargetID == "" {
		replyTargetID = strings.TrimSpace(request.Context.ConversationID)
	}
	currentText, failure, hasFailure := service.chatdCurrentMessageText(ctx, request.Context.Platform, replyTargetID, messageID)
	if hasFailure {
		return mattermostToolErrorResponse(request.ToolName, failure), nil
	}
	matchCount := strings.Count(currentText, *input.OldText)
	if matchCount != 1 {
		return mattermostToolErrorResponse(request.ToolName, platformMessageEditMatchFailure(matchCount, currentText)), nil
	}
	editedText := strings.Replace(currentText, *input.OldText, *input.NewText, 1)
	requestBody := map[string]any{"replyTargetID": replyTargetID, "messageID": messageID, "message": editedText}
	var response map[string]any
	if errorValue := service.chatdPlatformRequest(ctx, request.Context.Platform, "message.edit", requestBody, &response); errorValue != nil {
		return mattermostToolErrorResponse(request.ToolName, mattermostToolFailureForError("message_update", "platform_unavailable", errorValue)), nil
	}
	return mattermostToolSuccessResponse(request.ToolName, "updated", map[string]any{
		"messageID":      messageID,
		"deliveryStatus": "updated",
		"messageUpdated": true,
	})
}

func (service Service) chatdCurrentMessageText(ctx context.Context, platform string, replyTargetID string, messageID string) (string, mattermostToolFailure, bool) {
	if replyTargetID == "" {
		return "", mattermostToolStaticFailure("invalid_input", "platform_route",
			"a message is edited in the conversation it belongs to, and this call names none"), true
	}
	var response chatdHistoryFetchResponse
	historyRequest := chatdHistoryFetchRequest{ThreadID: replyTargetID, Limit: 50}
	if errorValue := service.chatdPlatformRequest(ctx, platform, "history.fetch", historyRequest, &response); errorValue != nil {
		return "", mattermostToolFailureForError("history_fetch", "platform_unavailable", errorValue), true
	}
	for _, message := range response.Messages {
		if strings.TrimSpace(message.ID) == messageID {
			return message.Text, mattermostToolFailure{}, false
		}
	}
	return "", mattermostToolStaticFailure("not_found", "message_lookup",
		"message "+messageID+" is not in the recent record of this conversation"), true
}
