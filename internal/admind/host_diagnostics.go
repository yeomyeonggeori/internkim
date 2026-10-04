package admind

import (
	"net/http"
	"strings"
)

func (service *Service) hostDiagnosticReaders(viewerEmail string) map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"runs": func(writer http.ResponseWriter, request *http.Request) {
			service.proxyScopedTaskList(writer, request, viewerEmail, true)
		},
		"run": func(writer http.ResponseWriter, request *http.Request) {
			service.proxyScopedTaskDetail(writer, request, viewerEmail, true)
		},
		"model_call": func(writer http.ResponseWriter, request *http.Request) {
			service.proxyAdminOnlyRunsRead(writer, request, true, "/admin/api/run/llm-call", "id")
		},
		"turn_input": func(writer http.ResponseWriter, request *http.Request) {
			service.proxyAdminOnlyRunsRead(writer, request, true, "/admin/api/run/turn-input", "id")
		},
		"inbound": func(writer http.ResponseWriter, request *http.Request) {
			service.proxyAdminOnlyRunsRead(writer, request, true, "/admin/api/connector/events", "conversationID", "messageID", "limit")
		},
		"service_logs": service.writeServiceLogs,
		"requests": func(writer http.ResponseWriter, _ *http.Request) {
			service.writeAdminRequestDiagnostics(writer)
		},
	}
}

func (service *Service) readPublicHostDiagnostics(responseWriter http.ResponseWriter, request *http.Request, actor publicToolGatewayActor) {
	if !actor.Actor.IsAdmin {
		http.Error(responseWriter, "only a company administrator can read host diagnostics", http.StatusForbidden)
		return
	}
	view := strings.TrimPrefix(request.URL.Path, "/api/v1/diagnostics/")
	reader := service.hostDiagnosticReaders(actor.Actor.Email)[view]
	if reader == nil {
		http.Error(responseWriter, "unknown diagnostics view", http.StatusBadRequest)
		return
	}
	reader(responseWriter, request)
}
