package admind

import (
	"net/http"
	"strings"
)

func calendarParticipantsFromMembers(members []flowMember) []calendarParticipant {
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
	members := service.flowMembers(request)
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

func calendarParticipantsWithMemberImages(participants []calendarParticipant, members []flowMember) []calendarParticipant {
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

func calendarEventRelatedParticipants(event calendarEvent) []calendarParticipant {
	participants := normalizeCalendarParticipants(event.Participants)
	if len(participants) == 0 {
		if people, hasPeopleLine := calendarPeopleFromDescription(event.Description); hasPeopleLine {
			participants = calendarParticipantsFromPeople(people)
		}
	}
	creatorParticipant := calendarEventCreatorParticipant(event)
	if calendarParticipantsIncludeParticipant(participants, creatorParticipant) {
		return participants
	}
	return normalizeCalendarParticipants(append(participants, creatorParticipant))
}

func calendarEventCreatorParticipant(event calendarEvent) calendarParticipant {
	return calendarParticipant{
		Name:  firstNonEmpty(strings.TrimSpace(event.CreatedByName), strings.TrimSpace(event.CreatedByEmail)),
		Email: strings.TrimSpace(event.CreatedByEmail),
	}
}

func calendarNotificationPeople(event calendarEvent) ([]string, bool) {
	people := calendarParticipantNames(event.Participants)
	if len(people) > 0 {
		return people, true
	}
	return calendarPeopleFromDescription(event.Description)
}

func calendarNotificationCreatorPeople(event calendarEvent) []string {
	return calendarParticipantNames([]calendarParticipant{calendarEventCreatorParticipant(event)})
}
