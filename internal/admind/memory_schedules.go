package admind

import (
	"log"
	"net/http"
	"net/url"
	"strconv"
)

const (
	memorySchedulesDefaultPage     = 1
	memorySchedulesDefaultPageSize = 25
	memorySchedulesMaxPageSize     = 100
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
	path := "/admin/api/task-schedules?" + memorySchedulesQuery(request.URL.Query(), personID)
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodGet, path, nil, &schedules); errorValue != nil {
		log.Printf("memory schedules upstream failed: %v", errorValue)
		http.Error(responseWriter, "memory schedules unavailable", http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, schedules)
}

func memorySchedulesQuery(values url.Values, personID string) string {
	query := url.Values{}
	query.Set("creatorPersonID", personID)
	query.Set("includeExpired", "true")
	query.Set("page", strconv.Itoa(memorySchedulesPage(values.Get("page"))))
	query.Set("pageSize", strconv.Itoa(memorySchedulesPageSize(values.Get("pageSize"))))
	return query.Encode()
}

func memorySchedulesPage(value string) int {
	page, errorValue := strconv.Atoi(value)
	if errorValue != nil || page < 1 {
		return memorySchedulesDefaultPage
	}
	return page
}

func memorySchedulesPageSize(value string) int {
	pageSize, errorValue := strconv.Atoi(value)
	if errorValue != nil || pageSize < 1 {
		return memorySchedulesDefaultPageSize
	}
	if pageSize > memorySchedulesMaxPageSize {
		return memorySchedulesMaxPageSize
	}
	return pageSize
}
