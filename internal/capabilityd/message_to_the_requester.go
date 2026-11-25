package capabilityd

import (
	"context"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

func (service Service) sendReachesOnlyTheRequester(ctx context.Context, request capabilities.ToolInvokeRequest) bool {
	if request.ToolName != "message_send" {
		return false
	}
	input, errorValue := decodePlatformMessageSendInput(request.Input)
	if errorValue != nil || input.DeliveryTarget.Type != "directMessage" || addressesOnlyTheRequester(input.DeliveryTarget) {
		return false
	}
	people, _, isRefused := service.resolvePersonHints(ctx, namedRecipients(input.DeliveryTarget), request.Context.ResponseLanguage)
	return !isRefused && isOnlyTheRequester(request.Context, people)
}

func namedRecipients(target platformMessageDeliveryTarget) []string {
	if target.PersonHint == "" {
		return target.PersonHints
	}
	return append([]string{target.PersonHint}, target.PersonHints...)
}

func isTheRequestersOwnConversation(toolContext capabilities.ToolInvokeContext, recipients []directoryPerson) bool {
	return hasArrivedOnAConversation(toolContext) && isOnlyTheRequester(toolContext, recipients)
}

func isOnlyTheRequester(toolContext capabilities.ToolInvokeContext, people []directoryPerson) bool {
	if len(people) == 0 {
		return false
	}
	for _, person := range people {
		if !isTheRequester(toolContext, person) {
			return false
		}
	}
	return true
}

func isTheRequester(toolContext capabilities.ToolInvokeContext, person directoryPerson) bool {
	requesterPersonID := strings.TrimSpace(toolContext.RequesterPersonID)
	requesterEmail := strings.TrimSpace(toolContext.RequesterEmail)
	return (requesterPersonID != "" && strings.TrimSpace(person.MemberID) == requesterPersonID) ||
		(requesterEmail != "" && strings.EqualFold(strings.TrimSpace(person.Email), requesterEmail))
}

func (input platformMessageSendInput) toTheRequestersOwnConversation() platformMessageSendInput {
	input.DeliveryTarget.PersonHint = ""
	input.DeliveryTarget.PersonHints = nil
	return input
}
