package browser

import (
	"context"
	"errors"
	"os"
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

func (runner *fakeCommandRunner) Run(ctx context.Context, commandPath string, arguments []string) ([]byte, error) {
	_ = ctx
	runner.calls = append(runner.calls, fakeCommandCall{
		commandPath: commandPath,
		arguments:   append([]string{}, arguments...),
	})
	return runner.output, runner.errorValue
}

func TestAgentBrowserRuntimeNavigatesThroughCommandRunner(t *testing.T) {
	runner := &fakeCommandRunner{}
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
	expectedOpenArguments := []string{"--session", "internkim-test", "--headed", "true", "--profile", "/profile", "--session-name", "internkim-test", "open", "https://example.com", "--headers", stealthRequestHeaders()}
	expectedEvalArguments := []string{"--session", "internkim-test", "--session-name", "internkim-test", "eval", stealthPostLoadScript()}
	if len(runner.calls) != 2 {
		t.Fatalf("expected 2 command calls (open + stealth eval), got %d: %+v", len(runner.calls), runner.calls)
	}
	if runner.calls[0].commandPath != "agent-browser-test" || !reflect.DeepEqual(runner.calls[0].arguments, expectedOpenArguments) {
		t.Fatalf("unexpected open call: %+v", runner.calls[0])
	}
	if runner.calls[1].commandPath != "agent-browser-test" || !reflect.DeepEqual(runner.calls[1].arguments, expectedEvalArguments) {
		t.Fatalf("unexpected eval call: %+v", runner.calls[1])
	}
}

func TestAgentBrowserRuntimeChromeEngineUsesHeadedProfile(t *testing.T) {
	runner := &fakeCommandRunner{}
	runtime := AgentBrowserRuntime{
		CommandPath: "agent-browser-test",
		Engine:      BrowserEngineChrome,
		ProfilePath: "/profile",
		SessionName: "internkim-test",
		Headed:      true,
		ExtensionPaths: []string{
			"",
			"/extensions/internkim",
		},
		Runner: runner,
	}

	_, errorValue := runtime.Navigate(context.Background(), NavigateRequest{URL: "https://example.com"})
	if errorValue != nil {
		t.Fatalf("expected navigate success: %v", errorValue)
	}
	expectedOpenArguments := []string{"--session", "internkim-test", "--engine", "chrome", "--headed", "true", "--profile", "/profile", "--extension", "/extensions/internkim", "--session-name", "internkim-test", "open", "https://example.com", "--headers", stealthRequestHeaders()}
	if !reflect.DeepEqual(runner.calls[0].arguments, expectedOpenArguments) {
		t.Fatalf("unexpected chrome arguments: %+v", runner.calls[0].arguments)
	}
}

func TestDeviceReadinessShellScriptChecksChromeScreenshotReadiness(t *testing.T) {
	script := DeviceReadinessShellScript()

	for _, fragment := range []string{"agent-browser close --all", "pkill -TERM -x agent-browser", "pkill -KILL -x agent-browser", "agent-browser doctor --offline --quick", "--session internkim-device-smoke --engine chrome", "--executable-path \"$browserExecutablePath\"", "--headed false", "snapshot", "screenshot", "test -s /tmp/internkim-agent-browser-chrome-smoke.png", DeviceBrowserExecutablePath, DeviceBrowserManifestPath} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected device readiness script to contain %q: %s", fragment, script)
		}
	}
	for _, forbiddenFragment := range []string{"google-chrome", "PUPPETEER_CACHE_DIR", "chrome-for-testing", "chromium-browser", "/snap/bin/chromium", "--engine lightpanda", "agent-browser install", "apt-get install"} {
		if strings.Contains(script, forbiddenFragment) {
			t.Fatalf("device readiness script must not use fallback %q: %s", forbiddenFragment, script)
		}
	}
	if strings.Contains(script, "snapshot --engine") || strings.Contains(script, "screenshot --engine") {
		t.Fatalf("device readiness should pass engine options only while opening the session: %s", script)
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
