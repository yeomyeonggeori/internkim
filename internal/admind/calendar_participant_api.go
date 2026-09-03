package admind

import (
	"net/http"
)

type calendarParticipantsResponse struct {
	Participants []calendarParticipant `json:"participants"`
}

func (service *Service) listCalendarParticipants(responseWriter http.ResponseWriter, request *http.Request) {
	members := service.taskMembers(request)
	service.writeJSON(responseWriter, calendarParticipantsResponse{
		Participants: calendarParticipantsFromMembers(members),
	})
}
