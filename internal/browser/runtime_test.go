package browser

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeCommandRunner struct {
	output     []byte
	errorValue error
	calls      []fakeCommandCall
}

type fakeCommandCall struct {
	commandPath string
	arguments   []string
}

func containsString(values []string, expectedValue string) bool {
	for _, value := range values {
		if value == expectedValue {
			return true
		}
	}
	return false
}

func (runner *fakeCommandRunner) Run(ctx context.Context, commandPath string, arguments []string) ([]byte, error) {
	_ = ctx
	runner.calls = append(runner.calls, fakeCommandCall{
		commandPath: commandPath,
		arguments:   append([]string{}, arguments...),
	})
	return runner.output, runner.errorValue
}

func TestAgentBrowserRuntimeNavigatesThroughCommandRunner(t *testing.T) {
	runner := &fakeCommandRunner{output: []byte("https://example.com\n")}
	runtime := AgentBrowserRuntime{
		CommandPath: "agent-browser-test",
		ProfilePath: "/profile",
		SessionName: "internkim-test",
		Headed:      true,
		Runner:      runner,
	}

	result, errorValue := runtime.Navigate(context.Background(), NavigateRequest{URL: "https://example.com"})
	if errorValue != nil {
		t.Fatalf("expected navigate success: %v", errorValue)
	}
	if result.URL != "https://example.com" {
		t.Fatalf("unexpected navigate result: %+v", result)
	}
	expectedOpenArguments := []string{"--session", "internkim-test", "--headed", "true", "--profile", "/profile", "--session-name", "internkim-test", "open", "https://example.com"}
	expectedURLArguments := []string{"--session", "internkim-test", "--session-name", "internkim-test", "get", "url"}
	expectedSnapshotArguments := []string{"--session", "internkim-test", "--session-name", "internkim-test", "snapshot", "-i", "--compact", "--json"}
	if len(runner.calls) != 3 {
		t.Fatalf("expected 3 command calls (open + url check + snapshot), got %d: %+v", len(runner.calls), runner.calls)
	}
	if runner.calls[0].commandPath != "agent-browser-test" || !reflect.DeepEqual(runner.calls[0].arguments, expectedOpenArguments) {
		t.Fatalf("unexpected open call: %+v", runner.calls[0])
	}
	if runner.calls[1].commandPath != "agent-browser-test" || !reflect.DeepEqual(runner.calls[1].arguments, expectedURLArguments) {
		t.Fatalf("unexpected current URL call: %+v", runner.calls[1])
	}
	if runner.calls[2].commandPath != "agent-browser-test" || !reflect.DeepEqual(runner.calls[2].arguments, expectedSnapshotArguments) {
		t.Fatalf("unexpected snapshot call: %+v", runner.calls[2])
	}
}

func TestAgentBrowserRuntimeAcceptsLoginRedirect(t *testing.T) {
	runner := &fakeCommandRunner{output: []byte("https://accounts.google.com/signin/v2/identifier\n")}
	runtime := AgentBrowserRuntime{
		CommandPath:        "agent-browser-test",
		SessionName:        "internkim-test",
		DisableHumanPacing: true,
		Runner:             runner,
	}

	result, errorValue := runtime.Navigate(context.Background(), NavigateRequest{URL: "https://console.cloud.google.com/"})
	if errorValue != nil {
		t.Fatalf("expected redirect navigate success: %v", errorValue)
	}
	if result.URL != "https://accounts.google.com/signin/v2/identifier" {
		t.Fatalf("unexpected redirect result: %+v", result)
	}
	expectedSnapshotArguments := []string{"--session", "internkim-test", "--session-name", "internkim-test", "snapshot", "-i", "--compact", "--json"}
	if runner.calls[2].commandPath != "agent-browser-test" || !reflect.DeepEqual(runner.calls[2].arguments, expectedSnapshotArguments) {
		t.Fatalf("unexpected snapshot call: %+v", runner.calls[2])
	}
}

func TestAgentBrowserRuntimeAcceptsOpenSettleTimeoutWithCurrentURL(t *testing.T) {
	runner := &openTimeoutCommandRunner{
		currentURL: "https://console.cloud.google.com/apis/credentials?project=internkim-7373e2a4",
	}
	runtime := AgentBrowserRuntime{
		CommandPath:        "agent-browser-test",
		SessionName:        "internkim-test",
		DisableHumanPacing: true,
		Runner:             runner,
		OpenCommandTimeout: 5 * time.Millisecond,
	}

	result, errorValue := runtime.Navigate(context.Background(), NavigateRequest{URL: "https://console.cloud.google.com/apis/credentials?project=internkim-7373e2a4"})

	if errorValue != nil {
		t.Fatalf("expected navigate success after bounded open timeout: %v", errorValue)
	}
	if result.URL != runner.currentURL {
		t.Fatalf("expected current URL after bounded open timeout, got %+v", result)
	}
	if len(runner.calls) != 3 {
		t.Fatalf("expected open, url, snapshot calls, got %+v", runner.calls)
	}
}

func TestAgentBrowserRuntimeNavigateUsesSnapshotURLWhenCurrentURLIsStale(t *testing.T) {
	runner := &sequenceCommandRunner{results: []commandResult{
		{output: []byte("ok\n")},
		{output: []byte("about:blank\n")},
		{output: []byte(`{"url":"https://example.com/dashboard","title":"Dashboard","snapshotText":"ready","nodes":[{"ref":"e1"}]}`)},
	}}
	runtime := AgentBrowserRuntime{
		CommandPath:        "agent-browser-test",
		SessionName:        "internkim-test",
		DisableHumanPacing: true,
		Runner:             runner,
	}

	result, errorValue := runtime.Navigate(context.Background(), NavigateRequest{URL: "https://example.com/dashboard"})

	if errorValue != nil {
		t.Fatalf("expected snapshot URL to recover stale current URL: %v", errorValue)
	}
	if result.URL != "https://example.com/dashboard" || result.SnapshotText != "ready" || !containsString(result.InteractiveRefs, "@e1") {
		t.Fatalf("unexpected recovered navigation result: %+v", result)
	}
}

func TestAgentBrowserRuntimeNavigateRecordsCaptureTimeWithoutSnapshot(t *testing.T) {
	capturedAt := time.Date(2026, 7, 19, 1, 2, 3, 0, time.UTC)
	runner := &sequenceCommandRunner{results: []commandResult{
		{output: []byte("ok\n")},
		{output: []byte("https://example.com/dashboard\n")},
		{errorValue: errors.New("snapshot failed")},
	}}
	runtime := AgentBrowserRuntime{
		CommandPath:        "agent-browser-test",
		SessionName:        "internkim-test",
		DisableHumanPacing: true,
		Runner:             runner,
		Now:                func() time.Time { return capturedAt },
	}

	result, errorValue := runtime.Navigate(context.Background(), NavigateRequest{URL: "https://example.com/dashboard"})

	if errorValue != nil {
		t.Fatalf("expected current URL to preserve successful navigation: %v", errorValue)
	}
	if result.CapturedAt != capturedAt.Format(time.RFC3339) {
		t.Fatalf("expected canonical capture time, got %+v", result)
	}
}

func TestAgentBrowserRuntimeStartsSessionAfterOpenSettleTimeout(t *testing.T) {
	runner := &openTimeoutCommandRunner{
		currentURL: "https://console.cloud.google.com/apis/credentials?project=internkim-7373e2a4",
	}
	runtime := AgentBrowserRuntime{
		CommandPath:        "agent-browser-test",
		SessionName:        "internkim-test",
		DisableHumanPacing: true,
		Runner:             runner,
		OpenCommandTimeout: 5 * time.Millisecond,
	}

	result, errorValue := runtime.StartSession(context.Background(), SessionStartRequest{URL: "https://console.cloud.google.com/apis/credentials?project=internkim-7373e2a4"})

	if errorValue != nil {
		t.Fatalf("expected session start success after bounded open timeout: %v", errorValue)
	}
	if result.URL != runner.currentURL || result.SnapshotText != "Credentials" {
		t.Fatalf("expected observed page after bounded open timeout, got %+v", result)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("expected open and snapshot calls, got %+v", runner.calls)
	}
}

func TestMacOSApplicationPathFromChromeExecutable(t *testing.T) {
	path := macosApplicationPath("/Applications/Google Chrome.app/Contents/MacOS/Google Chrome")
	if path != "/Applications/Google Chrome.app" {
		t.Fatalf("unexpected macOS app path: %s", path)
	}
}

func TestAgentBrowserRuntimeOpenReturnsObservedRedirectSnapshot(t *testing.T) {
	runner := &fakeCommandRunner{output: []byte(`{"data":{"origin":"https://console.cloud.google.com/apis/credentials?project=internkim-7373e2a4","snapshot":"Credentials","refs":{"e1":{"role":"button","name":"Create credential"}}}}`)}
	runtime := AgentBrowserRuntime{
		CommandPath:          "agent-browser-test",
		SessionName:          "internkim-test",
		Runner:               runner,
		DisableHumanPacing:   true,
		TemporaryDirectory:   "/tmp/internkim-test",
		EngineExecutablePath: "/Applications/Google Chrome.app",
	}

	result, errorValue := runtime.StartSession(context.Background(), SessionStartRequest{URL: "https://console.cloud.google.com/apis/credentials"})
	if errorValue != nil {
		t.Fatalf("expected open success: %v", errorValue)
	}
	if result.URL != "https://console.cloud.google.com/apis/credentials?project=internkim-7373e2a4" || result.RequestedURL != "https://console.cloud.google.com/apis/credentials" {
		t.Fatalf("expected observed redirect URL with requested URL, got %+v", result)
	}
	if result.SnapshotText != "Credentials" || !containsString(result.InteractiveRefs, "@e1") {
		t.Fatalf("expected observed snapshot in open result, got %+v", result)
	}
}

func TestAgentBrowserRuntimeChromeEngineUsesHeadedProfile(t *testing.T) {
	runner := &fakeCommandRunner{output: []byte("https://example.com\n")}
	runtime := AgentBrowserRuntime{
		CommandPath:        "agent-browser-test",
		Engine:             BrowserEngineChrome,
		ProfilePath:        "/profile",
		SessionName:        "internkim-test",
		Headed:             true,
		DisableHumanPacing: true,
		Runner:             runner,
	}

	_, errorValue := runtime.Navigate(context.Background(), NavigateRequest{URL: "https://example.com"})
	if errorValue != nil {
		t.Fatalf("expected navigate success: %v", errorValue)
	}
	expectedOpenArguments := []string{"--session", "internkim-test", "--engine", "chrome", "--headed", "true", "--profile", "/profile", "--session-name", "internkim-test", "open", "https://example.com"}
	if !reflect.DeepEqual(runner.calls[0].arguments, expectedOpenArguments) {
		t.Fatalf("unexpected chrome arguments: %+v", runner.calls[0].arguments)
	}
}

func TestAgentBrowserRuntimeHandoffSessionOmitsExtensionArguments(t *testing.T) {
	runner := &fakeCommandRunner{output: []byte(`{"data":{"origin":"https://example.com/login","snapshot":"Login"}}`)}
	runtime := AgentBrowserRuntime{
		CommandPath:        "agent-browser-test",
		Engine:             BrowserEngineChrome,
		ProfilePath:        "/profile",
		SessionName:        "internkim-test",
		Headed:             true,
		DisableHumanPacing: true,
		Runner:             runner,
	}

	result, errorValue := runtime.StartSession(context.Background(), SessionStartRequest{URL: "https://example.com/login"})
	if errorValue != nil {
		t.Fatalf("expected session start success: %v", errorValue)
	}
	if result.URL != "https://example.com/login" {
		t.Fatalf("unexpected session result: %+v", result)
	}
	for _, call := range runner.calls {
		if containsString(call.arguments, "--extension") || containsString(call.arguments, "eval") {
			t.Fatalf("handoff session must not use extension or eval commands: %+v", runner.calls)
		}
	}
}

func TestAgentBrowserRuntimePacesHeadedChromeCommands(t *testing.T) {
	runner := &fakeCommandRunner{output: []byte("https://example.com\n")}
	delays := []time.Duration{}
	runtime := AgentBrowserRuntime{
		Engine:      BrowserEngineChrome,
		SessionName: "internkim-test",
		Headed:      true,
		Runner:      runner,
		Sleep: func(ctx context.Context, delay time.Duration) error {
			_ = ctx
			delays = append(delays, delay)
			return nil
		},
	}

	_, errorValue := runtime.Navigate(context.Background(), NavigateRequest{URL: "https://example.com"})
	if errorValue != nil {
		t.Fatalf("expected navigate success: %v", errorValue)
	}
	if len(delays) != 2 {
		t.Fatalf("expected open and snapshot pacing delays, got %+v", delays)
	}
	for _, delay := range delays {
		if delay < 900*time.Millisecond || delay >= 1800*time.Millisecond {
			t.Fatalf("unexpected human pacing delay: %s", delay)
		}
	}
}

func TestAgentBrowserRuntimeFailsWhenBrowserDoesNotNavigate(t *testing.T) {
	runner := &fakeCommandRunner{output: []byte("about:blank\n")}
	runtime := AgentBrowserRuntime{
		CommandPath:        "agent-browser-test",
		SessionName:        "internkim-test",
		DisableHumanPacing: true,
		Runner:             runner,
	}

	_, errorValue := runtime.Navigate(context.Background(), NavigateRequest{URL: "https://example.com"})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "did not navigate") {
		t.Fatalf("expected navigation verification failure, got %v", errorValue)
	}
}

func TestAgentBrowserRuntimeDoesNotPaceLightpandaCommands(t *testing.T) {
	runner := &fakeCommandRunner{output: []byte("https://example.com\n")}
	sleepWasCalled := false
	runtime := AgentBrowserRuntime{
		Engine: BrowserEngineLightpanda,
		Headed: true,
		Runner: runner,
		Sleep: func(ctx context.Context, delay time.Duration) error {
			_ = ctx
			_ = delay
			sleepWasCalled = true
			return nil
		},
	}

	_, errorValue := runtime.Navigate(context.Background(), NavigateRequest{URL: "https://example.com"})
	if errorValue != nil {
		t.Fatalf("expected navigate success: %v", errorValue)
	}
	if sleepWasCalled {
		t.Fatal("expected Lightpanda device browser not to use human pacing")
	}
}

func TestAgentBrowserRuntimeRealChromeSmoke(t *testing.T) {
	if os.Getenv("INTERNKIM_REAL_CHROME_BROWSER_TEST") != "1" {
		t.Skip("set INTERNKIM_REAL_CHROME_BROWSER_TEST=1 to launch a real headed Chrome smoke test")
	}
	commandPath := os.Getenv("INTERNKIM_AGENT_BROWSER_PATH")
	if strings.TrimSpace(commandPath) == "" {
		var errorValue error
		commandPath, errorValue = exec.LookPath("agent-browser")
		if errorValue != nil {
			t.Fatal("agent-browser is required for the real Chrome smoke test")
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = responseWriter.Write([]byte(`<html><head><title>InternKim Chrome Smoke</title></head><body><button>Chrome smoke ready</button></body></html>`))
	}))
	defer server.Close()
	sessionName := "internkim-real-chrome-smoke-" + time.Now().UTC().Format("20060102T150405")
	runtime := AgentBrowserRuntime{
		CommandPath: commandPath,
		Engine:      BrowserEngineChrome,
		ProfilePath: filepath.Join(t.TempDir(), "profile"),
		SessionName: sessionName,
		Headed:      true,
	}
	t.Cleanup(func() {
		closeContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = OSCommandRunner{}.Run(closeContext, commandPath, []string{"--session", sessionName, "--session-name", sessionName, "close"})
	})
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if _, errorValue := runtime.Navigate(ctx, NavigateRequest{URL: server.URL}); errorValue != nil {
		t.Fatalf("expected real Chrome navigate success: %v", errorValue)
	}
	observation, errorValue := runtime.Observe(ctx, ObserveRequest{})
	if errorValue != nil {
		t.Fatalf("expected real Chrome snapshot success: %v", errorValue)
	}
	if !strings.Contains(observation.SnapshotText, "Chrome smoke ready") {
		t.Fatalf("expected real Chrome page text, got %+v", observation)
	}
}

func TestDeviceReadinessShellScriptChecksLightpandaSnapshotReadiness(t *testing.T) {
	script := DeviceReadinessShellScript()

	for _, fragment := range []string{"agent-browser close --all", "pkill -TERM -x agent-browser", "pkill -KILL -x agent-browser", "agent-browser doctor --offline --quick", "--session internkim-device-smoke --engine lightpanda", "--executable-path \"$browserExecutablePath\"", "snapshot -i --compact --json", DeviceBrowserExecutablePath} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected device readiness script to contain %q: %s", fragment, script)
		}
	}
	for _, forbiddenFragment := range []string{"google-chrome", "PUPPETEER_CACHE_DIR", "chrome-for-testing", "chromium-browser", "/snap/bin/chromium", "--engine chrome", "--headed false", "screenshot", "agent-browser install", "apt-get install"} {
		if strings.Contains(script, forbiddenFragment) {
			t.Fatalf("device readiness script must not use fallback %q: %s", forbiddenFragment, script)
		}
	}
	if strings.Contains(script, "snapshot --engine") {
		t.Fatalf("device readiness should pass engine options only while opening the session: %s", script)
	}
}

func TestAgentBrowserRuntimeLightpandaOmitsChromeOnlyArguments(t *testing.T) {
	runner := &fakeCommandRunner{output: []byte("https://example.com\n")}
	runtime := AgentBrowserRuntime{
		CommandPath:          "agent-browser-test",
		Engine:               BrowserEngineLightpanda,
		EngineExecutablePath: "/usr/local/bin/lightpanda",
		ProfilePath:          "/profile",
		SessionName:          "internkim-test",
		Headed:               true,
		Runner:               runner,
	}

	_, errorValue := runtime.Navigate(context.Background(), NavigateRequest{URL: "https://example.com"})
	if errorValue != nil {
		t.Fatalf("expected navigate success: %v", errorValue)
	}
	expectedOpenArguments := []string{"--session", "internkim-test", "--engine", "lightpanda", "--executable-path", "/usr/local/bin/lightpanda", "--session-name", "internkim-test", "open", "https://example.com"}
	if !reflect.DeepEqual(runner.calls[0].arguments, expectedOpenArguments) {
		t.Fatalf("unexpected lightpanda arguments: %+v", runner.calls[0].arguments)
	}
}

func TestAgentBrowserRuntimeRejectsUnsafeNavigateURL(t *testing.T) {
	runner := &fakeCommandRunner{}
	runtime := AgentBrowserRuntime{Runner: runner}

	_, errorValue := runtime.Navigate(context.Background(), NavigateRequest{URL: "file:///etc/passwd"})
	if errorValue == nil {
		t.Fatal("expected invalid URL to fail")
	}
	if len(runner.calls) != 0 {
		t.Fatalf("expected no command call, got %+v", runner.calls)
	}
}

func TestAgentBrowserRuntimeObserveParsesSafeSnapshotShape(t *testing.T) {
	runner := &fakeCommandRunner{output: []byte(`{
		"url":"https://example.com",
		"title":"Example",
		"snapshotText":"- link \"More\" [ref=e2]",
		"nodes":[{"ref":"e1"}],
		"profilePath":"/Users/lee/Library/Application Support/InternKim/BrowserProfile",
		"hasMore":true
	}`)}
	runtime := AgentBrowserRuntime{
		Runner: runner,
		Now:    func() time.Time { return time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC) },
	}

	result, errorValue := runtime.Observe(context.Background(), ObserveRequest{})
	if errorValue != nil {
		t.Fatalf("expected observe success: %v", errorValue)
	}
	if result.URL != "https://example.com" || result.Title != "Example" || result.SnapshotText == "" || !result.HasMore {
		t.Fatalf("unexpected observe result: %+v", result)
	}
	if !reflect.DeepEqual(result.InteractiveRefs, []string{"@e1", "@e2"}) {
		t.Fatalf("unexpected refs: %+v", result.InteractiveRefs)
	}
	if strings.Contains(result.SnapshotText, "cookie") || strings.Contains(result.SnapshotText, "cdp") {
		t.Fatalf("unexpected sensitive snapshot: %+v", result)
	}
}

func TestAgentBrowserRuntimeObserveAvoidsRawJSONLeak(t *testing.T) {
	runner := &fakeCommandRunner{output: []byte(`{
		"title":"Example",
		"profilePath":"/Users/lee/Library/Application Support/InternKim/BrowserProfile",
		"cdpWebSocketURL":"ws://127.0.0.1:9222/devtools/browser",
		"nodes":[{"ref":"e1","name":"Visible button"}]
	}`)}
	runtime := AgentBrowserRuntime{Runner: runner}

	result, errorValue := runtime.Observe(context.Background(), ObserveRequest{})
	if errorValue != nil {
		t.Fatalf("expected observe success: %v", errorValue)
	}
	if strings.Contains(result.SnapshotText, "BrowserProfile") || strings.Contains(result.SnapshotText, "devtools") {
		t.Fatalf("snapshot leaked browser internals: %+v", result)
	}
	if !strings.Contains(result.SnapshotText, "Visible button") {
		t.Fatalf("snapshot dropped visible text: %+v", result)
	}
}

func TestAgentBrowserRuntimeScreenshotRunsCommandAndReturnsOmittedPath(t *testing.T) {
	temporaryDirectory := t.TempDir()
	runner := &screenshotCommandRunner{}
	runtime := AgentBrowserRuntime{
		TemporaryDirectory: temporaryDirectory,
		Runner:             runner,
		Now:                func() time.Time { return time.Date(2026, 4, 27, 1, 2, 3, 4, time.UTC) },
	}

	result, errorValue := runtime.Screenshot(context.Background(), ScreenshotRequest{})
	if errorValue != nil {
		t.Fatalf("expected screenshot success: %v", errorValue)
	}
	if result.LocalPath == "" || result.ContentType != "image/png" || result.SizeBytes != 3 {
		t.Fatalf("unexpected screenshot result: %+v", result)
	}
	if filepath.Dir(result.LocalPath) != temporaryDirectory {
		t.Fatalf("unexpected screenshot directory: %s", result.LocalPath)
	}
}

func TestAgentBrowserRuntimeControlCommands(t *testing.T) {
	runner := &fakeCommandRunner{}
	runtime := AgentBrowserRuntime{
		CommandPath: "agent-browser-test",
		ProfilePath: "/profile",
		SessionName: "internkim-test",
		Runner:      runner,
		Now:         func() time.Time { return time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC) },
	}

	cases := []struct {
		name      string
		run       func() (ActionResult, error)
		arguments []string
	}{
		{
			name:      "click",
			run:       func() (ActionResult, error) { return runtime.Click(context.Background(), ClickRequest{Ref: "@e1"}) },
			arguments: []string{"--session", "internkim-test", "--session-name", "internkim-test", "click", "@e1"},
		},
		{
			name: "fill",
			run: func() (ActionResult, error) {
				return runtime.Fill(context.Background(), FillRequest{Target: "@e2", Text: "hello"})
			},
			arguments: []string{"--session", "internkim-test", "--session-name", "internkim-test", "fill", "@e2", "hello"},
		},
		{
			name: "select",
			run: func() (ActionResult, error) {
				return runtime.Select(context.Background(), SelectRequest{Selector: "select[name=team]", Value: "ops"})
			},
			arguments: []string{"--session", "internkim-test", "--session-name", "internkim-test", "select", "select[name=team]", "ops"},
		},
		{
			name:      "press",
			run:       func() (ActionResult, error) { return runtime.Press(context.Background(), PressRequest{Key: "Enter"}) },
			arguments: []string{"--session", "internkim-test", "--session-name", "internkim-test", "press", "Enter"},
		},
		{
			name: "wait",
			run: func() (ActionResult, error) {
				return runtime.Wait(context.Background(), WaitRequest{Milliseconds: 250})
			},
			arguments: []string{"--session", "internkim-test", "--session-name", "internkim-test", "wait", "250"},
		},
	}

	for index, testCase := range cases {
		result, errorValue := testCase.run()
		if errorValue != nil {
			t.Fatalf("expected %s success: %v", testCase.name, errorValue)
		}
		if !result.OK || result.Action != testCase.name {
			t.Fatalf("unexpected %s result: %+v", testCase.name, result)
		}
		if !reflect.DeepEqual(runner.calls[index].arguments, testCase.arguments) {
			t.Fatalf("unexpected %s arguments: %+v", testCase.name, runner.calls[index].arguments)
		}
	}
}

func TestAgentBrowserRuntimeControlValidation(t *testing.T) {
	runner := &fakeCommandRunner{}
	runtime := AgentBrowserRuntime{Runner: runner}

	if _, errorValue := runtime.Click(context.Background(), ClickRequest{}); errorValue == nil {
		t.Fatal("expected empty click target to fail")
	}
	if _, errorValue := runtime.Fill(context.Background(), FillRequest{Target: "@e1"}); errorValue == nil {
		t.Fatal("expected empty fill text to fail")
	}
	if _, errorValue := runtime.Select(context.Background(), SelectRequest{Target: "@e1"}); errorValue == nil {
		t.Fatal("expected empty select value to fail")
	}
	if _, errorValue := runtime.Press(context.Background(), PressRequest{}); errorValue == nil {
		t.Fatal("expected empty press key to fail")
	}
	if _, errorValue := runtime.Wait(context.Background(), WaitRequest{}); errorValue == nil {
		t.Fatal("expected empty wait to fail")
	}
	if len(runner.calls) != 0 {
		t.Fatalf("expected invalid controls not to run commands: %+v", runner.calls)
	}
}

func TestAgentBrowserRuntimeMissingCommandUsesSafeError(t *testing.T) {
	runtime := AgentBrowserRuntime{Runner: &fakeCommandRunner{errorValue: errors.New(`exec: "agent-browser": executable file not found in $PATH`)}}

	_, errorValue := runtime.Observe(context.Background(), ObserveRequest{})
	if errorValue == nil {
		t.Fatal("expected missing command to fail")
	}
	if strings.Contains(errorValue.Error(), "$PATH") || strings.Contains(errorValue.Error(), "exec:") {
		t.Fatalf("expected sanitized error, got %v", errorValue)
	}
}

func TestAgentBrowserRuntimeEnsureInstalledRunsInstallWhenDoctorFails(t *testing.T) {
	runner := &sequenceCommandRunner{
		results: []commandResult{
			{errorValue: errors.New("doctor failed")},
			{},
			{},
		},
	}
	runtime := AgentBrowserRuntime{Runner: runner}

	readiness := runtime.EnsureInstalled(context.Background())
	if readiness.Status != "ready" {
		t.Fatalf("expected ready runtime, got %+v", readiness)
	}
	if !reflect.DeepEqual(runner.commands, [][]string{
		{"doctor", "--offline", "--quick"},
		{"install"},
		{"doctor", "--offline", "--quick"},
	}) {
		t.Fatalf("unexpected commands: %+v", runner.commands)
	}
}

func TestAgentBrowserRuntimeChromeMissingExecutableIsNotReady(t *testing.T) {
	runner := &fakeCommandRunner{}
	runtime := AgentBrowserRuntime{
		Engine:               BrowserEngineChrome,
		EngineExecutablePath: filepath.Join(t.TempDir(), "Google Chrome"),
		Runner:               runner,
	}

	readiness := runtime.EnsureInstalled(context.Background())
	if readiness.Status != "not_ready" {
		t.Fatalf("expected not_ready without installing Chrome for Testing, got %+v", readiness)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("expected no doctor/install command for missing real Chrome, got %+v", runner.calls)
	}
}

func TestAgentBrowserRuntimeEnsureInstalledReturnsSanitizedFailure(t *testing.T) {
	runner := &sequenceCommandRunner{
		results: []commandResult{
			{errorValue: errors.New(`exec: "agent-browser": executable file not found in $PATH`)},
			{errorValue: errors.New(`exec: "agent-browser": executable file not found in $PATH`)},
		},
	}
	runtime := AgentBrowserRuntime{Runner: runner}

	readiness := runtime.EnsureInstalled(context.Background())
	if readiness.Status != "unavailable" || strings.Contains(readiness.Error, "$PATH") {
		t.Fatalf("expected sanitized unavailable status, got %+v", readiness)
	}
}

type screenshotCommandRunner struct{}

func (runner *screenshotCommandRunner) Run(ctx context.Context, commandPath string, arguments []string) ([]byte, error) {
	_ = ctx
	_ = commandPath
	if len(arguments) == 0 {
		return nil, errors.New("missing arguments")
	}
	path := arguments[len(arguments)-1]
	if errorValue := os.WriteFile(path, []byte("png"), 0o600); errorValue != nil {
		return nil, errorValue
	}
	return []byte("ok"), nil
}

type commandResult struct {
	output     []byte
	errorValue error
}

type openTimeoutCommandRunner struct {
	currentURL string
	calls      []fakeCommandCall
}

func (runner *openTimeoutCommandRunner) Run(ctx context.Context, commandPath string, arguments []string) ([]byte, error) {
	runner.calls = append(runner.calls, fakeCommandCall{
		commandPath: commandPath,
		arguments:   append([]string{}, arguments...),
	})
	if containsString(arguments, "get") {
		return []byte(runner.currentURL + "\n"), nil
	}
	switch browserCommandName(arguments) {
	case "open":
		<-ctx.Done()
		return nil, ctx.Err()
	case "snapshot":
		return []byte(`{"data":{"origin":"` + runner.currentURL + `","snapshot":"Credentials","refs":{"e1":{"role":"button","name":"Create credential"}}}}`), nil
	default:
		return []byte("ok"), nil
	}
}

type sequenceCommandRunner struct {
	results  []commandResult
	commands [][]string
}

func (runner *sequenceCommandRunner) Run(ctx context.Context, commandPath string, arguments []string) ([]byte, error) {
	_ = ctx
	_ = commandPath
	runner.commands = append(runner.commands, append([]string{}, arguments...))
	if len(runner.results) == 0 {
		return nil, nil
	}
	result := runner.results[0]
	runner.results = runner.results[1:]
	return result.output, result.errorValue
}
