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
	personID, errorValue := service.resolveMemoryPersonIDFromPolicy(request.Context(), actorEmail)
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
	path := "/admin/api/task-schedules"
	if query := memorySchedulesQuery(request.URL.Query()); query != "" {
		path += "?" + query
	}
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodGet, path, nil, &schedules); errorValue != nil {
		log.Printf("memory schedules upstream failed: %v", errorValue)
		http.Error(responseWriter, "memory schedules unavailable", http.StatusBadGateway)
		return
	}
	schedules["currentPersonID"] = personID
	service.writeJSON(responseWriter, schedules)
}

func memorySchedulesQuery(values url.Values) string {
	query := url.Values{}
	if page := values.Get("page"); page != "" {
		query.Set("page", page)
	}
	if pageSize := values.Get("pageSize"); pageSize != "" {
		query.Set("pageSize", pageSize)
	}
	return query.Encode()
}
