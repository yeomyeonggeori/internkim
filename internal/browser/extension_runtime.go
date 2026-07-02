package browser

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync/atomic"
	"time"
)

// ExtensionInputRuntime drives a normal, un-automated Chrome instance
// (--load-extension, no --enable-automation, no --remote-debugging-port) for
// the companion app. It resolves element coordinates through
// ExtensionBridge (a companion-bundled browser extension reporting
// getBoundingClientRect() data) and performs clicks/typing through
// OSInputSynthesizer (real OS input events), so neither leg of the
// automation sets navigator.webdriver or otherwise trips CDP-based bot
// detection.
type ExtensionInputRuntime struct {
	ChromeExecutablePath string
	ExtensionPath        string
	ProfilePath          string
	SessionName          string
	Runner               CommandRunner
	Bridge               ExtensionBridge
	InputSynthesizer     OSInputSynthesizer
	Now                  func() time.Time
	ReadyTimeout         time.Duration

	paused atomic.Bool
}

const extensionRuntimePausedError = "browser automation is paused for a human handoff"

func (runtime *ExtensionInputRuntime) StartSession(ctx context.Context, request SessionStartRequest) (SessionStartResult, error) {
	targetURL := firstNonEmpty(request.URL, request.StartURL)
	if targetURL == "" {
		return SessionStartResult{}, errors.New("browser session url is required")
	}
	if errorValue := ValidateWebURL(targetURL); errorValue != nil {
		return SessionStartResult{}, errorValue
	}
	if errorValue := runtime.bridge().Start(ctx); errorValue != nil {
		return SessionStartResult{}, errorValue
	}
	if errorValue := runtime.writeExtensionRuntimeConfig(); errorValue != nil {
		return SessionStartResult{}, errorValue
	}
	if errorValue := runtime.launchChrome(ctx); errorValue != nil {
		return SessionStartResult{}, errorValue
	}
	readyContext, cancelReadyContext := context.WithTimeout(ctx, runtime.readyTimeout())
	defer cancelReadyContext()
	if errorValue := runtime.bridge().WaitForReady(readyContext); errorValue != nil {
		return SessionStartResult{}, errors.New("browser extension did not connect: " + errorValue.Error())
	}
	snapshot, errorValue := runtime.bridge().Navigate(ctx, targetURL)
	if errorValue != nil {
		return SessionStartResult{}, errorValue
	}
	return SessionStartResult{
		SessionID:       runtime.sessionName(),
		Opened:          true,
		URL:             firstNonEmpty(snapshot.URL, targetURL),
		RequestedURL:    targetURL,
		Title:           snapshot.Title,
		SnapshotText:    extensionSnapshotText(snapshot),
		InteractiveRefs: extensionSnapshotRefs(snapshot),
		CapturedAt:      runtime.now().UTC().Format(time.RFC3339),
	}, nil
}

func (runtime *ExtensionInputRuntime) Navigate(ctx context.Context, request NavigateRequest) (NavigateResult, error) {
	if errorValue := ValidateWebURL(request.URL); errorValue != nil {
		return NavigateResult{}, errorValue
	}
	trimmedURL := strings.TrimSpace(request.URL)
	snapshot, errorValue := runtime.bridge().Navigate(ctx, trimmedURL)
	if errorValue != nil {
		return NavigateResult{}, errorValue
	}
	return NavigateResult{
		URL:             firstNonEmpty(snapshot.URL, trimmedURL),
		RequestedURL:    trimmedURL,
		Title:           snapshot.Title,
		SnapshotText:    extensionSnapshotText(snapshot),
		InteractiveRefs: extensionSnapshotRefs(snapshot),
		CapturedAt:      runtime.now().UTC().Format(time.RFC3339),
	}, nil
}

func (runtime *ExtensionInputRuntime) Observe(ctx context.Context, request ObserveRequest) (ObserveResult, error) {
	_ = request
	snapshot, errorValue := runtime.bridge().RequestSnapshot(ctx)
	if errorValue != nil {
		return ObserveResult{}, errorValue
	}
	return ObserveResult{
		URL:             snapshot.URL,
		Title:           snapshot.Title,
		SnapshotText:    extensionSnapshotText(snapshot),
		InteractiveRefs: extensionSnapshotRefs(snapshot),
		CapturedAt:      runtime.now().UTC().Format(time.RFC3339),
	}, nil
}

func (runtime *ExtensionInputRuntime) Screenshot(ctx context.Context, request ScreenshotRequest) (ScreenshotResult, error) {
	_ = ctx
	_ = request
	return ScreenshotResult{}, errors.New("extension input runtime screenshot is not yet implemented")
}

func (runtime *ExtensionInputRuntime) Click(ctx context.Context, request ClickRequest) (ActionResult, error) {
	target, errorValue := browserTarget(request.Selector, request.Ref, request.Target)
	if errorValue != nil {
		return ActionResult{}, errorValue
	}
	point, errorValue := runtime.resolveScreenPoint(ctx, target)
	if errorValue != nil {
		return ActionResult{}, errorValue
	}
	if errorValue := runtime.synthesizeInput(func(synthesizer OSInputSynthesizer) error {
		return synthesizer.Click(ctx, point.X, point.Y, MouseButtonLeft)
	}); errorValue != nil {
		return ActionResult{}, errorValue
	}
	return runtime.actionResult("click", target), nil
}

func (runtime *ExtensionInputRuntime) Fill(ctx context.Context, request FillRequest) (ActionResult, error) {
	target, errorValue := browserTarget(request.Selector, request.Ref, request.Target)
	if errorValue != nil {
		return ActionResult{}, errorValue
	}
	if strings.TrimSpace(request.Text) == "" {
		return ActionResult{}, errors.New("browser fill text is required")
	}
	point, errorValue := runtime.resolveScreenPoint(ctx, target)
	if errorValue != nil {
		return ActionResult{}, errorValue
	}
	if errorValue := runtime.synthesizeInput(func(synthesizer OSInputSynthesizer) error {
		if errorValue := synthesizer.Click(ctx, point.X, point.Y, MouseButtonLeft); errorValue != nil {
			return errorValue
		}
		return synthesizer.TypeText(ctx, request.Text)
	}); errorValue != nil {
		return ActionResult{}, errorValue
	}
	return runtime.actionResult("fill", target), nil
}

func (runtime *ExtensionInputRuntime) Select(ctx context.Context, request SelectRequest) (ActionResult, error) {
	target, errorValue := browserTarget(request.Selector, request.Ref, request.Target)
	if errorValue != nil {
		return ActionResult{}, errorValue
	}
	if strings.TrimSpace(request.Value) == "" {
		return ActionResult{}, errors.New("browser select value is required")
	}
	point, errorValue := runtime.resolveScreenPoint(ctx, target)
	if errorValue != nil {
		return ActionResult{}, errorValue
	}
	if errorValue := runtime.synthesizeInput(func(synthesizer OSInputSynthesizer) error {
		if errorValue := synthesizer.Click(ctx, point.X, point.Y, MouseButtonLeft); errorValue != nil {
			return errorValue
		}
		if errorValue := synthesizer.TypeText(ctx, request.Value); errorValue != nil {
			return errorValue
		}
		return synthesizer.PressKey(ctx, "Enter")
	}); errorValue != nil {
		return ActionResult{}, errorValue
	}
	return runtime.actionResult("select", target), nil
}

func (runtime *ExtensionInputRuntime) Press(ctx context.Context, request PressRequest) (ActionResult, error) {
	if strings.TrimSpace(request.Key) == "" {
		return ActionResult{}, errors.New("browser press key is required")
	}
	if errorValue := runtime.synthesizeInput(func(synthesizer OSInputSynthesizer) error {
		return synthesizer.PressKey(ctx, strings.TrimSpace(request.Key))
	}); errorValue != nil {
		return ActionResult{}, errorValue
	}
	return runtime.actionResult("press", ""), nil
}

func (runtime *ExtensionInputRuntime) Wait(ctx context.Context, request WaitRequest) (ActionResult, error) {
	target := strings.TrimSpace(firstNonEmpty(request.Selector, request.Ref, request.Target))
	if target == "" && request.Milliseconds <= 0 {
		return ActionResult{}, errors.New("browser wait requires target or milliseconds")
	}
	if target == "" {
		if errorValue := runtime.sleep(ctx, time.Duration(request.Milliseconds)*time.Millisecond); errorValue != nil {
			return ActionResult{}, errorValue
		}
		return runtime.actionResult("wait", ""), nil
	}
	if _, errorValue := runtime.resolveScreenPoint(ctx, target); errorValue != nil {
		return ActionResult{}, errorValue
	}
	return runtime.actionResult("wait", target), nil
}

// Pause stops ExtensionInputRuntime from sending OS-level input, so a human
// can take over the still-open Chrome window during a handoff (captcha,
// 2FA) without a second automation window being opened. Resume restores
// normal operation once the handoff is validated complete.
func (runtime *ExtensionInputRuntime) Pause(ctx context.Context) error {
	_ = ctx
	runtime.paused.Store(true)
	return nil
}

func (runtime *ExtensionInputRuntime) Resume(ctx context.Context) error {
	_ = ctx
	runtime.paused.Store(false)
	return nil
}

func (runtime *ExtensionInputRuntime) IsPaused() bool {
	return runtime.paused.Load()
}

func (runtime *ExtensionInputRuntime) CloseSession(ctx context.Context) error {
	return runtime.bridge().Stop(ctx)
}

func (runtime *ExtensionInputRuntime) synthesizeInput(action func(OSInputSynthesizer) error) error {
	if runtime.paused.Load() {
		return errors.New(extensionRuntimePausedError)
	}
	return action(runtime.inputSynthesizer())
}

func (runtime *ExtensionInputRuntime) resolveScreenPoint(ctx context.Context, ref string) (ScreenPoint, error) {
	resolved, errorValue := runtime.bridge().ResolveRef(ctx, ref)
	if errorValue != nil {
		return ScreenPoint{}, errorValue
	}
	if !resolved.Found {
		return ScreenPoint{}, errors.New("browser target was not found: " + ref)
	}
	return elementCenterScreenPoint(resolved.Rect, resolved.Viewport), nil
}

// writeExtensionRuntimeConfig tells the unpacked browser extension which
// bridge port to connect to (see companion/browser-extension/background.js,
// internkimResolvePort) by writing runtime-config.json into the extension
// directory. This must run after bridge().Start (so Port() is known) and
// before launchChrome, since Chrome reads an unpacked extension's directory
// fresh on every launch.
func (runtime *ExtensionInputRuntime) writeExtensionRuntimeConfig() error {
	extensionPath := strings.TrimSpace(runtime.ExtensionPath)
	if extensionPath == "" {
		return errors.New("browser extension path is required")
	}
	document, errorValue := json.Marshal(struct {
		Port int `json:"port"`
	}{Port: runtime.bridge().Port()})
	if errorValue != nil {
		return errorValue
	}
	return os.WriteFile(filepath.Join(extensionPath, "runtime-config.json"), document, 0o600)
}

func (runtime *ExtensionInputRuntime) launchChrome(ctx context.Context) error {
	commandPath, arguments, errorValue := runtime.launchCommand()
	if errorValue != nil {
		return errorValue
	}
	runner := runtime.Runner
	if runner == nil {
		runner = OSCommandRunner{}
	}
	if _, errorValue := runner.Run(ctx, commandPath, arguments); errorValue != nil {
		return errors.New("companion browser extension runtime open failed")
	}
	return nil
}

func (runtime *ExtensionInputRuntime) launchCommand() (string, []string, error) {
	extensionPath := strings.TrimSpace(runtime.ExtensionPath)
	profilePath := strings.TrimSpace(runtime.ProfilePath)
	if extensionPath == "" {
		return "", nil, errors.New("browser extension path is required")
	}
	extensionArguments := []string{
		"--load-extension=" + extensionPath,
		"--disable-extensions-except=" + extensionPath,
		"--no-first-run",
	}
	if profilePath != "" {
		extensionArguments = append(extensionArguments, "--user-data-dir="+profilePath)
	}
	if goruntime.GOOS == "darwin" {
		applicationPath := firstNonEmpty(macosApplicationPath(runtime.ChromeExecutablePath), "Google Chrome")
		return "open", append([]string{"-na", applicationPath, "--args"}, extensionArguments...), nil
	}
	if strings.TrimSpace(runtime.ChromeExecutablePath) == "" {
		return "", nil, errors.New("Google Chrome is not installed")
	}
	return strings.TrimSpace(runtime.ChromeExecutablePath), extensionArguments, nil
}

func (runtime *ExtensionInputRuntime) bridge() ExtensionBridge {
	if runtime.Bridge != nil {
		return runtime.Bridge
	}
	runtime.Bridge = &ExtensionWebSocketBridge{}
	return runtime.Bridge
}

func (runtime *ExtensionInputRuntime) inputSynthesizer() OSInputSynthesizer {
	if runtime.InputSynthesizer != nil {
		return runtime.InputSynthesizer
	}
	return UnsupportedInputSynthesizer{}
}

func (runtime *ExtensionInputRuntime) readyTimeout() time.Duration {
	if runtime.ReadyTimeout > 0 {
		return runtime.ReadyTimeout
	}
	return 30 * time.Second
}

func (runtime *ExtensionInputRuntime) sessionName() string {
	return firstNonEmpty(runtime.SessionName, "internkim")
}

func (runtime *ExtensionInputRuntime) sleep(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
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

func (runtime *ExtensionInputRuntime) now() time.Time {
	if runtime.Now != nil {
		return runtime.Now()
	}
	return time.Now()
}

func (runtime *ExtensionInputRuntime) actionResult(action string, target string) ActionResult {
	return ActionResult{
		OK:         true,
		Action:     action,
		Target:     target,
		CapturedAt: runtime.now().UTC().Format(time.RFC3339),
	}
}

func extensionSnapshotText(snapshot ExtensionDOMSnapshot) string {
	elementDescriptions := make([]string, 0, len(snapshot.Elements))
	for _, element := range snapshot.Elements {
		description := "@" + element.Ref + " " + element.Tag
		if strings.TrimSpace(element.Text) != "" {
			description += " " + strings.TrimSpace(element.Text)
		}
		elementDescriptions = append(elementDescriptions, description)
	}
	return strings.Join(elementDescriptions, "\n")
}

func extensionSnapshotRefs(snapshot ExtensionDOMSnapshot) []string {
	references := make([]string, 0, len(snapshot.Elements))
	for _, element := range snapshot.Elements {
		references = append(references, "@"+element.Ref)
	}
	return uniqueSortedStrings(references)
}
