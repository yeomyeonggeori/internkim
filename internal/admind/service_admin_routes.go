package admind

import (
	"net/http"

	"strings"
	"time"
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
	DeviceManaged          bool   `json:"deviceManaged"`
}

func (service *Service) handleAdmin(responseWriter http.ResponseWriter, request *http.Request) {
	path := strings.TrimPrefix(request.URL.Path, "/admin/api")
	if service.handleAdminSessionRoute(responseWriter, request, path) {
		return
	}
	if service.handleAdminDirectoryRoute(responseWriter, request, path) {
		return
	}
	if service.handleAdminRecoveryRoute(responseWriter, request, path) {
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
	if service.handleAdminSiteRoute(responseWriter, request, path) {
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
	return false
}

func (service *Service) handleAdminDirectoryRoute(responseWriter http.ResponseWriter, request *http.Request, path string) bool {
	if request.Method == http.MethodPost && path == "/directory/person" {
		service.handleDirectoryPerson(responseWriter, request)
		return true
	}
	if request.Method == http.MethodGet && path == "/directory/people" {
		service.handleDirectoryPeople(responseWriter, request)
		return true
	}
	if request.Method == http.MethodPost && path == "/buzz/signing-secret" {
		service.handleBuzzSigningSecret(responseWriter, request)
		return true
	}
	if request.Method == http.MethodPost && path == "/directory/buzz-key" {
		service.handleDirectoryBuzzKey(responseWriter, request)
		return true
	}
	if request.Method == http.MethodPost && path == "/directory/direct-message" {
		service.handleDirectoryDirectMessage(responseWriter, request)
		return true
	}
	if request.Method == http.MethodPost && path == "/directory/changed" {
		service.handleDirectoryChanged(responseWriter, request)
		return true
	}
	return false
}

func (service *Service) handleAdminRecoveryRoute(responseWriter http.ResponseWriter, request *http.Request, path string) bool {
	if strings.HasPrefix(path, "/recovery/ssh-tunnel") {
		service.handleSSHRecovery(responseWriter, request, path)
		return true
	}
	return false
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

func (service *Service) handleAdminSiteRoute(responseWriter http.ResponseWriter, request *http.Request, path string) bool {
	switch {
	case request.Method == http.MethodGet && path == "/sites":
		service.listSites(responseWriter, request)
	case request.Method == http.MethodPost && path == "/sites":
		service.createSite(responseWriter, request)
	case request.Method == http.MethodPost && path == "/sites/serve":
		service.serveSiteFromRequest(responseWriter, request)
	case strings.HasPrefix(path, "/sites/"):
		service.handleSite(responseWriter, request, strings.TrimPrefix(path, "/sites/"))
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
		DeviceManaged:     service.hasDeviceAuth(),
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
