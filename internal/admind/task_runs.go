package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	case "/runs/api/retry":
		service.proxyScopedTaskRetry(responseWriter, request, viewerEmail, isViewerAdmin)
	case "/runs/api/llm-call":
		service.proxyAdminOnlyRunsRead(responseWriter, request, isViewerAdmin, "/admin/api/run/llm-call", "id")
	case "/runs/api/turn-input":
		service.proxyAdminOnlyRunsRead(responseWriter, request, isViewerAdmin, "/admin/api/run/turn-input", "id")
	case "/runs/api/inbound":
		service.proxyAdminOnlyRunsRead(responseWriter, request, isViewerAdmin, "/admin/api/connector/events", "conversationID", "messageID", "limit")
	default:
		if request.Method == http.MethodDelete && strings.HasPrefix(request.URL.Path, "/runs/api/") {
			service.proxyScopedTaskDelete(responseWriter, request, viewerEmail, isViewerAdmin)
			return
		}
		http.NotFound(responseWriter, request)
	}
}

type taskRetryRequest struct {
	TaskRunID string `json:"taskRunID"`
}

type trustedTaskRetryRequest struct {
	TaskRunID     string `json:"taskRunID"`
	ViewerEmail   string `json:"viewerEmail"`
	ViewerIsAdmin bool   `json:"viewerIsAdmin"`
}

func (service *Service) proxyScopedTaskRetry(responseWriter http.ResponseWriter, request *http.Request, viewerEmail string, isViewerAdmin bool) {
	if request.Method != http.MethodPost {
		http.NotFound(responseWriter, request)
		return
	}
	var retryRequest taskRetryRequest
	if json.NewDecoder(request.Body).Decode(&retryRequest) != nil {
		http.Error(responseWriter, "invalid task retry request", http.StatusBadRequest)
		return
	}
	taskRunID := strings.TrimSpace(retryRequest.TaskRunID)
	if taskRunID == "" {
		http.Error(responseWriter, "taskRunID is required", http.StatusBadRequest)
		return
	}
	blueclawRequest := trustedTaskRetryRequest{
		TaskRunID:     taskRunID,
		ViewerEmail:   viewerEmail,
		ViewerIsAdmin: isViewerAdmin,
	}
	status, responseBody, contentType, errorValue := service.blueclawTaskRetryRequest(request.Context(), blueclawRequest)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusServiceUnavailable)
		return
	}
	if contentType != "" {
		responseWriter.Header().Set("Content-Type", contentType)
	}
	responseWriter.WriteHeader(status)
	_, _ = responseWriter.Write(responseBody)
}

func (service *Service) blueclawTaskRetryRequest(ctx context.Context, body trustedTaskRetryRequest) (int, []byte, string, error) {
	document, errorValue := json.Marshal(body)
	if errorValue != nil {
		return 0, nil, "", errorValue
	}
	requestURL := strings.TrimRight(service.Configuration.BlueclawBaseURL, "/") + "/admin/api/run/retry"
	blueclawRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, strings.NewReader(string(document)))
	if errorValue != nil {
		return 0, nil, "", errorValue
	}
	blueclawRequest.Header.Set("Content-Type", "application/json")
	response, errorValue := service.httpClient().Do(blueclawRequest)
	if errorValue != nil {
		return 0, nil, "", fmt.Errorf("Blueclaw retry unavailable: %w", errorValue)
	}
	defer response.Body.Close()
	responseBody, errorValue := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if errorValue != nil {
		return 0, nil, "", fmt.Errorf("reading Blueclaw retry response: %w", errorValue)
	}
	return response.StatusCode, responseBody, response.Header.Get("Content-Type"), nil
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
	inlineLLMFailureEvidence(service.Configuration.BlueclawWorkspacePath, detail, isViewerAdmin)
	service.writeJSON(responseWriter, detail)
}

func (service *Service) proxyAdminOnlyRunsRead(responseWriter http.ResponseWriter, request *http.Request, isViewerAdmin bool, blueclawPath string, forwardedNames ...string) {
	if request.Method != http.MethodGet {
		http.NotFound(responseWriter, request)
		return
	}
	if !isViewerAdmin {
		http.Error(responseWriter, "only an admin can read what the agent was sent", http.StatusForbidden)
		return
	}
	query := url.Values{}
	for _, name := range forwardedNames {
		if value := strings.TrimSpace(request.URL.Query().Get(name)); value != "" {
			query.Set(name, value)
		}
	}
	var answer any
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodGet, blueclawPath+"?"+query.Encode(), nil, &answer); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, answer)
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
