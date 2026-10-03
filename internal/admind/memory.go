package admind

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

type memoryPolicyDocument struct {
	People []memoryPolicyPerson `json:"people"`
}

type memoryPolicyPerson struct {
	PersonID string   `json:"personID"`
	Emails   []string `json:"emails"`
}

func (service *Service) handleMemory(responseWriter http.ResponseWriter, request *http.Request) {
	path := strings.TrimPrefix(request.URL.Path, "/memory/api")
	if request.Method == http.MethodGet && path == "/facts" {
		service.writeUserMemoryFacts(responseWriter, request)
		return
	}
	if request.Method == http.MethodPost && path == "/facts/forget" {
		service.writeUserMemoryMutation(responseWriter, request, "/admin/api/memory/facts/forget")
		return
	}
	if request.Method == http.MethodGet && path == "/schedules" {
		service.writeUserMemorySchedules(responseWriter, request)
		return
	}
	if request.Method == http.MethodPost && path == "/schedules/tool-list" {
		service.writeUserScheduleToolList(responseWriter, request)
		return
	}
	if request.Method == http.MethodPost && path == "/schedules/tool-create" {
		service.writeUserScheduleToolCreate(responseWriter, request)
		return
	}
	if request.Method == http.MethodPost && path == "/schedules/tool-update" {
		service.writeUserScheduleToolUpdate(responseWriter, request)
		return
	}
	if request.Method == http.MethodPost && path == "/schedules/tool-cancel" {
		service.writeUserScheduleToolCancel(responseWriter, request)
		return
	}
	if request.Method == http.MethodPost && path == "/schedules/cancel" {
		service.cancelUserMemorySchedule(responseWriter, request)
		return
	}
	if request.Method == http.MethodPost && path == "/schedules/delete" {
		service.deleteUserMemorySchedule(responseWriter, request)
		return
	}
	if request.Method == http.MethodPost && path == "/schedules/update" {
		service.updateUserMemorySchedule(responseWriter, request)
		return
	}
	http.NotFound(responseWriter, request)
}

func (service *Service) writeUserMemoryFacts(responseWriter http.ResponseWriter, request *http.Request) {
	actorEmail := service.memoryActorEmail(request)
	if actorEmail == "" {
		http.Error(responseWriter, "memory access required", http.StatusForbidden)
		return
	}
	personID, errorValue := service.resolveMemoryPersonID(request.Context(), actorEmail)
	if errorValue != nil {
		log.Printf("memory facts identity resolution failed: %v", errorValue)
		http.Error(responseWriter, "memory identity unavailable", http.StatusBadGateway)
		return
	}
	if personID == "" {
		http.Error(responseWriter, "memory person not found", http.StatusNotFound)
		return
	}

	var facts map[string]any
	path := "/admin/api/memory/facts?" + memoryFactsQuery(request, personID)
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodGet, path, nil, &facts); errorValue != nil {
		log.Printf("memory facts upstream failed: %v", errorValue)
		http.Error(responseWriter, "memory facts unavailable", http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, facts)
}

func (service *Service) writeUserMemoryMutation(responseWriter http.ResponseWriter, request *http.Request, upstreamPath string) {
	actorEmail := service.memoryActorEmail(request)
	if actorEmail == "" {
		http.Error(responseWriter, "memory access required", http.StatusForbidden)
		return
	}
	personID, errorValue := service.resolveMemoryPersonID(request.Context(), actorEmail)
	if errorValue != nil {
		log.Printf("memory mutation identity resolution failed: %v", errorValue)
		http.Error(responseWriter, "memory identity unavailable", http.StatusBadGateway)
		return
	}
	if personID == "" {
		http.Error(responseWriter, "memory person not found", http.StatusNotFound)
		return
	}

	body := map[string]any{}
	if errorValue := json.NewDecoder(request.Body).Decode(&body); errorValue != nil && errorValue != io.EOF {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	body["readerPersonID"] = personID

	var response map[string]any
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodPost, upstreamPath, body, &response); errorValue != nil {
		log.Printf("memory mutation upstream failed: %v", errorValue)
		http.Error(responseWriter, "memory mutation unavailable", http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, response)
}

func (service *Service) memoryActorEmail(request *http.Request) string {
	if actorEmail := service.webMemberActorEmail(request); actorEmail != "" {
		return actorEmail
	}
	return assertedRequesterEmail(request)
}

func memoryFactsQuery(request *http.Request, personID string) string {
	query := url.Values{}
	if limit := strings.TrimSpace(request.URL.Query().Get("limit")); limit != "" {
		query.Set("limit", limit)
	}
	query.Set("readerPersonID", personID)
	return query.Encode()
}

func (service *Service) resolveMemoryPersonID(ctx context.Context, actorEmail string) (string, error) {
	actor, found, errorValue := service.resolveUserActorByEmail(ctx, actorEmail)
	if errorValue != nil || !found {
		return "", errorValue
	}
	return actor.UserID, nil
}
