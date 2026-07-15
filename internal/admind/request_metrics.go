package admind

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const adminRecentSlowRequestLimit = 50

type adminEndpointClass string

const (
	adminEndpointStatic   adminEndpointClass = "static"
	adminEndpointReadAPI  adminEndpointClass = "read_api"
	adminEndpointWriteAPI adminEndpointClass = "write_api"
	adminEndpointProxy    adminEndpointClass = "proxy"
	adminEndpointControl  adminEndpointClass = "control"
)

type adminRequestRecord struct {
	Method        string             `json:"method"`
	Path          string             `json:"path"`
	EndpointClass adminEndpointClass `json:"endpointClass"`
	Status        int                `json:"status"`
	DurationMs    int64              `json:"durationMs"`
	RecordedAt    time.Time          `json:"recordedAt"`
}

type adminRequestMetrics struct {
	mutex              sync.Mutex
	recentSlowRequests []adminRequestRecord
}

type adminStatusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func newAdminRequestMetrics() *adminRequestMetrics {
	return &adminRequestMetrics{}
}

func (service *Service) withRequestMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		startedAt := time.Now()
		statusRecorder := &adminStatusRecorder{ResponseWriter: responseWriter, status: http.StatusOK}
		next.ServeHTTP(statusRecorder, request)
		service.recordAdminRequest(request, statusRecorder.status, time.Since(startedAt), startedAt)
	})
}

func (service *Service) withReadAPITimeout(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if adminEndpointClassForRequest(request) != adminEndpointReadAPI {
			next.ServeHTTP(responseWriter, request)
			return
		}
		ctx, cancel := context.WithTimeout(request.Context(), 5*time.Second)
		defer cancel()
		next.ServeHTTP(responseWriter, request.WithContext(ctx))
	})
}

func (statusRecorder *adminStatusRecorder) WriteHeader(status int) {
	if statusRecorder.wroteHeader {
		return
	}
	statusRecorder.status = status
	statusRecorder.wroteHeader = true
	statusRecorder.ResponseWriter.WriteHeader(status)
}

func (statusRecorder *adminStatusRecorder) Unwrap() http.ResponseWriter {
	return statusRecorder.ResponseWriter
}

func (service *Service) writeAdminRequestDiagnostics(responseWriter http.ResponseWriter) {
	service.writeJSON(responseWriter, map[string]any{
		"recentSlowRequests": service.requestMetricState().RecentSlowRequests(),
	})
}

func (service *Service) proxyBlueclawConnectorEvents(responseWriter http.ResponseWriter, request *http.Request) {
	path := "/admin/api/connector/events"
	if strings.TrimSpace(request.URL.RawQuery) != "" {
		path += "?" + request.URL.RawQuery
	}
	var diagnostics []map[string]any
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodGet, path, nil, &diagnostics); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, diagnostics)
}

func (service *Service) proxyBlueclawTaskList(responseWriter http.ResponseWriter, request *http.Request) {
	path := "/admin/api/task"
	if strings.TrimSpace(request.URL.RawQuery) != "" {
		path += "?" + request.URL.RawQuery
	}
	var taskRunResponse any
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodGet, path, nil, &taskRunResponse); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, taskRunResponse)
}

func (service *Service) proxyBlueclawTaskDetail(responseWriter http.ResponseWriter, request *http.Request) {
	taskRunID := strings.TrimSpace(request.URL.Query().Get("taskRunID"))
	if taskRunID == "" {
		http.Error(responseWriter, "taskRunID is required", http.StatusBadRequest)
		return
	}
	var detail map[string]any
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodGet, "/admin/api/task/detail?taskRunID="+url.QueryEscape(taskRunID), nil, &detail); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, detail)
}

func (service *Service) recordAdminRequest(request *http.Request, status int, duration time.Duration, startedAt time.Time) {
	endpointClass := adminEndpointClassForRequest(request)
	record := adminRequestRecord{
		Method:        request.Method,
		Path:          sanitizedRequestPath(request.URL.Path),
		EndpointClass: endpointClass,
		Status:        status,
		DurationMs:    duration.Milliseconds(),
		RecordedAt:    startedAt.UTC(),
	}
	if shouldLogAdminRequest(record, duration) {
		log.Printf("admind request completed method=%s path=%s class=%s status=%d duration_ms=%d", record.Method, record.Path, record.EndpointClass, record.Status, record.DurationMs)
	}
	if shouldKeepSlowAdminRequest(record, duration) {
		service.requestMetricState().recordSlowRequest(record)
	}
}

func (service *Service) requestMetricState() *adminRequestMetrics {
	if service.requestMetrics != nil {
		return service.requestMetrics
	}
	service.requestMetrics = newAdminRequestMetrics()
	return service.requestMetrics
}

func (metrics *adminRequestMetrics) recordSlowRequest(record adminRequestRecord) {
	metrics.mutex.Lock()
	defer metrics.mutex.Unlock()
	metrics.recentSlowRequests = append(metrics.recentSlowRequests, record)
	if len(metrics.recentSlowRequests) > adminRecentSlowRequestLimit {
		metrics.recentSlowRequests = metrics.recentSlowRequests[len(metrics.recentSlowRequests)-adminRecentSlowRequestLimit:]
	}
}

func (metrics *adminRequestMetrics) RecentSlowRequests() []adminRequestRecord {
	metrics.mutex.Lock()
	defer metrics.mutex.Unlock()
	return append([]adminRequestRecord{}, metrics.recentSlowRequests...)
}

func shouldLogAdminRequest(record adminRequestRecord, duration time.Duration) bool {
	return record.Status >= http.StatusInternalServerError || shouldKeepSlowAdminRequest(record, duration)
}

func shouldKeepSlowAdminRequest(record adminRequestRecord, duration time.Duration) bool {
	if record.Status >= http.StatusInternalServerError {
		return true
	}
	switch record.EndpointClass {
	case adminEndpointStatic:
		return duration > 300*time.Millisecond
	case adminEndpointReadAPI:
		return duration > time.Second
	case adminEndpointControl:
		return duration > 500*time.Millisecond
	default:
		return duration > 2*time.Second
	}
}

func adminEndpointClassForRequest(request *http.Request) adminEndpointClass {
	path := request.URL.Path
	if isAdminControlPath(path) {
		return adminEndpointControl
	}
	if isAdminStaticPath(path) {
		return adminEndpointStatic
	}
	if isInternKimAPIPath(path) {
		if request.Method == http.MethodGet || request.Method == http.MethodHead {
			return adminEndpointReadAPI
		}
		return adminEndpointWriteAPI
	}
	return adminEndpointProxy
}

func isAdminControlPath(path string) bool {
	return path == "/_internkim/mattermost/actions" || path == "/_internkim/mattermost/commands"
}

func isAdminStaticPath(path string) bool {
	switch path {
	case "/admin", "/flow", "/memory", "/calendar", "/mail", "/attendance", "/tasks", "/company", "/poc-admin", "/logo.svg":
		return true
	default:
		return strings.HasPrefix(path, "/_app/") ||
			strings.HasPrefix(path, "/admin/") && !strings.HasPrefix(path, "/admin/api/") ||
			strings.HasPrefix(path, "/flow/") && !strings.HasPrefix(path, "/flow/api/") ||
			strings.HasPrefix(path, "/memory/") && !strings.HasPrefix(path, "/memory/api/") ||
			strings.HasPrefix(path, "/calendar/") && !strings.HasPrefix(path, "/calendar/api/") && !strings.HasPrefix(path, "/calendar/dav/") ||
			strings.HasPrefix(path, "/mail/") && !strings.HasPrefix(path, "/mail/api/") ||
			strings.HasPrefix(path, "/attendance/") && !strings.HasPrefix(path, "/attendance/api/") ||
			strings.HasPrefix(path, "/tasks/") && !strings.HasPrefix(path, "/tasks/api/") ||
			strings.HasPrefix(path, "/company/") && !strings.HasPrefix(path, "/company/api/") ||
			strings.HasPrefix(path, "/poc-admin/")
	}
}

func isInternKimAPIPath(path string) bool {
	return strings.HasPrefix(path, "/admin/api/") ||
		strings.HasPrefix(path, "/flow/api/") ||
		strings.HasPrefix(path, "/memory/api/") ||
		strings.HasPrefix(path, "/calendar/api/") ||
		strings.HasPrefix(path, "/mail/api/") ||
		strings.HasPrefix(path, "/attendance/api/") ||
		strings.HasPrefix(path, "/tasks/api/") ||
		strings.HasPrefix(path, "/company/api/") ||
		strings.HasPrefix(path, "/_internkim/companion/") ||
		strings.HasPrefix(path, "/_internkim/runtime/")
}

func sanitizedRequestPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return "/"
	}
	return path
}
