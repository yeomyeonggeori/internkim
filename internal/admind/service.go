package admind

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	blueclawruntime "gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

var BuildID = "unknown"
var GitRevision = "unknown"

type Configuration struct {
	ListenAddress                  string
	MattermostBaseURL              string
	ChatdEndpoint                  string
	ChatdPlatform                  string
	MattermostTeamName             string
	BotUsername                    string
	MattermostPublicURL            string
	FlowPublicURL                  string
	APIBaseURL                     string
	BlueclawBaseURL                string
	CapabilitySocketPath           string
	StateDirectory                 string
	CompanionJobPath               string
	DatabasePath                   string
	FlowDatabasePath               string
	CalendarDatabasePath           string
	CalendarSecretsDirectory       string
	MailDatabasePath               string
	AttendanceDatabasePath         string
	BridgeMapDatabasePath          string
	MattermostAdminPasswordPath    string
	MattermostTokenPath            string
	MattermostInteractiveTokenPath string
	MattermostInteractiveBaseURL   string
	MattermostOAuthClientPath      string
	OpenRouterKeyPath              string
	OpenRouterModelsURL            string
	ReleaseRegistryURL             string
	ReleaseDownloadTokenPath       string
	ReleaseSigningKeyPath          string
	MattermostBotTokenPath         string
	MattermostPluginBundlePath     string
	MattermostConfigFilePath       string
	AdminEmailPath                 string
	ClaimedAdminEmailPath          string
	FleetIDPath                    string
	DeviceURLPath                  string
	FleetSecretPath                string
	AdminUIPath                    string
	RepositoryRoot                 string
	CompanionFileDirectory         string
	SitesRoot                      string
	FontsDirectory                 string
	SiteSecretDirectory            string
	SiteSystemdDirectory           string
	BotProfilePath                 string
	BotProfileImagePath            string
	BlueclawWorkspacePath          string
	BlueclawRuntimeConfigPath      string
	CalendarSyncDisabled           bool
	BuzzInviteKeyPath              string
	BuzzCommunityID                string
	BuzzRelayURL                   string
	BuzzLandingBaseURL             string
	BuzzAdminCommandPath           string
	BuzzDatabaseURL                string
	BuzzAccountLinksPath           string
	BuzzKeySeedPath                string
	BuzzRelayKeyPath               string
	CloudflareAccessTeamDomain     string
	CloudflareAccessAUDs           string
	TrustProxyForwardedEmail       bool
}

type Service struct {
	Configuration Configuration
	HTTPClient    *http.Client
	RunCommand    func(context.Context, string, ...string) ([]byte, error)

	mutex                      sync.Mutex
	jobs                       map[string]*Job
	uploads                    map[string]*RestoreUpload
	blueclawUpdateUploads      map[string]*BlueclawUpdateUpload
	releaseUpdateUploads       map[string]*ReleaseUpdateUpload
	pairingCodes               map[string]*CompanionPairingCode
	companions                 map[string]*CompanionRecord
	companionJobs              map[string]*CompanionJob
	companionFileUploads       map[string]*CompanionFileUpload
	companionMounts            map[string]*CompanionMountRecord
	buzzInviteStore            *buzzInviteStore
	buzzInviteStoreOnce        sync.Once
	buzzKeySeedOnce            sync.Once
	buzzKeySeedValue           string
	cloudflareAccessOnce       sync.Once
	cloudflareAccessCheck      *cloudflareAccessVerifier
	sites                      map[string]*SiteRecord
	siteRuntimeMutex           sync.Mutex
	siteRuntimeActivities      map[string]*siteRuntimeActivity
	mailBackend                mailBackend
	googleOAuthStates          sync.Map
	calendarSyncWakeUp         chan struct{}
	calendarDeleteIntentWakeUp chan struct{}
	calendarSyncCycleMutex     sync.Mutex
	calendarRemoteMutex        sync.Mutex
	calendarOAuthTokenMutex    sync.Mutex
	calendarNotificationMutex  sync.Mutex
	calendarNotificationStates map[string]*calendarNotificationReconciliationState
	calendarNotificationLive   chan calendarNotificationReconciliationJob
	calendarNotificationRepair chan calendarNotificationReconciliationJob
	calendarNotificationCtx    context.Context
	calendarNotificationGroup  sync.WaitGroup
	calendarSwitchWaiters      atomic.Int64
	calendarStoreWriteMutex    sync.Mutex
	calendarHolidayCacheMutex  sync.RWMutex
	calendarHolidayLoadMutex   sync.Mutex
	calendarHolidayCache       map[calendarHolidayCacheKey][]calendarHoliday
	calendarCandidateClock     calendarConflictCandidateClock
	calendarPullCacheMutex     sync.Mutex
	lastCalendarPullAt         time.Time
	calendarActorCacheMutex    sync.Mutex
	calendarActorCache         map[string]calendarActorProfileCacheEntry
	companyShareMutex          sync.Mutex
	companyShareAttempts       map[string]companyShareAttempt
	attendanceLeavePolicyMutationMutex sync.Mutex
	policyRecordCacheMutex     sync.Mutex
	policyRecordCache          []adminUserMutation
	requestMetrics             *adminRequestMetrics
	databaseSchemas            *adminDatabaseSchemas
	legacyDatabaseMigration    sync.Once
	calendarWindowCache        calendarEventWindowCacheAvailability
	calendarWindowBuilds       calendarEventWindowCacheBuildCoordinator
	mattermostSessions         *mattermostSessionCache
	removeTokenQuarantineFile  func(string) error
	promoteCalendarTokenFile   func(string, string) error
	startedAt                  time.Time
}

type Job struct {
	JobID        string            `json:"jobID"`
	Type         string            `json:"type"`
	Status       string            `json:"status"`
	Phase        string            `json:"phase"`
	Error        string            `json:"error,omitempty"`
	CreatedAt    time.Time         `json:"createdAt"`
	UpdatedAt    time.Time         `json:"updatedAt"`
	DownloadURL  string            `json:"downloadURL,omitempty"`
	Manifest     *BackupManifest   `json:"manifest,omitempty"`
	Logs         []string          `json:"logs"`
	Result       map[string]string `json:"result,omitempty"`
	artifactPath string
}

type BackupManifest struct {
	FormatVersion  int               `json:"formatVersion"`
	FleetID        string            `json:"fleetID"`
	CreatedAt      time.Time         `json:"createdAt"`
	Components     []string          `json:"components"`
	Checksums      map[string]string `json:"checksums"`
	InternKim      map[string]string `json:"internKim"`
	Blueclaw       map[string]any    `json:"blueclaw,omitempty"`
	MattermostDump bool              `json:"mattermostDump"`
	BlueclawDump   bool              `json:"blueclawDump"`
}

type RestoreUpload struct {
	UploadID       string       `json:"uploadID"`
	Filename       string       `json:"filename"`
	Size           int64        `json:"size"`
	CreatedAt      time.Time    `json:"createdAt"`
	DirectoryPath  string       `json:"-"`
	ReceivedChunks map[int]bool `json:"-"`
}

type backupRequest struct {
	Passphrase string `json:"passphrase"`
}

type restoreRequest struct {
	Passphrase string `json:"passphrase"`
	Confirm    string `json:"confirm"`
}

type restoreUploadRequest struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

type restoreUploadResponse struct {
	UploadID  string `json:"uploadID"`
	ChunkSize int64  `json:"chunkSize"`
}

type restoreUploadCompleteRequest struct {
	Passphrase string `json:"passphrase"`
	Confirm    string `json:"confirm"`
	Chunks     int    `json:"chunks"`
}

type companionRelease struct {
	Platform     string `json:"platform"`
	Label        string `json:"label"`
	Architecture string `json:"architecture"`
	Status       string `json:"status"`
	URL          string `json:"url"`
}

type companionReleaseResponse struct {
	Platforms []companionRelease `json:"platforms"`
}

type adminSessionResponse struct {
	Email                  string `json:"email"`
	Image                  string `json:"image,omitempty"`
	ClaimedAdminEmail      string `json:"claimedAdminEmail"`
	IsAdmin                bool   `json:"isAdmin"`
	Role                   string `json:"role"`
	CanViewTasks           bool   `json:"canViewTasks"`
	IsPoCSuperAdmin        bool   `json:"isPocSuperAdmin"`
	IsClaimed              bool   `json:"isClaimed"`
	BootstrapStatus        string `json:"bootstrapStatus"`
	BootstrapError         string `json:"bootstrapError,omitempty"`
	TemporaryPassword      string `json:"temporaryPassword,omitempty"`
	TemporaryPasswordEmail string `json:"temporaryPasswordEmail,omitempty"`
	MattermostURL          string `json:"mattermostURL,omitempty"`
	DeviceManaged          bool   `json:"deviceManaged"`
}

type firstAdminPasswordDocument struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type firstAdminBootstrapResult struct {
	Email                     string `json:"email"`
	Status                    string `json:"status"`
	MattermostPasswordVersion string `json:"mattermostPasswordVersion,omitempty"`
	PolicyVersion             string `json:"policyVersion,omitempty"`
	Error                     string `json:"error,omitempty"`
}

const firstAdminBootstrapPending = "pending"
const firstAdminBootstrapClaimed = "claimed"
const firstAdminBootstrapIdentityMissing = "identity_missing"
const firstAdminBootstrapRejected = "rejected"
const firstAdminBootstrapFailed = "failed"
const firstAdminMattermostPasswordVersion = "api-v4-users-password-sidebar-v2"
const firstAdminPolicyVersion = "blueclaw-admin-claim-v1"
const firstAdminClaimTimeout = 90 * time.Second

func DefaultConfiguration() Configuration {
	return Configuration{
		ListenAddress:                  "127.0.0.1:18080",
		MattermostBaseURL:              "http://127.0.0.1:8065",
		MattermostTeamName:             "internkim",
		BotUsername:                    "internkim",
		APIBaseURL:                     "https://api.example.test",
		BlueclawBaseURL:                "http://127.0.0.1:8080",
		CapabilitySocketPath:           blueclawruntime.CapabilitySocketPath,
		StateDirectory:                 "/root/.internkim/state/admin",
		CompanionJobPath:               "/root/.internkim/state/companion-jobs.json",
		FlowDatabasePath:               "/root/.internkim/state/flow.sqlite",
		CalendarDatabasePath:           "/root/.internkim/state/calendar.sqlite",
		CalendarSecretsDirectory:       "/root/.internkim/secrets/google-oauth",
		MailDatabasePath:               "/root/.internkim/state/mail.sqlite",
		AttendanceDatabasePath:         "/root/.internkim/state/attendance.sqlite",
		BridgeMapDatabasePath:          "/root/.internkim/state/bridge-map.sqlite",
		BuzzKeySeedPath:                "/root/.internkim/secrets/buzz-key-seed",
		MattermostAdminPasswordPath:    "/root/.internkim/secrets/mm-admin-pass",
		MattermostTokenPath:            "/root/.internkim/secrets/mattermost-bot-token",
		MattermostInteractiveTokenPath: "/root/.internkim/state/admin/mattermost-interactive-token",
		MattermostOAuthClientPath:      "/root/.internkim/secrets/mattermost-oauth-client.json",
		OpenRouterKeyPath:              "/root/.internkim/secrets/openrouter-api-key",
		OpenRouterModelsURL:            "https://openrouter.ai/api/v1/models",
		ReleaseRegistryURL:             "https://updates.example.test",
		ReleaseDownloadTokenPath:       "/root/.internkim/secrets/release-download-token",
		ReleaseSigningKeyPath:          "/root/.internkim/secrets/release-signing-key",
		MattermostBotTokenPath:         "/root/.internkim/secrets/mattermost-bot-token",
		MattermostPluginBundlePath:     "/opt/internkim/mattermost-plugins/com.internkim.ephemeral-0.2.1.tar.gz",
		MattermostConfigFilePath:       "/opt/mattermost/config/config.json",
		AdminEmailPath:                 "/root/.internkim/config/admin-email",
		ClaimedAdminEmailPath:          "/root/.internkim/state/admin/claimed-admin-email",
		FleetIDPath:                    "/root/.internkim/env/fleet-id",
		DeviceURLPath:                  "/root/.internkim/env/device-url",
		FleetSecretPath:                "/root/.internkim/secrets/fleet-secret",
		AdminUIPath:                    "/opt/internkim/admin-ui",
		RepositoryRoot:                 "/",
		CompanionFileDirectory:         "/tmp/internkim-companion-files",
		SitesRoot:                      "/root/.internkim/sites",
		FontsDirectory:                 "/opt/internkim/fonts",
		SiteSecretDirectory:            "/root/.internkim/secrets/sites",
		SiteSystemdDirectory:           "/etc/systemd/system",
		BotProfilePath:                 "/root/.internkim/config/bot-profile.yaml",
		BotProfileImagePath:            "/opt/internkim/board-ui/logo.png",
		BlueclawWorkspacePath:          "/root/.blueclaw/workspace",
		BlueclawRuntimeConfigPath:      "/root/.blueclaw/config/runtime.json",
	}
}

func NewService(configuration Configuration) *Service {
	configuration = configuration.withDefaults()
	service := &Service{
		Configuration:              configuration,
		jobs:                       map[string]*Job{},
		uploads:                    map[string]*RestoreUpload{},
		blueclawUpdateUploads:      map[string]*BlueclawUpdateUpload{},
		releaseUpdateUploads:       map[string]*ReleaseUpdateUpload{},
		pairingCodes:               map[string]*CompanionPairingCode{},
		companions:                 map[string]*CompanionRecord{},
		companionJobs:              map[string]*CompanionJob{},
		companionFileUploads:       map[string]*CompanionFileUpload{},
		companionMounts:            map[string]*CompanionMountRecord{},
		sites:                      map[string]*SiteRecord{},
		mailBackend:                standardMailBackend{},
		calendarSyncWakeUp:         make(chan struct{}, 1),
		calendarDeleteIntentWakeUp: make(chan struct{}, 1),
		calendarNotificationStates: map[string]*calendarNotificationReconciliationState{},
		calendarActorCache:         map[string]calendarActorProfileCacheEntry{},
		calendarHolidayCache:       map[calendarHolidayCacheKey][]calendarHoliday{},
		companyShareAttempts:       map[string]companyShareAttempt{},
		requestMetrics:             newAdminRequestMetrics(),
		databaseSchemas:            newAdminDatabaseSchemas(),
		mattermostSessions:         newMattermostSessionCache(),
		startedAt:                  time.Now().UTC(),
	}
	service.loadCompanions()
	service.loadCompanionJobs()
	service.loadCompanionMounts()
	service.loadSites()
	return service
}

func (service *Service) Run(ctx context.Context) error {
	if errorValue := service.reconcileReleaseLLMDBootstrap(ctx); errorValue != nil {
		return fmt.Errorf("reconcile LLMD release bootstrap: %w", errorValue)
	}
	go service.reconcileBlueclawRuntimeConfiguration(ctx)
	service.reconcileSiteSourcesToStaffCircle()
	service.reconcilePublishedSitePocketBaseRuntimes(ctx)
	if errorValue := service.repairFutureAttendanceEvents(ctx, time.Now().UTC()); errorValue != nil {
		log.Printf("attendance future event repair failed: %v", errorValue)
	}
	if repairedCount, errorValue := service.repairAttendanceClockOutDates(ctx); errorValue != nil {
		log.Printf("attendance clock-out date repair failed: %v", errorValue)
	} else if repairedCount > 0 {
		log.Printf("attendance clock-out date repair completed: repaired=%d", repairedCount)
	}
	if errorValue := service.recoverGoogleOAuthTokenResetState(ctx); errorValue != nil {
		return fmt.Errorf("recover google calendar OAuth token reset state: %w", errorValue)
	}
	service.adoptAccountHireDates(ctx)
	service.startBotProfileSync(ctx)
	service.startCompanionFileCleanup(ctx)
	service.startMattermostProvisionerSync(ctx)
	service.startMattermostCircleSync(ctx)
	service.startMattermostProjectionOutboxWorker(ctx)
	service.startMattermostAttendanceStatusSync(ctx)
	service.startCalendarNotificationReconciliation(ctx)
	service.startCalendarNotificationWorker(ctx)
	service.startCalendarDeleteIntentWorker(ctx)
	service.startCalendarSyncWorker(ctx)
	service.startCalendarHolidayScheduler(ctx)
	service.startSoftDeletedMattermostPostPurge(ctx)
	service.startSiteRuntimeJanitor(ctx)
	service.startScheduledBackups(ctx)
	service.startBuzzMemberLinker(ctx)
	service.startBuzzAccountLinkSync(ctx)
	service.startStaffChannelMembershipSync(ctx)
	service.startMattermostPasswordHashSync(ctx)
	service.warnWhenFontAssetsMissing()
	server := &http.Server{
		Addr:    service.Configuration.ListenAddress,
		Handler: service.router(),
	}
	go func() {
		<-ctx.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
	}()
	errorValue := server.ListenAndServe()
	if errorValue != nil && !errors.Is(errorValue, http.ErrServerClosed) {
		return errorValue
	}
	return nil
}

func (service *Service) startMattermostCircleSync(ctx context.Context) {
	if strings.TrimSpace(readTrimmedFile(service.Configuration.MattermostAdminPasswordPath)) == "" {
		return
	}
	go func() {
		service.syncMattermostCirclesWithTimeout(ctx)
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				service.syncMattermostCirclesWithTimeout(ctx)
			}
		}
	}()
}

func (service *Service) startMattermostProjectionOutboxWorker(ctx context.Context) {
	if strings.TrimSpace(readTrimmedFile(service.Configuration.MattermostAdminPasswordPath)) == "" {
		return
	}
	go func() {
		service.repairMattermostProjectionsWithTimeout(ctx)
		service.sweepExpiredMattermostChannelPostsWithTimeout(ctx)
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		expiryTicker := time.NewTicker(time.Hour)
		defer expiryTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				service.drainMattermostProjectionOutboxWithTimeout(ctx)
			case <-expiryTicker.C:
				service.sweepExpiredMattermostChannelPostsWithTimeout(ctx)
			}
		}
	}()
}

func (service *Service) repairMattermostProjectionsWithTimeout(ctx context.Context) {
	service.syncExistingMattermostManagedPosts(ctx)
}

func (service *Service) sweepExpiredMattermostChannelPostsWithTimeout(ctx context.Context) {
	service.sweepExpiredFlowMattermostNotificationsWithTimeout(ctx)
	service.sweepExpiredCalendarMattermostLogsWithTimeout(ctx)
}

func (service *Service) sweepExpiredFlowMattermostNotificationsWithTimeout(ctx context.Context) {
	syncContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	service.sweepExpiredFlowMattermostNotifications(syncContext)
}

func (service *Service) sweepExpiredCalendarMattermostLogsWithTimeout(ctx context.Context) {
	syncContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	service.sweepExpiredCalendarMattermostLogs(syncContext)
}

func (service *Service) drainMattermostProjectionOutboxWithTimeout(ctx context.Context) {
	syncContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	service.drainMattermostManagedChannelProjections(syncContext)
}

func (service *Service) syncMattermostCirclesWithTimeout(ctx context.Context) {
	syncContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if errorValue := service.syncMattermostCirclesBestEffort(syncContext); errorValue != nil {
		log.Printf("Mattermost circle policy sync failed: %v", errorValue)
	}
}

func (service *Service) startMattermostProvisionerSync(ctx context.Context) {
	if strings.TrimSpace(readTrimmedFile(service.Configuration.MattermostAdminPasswordPath)) == "" {
		return
	}
	go func() {
		service.ensureMattermostEphemeralPluginWithRetry(ctx)
		syncContext, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if errorValue := service.ensureMattermostProvisionerDefaults(syncContext); errorValue != nil {
			log.Printf("Mattermost provisioner sync failed: %v", errorValue)
		}
	}()
}

func (service *Service) router() http.Handler {
	multiplexer := http.NewServeMux()
	multiplexer.HandleFunc("/admin", service.serveAdminPage)
	multiplexer.HandleFunc("/admin/api/", service.handleAdmin)
	multiplexer.HandleFunc("/admin/", service.serveAdminPage)
	multiplexer.HandleFunc("/api/v1/", service.handlePublicAPI)
	multiplexer.HandleFunc("/flow", service.serveFlowPage)
	multiplexer.HandleFunc("/flow/api/", service.handleFlow)
	multiplexer.HandleFunc("/flow/", service.serveFlowPage)
	multiplexer.HandleFunc("/memory", service.serveMemoryPage)
	multiplexer.HandleFunc("/memory/api/", service.handleMemory)
	multiplexer.HandleFunc("/agent/api/dm", service.handleAgentDirectMessage)
	multiplexer.HandleFunc("/agent/api/channels", service.handleAgentChannels)
	multiplexer.HandleFunc("/agent/api/people", service.handleAgentPeople)
	multiplexer.HandleFunc("/agent/api/dm/ensure", service.handleEnsureDirectMessage)
	multiplexer.HandleFunc("/agent/api/buzz-vault", service.handleBuzzClientVault)
	multiplexer.HandleFunc("/agent/api/buzz-claim", service.handleBuzzClaim)
	multiplexer.HandleFunc("/agent/api/buzz-invite", service.handleBuzzInviteEmail)
	multiplexer.HandleFunc("/agent/api/buzz-relay-config", service.handleBuzzRelayConfig)
	multiplexer.HandleFunc(buzzMediaProxyPrefix, service.handleBuzzMediaProxy)
	multiplexer.HandleFunc("/agent/api/buzz-mm-pending", service.handleBuzzMMPending)
	multiplexer.HandleFunc("/agent/api/buzz-mm-mirrored", service.handleBuzzMMMirrored)
	multiplexer.HandleFunc("/agent/api/buzz-admin-wipe", service.handleBuzzAdminWipe)
	multiplexer.HandleFunc("/agent/api/buzz-admin-reset", service.handleBuzzAdminReset)
	multiplexer.HandleFunc("/memory/", service.serveMemoryPage)
	multiplexer.HandleFunc("/calendar", service.serveCalendarPage)
	multiplexer.HandleFunc("/calendar/api/", service.handleCalendar)
	multiplexer.HandleFunc("/calendar/ics/", service.serveCalendarICS)
	multiplexer.HandleFunc("/calendar/dav/", service.serveCalendarDAV)
	multiplexer.HandleFunc("/calendar/oauth/google/start", service.handleGoogleOAuthStart)
	multiplexer.HandleFunc("/calendar/oauth/google/callback", service.handleGoogleOAuthCallback)
	multiplexer.HandleFunc("/calendar/", service.serveCalendarPage)
	multiplexer.HandleFunc("/auth/session", service.handleWebSession)
	multiplexer.HandleFunc("/auth/vault", service.handleAuthVault)
	multiplexer.HandleFunc("/auth/challenge", service.handleKeyLoginChallenge)
	multiplexer.HandleFunc("/auth/key-login", service.handleKeyLogin)
	multiplexer.HandleFunc("/auth/password-login", service.handleMattermostPasswordLogin)
	multiplexer.HandleFunc("/auth/identity", service.handleAuthIdentity)
	multiplexer.HandleFunc("/auth/verify/start", service.handleEmailVerifyStart)
	multiplexer.HandleFunc("/auth/verify/callback", service.handleEmailVerifyCallback)
	multiplexer.HandleFunc("/auth/logout", service.handleWebLogout)
	multiplexer.HandleFunc("/mail", service.serveMailPage)
	multiplexer.HandleFunc("/mail/api/", service.handleMail)
	multiplexer.HandleFunc("/mail/", service.serveMailPage)
	multiplexer.HandleFunc("/attendance", service.serveAttendancePage)
	multiplexer.HandleFunc("/attendance/api/", service.handleAttendance)
	multiplexer.HandleFunc("/attendance/", service.serveAttendancePage)
	multiplexer.HandleFunc("/organization", service.serveOrganizationPage)
	multiplexer.HandleFunc("/organization/api/", service.handleOrganization)
	multiplexer.HandleFunc("/organization/", service.serveOrganizationPage)
	multiplexer.HandleFunc("/buzz/api/", service.handleBuzz)
	multiplexer.HandleFunc("/bridge/api/", service.handleBridgeMap)
	multiplexer.HandleFunc("/files", service.serveFilesPage)
	multiplexer.HandleFunc("/files/api/", service.handleFiles)
	multiplexer.HandleFunc("/files/", service.serveFilesPage)
	multiplexer.HandleFunc("/poc-admin", service.serveProofOfConceptAdminPage)
	multiplexer.HandleFunc("/poc-admin/", service.serveProofOfConceptAdminPage)
	multiplexer.HandleFunc("/tasks", service.serveTasksPage)
	multiplexer.HandleFunc("/tasks/api/", service.handleTasks)
	multiplexer.HandleFunc("/tasks/", service.serveTasksPage)
	multiplexer.HandleFunc("/company", service.serveCompanySharePage)
	multiplexer.HandleFunc("/company/api/", service.handleCompanyShare)
	multiplexer.HandleFunc("/company/", service.serveCompanySharePage)
	multiplexer.HandleFunc("/.well-known/caldav", service.serveCalendarDAV)
	multiplexer.Handle("/_app/", http.FileServer(http.Dir(service.Configuration.AdminUIPath)))
	multiplexer.HandleFunc("/logo.svg", service.serveAdminAsset)
	multiplexer.HandleFunc("/_internkim/companion/", service.handleCompanion)
	multiplexer.HandleFunc("/_internkim/runtime/", service.handleRuntime)
	multiplexer.HandleFunc("/_internkim/mattermost/commands", service.handleMattermostCommand)
	multiplexer.HandleFunc("/_internkim/mattermost/actions", service.handleMattermostInteractiveAction)
	multiplexer.Handle(relayProxyPrefix, service.handleRelayProxy())
	multiplexer.Handle(relayProxyPrefix+"/", service.handleRelayProxy())
	multiplexer.HandleFunc("/messenger", service.serveBoardSection("messenger"))
	multiplexer.HandleFunc("/messenger/", service.serveBoardSection("messenger"))
	multiplexer.HandleFunc("/settings", service.serveBoardSection("settings"))
	multiplexer.HandleFunc("/settings/", service.serveBoardSection("settings"))
	multiplexer.HandleFunc("/assistant", service.serveBoardSection("assistant"))
	multiplexer.HandleFunc("/assistant/", service.serveBoardSection("assistant"))
	multiplexer.Handle("/", service.managedChannelWriteGuard(service.attendancePostDeleteSync(service.mattermostProxy())))
	return service.withRequestMetrics(service.withReadAPITimeout(service.withCORS(service.withSiteGateway(multiplexer))))
}

func (service *Service) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		origin := request.Header.Get("Origin")
		if isInternKimCORSPath(request.URL.Path) && isAllowedOrigin(origin) {
			responseWriter.Header().Set("Access-Control-Allow-Origin", origin)
			responseWriter.Header().Set("Access-Control-Allow-Credentials", "true")
			responseWriter.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, CF-Access-Authenticated-User-Email, X-InternKim-Companion-ID, X-InternKim-Companion-Token")
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
		path == "/flow" ||
		path == "/memory" ||
		path == "/calendar" ||
		path == "/mail" ||
		path == "/attendance" ||
		path == "/files" ||
		path == "/poc-admin" ||
		path == "/tasks" ||
		path == "/logo.svg" ||
		path == "/.well-known/caldav" ||
		strings.HasPrefix(path, "/api/v1/") ||
		strings.HasPrefix(path, "/admin/") ||
		strings.HasPrefix(path, "/flow/") ||
		strings.HasPrefix(path, "/memory/") ||
		strings.HasPrefix(path, "/calendar/") ||
		strings.HasPrefix(path, "/mail/") ||
		strings.HasPrefix(path, "/attendance/") ||
		strings.HasPrefix(path, "/files/") ||
		strings.HasPrefix(path, "/poc-admin/") ||
		strings.HasPrefix(path, "/tasks/") ||
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

func (service *Service) managedChannelWriteGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		payload, command, isAttendanceCommand, errorValue := service.mattermostAttendancePostCreateCommand(request)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
			return
		}
		if isAttendanceCommand {
			responseBody, isCreated := serveMattermostPostCreate(next, responseWriter, request)
			if isCreated {
				if errorValue := service.syncMattermostAttendancePostCommand(request.Context(), request, payload, command, responseBody); errorValue != nil {
					log.Printf("Mattermost Attendance post command sync failed: channelID=%q rootID=%q kind=%q timeUpdate=%v: %v", strings.TrimSpace(payload.ChannelID), strings.TrimSpace(payload.RootID), strings.TrimSpace(command.Kind), command.IsTimeUpdate, errorValue)
				}
			}
			return
		}
		if service.isMattermostFlowOrCalendarPostCreateRequest(request) {
			http.Error(responseWriter, "managed channel is read-only", http.StatusBadRequest)
			return
		}
		if !service.isMattermostManagedPostCreateRequest(request) {
			next.ServeHTTP(responseWriter, request)
			return
		}
		responseBody, isCreated := serveMattermostPostCreate(next, responseWriter, request)
		if !isCreated {
			return
		}
		if errorValue := service.deleteCreatedMattermostPost(request.Context(), responseBody); errorValue != nil {
			log.Printf("Mattermost managed channel post cleanup failed: %v", errorValue)
		}
	})
}

func serveMattermostPostCreate(next http.Handler, responseWriter http.ResponseWriter, request *http.Request) ([]byte, bool) {
	recorder := &bodyRecordingResponseWriter{ResponseWriter: responseWriter, statusCode: http.StatusOK}
	next.ServeHTTP(recorder, request)
	isCreated := recorder.statusCode >= http.StatusOK && recorder.statusCode < http.StatusMultipleChoices
	return recorder.body.Bytes(), isCreated
}

func (service *Service) deleteCreatedMattermostPost(ctx context.Context, responseBody []byte) error {
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return errorValue
	}
	return service.deleteMattermostPost(ctx, adminToken, mattermostCreatedPostID(responseBody))
}

func (service *Service) isMattermostFlowOrCalendarPostCreateRequest(request *http.Request) bool {
	return service.isMattermostPostCreateRequestForChannels(request, []string{
		readTrimmedFile(service.mattermostFlowChannelIDPath()),
		readTrimmedFile(service.mattermostCalendarChannelIDPath()),
	})
}

func (service *Service) isMattermostManagedPostCreateRequest(request *http.Request) bool {
	return service.isMattermostPostCreateRequestForChannels(request, []string{
		readTrimmedFile(service.mattermostFlowChannelIDPath()),
		readTrimmedFile(service.mattermostCalendarChannelIDPath()),
		readTrimmedFile(service.mattermostAttendanceChannelIDPath()),
	})
}

func (service *Service) isMattermostPostCreateRequestForChannels(request *http.Request, channelIDs []string) bool {
	if request.Method != http.MethodPost || request.URL.Path != "/api/v4/posts" {
		return false
	}
	allowedChannelIDs := map[string]bool{}
	for _, channelID := range channelIDs {
		if trimmedChannelID := strings.TrimSpace(channelID); trimmedChannelID != "" {
			allowedChannelIDs[trimmedChannelID] = true
		}
	}
	if len(allowedChannelIDs) == 0 {
		return false
	}
	document, errorValue := io.ReadAll(request.Body)
	if errorValue != nil {
		return false
	}
	request.Body = io.NopCloser(bytes.NewReader(document))
	var payload struct {
		ChannelID string `json:"channel_id"`
	}
	if errorValue := json.Unmarshal(document, &payload); errorValue != nil {
		return false
	}
	return allowedChannelIDs[strings.TrimSpace(payload.ChannelID)]
}

func (service *Service) attendancePostDeleteSync(next http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		postID, ok := mattermostDeletedPostID(request)
		if !ok {
			next.ServeHTTP(responseWriter, request)
			return
		}
		if service.isMattermostAttendanceEntryPostID(postID) {
			http.Error(responseWriter, "attendance entry post is protected", http.StatusForbidden)
			return
		}
		recorder := &statusRecordingResponseWriter{ResponseWriter: responseWriter, statusCode: http.StatusOK}
		next.ServeHTTP(recorder, request)
		if recorder.statusCode >= http.StatusOK && recorder.statusCode < http.StatusMultipleChoices {
			if errorValue := service.deleteAttendanceEventByResultPostID(request.Context(), postID); errorValue != nil {
				log.Printf("Mattermost Attendance event delete sync failed: %v", errorValue)
			}
		}
	})
}

type statusRecordingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (responseWriter *statusRecordingResponseWriter) WriteHeader(statusCode int) {
	responseWriter.statusCode = statusCode
	responseWriter.ResponseWriter.WriteHeader(statusCode)
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

func mattermostDeletedPostID(request *http.Request) (string, bool) {
	if request.Method != http.MethodDelete {
		return "", false
	}
	prefix := "/api/v4/posts/"
	if !strings.HasPrefix(request.URL.Path, prefix) {
		return "", false
	}
	postID := strings.Trim(strings.TrimPrefix(request.URL.Path, prefix), "/")
	if postID == "" || strings.Contains(postID, "/") {
		return "", false
	}
	return postID, true
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

func (service *Service) handleAdmin(responseWriter http.ResponseWriter, request *http.Request) {
	path := strings.TrimPrefix(request.URL.Path, "/admin/api")
	if request.Method == http.MethodGet && path == "/session" {
		service.writeAdminSession(responseWriter, request)
		return
	}
	if request.Method == http.MethodGet && path == "/health" {
		service.writeAdminHealth(responseWriter)
		return
	}
	if strings.HasPrefix(path, "/recovery/ssh-tunnel") {
		service.handleSSHRecovery(responseWriter, request, path)
		return
	}
	if strings.HasPrefix(path, "/updates/blueclaw/uploads") {
		service.handleBlueclawUpdateUpload(responseWriter, request, path)
		return
	}
	if strings.HasPrefix(path, "/updates/uploads") {
		service.handleReleaseUpdateUpload(responseWriter, request, path)
		return
	}
	if request.Method == http.MethodGet && path == "/updates/status" {
		service.writeReleaseUpdateStatus(responseWriter, request)
		return
	}
	if request.Method == http.MethodGet && path == "/updates/releases" {
		service.writeReleaseHistory(responseWriter, request)
		return
	}
	if request.Method == http.MethodGet && strings.HasPrefix(path, "/updates/jobs/") {
		service.writeJob(responseWriter, strings.TrimPrefix(path, "/updates/jobs/"))
		return
	}
	if request.Method == http.MethodPost && path == "/updates/apply" && !service.isAuthorized(request) {
		service.applyReleaseUpdateSigned(responseWriter, request)
		return
	}
	if !service.isAuthorized(request) {
		if !service.isOperationsAdminRequest(request, path) {
			http.Error(responseWriter, "admin access required", http.StatusForbidden)
			return
		}
	}
	if service.rejectOperationsAdminRestrictedMutation(responseWriter, request, path) {
		return
	}

	switch {
	case request.Method == http.MethodPost && path == "/updates/apply":
		service.applyReleaseUpdate(responseWriter, request)
	case request.Method == http.MethodPost && path == "/updates/rollback":
		service.rollbackReleaseUpdate(responseWriter, request)
	case request.Method == http.MethodGet && path == "/updates/blueclaw/status":
		service.writeBlueclawUpdateStatus(responseWriter)
	case request.Method == http.MethodPost && path == "/updates/blueclaw/apply":
		service.applyLatestBlueclawUpdate(responseWriter, request)
	case request.Method == http.MethodGet && strings.HasPrefix(path, "/updates/blueclaw/jobs/"):
		service.writeJob(responseWriter, strings.TrimPrefix(path, "/updates/blueclaw/jobs/"))
	case request.Method == http.MethodGet && path == "/diagnostics/requests":
		service.writeAdminRequestDiagnostics(responseWriter)
	case request.Method == http.MethodGet && path == "/diagnostics/connector-events":
		service.proxyBlueclawConnectorEvents(responseWriter, request)
	case request.Method == http.MethodGet && path == "/diagnostics/tasks":
		service.proxyBlueclawTaskList(responseWriter, request)
	case request.Method == http.MethodGet && path == "/diagnostics/task-detail":
		service.proxyBlueclawTaskDetail(responseWriter, request)
	case request.Method == http.MethodGet && path == "/maintenance/attachment-cleanup":
		service.handleAttachmentCleanup(responseWriter, request, false)
	case request.Method == http.MethodPost && path == "/maintenance/attachment-cleanup":
		service.handleAttachmentCleanup(responseWriter, request, true)
	case request.Method == http.MethodGet && path == "/maintenance/attachment-migration":
		service.handleAttachmentMigration(responseWriter, request, false)
	case request.Method == http.MethodPost && path == "/maintenance/attachment-migration":
		service.handleAttachmentMigration(responseWriter, request, true)
	case request.Method == http.MethodGet && path == "/diagnostics/service-logs":
		service.writeServiceLogs(responseWriter, request)
	case request.Method == http.MethodGet && path == "/diagnostics/mattermost-post":
		service.writeMattermostPostDiagnostic(responseWriter, request)
	case request.Method == http.MethodPost && path == "/diagnostics/sync-mattermost-plugins":
		service.writeMattermostPluginSyncDiagnostic(responseWriter, request)
	case request.Method == http.MethodGet && path == "/locale":
		service.writeAdminLocale(responseWriter)
	case request.Method == http.MethodPut && path == "/locale":
		service.updateAdminLocale(responseWriter, request)
	case request.Method == http.MethodGet && path == "/users":
		if !service.hasDeviceAuth() {
			service.localListUsers(responseWriter, request)
			return
		}
		service.proxyUsers(responseWriter, request)
	case request.Method == http.MethodPost && path == "/users/org-profiles":
		service.localUpdateOrgProfiles(responseWriter, request)
	case request.Method == http.MethodPut && path == "/org-groups":
		service.localSetOrgGroups(responseWriter, request)
	case request.Method == http.MethodPost && path == "/users/batch":
		if !service.hasDeviceAuth() {
			service.localUpsertUsersBatch(responseWriter, request)
			return
		}
		service.proxyUsers(responseWriter, request)
	case request.Method == http.MethodPost && path == "/users":
		if !service.hasDeviceAuth() {
			service.localUpsertUser(responseWriter, request)
			return
		}
		service.proxyUsers(responseWriter, request)
	case request.Method == http.MethodPost && path == "/circles":
		service.saveBlueclawCircle(responseWriter, request)
	case request.Method == http.MethodDelete && strings.HasPrefix(path, "/circles/"):
		service.deleteBlueclawCircle(responseWriter, request, strings.TrimPrefix(path, "/circles/"))
	case request.Method == http.MethodPost && strings.HasPrefix(path, "/users/") && strings.HasSuffix(path, "/password-reset"):
		if !service.hasDeviceAuth() {
			service.localResetUserPassword(responseWriter, request, strings.TrimSuffix(strings.TrimPrefix(path, "/users/"), "/password-reset"))
			return
		}
		service.resetUserPassword(responseWriter, request, strings.TrimSuffix(strings.TrimPrefix(path, "/users/"), "/password-reset"))
	case request.Method == http.MethodDelete && strings.HasPrefix(path, "/users/"):
		if !service.hasDeviceAuth() {
			service.localRemoveUser(responseWriter, request, strings.TrimPrefix(path, "/users/"))
			return
		}
		service.proxyUsers(responseWriter, request)
	case request.Method == http.MethodPost && path == "/companion/pairing-codes":
		service.createCompanionPairingCode(responseWriter, request)
	case request.Method == http.MethodGet && path == "/companion/status":
		service.writeCompanionStatus(responseWriter, request)
	case request.Method == http.MethodGet && path == "/companion/releases":
		service.writeCompanionReleases(responseWriter)
	case request.Method == http.MethodDelete && strings.HasPrefix(path, "/companion/"):
		service.revokeCompanion(responseWriter, request, strings.TrimPrefix(path, "/companion/"))
	case request.Method == http.MethodGet && path == "/flow/status":
		service.writeFlowStatus(responseWriter)
	case request.Method == http.MethodGet && path == "/bot-profile":
		service.writeBotProfile(responseWriter, request)
	case request.Method == http.MethodPut && path == "/bot-profile":
		service.updateBotProfile(responseWriter, request)
	case request.Method == http.MethodGet && path == "/credentials/providers":
		service.writeCredentialProviders(responseWriter)
	case request.Method == http.MethodPut && path == "/credentials/openrouter-key":
		service.updateOpenRouterKey(responseWriter, request)
	case request.Method == http.MethodDelete && path == "/credentials/openrouter-key":
		service.deleteOpenRouterKey(responseWriter)
	case request.Method == http.MethodGet && path == "/workspace-settings":
		service.writeWorkspaceSettings(responseWriter)
	case request.Method == http.MethodPut && path == "/workspace-settings":
		service.updateWorkspaceSettings(responseWriter, request)
	case request.Method == http.MethodGet && path == "/holiday-countries":
		service.serveCalendarHolidayCountries(responseWriter, request)
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
	case request.Method == http.MethodPost && path == "/company-metrics":
		service.recordCompanyMetric(responseWriter, request)
	case request.Method == http.MethodGet && path == "/company-metrics":
		service.listCompanyMetrics(responseWriter, request)
	case request.Method == http.MethodPost && path == "/company-records":
		service.addCompanyRecord(responseWriter, request)
	case request.Method == http.MethodGet && path == "/company-records":
		service.listCompanyRecords(responseWriter, request)
	case request.Method == http.MethodPut && path == "/company-records":
		service.updateCompanyRecord(responseWriter, request)
	case request.Method == http.MethodDelete && path == "/company-records":
		service.deleteCompanyRecord(responseWriter, request)
	case request.Method == http.MethodPost && path == "/company-documents":
		service.registerCompanyDocument(responseWriter, request)
	case request.Method == http.MethodGet && path == "/company-documents":
		service.listCompanyDocuments(responseWriter, request)
	case request.Method == http.MethodPost && path == "/company-documents/search":
		service.searchCompanyDocuments(responseWriter, request)
	case request.Method == http.MethodPut && path == "/company-documents":
		service.updateCompanyDocument(responseWriter, request)
	case request.Method == http.MethodGet && path == "/attendance-locations":
		service.writeAttendanceLocations(responseWriter)
	case request.Method == http.MethodPut && path == "/attendance-locations":
		service.updateAttendanceLocations(responseWriter, request)
	case (request.Method == http.MethodGet || request.Method == http.MethodPut) && path == "/attendance-leave-policy":
		service.handleAttendanceLeavePolicy(responseWriter, request)
	case request.Method == http.MethodGet && path == "/wifi-profiles":
		service.writeWifiProfiles(responseWriter)
	case request.Method == http.MethodPost && path == "/wifi-profiles":
		service.addWifiProfile(responseWriter, request)
	case request.Method == http.MethodPut && strings.HasPrefix(path, "/wifi-profiles/"):
		service.updateWifiPassword(responseWriter, request, strings.TrimPrefix(path, "/wifi-profiles/"))
	case request.Method == http.MethodDelete && strings.HasPrefix(path, "/wifi-profiles/"):
		service.removeWifiProfile(responseWriter, request, strings.TrimPrefix(path, "/wifi-profiles/"))
	case request.Method == http.MethodPost && strings.HasPrefix(path, "/maintenance/mattermost-posts/") && strings.HasSuffix(path, "/repair"):
		service.repairMattermostPost(responseWriter, request, strings.TrimSuffix(strings.TrimPrefix(path, "/maintenance/mattermost-posts/"), "/repair"))
	case request.Method == http.MethodGet && path == "/sites":
		service.listSites(responseWriter, request)
	case request.Method == http.MethodPost && path == "/sites":
		service.createSite(responseWriter, request)
	case request.Method == http.MethodPost && path == "/sites/serve":
		service.serveSiteFromRequest(responseWriter, request)
	case strings.HasPrefix(path, "/sites/"):
		service.handleSite(responseWriter, request, strings.TrimPrefix(path, "/sites/"))
	case request.Method == http.MethodPost && path == "/backups":
		service.createBackup(responseWriter, request)
	case request.Method == http.MethodGet && strings.HasPrefix(path, "/backups/") && strings.HasSuffix(path, "/status"):
		service.writeJob(responseWriter, strings.TrimSuffix(strings.TrimPrefix(path, "/backups/"), "/status"))
	case request.Method == http.MethodGet && strings.HasPrefix(path, "/backups/") && strings.HasSuffix(path, "/download"):
		service.downloadBackup(responseWriter, request, strings.TrimSuffix(strings.TrimPrefix(path, "/backups/"), "/download"))
	case request.Method == http.MethodPost && path == "/restore/uploads":
		service.createRestoreUpload(responseWriter, request)
	case request.Method == http.MethodPut && strings.HasPrefix(path, "/restore/uploads/") && strings.Contains(path, "/chunks/"):
		service.writeRestoreUploadChunk(responseWriter, request, path)
	case request.Method == http.MethodPost && strings.HasPrefix(path, "/restore/uploads/") && strings.HasSuffix(path, "/complete"):
		service.completeRestoreUpload(responseWriter, request, path)
	case request.Method == http.MethodPost && path == "/restore":
		service.createRestore(responseWriter, request)
	case request.Method == http.MethodGet && strings.HasPrefix(path, "/restore/") && strings.HasSuffix(path, "/status"):
		service.writeJob(responseWriter, strings.TrimSuffix(strings.TrimPrefix(path, "/restore/"), "/status"))
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) writeCompanionReleases(responseWriter http.ResponseWriter) {
	service.writeJSON(responseWriter, companionReleaseResponse{Platforms: companionReleases()})
}

func (service *Service) writeAdminSession(responseWriter http.ResponseWriter, request *http.Request) {
	callerEmail := service.authenticatedCallerEmail(request)
	bootstrapResult := service.ensureFirstAdminClaim(request.Context(), callerEmail)
	claimedAdminEmail := service.claimedAdminEmail()
	isClaimedAdmin := callerEmail != "" && strings.EqualFold(callerEmail, claimedAdminEmail)
	consoleEmail := service.adminConsoleActorEmail(request)
	role := service.adminSessionRole(request.Context(), consoleEmail)
	isPoCSuperAdmin := service.isFlowAdminEmail(request.Context(), consoleEmail)
	canViewTasks := role == adminUserRoleAdmin || role == adminUserRoleOperationsAdmin
	sessionImageEmail := firstNonEmpty(consoleEmail, claimedAdminEmail)
	response := adminSessionResponse{
		Email:             consoleEmail,
		Image:             profileImagePathForEmail(sessionImageEmail),
		ClaimedAdminEmail: claimedAdminEmail,
		IsAdmin:           role == adminUserRoleAdmin,
		Role:              role,
		CanViewTasks:      canViewTasks,
		IsPoCSuperAdmin:   isPoCSuperAdmin,
		IsClaimed:         claimedAdminEmail != "",
		BootstrapStatus:   bootstrapResult.Status,
		BootstrapError:    bootstrapResult.Error,
		MattermostURL:     strings.TrimRight(strings.TrimSpace(service.Configuration.MattermostPublicURL), "/"),
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

func companionReleases() []companionRelease {
	return []companionRelease{
		{
			Platform:     "macos",
			Label:        "macOS",
			Architecture: "Apple Silicon beta",
			Status:       "available",
			URL:          capabilities.CompanionMacOSBetaDownloadURL(),
		},
		{
			Platform:     "windows",
			Label:        "Windows",
			Architecture: "x64",
			Status:       "coming_soon",
		},
		{
			Platform:     "linux",
			Label:        "Linux",
			Architecture: "x64 AppImage",
			Status:       "coming_soon",
		},
	}
}

func userIDFromAdminUsersResponse(responseBody []byte, email string) string {
	var responseDocument pagesUsersResponse
	if json.Unmarshal(responseBody, &responseDocument) != nil {
		return ""
	}
	for _, record := range responseDocument.Records {
		if strings.EqualFold(record.Email, email) {
			return strings.TrimSpace(record.UserID)
		}
	}
	return ""
}

func (service *Service) userIDForAdminUserMutation(ctx context.Context, fleetID string, fleetSecret string, payload adminUserMutation) (string, error) {
	if userID := strings.TrimSpace(payload.UserID); userID != "" {
		return userID, nil
	}
	records, errorValue := service.lookupUserRecords(ctx, fleetID, fleetSecret)
	if errorValue != nil {
		return "", errorValue
	}
	for _, record := range records {
		if strings.EqualFold(record.Email, payload.Email) && strings.TrimSpace(record.UserID) != "" {
			return strings.TrimSpace(record.UserID), nil
		}
	}
	return newInternKimUserID(), nil
}

func newInternKimUserID() string {
	return "user-" + randomHex(16)
}

const (
	adminUserRoleAdmin           = "admin"
	adminUserRoleMember          = "member"
	adminUserRoleOperationsAdmin = "operationsAdmin"
)

func normalizeAdminUserRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "admin":
		return adminUserRoleAdmin
	case "operationsadmin":
		return adminUserRoleOperationsAdmin
	}
	return adminUserRoleMember
}

func (service *Service) saveBlueclawCircle(responseWriter http.ResponseWriter, request *http.Request) {
	var input adminCircleRecord
	if errorValue := json.NewDecoder(request.Body).Decode(&input); errorValue != nil {
		http.Error(responseWriter, "invalid request body", http.StatusBadRequest)
		return
	}
	circleID := strings.ToLower(strings.TrimSpace(input.CircleID))
	if circleID == "" {
		http.Error(responseWriter, "circleID required", http.StatusBadRequest)
		return
	}
	if isReservedAdminCircleID(circleID) {
		http.Error(responseWriter, "reserved group cannot be changed", http.StatusBadRequest)
		return
	}
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	policyDocument["circles"] = upsertBlueclawCircle(policyDocument["circles"], adminCircleRecord{
		CircleID:            circleID,
		DisplayName:         firstNonEmpty(strings.TrimSpace(input.DisplayName), circleID),
		IsMattermostManaged: input.IsMattermostManaged,
	})
	policyDocument["circleSync"] = upsertBlueclawCircleSync(policyDocument["circleSync"], circleID, input.IsMattermostManaged)
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodPost, "/admin/api/policy/save", policyDocument, nil); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]any{"availableCircles": blueclawAvailableCircles(policyDocument)})
}

func (service *Service) deleteBlueclawCircle(responseWriter http.ResponseWriter, request *http.Request, encodedCircleID string) {
	circleID, _ := url.PathUnescape(encodedCircleID)
	circleID = strings.ToLower(strings.TrimSpace(circleID))
	if circleID == "" {
		http.Error(responseWriter, "circleID required", http.StatusBadRequest)
		return
	}
	if isReservedAdminCircleID(circleID) {
		http.Error(responseWriter, "reserved group cannot be removed", http.StatusBadRequest)
		return
	}
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	policyDocument["circles"] = removeBlueclawCircle(policyDocument["circles"], circleID)
	policyDocument["circleSync"] = removeBlueclawCircleSync(policyDocument["circleSync"], circleID)
	removeCircleFromBlueclawPeople(policyDocument["people"], circleID)
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodPost, "/admin/api/policy/save", policyDocument, nil); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]any{"availableCircles": blueclawAvailableCircles(policyDocument)})
}

func normalizeAdminUserCircles(circles []string, role string) []string {
	normalizedCircles := []string{"staff"}
	if normalizeAdminUserRole(role) == "admin" {
		normalizedCircles = append(normalizedCircles, "admin")
	}
	for _, circle := range circles {
		normalizedCircle := strings.ToLower(strings.TrimSpace(circle))
		if normalizedCircle == "" || normalizedCircle == "staff" {
			continue
		}
		normalizedCircles = append(normalizedCircles, normalizedCircle)
	}
	return uniqueAdminStrings(normalizedCircles)
}

func uniqueAdminStrings(values []string) []string {
	seenValue := map[string]bool{}
	result := []string{}
	for _, value := range values {
		normalizedValue := strings.ToLower(strings.TrimSpace(value))
		if normalizedValue == "" || seenValue[normalizedValue] {
			continue
		}
		seenValue[normalizedValue] = true
		result = append(result, normalizedValue)
	}
	return result
}

func upsertBlueclawCircle(value any, circle adminCircleRecord) []any {
	circleValues, _ := value.([]any)
	result := make([]any, 0, len(circleValues)+1)
	isUpdated := false
	for _, item := range circleValues {
		existingCircle, isCircle := item.(map[string]any)
		if !isCircle {
			continue
		}
		if strings.ToLower(strings.TrimSpace(mattermostPolicyString(existingCircle["circleID"]))) == circle.CircleID {
			existingCircle["displayName"] = circle.DisplayName
			existingCircle["isMattermostManaged"] = circle.IsMattermostManaged
			existingCircle["workspaceDirectoryPath"] = "/workspace/circles/" + circle.CircleID
			isUpdated = true
		}
		result = append(result, existingCircle)
	}
	if !isUpdated {
		result = append(result, map[string]any{
			"circleID":               circle.CircleID,
			"displayName":            circle.DisplayName,
			"isMattermostManaged":    circle.IsMattermostManaged,
			"workspaceDirectoryPath": "/workspace/circles/" + circle.CircleID,
		})
	}
	return result
}

func removeBlueclawCircle(value any, circleID string) []any {
	circleValues, _ := value.([]any)
	result := []any{}
	for _, item := range circleValues {
		circle, isCircle := item.(map[string]any)
		if !isCircle || strings.ToLower(strings.TrimSpace(mattermostPolicyString(circle["circleID"]))) == circleID {
			continue
		}
		result = append(result, circle)
	}
	return result
}

func upsertBlueclawCircleSync(value any, circleID string, isMattermostManaged bool) map[string]any {
	circleSync, _ := value.(map[string]any)
	if circleSync == nil {
		circleSync = map[string]any{}
	}
	if !isMattermostManaged {
		circleSync["mattermostPrivateChannels"] = removeMattermostCircleSync(circleSync["mattermostPrivateChannels"], circleID)
		return circleSync
	}
	channelValues := removeMattermostCircleSync(circleSync["mattermostPrivateChannels"], circleID)
	channelValues = append(channelValues, map[string]any{"circleID": circleID, "channelName": "circle-" + circleID})
	circleSync["mattermostPrivateChannels"] = channelValues
	return circleSync
}

func removeBlueclawCircleSync(value any, circleID string) map[string]any {
	circleSync, _ := value.(map[string]any)
	if circleSync == nil {
		return map[string]any{}
	}
	circleSync["mattermostPrivateChannels"] = removeMattermostCircleSync(circleSync["mattermostPrivateChannels"], circleID)
	return circleSync
}

func removeMattermostCircleSync(value any, circleID string) []any {
	channelValues, _ := value.([]any)
	result := []any{}
	for _, item := range channelValues {
		channel, isChannel := item.(map[string]any)
		if !isChannel || strings.ToLower(strings.TrimSpace(mattermostPolicyString(channel["circleID"]))) == circleID {
			continue
		}
		result = append(result, channel)
	}
	return result
}

func removeCircleFromBlueclawPeople(value any, circleID string) {
	people, _ := value.([]any)
	for _, item := range people {
		person, isPerson := item.(map[string]any)
		if !isPerson {
			continue
		}
		person["circles"] = removeAdminString(policyStringList(person["circles"]), circleID)
	}
}

func removeAdminString(values []string, removedValue string) []string {
	result := []string{}
	for _, value := range values {
		if strings.ToLower(strings.TrimSpace(value)) != removedValue {
			result = append(result, value)
		}
	}
	return result
}

func (service *Service) lookupRemovableUser(ctx context.Context, fleetID string, fleetSecret string, targetPath string) (*adminUserMutation, error) {
	email := strings.TrimPrefix(targetPath, "/")
	if decodedEmail, errorValue := url.PathUnescape(email); errorValue == nil {
		email = decodedEmail
	}
	records, errorValue := service.lookupUserRecords(ctx, fleetID, fleetSecret)
	if errorValue != nil {
		return nil, errorValue
	}
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	adminTotal := 0
	for index := range records {
		if records[index].Role == "admin" {
			adminTotal++
		}
	}
	for index := range records {
		record := records[index]
		if !strings.EqualFold(record.Email, normalizedEmail) {
			continue
		}
		if record.Role == "admin" && adminTotal <= 1 {
			return nil, fmt.Errorf("cannot remove the last admin user")
		}
		return &record, nil
	}
	return nil, nil
}

func (service *Service) resetUserPassword(responseWriter http.ResponseWriter, request *http.Request, encodedEmail string) {
	fleetID := strings.ToLower(strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)))
	fleetSecret := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetSecretPath))
	if fleetID == "" || fleetSecret == "" {
		http.Error(responseWriter, "device auth is not configured", http.StatusServiceUnavailable)
		return
	}
	email, errorValue := url.PathUnescape(strings.Trim(encodedEmail, "/"))
	if errorValue != nil {
		http.Error(responseWriter, "invalid email", http.StatusBadRequest)
		return
	}
	userRecord, errorValue := service.lookupUserRecordByEmail(request.Context(), fleetID, fleetSecret, email)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if userRecord == nil {
		http.Error(responseWriter, "user not found", http.StatusNotFound)
		return
	}
	resetResult, errorValue := service.resetMattermostUserPasswordAndHistory(request.Context(), *userRecord)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]any{
		"temporaryPassword":      resetResult.TemporaryPassword,
		"temporaryPasswordEmail": userRecord.Email,
		"deletedDMPostCount":     resetResult.DeletedPostCount,
	})
}

func (service *Service) lookupUserRecordByEmail(ctx context.Context, fleetID string, fleetSecret string, email string) (*adminUserMutation, error) {
	records, errorValue := service.lookupUserRecords(ctx, fleetID, fleetSecret)
	if errorValue != nil {
		return nil, errorValue
	}
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	for index := range records {
		if strings.EqualFold(records[index].Email, normalizedEmail) {
			return &records[index], nil
		}
	}
	return nil, nil
}

func (service *Service) isLastAdminDemotion(ctx context.Context, fleetID string, fleetSecret string, email string, role string) (bool, error) {
	if role == "admin" {
		return false, nil
	}
	records, errorValue := service.lookupUserRecords(ctx, fleetID, fleetSecret)
	if errorValue != nil {
		return false, errorValue
	}
	adminTotal := 0
	isTargetAdmin := false
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	for _, record := range records {
		if record.Role != "admin" {
			continue
		}
		adminTotal++
		if strings.EqualFold(record.Email, normalizedEmail) {
			isTargetAdmin = true
		}
	}
	return isTargetAdmin && adminTotal <= 1, nil
}

func (service *Service) lookupUserRecords(ctx context.Context, fleetID string, fleetSecret string) ([]adminUserMutation, error) {
	requestURL := strings.TrimRight(service.Configuration.APIBaseURL, "/") + "/api/users?fleet_id=" + url.QueryEscape(fleetID)
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if errorValue != nil {
		return nil, errorValue
	}
	request.Header.Set("X-InternKim-Fleet-ID", fleetID)
	request.Header.Set("X-InternKim-Fleet-Secret", fleetSecret)
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return nil, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		document, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("users lookup returned %d: %s", response.StatusCode, strings.TrimSpace(string(document)))
	}
	var usersResponse pagesUsersResponse
	if errorValue := json.NewDecoder(response.Body).Decode(&usersResponse); errorValue != nil {
		return nil, errorValue
	}
	return adminUserRecordsWithProfileImages(usersResponse.Records), nil
}

func (service *Service) createRestoreUpload(responseWriter http.ResponseWriter, request *http.Request) {
	var payload restoreUploadRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	uploadID := randomHex(16)
	directoryPath := filepath.Join(service.Configuration.StateDirectory, "uploads", uploadID)
	upload := &RestoreUpload{
		UploadID:       uploadID,
		Filename:       filepath.Base(payload.Filename),
		Size:           payload.Size,
		CreatedAt:      time.Now().UTC(),
		DirectoryPath:  directoryPath,
		ReceivedChunks: map[int]bool{},
	}
	service.mutex.Lock()
	if errorValue := os.MkdirAll(filepath.Join(directoryPath, "chunks"), 0o700); errorValue != nil {
		service.mutex.Unlock()
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.uploads[uploadID] = upload
	service.mutex.Unlock()
	service.writeJSON(responseWriter, restoreUploadResponse{UploadID: uploadID, ChunkSize: 4 << 20})
}

func (service *Service) writeRestoreUploadChunk(responseWriter http.ResponseWriter, request *http.Request, path string) {
	uploadID, chunkIndex, isValid := parseRestoreUploadChunkPath(path)
	if !isValid {
		http.NotFound(responseWriter, request)
		return
	}
	upload, isFound := service.findUpload(uploadID)
	if !isFound {
		http.NotFound(responseWriter, request)
		return
	}
	chunkPath := filepath.Join(upload.DirectoryPath, "chunks", strconv.Itoa(chunkIndex))
	chunkFile, errorValue := os.OpenFile(chunkPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	_, copyErrorValue := io.Copy(chunkFile, io.LimitReader(request.Body, 16<<20))
	closeErrorValue := chunkFile.Close()
	if copyErrorValue != nil {
		http.Error(responseWriter, copyErrorValue.Error(), http.StatusInternalServerError)
		return
	}
	if closeErrorValue != nil {
		http.Error(responseWriter, closeErrorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.markUploadChunk(uploadID, chunkIndex)
	service.writeJSON(responseWriter, map[string]string{"status": "ok"})
}

func (service *Service) completeRestoreUpload(responseWriter http.ResponseWriter, request *http.Request, path string) {
	uploadID := strings.TrimSuffix(strings.TrimPrefix(path, "/restore/uploads/"), "/complete")
	upload, isFound := service.findUpload(uploadID)
	if !isFound {
		http.NotFound(responseWriter, request)
		return
	}
	var payload restoreUploadCompleteRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.Passphrase) == "" {
		http.Error(responseWriter, "passphrase is required", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.Confirm) != "RESTORE" {
		http.Error(responseWriter, "confirm must be RESTORE", http.StatusBadRequest)
		return
	}
	if payload.Chunks <= 0 {
		http.Error(responseWriter, "chunks is required", http.StatusBadRequest)
		return
	}
	bundlePath := filepath.Join(upload.DirectoryPath, firstNonEmpty(upload.Filename, "restore.ikbak"))
	if errorValue := service.assembleRestoreUpload(upload, payload.Chunks, bundlePath); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	job := service.newJob("restore")
	service.writeJSON(responseWriter, job)
	go service.runRestoreJob(context.Background(), job.JobID, bundlePath, payload.Passphrase)
}

func (service *Service) createBackup(responseWriter http.ResponseWriter, request *http.Request) {
	var payload backupRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.Passphrase) == "" {
		http.Error(responseWriter, "passphrase is required", http.StatusBadRequest)
		return
	}
	job := service.newJob("backup")
	service.writeJSON(responseWriter, job)
	go service.runBackupJob(context.Background(), job.JobID, payload.Passphrase)
}

func (service *Service) createRestore(responseWriter http.ResponseWriter, request *http.Request) {
	if errorValue := request.ParseMultipartForm(64 << 20); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	payload := restoreRequest{
		Passphrase: request.FormValue("passphrase"),
		Confirm:    request.FormValue("confirm"),
	}
	if strings.TrimSpace(payload.Passphrase) == "" {
		http.Error(responseWriter, "passphrase is required", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.Confirm) != "RESTORE" {
		http.Error(responseWriter, "confirm must be RESTORE", http.StatusBadRequest)
		return
	}
	file, header, errorValue := request.FormFile("bundle")
	if errorValue != nil {
		http.Error(responseWriter, "bundle is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	job := service.newJob("restore")
	uploadPath := filepath.Join(service.jobDirectory(job.JobID), filepath.Base(header.Filename))
	if errorValue := os.MkdirAll(filepath.Dir(uploadPath), 0o700); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	uploadFile, errorValue := os.Create(uploadPath)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	_, copyErrorValue := io.Copy(uploadFile, file)
	closeErrorValue := uploadFile.Close()
	if copyErrorValue != nil {
		http.Error(responseWriter, copyErrorValue.Error(), http.StatusInternalServerError)
		return
	}
	if closeErrorValue != nil {
		http.Error(responseWriter, closeErrorValue.Error(), http.StatusInternalServerError)
		return
	}

	service.writeJSON(responseWriter, job)
	go service.runRestoreJob(context.Background(), job.JobID, uploadPath, payload.Passphrase)
}

func (service *Service) runBackupJob(ctx context.Context, jobID string, passphrase string) {
	job := service.mustJob(jobID)
	service.updateJob(jobID, "running", "collecting", "")
	jobDirectory := service.jobDirectory(jobID)
	plainPath := filepath.Join(jobDirectory, "internkim-backup.tar.gz")
	encryptedPath := filepath.Join(jobDirectory, "internkim-backup.ikbak")

	blueclawManifest, completeBlueclawBackup := service.prepareBlueclawBackup(ctx)
	defer completeBlueclawBackup()
	manifest, errorValue := service.createPlainBackup(ctx, plainPath, blueclawManifest)
	if errorValue != nil {
		service.updateJob(jobID, "failed", "collecting", errorValue.Error())
		return
	}
	service.updateJobManifest(jobID, manifest)
	service.updateJob(jobID, "running", "encrypting", "")
	if errorValue := encryptFile(plainPath, encryptedPath, passphrase); errorValue != nil {
		service.updateJob(jobID, "failed", "encrypting", errorValue.Error())
		return
	}
	_ = os.Remove(plainPath)

	job.artifactPath = encryptedPath
	job.DownloadURL = "/admin/api/backups/" + jobID + "/download"
	service.updateJob(jobID, "completed", "ready", "")
}

func (service *Service) createPlainBackup(ctx context.Context, plainPath string, blueclawManifest map[string]any) (*BackupManifest, error) {
	if errorValue := os.MkdirAll(filepath.Dir(plainPath), 0o700); errorValue != nil {
		return nil, errorValue
	}
	manifest := &BackupManifest{
		FormatVersion: 1,
		FleetID:       readTrimmedFile(service.Configuration.FleetIDPath),
		CreatedAt:     time.Now().UTC(),
		Components: []string{
			"internkim",
			"blueclaw",
			"mattermost",
			"cloudflared",
		},
		Checksums: map[string]string{},
		InternKim: map[string]string{
			"backupFormat": "internkim-admin-v1",
		},
		Blueclaw: blueclawManifest,
	}

	plainFile, errorValue := os.Create(plainPath)
	if errorValue != nil {
		return nil, errorValue
	}
	defer plainFile.Close()
	gzipWriter := gzip.NewWriter(plainFile)
	defer gzipWriter.Close()
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	for _, includedPath := range backupIncludedPaths() {
		if errorValue := addPathToTar(tarWriter, includedPath, manifest); errorValue != nil && !os.IsNotExist(errorValue) {
			return nil, errorValue
		}
	}
	dumpPath, errorValue := service.dumpMattermostDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	if dumpPath != "" {
		defer os.Remove(dumpPath)
		manifest.MattermostDump = true
		if errorValue := addNamedFileToTar(tarWriter, dumpPath, "mattermost-db.sql", manifest); errorValue != nil {
			return nil, errorValue
		}
	}
	blueclawDumpPath, errorValue := service.dumpBlueclawDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	if blueclawDumpPath != "" {
		defer os.Remove(blueclawDumpPath)
		manifest.BlueclawDump = true
		if errorValue := addNamedFileToTar(tarWriter, blueclawDumpPath, "blueclaw-db.sql", manifest); errorValue != nil {
			return nil, errorValue
		}
	}
	if errorValue := addManifestToTar(tarWriter, manifest); errorValue != nil {
		return nil, errorValue
	}
	return manifest, nil
}

func (service *Service) runRestoreJob(ctx context.Context, jobID string, encryptedPath string, passphrase string) {
	jobDirectory := service.jobDirectory(jobID)
	plainPath := filepath.Join(jobDirectory, "restore.tar.gz")
	extractDirectory := filepath.Join(jobDirectory, "extract")

	service.updateJob(jobID, "running", "decrypting", "")
	if errorValue := decryptFile(encryptedPath, plainPath, passphrase); errorValue != nil {
		service.updateJob(jobID, "failed", "decrypting", errorValue.Error())
		return
	}
	service.updateJob(jobID, "running", "extracting", "")
	manifest, errorValue := extractBundle(plainPath, extractDirectory)
	if errorValue != nil {
		service.updateJob(jobID, "failed", "extracting", errorValue.Error())
		return
	}
	service.updateJobManifest(jobID, manifest)

	service.updateJob(jobID, "running", "applying", "")
	if errorValue := service.applyRestore(ctx, extractDirectory); errorValue != nil {
		service.updateJob(jobID, "failed", "applying", errorValue.Error())
		return
	}
	service.updateJob(jobID, "completed", "restarting", "")
}

func (service *Service) applyRestore(ctx context.Context, extractDirectory string) error {
	commands := [][]string{
		{"systemctl", "stop", "blueclaw", "internkim-capabilityd", "mattermost", "internkim-users-sync.timer", "internkim-users-sync.service"},
	}
	for _, arguments := range commands {
		_, _ = service.runCommand(ctx, arguments[0], arguments[1:]...)
	}

	for _, relativePath := range []string{"root/.internkim", "root/.blueclaw", "opt/mattermost/config", "opt/mattermost/data", "etc/cloudflared"} {
		sourcePath := filepath.Join(extractDirectory, relativePath)
		if _, errorValue := os.Stat(sourcePath); errorValue == nil {
			targetPath := "/" + relativePath
			if errorValue := copyDirectory(sourcePath, targetPath); errorValue != nil {
				return errorValue
			}
		}
	}
	for _, relativePath := range []string{"root/.internkim/admin-email", "root/.internkim/claimed-admin-email"} {
		sourcePath := filepath.Join(extractDirectory, relativePath)
		if _, errorValue := os.Stat(sourcePath); errorValue == nil {
			if errorValue := copyRegularFile(sourcePath, "/"+relativePath); errorValue != nil {
				return errorValue
			}
		}
	}

	databaseDumpPath := filepath.Join(extractDirectory, "mattermost-db.sql")
	if _, errorValue := os.Stat(databaseDumpPath); errorValue == nil {
		_, _ = service.runCommand(ctx, "systemctl", "start", "postgresql")
		if _, errorValue := service.runCommand(ctx, "su", "-", "postgres", "-c", "dropdb --if-exists mattermost && createdb mattermost"); errorValue != nil {
			return errorValue
		}
		if _, errorValue := service.runCommand(ctx, "su", "-", "postgres", "-c", "psql mattermost < "+shellQuote(databaseDumpPath)); errorValue != nil {
			return errorValue
		}
	}
	blueclawDatabaseDumpPath := filepath.Join(extractDirectory, "blueclaw-db.sql")
	if _, errorValue := os.Stat(blueclawDatabaseDumpPath); errorValue == nil {
		_, _ = service.runCommand(ctx, "systemctl", "start", "postgresql")
		_, _ = service.runCommand(ctx, "su", "-", "postgres", "-c", "psql -tAc "+shellQuote("SELECT 1 FROM pg_roles WHERE rolname='blueclaw'")+" | grep -q 1 || createuser blueclaw")
		if _, errorValue := service.runCommand(ctx, "su", "-", "postgres", "-c", "dropdb --if-exists blueclaw && createdb -O blueclaw blueclaw"); errorValue != nil {
			return errorValue
		}
		if _, errorValue := service.runCommand(ctx, "su", "-", "postgres", "-c", "psql blueclaw < "+shellQuote(blueclawDatabaseDumpPath)); errorValue != nil {
			return errorValue
		}
	}

	_, _ = service.runCommand(ctx, "systemctl", "daemon-reload")
	_, _ = service.runCommand(ctx, "systemctl", "restart", "mattermost", "internkim-capabilityd", "blueclaw", "cloudflared")
	_, _ = service.runCommand(ctx, "systemctl", "enable", "--now", "internkim-users-sync.timer")
	return nil
}

func (service *Service) writeJob(responseWriter http.ResponseWriter, jobID string) {
	job, isFound := service.findJob(jobID)
	if !isFound {
		http.NotFound(responseWriter, nil)
		return
	}
	service.writeJSON(responseWriter, job)
}

func (service *Service) downloadBackup(responseWriter http.ResponseWriter, request *http.Request, jobID string) {
	job, isFound := service.findJob(jobID)
	if !isFound || job.Status != "completed" || strings.TrimSpace(job.artifactPath) == "" {
		http.NotFound(responseWriter, nil)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/octet-stream")
	responseWriter.Header().Set("Content-Disposition", `attachment; filename="internkim-backup-`+jobID+`.ikbak"`)
	http.ServeFile(responseWriter, request, job.artifactPath)
}

func (service *Service) newJob(jobType string) *Job {
	jobID := randomHex(16)
	now := time.Now().UTC()
	job := &Job{
		JobID:     jobID,
		Type:      jobType,
		Status:    "queued",
		Phase:     "queued",
		CreatedAt: now,
		UpdatedAt: now,
		Logs:      []string{},
		Result:    map[string]string{},
	}
	service.mutex.Lock()
	service.jobs[jobID] = job
	service.mutex.Unlock()
	return job
}

func (service *Service) mustJob(jobID string) *Job {
	job, _ := service.findJob(jobID)
	return job
}

func (service *Service) findJob(jobID string) (*Job, bool) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	job, isFound := service.jobs[jobID]
	return job, isFound
}

func (service *Service) findUpload(uploadID string) (*RestoreUpload, bool) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	upload, isFound := service.uploads[uploadID]
	return upload, isFound
}

func (service *Service) markUploadChunk(uploadID string, chunkIndex int) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	upload := service.uploads[uploadID]
	if upload == nil {
		return
	}
	upload.ReceivedChunks[chunkIndex] = true
}

func (service *Service) assembleRestoreUpload(upload *RestoreUpload, chunkCount int, bundlePath string) error {
	return assembleChunkDirectory(upload.DirectoryPath, chunkCount, bundlePath, "restore upload")
}

func assembleChunkDirectory(directoryPath string, chunkCount int, targetPath string, label string) error {
	targetFile, errorValue := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if errorValue != nil {
		return errorValue
	}
	defer targetFile.Close()
	for chunkIndex := 0; chunkIndex < chunkCount; chunkIndex++ {
		chunkPath := filepath.Join(directoryPath, "chunks", strconv.Itoa(chunkIndex))
		chunkFile, errorValue := os.Open(chunkPath)
		if errorValue != nil {
			return errors.New(label + " is missing chunk " + strconv.Itoa(chunkIndex))
		}
		_, copyErrorValue := io.Copy(targetFile, chunkFile)
		closeErrorValue := chunkFile.Close()
		if copyErrorValue != nil {
			return copyErrorValue
		}
		if closeErrorValue != nil {
			return closeErrorValue
		}
	}
	return nil
}

func (service *Service) updateJob(jobID string, status string, phase string, errorMessage string) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	job := service.jobs[jobID]
	if job == nil {
		return
	}
	job.Status = status
	job.Phase = phase
	job.Error = errorMessage
	job.UpdatedAt = time.Now().UTC()
	job.Logs = append(job.Logs, phase)
}

func (service *Service) updateJobManifest(jobID string, manifest *BackupManifest) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	job := service.jobs[jobID]
	if job == nil {
		return
	}
	job.Manifest = manifest
	job.UpdatedAt = time.Now().UTC()
}

func (service *Service) jobDirectory(jobID string) string {
	return filepath.Join(service.Configuration.StateDirectory, "jobs", jobID)
}

func (service *Service) isAuthorized(request *http.Request) bool {
	if isLocalRequest(request) {
		return true
	}
	callerEmail := service.adminConsoleActorEmail(request)
	if callerEmail == "" {
		return false
	}
	return service.isFlowAdminEmail(request.Context(), callerEmail)
}

func (service *Service) adminConsoleActorEmail(request *http.Request) string {
	return service.webActorEmail(request)
}

func (service *Service) authenticatedCallerEmail(request *http.Request) string {
	if email := service.cloudflareAccessVerifier().verifiedEmail(request.Context(), request); email != "" {
		return email
	}
	// Deployments that do not use Cloudflare front the app with their own
	// identity-aware reverse proxy (oauth2-proxy, Authelia, Authentik, Pomerium)
	// that authenticates the user and injects a trusted email header. The
	// operator opts in with TrustProxyForwardedEmail, asserting the proxy is the
	// only ingress. Absent that, a verified Cloudflare Access JWT is required,
	// except on loopback for local development and tests.
	if service.Configuration.TrustProxyForwardedEmail {
		return forwardedProxyEmail(request)
	}
	if service.cloudflareAccessVerifier().isConfigured() {
		return ""
	}
	if !trustsForwardedIdentity(service.Configuration.ListenAddress) {
		return ""
	}
	return forwardedProxyEmail(request)
}

func forwardedProxyEmail(request *http.Request) string {
	return strings.ToLower(strings.TrimSpace(firstNonEmpty(
		request.Header.Get("Cf-Access-Authenticated-User-Email"),
		request.Header.Get("CF-Access-Authenticated-User-Email"),
		request.Header.Get("X-Forwarded-Email"),
		request.Header.Get("X-Auth-Request-Email"),
	)))
}

func (service *Service) cloudflareAccessVerifier() *cloudflareAccessVerifier {
	service.cloudflareAccessOnce.Do(func() {
		audiences := strings.Split(service.Configuration.CloudflareAccessAUDs, ",")
		service.cloudflareAccessCheck = newCloudflareAccessVerifier(
			service.Configuration.CloudflareAccessTeamDomain,
			audiences,
			service.httpClient(),
		)
	})
	return service.cloudflareAccessCheck
}

func trustsForwardedIdentity(listenAddress string) bool {
	host, _, splitError := net.SplitHostPort(strings.TrimSpace(listenAddress))
	if splitError != nil {
		host = strings.TrimSpace(listenAddress)
	}
	switch host {
	case "127.0.0.1", "::1", "localhost":
		return true
	default:
		return false
	}
}

func (service *Service) hasDeviceAuth() bool {
	fleetID := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath))
	fleetSecret := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetSecretPath))
	return fleetID != "" && fleetSecret != ""
}

func (service *Service) isClaimedAdminEmail(email string) bool {
	claimedEmail := service.claimedAdminEmail()
	return claimedEmail != "" && strings.EqualFold(claimedEmail, email)
}

func (service *Service) claimedAdminEmail() string {
	paths := append([]string{service.Configuration.ClaimedAdminEmailPath}, legacyClaimedAdminEmailPaths(service.Configuration.ClaimedAdminEmailPath)...)
	return readLowerTrimmedFirstExistingFile(paths...)
}

func (service *Service) seedAdminEmail() string {
	paths := append([]string{service.Configuration.AdminEmailPath}, legacyAdminEmailPaths(service.Configuration.AdminEmailPath)...)
	return readLowerTrimmedFirstExistingFile(paths...)
}

func (service *Service) ensureFirstAdminClaim(ctx context.Context, callerEmail string) firstAdminBootstrapResult {
	normalizedEmail := strings.ToLower(strings.TrimSpace(callerEmail))
	if normalizedEmail == "" {
		return firstAdminBootstrapResult{Status: firstAdminBootstrapIdentityMissing}
	}

	claimContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), firstAdminClaimTimeout)
	defer cancel()

	claimedEmail := service.claimedAdminEmail()
	if claimedEmail != "" {
		if strings.EqualFold(claimedEmail, normalizedEmail) {
			return service.ensureClaimedFirstAdminAccount(claimContext, normalizedEmail)
		}
		return firstAdminBootstrapResult{Email: normalizedEmail, Status: firstAdminBootstrapRejected, Error: "first admin is already claimed by another email"}
	}
	if hasAdmin, errorValue := service.hasCurrentAdminUsers(claimContext); errorValue == nil && hasAdmin {
		return firstAdminBootstrapResult{Email: normalizedEmail, Status: firstAdminBootstrapRejected, Error: "first admin is already claimed by another email"}
	}

	result, errorValue := service.claimFirstAdmin(claimContext, normalizedEmail)
	if errorValue != nil {
		log.Printf("first admin bootstrap failed for %s: %v", normalizedEmail, errorValue)
		return service.writeFirstAdminBootstrapResult(firstAdminBootstrapResult{Email: normalizedEmail, Status: firstAdminBootstrapFailed, Error: errorValue.Error()})
	}
	log.Printf("first admin bootstrap claimed by %s", normalizedEmail)
	return service.writeFirstAdminBootstrapResult(result)
}

func (service *Service) ensureClaimedFirstAdminAccount(ctx context.Context, email string) firstAdminBootstrapResult {
	currentResult := service.readFirstAdminBootstrapResult()
	isCurrentAdmin := service.isCurrentAdminEmail(ctx, email)
	if currentResult.MattermostPasswordVersion == firstAdminMattermostPasswordVersion && currentResult.PolicyVersion == firstAdminPolicyVersion && isCurrentAdmin {
		currentResult.Email = email
		currentResult.Status = firstAdminBootstrapClaimed
		return currentResult
	}
	if hasAdmin, errorValue := service.hasCurrentAdminUsers(ctx); errorValue == nil && hasAdmin && !isCurrentAdmin {
		currentResult.Email = email
		currentResult.Status = firstAdminBootstrapClaimed
		return currentResult
	}
	result, errorValue := service.ensureFirstAdminAccount(ctx, email)
	if errorValue != nil {
		log.Printf("first admin repair failed for %s: %v", email, errorValue)
		return service.writeFirstAdminBootstrapResult(firstAdminBootstrapResult{Email: email, Status: firstAdminBootstrapFailed, Error: errorValue.Error()})
	}
	if !isCurrentAdmin {
		if errorValue := service.writeClaimedAdminRole(ctx, email); errorValue != nil {
			log.Printf("first admin role repair failed for %s: %v", email, errorValue)
			return service.writeFirstAdminBootstrapResult(firstAdminBootstrapResult{Email: email, Status: firstAdminBootstrapFailed, Error: errorValue.Error()})
		}
	}
	return service.writeFirstAdminBootstrapResult(result)
}

func (service *Service) claimFirstAdmin(ctx context.Context, email string) (firstAdminBootstrapResult, error) {
	fleetID := strings.ToLower(strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)))
	fleetSecret := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetSecretPath))
	if fleetID == "" || fleetSecret == "" {
		return firstAdminBootstrapResult{}, fmt.Errorf("device auth is not configured")
	}

	records, errorValue := service.lookupUserRecords(ctx, fleetID, fleetSecret)
	if errorValue != nil {
		return firstAdminBootstrapResult{}, errorValue
	}
	if errorValue := service.writeUserRole(ctx, fleetID, fleetSecret, email, "admin"); errorValue != nil {
		return firstAdminBootstrapResult{}, errorValue
	}
	for _, record := range records {
		if record.Role != "admin" || strings.EqualFold(record.Email, email) {
			continue
		}
		if errorValue := service.writeUserRole(ctx, fleetID, fleetSecret, record.Email, "member"); errorValue != nil {
			return firstAdminBootstrapResult{}, errorValue
		}
	}
	bootstrapResult, errorValue := service.ensureFirstAdminAccount(ctx, email)
	if errorValue != nil {
		return firstAdminBootstrapResult{}, errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(service.Configuration.ClaimedAdminEmailPath), 0o700); errorValue != nil {
		return firstAdminBootstrapResult{}, errorValue
	}
	if errorValue := os.WriteFile(service.Configuration.ClaimedAdminEmailPath, []byte(email), 0o600); errorValue != nil {
		return firstAdminBootstrapResult{}, errorValue
	}
	_ = os.WriteFile(service.Configuration.AdminEmailPath, []byte(email), 0o644)
	return bootstrapResult, nil
}

func (service *Service) ensureFirstAdminAccount(ctx context.Context, email string) (firstAdminBootstrapResult, error) {
	provisionResult, errorValue := service.provisionMattermostUserWithPassword(ctx, adminUserMutation{
		Email:  email,
		Role:   "admin",
		Handle: mattermostUsernameBase(email),
	}, firstAdminMattermostPassword)
	if errorValue != nil {
		return firstAdminBootstrapResult{}, errorValue
	}
	if provisionResult.TemporaryPassword != "" {
		if errorValue := service.writeFirstAdminPassword(email, provisionResult.TemporaryPassword); errorValue != nil {
			return firstAdminBootstrapResult{}, errorValue
		}
	}
	if errorValue := service.claimBlueclawAdminPerson(ctx, email); errorValue != nil {
		return firstAdminBootstrapResult{}, errorValue
	}
	service.triggerUsersSync(ctx)
	return firstAdminBootstrapResult{
		Email:                     email,
		Status:                    firstAdminBootstrapClaimed,
		MattermostPasswordVersion: firstAdminMattermostPasswordVersion,
		PolicyVersion:             firstAdminPolicyVersion,
	}, nil
}

func (service *Service) claimBlueclawAdminPerson(ctx context.Context, email string) error {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return fmt.Errorf("email required")
	}
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		return errorValue
	}
	policyDocument["people"] = claimedAdminPeople(policyDocument["people"], normalizedEmail)
	return service.blueclawJSONRequest(ctx, http.MethodPost, "/admin/api/policy/save", policyDocument, nil)
}

func claimedAdminPeople(value any, email string) []map[string]any {
	existingPeople, _ := value.([]any)
	claimedPeople := make([]map[string]any, 0, len(existingPeople)+1)
	adminPerson := map[string]any{}
	for _, value := range existingPeople {
		person, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if isBlueclawAdminPerson(person) {
			adminPerson = person
			continue
		}
		remainingEmails := blueclawPersonEmailsExcept(person, email)
		if len(remainingEmails) == 0 {
			continue
		}
		person["emails"] = remainingEmails
		claimedPeople = append(claimedPeople, person)
	}
	claimedPeople = append([]map[string]any{claimedAdminPerson(adminPerson, email)}, claimedPeople...)
	return claimedPeople
}

func isBlueclawAdminPerson(person map[string]any) bool {
	personID, _ := person["personID"].(string)
	if personID == blueclawruntime.BlueclawPolicyAdminID {
		return true
	}
	isAdmin, _ := person["isAdmin"].(bool)
	return isAdmin
}

func claimedAdminPerson(person map[string]any, email string) map[string]any {
	if person == nil {
		person = map[string]any{}
	}
	person["personID"] = blueclawruntime.BlueclawPolicyAdminID
	person["displayName"] = "Intern Kim Admin"
	person["emails"] = []string{email}
	person["circles"] = []string{"staff", "admin"}
	person["securityLevelName"] = "admin"
	person["securityLevelRank"] = 100
	person["grantedClasses"] = []string{"internal", "executive"}
	person["isAdmin"] = true
	return person
}

func blueclawPersonEmailsExcept(person map[string]any, excludedEmail string) []string {
	values, _ := person["emails"].([]any)
	emails := make([]string, 0, len(values))
	for _, value := range values {
		email, ok := value.(string)
		if !ok {
			continue
		}
		normalizedEmail := strings.ToLower(strings.TrimSpace(email))
		if normalizedEmail == "" || normalizedEmail == excludedEmail {
			continue
		}
		emails = append(emails, normalizedEmail)
	}
	return emails
}

func (service *Service) localUpdateOrgProfiles(responseWriter http.ResponseWriter, request *http.Request) {
	service.handleOrganizationProfileUpdate(responseWriter, request)
}

func (service *Service) localSetOrgGroups(responseWriter http.ResponseWriter, request *http.Request) {
	service.handleOrganizationGroupsUpdate(responseWriter, request)
}

func (service *Service) writeFullLocalUsersResponse(responseWriter http.ResponseWriter, request *http.Request) {
	response, errorValue := service.buildLocalUsersResponse(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeLocalUsersResponse(responseWriter, request, response)
}

func policyStringList(value any) []string {
	values, _ := value.([]any)
	result := []string{}
	for _, item := range values {
		stringValue, isString := item.(string)
		if isString {
			result = append(result, stringValue)
		}
	}
	return result
}

func (service *Service) syncMattermostCirclesBestEffort(ctx context.Context) error {
	if !service.hasDeviceAuth() {
		return nil
	}
	token, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return errorValue
	}
	return service.syncMattermostCircleMemberships(ctx, token)
}

func (service *Service) removeBlueclawPerson(ctx context.Context, email string) error {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return nil
	}
	path := "/admin/api/people?email=" + url.QueryEscape(normalizedEmail)
	return service.blueclawJSONRequest(ctx, http.MethodDelete, path, nil, nil)
}

func (service *Service) blueclawJSONRequest(ctx context.Context, method string, path string, body any, responseValue any) error {
	var reader io.Reader
	if body != nil {
		document, errorValue := json.Marshal(body)
		if errorValue != nil {
			return errorValue
		}
		reader = strings.NewReader(string(document))
	}
	requestURL := strings.TrimRight(service.Configuration.BlueclawBaseURL, "/") + path
	request, errorValue := http.NewRequestWithContext(ctx, method, requestURL, reader)
	if errorValue != nil {
		return errorValue
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		if responseValue != nil {
			return json.NewDecoder(response.Body).Decode(responseValue)
		}
		return nil
	}
	responseDocument, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	return fmt.Errorf("Blueclaw %s %s returned %d: %s", method, path, response.StatusCode, strings.TrimSpace(string(responseDocument)))
}

func (service *Service) triggerUsersSync(ctx context.Context) {
	if service.RunCommand == nil {
		if _, errorValue := exec.LookPath("systemctl"); errorValue != nil {
			return
		}
	}
	if _, errorValue := service.runCommand(ctx, "systemctl", "start", "internkim-users-sync.service"); errorValue != nil {
		log.Printf("users sync trigger failed: %v", errorValue)
	}
}

func (service *Service) firstAdminBootstrapPath() string {
	return filepath.Join(service.Configuration.StateDirectory, "first-admin-bootstrap.json")
}

func (service *Service) readFirstAdminBootstrapResult() firstAdminBootstrapResult {
	document, errorValue := os.ReadFile(service.firstAdminBootstrapPath())
	if errorValue != nil {
		return firstAdminBootstrapResult{}
	}
	var result firstAdminBootstrapResult
	if errorValue := json.Unmarshal(document, &result); errorValue != nil {
		return firstAdminBootstrapResult{}
	}
	return result
}

func (service *Service) writeFirstAdminBootstrapResult(result firstAdminBootstrapResult) firstAdminBootstrapResult {
	if result.Status == "" {
		result.Status = firstAdminBootstrapPending
	}
	if result.Status == firstAdminBootstrapIdentityMissing || result.Status == firstAdminBootstrapRejected {
		return result
	}
	if errorValue := os.MkdirAll(service.Configuration.StateDirectory, 0o700); errorValue != nil {
		log.Printf("first admin bootstrap status write failed: %v", errorValue)
		return result
	}
	document, errorValue := json.Marshal(result)
	if errorValue != nil {
		log.Printf("first admin bootstrap status marshal failed: %v", errorValue)
		return result
	}
	if errorValue := os.WriteFile(service.firstAdminBootstrapPath(), document, 0o600); errorValue != nil {
		log.Printf("first admin bootstrap status write failed: %v", errorValue)
	}
	return result
}

func (service *Service) firstAdminPasswordPath() string {
	return filepath.Join(service.Configuration.StateDirectory, "first-admin-password.json")
}

func (service *Service) writeFirstAdminPassword(email string, password string) error {
	if errorValue := os.MkdirAll(service.Configuration.StateDirectory, 0o700); errorValue != nil {
		return errorValue
	}
	document, errorValue := json.Marshal(firstAdminPasswordDocument{
		Email:    strings.ToLower(strings.TrimSpace(email)),
		Password: password,
	})
	if errorValue != nil {
		return errorValue
	}
	return os.WriteFile(service.firstAdminPasswordPath(), document, 0o600)
}

func (service *Service) consumeFirstAdminPassword(email string) firstAdminPasswordDocument {
	path := service.firstAdminPasswordPath()
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return firstAdminPasswordDocument{}
	}
	var passwordDocument firstAdminPasswordDocument
	if errorValue := json.Unmarshal(document, &passwordDocument); errorValue != nil {
		return firstAdminPasswordDocument{}
	}
	if !strings.EqualFold(passwordDocument.Email, email) {
		return firstAdminPasswordDocument{}
	}
	_ = os.Remove(path)
	return passwordDocument
}

func (service *Service) writeUserRole(ctx context.Context, fleetID string, fleetSecret string, email string, role string) error {
	payload := map[string]string{
		"fleet_id": fleetID,
		"email":    strings.ToLower(strings.TrimSpace(email)),
		"role":     normalizeAdminUserRole(role),
		"handle":   normalizeMattermostHandle(mattermostUsernameBase(email)),
		"name":     firstNonEmpty(strings.TrimSpace(email), "Admin"),
	}
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		return errorValue
	}
	requestURL := strings.TrimRight(service.Configuration.APIBaseURL, "/") + "/api/users"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, strings.NewReader(string(document)))
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-InternKim-Fleet-ID", fleetID)
	request.Header.Set("X-InternKim-Fleet-Secret", fleetSecret)
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	responseDocument, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	return fmt.Errorf("write user role returned %d: %s", response.StatusCode, strings.TrimSpace(string(responseDocument)))
}

func (service *Service) writeClaimedAdminRole(ctx context.Context, email string) error {
	fleetID := strings.ToLower(strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)))
	fleetSecret := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetSecretPath))
	if fleetID == "" || fleetSecret == "" {
		return fmt.Errorf("device auth is not configured")
	}
	return service.writeUserRole(ctx, fleetID, fleetSecret, email, "admin")
}

func (service *Service) currentUserRecords(ctx context.Context) ([]adminUserMutation, error) {
	fleetID := strings.ToLower(strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)))
	fleetSecret := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetSecretPath))
	if fleetID == "" || fleetSecret == "" {
		return nil, fmt.Errorf("device auth is not configured")
	}
	return service.lookupUserRecords(ctx, fleetID, fleetSecret)
}

func (service *Service) hasCurrentAdminUsers(ctx context.Context) (bool, error) {
	records, errorValue := service.currentUserRecords(ctx)
	if errorValue != nil {
		return false, errorValue
	}
	for _, record := range records {
		if record.Role == "admin" {
			return true, nil
		}
	}
	return false, nil
}

func (service *Service) isCurrentAdminEmail(ctx context.Context, callerEmail string) bool {
	if strings.TrimSpace(callerEmail) == "" {
		return false
	}
	records, errorValue := service.currentUserRecords(ctx)
	if errorValue != nil {
		return false
	}
	for _, record := range records {
		if record.Role == "admin" && strings.EqualFold(record.Email, callerEmail) {
			return true
		}
	}
	return false
}

func (service *Service) currentAdminUserRole(ctx context.Context, callerEmail string) string {
	if strings.TrimSpace(callerEmail) == "" {
		return adminUserRoleMember
	}
	records, errorValue := service.currentUserRecords(ctx)
	if errorValue != nil {
		return adminUserRoleMember
	}
	for _, record := range records {
		if strings.EqualFold(record.Email, callerEmail) {
			return normalizeAdminUserRole(record.Role)
		}
	}
	return adminUserRoleMember
}

func isLocalRequest(request *http.Request) bool {
	host, _, splitError := net.SplitHostPort(request.RemoteAddr)
	if splitError != nil {
		host = request.RemoteAddr
	}
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}

func (service *Service) dumpMattermostDatabase(ctx context.Context) (string, error) {
	dumpPath := filepath.Join(os.TempDir(), "internkim-mattermost-"+randomHex(8)+".sql")
	command := "pg_dump mattermost > " + shellQuote(dumpPath)
	_, errorValue := service.runCommand(ctx, "su", "-", "postgres", "-c", command)
	if errorValue != nil {
		return "", errorValue
	}
	return dumpPath, nil
}

func (service *Service) dumpBlueclawDatabase(ctx context.Context) (string, error) {
	output, errorValue := service.runCommand(ctx, "su", "-", "postgres", "-c", "psql -tAc "+shellQuote("SELECT 1 FROM pg_database WHERE datname='blueclaw'"))
	if errorValue != nil {
		return "", nil
	}
	exists := strings.TrimSpace(string(output))
	if exists != "1" {
		return "", nil
	}
	dumpPath := filepath.Join(os.TempDir(), "internkim-blueclaw-"+randomHex(8)+".sql")
	command := "pg_dump blueclaw > " + shellQuote(dumpPath)
	_, errorValue = service.runCommand(ctx, "su", "-", "postgres", "-c", command)
	if errorValue != nil {
		return "", errorValue
	}
	return dumpPath, nil
}

func (service *Service) prepareBlueclawBackup(ctx context.Context) (map[string]any, func()) {
	manifest := service.fetchBlueclawManifest(ctx)
	if manifest == nil {
		return nil, func() {}
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, service.Configuration.BlueclawBaseURL+"/admin/api/backup/prepare", strings.NewReader(`{"holder":"internkim-admind"}`))
	if errorValue != nil {
		return manifest, func() {}
	}
	request.Header.Set("Content-Type", "application/json")
	client := service.httpClient()
	response, errorValue := client.Do(request)
	if errorValue != nil {
		return manifest, func() {}
	}
	_ = response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return manifest, func() {}
	}
	return manifest, func() {
		completeRequest, errorValue := http.NewRequestWithContext(context.Background(), http.MethodPost, service.Configuration.BlueclawBaseURL+"/admin/api/backup/complete", nil)
		if errorValue != nil {
			return
		}
		completeResponse, errorValue := client.Do(completeRequest)
		if errorValue == nil {
			_ = completeResponse.Body.Close()
		}
	}
}

func (service *Service) fetchBlueclawManifest(ctx context.Context) map[string]any {
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, service.Configuration.BlueclawBaseURL+"/admin/api/backup/manifest", nil)
	if errorValue != nil {
		return nil
	}
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return nil
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil
	}
	var manifest map[string]any
	if errorValue := json.NewDecoder(response.Body).Decode(&manifest); errorValue != nil {
		return nil
	}
	return manifest
}

func (service *Service) httpClient() *http.Client {
	if service.HTTPClient != nil {
		return service.HTTPClient
	}
	return admindHTTPClient
}

var admindHTTPClient = &http.Client{Transport: newAdmindHTTPTransport()}

func newAdmindHTTPTransport() http.RoundTripper {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 30 * time.Second
	return transport
}

func (service *Service) runCommand(ctx context.Context, name string, arguments ...string) ([]byte, error) {
	if service.RunCommand != nil {
		return service.RunCommand(ctx, name, arguments...)
	}
	commandContext, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	command := exec.CommandContext(commandContext, name, arguments...)
	return command.CombinedOutput()
}

func (service *Service) writeJSON(responseWriter http.ResponseWriter, value any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(responseWriter).Encode(value)
}

func readLowerTrimmedFirstExistingFile(paths ...string) string {
	for _, path := range paths {
		value := strings.ToLower(strings.TrimSpace(readTrimmedFile(path)))
		if value != "" {
			return value
		}
	}
	return ""
}

func legacyAdminEmailPaths(path string) []string {
	if filepath.Base(filepath.Dir(path)) != "config" || filepath.Base(path) != "admin-email" {
		return nil
	}
	return []string{filepath.Join(filepath.Dir(filepath.Dir(path)), "admin-email")}
}

func legacyClaimedAdminEmailPaths(path string) []string {
	if filepath.Base(filepath.Dir(path)) != "admin" || filepath.Base(path) != "claimed-admin-email" {
		return nil
	}
	statePath := filepath.Dir(filepath.Dir(path))
	if filepath.Base(statePath) != "state" {
		return nil
	}
	return []string{filepath.Join(filepath.Dir(statePath), "claimed-admin-email")}
}

func (configuration Configuration) withDefaults() Configuration {
	defaultConfiguration := DefaultConfiguration()
	if configuration.ListenAddress == "" {
		configuration.ListenAddress = defaultConfiguration.ListenAddress
	}
	if configuration.MattermostBaseURL == "" {
		configuration.MattermostBaseURL = defaultConfiguration.MattermostBaseURL
	}
	if configuration.APIBaseURL == "" {
		configuration.APIBaseURL = defaultConfiguration.APIBaseURL
	}
	if configuration.BlueclawBaseURL == "" {
		configuration.BlueclawBaseURL = defaultConfiguration.BlueclawBaseURL
	}
	if configuration.CapabilitySocketPath == "" {
		configuration.CapabilitySocketPath = defaultConfiguration.CapabilitySocketPath
	}
	if configuration.StateDirectory == "" {
		configuration.StateDirectory = defaultConfiguration.StateDirectory
	}
	configuration.DatabasePath = resolvedStateDatabasePath(configuration, defaultConfiguration)
	if configuration.CompanionJobPath == "" {
		if configuration.StateDirectory == defaultConfiguration.StateDirectory {
			configuration.CompanionJobPath = defaultConfiguration.CompanionJobPath
		} else {
			configuration.CompanionJobPath = filepath.Join(configuration.StateDirectory, "companion-jobs.json")
		}
	}
	if configuration.FlowDatabasePath == "" {
		if configuration.CompanionJobPath == defaultConfiguration.CompanionJobPath {
			configuration.FlowDatabasePath = defaultConfiguration.FlowDatabasePath
		} else {
			configuration.FlowDatabasePath = filepath.Join(filepath.Dir(configuration.CompanionJobPath), "flow.sqlite")
		}
	}
	if configuration.CalendarDatabasePath == "" {
		if configuration.CompanionJobPath == defaultConfiguration.CompanionJobPath {
			configuration.CalendarDatabasePath = defaultConfiguration.CalendarDatabasePath
		} else {
			configuration.CalendarDatabasePath = filepath.Join(filepath.Dir(configuration.CompanionJobPath), "calendar.sqlite")
		}
	}
	if configuration.MailDatabasePath == "" {
		if configuration.CompanionJobPath == defaultConfiguration.CompanionJobPath {
			configuration.MailDatabasePath = defaultConfiguration.MailDatabasePath
		} else {
			configuration.MailDatabasePath = filepath.Join(filepath.Dir(configuration.CompanionJobPath), "mail.sqlite")
		}
	}
	if configuration.AttendanceDatabasePath == "" {
		if configuration.CompanionJobPath == defaultConfiguration.CompanionJobPath {
			configuration.AttendanceDatabasePath = defaultConfiguration.AttendanceDatabasePath
		} else {
			configuration.AttendanceDatabasePath = filepath.Join(filepath.Dir(configuration.CompanionJobPath), "attendance.sqlite")
		}
	}
	if configuration.BridgeMapDatabasePath == "" {
		if configuration.CompanionJobPath == defaultConfiguration.CompanionJobPath {
			configuration.BridgeMapDatabasePath = defaultConfiguration.BridgeMapDatabasePath
		} else {
			configuration.BridgeMapDatabasePath = filepath.Join(filepath.Dir(configuration.CompanionJobPath), "bridge-map.sqlite")
		}
	}
	if configuration.MattermostAdminPasswordPath == "" {
		configuration.MattermostAdminPasswordPath = defaultConfiguration.MattermostAdminPasswordPath
	}
	if configuration.MattermostTokenPath == "" {
		configuration.MattermostTokenPath = defaultConfiguration.MattermostTokenPath
	}
	if configuration.MattermostInteractiveTokenPath == "" {
		configuration.MattermostInteractiveTokenPath = defaultConfiguration.MattermostInteractiveTokenPath
	}
	if configuration.MattermostOAuthClientPath == "" {
		configuration.MattermostOAuthClientPath = defaultConfiguration.MattermostOAuthClientPath
	}
	if configuration.OpenRouterKeyPath == "" {
		configuration.OpenRouterKeyPath = defaultConfiguration.OpenRouterKeyPath
	}
	if configuration.OpenRouterModelsURL == "" {
		configuration.OpenRouterModelsURL = defaultConfiguration.OpenRouterModelsURL
	}
	if configuration.ReleaseRegistryURL == "" {
		configuration.ReleaseRegistryURL = defaultConfiguration.ReleaseRegistryURL
	}
	if configuration.ReleaseDownloadTokenPath == "" {
		configuration.ReleaseDownloadTokenPath = defaultConfiguration.ReleaseDownloadTokenPath
	}
	if configuration.ReleaseSigningKeyPath == "" {
		configuration.ReleaseSigningKeyPath = defaultConfiguration.ReleaseSigningKeyPath
	}
	if configuration.MattermostBotTokenPath == "" {
		configuration.MattermostBotTokenPath = defaultConfiguration.MattermostBotTokenPath
	}
	if configuration.AdminEmailPath == "" {
		configuration.AdminEmailPath = defaultConfiguration.AdminEmailPath
	}
	if configuration.ClaimedAdminEmailPath == "" {
		configuration.ClaimedAdminEmailPath = defaultConfiguration.ClaimedAdminEmailPath
	}
	if configuration.FleetIDPath == "" {
		configuration.FleetIDPath = defaultConfiguration.FleetIDPath
	}
	if configuration.DeviceURLPath == "" {
		configuration.DeviceURLPath = defaultConfiguration.DeviceURLPath
	}
	if configuration.FleetSecretPath == "" {
		configuration.FleetSecretPath = defaultConfiguration.FleetSecretPath
	}
	if configuration.AdminUIPath == "" {
		configuration.AdminUIPath = defaultConfiguration.AdminUIPath
	}
	if configuration.RepositoryRoot == "" {
		configuration.RepositoryRoot = defaultConfiguration.RepositoryRoot
	}
	if configuration.CompanionFileDirectory == "" {
		configuration.CompanionFileDirectory = defaultConfiguration.CompanionFileDirectory
	}
	if configuration.SitesRoot == "" {
		configuration.SitesRoot = defaultConfiguration.SitesRoot
	}
	if configuration.FontsDirectory == "" {
		configuration.FontsDirectory = defaultConfiguration.FontsDirectory
	}
	if configuration.SiteSecretDirectory == "" {
		configuration.SiteSecretDirectory = defaultConfiguration.SiteSecretDirectory
	}
	if configuration.SiteSystemdDirectory == "" {
		configuration.SiteSystemdDirectory = defaultConfiguration.SiteSystemdDirectory
	}
	if configuration.BotProfilePath == "" {
		if configuration.CompanionJobPath == defaultConfiguration.CompanionJobPath {
			configuration.BotProfilePath = defaultConfiguration.BotProfilePath
		} else {
			configuration.BotProfilePath = filepath.Join(filepath.Dir(configuration.CompanionJobPath), "bot-profile.yaml")
		}
	}
	if configuration.BotProfileImagePath == "" {
		configuration.BotProfileImagePath = defaultConfiguration.BotProfileImagePath
	}
	if configuration.BotUsername == "" {
		configuration.BotUsername = defaultConfiguration.BotUsername
	}
	if configuration.BlueclawWorkspacePath == "" {
		configuration.BlueclawWorkspacePath = defaultConfiguration.BlueclawWorkspacePath
	}
	return configuration
}

func backupIncludedPaths() []string {
	return []string{
		"/root/.internkim/env",
		"/root/.internkim/config",
		"/root/.internkim/secrets",
		"/root/.internkim/state",
		"/root/.internkim/sites",
		"/root/.blueclaw/config",
		"/root/.blueclaw/workspace",
		"/var/lib/blueclaw/workspace.ext4",
		"/opt/mattermost/config",
		"/opt/mattermost/data",
		"/etc/cloudflared",
	}
}

func addManifestToTar(tarWriter *tar.Writer, manifest *BackupManifest) error {
	document, errorValue := json.MarshalIndent(manifest, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	return addBytesToTar(tarWriter, "manifest.json", document)
}

func addPathToTar(tarWriter *tar.Writer, includedPath string, manifest *BackupManifest) error {
	fileInfo, errorValue := os.Stat(includedPath)
	if errorValue != nil {
		return errorValue
	}
	if fileInfo.IsDir() {
		return filepath.Walk(includedPath, func(path string, information os.FileInfo, walkError error) error {
			if walkError != nil || information.IsDir() {
				return walkError
			}
			return addFileToTar(tarWriter, path, tarName(path), manifest)
		})
	}
	return addFileToTar(tarWriter, includedPath, tarName(includedPath), manifest)
}

func addNamedFileToTar(tarWriter *tar.Writer, filePath string, tarPath string, manifest *BackupManifest) error {
	return addFileToTar(tarWriter, filePath, tarPath, manifest)
}

func addFileToTar(tarWriter *tar.Writer, filePath string, tarPath string, manifest *BackupManifest) error {
	fileInfo, errorValue := os.Stat(filePath)
	if errorValue != nil {
		return errorValue
	}
	if !fileInfo.Mode().IsRegular() {
		return nil
	}
	file, errorValue := os.Open(filePath)
	if errorValue != nil {
		return errorValue
	}
	defer file.Close()
	header, errorValue := tar.FileInfoHeader(fileInfo, "")
	if errorValue != nil {
		return errorValue
	}
	header.Name = tarPath
	if errorValue := tarWriter.WriteHeader(header); errorValue != nil {
		return errorValue
	}
	hasher := sha256.New()
	_, errorValue = io.Copy(tarWriter, io.TeeReader(file, hasher))
	if errorValue != nil {
		return errorValue
	}
	manifest.Checksums[tarPath] = hex.EncodeToString(hasher.Sum(nil))
	return nil
}

func addBytesToTar(tarWriter *tar.Writer, tarPath string, document []byte) error {
	header := &tar.Header{Name: tarPath, Mode: 0o600, Size: int64(len(document)), ModTime: time.Now()}
	if errorValue := tarWriter.WriteHeader(header); errorValue != nil {
		return errorValue
	}
	_, errorValue := tarWriter.Write(document)
	return errorValue
}

func extractBundle(bundlePath string, targetDirectoryPath string) (*BackupManifest, error) {
	bundleFile, errorValue := os.Open(bundlePath)
	if errorValue != nil {
		return nil, errorValue
	}
	defer bundleFile.Close()
	gzipReader, errorValue := gzip.NewReader(bundleFile)
	if errorValue != nil {
		return nil, errorValue
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	var manifest *BackupManifest
	for {
		header, nextErrorValue := tarReader.Next()
		if nextErrorValue == io.EOF {
			break
		}
		if nextErrorValue != nil {
			return nil, nextErrorValue
		}
		if !isSafeTarPath(header.Name) {
			return nil, errors.New("unsafe backup path: " + header.Name)
		}
		targetPath := filepath.Join(targetDirectoryPath, header.Name)
		if header.Name == "manifest.json" {
			document, errorValue := io.ReadAll(tarReader)
			if errorValue != nil {
				return nil, errorValue
			}
			var parsedManifest BackupManifest
			if errorValue := json.Unmarshal(document, &parsedManifest); errorValue != nil {
				return nil, errorValue
			}
			manifest = &parsedManifest
			continue
		}
		if errorValue := os.MkdirAll(filepath.Dir(targetPath), 0o755); errorValue != nil {
			return nil, errorValue
		}
		file, errorValue := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
		if errorValue != nil {
			return nil, errorValue
		}
		_, copyErrorValue := io.Copy(file, tarReader)
		closeErrorValue := file.Close()
		if copyErrorValue != nil {
			return nil, copyErrorValue
		}
		if closeErrorValue != nil {
			return nil, closeErrorValue
		}
	}
	if manifest == nil {
		return nil, errors.New("backup manifest is missing")
	}
	return manifest, nil
}

func encryptFile(inputPath string, outputPath string, passphrase string) error {
	plainDocument, errorValue := os.ReadFile(inputPath)
	if errorValue != nil {
		return errorValue
	}
	salt := randomBytes(16)
	nonce := randomBytes(12)
	key := deriveKey([]byte(passphrase), salt, 200000, 32)
	block, errorValue := aes.NewCipher(key)
	if errorValue != nil {
		return errorValue
	}
	aead, errorValue := cipher.NewGCM(block)
	if errorValue != nil {
		return errorValue
	}
	ciphertext := aead.Seal(nil, nonce, plainDocument, []byte("internkim-backup-v1"))
	outputDocument := append([]byte("IKBAK1\n"), salt...)
	outputDocument = append(outputDocument, nonce...)
	outputDocument = append(outputDocument, ciphertext...)
	return os.WriteFile(outputPath, outputDocument, 0o600)
}

func decryptFile(inputPath string, outputPath string, passphrase string) error {
	encryptedDocument, errorValue := os.ReadFile(inputPath)
	if errorValue != nil {
		return errorValue
	}
	if len(encryptedDocument) < len("IKBAK1\n")+16+12 || string(encryptedDocument[:7]) != "IKBAK1\n" {
		return errors.New("backup format is not supported")
	}
	offset := 7
	salt := encryptedDocument[offset : offset+16]
	offset += 16
	nonce := encryptedDocument[offset : offset+12]
	offset += 12
	ciphertext := encryptedDocument[offset:]
	key := deriveKey([]byte(passphrase), salt, 200000, 32)
	block, errorValue := aes.NewCipher(key)
	if errorValue != nil {
		return errorValue
	}
	aead, errorValue := cipher.NewGCM(block)
	if errorValue != nil {
		return errorValue
	}
	plainDocument, errorValue := aead.Open(nil, nonce, ciphertext, []byte("internkim-backup-v1"))
	if errorValue != nil {
		return errors.New("backup passphrase is incorrect or bundle is corrupted")
	}
	return os.WriteFile(outputPath, plainDocument, 0o600)
}

func deriveKey(password []byte, salt []byte, iterations int, keyLength int) []byte {
	var derivedKey []byte
	var block []byte
	blockIndex := 1
	for len(derivedKey) < keyLength {
		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		mac.Write([]byte{byte(blockIndex >> 24), byte(blockIndex >> 16), byte(blockIndex >> 8), byte(blockIndex)})
		block = mac.Sum(nil)
		accumulator := append([]byte{}, block...)
		for iteration := 1; iteration < iterations; iteration++ {
			mac = hmac.New(sha256.New, password)
			mac.Write(block)
			block = mac.Sum(nil)
			for index := range accumulator {
				accumulator[index] ^= block[index]
			}
		}
		derivedKey = append(derivedKey, accumulator...)
		blockIndex++
	}
	return derivedKey[:keyLength]
}

func copyDirectory(sourceRoot string, targetRoot string) error {
	return filepath.Walk(sourceRoot, func(sourcePath string, information os.FileInfo, walkError error) error {
		if walkError != nil {
			return walkError
		}
		relativePath, errorValue := filepath.Rel(sourceRoot, sourcePath)
		if errorValue != nil || relativePath == "." {
			return errorValue
		}
		targetPath := filepath.Join(targetRoot, relativePath)
		if information.IsDir() {
			return os.MkdirAll(targetPath, information.Mode())
		}
		if !information.Mode().IsRegular() {
			return nil
		}
		if errorValue := os.MkdirAll(filepath.Dir(targetPath), 0o755); errorValue != nil {
			return errorValue
		}
		sourceFile, errorValue := os.Open(sourcePath)
		if errorValue != nil {
			return errorValue
		}
		defer sourceFile.Close()
		targetFile, errorValue := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, information.Mode())
		if errorValue != nil {
			return errorValue
		}
		_, copyErrorValue := io.Copy(targetFile, sourceFile)
		closeErrorValue := targetFile.Close()
		if copyErrorValue != nil {
			return copyErrorValue
		}
		return closeErrorValue
	})
}

func copyRegularFile(sourcePath string, targetPath string) error {
	sourceFile, errorValue := os.Open(sourcePath)
	if errorValue != nil {
		return errorValue
	}
	defer sourceFile.Close()
	sourceInfo, errorValue := sourceFile.Stat()
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(targetPath), 0o755); errorValue != nil {
		return errorValue
	}
	targetFile, errorValue := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, sourceInfo.Mode())
	if errorValue != nil {
		return errorValue
	}
	_, copyErrorValue := io.Copy(targetFile, sourceFile)
	closeErrorValue := targetFile.Close()
	if copyErrorValue != nil {
		return copyErrorValue
	}
	return closeErrorValue
}

func isAllowedOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	parsedURL, errorValue := url.Parse(origin)
	if errorValue != nil {
		return false
	}
	host := strings.ToLower(parsedURL.Hostname())
	return host == "example.test" || strings.HasSuffix(host, ".example.test") || host == "localhost" || host == "127.0.0.1"
}

func isSafeTarPath(path string) bool {
	cleanPath := filepath.Clean(path)
	return cleanPath == path && !strings.HasPrefix(cleanPath, "..") && !filepath.IsAbs(cleanPath)
}

func parseRestoreUploadChunkPath(path string) (string, int, bool) {
	trimmedPath := strings.TrimPrefix(path, "/restore/uploads/")
	parts := strings.Split(trimmedPath, "/chunks/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", 0, false
	}
	chunkIndex, errorValue := strconv.Atoi(parts[1])
	if errorValue != nil || chunkIndex < 0 {
		return "", 0, false
	}
	return parts[0], chunkIndex, true
}

func tarName(path string) string {
	return strings.TrimPrefix(filepath.ToSlash(filepath.Clean(path)), "/")
}

func randomHex(size int) string {
	return hex.EncodeToString(randomBytes(size))
}

func randomBytes(size int) []byte {
	value := make([]byte, size)
	_, _ = rand.Read(value)
	return value
}

func readTrimmedFile(path string) string {
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return ""
	}
	return strings.TrimSpace(string(document))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue != "" {
			return trimmedValue
		}
	}
	return ""
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func Run(configuration Configuration) error {
	ctx := context.Background()
	service := NewService(configuration)
	log.Printf("internkim admind listening on %s", service.Configuration.ListenAddress)
	return service.Run(ctx)
}
