package admind

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type memoryPolicyDocument struct {
	People []memoryPolicyPerson `json:"people"`
}

type memoryPolicyPerson struct {
	PersonID string   `json:"personID"`
	Emails   []string `json:"emails"`
}

func (service *Service) serveMemoryPage(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/memory" {
		http.Redirect(responseWriter, request, "/memory/", http.StatusFound)
		return
	}
	if service.serveMemoryStaticFile(responseWriter, request) {
		return
	}
	http.ServeFile(responseWriter, request, filepath.Join(service.Configuration.AdminUIPath, "index.html"))
}

func (service *Service) serveMemoryStaticFile(responseWriter http.ResponseWriter, request *http.Request) bool {
	relativePath := strings.TrimPrefix(request.URL.Path, "/memory/")
	if relativePath == "" {
		return false
	}
	filePath := filepath.Join(service.Configuration.AdminUIPath, "memory", relativePath)
	fileInfo, errorValue := os.Stat(filePath)
	if errorValue != nil || fileInfo.IsDir() {
		return false
	}
	http.ServeFile(responseWriter, request, filePath)
	return true
}

func (service *Service) handleMemory(responseWriter http.ResponseWriter, request *http.Request) {
	path := strings.TrimPrefix(request.URL.Path, "/memory/api")
	if request.Method == http.MethodGet && path == "/graph" {
		service.writeUserMemoryGraph(responseWriter, request)
		return
	}
	http.NotFound(responseWriter, request)
}

func (service *Service) writeUserMemoryGraph(responseWriter http.ResponseWriter, request *http.Request) {
	actorEmail := service.memoryActorEmail(request)
	if actorEmail == "" {
		http.Error(responseWriter, "memory access required", http.StatusForbidden)
		return
	}
	personID, errorValue := service.resolveMemoryPersonID(request.Context(), actorEmail)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if personID == "" {
		http.Error(responseWriter, "memory person not found", http.StatusNotFound)
		return
	}

	var graph map[string]any
	path := "/admin/api/memory/graph?" + memoryGraphQuery(request, personID)
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodGet, path, nil, &graph); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, graph)
}

func (service *Service) memoryActorEmail(request *http.Request) string {
	if actorEmail := service.webStaffActorEmail(request); actorEmail != "" {
		return actorEmail
	}
	if !isLocalRequest(request) {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(request.Header.Get(flowRequesterEmailHeader)))
}

func memoryGraphQuery(request *http.Request, personID string) string {
	query := request.URL.Query()
	query.Set("readerPersonID", personID)
	return query.Encode()
}

func (service *Service) resolveMemoryPersonID(ctx context.Context, actorEmail string) (string, error) {
	if service.hasDeviceAuth() {
		personID, errorValue := service.resolveMemoryPersonIDFromUserRecords(ctx, actorEmail)
		if errorValue != nil {
			return "", errorValue
		}
		if personID != "" {
			return personID, nil
		}
	}
	return service.resolveMemoryPersonIDFromPolicy(ctx, actorEmail)
}

func (service *Service) resolveMemoryPersonIDFromUserRecords(ctx context.Context, actorEmail string) (string, error) {
	fleetID := strings.ToLower(strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)))
	fleetSecret := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetSecretPath))
	records, errorValue := service.lookupUserRecords(ctx, fleetID, fleetSecret)
	if errorValue != nil {
		return "", errorValue
	}
	for _, record := range records {
		if strings.EqualFold(record.Email, actorEmail) && isActiveFlowUser(record) {
			return strings.TrimSpace(record.UserID), nil
		}
	}
	return "", nil
}

func (service *Service) resolveMemoryPersonIDFromPolicy(ctx context.Context, actorEmail string) (string, error) {
	var policyDocument memoryPolicyDocument
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		return "", errorValue
	}
	for _, person := range policyDocument.People {
		if personHasMemoryEmail(person, actorEmail) {
			return strings.TrimSpace(person.PersonID), nil
		}
	}
	return "", nil
}

func personHasMemoryEmail(person memoryPolicyPerson, actorEmail string) bool {
	for _, email := range person.Emails {
		if strings.EqualFold(strings.TrimSpace(email), strings.TrimSpace(actorEmail)) {
			return true
		}
	}
	return false
}
