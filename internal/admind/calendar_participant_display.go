package admind

import (
	"strings"
)

func calendarParticipantsFromMembers(members []taskMember) []calendarParticipant {
	participants := make([]calendarParticipantIdentity, 0, len(members))
	for _, member := range members {
		participants = append(participants, calendarParticipantIdentity{
			PersonID: strings.TrimSpace(member.ID),
			Name:     strings.TrimSpace(member.Name),
			Email:    strings.ToLower(strings.TrimSpace(member.Email)),
		})
	}
	return calendarParticipantsFromIdentities(participants)
}

func calendarEventsWithNormalizedParticipants(events []calendarEvent) []calendarEvent {
	result := append([]calendarEvent(nil), events...)
	for index := range result {
		result[index].Participants = normalizeCalendarParticipants(result[index].Participants)
		result[index].People = calendarParticipantNames(result[index].Participants)
	}
	return result
}

func calendarEventWithNormalizedParticipants(event calendarEvent) calendarEvent {
	events := calendarEventsWithNormalizedParticipants([]calendarEvent{event})
	if len(events) == 0 {
		return event
	}
	return events[0]
}
