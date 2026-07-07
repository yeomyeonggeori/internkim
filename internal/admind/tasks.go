package admind

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (service *Service) serveTasksPage(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/tasks" {
		http.Redirect(responseWriter, request, "/tasks/", http.StatusFound)
		return
	}
	if service.serveTasksStaticFile(responseWriter, request) {
		return
	}
	service.serveTasksIndex(responseWriter, request)
}

func (service *Service) serveTasksStaticFile(responseWriter http.ResponseWriter, request *http.Request) bool {
	relativePath := strings.TrimPrefix(request.URL.Path, "/tasks/")
	if relativePath == "" {
		return false
	}
	filePath := filepath.Join(service.Configuration.AdminUIPath, "tasks", relativePath)
	fileInformation, errorValue := os.Stat(filePath)
	if errorValue != nil || fileInformation.IsDir() {
		return false
	}
	http.ServeFile(responseWriter, request, filePath)
	return true
}

func (service *Service) serveTasksIndex(responseWriter http.ResponseWriter, request *http.Request) {
	tasksIndexPath := filepath.Join(service.Configuration.AdminUIPath, "tasks", "index.html")
	if fileInformation, errorValue := os.Stat(tasksIndexPath); errorValue == nil && !fileInformation.IsDir() {
		http.ServeFile(responseWriter, request, tasksIndexPath)
		return
	}
	http.ServeFile(responseWriter, request, filepath.Join(service.Configuration.AdminUIPath, "index.html"))
}

func (service *Service) serveProofOfConceptAdminPage(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/poc-admin" {
		http.Redirect(responseWriter, request, "/poc-admin/", http.StatusFound)
		return
	}
	relativePath := strings.TrimPrefix(request.URL.Path, "/poc-admin/")
	if relativePath != "" {
		filePath := filepath.Join(service.Configuration.AdminUIPath, "poc-admin", relativePath)
		fileInformation, errorValue := os.Stat(filePath)
		if errorValue == nil && !fileInformation.IsDir() {
			http.ServeFile(responseWriter, request, filePath)
			return
		}
	}
	proofOfConceptAdminIndexPath := filepath.Join(service.Configuration.AdminUIPath, "poc-admin", "index.html")
	if fileInformation, errorValue := os.Stat(proofOfConceptAdminIndexPath); errorValue == nil && !fileInformation.IsDir() {
		http.ServeFile(responseWriter, request, proofOfConceptAdminIndexPath)
		return
	}
	service.serveTasksIndex(responseWriter, request)
}

func (service *Service) handleTasks(responseWriter http.ResponseWriter, request *http.Request) {
	viewerEmail := service.webTaskRunActorEmail(request)
	if viewerEmail == "" {
		if service.webActorEmail(request) != "" {
			http.Error(responseWriter, "task run access requires PoC super admin", http.StatusForbidden)
			return
		}
		http.Error(responseWriter, "authentication required", http.StatusUnauthorized)
		return
	}
	isViewerAdmin := service.canManageTaskRuns(request.Context(), viewerEmail)
	switch request.URL.Path {
	case "/tasks/api/runs":
		service.proxyScopedTaskList(responseWriter, request, viewerEmail, isViewerAdmin)
	case "/tasks/api/run-detail":
		service.proxyScopedTaskDetail(responseWriter, request, viewerEmail, isViewerAdmin)
	default:
		if request.Method == http.MethodDelete && strings.HasPrefix(request.URL.Path, "/tasks/api/runs/") {
			service.proxyScopedTaskDelete(responseWriter, request, viewerEmail, isViewerAdmin)
			return
		}
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) proxyScopedTaskList(responseWriter http.ResponseWriter, request *http.Request, viewerEmail string, isViewerAdmin bool) {
	if request.Method != http.MethodGet {
		http.NotFound(responseWriter, request)
		return
	}
	query := url.Values{}
	if status := strings.TrimSpace(request.URL.Query().Get("status")); status != "" {
		query.Set("status", status)
	}
	if limit := strings.TrimSpace(request.URL.Query().Get("limit")); limit != "" {
		query.Set("limit", limit)
	}
	if offset := strings.TrimSpace(request.URL.Query().Get("offset")); offset != "" {
		query.Set("offset", offset)
	}
	if includeTotal := strings.TrimSpace(request.URL.Query().Get("includeTotal")); includeTotal != "" {
		query.Set("includeTotal", includeTotal)
	}
	if includeCost := strings.TrimSpace(request.URL.Query().Get("includeCost")); includeCost != "" {
		query.Set("includeCost", includeCost)
	}
	if dailyCostTaskRunLimit := strings.TrimSpace(request.URL.Query().Get("dailyCostTaskRunLimit")); dailyCostTaskRunLimit != "" {
		query.Set("dailyCostTaskRunLimit", dailyCostTaskRunLimit)
	}
	query.Set("viewerEmail", viewerEmail)
	query.Set("viewerIsAdmin", strconv.FormatBool(isViewerAdmin))
	var taskRunResponse any
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodGet, "/admin/api/task?"+query.Encode(), nil, &taskRunResponse); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, taskRunResponse)
}

func (service *Service) proxyScopedTaskDetail(responseWriter http.ResponseWriter, request *http.Request, viewerEmail string, isViewerAdmin bool) {
	if request.Method != http.MethodGet {
		http.NotFound(responseWriter, request)
		return
	}
	taskRunID := strings.TrimSpace(request.URL.Query().Get("taskRunID"))
	if taskRunID == "" {
		http.Error(responseWriter, "taskRunID is required", http.StatusBadRequest)
		return
	}
	query := url.Values{}
	query.Set("taskRunID", taskRunID)
	query.Set("viewerEmail", viewerEmail)
	query.Set("viewerIsAdmin", strconv.FormatBool(isViewerAdmin))
	var detail map[string]any
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodGet, "/admin/api/task/detail?"+query.Encode(), nil, &detail); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, detail)
}

func (service *Service) proxyScopedTaskDelete(responseWriter http.ResponseWriter, request *http.Request, viewerEmail string, isViewerAdmin bool) {
	taskRunID := strings.TrimSpace(strings.TrimPrefix(request.URL.Path, "/tasks/api/runs/"))
	if taskRunID == "" {
		http.Error(responseWriter, "taskRunID is required", http.StatusBadRequest)
		return
	}
	blueclawRequest := map[string]any{
		"taskRunID":     taskRunID,
		"viewerEmail":   viewerEmail,
		"viewerIsAdmin": isViewerAdmin,
	}
	var deleteResponse any
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodPost, "/admin/api/task/delete", blueclawRequest, &deleteResponse); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if deleteResponse == nil {
		deleteResponse = map[string]any{
			"status":    "deleted",
			"taskRunID": taskRunID,
		}
	}
	service.writeJSON(responseWriter, deleteResponse)
}
