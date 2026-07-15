package localfleet

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

type Service struct {
	options Options
	client  *http.Client
}

func NewService(options Options) (Service, error) {
	normalizedOptions, errorValue := normalizeOptions(options)
	if errorValue != nil {
		return Service{}, errorValue
	}
	return Service{
		options: normalizedOptions,
		client:  &http.Client{Timeout: 3 * time.Second},
	}, nil
}

func (service Service) Status(contextValue context.Context) Status {
	adminURL := service.adminHostURL()
	mattermostURL := service.mattermostHostURL()
	if errorValue := service.EnsureConfiguration(); errorValue != nil {
		return Status{
			CheckedAt: time.Now(),
			VirtualMachine: EndpointStatus{
				State:   "failed",
				Message: "configuration failed: " + errorValue.Error(),
			},
			SSH:           EndpointStatus{State: "unknown", Message: "VM status unavailable"},
			Admin:         EndpointStatus{State: "unknown", Message: "configuration unavailable"},
			Mattermost:    EndpointStatus{State: "unknown", Message: "configuration unavailable"},
			AdminURL:      adminURL,
			MattermostURL: mattermostURL,
			LastResult:    readTrimmedFile(service.lastResultPath()),
			CleanupNeeded: service.cleanupNeeded(),
			StatePath:     service.options.StateRootPath,
		}
	}
	virtualMachineState := service.readCommandState(contextValue, service.labCommand("status"))
	return Status{
		CheckedAt:      time.Now(),
		VirtualMachine: virtualMachineState,
		SSH:            service.sshStatus(contextValue, virtualMachineState),
		Admin:          service.httpStatus(contextValue, adminURL+"/admin/api/health", "admind"),
		Mattermost:     service.httpStatus(contextValue, mattermostURL+"/api/v4/system/ping", "mattermost"),
		AdminURL:       adminURL,
		MattermostURL:  mattermostURL,
		LastResult:     readTrimmedFile(service.lastResultPath()),
		CleanupNeeded:  service.cleanupNeeded(),
		StatePath:      service.options.StateRootPath,
	}
}

func (service Service) Run(contextValue context.Context, logger Logger, request JobRequest) error {
	if !service.options.IsEphemeral {
		return service.runAction(contextValue, logger, request)
	}
	service.logEphemeralContext(logger, request)
	service.reapOrphanedEphemeralContainers(contextValue, logger)
	if !request.KeepArtifacts {
		return service.runWithEphemeralCleanup(contextValue, logger, request)
	}
	return service.runAction(contextValue, logger, request)
}

func (service Service) ConfigurationPath() string {
	return service.configurationPath()
}

func (service Service) reapOrphanedEphemeralContainers(contextValue context.Context, logger Logger) {
	reapContext, cancel := context.WithTimeout(contextValue, time.Minute)
	defer cancel()
	plan := service.shellPlan("reap orphaned ephemeral containers", service.reapOrphanedEphemeralContainersCommand())
	if errorValue := service.runCleanupPlans(reapContext, logger, []CommandPlan{plan}); errorValue != nil {
		logger.Info("orphaned container reap skipped: " + errorValue.Error())
	}
}

func (service Service) CleanupEphemeral(contextValue context.Context, logger Logger) error {
	cleanupContext, cancel := newEphemeralCleanupContext(contextValue)
	defer cancel()
	return service.runCleanupPlans(cleanupContext, logger, service.ephemeralCleanupPlans())
}

func (service Service) runWithEphemeralCleanup(contextValue context.Context, logger Logger, request JobRequest) error {
	errorValue := service.runAction(contextValue, logger, request)
	cleanupError := service.CleanupEphemeral(contextValue, logger)
	if errorValue != nil {
		if cleanupError != nil {
			return fmt.Errorf("%w; cleanup failed: %v", errorValue, cleanupError)
		}
		return errorValue
	}
	return cleanupError
}

func newEphemeralCleanupContext(contextValue context.Context) (context.Context, context.CancelFunc) {
	if contextValue.Err() == nil {
		return context.WithTimeout(contextValue, 5*time.Minute)
	}
	return context.WithTimeout(context.Background(), 5*time.Minute)
}

func (service Service) runAction(contextValue context.Context, logger Logger, request JobRequest) error {
	switch request.Action {
	case ActionUp:
		return service.runPlans(contextValue, logger, service.upPlans(request.SkipWeb))
	case ActionDown:
		return service.runPlans(contextValue, logger, service.downPlans())
	case ActionReset:
		return service.runPlans(contextValue, logger, service.resetPlans())
	case ActionRunRecipe:
		if request.WithoutMattermost {
			return errors.New("without-mattermost mode requires --scenario")
		}
		return service.RunRecipe(contextValue, logger, firstNonEmpty(request.Recipe, DefaultRecipe))
	case ActionRunScenario:
		return service.RunScenario(contextValue, logger, request.Scenario, request.WithoutMattermost, request.KeepArtifacts)
	case ActionVerifyRegression:
		if request.WithoutMattermost {
			return errors.New("without-mattermost regression is not supported")
		}
		return service.VerifyRegression(contextValue, logger, request.Base, request.Scenario)
	default:
		return fmt.Errorf("unsupported local fleet action: %s", request.Action)
	}
}

func (service Service) RunRecipe(contextValue context.Context, logger Logger, recipe string) error {
	switch strings.TrimSpace(recipe) {
	case "", DefaultRecipe:
		return service.runPlans(contextValue, logger, service.predeployGatePlans())
	default:
		return fmt.Errorf("unsupported local fleet recipe: %s", recipe)
	}
}

func (service Service) RunScenario(contextValue context.Context, logger Logger, scenario string, withoutMattermost bool, keepArtifacts bool) error {
	normalizedScenario := strings.TrimSpace(scenario)
	if normalizedScenario == "" {
		return errors.New("scenario is required")
	}
	if withoutMattermost {
		return service.runPlans(contextValue, logger, service.withoutMattermostScenarioPlans(normalizedScenario))
	}
	switch normalizedScenario {
	case "dm-recipient-resolve":
		return service.runPlans(contextValue, logger, service.dmRecipientResolveScenarioPlans())
	case "mattermost-bot-invited":
		return service.runPlans(contextValue, logger, service.mattermostScenarioPlans())
	case "mattermost-direct-message-send":
		return service.runPlans(contextValue, logger, service.mattermostDirectMessageScenarioPlans(keepArtifacts))
	case "mattermost-manual":
		if !keepArtifacts {
			return errors.New("mattermost-manual requires --keep so the browser test session remains available")
		}
		return service.runPlans(contextValue, logger, service.mattermostManualScenarioPlans())
	case "mattermost-ask-ephemeral":
		return service.runPlans(contextValue, logger, service.mattermostAskEphemeralScenarioPlans())
	case "mattermost-docx-attachment":
		return service.runPlans(contextValue, logger, service.mattermostDocxAttachmentScenarioPlans(keepArtifacts))
	case "restart-policy-survival":
		return service.runPlans(contextValue, logger, service.restartPolicySurvivalScenarioPlans())
	case "web-backed-ui", "regression-proof":
		return service.runPlans(contextValue, logger, service.webBackedScenarioPlans(normalizedScenario))
	default:
		return fmt.Errorf("unsupported local fleet scenario: %s", normalizedScenario)
	}
}

func (service Service) VerifyRegression(contextValue context.Context, logger Logger, base string, scenario string) error {
	normalizedBase := firstNonEmpty(base, "main")
	normalizedScenario := strings.TrimSpace(scenario)
	if normalizedScenario == "" {
		return errors.New("scenario is required")
	}
	logger.Info("checking base branch " + normalizedBase)
	if errorValue := service.runPlans(contextValue, logger, service.baseRegressionPlans(normalizedBase, normalizedScenario)); errorValue == nil {
		return fmt.Errorf("scenario %s passed on %s; regression test is not proving the fix", normalizedScenario, normalizedBase)
	}
	logger.Info("base failed as expected")
	return service.RunScenario(contextValue, logger, normalizedScenario, false, false)
}

func (service Service) EnsureConfiguration() error {
	if errorValue := os.MkdirAll(service.options.StateRootPath, 0o700); errorValue != nil {
		return errorValue
	}
	document, errorValue := json.MarshalIndent(service.configurationDocument(), "", "  ")
	if errorValue != nil {
		return errorValue
	}
	return os.WriteFile(service.configurationPath(), document, 0o600)
}

func normalizeOptions(options Options) (Options, error) {
	if strings.TrimSpace(options.RepositoryRootPath) == "" {
		workingDirectoryPath, errorValue := os.Getwd()
		if errorValue != nil {
			return options, errorValue
		}
		options.RepositoryRootPath = workingDirectoryPath
	}
	if strings.TrimSpace(options.ExecutablePath) == "" {
		executablePath, errorValue := os.Executable()
		if errorValue != nil {
			return options, errorValue
		}
		options.ExecutablePath = executablePath
	}
	if options.IsEphemeral {
		if strings.TrimSpace(options.RunID) == "" {
			options.RunID = generateRunID()
		}
		options.RunID = safeIdentifier(options.RunID)
	}
	if strings.TrimSpace(options.StateRootPath) == "" {
		options.StateRootPath = defaultStateRootPath(options)
	}
	if strings.TrimSpace(options.VirtualMachineName) == "" {
		options.VirtualMachineName = defaultVirtualMachineName(options)
	}
	if options.AdminHostPort == 0 {
		adminHostPort, errorValue := defaultHostPort(options.IsEphemeral, DefaultAdminHostPort)
		if errorValue != nil {
			return options, errorValue
		}
		options.AdminHostPort = adminHostPort
	}
	if options.MattermostHostPort == 0 {
		mattermostHostPort, errorValue := defaultHostPort(options.IsEphemeral, DefaultMattermostHostPort)
		if errorValue != nil {
			return options, errorValue
		}
		options.MattermostHostPort = mattermostHostPort
	}
	maximumModelTier, errorValue := blueclaw.NormalizeMaximumModelTier(options.MaximumModelTier)
	if errorValue != nil {
		return options, errorValue
	}
	if options.ShouldUseRealModels && maximumModelTier != "" {
		return options, errors.New("maximum model tier cannot be combined with real models")
	}
	if !options.ShouldUseRealModels && maximumModelTier == "" {
		maximumModelTier = "xlow"
	}
	options.MaximumModelTier = maximumModelTier
	return options, nil
}

func defaultStateRootPath(options Options) string {
	if options.IsEphemeral {
		return filepath.Join(options.RepositoryRootPath, ".local", "local-fleet", "runs", options.RunID)
	}
	return filepath.Join(options.RepositoryRootPath, ".local", "local-fleet")
}

func defaultVirtualMachineName(options Options) string {
	if options.IsEphemeral {
		return "internkim-e2e-" + options.RunID
	}
	return DefaultVirtualMachineName
}

func defaultHostPort(isEphemeral bool, fallback int) (int, error) {
	if !isEphemeral {
		return fallback, nil
	}
	return availableLoopbackPort()
}

func availableLoopbackPort() (int, error) {
	listener, errorValue := net.Listen("tcp", "127.0.0.1:0")
	if errorValue != nil {
		return 0, errorValue
	}
	defer listener.Close()
	networkAddress, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, errors.New("loopback listener did not return a TCP address")
	}
	return networkAddress.Port, nil
}

func generateRunID() string {
	randomBytes := make([]byte, 4)
	if _, errorValue := rand.Read(randomBytes); errorValue != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%s-%s", time.Now().UTC().Format("20060102T150405"), hex.EncodeToString(randomBytes))
}

func (service Service) configurationDocument() map[string]any {
	return map[string]any{
		"host": map[string]any{"mode": "single-mac"},
		"vm": map[string]any{
			"container": map[string]any{
				"binaryPath": "container",
				"name":       service.options.VirtualMachineName,
				"image":      "ubuntu:24.04",
				"cpuCount":   6,
				"memoryMiB":  8192,
			},
			"mattermost":          map[string]any{"listenAddress": "127.0.0.1:8065"},
			"sharedWorkspacePath": service.options.RepositoryRootPath,
			"mountDirectoryPath":  "/mnt/shared",
			"sshUsername":         "admin",
			"sshPassword":         "admin",
		},
	}
}

func (service Service) adminHostURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", service.options.AdminHostPort)
}

func (service Service) mattermostHostURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", service.options.MattermostHostPort)
}

func (service Service) cleanupNeeded() bool {
	return service.options.IsEphemeral || fileExists(service.leasesPath())
}

func (service Service) readCommandState(contextValue context.Context, plan CommandPlan) EndpointStatus {
	output, errorValue := service.runBufferedPlan(contextValue, plan)
	if errorValue != nil {
		return EndpointStatus{State: "failed", Message: strings.TrimSpace(output)}
	}
	if strings.Contains(output, "running") {
		return EndpointStatus{State: "ok", Message: "running"}
	}
	return EndpointStatus{State: "stopped", Message: strings.TrimSpace(output)}
}

func (service Service) sshStatus(contextValue context.Context, virtualMachine EndpointStatus) EndpointStatus {
	if virtualMachine.State != "ok" {
		return EndpointStatus{State: "unknown", Message: "VM is not running"}
	}
	_, errorValue := service.runBufferedPlan(contextValue, service.labCommand("vm-ssh", "true"))
	if errorValue != nil {
		return EndpointStatus{State: "failed", Message: "SSH unavailable"}
	}
	return EndpointStatus{State: "ok", Message: "ready"}
}

func (service Service) httpStatus(contextValue context.Context, rawURL string, name string) EndpointStatus {
	if !isLoopbackURL(rawURL) {
		return EndpointStatus{State: "failed", Message: name + " URL is not loopback"}
	}
	request, errorValue := http.NewRequestWithContext(contextValue, http.MethodGet, rawURL, nil)
	if errorValue != nil {
		return EndpointStatus{State: "failed", Message: errorValue.Error()}
	}
	response, errorValue := service.client.Do(request)
	if errorValue != nil {
		return EndpointStatus{State: "unknown", Message: errorValue.Error()}
	}
	defer response.Body.Close()
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return EndpointStatus{State: "ok", Message: fmt.Sprintf("HTTP %d", response.StatusCode)}
	}
	return EndpointStatus{State: "failed", Message: fmt.Sprintf("HTTP %d", response.StatusCode)}
}

func isLoopbackURL(rawURL string) bool {
	trimmedURL := strings.TrimPrefix(strings.TrimPrefix(rawURL, "http://"), "https://")
	host, _, errorValue := net.SplitHostPort(strings.Split(trimmedURL, "/")[0])
	if errorValue != nil {
		host = strings.Split(trimmedURL, "/")[0]
	}
	if host == "localhost" {
		return true
	}
	parsedIP := net.ParseIP(host)
	return parsedIP != nil && parsedIP.IsLoopback()
}

func (service Service) runBufferedPlan(contextValue context.Context, plan CommandPlan) (string, error) {
	command := exec.CommandContext(contextValue, plan.Name, plan.Arguments...)
	command.Dir = plan.DirectoryPath
	command.Env = plan.Environment
	output, errorValue := command.CombinedOutput()
	return string(output), errorValue
}

func readTrimmedFile(path string) string {
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return ""
	}
	return strings.TrimSpace(string(document))
}

func fileExists(path string) bool {
	_, errorValue := os.Stat(path)
	return errorValue == nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
