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
	attachmentFiles, failure, hasFailure := service.resolvePlatformMessageAttachments(input.Attachments)
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
		return chatdMessagePostRequest{
			ChannelID:   strings.TrimSpace(target.ChannelID),
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
