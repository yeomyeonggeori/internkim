package browser

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
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
	ExtensionPaths       []string
	Runner               CommandRunner
	Now                  func() time.Time
	Sleep                func(context.Context, time.Duration) error
	DisableHumanPacing   bool
	OpenCommandTimeout   time.Duration
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
var activeChromePipeFiles chromePipeFileStore

type chromePipeFileStore struct {
	mutex sync.Mutex
	files []*os.File
}

func (store *chromePipeFileStore) keep(files ...*os.File) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	store.files = append(store.files, files...)
}

const BrowserEngineChrome = "chrome"
const BrowserEngineLightpanda = "lightpanda"

const DeviceBrowserExecutablePath = "/usr/local/bin/lightpanda"
const BrowserExtensionNotReadyError = "브라우저 확장이 준비되지 않았습니다."
const browserExtensionReadyElementID = "internkim-companion-extension-ready"
const browserExtensionReadyTimeout = 3 * time.Second
const browserExtensionReadyPollInterval = 150 * time.Millisecond

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
	if runtime.requiresExtensionVerification() {
		if errorValue := runtime.prepareChromeExtensionSession(ctx); errorValue != nil {
			return SessionStartResult{}, errorValue
		}
		if runtime.canLoadChromeExtensionsThroughPipe() {
			openArguments = runtime.sessionCommandArguments()
		}
	}
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
	if errorValue := runtime.ensureExtensionReady(ctx, targetURL); errorValue != nil {
		return SessionStartResult{}, errorValue
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

func (runtime AgentBrowserRuntime) Navigate(ctx context.Context, request NavigateRequest) (NavigateResult, error) {
	if errorValue := ValidateWebURL(request.URL); errorValue != nil {
		return NavigateResult{}, errorValue
	}
	trimmedURL := strings.TrimSpace(request.URL)
	openArguments := runtime.sessionStartArguments()
	if runtime.requiresExtensionVerification() {
		if errorValue := runtime.prepareChromeExtensionSession(ctx); errorValue != nil {
			return NavigateResult{}, errorValue
		}
		if runtime.canLoadChromeExtensionsThroughPipe() {
			openArguments = runtime.sessionCommandArguments()
		}
	}
	openArguments = append(openArguments, "open", trimmedURL, "--headers", stealthRequestHeaders())
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
	stealthEvalArguments := append(runtime.sessionCommandArguments(), "eval", stealthPostLoadScript())
	_, _ = runtime.run(ctx, stealthEvalArguments...)
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

func stealthRequestHeaders() string {
	return `{"User-Agent":"Mozilla/5.0 (X11; Linux aarch64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36","Accept-Language":"ko,en-US;q=0.9,en;q=0.8"}`
}

func stealthPostLoadScript() string {
	return `Object.defineProperty(navigator,'webdriver',{get:()=>undefined});Object.defineProperty(navigator,'languages',{get:()=>['ko-KR','ko','en-US','en']});`
}

func (runtime AgentBrowserRuntime) prepareChromeExtensionSession(ctx context.Context) error {
	if !runtime.canLoadChromeExtensionsThroughPipe() {
		return nil
	}
	if ready, errorValue := runtime.isExtensionReady(ctx); errorValue == nil && ready {
		return nil
	}
	_ = runtime.closeSession(ctx)
	port, closePort, errorValue := reserveLocalPort()
	if errorValue != nil {
		return errorValue
	}
	closePort()
	if errorValue := runtime.launchChromeWithExtensions(ctx, port); errorValue != nil {
		return errorValue
	}
	_, errorValue = runtime.run(ctx, append(runtime.sessionCommandArguments(), "connect", strconv.Itoa(port))...)
	return errorValue
}

func (runtime AgentBrowserRuntime) canLoadChromeExtensionsThroughPipe() bool {
	return runtime.requiresExtensionVerification() && isExecutablePath(runtime.EngineExecutablePath)
}

func reserveLocalPort() (int, func(), error) {
	listener, errorValue := net.Listen("tcp", "127.0.0.1:0")
	if errorValue != nil {
		return 0, func() {}, errorValue
	}
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		_ = listener.Close()
		return 0, func() {}, errors.New("browser debug port is unavailable")
	}
	return address.Port, func() { _ = listener.Close() }, nil
}

func (runtime AgentBrowserRuntime) launchChromeWithExtensions(ctx context.Context, port int) error {
	parentRead, childWrite, errorValue := os.Pipe()
	if errorValue != nil {
		return errorValue
	}
	childRead, parentWrite, errorValue := os.Pipe()
	if errorValue != nil {
		_ = parentRead.Close()
		_ = childWrite.Close()
		return errorValue
	}
	command := exec.Command(runtime.EngineExecutablePath, runtime.chromeExtensionLaunchArguments(port)...)
	command.ExtraFiles = []*os.File{childRead, childWrite}
	command.Stderr = os.Stderr
	if errorValue := command.Start(); errorValue != nil {
		closeChromePipeFiles(parentRead, parentWrite, childRead, childWrite)
		return errorValue
	}
	_ = childRead.Close()
	_ = childWrite.Close()
	go func() { _ = command.Wait() }()
	if errorValue := runtime.loadExtensionsThroughPipe(ctx, parentRead, parentWrite); errorValue != nil {
		_ = command.Process.Kill()
		_ = parentRead.Close()
		_ = parentWrite.Close()
		return errorValue
	}
	activeChromePipeFiles.keep(parentRead, parentWrite)
	return nil
}

func (runtime AgentBrowserRuntime) chromeExtensionLaunchArguments(port int) []string {
	arguments := []string{
		"--remote-debugging-pipe",
		"--remote-debugging-port=" + strconv.Itoa(port),
		"--enable-unsafe-extension-debugging",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-networking",
		"--disable-backgrounding-occluded-windows",
		"--disable-component-update",
		"--disable-default-apps",
		"--disable-hang-monitor",
		"--disable-popup-blocking",
		"--disable-prompt-on-repost",
		"--disable-sync",
		"--disable-features=Translate",
		"--enable-features=NetworkService,NetworkServiceInProcess",
		"--metrics-recording-only",
		"--password-store=basic",
		"--use-mock-keychain",
	}
	if strings.TrimSpace(runtime.ProfilePath) != "" {
		arguments = append(arguments, "--user-data-dir="+strings.TrimSpace(runtime.ProfilePath))
	}
	if runtime.Headed {
		arguments = append(arguments, "about:blank")
	} else {
		arguments = append(arguments, "--headless=new", "about:blank")
	}
	return arguments
}

func (runtime AgentBrowserRuntime) loadExtensionsThroughPipe(ctx context.Context, reader *os.File, writer *os.File) error {
	client := chromePipeClient{reader: reader, writer: writer}
	if _, errorValue := client.send(ctx, "Browser.getVersion", map[string]any{}); errorValue != nil {
		return errorValue
	}
	for _, extensionPath := range runtime.extensionPaths() {
		if _, errorValue := client.send(ctx, "Extensions.loadUnpacked", map[string]any{"path": extensionPath, "enableInIncognito": false}); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func closeChromePipeFiles(files ...*os.File) {
	for _, file := range files {
		if file != nil {
			_ = file.Close()
		}
	}
}

type chromePipeClient struct {
	reader *os.File
	writer *os.File
	nextID int
}

func (client *chromePipeClient) send(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
	client.nextID++
	request := map[string]any{"id": client.nextID, "method": method, "params": params}
	document, errorValue := json.Marshal(request)
	if errorValue != nil {
		return nil, errorValue
	}
	if _, errorValue := client.writer.Write(append(document, 0)); errorValue != nil {
		return nil, errorValue
	}
	for {
		response, errorValue := client.readResponse(ctx)
		if errorValue != nil {
			return nil, errorValue
		}
		if id, _ := response["id"].(float64); int(id) != client.nextID {
			continue
		}
		if errorDocument, ok := response["error"].(map[string]any); ok {
			return nil, errors.New(firstNonEmpty(findStringValue(errorDocument, "message"), BrowserExtensionNotReadyError))
		}
		result, _ := response["result"].(map[string]any)
		return result, nil
	}
}

func (client *chromePipeClient) readResponse(ctx context.Context) (map[string]any, error) {
	type pipeResponse struct {
		document map[string]any
		error    error
	}
	channel := make(chan pipeResponse, 1)
	go func() {
		document, errorValue := readChromePipeDocument(client.reader)
		channel <- pipeResponse{document: document, error: errorValue}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case response := <-channel:
		return response.document, response.error
	}
}

func readChromePipeDocument(reader *os.File) (map[string]any, error) {
	buffer := []byte{}
	chunk := make([]byte, 1)
	for {
		if _, errorValue := reader.Read(chunk); errorValue != nil {
			return nil, errorValue
		}
		if chunk[0] == 0 {
			break
		}
		buffer = append(buffer, chunk[0])
	}
	var document map[string]any
	if errorValue := json.Unmarshal(buffer, &document); errorValue != nil {
		return nil, errorValue
	}
	return document, nil
}

func (runtime AgentBrowserRuntime) ensureExtensionReady(ctx context.Context, targetURL string) error {
	if !runtime.requiresExtensionVerification() {
		return nil
	}
	if runtime.waitForExtensionReady(ctx, 3) == nil {
		return nil
	}
	_ = runtime.closeSession(ctx)
	if errorValue := runtime.runOpenCommand(ctx, append(runtime.sessionStartArguments(), "open", targetURL)); errorValue != nil && !isOpenSettleTimeout(errorValue) {
		return errorValue
	}
	if runtime.waitForExtensionReady(ctx, extensionReadyAttemptCount()) == nil {
		return nil
	}
	return errors.New(BrowserExtensionNotReadyError)
}

func (runtime AgentBrowserRuntime) requiresExtensionVerification() bool {
	return runtime.browserEngine() == BrowserEngineChrome && len(runtime.extensionPaths()) > 0
}

func (runtime AgentBrowserRuntime) waitForExtensionReady(ctx context.Context, attempts int) error {
	for attempt := 0; attempt < attempts; attempt++ {
		if ready, errorValue := runtime.isExtensionReady(ctx); errorValue == nil && ready {
			return nil
		}
		if errorValue := runtime.sleep(ctx, browserExtensionReadyPollInterval); errorValue != nil {
			return errorValue
		}
	}
	return errors.New(BrowserExtensionNotReadyError)
}

func extensionReadyAttemptCount() int {
	attempts := int(browserExtensionReadyTimeout / browserExtensionReadyPollInterval)
	if attempts < 1 {
		return 1
	}
	return attempts
}

func (runtime AgentBrowserRuntime) isExtensionReady(ctx context.Context) (bool, error) {
	script := `Boolean(document.getElementById("` + browserExtensionReadyElementID + `"))`
	output, errorValue := runtime.run(ctx, append(runtime.sessionCommandArguments(), "eval", script)...)
	if errorValue != nil {
		return false, errorValue
	}
	return strings.Contains(strings.TrimSpace(string(output)), "true"), nil
}

func (runtime AgentBrowserRuntime) closeSession(ctx context.Context) error {
	_, errorValue := runtime.run(ctx, append(runtime.sessionCommandArguments(), "close")...)
	return errorValue
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
	for _, extensionPath := range runtime.extensionPaths() {
		if !isBrowserExtensionDirectory(extensionPath) {
			return RuntimeReadiness{
				Status: "not_ready",
				Error:  BrowserExtensionNotReadyError,
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

func isBrowserExtensionDirectory(path string) bool {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return false
	}
	information, errorValue := os.Stat(trimmedPath)
	if errorValue != nil || !information.IsDir() {
		return false
	}
	for _, filename := range []string{"manifest.json", "content-script.js", "service-worker.js"} {
		if !isRegularFile(filepath.Join(trimmedPath, filename)) {
			return false
		}
	}
	return true
}

func isRegularFile(path string) bool {
	information, errorValue := os.Stat(path)
	return errorValue == nil && !information.IsDir()
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
	if engine != BrowserEngineLightpanda {
		for _, extensionPath := range runtime.extensionPaths() {
			arguments = append(arguments, "--extension", extensionPath)
		}
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

func (runtime AgentBrowserRuntime) extensionPaths() []string {
	paths := []string{}
	for _, path := range runtime.ExtensionPaths {
		trimmedPath := strings.TrimSpace(path)
		if trimmedPath != "" {
			paths = append(paths, trimmedPath)
		}
	}
	return paths
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
