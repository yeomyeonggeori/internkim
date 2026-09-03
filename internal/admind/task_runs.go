package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (service *Service) serveTaskRunsPage(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/runs" {
		http.Redirect(responseWriter, request, "/runs/", http.StatusFound)
		return
	}
	if service.serveTasksStaticFile(responseWriter, request) {
		return
	}
	service.serveTasksIndex(responseWriter, request)
}

func (service *Service) serveTasksStaticFile(responseWriter http.ResponseWriter, request *http.Request) bool {
	relativePath := strings.TrimPrefix(request.URL.Path, "/runs/")
	if relativePath == "" {
		return false
	}
	filePath := filepath.Join(service.Configuration.AdminUIPath, "runs", relativePath)
	fileInformation, errorValue := os.Stat(filePath)
	if errorValue != nil || fileInformation.IsDir() {
		return false
	}
	http.ServeFile(responseWriter, request, filePath)
	return true
}

func (service *Service) serveTasksIndex(responseWriter http.ResponseWriter, request *http.Request) {
	runsIndexPath := filepath.Join(service.Configuration.AdminUIPath, "runs", "index.html")
	if fileInformation, errorValue := os.Stat(runsIndexPath); errorValue == nil && !fileInformation.IsDir() {
		http.ServeFile(responseWriter, request, runsIndexPath)
		return
	}
	http.ServeFile(responseWriter, request, filepath.Join(service.Configuration.AdminUIPath, "index.html"))
}

func (service *Service) handleTaskRuns(responseWriter http.ResponseWriter, request *http.Request) {
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
	case "/runs/api", "/runs/api/":
		service.proxyScopedTaskList(responseWriter, request, viewerEmail, isViewerAdmin)
	case "/runs/api/detail":
		service.proxyScopedTaskDetail(responseWriter, request, viewerEmail, isViewerAdmin)
	case "/runs/api/approve":
		service.proxyScopedTaskApproval(responseWriter, request, viewerEmail, isViewerAdmin)
	default:
		if request.Method == http.MethodDelete && strings.HasPrefix(request.URL.Path, "/runs/api/") {
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
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodGet, "/admin/api/run?"+query.Encode(), nil, &taskRunResponse); errorValue != nil {
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
	var detail map[string]any
	path := scopedTaskDetailPath(taskRunID, viewerEmail, isViewerAdmin)
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodGet, path, nil, &detail); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, detail)
}

func scopedTaskDetailPath(taskRunID string, viewerEmail string, isViewerAdmin bool) string {
	query := url.Values{}
	query.Set("taskRunID", taskRunID)
	query.Set("viewerEmail", viewerEmail)
	query.Set("viewerIsAdmin", strconv.FormatBool(isViewerAdmin))
	return "/admin/api/run/detail?" + query.Encode()
}

type taskApprovalRequest struct {
	TaskRunID string `json:"taskRunID"`
	Decision  string `json:"decision"`
}

func (service *Service) proxyScopedTaskApproval(responseWriter http.ResponseWriter, request *http.Request, viewerEmail string, isViewerAdmin bool) {
	if request.Method != http.MethodPost {
		http.NotFound(responseWriter, request)
		return
	}
	var approvalRequest taskApprovalRequest
	if json.NewDecoder(request.Body).Decode(&approvalRequest) != nil {
		http.Error(responseWriter, "invalid task approval request", http.StatusBadRequest)
		return
	}
	taskRunID := strings.TrimSpace(approvalRequest.TaskRunID)
	if taskRunID == "" {
		http.Error(responseWriter, "taskRunID is required", http.StatusBadRequest)
		return
	}
	if !service.taskRunIsVisibleToViewer(request.Context(), taskRunID, viewerEmail, isViewerAdmin) {
		http.Error(responseWriter, "task run not found", http.StatusNotFound)
		return
	}
	blueclawRequest := taskApprovalRequest{TaskRunID: taskRunID, Decision: strings.TrimSpace(approvalRequest.Decision)}
	var approvalResponse any
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodPost, "/admin/api/run/approve", blueclawRequest, &approvalResponse); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, approvalResponse)
}

func (service *Service) taskRunIsVisibleToViewer(ctx context.Context, taskRunID string, viewerEmail string, isViewerAdmin bool) bool {
	var detail map[string]any
	path := scopedTaskDetailPath(taskRunID, viewerEmail, isViewerAdmin)
	return service.blueclawJSONRequest(ctx, http.MethodGet, path, nil, &detail) == nil
}

func (service *Service) proxyScopedTaskDelete(responseWriter http.ResponseWriter, request *http.Request, viewerEmail string, isViewerAdmin bool) {
	taskRunID := strings.TrimSpace(strings.TrimPrefix(request.URL.Path, "/runs/api/"))
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
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodPost, "/admin/api/run/delete", blueclawRequest, &deleteResponse); errorValue != nil {
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
