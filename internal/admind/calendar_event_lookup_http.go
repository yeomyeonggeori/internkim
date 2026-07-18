package admind

import (
	"net/http"
	"net/url"
	"strings"
)

func calendarEventIDFromAPIPath(path string) (string, error) {
	return url.PathUnescape(strings.TrimPrefix(path, "/events/"))
}

func (service *Service) getCalendarEventFromAPIPath(responseWriter http.ResponseWriter, request *http.Request, path string) {
	eventID, errorValue := calendarEventIDFromAPIPath(path)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	service.getCalendarEvent(responseWriter, request, eventID)
}

func (service *Service) updateCalendarEventFromAPIPath(responseWriter http.ResponseWriter, request *http.Request, path string) {
	eventID, errorValue := calendarEventIDFromAPIPath(path)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	service.updateCalendarEvent(responseWriter, request, eventID)
}

func (service *Service) deleteCalendarEventFromAPIPath(responseWriter http.ResponseWriter, request *http.Request, path string) {
	eventID, errorValue := calendarEventIDFromAPIPath(path)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	service.deleteCalendarEvent(responseWriter, request, eventID)
}

func (service *Service) getCalendarEvent(responseWriter http.ResponseWriter, request *http.Request, eventID string) {
	event, found, errorValue := service.readCalendarEventByID(request.Context(), strings.TrimSpace(eventID))
	if errorValue != nil {
		writeCalendarMutationInternalError(responseWriter, request, eventID, errorValue)
		return
	}
	if !found {
		http.NotFound(responseWriter, request)
		return
	}
	event = service.calendarEventWithParticipantImages(request, event)
	service.writeJSON(responseWriter, service.calendarEventWithActorProfiles(request.Context(), event))
}

func (service *Service) searchCalendarEventCandidates(responseWriter http.ResponseWriter, request *http.Request) {
	query := strings.TrimSpace(request.URL.Query().Get("query"))
	if query == "" {
		http.Error(responseWriter, "calendar event search query is required", http.StatusBadRequest)
		return
	}
	events, errorValue := service.searchCalendarEvents(request.Context(), query)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, calendarEventsResponse{Events: events})
}
