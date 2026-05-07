package browser

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Runtime interface {
	StartSession(context.Context, SessionStartRequest) (SessionStartResult, error)
	Navigate(context.Context, NavigateRequest) (NavigateResult, error)
	Observe(context.Context, ObserveRequest) (ObserveResult, error)
	Screenshot(context.Context, ScreenshotRequest) (ScreenshotResult, error)
	Click(context.Context, ClickRequest) (ActionResult, error)
	Fill(context.Context, FillRequest) (ActionResult, error)
	Select(context.Context, SelectRequest) (ActionResult, error)
	Press(context.Context, PressRequest) (ActionResult, error)
	Wait(context.Context, WaitRequest) (ActionResult, error)
}

type CommandRunner interface {
	Run(context.Context, string, []string) ([]byte, error)
}

type AgentBrowserRuntime struct {
	CommandPath          string
	Engine               string
	EngineExecutablePath string
	ProfilePath          string
	SessionName          string
	Headed               bool
	TemporaryDirectory   string
	Runner               CommandRunner
	Now                  func() time.Time
	Sleep                func(context.Context, time.Duration) error
	DisableHumanPacing   bool
	OpenCommandTimeout   time.Duration
}

type NativeHandoffRuntime struct {
	EngineExecutablePath string
	ProfilePath          string
	SessionName          string
	Runner               CommandRunner
	Now                  func() time.Time
}

const browserOpenCommandTimeout = 8 * time.Second

type RuntimeReadiness struct {
	Status string
	Error  string
}

type SessionStartRequest struct {
	URL      string `json:"url,omitempty"`
	StartURL string `json:"startURL,omitempty"`
}

type SessionStartResult struct {
	SessionID       string   `json:"sessionID"`
	Opened          bool     `json:"opened"`
	URL             string   `json:"url,omitempty"`
	RequestedURL    string   `json:"requestedURL,omitempty"`
	Title           string   `json:"title,omitempty"`
	SnapshotText    string   `json:"snapshotText,omitempty"`
	InteractiveRefs []string `json:"interactiveRefs,omitempty"`
	CapturedAt      string   `json:"capturedAt,omitempty"`
}

type NavigateRequest struct {
	URL string `json:"url"`
}

type NavigateResult struct {
	URL             string   `json:"url"`
	RequestedURL    string   `json:"requestedURL,omitempty"`
	Title           string   `json:"title,omitempty"`
	SnapshotText    string   `json:"snapshotText,omitempty"`
	InteractiveRefs []string `json:"interactiveRefs,omitempty"`
	CapturedAt      string   `json:"capturedAt,omitempty"`
}

type ObserveRequest struct{}

type ObserveResult struct {
	URL             string   `json:"url,omitempty"`
	Title           string   `json:"title,omitempty"`
	SnapshotText    string   `json:"snapshotText"`
	InteractiveRefs []string `json:"interactiveRefs"`
	HasMore         bool     `json:"hasMore"`
	CapturedAt      string   `json:"capturedAt"`
}

type ScreenshotRequest struct{}

type ScreenshotResult struct {
	LocalPath   string `json:"-"`
	Filename    string `json:"filename"`
	SizeBytes   int64  `json:"sizeBytes"`
	ContentType string `json:"contentType"`
	CapturedAt  string `json:"capturedAt"`
}

type ClickRequest struct {
	Selector string `json:"selector,omitempty"`
	Ref      string `json:"ref,omitempty"`
	Target   string `json:"target,omitempty"`
}

type FillRequest struct {
	Selector string `json:"selector,omitempty"`
	Ref      string `json:"ref,omitempty"`
	Target   string `json:"target,omitempty"`
	Text     string `json:"text"`
}

type SelectRequest struct {
	Selector string `json:"selector,omitempty"`
	Ref      string `json:"ref,omitempty"`
	Target   string `json:"target,omitempty"`
	Value    string `json:"value"`
}

type PressRequest struct {
	Key string `json:"key"`
}

type WaitRequest struct {
	Selector     string `json:"selector,omitempty"`
	Ref          string `json:"ref,omitempty"`
	Target       string `json:"target,omitempty"`
	Milliseconds int    `json:"milliseconds,omitempty"`
}

type ActionResult struct {
	OK         bool   `json:"ok"`
	Action     string `json:"action"`
	Target     string `json:"target,omitempty"`
	CapturedAt string `json:"capturedAt"`
}

type OSCommandRunner struct{}

var agentBrowserReferencePattern = regexp.MustCompile(`@?[A-Za-z]+[0-9]+`)

const BrowserEngineChrome = "chrome"
const BrowserEngineLightpanda = "lightpanda"

const DeviceBrowserExecutablePath = "/usr/local/bin/lightpanda"

func (request *SessionStartRequest) UnmarshalJSON(document []byte) error {
	value, isString := decodeStringDocument(document)
	if isString {
		request.URL = value
		return nil
	}
	type sessionStartRequest SessionStartRequest
	return json.Unmarshal(document, (*sessionStartRequest)(request))
}

func (request *NavigateRequest) UnmarshalJSON(document []byte) error {
	value, isString := decodeStringDocument(document)
	if isString {
		request.URL = value
		return nil
	}
	type navigateRequest NavigateRequest
	return json.Unmarshal(document, (*navigateRequest)(request))
}

func (request *ObserveRequest) UnmarshalJSON(document []byte) error {
	if _, isString := decodeStringDocument(document); isString {
		return nil
	}
	type observeRequest ObserveRequest
	return json.Unmarshal(document, (*observeRequest)(request))
}

func (request *ClickRequest) UnmarshalJSON(document []byte) error {
	value, isString := decodeStringDocument(document)
	if isString {
		request.Target = value
		return nil
	}
	type clickRequest ClickRequest
	return json.Unmarshal(document, (*clickRequest)(request))
}

func (request *PressRequest) UnmarshalJSON(document []byte) error {
	value, isString := decodeStringDocument(document)
	if isString {
		request.Key = value
		return nil
	}
	type pressRequest PressRequest
	return json.Unmarshal(document, (*pressRequest)(request))
}

func (request *WaitRequest) UnmarshalJSON(document []byte) error {
	value, isString := decodeStringDocument(document)
	if isString {
		request.Target = value
		return nil
	}
	valueInt, isNumber := decodeIntegerDocument(document)
	if isNumber {
		request.Milliseconds = valueInt
		return nil
	}
	type waitRequest WaitRequest
	return json.Unmarshal(document, (*waitRequest)(request))
}

func decodeStringDocument(document []byte) (string, bool) {
	var value string
	if json.Unmarshal(bytes.TrimSpace(document), &value) != nil {
		return "", false
	}
	return strings.TrimSpace(value), true
}

func decodeIntegerDocument(document []byte) (int, bool) {
	var value int
	if json.Unmarshal(bytes.TrimSpace(document), &value) != nil {
		return 0, false
	}
	return value, true
}

func (runtime AgentBrowserRuntime) StartSession(ctx context.Context, request SessionStartRequest) (SessionStartResult, error) {
	targetURL := firstNonEmpty(request.URL, request.StartURL)
	exposedURL := targetURL
	if targetURL == "" {
		targetURL = "about:blank"
	}
	if exposedURL != "" {
		if errorValue := ValidateWebURL(exposedURL); errorValue != nil {
			return SessionStartResult{}, errorValue
		}
	}
	openArguments := runtime.sessionStartArguments()
	openError := runtime.runOpenCommand(ctx, append(openArguments, "open", targetURL))
	observation, observationError := runtime.observeCurrentPage(ctx)
	if openError != nil && observationError != nil {
		return SessionStartResult{}, openError
	}
	if openError != nil && !isOpenSettleTimeout(openError) {
		return SessionStartResult{}, openError
	}
	if exposedURL != "" && !isSuccessfulNavigationURL(firstNonEmpty(observation.URL, exposedURL)) {
		return SessionStartResult{}, errors.New("companion browser did not navigate to requested URL")
	}
	return SessionStartResult{
		SessionID:       runtime.sessionName(),
		Opened:          true,
		URL:             firstNonEmpty(observation.URL, exposedURL),
		RequestedURL:    exposedURL,
		Title:           observation.Title,
		SnapshotText:    observation.SnapshotText,
		InteractiveRefs: observation.InteractiveRefs,
		CapturedAt:      observation.CapturedAt,
	}, nil
}

func (runtime NativeHandoffRuntime) StartSession(ctx context.Context, request SessionStartRequest) (SessionStartResult, error) {
	targetURL := firstNonEmpty(request.URL, request.StartURL)
	if targetURL == "" {
		return SessionStartResult{}, errors.New("browser handoff url is required")
	}
	if errorValue := ValidateWebURL(targetURL); errorValue != nil {
		return SessionStartResult{}, errorValue
	}
	commandPath, arguments, errorValue := runtime.openCommand(targetURL)
	if errorValue != nil {
		return SessionStartResult{}, errorValue
	}
	runner := runtime.Runner
	if runner == nil {
		runner = OSCommandRunner{}
	}
	if _, errorValue := runner.Run(ctx, commandPath, arguments); errorValue != nil {
		return SessionStartResult{}, errors.New("companion browser handoff open failed")
	}
	return SessionStartResult{
		SessionID:    firstNonEmpty(runtime.SessionName, "internkim"),
		Opened:       true,
		URL:          targetURL,
		RequestedURL: targetURL,
		CapturedAt:   runtime.now().UTC().Format(time.RFC3339),
	}, nil
}

func (runtime NativeHandoffRuntime) Navigate(ctx context.Context, request NavigateRequest) (NavigateResult, error) {
	_ = ctx
	_ = request
	return NavigateResult{}, errors.New("native browser handoff runtime does not support navigation")
}

func (runtime NativeHandoffRuntime) Observe(ctx context.Context, request ObserveRequest) (ObserveResult, error) {
	_ = ctx
	_ = request
	return ObserveResult{}, errors.New("native browser handoff runtime does not support snapshots")
}

func (runtime NativeHandoffRuntime) Screenshot(ctx context.Context, request ScreenshotRequest) (ScreenshotResult, error) {
	_ = ctx
	_ = request
	return ScreenshotResult{}, errors.New("native browser handoff runtime does not support screenshots")
}

func (runtime NativeHandoffRuntime) Click(ctx context.Context, request ClickRequest) (ActionResult, error) {
	_ = ctx
	_ = request
	return ActionResult{}, errors.New("native browser handoff runtime does not support clicks")
}

func (runtime NativeHandoffRuntime) Fill(ctx context.Context, request FillRequest) (ActionResult, error) {
	_ = ctx
	_ = request
	return ActionResult{}, errors.New("native browser handoff runtime does not support fills")
}

func (runtime NativeHandoffRuntime) Select(ctx context.Context, request SelectRequest) (ActionResult, error) {
	_ = ctx
	_ = request
	return ActionResult{}, errors.New("native browser handoff runtime does not support selects")
}

func (runtime NativeHandoffRuntime) Press(ctx context.Context, request PressRequest) (ActionResult, error) {
	_ = ctx
	_ = request
	return ActionResult{}, errors.New("native browser handoff runtime does not support key presses")
}

func (runtime NativeHandoffRuntime) Wait(ctx context.Context, request WaitRequest) (ActionResult, error) {
	_ = ctx
	_ = request
	return ActionResult{}, errors.New("native browser handoff runtime does not support waits")
}

func (runtime NativeHandoffRuntime) CloseSession(ctx context.Context) error {
	commandPath, arguments, ok := runtime.closeCommand()
	if !ok {
		return nil
	}
	runner := runtime.Runner
	if runner == nil {
		runner = OSCommandRunner{}
	}
	_, _ = runner.Run(ctx, commandPath, arguments)
	return nil
}

func (runtime AgentBrowserRuntime) Navigate(ctx context.Context, request NavigateRequest) (NavigateResult, error) {
	if errorValue := ValidateWebURL(request.URL); errorValue != nil {
		return NavigateResult{}, errorValue
	}
	trimmedURL := strings.TrimSpace(request.URL)
	openArguments := runtime.sessionStartArguments()
	openArguments = append(openArguments, "open", trimmedURL)
	openError := runtime.runOpenCommand(ctx, openArguments)
	actualURL, errorValue := runtime.currentURL(ctx)
	if openError != nil && errorValue != nil {
		return NavigateResult{}, openError
	}
	if openError != nil && !isOpenSettleTimeout(openError) {
		return NavigateResult{}, openError
	}
	if errorValue != nil {
		return NavigateResult{}, errorValue
	}
	if !isSuccessfulNavigationURL(actualURL) {
		return NavigateResult{}, errors.New("companion browser did not navigate to requested URL")
	}
	observation, _ := runtime.observeCurrentPage(ctx)
	return NavigateResult{
		URL:             firstNonEmpty(observation.URL, actualURL),
		RequestedURL:    trimmedURL,
		Title:           observation.Title,
		SnapshotText:    observation.SnapshotText,
		InteractiveRefs: observation.InteractiveRefs,
		CapturedAt:      observation.CapturedAt,
	}, nil
}

func (runtime AgentBrowserRuntime) runOpenCommand(ctx context.Context, arguments []string) error {
	commandContext, cancel := context.WithTimeout(ctx, runtime.openCommandTimeout())
	defer cancel()
	_, errorValue := runtime.run(commandContext, arguments...)
	if commandContext.Err() != nil {
		return commandContext.Err()
	}
	return errorValue
}

func (runtime AgentBrowserRuntime) openCommandTimeout() time.Duration {
	if runtime.OpenCommandTimeout > 0 {
		return runtime.OpenCommandTimeout
	}
	return browserOpenCommandTimeout
}

func isOpenSettleTimeout(errorValue error) bool {
	return errors.Is(errorValue, context.DeadlineExceeded)
}

func (runtime AgentBrowserRuntime) currentURL(ctx context.Context) (string, error) {
	output, errorValue := runtime.run(ctx, append(runtime.sessionCommandArguments(), "get", "url")...)
	if errorValue != nil {
		return "", errorValue
	}
	currentURL := strings.TrimSpace(string(output))
	if currentURL == "" {
		return "", errors.New("companion browser current URL is empty")
	}
	return currentURL, nil
}

func isSuccessfulNavigationURL(actualURL string) bool {
	actual, actualError := url.Parse(strings.TrimSpace(actualURL))
	if actualError != nil {
		return false
	}
	if actual.Scheme != "http" && actual.Scheme != "https" {
		return false
	}
	return actual.Hostname() != ""
}

func (runtime AgentBrowserRuntime) closeSession(ctx context.Context) error {
	_, errorValue := runtime.run(ctx, append(runtime.sessionCommandArguments(), "close")...)
	return errorValue
}

func (runtime AgentBrowserRuntime) CloseSession(ctx context.Context) error {
	return runtime.closeSession(ctx)
}

func (runtime AgentBrowserRuntime) Observe(ctx context.Context, request ObserveRequest) (ObserveResult, error) {
	_ = request
	return runtime.observeCurrentPage(ctx)
}

func (runtime AgentBrowserRuntime) observeCurrentPage(ctx context.Context) (ObserveResult, error) {
	capturedAt := runtime.now().UTC().Format(time.RFC3339)
	output, errorValue := runtime.run(ctx, append(runtime.sessionCommandArguments(), "snapshot", "--compact", "--json")...)
	if errorValue != nil {
		return ObserveResult{}, errorValue
	}
	return observeResultFromOutput(output, capturedAt), nil
}

func (runtime AgentBrowserRuntime) Screenshot(ctx context.Context, request ScreenshotRequest) (ScreenshotResult, error) {
	_ = request
	capturedAt := runtime.now().UTC()
	directoryPath := runtime.temporaryDirectory()
	if errorValue := os.MkdirAll(directoryPath, 0o700); errorValue != nil {
		return ScreenshotResult{}, errors.New("companion browser screenshot directory is unavailable")
	}
	filename := "browser-screenshot-" + capturedAt.Format("20060102T150405.000000000Z") + ".png"
	path := filepath.Join(directoryPath, filename)
	if _, errorValue := runtime.run(ctx, append(runtime.sessionCommandArguments(), "screenshot", path)...); errorValue != nil {
		return ScreenshotResult{}, errorValue
	}
	information, errorValue := os.Stat(path)
	if errorValue != nil || information.IsDir() {
		return ScreenshotResult{}, errors.New("companion browser screenshot was not created")
	}
	return ScreenshotResult{
		LocalPath:   path,
		Filename:    filename,
		SizeBytes:   information.Size(),
		ContentType: "image/png",
		CapturedAt:  capturedAt.UTC().Format(time.RFC3339),
	}, nil
}

func (runtime AgentBrowserRuntime) Click(ctx context.Context, request ClickRequest) (ActionResult, error) {
	target, errorValue := browserTarget(request.Selector, request.Ref, request.Target)
	if errorValue != nil {
		return ActionResult{}, errorValue
	}
	if _, errorValue := runtime.run(ctx, append(runtime.sessionCommandArguments(), "click", target)...); errorValue != nil {
		return ActionResult{}, errorValue
	}
	return runtime.actionResult("click", target), nil
}

func (runtime AgentBrowserRuntime) Fill(ctx context.Context, request FillRequest) (ActionResult, error) {
	target, errorValue := browserTarget(request.Selector, request.Ref, request.Target)
	if errorValue != nil {
		return ActionResult{}, errorValue
	}
	if strings.TrimSpace(request.Text) == "" {
		return ActionResult{}, errors.New("browser fill text is required")
	}
	if _, errorValue := runtime.run(ctx, append(runtime.sessionCommandArguments(), "fill", target, request.Text)...); errorValue != nil {
		return ActionResult{}, errorValue
	}
	return runtime.actionResult("fill", target), nil
}

func (runtime AgentBrowserRuntime) Select(ctx context.Context, request SelectRequest) (ActionResult, error) {
	target, errorValue := browserTarget(request.Selector, request.Ref, request.Target)
	if errorValue != nil {
		return ActionResult{}, errorValue
	}
	if strings.TrimSpace(request.Value) == "" {
		return ActionResult{}, errors.New("browser select value is required")
	}
	if _, errorValue := runtime.run(ctx, append(runtime.sessionCommandArguments(), "select", target, request.Value)...); errorValue != nil {
		return ActionResult{}, errorValue
	}
	return runtime.actionResult("select", target), nil
}

func (runtime AgentBrowserRuntime) Press(ctx context.Context, request PressRequest) (ActionResult, error) {
	if strings.TrimSpace(request.Key) == "" {
		return ActionResult{}, errors.New("browser press key is required")
	}
	if _, errorValue := runtime.run(ctx, append(runtime.sessionCommandArguments(), "press", strings.TrimSpace(request.Key))...); errorValue != nil {
		return ActionResult{}, errorValue
	}
	return runtime.actionResult("press", ""), nil
}

func (runtime AgentBrowserRuntime) Wait(ctx context.Context, request WaitRequest) (ActionResult, error) {
	target := strings.TrimSpace(firstNonEmpty(request.Selector, request.Ref, request.Target))
	if target == "" && request.Milliseconds <= 0 {
		return ActionResult{}, errors.New("browser wait requires target or milliseconds")
	}
	waitValue := target
	if waitValue == "" {
		waitValue = strconv.Itoa(request.Milliseconds)
	}
	if _, errorValue := runtime.run(ctx, append(runtime.sessionCommandArguments(), "wait", waitValue)...); errorValue != nil {
		return ActionResult{}, errorValue
	}
	return runtime.actionResult("wait", target), nil
}

func (runtime AgentBrowserRuntime) Check(ctx context.Context) RuntimeReadiness {
	if readiness := runtime.validateLocalBrowserConfiguration(); readiness.Status != "" {
		return readiness
	}
	if _, errorValue := runtime.run(ctx, "doctor", "--offline", "--quick"); errorValue != nil {
		return RuntimeReadiness{
			Status: "unavailable",
			Error:  "companion browser runtime unavailable",
		}
	}
	return RuntimeReadiness{Status: "ready"}
}

func (runtime AgentBrowserRuntime) EnsureInstalled(ctx context.Context) RuntimeReadiness {
	readiness := runtime.Check(ctx)
	if readiness.Status == "ready" {
		return readiness
	}
	if !runtime.canInstallMissingRuntime(readiness) {
		return readiness
	}
	if _, errorValue := runtime.run(ctx, "install"); errorValue != nil {
		return RuntimeReadiness{
			Status: "unavailable",
			Error:  "companion browser runtime install failed",
		}
	}
	return runtime.Check(ctx)
}

func (runtime AgentBrowserRuntime) validateLocalBrowserConfiguration() RuntimeReadiness {
	if runtime.browserEngine() == BrowserEngineChrome {
		if !isExecutablePath(runtime.EngineExecutablePath) {
			return RuntimeReadiness{
				Status: "not_ready",
				Error:  "Google Chrome is not installed",
			}
		}
	}
	return RuntimeReadiness{}
}

func (runtime AgentBrowserRuntime) canInstallMissingRuntime(readiness RuntimeReadiness) bool {
	if readiness.Status == "not_ready" {
		return false
	}
	return runtime.browserEngine() != BrowserEngineChrome
}

func isExecutablePath(path string) bool {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return false
	}
	information, errorValue := os.Stat(trimmedPath)
	return errorValue == nil && !information.IsDir() && information.Mode()&0o111 != 0
}

func DeviceReadinessShellScript() string {
	return `set -eu
command -v agent-browser >/dev/null
browserExecutablePath="${INTERNKIM_DEVICE_BROWSER_PATH:-` + DeviceBrowserExecutablePath + `}"
test -x "$browserExecutablePath"
stop_agent_browser_daemons() {
  if command -v pkill >/dev/null 2>&1; then
    pkill -TERM -x agent-browser >/tmp/internkim-agent-browser-lightpanda-close.log 2>&1 || true
    sleep 1
    pkill -KILL -x agent-browser >>/tmp/internkim-agent-browser-lightpanda-close.log 2>&1 || true
  fi
  timeout 5s agent-browser close --all >>/tmp/internkim-agent-browser-lightpanda-close.log 2>&1 || true
  rm -f /root/.agent-browser/internkim-device-smoke.pid /root/.agent-browser/internkim-device-smoke.stream /root/.agent-browser/internkim-device-smoke.engine /root/.agent-browser/internkim-device-smoke.version
  sleep 1
}
stop_agent_browser_daemons
timeout 15s agent-browser doctor --offline --quick >/tmp/internkim-agent-browser-doctor.log 2>&1 || true
timeout 45s agent-browser --session internkim-device-smoke --engine lightpanda --executable-path "$browserExecutablePath" --session-name internkim-device-smoke open about:blank >/tmp/internkim-agent-browser-lightpanda-open.log 2>&1
timeout 45s agent-browser --session internkim-device-smoke --session-name internkim-device-smoke snapshot >/tmp/internkim-agent-browser-lightpanda-snapshot.log 2>&1
stop_agent_browser_daemons
`
}

func (runtime AgentBrowserRuntime) sessionStartArguments() []string {
	arguments := []string{}
	if runtime.sessionName() != "" {
		arguments = append(arguments, "--session", runtime.sessionName())
	}
	engine := runtime.browserEngine()
	if engine != "" {
		arguments = append(arguments, "--engine", engine)
	}
	executablePath := runtime.browserExecutablePath(engine)
	if executablePath != "" {
		arguments = append(arguments, "--executable-path", executablePath)
	}
	if engine != BrowserEngineLightpanda {
		if runtime.Headed {
			arguments = append(arguments, "--headed", "true")
		} else {
			arguments = append(arguments, "--headed", "false")
		}
	}
	if engine != BrowserEngineLightpanda && strings.TrimSpace(runtime.ProfilePath) != "" {
		arguments = append(arguments, "--profile", strings.TrimSpace(runtime.ProfilePath))
	}
	if runtime.sessionName() != "" {
		arguments = append(arguments, "--session-name", runtime.sessionName())
	}
	return arguments
}

func (runtime AgentBrowserRuntime) sessionCommandArguments() []string {
	if runtime.sessionName() == "" {
		return []string{}
	}
	return []string{"--session", runtime.sessionName(), "--session-name", runtime.sessionName()}
}

func (runtime AgentBrowserRuntime) browserExecutablePath(engine string) string {
	if strings.TrimSpace(runtime.EngineExecutablePath) != "" {
		return strings.TrimSpace(runtime.EngineExecutablePath)
	}
	return ""
}

func (runtime AgentBrowserRuntime) run(ctx context.Context, arguments ...string) ([]byte, error) {
	if errorValue := runtime.paceBrowserCommand(ctx, arguments); errorValue != nil {
		return nil, errorValue
	}
	runner := runtime.Runner
	if runner == nil {
		runner = OSCommandRunner{}
	}
	output, errorValue := runner.Run(ctx, runtime.commandPath(), arguments)
	if errorValue != nil {
		if isMissingCommandError(errorValue) {
			return nil, errors.New("companion browser runtime unavailable: agent-browser is not installed")
		}
		return nil, errors.New("companion browser runtime command failed")
	}
	return output, nil
}

func (runtime AgentBrowserRuntime) paceBrowserCommand(ctx context.Context, arguments []string) error {
	if !runtime.shouldPaceBrowserCommand(arguments) {
		return nil
	}
	return runtime.sleep(ctx, runtime.humanPacingDelay())
}

func (runtime AgentBrowserRuntime) shouldPaceBrowserCommand(arguments []string) bool {
	if runtime.DisableHumanPacing {
		return false
	}
	if runtime.browserEngine() != BrowserEngineChrome || !runtime.Headed {
		return false
	}
	switch browserCommandName(arguments) {
	case "open", "snapshot", "screenshot", "click", "fill", "select", "press", "wait", "eval":
		return true
	default:
		return false
	}
}

func browserCommandName(arguments []string) string {
	commands := map[string]bool{
		"open":       true,
		"snapshot":   true,
		"screenshot": true,
		"click":      true,
		"fill":       true,
		"select":     true,
		"press":      true,
		"wait":       true,
		"eval":       true,
		"close":      true,
	}
	for _, argument := range arguments {
		if commands[argument] {
			return argument
		}
	}
	return ""
}

func (runtime AgentBrowserRuntime) humanPacingDelay() time.Duration {
	const minimumDelay = 900 * time.Millisecond
	const jitterRange = 900
	jitter, errorValue := rand.Int(rand.Reader, big.NewInt(jitterRange))
	if errorValue != nil {
		return minimumDelay + 450*time.Millisecond
	}
	return minimumDelay + time.Duration(jitter.Int64())*time.Millisecond
}

func (runtime AgentBrowserRuntime) sleep(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	if runtime.Sleep != nil {
		return runtime.Sleep(ctx, delay)
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (runtime AgentBrowserRuntime) commandPath() string {
	return firstNonEmpty(runtime.CommandPath, "agent-browser")
}

func (runtime NativeHandoffRuntime) openCommand(targetURL string) (string, []string, error) {
	executablePath := strings.TrimSpace(runtime.EngineExecutablePath)
	profilePath := strings.TrimSpace(runtime.ProfilePath)
	switch goruntime.GOOS {
	case "darwin":
		applicationPath := firstNonEmpty(macosApplicationPath(executablePath), "Google Chrome")
		if profilePath == "" {
			return "open", []string{"-a", applicationPath, targetURL}, nil
		}
		return "open", []string{"-na", applicationPath, "--args", "--user-data-dir=" + profilePath, "--no-first-run", targetURL}, nil
	case "windows":
		if executablePath == "" {
			return "", nil, errors.New("Google Chrome is not installed")
		}
		return executablePath, append(nativeChromeProfileArguments(profilePath), targetURL), nil
	case "linux":
		if executablePath == "" {
			return "", nil, errors.New("Google Chrome is not installed")
		}
		return executablePath, append(nativeChromeProfileArguments(profilePath), targetURL), nil
	default:
		return "", nil, errors.New("browser handoff native open is unsupported on this operating system")
	}
}

func nativeChromeProfileArguments(profilePath string) []string {
	if strings.TrimSpace(profilePath) == "" {
		return []string{}
	}
	return []string{"--user-data-dir=" + strings.TrimSpace(profilePath), "--no-first-run"}
}

func (runtime NativeHandoffRuntime) closeCommand() (string, []string, bool) {
	profilePath := strings.TrimSpace(runtime.ProfilePath)
	if profilePath == "" {
		return "", nil, false
	}
	profilePattern := "--user-data-dir=" + profilePath
	switch goruntime.GOOS {
	case "darwin", "linux":
		return "pkill", []string{"-f", "--", profilePattern}, true
	case "windows":
		script := "$profile = " + strconv.Quote(profilePath) + "; Get-CimInstance Win32_Process -Filter \"name = 'chrome.exe'\" | Where-Object { $_.CommandLine -like \"*--user-data-dir=$profile*\" } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force }"
		return "powershell", []string{"-NoProfile", "-Command", script}, true
	default:
		return "", nil, false
	}
}

func macosApplicationPath(executablePath string) string {
	trimmedPath := strings.TrimSpace(executablePath)
	if trimmedPath == "" {
		return ""
	}
	applicationIndex := strings.Index(trimmedPath, ".app")
	if applicationIndex == -1 {
		return trimmedPath
	}
	return trimmedPath[:applicationIndex+len(".app")]
}

func (runtime NativeHandoffRuntime) now() time.Time {
	if runtime.Now != nil {
		return runtime.Now()
	}
	return time.Now()
}

func (runtime AgentBrowserRuntime) browserEngine() string {
	engine := strings.TrimSpace(runtime.Engine)
	switch engine {
	case BrowserEngineChrome:
		return BrowserEngineChrome
	case BrowserEngineLightpanda:
		return BrowserEngineLightpanda
	default:
		return engine
	}
}

func (runtime AgentBrowserRuntime) sessionName() string {
	return firstNonEmpty(runtime.SessionName, "internkim")
}

func (runtime AgentBrowserRuntime) temporaryDirectory() string {
	if strings.TrimSpace(runtime.TemporaryDirectory) != "" {
		return strings.TrimSpace(runtime.TemporaryDirectory)
	}
	return filepath.Join(os.TempDir(), "internkim-companion-browser")
}

func (runtime AgentBrowserRuntime) now() time.Time {
	if runtime.Now != nil {
		return runtime.Now()
	}
	return time.Now()
}

func (runtime AgentBrowserRuntime) actionResult(action string, target string) ActionResult {
	return ActionResult{
		OK:         true,
		Action:     action,
		Target:     target,
		CapturedAt: runtime.now().UTC().Format(time.RFC3339),
	}
}

func (OSCommandRunner) Run(ctx context.Context, commandPath string, arguments []string) ([]byte, error) {
	command := exec.CommandContext(ctx, commandPath, arguments...)
	return command.CombinedOutput()
}

func browserTarget(values ...string) (string, error) {
	target := strings.TrimSpace(firstNonEmpty(values...))
	if target == "" {
		return "", errors.New("browser target is required")
	}
	return target, nil
}

func observeResultFromOutput(output []byte, capturedAt string) ObserveResult {
	trimmedOutput := bytes.TrimSpace(output)
	result := ObserveResult{
		SnapshotText:    string(trimmedOutput),
		InteractiveRefs: referenceListFromText(string(trimmedOutput)),
		CapturedAt:      capturedAt,
	}
	var document any
	if json.Unmarshal(trimmedOutput, &document) != nil {
		return result
	}
	result.URL = firstNonEmpty(findStringValue(document, "url"), findStringValue(document, "origin"))
	result.Title = findStringValue(document, "title")
	result.HasMore = findBoolValue(document, "hasMore")
	result.InteractiveRefs = mergeReferences(result.InteractiveRefs, referenceListFromDocument(document))
	snapshotText := firstNonEmpty(
		findStringValue(document, "snapshotText"),
		findStringValue(document, "snapshot"),
		findStringValue(document, "text"),
		findStringValue(document, "content"),
	)
	if snapshotText != "" {
		result.SnapshotText = snapshotText
	} else {
		result.SnapshotText = safeDocumentText(document)
	}
	return result
}

func referenceListFromText(value string) []string {
	matches := agentBrowserReferencePattern.FindAllString(value, -1)
	references := make([]string, 0, len(matches))
	for _, match := range matches {
		reference := strings.TrimSpace(match)
		if reference == "" {
			continue
		}
		if !strings.HasPrefix(reference, "@") {
			reference = "@" + reference
		}
		references = append(references, reference)
	}
	return uniqueSortedStrings(references)
}

func referenceListFromDocument(value any) []string {
	references := []string{}
	collectReferences(value, &references)
	return uniqueSortedStrings(references)
}

func collectReferences(value any, references *[]string) {
	switch typedValue := value.(type) {
	case map[string]any:
		for key, childValue := range typedValue {
			if strings.EqualFold(key, "ref") || strings.EqualFold(key, "reference") {
				if text, ok := childValue.(string); ok {
					*references = append(*references, referenceListFromText(text)...)
				}
			}
			collectReferences(childValue, references)
		}
	case []any:
		for _, childValue := range typedValue {
			collectReferences(childValue, references)
		}
	case string:
		*references = append(*references, referenceListFromText(typedValue)...)
	}
}

func findStringValue(value any, key string) string {
	switch typedValue := value.(type) {
	case map[string]any:
		for childKey, childValue := range typedValue {
			if strings.EqualFold(childKey, key) {
				if text, ok := childValue.(string); ok {
					return text
				}
			}
			if text := findStringValue(childValue, key); text != "" {
				return text
			}
		}
	case []any:
		for _, childValue := range typedValue {
			if text := findStringValue(childValue, key); text != "" {
				return text
			}
		}
	}
	return ""
}

func findBoolValue(value any, key string) bool {
	switch typedValue := value.(type) {
	case map[string]any:
		for childKey, childValue := range typedValue {
			if strings.EqualFold(childKey, key) {
				if booleanValue, ok := childValue.(bool); ok {
					return booleanValue
				}
			}
			if findBoolValue(childValue, key) {
				return true
			}
		}
	case []any:
		for _, childValue := range typedValue {
			if findBoolValue(childValue, key) {
				return true
			}
		}
	}
	return false
}

func safeDocumentText(value any) string {
	values := []string{}
	collectSafeText(value, "", &values)
	return strings.Join(uniqueSortedStrings(values), "\n")
}

func collectSafeText(value any, key string, values *[]string) {
	if isSensitiveBrowserKey(key) {
		return
	}
	switch typedValue := value.(type) {
	case map[string]any:
		for childKey, childValue := range typedValue {
			collectSafeText(childValue, childKey, values)
		}
	case []any:
		for _, childValue := range typedValue {
			collectSafeText(childValue, key, values)
		}
	case string:
		trimmedValue := strings.TrimSpace(typedValue)
		if trimmedValue != "" && !looksLikeSensitiveBrowserValue(trimmedValue) {
			*values = append(*values, trimmedValue)
		}
	}
}

func isSensitiveBrowserKey(key string) bool {
	normalizedKey := strings.ToLower(strings.TrimSpace(key))
	for _, fragment := range []string{"cookie", "profile", "cdp", "websocket", "debug", "executable", "path"} {
		if strings.Contains(normalizedKey, fragment) {
			return true
		}
	}
	return false
}

func looksLikeSensitiveBrowserValue(value string) bool {
	lowercaseValue := strings.ToLower(value)
	return strings.Contains(lowercaseValue, "devtools://") ||
		strings.Contains(lowercaseValue, "ws://") ||
		strings.Contains(lowercaseValue, "wss://") ||
		strings.Contains(lowercaseValue, "/browserprofile") ||
		strings.Contains(lowercaseValue, "\\browserprofile")
}

func mergeReferences(left []string, right []string) []string {
	return uniqueSortedStrings(append(left, right...))
}

func uniqueSortedStrings(values []string) []string {
	seen := map[string]bool{}
	uniqueValues := []string{}
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue == "" || seen[trimmedValue] {
			continue
		}
		seen[trimmedValue] = true
		uniqueValues = append(uniqueValues, trimmedValue)
	}
	sort.Strings(uniqueValues)
	return uniqueValues
}

func ValidateWebURL(value string) error {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return errors.New("browser URL is required")
	}
	parsedURL, errorValue := url.Parse(trimmedValue)
	if errorValue != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return errors.New("browser URL must be absolute")
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errors.New("browser URL must use http or https")
	}
	return nil
}

func isMissingCommandError(errorValue error) bool {
	if errors.Is(errorValue, exec.ErrNotFound) {
		return true
	}
	return strings.Contains(errorValue.Error(), "executable file not found")
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
