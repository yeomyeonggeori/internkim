package capabilityd

import (
	"context"
	"fmt"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
	"github.com/yeomyeonggeori/internkim/internal/personname"
)

const personHintField = "personHint"
const personHintsField = "personHints"
const messageSendChannelField = "channelID"

func (service Service) resolveMessageSendTarget(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodePlatformMessageSendInput(request.Input)
	if errorValue != nil {
		return capabilityToolWithoutTargetResponse(request.ToolName), nil
	}
	switch input.DeliveryTarget.Type {
	case "directMessage":
		return service.resolveMessageDirectTarget(ctx, request, input.DeliveryTarget), nil
	case "channel":
		return service.resolveMessageChannelTarget(ctx, request, input.DeliveryTarget)
	}
	return capabilityToolWithoutTargetResponse(request.ToolName), nil
}

func (service Service) resolveMessageDirectTarget(ctx context.Context, request capabilities.ToolInvokeRequest, deliveryTarget platformMessageDeliveryTarget) capabilities.ToolInvokeResponse {
	if len(deliveryTarget.PersonHints) > 0 {
		return service.severalPeopleApprovalTargetResponse(ctx, request, personHintsField, deliveryTarget.PersonHints)
	}
	if deliveryTarget.PersonHint == "" {
		return capabilityToolWithoutTargetResponse(request.ToolName)
	}
	return service.onePersonApprovalTargetResponse(ctx, request, personHintField, deliveryTarget.PersonHint)
}

func (service Service) onePersonApprovalTargetResponse(ctx context.Context, request capabilities.ToolInvokeRequest, inputField string, personHint string) capabilities.ToolInvokeResponse {
	language := request.Context.ResponseLanguage
	person, failure, isRefused := service.resolvePersonHint(ctx, personHint, language)
	if isRefused {
		return platformDMErrorResponse(request.ToolName, failure)
	}
	return capabilityToolTargetResponse(request.ToolName, capabilities.ApprovalTarget{
		InputField: inputField,
		ID:         person.MemberID,
		Title:      personname.Render(person.Name, language),
	})
}

func (service Service) severalPeopleApprovalTargetResponse(ctx context.Context, request capabilities.ToolInvokeRequest, inputField string, personHints []string) capabilities.ToolInvokeResponse {
	language := request.Context.ResponseLanguage
	people, failure, isRefused := service.resolvePersonHints(ctx, personHints, language)
	if isRefused {
		return platformDMErrorResponse(request.ToolName, failure)
	}
	identifiers := make([]string, 0, len(people))
	names := make([]string, 0, len(people))
	for _, person := range people {
		identifiers = append(identifiers, person.MemberID)
		names = append(names, personname.Render(person.Name, language))
	}
	return capabilityToolTargetResponse(request.ToolName, capabilities.ApprovalTarget{
		InputField: inputField,
		IDs:        identifiers,
		Title:      strings.Join(names, "; "),
	})
}

func (service Service) resolveMessageChannelTarget(ctx context.Context, request capabilities.ToolInvokeRequest, deliveryTarget platformMessageDeliveryTarget) (capabilities.ToolInvokeResponse, error) {
	channels, errorValue := service.channelsTheRequesterMayName(ctx, request.Context)
	if errorValue != nil {
		return capabilityToolWithoutTargetResponse(request.ToolName), nil
	}
	if deliveryTarget.ChannelID != "" {
		return resolvedChannelIdentifierResponse(request, deliveryTarget.ChannelID, channels), nil
	}
	resolution := resolveHint(deliveryTarget.ChannelName, channels, nil)
	if resolution.Outcome == hintResolved {
		return channelTargetResponse(request, resolution.Match), nil
	}
	return platformDMErrorResponse(request.ToolName, messageChannelQuestion(deliveryTarget.ChannelName, resolution)), nil
}

func (service Service) channelsTheRequesterMayName(ctx context.Context, toolContext capabilities.ToolInvokeContext) ([]chatdChannel, error) {
	requesterPubkeyHex, errorValue := service.requesterKey(ctx, toolContext)
	if errorValue != nil {
		return nil, errorValue
	}
	listed, errorValue := service.channelsTheRequesterSees(ctx, requesterPubkeyHex)
	return namedChannels(listed), errorValue
}

func resolvedChannelIdentifierResponse(request capabilities.ToolInvokeRequest, channelID string, channels []chatdChannel) capabilities.ToolInvokeResponse {
	channel, isFound := itemWithHintIdentifier(channelID, channels)
	if !isFound {
		return platformDMErrorResponse(request.ToolName, platformDMStaticFailure("channel_not_found", "channel_resolve",
			fmt.Sprintf("channelID %q is not a channel on this messenger; name the channel with channelName instead", channelID)))
	}
	return channelTargetResponse(request, channel)
}

func channelTargetResponse(request capabilities.ToolInvokeRequest, channel chatdChannel) capabilities.ToolInvokeResponse {
	return capabilityToolTargetResponse(request.ToolName, capabilities.ApprovalTarget{
		InputField: messageSendChannelField,
		ID:         strings.TrimSpace(channel.ChannelID),
		Title:      channelTitle(channel),
	})
}

func messageChannelQuestion(channelName string, resolution hintResolution[chatdChannel]) platformDMFailure {
	var failure platformDMFailure
	switch resolution.Outcome {
	case hintAmbiguous:
		message := fmt.Sprintf("%q matches more than one channel: %s. Ask which one is meant, naming each by its ID.", channelName, channelList(resolution.Candidates))
		failure = platformDMStaticFailure("interaction_required", "target_resolution", message)
	case hintApproximate:
		message := fmt.Sprintf("no channel is called %q; the nearest are: %s. Ask whether one of them was meant, offering \"none of these\" as a choice.", channelName, channelList(resolution.Candidates))
		failure = platformDMStaticFailure("interaction_required", "target_resolution", message)
	default:
		return platformDMStaticFailure("channel_not_found", "channel_resolve", fmt.Sprintf("no channel is called %q on this messenger", channelName))
	}
	failure.Retryable = true
	failure.SafeRetry = true
	return failure
}

func channelList(channels []chatdChannel) string {
	titles := make([]string, 0, len(channels))
	for _, channel := range channels {
		titles = append(titles, channelTitle(channel))
	}
	return strings.Join(titles, "; ")
}
