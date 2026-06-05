package admind

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

const attendanceOverrideActorLocalAdmin = "local-admin"

func (service *Service) writeAttendanceEventLocation(responseWriter http.ResponseWriter, request *http.Request, eventID string) {
	var payload attendanceEventLocationRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	location, found, errorValue := service.attendanceLocationByIDExact(payload.LocationID)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(responseWriter, "attendance location was not found", http.StatusBadRequest)
		return
	}
	database, errorValue := service.openAttendanceDatabase(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	defer database.Close()
	event, found, errorValue := service.attendanceEventByID(request.Context(), database, eventID)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(responseWriter, request)
		return
	}
	if event.Kind != attendanceKindClockIn {
		http.Error(responseWriter, "only clock-in attendance events can change location", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(event.CanceledAt) != "" {
		http.Error(responseWriter, "canceled attendance events cannot change location", http.StatusBadRequest)
		return
	}
	actorIdentifier := service.attendanceEventLocationOverrideActor(request)
	if !service.canOverrideAttendanceEventLocation(request, event, actorIdentifier) {
		http.Error(responseWriter, "attendance event owner or admin required", http.StatusForbidden)
		return
	}
	updatedEvent, errorValue := service.updateAttendanceEventLocation(request.Context(), database, event, location, actorIdentifier, time.Now().UTC())
	if errorValue != nil {
		if errors.Is(errorValue, errAttendanceEventLocationConflict) {
			http.Error(responseWriter, errorValue.Error(), http.StatusConflict)
			return
		}
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, updatedEvent)
}

func (service *Service) attendanceEventLocationOverrideActor(request *http.Request) string {
	if actorEmail := strings.ToLower(strings.TrimSpace(service.webStaffActorEmail(request))); actorEmail != "" {
		return actorEmail
	}
	if !isLocalRequest(request) {
		return ""
	}
	if actorEmail := strings.ToLower(strings.TrimSpace(request.Header.Get(flowRequesterEmailHeader))); actorEmail != "" {
		return actorEmail
	}
	return attendanceOverrideActorLocalAdmin
}

func (service *Service) canOverrideAttendanceEventLocation(request *http.Request, event attendanceEvent, actorIdentifier string) bool {
	if service.isAuthorized(request) {
		return true
	}
	return actorIdentifier != "" && strings.EqualFold(actorIdentifier, event.Email)
}
