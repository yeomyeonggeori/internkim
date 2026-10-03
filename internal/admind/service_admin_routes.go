package admind

import (
	"net/http"
	"strings"
	"time"

	blueclawruntime "github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

type adminSessionResponse struct {
	Email                  string `json:"email"`
	ClaimedAdminEmail      string `json:"claimedAdminEmail"`
	IsAdmin                bool   `json:"isAdmin"`
	Role                   string `json:"role"`
	CanViewTasks           bool   `json:"canViewTasks"`
	IsClaimed              bool   `json:"isClaimed"`
	BootstrapStatus        string `json:"bootstrapStatus"`
	BootstrapError         string `json:"bootstrapError,omitempty"`
	TemporaryPassword      string `json:"temporaryPassword,omitempty"`
	TemporaryPasswordEmail string `json:"temporaryPasswordEmail,omitempty"`
}

func (service *Service) handleAdmin(responseWriter http.ResponseWriter, request *http.Request) {
	path := strings.TrimPrefix(request.URL.Path, "/admin/api")
	if service.handleAdminSessionRoute(responseWriter, request, path) {
		return
	}
	if service.handleAdminDirectoryRoute(responseWriter, request, path) {
		return
	}
	if !service.isAuthorized(request) {
		http.Error(responseWriter, "admin access required", http.StatusForbidden)
		return
	}
	if service.handleAdminDiagnosticsRoute(responseWriter, request, path) {
		return
	}
	if service.handleAdminUserRoute(responseWriter, request, path) {
		return
	}
	if service.handleAdminAgentRoute(responseWriter, request, path) {
		return
	}
	if service.handleAdminSettingsRoute(responseWriter, request, path) {
		return
	}
	if service.handleAdminCompanyRoute(responseWriter, request, path) {
		return
	}
	http.NotFound(responseWriter, request)
}

func (service *Service) handleAdminSessionRoute(responseWriter http.ResponseWriter, request *http.Request, path string) bool {
	if request.Method == http.MethodGet && path == "/session" {
		service.writeAdminSession(responseWriter, request)
		return true
	}
	if request.Method == http.MethodGet && path == "/health" {
		service.writeAdminHealth(responseWriter)
		return true
	}
	if request.Method == http.MethodGet && path == strings.TrimPrefix(blueclawruntime.AdmindRosterReadinessPath, "/admin/api") {
		service.writeRosterReadiness(responseWriter)
		return true
	}
	return false
}

func (service *Service) handleAdminDirectoryRoute(responseWriter http.ResponseWriter, request *http.Request, path string) bool {
	handler := service.adminDirectoryHandler(request.Method, path)
	if handler == nil {
		return false
	}
	localCallersOnly(handler)(responseWriter, request)
	return true
}

func (service *Service) adminDirectoryHandler(method string, path string) http.HandlerFunc {
	switch {
	case method == http.MethodPost && path == "/directory/person":
		return service.handleDirectoryPerson
	case method == http.MethodGet && path == "/directory/people":
		return service.handleDirectoryPeople
	case method == http.MethodPost && path == "/buzz/signing-secret":
		return service.handleBuzzSigningSecret
	case method == http.MethodPost && path == "/directory/buzz-key":
		return service.handleDirectoryBuzzKey
	case method == http.MethodPost && path == "/directory/direct-message":
		return service.handleDirectoryDirectMessage
	case method == http.MethodPost && path == "/directory/changed":
		return service.handleDirectoryChanged
	default:
		return nil
	}
}

func (service *Service) handleAdminDiagnosticsRoute(responseWriter http.ResponseWriter, request *http.Request, path string) bool {
	switch {
	case request.Method == http.MethodGet && path == "/diagnostics/requests":
		service.writeAdminRequestDiagnostics(responseWriter)
	case request.Method == http.MethodGet && path == "/diagnostics/connector-events":
		service.proxyBlueclawConnectorEvents(responseWriter, request)
	case request.Method == http.MethodGet && path == "/diagnostics/tasks":
		service.proxyBlueclawTaskList(responseWriter, request)
	case request.Method == http.MethodGet && path == "/diagnostics/learning":
		service.proxyBlueclawLearningOverview(responseWriter, request)
	case request.Method == http.MethodGet && path == "/diagnostics/task-detail":
		service.proxyBlueclawTaskDetail(responseWriter, request)
	case request.Method == http.MethodGet && path == "/maintenance/attachment-cleanup":
		service.handleAttachmentCleanup(responseWriter, request, false)
	case request.Method == http.MethodPost && path == "/maintenance/attachment-cleanup":
		service.handleAttachmentCleanup(responseWriter, request, true)
	case request.Method == http.MethodGet && path == "/diagnostics/service-logs":
		service.writeServiceLogs(responseWriter, request)
	case request.Method == http.MethodGet && path == "/diagnostics/llm-failure":
		service.writeLLMFailureEvidence(responseWriter, request)
	default:
		return false
	}
	return true
}

func (service *Service) handleAdminUserRoute(responseWriter http.ResponseWriter, request *http.Request, path string) bool {
	switch {
	case request.Method == http.MethodGet && path == "/users":
		service.proxyUsers(responseWriter, request)
	case request.Method == http.MethodPost && path == "/users":
		service.proxyUsers(responseWriter, request)
	case request.Method == http.MethodPost && path == "/circles":
		service.saveBlueclawCircle(responseWriter, request)
	case request.Method == http.MethodDelete && strings.HasPrefix(path, "/circles/"):
		service.deleteBlueclawCircle(responseWriter, request, strings.TrimPrefix(path, "/circles/"))
	case request.Method == http.MethodDelete && strings.HasPrefix(path, "/users/"):
		service.proxyUsers(responseWriter, request)
	default:
		return false
	}
	return true
}

func (service *Service) handleAdminAgentRoute(responseWriter http.ResponseWriter, request *http.Request, path string) bool {
	switch {
	case request.Method == http.MethodGet && path == "/soul":
		service.writeSoul(responseWriter, request)
	case request.Method == http.MethodPut && path == "/soul":
		service.updateSoul(responseWriter, request)
	case request.Method == http.MethodGet && path == "/credentials/providers":
		service.writeCredentialProviders(responseWriter)
	case request.Method == http.MethodPut && path == "/credentials/openrouter-key":
		service.updateOpenRouterKey(responseWriter, request)
	case request.Method == http.MethodDelete && path == "/credentials/openrouter-key":
		service.deleteOpenRouterKey(responseWriter)
	default:
		return false
	}
	return true
}

func (service *Service) handleAdminSettingsRoute(responseWriter http.ResponseWriter, request *http.Request, path string) bool {
	switch {
	case request.Method == http.MethodGet && path == "/locale":
		service.writeAdminLocale(responseWriter)
	case request.Method == http.MethodPut && path == "/locale":
		service.updateAdminLocale(responseWriter, request)
	case request.Method == http.MethodGet && path == "/workspace-settings":
		service.writeWorkspaceSettings(responseWriter, request)
	case request.Method == http.MethodPut && path == "/workspace-settings":
		service.updateWorkspaceSettings(responseWriter, request)
	default:
		return false
	}
	return true
}

func (service *Service) handleAdminCompanyRoute(responseWriter http.ResponseWriter, request *http.Request, path string) bool {
	switch {
	case request.Method == http.MethodGet && path == "/company-share":
		service.writeCompanyShareSettings(responseWriter)
	case request.Method == http.MethodPut && path == "/company-share":
		service.updateCompanyShareSettings(responseWriter, request)
	case request.Method == http.MethodPost && path == "/company-share/publish":
		service.publishCompanyShareSnapshot(responseWriter, request)
	case request.Method == http.MethodGet && path == "/company-info":
		service.writeCompanyInfo(responseWriter, request)
	case request.Method == http.MethodPut && path == "/company-info":
		service.updateCompanyInfo(responseWriter, request)
	default:
		return false
	}
	return true
}

func (service *Service) writeAdminSession(responseWriter http.ResponseWriter, request *http.Request) {
	callerEmail := service.authenticatedCallerEmail(request)
	bootstrapResult := service.ensureFirstAdminClaim(request.Context(), callerEmail)
	claimedAdminEmail := service.claimedAdminEmail()
	isClaimedAdmin := callerEmail != "" && strings.EqualFold(callerEmail, claimedAdminEmail)
	consoleEmail := service.adminConsoleActorEmail(request)
	role := service.adminSessionRole(request.Context(), consoleEmail)
	canViewTasks := role == adminUserRoleAdmin
	response := adminSessionResponse{
		Email:             consoleEmail,
		ClaimedAdminEmail: claimedAdminEmail,
		IsAdmin:           role == adminUserRoleAdmin,
		Role:              role,
		CanViewTasks:      canViewTasks,
		IsClaimed:         claimedAdminEmail != "",
		BootstrapStatus:   bootstrapResult.Status,
		BootstrapError:    bootstrapResult.Error,
	}
	if isClaimedAdmin {
		passwordDocument := service.consumeFirstAdminPassword(callerEmail)
		response.TemporaryPassword = passwordDocument.Password
		response.TemporaryPasswordEmail = passwordDocument.Email
	}
	service.writeJSON(responseWriter, response)
}

func (service *Service) writeAdminHealth(responseWriter http.ResponseWriter) {
	startedAt := service.startedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	service.writeJSON(responseWriter, map[string]any{
		"status":            "ok",
		"admindBuildID":     BuildID,
		"gitRevision":       GitRevision,
		"startedAt":         startedAt,
		"recoveryAvailable": true,
	})
}
