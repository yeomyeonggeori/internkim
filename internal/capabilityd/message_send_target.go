package capabilityd

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

const messageSendRecipientField = "personHint"
const messageSendBroadcastRecipientsField = "personHints"
const messageSendChannelField = "channelID"

func (service Service) resolveMessageSendTarget(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodePlatformMessageSendInput(request.Input)
	if errorValue != nil {
		return capabilityToolWithoutTargetResponse(request.ToolName), nil
	}
	switch input.DeliveryTarget.Type {
	case "directMessage":
		return service.resolveMessageDirectTarget(ctx, request, input.DeliveryTarget)
	case "channel":
		return service.resolveMessageChannelTarget(ctx, request, input.DeliveryTarget)
	}
	return capabilityToolWithoutTargetResponse(request.ToolName), nil
}

func (service Service) resolveMessageDirectTarget(ctx context.Context, request capabilities.ToolInvokeRequest, deliveryTarget platformMessageDeliveryTarget) (capabilities.ToolInvokeResponse, error) {
	if len(deliveryTarget.PersonHints) > 0 {
		return service.resolveMessageBroadcastTarget(ctx, request, deliveryTarget.PersonHints)
	}
	if deliveryTarget.PersonHint == "" {
		return capabilityToolWithoutTargetResponse(request.ToolName), nil
	}
	person, response, isRefused := service.resolveMessageRecipient(ctx, request, deliveryTarget.PersonHint)
	if isRefused {
		return response, nil
	}
	return capabilityToolTargetResponse(request.ToolName, capabilities.ApprovalTarget{
		InputField: messageSendRecipientField,
		ID:         exactRecipientIdentifier(person),
		Title:      recipientTitle(person),
	}), nil
}

func (service Service) resolveMessageBroadcastTarget(ctx context.Context, request capabilities.ToolInvokeRequest, personHints []string) (capabilities.ToolInvokeResponse, error) {
	identifiers := make([]string, 0, len(personHints))
	titles := make([]string, 0, len(personHints))
	for _, personHint := range personHints {
		person, response, isRefused := service.resolveMessageRecipient(ctx, request, personHint)
		if isRefused {
			return response, nil
		}
		identifier := exactRecipientIdentifier(person)
		if slices.Contains(identifiers, identifier) {
			continue
		}
		identifiers = append(identifiers, identifier)
		titles = append(titles, recipientTitle(person))
	}
	return capabilityToolTargetResponse(request.ToolName, capabilities.ApprovalTarget{
		InputField: messageSendBroadcastRecipientsField,
		IDs:        identifiers,
		Title:      strings.Join(titles, "; "),
	}), nil
}

func (service Service) resolveMessageRecipient(ctx context.Context, request capabilities.ToolInvokeRequest, personHint string) (directoryPerson, capabilities.ToolInvokeResponse, bool) {
	resolution, errorValue := service.resolveDirectoryPersonHint(ctx, personHint)
	if errorValue != nil {
		return directoryPerson{}, platformDMErrorResponse(request.ToolName, platformDMUnavailableFailure(errorValue)), true
	}
	if resolution.Outcome == hintResolved {
		return resolution.Match, capabilities.ToolInvokeResponse{}, false
	}
	failure := messageRecipientQuestion(personHint, resolution, request.Context.ResponseLanguage)
	return directoryPerson{}, platformDMErrorResponse(request.ToolName, failure), true
}

func messageRecipientQuestion(personHint string, resolution hintResolution[directoryPerson], responseLanguage string) platformDMFailure {
	candidates := platformDMRecipientsFromDirectoryPeople(resolution.Candidates, responseLanguage)
	var failure platformDMFailure
	switch resolution.Outcome {
	case hintAmbiguous:
		message := fmt.Sprintf("%q matches more than one person: %s. Ask which one is meant, naming each by address.", personHint, platformDMRecipientList(candidates))
		failure = platformDMStaticFailure("interaction_required", "target_resolution", message)
	case hintApproximate:
		message := fmt.Sprintf("no one is called %q; the nearest are: %s. Ask whether one of them was meant, offering \"none of these\" as a choice.", personHint, platformDMRecipientList(candidates))
		failure = platformDMStaticFailure("interaction_required", "target_resolution", message)
	default:
		return platformDMRecipientNotFoundFailure(personHint)
	}
	failure.Retryable = true
	failure.SafeRetry = true
	failure.Candidates = candidates
	return failure
}

func exactRecipientIdentifier(person directoryPerson) string {
	return firstNonEmpty(strings.TrimSpace(person.MemberID), strings.TrimSpace(person.Email))
}

func recipientTitle(person directoryPerson) string {
	return strings.TrimSpace(person.Name) + " <" + strings.TrimSpace(person.Email) + ">"
}

func (service Service) resolveMessageChannelTarget(ctx context.Context, request capabilities.ToolInvokeRequest, deliveryTarget platformMessageDeliveryTarget) (capabilities.ToolInvokeResponse, error) {
	channels, errorValue := service.chatdChannels(ctx)
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
