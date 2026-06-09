package admind

import (
	"log"
	"net/http"
	"net/url"
)

func (service *Service) writeUserMemorySchedules(responseWriter http.ResponseWriter, request *http.Request) {
	actorEmail := service.memoryActorEmail(request)
	if actorEmail == "" {
		http.Error(responseWriter, "memory access required", http.StatusForbidden)
		return
	}
	personID, errorValue := service.resolveMemoryPersonID(request.Context(), actorEmail)
	if errorValue != nil {
		log.Printf("memory schedules identity resolution failed: %v", errorValue)
		http.Error(responseWriter, "memory identity unavailable", http.StatusBadGateway)
		return
	}
	if personID == "" {
		http.Error(responseWriter, "memory person not found", http.StatusNotFound)
		return
	}

	var schedules map[string]any
	path := "/admin/api/task-schedules?" + memorySchedulesQuery(personID)
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodGet, path, nil, &schedules); errorValue != nil {
		log.Printf("memory schedules upstream failed: %v", errorValue)
		http.Error(responseWriter, "memory schedules unavailable", http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, schedules)
}

func memorySchedulesQuery(personID string) string {
	query := url.Values{}
	query.Set("creatorPersonID", personID)
	query.Set("limit", "50")
	return query.Encode()
}
