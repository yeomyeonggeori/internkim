package admind

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func (service *Service) handleTasks(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.NotFound(responseWriter, request)
		return
	}
	viewerEmail := service.webStaffActorEmail(request)
	if viewerEmail == "" {
		http.Error(responseWriter, "authentication required", http.StatusUnauthorized)
		return
	}
	isViewerAdmin := service.isFlowAdminEmail(request.Context(), viewerEmail)
	switch request.URL.Path {
	case "/tasks/api/runs":
		service.proxyScopedTaskList(responseWriter, request, viewerEmail, isViewerAdmin)
	case "/tasks/api/run-detail":
		service.proxyScopedTaskDetail(responseWriter, request, viewerEmail, isViewerAdmin)
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) proxyScopedTaskList(responseWriter http.ResponseWriter, request *http.Request, viewerEmail string, isViewerAdmin bool) {
	query := url.Values{}
	if status := strings.TrimSpace(request.URL.Query().Get("status")); status != "" {
		query.Set("status", status)
	}
	if limit := strings.TrimSpace(request.URL.Query().Get("limit")); limit != "" {
		query.Set("limit", limit)
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
