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
	if !service.belongsToACompany() {
		writeCalendarBelongsToTheCompany(responseWriter)
		return
	}
	event, held := service.centralCalendarEventByID(request, eventID)
	if !held {
		http.NotFound(responseWriter, request)
		return
	}
	service.writeJSON(responseWriter, calendarEventWithNormalizedParticipants(event))
}
