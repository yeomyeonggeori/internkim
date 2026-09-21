package admind

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"

	"gitlab.com/eastriver/internkim/internal/mail"

	"strings"
	"sync"
	"time"

	"gitlab.com/eastriver/internkim/internal/buzzimport/mattermostadmin"
	"gitlab.com/eastriver/internkim/internal/centralplane"
)

var BuildID = "unknown"
var GitRevision = "unknown"

type Service struct {
	Configuration Configuration
	HTTPClient    *http.Client
	RunCommand    func(context.Context, string, ...string) ([]byte, error)

	mutex                      sync.Mutex
	adminSeating               sync.Mutex
	centralPlaneOnce           sync.Once
	siteScaffoldOnce           sync.Once
	siteScaffoldDocuments      []siteScaffoldDocument
	siteScaffoldError          error
	centralPlaneClient         *centralplane.Client
	mattermostAdminOnce        sync.Once
	mattermostAdminClient      *mattermostadmin.Client
	jobs                       map[string]*Job
	uploads                    map[string]*RestoreUpload
	blueclawUpdateUploads      map[string]*BlueclawUpdateUpload
	pairingCodes               map[string]*CompanionPairingCode
	companions                 map[string]*CompanionRecord
	companionJobs              map[string]*CompanionJob
	companionFileUploads       map[string]*CompanionFileUpload
	buzzInviteStore            *buzzInviteStore
	buzzInviteStoreOnce        sync.Once
	buzzKeySeedOnce            sync.Once
	buzzKeySeedValue           string
	cloudflareAccessOnce       sync.Once
	cloudflareAccessCheck      *cloudflareAccessVerifier
	sites                      map[string]*SiteRecord
	siteRuntimeMutex           sync.Mutex
	siteRuntimeActivities      map[string]*siteRuntimeActivity
	siteRuntimeStartupDone     <-chan struct{}
	mailBackend                mail.Backend
	calendarDeleteIntentWakeUp chan struct{}
	calendarStoreWriteMutex    sync.Mutex
	companyShareMutex          sync.Mutex
	companyShareAttempts       map[string]companyShareAttempt
	companySettingsCache       heldCompanySettings
	policyRecordCacheMutex     sync.Mutex
	policyRecordCache          []adminUserMutation
	requestMetrics             *adminRequestMetrics
	databaseSchemas            *adminDatabaseSchemas
	buzzDatabaseOwner          buzzDatabaseHandle
	legacyDatabaseMigration    sync.Once
	mailNotifyMarkMutex        sync.Mutex
	taskNotifyMarkMutex        sync.Mutex
	removeTokenQuarantineFile  func(string) error
	promoteCalendarTokenFile   func(string, string) error
	startedAt                  time.Time
}

func NewService(configuration Configuration) *Service {
	configuration = configuration.withDefaults()
	service := &Service{
		Configuration:         configuration,
		jobs:                  map[string]*Job{},
		uploads:               map[string]*RestoreUpload{},
		blueclawUpdateUploads: map[string]*BlueclawUpdateUpload{},
		pairingCodes:          map[string]*CompanionPairingCode{},
		companions:            map[string]*CompanionRecord{},
		companionJobs:         map[string]*CompanionJob{},
		companionFileUploads:  map[string]*CompanionFileUpload{},
		sites:                 map[string]*SiteRecord{},
		mailBackend:           mail.StandardBackend{},
		companyShareAttempts:  map[string]companyShareAttempt{},
		requestMetrics:        newAdminRequestMetrics(),
		databaseSchemas:       newAdminDatabaseSchemas(),
		startedAt:             time.Now().UTC(),
	}
	service.loadCompanions()
	service.loadCompanionJobs()
	service.loadSites()
	return service
}

func (service *Service) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	service.reconcileSiteSourcesToMemberCircle()
	service.startSiteRuntimeReconcile(ctx)
	handler := service.router()
	server := &http.Server{
		Addr:    service.Configuration.ListenAddress,
		Handler: handler,
	}
	socketServer := &http.Server{Handler: markRequestsAsAssertedByTheListener(handler)}
	listener, errorValue := net.Listen("tcp", server.Addr)
	if errorValue != nil {
		return errorValue
	}
	defer listener.Close()
	if errorValue := service.startRequesterSocketListener(socketServer); errorValue != nil {
		return errorValue
	}
	log.Printf("internkim admind listening on %s", listener.Addr())
	go func() {
		<-ctx.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
		_ = socketServer.Shutdown(shutdownContext)
	}()
	service.startBackgroundWork(ctx)
	errorValue = server.Serve(listener)
	if errorValue != nil && !errors.Is(errorValue, http.ErrServerClosed) {
		return errorValue
	}
	return nil
}

func (service *Service) startBackgroundWork(ctx context.Context) {
	go service.reconcileBlueclawRuntimeConfiguration(ctx)
	go service.centralPlane()
	go service.keepUsersSyncInstalled(ctx)
	if service.Configuration.TaskRunNotifyEnabled {
		go service.keepTaskRunsNotified(ctx)
	}
	if service.Configuration.MailNotifyEnabled {
		go service.keepMailAnnounced(ctx)
	}
	service.sweepUpdateLeftovers()
	service.startPersonaSync(ctx)
	service.startCompanionFileCleanup(ctx)
	service.startBlueclawRosterReconcile(ctx)
	service.startCalendarSweep(ctx)
	service.startOrganizationSweep(ctx)
	service.startCompanyProfileSweep(ctx)
	service.startCompanyLedgerSweep(ctx)
	service.startAttendanceSweep(ctx)
	service.startCRMSweep(ctx)
	service.startMailAccountSweep(ctx)
	service.startTaskSweep(ctx)
	service.startSiteRuntimeJanitor(ctx)
	service.startScheduledBackups(ctx)
	service.startBuzzMemberLinker(ctx)
	service.startBuzzCredentialSweep(ctx)
	go service.sayIfTheRelayIsOpen(ctx)
	service.startBuzzAccountLinkSync(ctx)
	service.startMemberChannelMembershipSync(ctx)
	service.startCircleRoomMembershipSync(ctx)
	service.startAdminChannelSeatSync(ctx)
	service.ensureBuzzRelayTerminator()
	service.warnWhenFontAssetsMissing()
}

func (service *Service) startRequesterSocketListener(socketServer *http.Server) error {
	socketPath := strings.TrimSpace(service.Configuration.ListenSocketPath)
	if socketPath == "" {
		log.Printf("admind has no requester socket path, so callers that assert a requester have no way in")
		return nil
	}
	listener, errorValue := listenOnRequesterSocket(socketPath)
	if errorValue != nil {
		log.Printf("admind could not open its requester socket at %s: %v", socketPath, errorValue)
		return errorValue
	}
	go func() {
		if serveError := socketServer.Serve(listener); serveError != nil && !errors.Is(serveError, http.ErrServerClosed) {
			log.Printf("admind requester socket at %s stopped: %v", socketPath, serveError)
		}
	}()
	return nil
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
	return service.Run(ctx)
}
