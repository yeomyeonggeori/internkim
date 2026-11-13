package capabilityd

import (
	"context"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

const (
	messageSenderRequester = "requester"
	messageSenderAgent     = "agent"
)

type requesterMessageSent struct {
	ID string `json:"id"`
}

func isPostedAsTheRequester(input platformMessageSendInput) bool {
	return input.Sender == messageSenderRequester && input.DeliveryTarget.Type != "directMessage"
}

// The requester cannot see a direct message the agent sends somebody else, so
// nothing they approve could show them it arrived as they meant it.
func isTheAgentWritingToAPerson(input platformMessageSendInput) bool {
	return input.Sender == messageSenderAgent &&
		input.DeliveryTarget.Type == "directMessage" &&
		!addressesOnlyTheRequester(input.DeliveryTarget)
}

func agentDirectMessageRefusedResponse(toolName string) capabilities.ToolInvokeResponse {
	return platformToolErrorResponse(toolName, platformToolStaticFailure("sender_not_allowed", "input_decode",
		"a direct message to a person goes out only under the requester's name; omit sender"))
}

func (service Service) postChatdMessageAsRequester(ctx context.Context, request capabilities.ToolInvokeRequest, input platformMessageSendInput) (capabilities.ToolInvokeResponse, error) {
	if len(input.Attachments) > 0 {
		return platformToolErrorResponse(request.ToolName, platformToolStaticFailure("sender_unsupported", "input_decode",
			"a message under the requester's name carries no attachments; omit sender to send the files under the agent's name")), nil
	}
	requesterPubkeyHex, errorValue := service.requesterKey(ctx, request.Context)
	if errorValue != nil {
		return platformToolErrorResponse(request.ToolName, requesterNotAddressable(request.Context, errorValue)), nil
	}
	channelID, refusal, isRefused := service.channelPostedAsTheRequester(ctx, request, requesterPubkeyHex, input.DeliveryTarget)
	if isRefused {
		return refusal, nil
	}
	var sent requesterMessageSent
	fields := map[string]any{"conversationID": channelID, "body": input.Message}
	if errorValue := service.askChatdAsTheRequester(ctx, "requester.message.send", requesterPubkeyHex, fields, &sent); errorValue != nil {
		return platformToolErrorResponse(request.ToolName, platformToolFailureForError("post_create", "platform_unavailable", errorValue)), nil
	}
	if strings.TrimSpace(sent.ID) == "" {
		return platformToolErrorResponse(request.ToolName, platformToolStaticFailure("post_create_failed", "post_create", "chatd did not return a message ID")), nil
	}
	return platformToolSuccessResponse(request.ToolName, "sent", platformMessageSendResult{
		MessageIDs:     []string{sent.ID},
		DeliveryStatus: "sent",
	})
}

func (service Service) channelPostedAsTheRequester(ctx context.Context, request capabilities.ToolInvokeRequest, requesterPubkeyHex string, target platformMessageDeliveryTarget) (string, capabilities.ToolInvokeResponse, bool) {
	switch target.Type {
	case "currentChannel":
		channelID := strings.TrimSpace(request.Context.ChannelID)
		if channelID == "" {
			return "", platformToolErrorResponse(request.ToolName, platformToolStaticFailure("channel_not_available", "context", "current platform channel is not available")), true
		}
		return channelID, capabilities.ToolInvokeResponse{}, false
	case "channel":
		channel, refusal, isRefused := service.requesterChannel(ctx, request, requesterPubkeyHex, firstNonEmpty(target.ChannelID, target.ChannelName))
		return channel.ChannelID, refusal, isRefused
	default:
		return "", platformToolErrorResponse(request.ToolName, platformToolStaticFailure("sender_unsupported", "input_decode",
			"sender=requester posts to a channel, with targetType channel or currentChannel; a reply in this thread goes out under the agent's name, so omit sender")), true
	}
}
