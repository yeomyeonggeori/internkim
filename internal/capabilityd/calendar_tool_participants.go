package capabilityd

import (
	"context"
	"fmt"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func (service Service) prepareCalendarEventWriteInput(ctx context.Context, input calendarEventWriteInput, toolContext capabilities.ToolInvokeContext, includeRequesterDefault bool) (calendarEventWriteInput, platformDMFailure, bool) {
	participants := append([]calendarToolParticipant{}, input.Participants...)
	includeRequester := calendarToolShouldIncludeRequester(input, includeRequesterDefault)
	if includeRequester {
		participant := service.calendarRequesterParticipant(ctx, toolContext)
		if strings.TrimSpace(participant.Name) != "" || strings.TrimSpace(participant.Email) != "" || strings.TrimSpace(participant.PersonID) != "" {
			participants = append(participants, participant)
		}
	}
	for _, personHint := range input.People {
		participant, failure, hasFailure := service.calendarParticipantForPersonHint(ctx, personHint)
		if hasFailure {
			return calendarEventWriteInput{}, failure, true
		}
		participants = append(participants, participant)
	}
	input.Participants = normalizeCalendarToolParticipants(participants)
	if len(input.Participants) > 0 {
		input.People = calendarToolParticipantNames(input.Participants)
	}
	if input.GeneratedEventID {
		input.EventID = stableCalendarToolEventID(input)
	}
	return input, platformDMFailure{}, false
}

func calendarToolShouldIncludeRequester(input calendarEventWriteInput, includeRequesterDefault bool) bool {
	if calendarToolPeopleIncludesAll(input.People) {
		return false
	}
	if input.IncludeRequester != nil {
		return *input.IncludeRequester
	}
	return includeRequesterDefault
}

func calendarToolPeopleIncludesAll(people []string) bool {
	for _, person := range people {
		normalizedPerson := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(person, "@")))
		if normalizedPerson == "all" || normalizedPerson == "전체" {
			return true
		}
	}
	return false
}

func (service Service) calendarRequesterParticipant(ctx context.Context, toolContext capabilities.ToolInvokeContext) calendarToolParticipant {
	for _, hint := range calendarRequesterHints(toolContext) {
		resolution, errorValue := service.fetchPlatformDMRecipientResolution(ctx, hint)
		if errorValue != nil {
			continue
		}
		switch resolution.Status {
		case "resolved", "unlinked":
			participant := calendarToolParticipantFromRecipient(platformDMRecipientFromResolution(resolution.Recipient), hint)
			if strings.TrimSpace(participant.Name) != "" {
				return participant
			}
		}
	}
	return calendarToolParticipant{
		PersonID: strings.TrimSpace(toolContext.RequesterPersonID),
		Name:     firstNonEmpty(strings.TrimSpace(toolContext.RequesterName), strings.TrimSpace(toolContext.RequesterEmail), strings.TrimSpace(toolContext.RequesterPersonID)),
		Email:    strings.ToLower(strings.TrimSpace(toolContext.RequesterEmail)),
	}
}

func calendarRequesterHints(toolContext capabilities.ToolInvokeContext) []string {
	return uniqueTrimmedStringValues([]string{
		toolContext.RequesterEmail,
		toolContext.RequesterName,
		toolContext.RequesterPersonID,
	})
}

func (service Service) calendarParticipantForPersonHint(ctx context.Context, personHint string) (calendarToolParticipant, platformDMFailure, bool) {
	trimmedHint := strings.TrimSpace(personHint)
	if trimmedHint == "" {
		return calendarToolParticipant{}, platformDMFailure{}, false
	}
	resolution, errorValue := service.fetchPlatformDMRecipientResolution(ctx, trimmedHint)
	if errorValue != nil {
		return calendarToolParticipant{Name: trimmedHint}, platformDMFailure{}, false
	}
	switch resolution.Status {
	case "resolved", "unlinked":
		return calendarToolParticipantFromRecipient(platformDMRecipientFromResolution(resolution.Recipient), trimmedHint), platformDMFailure{}, false
	case "ambiguous":
		return calendarToolParticipant{}, calendarPersonAmbiguousFailure(trimmedHint, platformDMRecipientsFromResolution(resolution.Candidates)), true
	case "not_found":
		return calendarToolParticipant{Name: trimmedHint}, platformDMFailure{}, false
	default:
		return calendarToolParticipant{Name: trimmedHint}, platformDMFailure{}, false
	}
}

func calendarToolParticipantFromRecipient(recipient platformDMRecipient, fallbackName string) calendarToolParticipant {
	return calendarToolParticipant{
		PersonID: strings.TrimSpace(recipient.PersonID),
		Name:     firstNonEmpty(strings.TrimSpace(recipient.DisplayName), strings.TrimSpace(fallbackName), firstPlatformDMEmail(recipient.Emails)),
		Email:    firstPlatformDMEmail(recipient.Emails),
	}
}

func firstPlatformDMEmail(emails []string) string {
	if len(emails) == 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(emails[0]))
}

func calendarToolParticipantNames(participants []calendarToolParticipant) []string {
	names := make([]string, 0, len(participants))
	for _, participant := range participants {
		name := strings.TrimSpace(participant.Name)
		if name != "" {
			names = append(names, name)
		}
	}
	return normalizeCalendarToolPeople(names)
}

func calendarPersonAmbiguousFailure(personHint string, candidates []platformDMRecipient) platformDMFailure {
	message := fmt.Sprintf("calendar attendee %q is ambiguous: %s", personHint, platformDMRecipientList(candidates))
	failure := platformDMStaticFailure("recipient_ambiguous", "recipient_resolve", message)
	failure.Candidates = candidates
	failure.Retryable = true
	failure.SafeRetry = true
	return failure
}

func calendarToolPersonResolveErrorResponse(toolName string, failure platformDMFailure) capabilities.ToolInvokeResponse {
	return platformDMErrorResponse(toolName, failure)
}
