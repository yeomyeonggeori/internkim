package admind

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type memoryScheduleCancelRequest struct {
	TaskScheduleID string `json:"taskScheduleID"`
}

type memoryScheduleCancelBlueclawRequest struct {
	TaskScheduleID  string `json:"taskScheduleID"`
	CreatorPersonID string `json:"creatorPersonID"`
}

type memoryScheduleDeleteRequest struct {
	TaskScheduleID string `json:"taskScheduleID"`
}

type memoryScheduleDeleteBlueclawRequest struct {
	TaskScheduleID  string `json:"taskScheduleID"`
	CreatorPersonID string `json:"creatorPersonID"`
}

type memoryScheduleUpdateRequest struct {
	TaskScheduleID string  `json:"taskScheduleID"`
	Name           *string `json:"name"`
	Kind           *string `json:"kind"`
	RunAt          *string `json:"runAt"`
	IntervalSecond *int    `json:"intervalSecond"`
	CronExpression *string `json:"cronExpression"`
	TimeZone       *string `json:"timeZone"`
	ExpiresAt      *string `json:"expiresAt"`
	MaxRunCount    *int    `json:"maxRunCount"`
	RepeatPolicy   *string `json:"repeatPolicy"`
}

type memoryScheduleUpdateBlueclawRequest struct {
	TaskScheduleID  string  `json:"taskScheduleID"`
	CreatorPersonID string  `json:"creatorPersonID"`
	Name            *string `json:"name,omitempty"`
	Kind            *string `json:"kind,omitempty"`
	RunAt           *string `json:"runAt,omitempty"`
	IntervalSecond  *int    `json:"intervalSecond,omitempty"`
	CronExpression  *string `json:"cronExpression,omitempty"`
	TimeZone        *string `json:"timeZone,omitempty"`
	ExpiresAt       *string `json:"expiresAt,omitempty"`
	MaxRunCount     *int    `json:"maxRunCount,omitempty"`
	RepeatPolicy    *string `json:"repeatPolicy,omitempty"`
}

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
	path := "/admin/api/schedule?" + memorySchedulesQuery(request.URL.Query(), personID)
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodGet, path, nil, &schedules); errorValue != nil {
		log.Printf("memory schedules upstream failed: %v", errorValue)
		http.Error(responseWriter, "memory schedules unavailable", http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, schedules)
}

func (service *Service) cancelUserMemorySchedule(responseWriter http.ResponseWriter, request *http.Request) {
	var cancelRequest memoryScheduleCancelRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&cancelRequest); errorValue != nil {
		log.Printf("memory schedule cancel request decode failed: %v", errorValue)
		http.Error(responseWriter, "invalid memory schedule request", http.StatusBadRequest)
		return
	}
	personID, ok := service.writeMemorySchedulePersonID(responseWriter, request)
	if !ok {
		return
	}
	blueclawRequest := memoryScheduleCancelBlueclawRequest{
		TaskScheduleID:  cancelRequest.TaskScheduleID,
		CreatorPersonID: personID,
	}
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodPost, "/admin/api/schedule/cancel", blueclawRequest, nil); errorValue != nil {
		log.Printf("memory schedule cancel upstream failed: %v", errorValue)
		http.Error(responseWriter, "memory schedule cancel unavailable", http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]bool{"ok": true})
}

func (service *Service) deleteUserMemorySchedule(responseWriter http.ResponseWriter, request *http.Request) {
	var deleteRequest memoryScheduleDeleteRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&deleteRequest); errorValue != nil {
		log.Printf("memory schedule delete request decode failed: %v", errorValue)
		http.Error(responseWriter, "invalid memory schedule request", http.StatusBadRequest)
		return
	}
	personID, ok := service.writeMemorySchedulePersonID(responseWriter, request)
	if !ok {
		return
	}
	blueclawRequest := memoryScheduleDeleteBlueclawRequest{
		TaskScheduleID:  deleteRequest.TaskScheduleID,
		CreatorPersonID: personID,
	}
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodPost, "/admin/api/schedule/delete", blueclawRequest, nil); errorValue != nil {
		log.Printf("memory schedule delete upstream failed: %v", errorValue)
		http.Error(responseWriter, "memory schedule delete unavailable", http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]bool{"ok": true})
}

func (service *Service) updateUserMemorySchedule(responseWriter http.ResponseWriter, request *http.Request) {
	var updateRequest memoryScheduleUpdateRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&updateRequest); errorValue != nil {
		log.Printf("memory schedule update request decode failed: %v", errorValue)
		http.Error(responseWriter, "invalid memory schedule request", http.StatusBadRequest)
		return
	}
	personID, ok := service.writeMemorySchedulePersonID(responseWriter, request)
	if !ok {
		return
	}
	blueclawRequest := memoryScheduleUpdateBlueclawRequest{
		TaskScheduleID:  updateRequest.TaskScheduleID,
		CreatorPersonID: personID,
		Name:            updateRequest.Name,
		Kind:            updateRequest.Kind,
		RunAt:           updateRequest.RunAt,
		IntervalSecond:  updateRequest.IntervalSecond,
		CronExpression:  updateRequest.CronExpression,
		TimeZone:        updateRequest.TimeZone,
		ExpiresAt:       updateRequest.ExpiresAt,
		MaxRunCount:     updateRequest.MaxRunCount,
		RepeatPolicy:    updateRequest.RepeatPolicy,
	}
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodPost, "/admin/api/schedule/update", blueclawRequest, nil); errorValue != nil {
		log.Printf("memory schedule update upstream failed: %v", errorValue)
		http.Error(responseWriter, "memory schedule update unavailable", http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]bool{"ok": true})
}

func (service *Service) writeMemorySchedulePersonID(responseWriter http.ResponseWriter, request *http.Request) (string, bool) {
	actorEmail := service.memoryActorEmail(request)
	if actorEmail == "" {
		http.Error(responseWriter, "memory access required", http.StatusForbidden)
		return "", false
	}
	personID, errorValue := service.resolveMemoryPersonID(request.Context(), actorEmail)
	if errorValue != nil {
		log.Printf("memory schedules identity resolution failed: %v", errorValue)
		http.Error(responseWriter, "memory identity unavailable", http.StatusBadGateway)
		return "", false
	}
	if personID == "" {
		http.Error(responseWriter, "memory person not found", http.StatusNotFound)
		return "", false
	}
	return personID, true
}

func memorySchedulesQuery(values url.Values, personID string) string {
	query := url.Values{}
	query.Set("creatorPersonID", personID)
	query.Set("includeExpired", memorySchedulesIncludeExpired(values.Get("includeExpired")))
	query.Set("page", strconv.Itoa(memorySchedulesPage(values.Get("page"))))
	query.Set("pageSize", strconv.Itoa(memorySchedulesPageSize(values.Get("pageSize"))))
	return query.Encode()
}

func memorySchedulesIncludeExpired(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "false") {
		return "false"
	}
	return "true"
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
