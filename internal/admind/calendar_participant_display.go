package admind

import (
	"net/http"
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
	normalizedParticipants := calendarParticipantsFromIdentities(participants)
	for index := range normalizedParticipants {
		normalizedParticipants[index].Image = calendarParticipantImagePath(normalizedParticipants[index].PersonID)
	}
	return normalizedParticipants
}

func (service *Service) calendarEventsWithParticipantImages(request *http.Request, events []calendarEvent) []calendarEvent {
	members := service.taskMembers(request)
	result := append([]calendarEvent(nil), events...)
	for index := range result {
		result[index].Participants = calendarParticipantsWithMemberImages(result[index].Participants, members)
		result[index].People = calendarParticipantNames(result[index].Participants)
	}
	return result
}

func (service *Service) calendarEventWithParticipantImages(request *http.Request, event calendarEvent) calendarEvent {
	events := service.calendarEventsWithParticipantImages(request, []calendarEvent{event})
	if len(events) == 0 {
		return event
	}
	return events[0]
}

func calendarParticipantsWithMemberImages(participants []calendarParticipant, members []taskMember) []calendarParticipant {
	memberParticipants := calendarParticipantsFromMembers(members)
	result := normalizeCalendarParticipants(participants)
	for index := range result {
		for _, memberParticipant := range memberParticipants {
			if calendarParticipantsSamePerson(result[index], memberParticipant) {
				result[index].Image = memberParticipant.Image
				break
			}
		}
	}
	return result
}
