package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

const (
	channelAddToolName          = "channel_add"
	channelJoinToolName         = "channel_join"
	channelLeaveToolName        = "channel_leave"
	channelDeleteToolName       = "channel_delete"
	channelMemberAddToolName    = "channel_member_add"
	channelMemberDeleteToolName = "channel_member_delete"
	channelOwnerAddToolName     = "channel_owner_add"
	channelOwnerUpdateToolName  = "channel_owner_update"
)

const channelHintField = "channelHint"

type channelToolInput struct {
	ChannelHint string   `json:"channelHint"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Visibility  string   `json:"visibility"`
	PersonHint  string   `json:"personHint"`
	PersonHints []string `json:"personHints"`
}

type channelToolResult struct {
	ChannelID           string   `json:"channelID"`
	ChannelName         string   `json:"channelName"`
	NotAddedPersonHints []string `json:"notAddedPersonHints,omitempty"`
}

type requesterConversation struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type channelMember struct {
	PersonHint string
	PubkeyHex  string
}

// Every change is made by chatd as the person who asked, through the same
// person capability their messenger screen calls, so who may change a channel
// is decided once, where the screen's answer is decided.
func (service Service) invokeChannelTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if response, isDenied := service.authorizePlatformMessageTool(ctx, request); isDenied {
		return response, nil
	}
	input, errorValue := decodeChannelToolInput(request.Input)
	if errorValue != nil {
		return platformToolErrorResponse(request.ToolName, platformToolStaticFailure("invalid_input", "input_decode", errorValue.Error())), nil
	}
	requesterPubkeyHex, errorValue := service.directoryBuzzKey(ctx, request.Context.RequesterEmail)
	if errorValue != nil {
		return platformToolErrorResponse(request.ToolName, platformToolStaticFailure("requester_not_addressable", "requester_resolve",
			"this change was to be made as "+request.Context.RequesterEmail+" and could not: "+errorValue.Error())), nil
	}
	if request.ToolName == channelAddToolName {
		return service.addChannel(ctx, request, requesterPubkeyHex, input)
	}
	channel, response, isRefused := service.requesterChannel(ctx, request, requesterPubkeyHex, input.ChannelHint)
	if isRefused {
		return response, nil
	}
	return service.changeChannel(ctx, request, requesterPubkeyHex, channel, input)
}

func (service Service) addChannel(ctx context.Context, request capabilities.ToolInvokeRequest, requesterPubkeyHex string, input channelToolInput) (capabilities.ToolInvokeResponse, error) {
	members, response, isRefused := service.channelMembers(ctx, request, input.PersonHints)
	if isRefused {
		return response, nil
	}
	var created struct {
		ID                   string   `json:"id"`
		Name                 string   `json:"name"`
		UninvitedExternalIDs []string `json:"uninvitedExternalIDs"`
	}
	fields := map[string]any{
		"name":              input.Name,
		"description":       input.Description,
		"visibility":        input.Visibility,
		"memberExternalIDs": memberPubkeys(members),
	}
	if errorValue := service.askChatdAsTheRequester(ctx, "requester.channel.create", requesterPubkeyHex, fields, &created); errorValue != nil {
		return channelChangeRefusedResponse(request.ToolName, errorValue), nil
	}
	return platformToolSuccessResponse(request.ToolName, "created", channelToolResult{
		ChannelID:           created.ID,
		ChannelName:         created.Name,
		NotAddedPersonHints: personHintsOf(members, created.UninvitedExternalIDs),
	})
}

func (service Service) changeChannel(ctx context.Context, request capabilities.ToolInvokeRequest, requesterPubkeyHex string, channel chatdChannel, input channelToolInput) (capabilities.ToolInvokeResponse, error) {
	result := channelToolResult{ChannelID: channel.ChannelID, ChannelName: channel.Name}
	fields := map[string]any{"conversationID": channel.ChannelID}
	capability, status := channelCapabilityFor(request.ToolName)
	switch request.ToolName {
	case channelMemberAddToolName:
		members, response, isRefused := service.channelMembers(ctx, request, input.PersonHints)
		if isRefused {
			return response, nil
		}
		fields["memberExternalIDs"] = memberPubkeys(members)
		var added struct {
			UninvitedExternalIDs []string `json:"uninvitedExternalIDs"`
		}
		if errorValue := service.askChatdAsTheRequester(ctx, capability, requesterPubkeyHex, fields, &added); errorValue != nil {
			return channelChangeRefusedResponse(request.ToolName, errorValue), nil
		}
		result.NotAddedPersonHints = personHintsOf(members, added.UninvitedExternalIDs)
		return platformToolSuccessResponse(request.ToolName, status, result)
	case channelMemberDeleteToolName, channelOwnerAddToolName, channelOwnerUpdateToolName:
		members, response, isRefused := service.channelMembers(ctx, request, []string{input.PersonHint})
		if isRefused {
			return response, nil
		}
		fields["externalID"] = members[0].PubkeyHex
	}
	if errorValue := service.askChatdAsTheRequester(ctx, capability, requesterPubkeyHex, fields, nil); errorValue != nil {
		return channelChangeRefusedResponse(request.ToolName, errorValue), nil
	}
	return platformToolSuccessResponse(request.ToolName, status, result)
}

func channelCapabilityFor(toolName string) (string, string) {
	switch toolName {
	case channelJoinToolName:
		return "requester.channel.join", "joined"
	case channelLeaveToolName:
		return "requester.channel.leave", "left"
	case channelDeleteToolName:
		return "requester.channel.delete", "deleted"
	case channelMemberAddToolName:
		return "requester.channel.members.add", "updated"
	case channelMemberDeleteToolName:
		return "requester.channel.member.remove", "updated"
	case channelOwnerAddToolName:
		return "requester.channel.owner.add", "updated"
	default:
		return "requester.channel.owner.set", "updated"
	}
}

// A channel somebody is in is found among their own conversations; one they
// are joining is found among the open channels they may join. Either way the
// hint climbs the same ladder as every other named thing.
func (service Service) requesterChannel(ctx context.Context, request capabilities.ToolInvokeRequest, requesterPubkeyHex string, channelHint string) (chatdChannel, capabilities.ToolInvokeResponse, bool) {
	channels, errorValue := service.requesterChannels(ctx, request.ToolName, requesterPubkeyHex)
	if errorValue != nil {
		return chatdChannel{}, platformToolErrorResponse(request.ToolName, platformToolFailureForError("channel_resolve", "platform_unavailable", errorValue)), true
	}
	resolution := resolveHint(channelHint, channels, nil)
	if resolution.Outcome == hintResolved {
		return resolution.Match, capabilities.ToolInvokeResponse{}, false
	}
	return chatdChannel{}, platformDMErrorResponse(request.ToolName, messageChannelQuestion(channelHint, resolution)), true
}

func (service Service) requesterChannels(ctx context.Context, toolName string, requesterPubkeyHex string) ([]chatdChannel, error) {
	if toolName == channelJoinToolName {
		var answered struct {
			Channels []requesterConversation `json:"channels"`
		}
		errorValue := service.askChatdAsTheRequester(ctx, "requester.channels.open.list", requesterPubkeyHex, map[string]any{}, &answered)
		return groupChannels(answered.Channels), errorValue
	}
	var answered struct {
		Conversations []requesterConversation `json:"conversations"`
	}
	errorValue := service.askChatdAsTheRequester(ctx, "requester.conversations.list", requesterPubkeyHex, map[string]any{}, &answered)
	return groupChannels(answered.Conversations), errorValue
}

func groupChannels(conversations []requesterConversation) []chatdChannel {
	channels := []chatdChannel{}
	for _, conversation := range conversations {
		if conversation.Kind == "dm" {
			continue
		}
		channels = append(channels, chatdChannel{ChannelID: conversation.ID, Name: conversation.Name})
	}
	return channels
}

func (service Service) channelMembers(ctx context.Context, request capabilities.ToolInvokeRequest, personHints []string) ([]channelMember, capabilities.ToolInvokeResponse, bool) {
	members := make([]channelMember, 0, len(personHints))
	for _, personHint := range personHints {
		person, response, isRefused := service.resolveMessageRecipient(ctx, request, personHint)
		if isRefused {
			return nil, response, true
		}
		pubkeyHex, errorValue := service.directoryBuzzKey(ctx, person.Email)
		if errorValue != nil {
			return nil, platformDMErrorResponse(request.ToolName, platformDMUnavailableFailure(errorValue)), true
		}
		members = append(members, channelMember{PersonHint: personHint, PubkeyHex: pubkeyHex})
	}
	return members, capabilities.ToolInvokeResponse{}, false
}

func (service Service) askChatdAsTheRequester(ctx context.Context, capability string, requesterPubkeyHex string, fields map[string]any, response any) error {
	fields["requesterPubkeyHex"] = requesterPubkeyHex
	return service.chatdRequest(ctx, capability, fields, response)
}

func channelChangeRefusedResponse(toolName string, errorValue error) capabilities.ToolInvokeResponse {
	return platformToolErrorResponse(toolName, platformToolFailureForError("channel_change", "channel_change_refused", errorValue))
}

func memberPubkeys(members []channelMember) []string {
	pubkeys := make([]string, 0, len(members))
	for _, member := range members {
		pubkeys = append(pubkeys, member.PubkeyHex)
	}
	return pubkeys
}

func personHintsOf(members []channelMember, pubkeys []string) []string {
	personHints := []string{}
	for _, member := range members {
		for _, pubkeyHex := range pubkeys {
			if strings.EqualFold(member.PubkeyHex, pubkeyHex) {
				personHints = append(personHints, member.PersonHint)
			}
		}
	}
	return personHints
}

func decodeChannelToolInput(document json.RawMessage) (channelToolInput, error) {
	var input channelToolInput
	if len(bytes.TrimSpace(document)) > 0 {
		if errorValue := json.Unmarshal(document, &input); errorValue != nil {
			return channelToolInput{}, errorValue
		}
	}
	input.ChannelHint = strings.TrimSpace(input.ChannelHint)
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.PersonHint = strings.TrimSpace(input.PersonHint)
	input.PersonHints = uniqueTrimmedPlatformMessageHints(input.PersonHints)
	return input, nil
}

// Deleting a channel and handing it over are approved against the channel the
// hint resolves to, so the question names it rather than echoing the hint.
func (service Service) resolveChannelChangeTarget(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeChannelToolInput(request.Input)
	if errorValue != nil {
		return capabilityToolWithoutTargetResponse(request.ToolName), nil
	}
	requesterPubkeyHex, errorValue := service.directoryBuzzKey(ctx, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilityToolWithoutTargetResponse(request.ToolName), nil
	}
	channel, response, isRefused := service.requesterChannel(ctx, request, requesterPubkeyHex, input.ChannelHint)
	if isRefused {
		return response, nil
	}
	title := channelTitle(channel)
	if input.PersonHint != "" {
		title = fmt.Sprintf("%s → %s", title, input.PersonHint)
	}
	return capabilityToolTargetResponse(request.ToolName, capabilities.ApprovalTarget{
		InputField: channelHintField,
		ID:         channel.ChannelID,
		Title:      title,
	}), nil
}
