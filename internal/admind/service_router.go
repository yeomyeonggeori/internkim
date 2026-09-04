package admind

import (
	"bytes"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"

	"strings"

	"gitlab.com/eastriver/internkim/internal/fleetdomain"
)

func (service *Service) router() http.Handler {
	multiplexer := http.NewServeMux()
	service.registerAdminRoutes(multiplexer)
	service.registerPublicAPIRoutes(multiplexer)
	service.registerTaskRoutes(multiplexer)
	service.registerMemoryRoutes(multiplexer)
	service.registerAgentRoutes(multiplexer)
	service.registerCalendarRoutes(multiplexer)
	service.registerAuthenticationRoutes(multiplexer)
	service.registerMailRoutes(multiplexer)
	service.registerOrganizationRoutes(multiplexer)
	service.registerBuzzRoutes(multiplexer)
	service.registerFileRoutes(multiplexer)
	service.registerTaskRunRoutes(multiplexer)
	service.registerCompanyRoutes(multiplexer)
	service.registerAssetRoutes(multiplexer)
	service.registerBoardRoutes(multiplexer)
	multiplexer.Handle("/", service.mattermostProxy())
	return service.withRequestMetrics(service.withReadAPITimeout(service.withCORS(service.withSiteGateway(multiplexer))))
}

func (service *Service) registerAdminRoutes(multiplexer *http.ServeMux) {
	multiplexer.HandleFunc("/admin", service.serveAdminPage)
	multiplexer.HandleFunc("/admin/api/", service.handleAdmin)
	multiplexer.HandleFunc("/admin/", service.serveAdminPage)
}

func (service *Service) registerPublicAPIRoutes(multiplexer *http.ServeMux) {
	multiplexer.HandleFunc("/api/v1/", service.handlePublicAPI)
}

func (service *Service) registerTaskRoutes(multiplexer *http.ServeMux) {
	multiplexer.HandleFunc("/task", service.serveTaskPage)
	multiplexer.HandleFunc(taskAPIPrefix+"/", service.handleQuickTask)
	multiplexer.HandleFunc(recordToolPathPrefix, service.handleRecordTool)
	multiplexer.HandleFunc(tellDirectMessagePath, service.handleTellDirectMessage)
	multiplexer.HandleFunc("/task/", service.serveTaskPage)
	multiplexer.HandleFunc(retiredTaskAPIPrefix+"/", http.NotFound)
	multiplexer.HandleFunc("/flow", service.serveTaskPage)
	multiplexer.HandleFunc("/flow/", service.serveTaskPage)
}

func (service *Service) registerMemoryRoutes(multiplexer *http.ServeMux) {
	multiplexer.HandleFunc("/memory", service.serveMemoryPage)
	multiplexer.HandleFunc("/memory/api/", service.handleMemory)
	multiplexer.HandleFunc("/persona/api/", service.handlePersona)
	multiplexer.HandleFunc("/memory/", service.serveMemoryPage)
}

func (service *Service) registerAgentRoutes(multiplexer *http.ServeMux) {
	multiplexer.HandleFunc("/agent/api/dm", service.handleAgentDirectMessage)
	multiplexer.HandleFunc("/agent/api/channels", service.handleAgentChannels)
	multiplexer.HandleFunc("/agent/api/people", service.handleAgentPeople)
	multiplexer.HandleFunc("/agent/api/dm/ensure", service.handleEnsureDirectMessage)
	multiplexer.HandleFunc("/agent/api/buzz-vault", service.handleBuzzClientVault)
	multiplexer.HandleFunc("/agent/api/buzz-claim", service.handleBuzzClaim)
	multiplexer.HandleFunc("/agent/api/buzz-invite", service.handleBuzzInviteEmail)
	multiplexer.HandleFunc("/agent/api/buzz-relay-config", service.handleBuzzRelayConfig)
	multiplexer.HandleFunc(mediaProxyPrefix, service.handleBuzzMediaProxy)
	multiplexer.HandleFunc("/agent/api/buzz-mm-pending", service.handleBuzzMMPending)
	multiplexer.HandleFunc("/agent/api/buzz-mm-mirrored", service.handleBuzzMMMirrored)
	multiplexer.HandleFunc("/agent/api/buzz-admin-wipe", service.handleBuzzAdminWipe)
	multiplexer.HandleFunc("/agent/api/buzz-admin-reset", service.handleBuzzAdminReset)
	multiplexer.HandleFunc("/agent/api/buzz-repair-orphans", service.handleBuzzRepairOrphans)
	multiplexer.HandleFunc("/agent/api/buzz-stranger-members", service.handleBuzzStrangerMembers)
	multiplexer.HandleFunc("/agent/api/buzz-sweep-seats", service.handleBuzzSweepSeats)
	multiplexer.HandleFunc("/agent/api/buzz-ghost-rooms", service.handleBuzzGhostRooms)
	multiplexer.HandleFunc("/agent/api/buzz-identity-report", service.handleBuzzIdentityReport)
	multiplexer.HandleFunc("/agent/api/person-pictures", service.handlePersonPictures)
	multiplexer.HandleFunc("/agent/api/buzz-rewrite-old-links", service.handleBuzzRewriteOldLinks)
	multiplexer.HandleFunc("/agent/api/calendar-record-coverage", service.handleCalendarRecordCoverage)
	multiplexer.HandleFunc("/agent/api/attendance-record-coverage", service.handleAttendanceRecordCoverage)
	multiplexer.HandleFunc("/agent/api/task-record-coverage", service.handleTaskRecordCoverage)
	multiplexer.HandleFunc("/agent/api/organization-record-coverage", service.handleOrganizationRecordCoverage)
	multiplexer.HandleFunc("/agent/api/company-profile-carry", service.handleCompanyProfileCarry)
	multiplexer.HandleFunc("/agent/api/crm-record-coverage", service.handleCRMRecordCoverage)
	multiplexer.HandleFunc("/agent/api/company-ledger-coverage", service.handleCompanyLedgerCoverage)
	multiplexer.HandleFunc("/agent/api/mail-account-carry", service.handleMailAccountCarry)
	multiplexer.HandleFunc("/agent/api/buzz-channel-visibility-repair", service.handleBuzzChannelVisibilityRepair)
	multiplexer.HandleFunc("/agent/api/buzz-channel-membership-repair", service.handleBuzzChannelMembershipRepair)
	multiplexer.HandleFunc("/agent/api/buzz-channel-retire", service.handleBuzzChannelRetire)
	multiplexer.HandleFunc("/agent/api/circle-room-membership", service.handleCircleRoomMembership)
	multiplexer.HandleFunc("/agent/api/buzz-whose-key", service.handleBuzzWhoseKey)
}

func (service *Service) registerCalendarRoutes(multiplexer *http.ServeMux) {
	multiplexer.HandleFunc("/calendar", service.serveCalendarPage)
	multiplexer.HandleFunc("/calendar/api/", service.handleCalendar)
	multiplexer.HandleFunc("/calendar/", service.serveCalendarPage)
}

func (service *Service) registerAuthenticationRoutes(multiplexer *http.ServeMux) {
	multiplexer.HandleFunc("/auth/session", service.handleWebSession)
	multiplexer.HandleFunc("/auth/vault", service.handleAuthVault)
	multiplexer.HandleFunc("/auth/challenge", service.handleKeyLoginChallenge)
	multiplexer.HandleFunc("/auth/key-login", service.handleKeyLogin)
	multiplexer.HandleFunc("/auth/identity", service.handleAuthIdentity)
	multiplexer.HandleFunc("/auth/verify/start", service.handleEmailVerifyStart)
	multiplexer.HandleFunc("/auth/verify/callback", service.handleEmailVerifyCallback)
	multiplexer.HandleFunc("/auth/logout", service.handleWebLogout)
}

func (service *Service) registerMailRoutes(multiplexer *http.ServeMux) {
	multiplexer.HandleFunc("/mail", service.serveMailPage)
	multiplexer.HandleFunc("/mail/api/", service.handleMail)
	multiplexer.HandleFunc("/mail/", service.serveMailPage)
}

func (service *Service) registerOrganizationRoutes(multiplexer *http.ServeMux) {
	multiplexer.HandleFunc("/organization", service.serveOrganizationPage)
	multiplexer.HandleFunc("/organization/api/", service.handleOrganization)
	multiplexer.HandleFunc("/organization/", service.serveOrganizationPage)
}

func (service *Service) registerBuzzRoutes(multiplexer *http.ServeMux) {
	multiplexer.HandleFunc("/buzz/api/", service.handleBuzz)
	multiplexer.HandleFunc("/bridge/api/", service.handleBridgeMap)
}

func (service *Service) registerFileRoutes(multiplexer *http.ServeMux) {
	multiplexer.HandleFunc("/files", service.serveFilesPage)
	multiplexer.HandleFunc("/files/api/", service.handleFiles)
	multiplexer.HandleFunc("/files/", service.serveFilesPage)
	multiplexer.HandleFunc(skillInventoryPath, service.handleSkillInventory)
}

func (service *Service) registerTaskRunRoutes(multiplexer *http.ServeMux) {
	multiplexer.HandleFunc("/runs", service.serveTaskRunsPage)
	multiplexer.HandleFunc("/runs/api", service.handleTaskRuns)
	multiplexer.HandleFunc("/runs/api/", service.handleTaskRuns)
	multiplexer.HandleFunc("/runs/", service.serveTaskRunsPage)
}

func (service *Service) registerCompanyRoutes(multiplexer *http.ServeMux) {
	multiplexer.HandleFunc("/company", service.serveCompanySharePage)
	multiplexer.HandleFunc("/company/api/", service.handleCompanyShare)
	multiplexer.HandleFunc("/company/", service.serveCompanySharePage)
}

func (service *Service) registerAssetRoutes(multiplexer *http.ServeMux) {
	multiplexer.Handle("/_app/", http.FileServer(http.Dir(service.Configuration.AdminUIPath)))
	multiplexer.HandleFunc("/logo.svg", service.serveAdminAsset)
	multiplexer.HandleFunc("/_internkim/companion/", service.handleCompanion)
	multiplexer.HandleFunc("/_internkim/runtime/", service.handleRuntime)
	multiplexer.Handle(relayProxyPrefix, service.handleRelayProxy())
	multiplexer.Handle(relayProxyPrefix+"/", service.handleRelayProxy())
}

func (service *Service) registerBoardRoutes(multiplexer *http.ServeMux) {
	multiplexer.HandleFunc("/messenger", service.serveBoardSection("messenger"))
	multiplexer.HandleFunc("/messenger/", service.serveBoardSection("messenger"))
	multiplexer.HandleFunc("/settings", service.serveBoardSection("settings"))
	multiplexer.HandleFunc("/settings/", service.serveBoardSection("settings"))
	multiplexer.HandleFunc("/assistant", service.serveBoardSection("assistant"))
	multiplexer.HandleFunc("/assistant/", service.serveBoardSection("assistant"))
}

func (service *Service) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		origin := request.Header.Get("Origin")
		if isInternKimCORSPath(request.URL.Path) && service.isAllowedOrigin(origin) {
			responseWriter.Header().Set("Access-Control-Allow-Origin", origin)
			responseWriter.Header().Set("Access-Control-Allow-Credentials", "true")
			responseWriter.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, CF-Access-Authenticated-User-Email, X-INTERNKIM-COMPANION-ID, X-INTERNKIM-COMPANION-TOKEN")
			responseWriter.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS,PROPFIND,REPORT")
			if request.Method == http.MethodOptions {
				responseWriter.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(responseWriter, request)
	})
}

func isInternKimCORSPath(path string) bool {
	return path == "/admin" ||
		path == "/task" ||
		path == "/flow" ||
		path == "/memory" ||
		path == "/calendar" ||
		path == "/mail" ||
		path == "/attendance" ||
		path == "/files" ||
		path == "/runs" ||
		path == "/logo.svg" ||
		path == "/.well-known/caldav" ||
		strings.HasPrefix(path, "/api/v1/") ||
		strings.HasPrefix(path, "/admin/") ||
		strings.HasPrefix(path, "/task/") ||
		strings.HasPrefix(path, "/flow/") ||
		strings.HasPrefix(path, "/memory/") ||
		strings.HasPrefix(path, "/calendar/") ||
		strings.HasPrefix(path, "/mail/") ||
		strings.HasPrefix(path, "/attendance/") ||
		strings.HasPrefix(path, "/files/") ||
		strings.HasPrefix(path, "/runs/") ||
		strings.HasPrefix(path, "/_app/") ||
		strings.HasPrefix(path, "/_internkim/")
}

func (service *Service) mattermostProxy() http.Handler {
	targetURL, errorValue := url.Parse(service.Configuration.MattermostBaseURL)
	if errorValue != nil {
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		})
	}
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	if service.HTTPClient != nil && service.HTTPClient.Transport != nil {
		proxy.Transport = service.HTTPClient.Transport
	}
	return proxy
}

type bodyRecordingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

func (responseWriter *bodyRecordingResponseWriter) WriteHeader(statusCode int) {
	responseWriter.statusCode = statusCode
	responseWriter.ResponseWriter.WriteHeader(statusCode)
}

func (responseWriter *bodyRecordingResponseWriter) Write(document []byte) (int, error) {
	responseWriter.body.Write(document)
	return responseWriter.ResponseWriter.Write(document)
}

func (service *Service) serveAdminPage(responseWriter http.ResponseWriter, request *http.Request) {
	if !isLocalRequest(request) {
		service.ensureFirstAdminClaim(request.Context(), service.authenticatedCallerEmail(request))
	}
	if request.URL.Path == "/admin" {
		http.Redirect(responseWriter, request, "/admin/", http.StatusFound)
		return
	}
	relativePath := strings.TrimPrefix(request.URL.Path, "/admin/")
	if relativePath == "" {
		relativePath = "index.html"
	}
	filePath := filepath.Join(service.Configuration.AdminUIPath, relativePath)
	if fileInfo, errorValue := os.Stat(filePath); errorValue == nil && !fileInfo.IsDir() {
		http.ServeFile(responseWriter, request, filePath)
		return
	}
	http.ServeFile(responseWriter, request, filepath.Join(service.Configuration.AdminUIPath, "index.html"))
}

func (service *Service) serveAdminAsset(responseWriter http.ResponseWriter, request *http.Request) {
	http.ServeFile(responseWriter, request, filepath.Join(service.Configuration.AdminUIPath, strings.TrimPrefix(request.URL.Path, "/")))
}

func (service *Service) fleetZone() string {
	return fleetdomain.Zone(service.Configuration.APIBaseURL)
}

func (service *Service) isAllowedOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	parsedURL, errorValue := url.Parse(origin)
	if errorValue != nil {
		return false
	}
	host := strings.ToLower(parsedURL.Hostname())
	return fleetdomain.Covers(service.fleetZone(), host) || host == "localhost" || host == "127.0.0.1"
}
