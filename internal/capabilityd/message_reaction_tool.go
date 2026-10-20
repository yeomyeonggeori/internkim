package capabilityd

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

const (
	messageReactionAddToolName    = "message_reaction_add"
	messageReactionDeleteToolName = "message_reaction_delete"
	conversationHideToolName      = "conversation_hide"
	conversationRevealToolName    = "conversation_reveal"
)

type messageReactionInput struct {
	MessageID string `json:"messageID"`
	Emoji     string `json:"emoji"`
}

type messageReactionResult struct {
	MessageID string `json:"messageID"`
	Emoji     string `json:"emoji"`
}

type conversationVisibilityInput struct {
	PersonHint string `json:"personHint"`
}

type conversationVisibilityResult struct {
	ConversationID string `json:"conversationID"`
	IsHidden       bool   `json:"isHidden"`
}

// A reaction and a hidden conversation belong to the person who asked, so
// both are made by chatd as that person, through the capability their
// messenger screen calls.
func (service Service) invokeMessageReactionTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if response, isDenied := service.authorizePlatformMessageTool(ctx, request); isDenied {
		return response, nil
	}
	var input messageReactionInput
	if errorValue := json.Unmarshal(request.Input, &input); errorValue != nil {
		return platformToolErrorResponse(request.ToolName, platformToolStaticFailure("invalid_input", "input_decode", errorValue.Error())), nil
	}
	messageID := strings.TrimSpace(input.MessageID)
	requesterPubkeyHex, response, isRefused := service.requesterMessengerKey(ctx, request)
	if isRefused {
		return response, nil
	}
	channelID, failure, hasFailure := service.channelHoldingMessage(ctx, messageID)
	if hasFailure {
		return platformToolErrorResponse(request.ToolName, failure), nil
	}
	capability := "requester.reaction.add"
	if request.ToolName == messageReactionDeleteToolName {
		capability = "requester.reaction.remove"
	}
	fields := map[string]any{"conversationID": channelID, "messageID": messageID, "emoji": strings.TrimSpace(input.Emoji)}
	if errorValue := service.askChatdAsTheRequester(ctx, capability, requesterPubkeyHex, fields, nil); errorValue != nil {
		return platformToolErrorResponse(request.ToolName, platformToolFailureForError("message_reaction", "message_reaction_refused", errorValue)), nil
	}
	return platformToolSuccessResponse(request.ToolName, "updated", messageReactionResult{MessageID: messageID, Emoji: strings.TrimSpace(input.Emoji)})
}

func (service Service) channelHoldingMessage(ctx context.Context, messageID string) (string, platformToolFailure, bool) {
	var response chatdMessageSearchResponse
	searchRequest := chatdMessageSearchRequest{MessageIDs: []string{messageID}, Queries: []string{}}
	if errorValue := service.chatdRequest(ctx, "message.search", searchRequest, &response); errorValue != nil {
		return "", platformToolFailureForError("message_lookup", "platform_unavailable", errorValue), true
	}
	for _, candidate := range response.Candidates {
		if strings.TrimSpace(candidate.MessageID) == messageID && strings.TrimSpace(candidate.ChannelID) != "" {
			return strings.TrimSpace(candidate.ChannelID), platformToolFailure{}, false
		}
	}
	return "", platformToolStaticFailure("not_found", "message_lookup",
		"message "+messageID+" does not exist on this platform or was deleted"), true
}

func (service Service) invokeConversationVisibilityTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if response, isDenied := service.authorizePlatformMessageTool(ctx, request); isDenied {
		return response, nil
	}
	var input conversationVisibilityInput
	if errorValue := json.Unmarshal(request.Input, &input); errorValue != nil {
		return platformToolErrorResponse(request.ToolName, platformToolStaticFailure("invalid_input", "input_decode", errorValue.Error())), nil
	}
	requesterPubkeyHex, response, isRefused := service.requesterMessengerKey(ctx, request)
	if isRefused {
		return response, nil
	}
	counterparts, response, isRefused := service.messengerPeople(ctx, request, []string{strings.TrimSpace(input.PersonHint)})
	if isRefused {
		return response, nil
	}
	conversationID, failure, hasFailure := service.directConversationWith(ctx, requesterPubkeyHex, counterparts[0])
	if hasFailure {
		return platformToolErrorResponse(request.ToolName, failure), nil
	}
	isHidden := request.ToolName == conversationHideToolName
	capability := "requester.dm.reveal"
	if isHidden {
		capability = "requester.dm.hide"
	}
	if errorValue := service.askChatdAsTheRequester(ctx, capability, requesterPubkeyHex, map[string]any{"conversationID": conversationID}, nil); errorValue != nil {
		return platformToolErrorResponse(request.ToolName, platformToolFailureForError("conversation_visibility", "conversation_visibility_refused", errorValue)), nil
	}
	return platformToolSuccessResponse(request.ToolName, "updated", conversationVisibilityResult{ConversationID: conversationID, IsHidden: isHidden})
}

func (service Service) directConversationWith(ctx context.Context, requesterPubkeyHex string, counterpart messengerPerson) (string, platformToolFailure, bool) {
	var answered struct {
		Conversations []requesterConversation `json:"conversations"`
	}
	if errorValue := service.askChatdAsTheRequester(ctx, "requester.conversations.list", requesterPubkeyHex, map[string]any{}, &answered); errorValue != nil {
		return "", platformToolFailureForError("conversation_resolve", "platform_unavailable", errorValue), true
	}
	for _, conversation := range answered.Conversations {
		if conversation.Kind == "dm" && isOnlyCounterpart(conversation.ParticipantExternalIDs, requesterPubkeyHex, counterpart.PubkeyHex) {
			return conversation.ID, platformToolFailure{}, false
		}
	}
	return "", platformToolStaticFailure("conversation_not_found", "conversation_resolve",
		"the requester has no direct conversation with "+counterpart.PersonHint), true
}

func isOnlyCounterpart(participants []string, requesterPubkeyHex string, counterpartPubkeyHex string) bool {
	others := []string{}
	for _, participant := range participants {
		if !strings.EqualFold(participant, requesterPubkeyHex) {
			others = append(others, participant)
		}
	}
	return len(others) == 1 && strings.EqualFold(others[0], counterpartPubkeyHex)
}
