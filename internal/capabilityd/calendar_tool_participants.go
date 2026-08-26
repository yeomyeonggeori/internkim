package capabilityd

import (
	"context"
	"fmt"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/internal/personname"
)

// Who attends is whoever was named. Nobody named is the person asking, and
// everybody named is nobody in particular: an event the whole company is invited
// to carries no attendee list to narrow it by.
func (service Service) prepareCalendarEventWriteInput(ctx context.Context, input calendarEventWriteInput, toolContext capabilities.ToolInvokeContext, mayAddTheRequesterAlone bool) (calendarEventWriteInput, platformDMFailure, bool) {
	if input.EveryoneAttends {
		input.People = nil
		input.Participants = nil
		return input, platformDMFailure{}, false
	}
	participants := append([]calendarToolParticipant{}, input.Participants...)
	if calendarToolShouldIncludeRequester(input, mayAddTheRequesterAlone) {
		participant := service.calendarRequesterParticipant(ctx, toolContext)
		if strings.TrimSpace(participant.Name) != "" || strings.TrimSpace(participant.Email) != "" || strings.TrimSpace(participant.PersonID) != "" {
			participants = append(participants, participant)
		}
	}
	for _, personHint := range input.People {
		participant, failure, hasFailure := service.calendarParticipantForPersonHint(ctx, personHint, toolContext.ResponseLanguage)
		if hasFailure {
			return calendarEventWriteInput{}, failure, true
		}
		participants = append(participants, participant)
	}
	input.Participants = normalizeCalendarToolParticipants(participants)
	if len(input.Participants) > 0 {
		input.People = calendarToolParticipantNames(input.Participants)
	}
	input.IsRequestedOfSomebodyElse = isRequestedOfSomebodyElse(service.calendarRequesterParticipant(ctx, toolContext), input.Participants)
	return input, platformDMFailure{}, false
}

// Somebody asked somebody else to be somewhere, and the record says so by
// standing at requested until they answer. An event the person asking attends,
// or one the whole company is invited to, was nobody's request of anybody. The
// company stamps who asked from the status, so this is the only decision.
func isRequestedOfSomebodyElse(requester calendarToolParticipant, participants []calendarToolParticipant) bool {
	if len(participants) == 0 {
		return false
	}
	requesterEmail := strings.ToLower(strings.TrimSpace(requester.Email))
	requesterPersonID := strings.TrimSpace(requester.PersonID)
	if requesterEmail == "" && requesterPersonID == "" {
		return false
	}
	for _, participant := range participants {
		if requesterEmail != "" && strings.ToLower(strings.TrimSpace(participant.Email)) == requesterEmail {
			return false
		}
		if requesterPersonID != "" && strings.TrimSpace(participant.PersonID) == requesterPersonID {
			return false
		}
	}
	return true
}

func calendarToolShouldIncludeRequester(input calendarEventWriteInput, mayAddTheRequesterAlone bool) bool {
	if input.IncludeRequester != nil {
		return *input.IncludeRequester
	}
	// Naming somebody says whose event it is. Adding the person who asked as well
	// puts them in a meeting they said was not theirs.
	return mayAddTheRequesterAlone && len(input.People) == 0 && len(input.Participants) == 0
}

// The person asking is looked up in the same directory as everybody else, so
// they carry the company's own identifier rather than whatever this messenger
// calls them.
func (service Service) calendarRequesterParticipant(ctx context.Context, toolContext capabilities.ToolInvokeContext) calendarToolParticipant {
	for _, hint := range calendarRequesterHints(toolContext) {
		resolution, errorValue := service.resolveDirectoryPersonHint(ctx, hint)
		if errorValue != nil {
			continue
		}
		if resolution.Outcome == hintResolved {
			return calendarToolParticipantFromDirectoryPerson(resolution.Match, hint, toolContext.ResponseLanguage)
		}
	}
	return calendarToolParticipant{
		PersonID: strings.TrimSpace(toolContext.RequesterPersonID),
		Name:     firstNonEmpty(personname.Render(toolContext.RequesterName, toolContext.ResponseLanguage), strings.TrimSpace(toolContext.RequesterEmail), strings.TrimSpace(toolContext.RequesterPersonID)),
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

// An attendee nobody can place must not become an attendee anyway: a name the
// company does not carry once went onto the event as written, the company kept
// no participant under it, and the person who filed the event was left as its
// only attendee.
func (service Service) calendarParticipantForPersonHint(ctx context.Context, personHint string, responseLanguage string) (calendarToolParticipant, platformDMFailure, bool) {
	trimmedHint := strings.TrimSpace(personHint)
	if trimmedHint == "" {
		return calendarToolParticipant{}, platformDMFailure{}, false
	}
	resolution, errorValue := service.resolveDirectoryPersonHint(ctx, trimmedHint)
	if errorValue != nil {
		return calendarToolParticipant{}, platformDMUnavailableFailure(errorValue), true
	}
	switch resolution.Outcome {
	case hintResolved:
		return calendarToolParticipantFromDirectoryPerson(resolution.Match, trimmedHint, responseLanguage), platformDMFailure{}, false
	case hintAmbiguous, hintApproximate:
		return calendarToolParticipant{}, calendarPersonAmbiguousFailure(trimmedHint, resolution.Candidates, responseLanguage), true
	default:
		return calendarToolParticipant{}, calendarPersonNotFoundFailure(trimmedHint), true
	}
}

func calendarToolParticipantFromDirectoryPerson(person directoryPerson, fallbackName string, responseLanguage string) calendarToolParticipant {
	return calendarToolParticipant{
		PersonID: strings.TrimSpace(person.MemberID),
		Name:     firstNonEmpty(personname.Render(person.Name, responseLanguage), strings.TrimSpace(fallbackName), strings.TrimSpace(person.Email)),
		Email:    strings.TrimSpace(person.Email),
	}
}

func calendarPersonNotFoundFailure(personHint string) platformDMFailure {
	message := fmt.Sprintf("calendar attendee %q is not someone the company directory carries", personHint)
	failure := platformDMStaticFailure("recipient_not_found", "recipient_resolve", message)
	failure.Retryable = true
	failure.SafeRetry = true
	return failure
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

func calendarPersonAmbiguousFailure(personHint string, candidates []directoryPerson, responseLanguage string) platformDMFailure {
	message := fmt.Sprintf("calendar attendee %q is ambiguous: %s", personHint, strings.Join(directoryPersonNames(candidates, responseLanguage), ", "))
	failure := platformDMStaticFailure("recipient_ambiguous", "recipient_resolve", message)
	failure.Candidates = platformDMRecipientsFromDirectoryPeople(candidates, responseLanguage)
	failure.Retryable = true
	failure.SafeRetry = true
	return failure
}

func calendarToolPersonResolveErrorResponse(toolName string, failure platformDMFailure) capabilities.ToolInvokeResponse {
	response := platformDMErrorResponse(toolName, failure)
	response.Outcome = capabilities.ToolOutcomeFailed
	return response
}
