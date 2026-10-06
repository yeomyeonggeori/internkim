package capabilityd

import (
	"context"
	"fmt"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

const messageSendRecipientField = "personHint"

func (service Service) resolveMessageSendTarget(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodePlatformMessageSendInput(request.Input)
	if errorValue != nil || input.DeliveryTarget.Type != "directMessage" {
		return capabilityToolWithoutTargetResponse(request.ToolName), nil
	}
	if len(input.DeliveryTarget.PersonHints) > 0 {
		return service.resolveMessageBroadcastTarget(ctx, request, input.DeliveryTarget.PersonHints)
	}
	if input.DeliveryTarget.PersonHint == "" {
		return capabilityToolWithoutTargetResponse(request.ToolName), nil
	}
	person, response, isRefused := service.resolveMessageRecipient(ctx, request, input.DeliveryTarget.PersonHint)
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
	titles := make([]string, 0, len(personHints))
	for _, personHint := range personHints {
		person, response, isRefused := service.resolveMessageRecipient(ctx, request, personHint)
		if isRefused {
			return response, nil
		}
		titles = append(titles, recipientTitle(person))
	}
	return capabilityToolTargetResponse(request.ToolName, capabilities.ApprovalTarget{
		Preview: strings.Join(titles, "\n"),
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
