package browser

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeExtensionBridge struct {
	startError    error
	readyError    error
	snapshot      ExtensionDOMSnapshot
	snapshotError error
	resolved      map[string]ExtensionResolvedRef
	resolveError  error
	navigateCalls []string
	stopped       bool
	portValue     int
}

func (bridge *fakeExtensionBridge) Start(ctx context.Context) error {
	_ = ctx
	return bridge.startError
}

func (bridge *fakeExtensionBridge) Stop(ctx context.Context) error {
	_ = ctx
	bridge.stopped = true
	return nil
}

func (bridge *fakeExtensionBridge) Port() int {
	return bridge.portValue
}

func (bridge *fakeExtensionBridge) WaitForReady(ctx context.Context) error {
	_ = ctx
	return bridge.readyError
}

func (bridge *fakeExtensionBridge) RequestSnapshot(ctx context.Context) (ExtensionDOMSnapshot, error) {
	_ = ctx
	if bridge.snapshotError != nil {
		return ExtensionDOMSnapshot{}, bridge.snapshotError
	}
	return bridge.snapshot, nil
}

func (bridge *fakeExtensionBridge) ResolveRef(ctx context.Context, ref string) (ExtensionResolvedRef, error) {
	_ = ctx
	if bridge.resolveError != nil {
		return ExtensionResolvedRef{}, bridge.resolveError
	}
	resolved, ok := bridge.resolved[ref]
	if !ok {
		return ExtensionResolvedRef{Ref: ref, Found: false}, nil
	}
	return resolved, nil
}

func (bridge *fakeExtensionBridge) Navigate(ctx context.Context, url string) (ExtensionDOMSnapshot, error) {
	_ = ctx
	bridge.navigateCalls = append(bridge.navigateCalls, url)
	if bridge.snapshotError != nil {
		return ExtensionDOMSnapshot{}, bridge.snapshotError
	}
	result := bridge.snapshot
	result.URL = url
	return result, nil
}

type fakeInputSynthesizerCall struct {
	kind    string
	screenX int
	screenY int
	button  MouseButton
	text    string
	key     string
}

type fakeInputSynthesizer struct {
	calls []fakeInputSynthesizerCall
	err   error
}

func (synthesizer *fakeInputSynthesizer) MoveMouse(ctx context.Context, screenX int, screenY int) error {
	_ = ctx
	synthesizer.calls = append(synthesizer.calls, fakeInputSynthesizerCall{kind: "move", screenX: screenX, screenY: screenY})
	return synthesizer.err
}

func (synthesizer *fakeInputSynthesizer) Click(ctx context.Context, screenX int, screenY int, button MouseButton) error {
	_ = ctx
	synthesizer.calls = append(synthesizer.calls, fakeInputSynthesizerCall{kind: "click", screenX: screenX, screenY: screenY, button: button})
	return synthesizer.err
}

func (synthesizer *fakeInputSynthesizer) TypeText(ctx context.Context, text string) error {
	_ = ctx
	synthesizer.calls = append(synthesizer.calls, fakeInputSynthesizerCall{kind: "type", text: text})
	return synthesizer.err
}

func (synthesizer *fakeInputSynthesizer) PressKey(ctx context.Context, key string) error {
	_ = ctx
	synthesizer.calls = append(synthesizer.calls, fakeInputSynthesizerCall{kind: "press", key: key})
	return synthesizer.err
}

func testResolvedRef(x float64, y float64) ExtensionResolvedRef {
	return ExtensionResolvedRef{
		Found: true,
		Rect:  ExtensionElementRect{X: x, Y: y, Width: 10, Height: 10},
		Viewport: ExtensionViewportGeometry{
			ScreenX: 0, ScreenY: 0, OuterWidth: 100, InnerWidth: 100, OuterHeight: 100, InnerHeight: 100,
		},
	}
}

func TestExtensionInputRuntimeClickResolvesRefAndSynthesizesClick(t *testing.T) {
	bridge := &fakeExtensionBridge{resolved: map[string]ExtensionResolvedRef{"@e1": testResolvedRef(10, 20)}}
	synthesizer := &fakeInputSynthesizer{}
	runtime := &ExtensionInputRuntime{Bridge: bridge, InputSynthesizer: synthesizer}

	result, errorValue := runtime.Click(context.Background(), ClickRequest{Ref: "@e1"})
	if errorValue != nil {
		t.Fatalf("expected click success: %v", errorValue)
	}
	if !result.OK || result.Action != "click" {
		t.Fatalf("unexpected click result: %+v", result)
	}
	if len(synthesizer.calls) != 1 || synthesizer.calls[0].kind != "click" {
		t.Fatalf("expected exactly one click call, got %+v", synthesizer.calls)
	}
	if synthesizer.calls[0].screenX != 15 || synthesizer.calls[0].screenY != 25 {
		t.Fatalf("unexpected resolved click coordinates: %+v", synthesizer.calls[0])
	}
}

func TestExtensionInputRuntimeClickFailsWhenRefNotFound(t *testing.T) {
	bridge := &fakeExtensionBridge{resolved: map[string]ExtensionResolvedRef{}}
	synthesizer := &fakeInputSynthesizer{}
	runtime := &ExtensionInputRuntime{Bridge: bridge, InputSynthesizer: synthesizer}

	_, errorValue := runtime.Click(context.Background(), ClickRequest{Ref: "@missing"})
	if errorValue == nil {
		t.Fatal("expected an error for an unresolved ref")
	}
	if len(synthesizer.calls) != 0 {
		t.Fatalf("expected no synthesizer calls when resolution fails, got %+v", synthesizer.calls)
	}
}

func TestExtensionInputRuntimeFillClicksThenTypes(t *testing.T) {
	bridge := &fakeExtensionBridge{resolved: map[string]ExtensionResolvedRef{"@field": testResolvedRef(0, 0)}}
	synthesizer := &fakeInputSynthesizer{}
	runtime := &ExtensionInputRuntime{Bridge: bridge, InputSynthesizer: synthesizer}

	_, errorValue := runtime.Fill(context.Background(), FillRequest{Ref: "@field", Text: "hello"})
	if errorValue != nil {
		t.Fatalf("expected fill success: %v", errorValue)
	}
	if len(synthesizer.calls) != 2 || synthesizer.calls[0].kind != "click" || synthesizer.calls[1].kind != "type" {
		t.Fatalf("expected click then type, got %+v", synthesizer.calls)
	}
	if synthesizer.calls[1].text != "hello" {
		t.Fatalf("unexpected typed text: %+v", synthesizer.calls[1])
	}
}

func TestExtensionInputRuntimeFillRequiresText(t *testing.T) {
	runtime := &ExtensionInputRuntime{Bridge: &fakeExtensionBridge{}, InputSynthesizer: &fakeInputSynthesizer{}}

	_, errorValue := runtime.Fill(context.Background(), FillRequest{Ref: "@field", Text: "  "})
	if errorValue == nil {
		t.Fatal("expected an error for empty fill text")
	}
}

func TestExtensionInputRuntimePressDoesNotResolveARef(t *testing.T) {
	bridge := &fakeExtensionBridge{}
	synthesizer := &fakeInputSynthesizer{}
	runtime := &ExtensionInputRuntime{Bridge: bridge, InputSynthesizer: synthesizer}

	_, errorValue := runtime.Press(context.Background(), PressRequest{Key: "Enter"})
	if errorValue != nil {
		t.Fatalf("expected press success: %v", errorValue)
	}
	if len(synthesizer.calls) != 1 || synthesizer.calls[0].kind != "press" || synthesizer.calls[0].key != "Enter" {
		t.Fatalf("unexpected press calls: %+v", synthesizer.calls)
	}
}

func TestExtensionInputRuntimeObserveReturnsSnapshotRefs(t *testing.T) {
	bridge := &fakeExtensionBridge{snapshot: ExtensionDOMSnapshot{
		URL:   "https://example.com",
		Title: "Example",
		Elements: []ExtensionElementDescriptor{
			{Ref: "e1", Tag: "button", Text: "Submit"},
			{Ref: "e2", Tag: "input"},
		},
	}}
	runtime := &ExtensionInputRuntime{Bridge: bridge}

	result, errorValue := runtime.Observe(context.Background(), ObserveRequest{})
	if errorValue != nil {
		t.Fatalf("expected observe success: %v", errorValue)
	}
	if result.URL != "https://example.com" || result.Title != "Example" {
		t.Fatalf("unexpected observe result: %+v", result)
	}
	if !containsString(result.InteractiveRefs, "@e1") || !containsString(result.InteractiveRefs, "@e2") {
		t.Fatalf("expected both refs in observe result: %+v", result.InteractiveRefs)
	}
}

func TestExtensionInputRuntimeNavigateValidatesURL(t *testing.T) {
	runtime := &ExtensionInputRuntime{Bridge: &fakeExtensionBridge{}}

	_, errorValue := runtime.Navigate(context.Background(), NavigateRequest{URL: "not-a-url"})
	if errorValue == nil {
		t.Fatal("expected an error for an invalid navigate URL")
	}
}

func TestExtensionInputRuntimeNavigateReturnsBridgeSnapshot(t *testing.T) {
	bridge := &fakeExtensionBridge{snapshot: ExtensionDOMSnapshot{Title: "Dashboard"}}
	runtime := &ExtensionInputRuntime{Bridge: bridge}

	result, errorValue := runtime.Navigate(context.Background(), NavigateRequest{URL: "https://example.com/dashboard"})
	if errorValue != nil {
		t.Fatalf("expected navigate success: %v", errorValue)
	}
	if result.URL != "https://example.com/dashboard" || result.Title != "Dashboard" {
		t.Fatalf("unexpected navigate result: %+v", result)
	}
	if len(bridge.navigateCalls) != 1 || bridge.navigateCalls[0] != "https://example.com/dashboard" {
		t.Fatalf("unexpected bridge navigate calls: %+v", bridge.navigateCalls)
	}
}

func TestExtensionInputRuntimeWaitSleepsOnMilliseconds(t *testing.T) {
	runtime := &ExtensionInputRuntime{Bridge: &fakeExtensionBridge{}}

	started := time.Now()
	_, errorValue := runtime.Wait(context.Background(), WaitRequest{Milliseconds: 5})
	if errorValue != nil {
		t.Fatalf("expected wait success: %v", errorValue)
	}
	if time.Since(started) < 5*time.Millisecond {
		t.Fatal("expected wait to actually sleep for the requested duration")
	}
}

func TestExtensionInputRuntimeWaitResolvesTargetRef(t *testing.T) {
	bridge := &fakeExtensionBridge{resolved: map[string]ExtensionResolvedRef{"@e1": testResolvedRef(0, 0)}}
	runtime := &ExtensionInputRuntime{Bridge: bridge}

	_, errorValue := runtime.Wait(context.Background(), WaitRequest{Ref: "@e1"})
	if errorValue != nil {
		t.Fatalf("expected wait success: %v", errorValue)
	}
}

func TestExtensionInputRuntimePauseBlocksSynthesizedInput(t *testing.T) {
	bridge := &fakeExtensionBridge{resolved: map[string]ExtensionResolvedRef{"@e1": testResolvedRef(0, 0)}}
	synthesizer := &fakeInputSynthesizer{}
	runtime := &ExtensionInputRuntime{Bridge: bridge, InputSynthesizer: synthesizer}

	if errorValue := runtime.Pause(context.Background()); errorValue != nil {
		t.Fatalf("expected pause success: %v", errorValue)
	}
	if !runtime.IsPaused() {
		t.Fatal("expected runtime to report paused")
	}

	_, errorValue := runtime.Click(context.Background(), ClickRequest{Ref: "@e1"})
	if errorValue == nil {
		t.Fatal("expected click to fail while paused")
	}
	if len(synthesizer.calls) != 0 {
		t.Fatalf("expected no synthesizer calls while paused, got %+v", synthesizer.calls)
	}

	if errorValue := runtime.Resume(context.Background()); errorValue != nil {
		t.Fatalf("expected resume success: %v", errorValue)
	}
	if runtime.IsPaused() {
		t.Fatal("expected runtime to report resumed")
	}
	if _, errorValue := runtime.Click(context.Background(), ClickRequest{Ref: "@e1"}); errorValue != nil {
		t.Fatalf("expected click success after resume: %v", errorValue)
	}
	if len(synthesizer.calls) != 1 {
		t.Fatalf("expected one synthesizer call after resume, got %+v", synthesizer.calls)
	}
}

func TestExtensionInputRuntimeFillBlockedWhilePaused(t *testing.T) {
	bridge := &fakeExtensionBridge{resolved: map[string]ExtensionResolvedRef{"@field": testResolvedRef(0, 0)}}
	synthesizer := &fakeInputSynthesizer{}
	runtime := &ExtensionInputRuntime{Bridge: bridge, InputSynthesizer: synthesizer}

	if errorValue := runtime.Pause(context.Background()); errorValue != nil {
		t.Fatalf("expected pause success: %v", errorValue)
	}
	_, errorValue := runtime.Fill(context.Background(), FillRequest{Ref: "@field", Text: "hello"})
	if errorValue == nil {
		t.Fatal("expected fill to fail while paused")
	}
	if len(synthesizer.calls) != 0 {
		t.Fatalf("expected no click or type calls while paused, got %+v", synthesizer.calls)
	}
}

func TestExtensionInputRuntimeScreenshotIsNotYetImplemented(t *testing.T) {
	runtime := &ExtensionInputRuntime{Bridge: &fakeExtensionBridge{}}

	_, errorValue := runtime.Screenshot(context.Background(), ScreenshotRequest{})
	if errorValue == nil {
		t.Fatal("expected screenshot to report not yet implemented")
	}
}

func TestExtensionInputRuntimeCloseSessionStopsBridge(t *testing.T) {
	bridge := &fakeExtensionBridge{}
	runtime := &ExtensionInputRuntime{Bridge: bridge}

	if errorValue := runtime.CloseSession(context.Background()); errorValue != nil {
		t.Fatalf("expected close session success: %v", errorValue)
	}
	if !bridge.stopped {
		t.Fatal("expected bridge to be stopped")
	}
}

func TestExtensionInputRuntimeLaunchCommandRequiresExtensionPath(t *testing.T) {
	runtime := &ExtensionInputRuntime{}

	_, _, errorValue := runtime.launchCommand()
	if errorValue == nil {
		t.Fatal("expected an error when extension path is not set")
	}
}

func TestExtensionInputRuntimeStartSessionWritesRuntimeConfigBeforeLaunch(t *testing.T) {
	extensionDirectory := t.TempDir()
	bridge := &fakeExtensionBridge{
		portValue: 54321,
		snapshot:  ExtensionDOMSnapshot{URL: "https://example.com", Title: "Example"},
	}
	runner := &fakeCommandRunner{}
	runtime := &ExtensionInputRuntime{
		Bridge:        bridge,
		ExtensionPath: extensionDirectory,
		Runner:        runner,
	}

	if _, errorValue := runtime.StartSession(context.Background(), SessionStartRequest{URL: "https://example.com"}); errorValue != nil {
		t.Fatalf("expected start session success: %v", errorValue)
	}

	document, readError := os.ReadFile(filepath.Join(extensionDirectory, "runtime-config.json"))
	if readError != nil {
		t.Fatalf("expected runtime-config.json to be written: %v", readError)
	}
	var configuration struct {
		Port int `json:"port"`
	}
	if errorValue := json.Unmarshal(document, &configuration); errorValue != nil {
		t.Fatalf("expected valid runtime-config.json: %v", errorValue)
	}
	if configuration.Port != 54321 {
		t.Fatalf("expected runtime-config.json to carry the bridge port, got %+v", configuration)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected Chrome to be launched once, got %+v", runner.calls)
	}
}

func TestExtensionInputRuntimeStartSessionRequiresExtensionPathForConfig(t *testing.T) {
	bridge := &fakeExtensionBridge{snapshot: ExtensionDOMSnapshot{URL: "https://example.com"}}
	runtime := &ExtensionInputRuntime{Bridge: bridge}

	_, errorValue := runtime.StartSession(context.Background(), SessionStartRequest{URL: "https://example.com"})
	if errorValue == nil {
		t.Fatal("expected start session to fail without an extension path")
	}
}

func TestExtensionInputRuntimeImplementsRuntimeInterface(t *testing.T) {
	var _ Runtime = &ExtensionInputRuntime{}
}

func TestExtensionInputRuntimeStartSessionSurfacesBridgeStartError(t *testing.T) {
	bridge := &fakeExtensionBridge{startError: errors.New("listen failed")}
	runtime := &ExtensionInputRuntime{Bridge: bridge, ExtensionPath: "/tmp/extension"}

	_, errorValue := runtime.StartSession(context.Background(), SessionStartRequest{URL: "https://example.com"})
	if errorValue == nil {
		t.Fatal("expected start session to surface the bridge start error")
	}
}

func TestExtensionInputRuntimeStartSessionSurfacesReadyTimeoutError(t *testing.T) {
	bridge := &fakeExtensionBridge{readyError: context.DeadlineExceeded}
	runtime := &ExtensionInputRuntime{Bridge: bridge, ExtensionPath: t.TempDir(), Runner: &fakeCommandRunner{}}

	_, errorValue := runtime.StartSession(context.Background(), SessionStartRequest{URL: "https://example.com"})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "did not connect") {
		t.Fatalf("expected a did-not-connect error when the extension never becomes ready, got %v", errorValue)
	}
}

func TestExtensionInputRuntimeStartSessionSurfacesRuntimeConfigWriteError(t *testing.T) {
	extensionPathThatIsAFile := filepath.Join(t.TempDir(), "not-a-directory")
	if errorValue := os.WriteFile(extensionPathThatIsAFile, []byte("not a directory"), 0o600); errorValue != nil {
		t.Fatalf("expected to create the file fixture: %v", errorValue)
	}
	bridge := &fakeExtensionBridge{}
	runtime := &ExtensionInputRuntime{Bridge: bridge, ExtensionPath: extensionPathThatIsAFile, Runner: &fakeCommandRunner{}}

	_, errorValue := runtime.StartSession(context.Background(), SessionStartRequest{URL: "https://example.com"})
	if errorValue == nil {
		t.Fatal("expected start session to surface a runtime-config.json write error when the extension path is not a directory")
	}
}
