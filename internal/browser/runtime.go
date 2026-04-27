package browser

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	CommandPath        string
	ProfilePath        string
	SessionName        string
	Headed             bool
	TemporaryDirectory string
	Runner             CommandRunner
	Now                func() time.Time
}

type RuntimeReadiness struct {
	Status string
	Error  string
}

type SessionStartRequest struct {
	URL      string `json:"url,omitempty"`
	StartURL string `json:"startURL,omitempty"`
}

type SessionStartResult struct {
	SessionID string `json:"sessionID"`
	Opened    bool   `json:"opened"`
	URL       string `json:"url,omitempty"`
}

type NavigateRequest struct {
	URL string `json:"url"`
}

type NavigateResult struct {
	URL string `json:"url"`
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
	if _, errorValue := runtime.run(ctx, append(runtime.baseArguments(), "open", targetURL)...); errorValue != nil {
		return SessionStartResult{}, errorValue
	}
	return SessionStartResult{
		SessionID: runtime.sessionName(),
		Opened:    true,
		URL:       exposedURL,
	}, nil
}

func (runtime AgentBrowserRuntime) Navigate(ctx context.Context, request NavigateRequest) (NavigateResult, error) {
	if errorValue := ValidateWebURL(request.URL); errorValue != nil {
		return NavigateResult{}, errorValue
	}
	if _, errorValue := runtime.run(ctx, append(runtime.baseArguments(), "open", strings.TrimSpace(request.URL))...); errorValue != nil {
		return NavigateResult{}, errorValue
	}
	return NavigateResult{URL: strings.TrimSpace(request.URL)}, nil
}

func (runtime AgentBrowserRuntime) Observe(ctx context.Context, request ObserveRequest) (ObserveResult, error) {
	_ = request
	capturedAt := runtime.now().UTC().Format(time.RFC3339)
	output, errorValue := runtime.run(ctx, append(runtime.baseArguments(), "snapshot", "-i", "--json")...)
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
	if _, errorValue := runtime.run(ctx, append(runtime.baseArguments(), "screenshot", path)...); errorValue != nil {
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
	if _, errorValue := runtime.run(ctx, append(runtime.baseArguments(), "click", target)...); errorValue != nil {
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
	if _, errorValue := runtime.run(ctx, append(runtime.baseArguments(), "fill", target, request.Text)...); errorValue != nil {
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
	if _, errorValue := runtime.run(ctx, append(runtime.baseArguments(), "select", target, request.Value)...); errorValue != nil {
		return ActionResult{}, errorValue
	}
	return runtime.actionResult("select", target), nil
}

func (runtime AgentBrowserRuntime) Press(ctx context.Context, request PressRequest) (ActionResult, error) {
	if strings.TrimSpace(request.Key) == "" {
		return ActionResult{}, errors.New("browser press key is required")
	}
	if _, errorValue := runtime.run(ctx, append(runtime.baseArguments(), "press", strings.TrimSpace(request.Key))...); errorValue != nil {
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
	if _, errorValue := runtime.run(ctx, append(runtime.baseArguments(), "wait", waitValue)...); errorValue != nil {
		return ActionResult{}, errorValue
	}
	return runtime.actionResult("wait", target), nil
}

func (runtime AgentBrowserRuntime) Check(ctx context.Context) RuntimeReadiness {
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
	if _, errorValue := runtime.run(ctx, "install"); errorValue != nil {
		return RuntimeReadiness{
			Status: "unavailable",
			Error:  "companion browser runtime install failed",
		}
	}
	return runtime.Check(ctx)
}

func (runtime AgentBrowserRuntime) baseArguments() []string {
	arguments := []string{}
	if runtime.Headed {
		arguments = append(arguments, "--headed", "true")
	} else {
		arguments = append(arguments, "--headed", "false")
	}
	if strings.TrimSpace(runtime.ProfilePath) != "" {
		arguments = append(arguments, "--profile", strings.TrimSpace(runtime.ProfilePath))
	}
	if runtime.sessionName() != "" {
		arguments = append(arguments, "--session-name", runtime.sessionName())
	}
	return arguments
}

func (runtime AgentBrowserRuntime) run(ctx context.Context, arguments ...string) ([]byte, error) {
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

func (runtime AgentBrowserRuntime) commandPath() string {
	return firstNonEmpty(runtime.CommandPath, "agent-browser")
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
	result.URL = findStringValue(document, "url")
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
