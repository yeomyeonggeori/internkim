package companion

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	browserruntime "gitlab.com/eastriver/internkim/internal/browser"
	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/internal/llmbackend"
)

type stubLLMChain struct {
	textResponse       llmbackend.Response
	textError          error
	structuredResponse llmbackend.Response
	structuredError    error
	receivedText       *llmbackend.TextRequest
	receivedStructured *llmbackend.StructuredRequest
}

type stubEmbeddingChain struct {
	response        llmbackend.EmbeddingResponse
	errorValue      error
	receivedRequest *llmbackend.EmbeddingRequest
}

func (chain *stubLLMChain) CompleteText(ctx context.Context, request llmbackend.TextRequest) (llmbackend.Response, error) {
	_ = ctx
	chain.receivedText = &request
	return chain.textResponse, chain.textError
}

func (chain *stubLLMChain) CompleteStructured(ctx context.Context, request llmbackend.StructuredRequest) (llmbackend.Response, error) {
	_ = ctx
	chain.receivedStructured = &request
	return chain.structuredResponse, chain.structuredError
}

func (chain *stubEmbeddingChain) CreateEmbedding(ctx context.Context, request llmbackend.EmbeddingRequest) (llmbackend.EmbeddingResponse, error) {
	_ = ctx
	chain.receivedRequest = &request
	return chain.response, chain.errorValue
}

type fakeBrowserRuntime struct {
	startRequest    browserruntime.SessionStartRequest
	navigateRequest browserruntime.NavigateRequest
	clickRequest    browserruntime.ClickRequest
	fillRequest     browserruntime.FillRequest
	selectRequest   browserruntime.SelectRequest
	pressRequest    browserruntime.PressRequest
	waitRequest     browserruntime.WaitRequest
	observeResult   browserruntime.ObserveResult
	startResults    []browserruntime.SessionStartResult
	navigateResults []browserruntime.NavigateResult
	observeCount    int
	startCount      int
	navigateCount   int
	closeCount      int
	pauseCount      int
	resumeCount     int
	pauseError      error
	screenshot      browserruntime.ScreenshotResult
	errorValue      error
	actionError     error
	observeError    error
}

func (runtime *fakeBrowserRuntime) Pause(ctx context.Context) error {
	_ = ctx
	runtime.pauseCount++
	return runtime.pauseError
}

func (runtime *fakeBrowserRuntime) Resume(ctx context.Context) error {
	_ = ctx
	runtime.resumeCount++
	return nil
}

type fakeFilePicker struct {
	pickedFile PickedFile
	errorValue error
}

type fakeFileUploader struct {
	request FileUploadRequest
}

func (runtime *fakeBrowserRuntime) StartSession(ctx context.Context, request browserruntime.SessionStartRequest) (browserruntime.SessionStartResult, error) {
	_ = ctx
	runtime.startRequest = request
	runtime.startCount++
	if runtime.errorValue != nil {
		return browserruntime.SessionStartResult{}, runtime.errorValue
	}
	if runtime.startCount <= len(runtime.startResults) {
		return runtime.startResults[runtime.startCount-1], nil
	}
	return browserruntime.SessionStartResult{
		SessionID:       "internkim",
		Opened:          true,
		URL:             firstNonEmpty(runtime.observeResult.URL, request.URL, request.StartURL),
		Title:           runtime.observeResult.Title,
		SnapshotText:    runtime.observeResult.SnapshotText,
		InteractiveRefs: runtime.observeResult.InteractiveRefs,
		CapturedAt:      runtime.observeResult.CapturedAt,
	}, nil
}

func (runtime *fakeBrowserRuntime) CloseSession(ctx context.Context) error {
	_ = ctx
	runtime.closeCount++
	return nil
}

func (runtime *fakeBrowserRuntime) Navigate(ctx context.Context, request browserruntime.NavigateRequest) (browserruntime.NavigateResult, error) {
	_ = ctx
	runtime.navigateRequest = request
	runtime.navigateCount++
	if runtime.errorValue != nil {
		return browserruntime.NavigateResult{}, runtime.errorValue
	}
	if runtime.navigateCount <= len(runtime.navigateResults) {
		return runtime.navigateResults[runtime.navigateCount-1], nil
	}
	return browserruntime.NavigateResult{URL: request.URL}, nil
}

func (runtime *fakeBrowserRuntime) Observe(ctx context.Context, request browserruntime.ObserveRequest) (browserruntime.ObserveResult, error) {
	_ = ctx
	_ = request
	runtime.observeCount++
	if runtime.observeError != nil {
		return browserruntime.ObserveResult{}, runtime.observeError
	}
	if runtime.errorValue != nil {
		return browserruntime.ObserveResult{}, runtime.errorValue
	}
	return runtime.observeResult, nil
}

func (runtime *fakeBrowserRuntime) Screenshot(ctx context.Context, request browserruntime.ScreenshotRequest) (browserruntime.ScreenshotResult, error) {
	_ = ctx
	_ = request
	if runtime.errorValue != nil {
		return browserruntime.ScreenshotResult{}, runtime.errorValue
	}
	return runtime.screenshot, nil
}

func (runtime *fakeBrowserRuntime) Click(ctx context.Context, request browserruntime.ClickRequest) (browserruntime.ActionResult, error) {
	_ = ctx
	runtime.clickRequest = request
	if runtime.actionError != nil {
		return browserruntime.ActionResult{}, runtime.actionError
	}
	if runtime.errorValue != nil {
		return browserruntime.ActionResult{}, runtime.errorValue
	}
	return browserruntime.ActionResult{OK: true, Action: "click", Target: request.Target}, nil
}

func (runtime *fakeBrowserRuntime) Fill(ctx context.Context, request browserruntime.FillRequest) (browserruntime.ActionResult, error) {
	_ = ctx
	runtime.fillRequest = request
	if runtime.actionError != nil {
		return browserruntime.ActionResult{}, runtime.actionError
	}
	if runtime.errorValue != nil {
		return browserruntime.ActionResult{}, runtime.errorValue
	}
	return browserruntime.ActionResult{OK: true, Action: "fill", Target: request.Target}, nil
}

func (runtime *fakeBrowserRuntime) Select(ctx context.Context, request browserruntime.SelectRequest) (browserruntime.ActionResult, error) {
	_ = ctx
	runtime.selectRequest = request
	if runtime.actionError != nil {
		return browserruntime.ActionResult{}, runtime.actionError
	}
	if runtime.errorValue != nil {
		return browserruntime.ActionResult{}, runtime.errorValue
	}
	return browserruntime.ActionResult{OK: true, Action: "select", Target: request.Target}, nil
}

func (runtime *fakeBrowserRuntime) Press(ctx context.Context, request browserruntime.PressRequest) (browserruntime.ActionResult, error) {
	_ = ctx
	runtime.pressRequest = request
	if runtime.actionError != nil {
		return browserruntime.ActionResult{}, runtime.actionError
	}
	if runtime.errorValue != nil {
		return browserruntime.ActionResult{}, runtime.errorValue
	}
	return browserruntime.ActionResult{OK: true, Action: "press"}, nil
}

func (runtime *fakeBrowserRuntime) Wait(ctx context.Context, request browserruntime.WaitRequest) (browserruntime.ActionResult, error) {
	_ = ctx
	runtime.waitRequest = request
	if runtime.actionError != nil {
		return browserruntime.ActionResult{}, runtime.actionError
	}
	if runtime.errorValue != nil {
		return browserruntime.ActionResult{}, runtime.errorValue
	}
	return browserruntime.ActionResult{OK: true, Action: "wait", Target: request.Target}, nil
}

func (picker fakeFilePicker) PickFile(ctx context.Context, request FilePickRequest) (PickedFile, error) {
	_ = ctx
	_ = request
	return picker.pickedFile, picker.errorValue
}

func (uploader *fakeFileUploader) UploadFile(ctx context.Context, request FileUploadRequest) (UploadedFile, error) {
	_ = ctx
	uploader.request = request
	return UploadedFile{
		FileID:      "file-1",
		Filename:    request.Filename,
		SizeBytes:   request.SizeBytes,
		ContentType: request.ContentType,
		DevicePath:  "/tmp/internkim-companion-files/" + request.Filename,
		ExpiresAt:   "2026-04-27T00:00:00Z",
	}, nil
}

func waitForHandoffActive(t *testing.T, handoffStore *BrowserHandoffStore) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if handoffStore.Snapshot().Active {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for active handoff")
}

func completeActiveBrowserHandoffForExecutorTest(t *testing.T, store *BrowserHandoffStore, completion HandoffCompletion) {
	t.Helper()
	store.mutex.Lock()
	handoff, errorValue := store.validateCompletionLocked(completion)
	if errorValue != nil {
		store.mutex.Unlock()
		t.Fatalf("validate handoff completion: %v", errorValue)
	}
	if handoff == nil {
		store.mutex.Unlock()
		return
	}
	waiter := handoff.waiter
	store.mutex.Unlock()
	waiter <- completion
}

func TestBrowserNavigateOpensValidatedURL(t *testing.T) {
	browserRuntime := &fakeBrowserRuntime{}
	executor := Executor{BrowserRuntime: browserRuntime}

	response, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "browser.open",
		Input:    json.RawMessage(`{"url":"https://example.com/path"}`),
	})
	if errorValue != nil {
		t.Fatalf("expected browser navigate success: %v", errorValue)
	}
	if response.ToolName != "browser.open" || browserRuntime.navigateRequest.URL != "https://example.com/path" {
		t.Fatalf("unexpected browser result: response=%+v request=%+v", response, browserRuntime.navigateRequest)
	}
}

func TestBrowserNavigateRejectsNonHTTPURL(t *testing.T) {
	executor := Executor{BrowserRuntime: browserruntime.AgentBrowserRuntime{}}

	_, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "browser.open",
		Input:    json.RawMessage(`{"url":"file:///etc/passwd"}`),
	})
	if errorValue == nil {
		t.Fatal("expected non-http browser URL to fail")
	}
}

func TestBrowserObserveUsesRuntimeSchema(t *testing.T) {
	executor := Executor{BrowserRuntime: &fakeBrowserRuntime{observeResult: browserruntime.ObserveResult{
		URL:             "https://example.com",
		Title:           "Example",
		SnapshotText:    "- link \"More\" [ref=e1]",
		InteractiveRefs: []string{"@e1"},
		CapturedAt:      "2026-04-27T00:00:00Z",
	}}}

	response, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{ToolName: "browser.snapshot"})
	if errorValue != nil {
		t.Fatalf("expected observe success: %v", errorValue)
	}
	var result browserruntime.ObserveResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if result.URL != "https://example.com" || result.InteractiveRefs[0] != "@e1" {
		t.Fatalf("unexpected observe result: %+v", result)
	}
	if strings.Contains(string(response.Result), "profile") || strings.Contains(string(response.Result), "cdp") || strings.Contains(string(response.Result), "cookie") {
		t.Fatalf("observe leaked browser internals: %s", string(response.Result))
	}
}

func TestBrowserScreenshotUploadsDevicePathOnly(t *testing.T) {
	screenshotPath := writeExecutorTestFile(t, "screen.png", "png")
	uploader := &fakeFileUploader{}
	executor := Executor{
		BrowserRuntime: &fakeBrowserRuntime{screenshot: browserruntime.ScreenshotResult{
			LocalPath:   screenshotPath,
			Filename:    "screen.png",
			SizeBytes:   3,
			ContentType: "image/png",
			CapturedAt:  "2026-04-27T00:00:00Z",
		}},
		FileUploader: uploader,
	}

	response, errorValue := executor.ExecuteJob(context.Background(), JobEnvelope{JobID: "job-1", ToolName: "browser.screenshot"}, capabilities.ToolInvokeRequest{
		ToolName: "browser.screenshot",
		Input:    json.RawMessage(`{"ttlSeconds":300}`),
	})
	if errorValue != nil {
		t.Fatalf("expected screenshot success: %v", errorValue)
	}
	if uploader.request.Path != screenshotPath || uploader.request.TTLSeconds != 300 {
		t.Fatalf("unexpected screenshot upload request: %+v", uploader.request)
	}
	if strings.Contains(string(response.Result), screenshotPath) {
		t.Fatalf("screenshot leaked local path: %s", string(response.Result))
	}
	if !strings.Contains(string(response.Result), "/tmp/internkim-companion-files/screen.png") {
		t.Fatalf("screenshot did not return device path: %s", string(response.Result))
	}
}

func TestBrowserControlToolsUseRuntime(t *testing.T) {
	browserRuntime := &fakeBrowserRuntime{}
	executor := Executor{BrowserRuntime: browserRuntime}

	requests := []capabilities.ToolInvokeRequest{
		{ToolName: "browser.click", Input: json.RawMessage(`{"ref":"@e1"}`)},
		{ToolName: "browser.fill", Input: json.RawMessage(`{"target":"@e2","text":"hello"}`)},
		{ToolName: "browser.select", Input: json.RawMessage(`{"selector":"select[name=team]","value":"ops"}`)},
		{ToolName: "browser.press", Input: json.RawMessage(`{"key":"Enter"}`)},
		{ToolName: "browser.wait", Input: json.RawMessage(`{"milliseconds":250}`)},
	}
	for _, request := range requests {
		response, errorValue := executor.Execute(context.Background(), request)
		if errorValue != nil {
			t.Fatalf("expected %s success: %v", request.ToolName, errorValue)
		}
		if response.ToolName != request.ToolName {
			t.Fatalf("unexpected response tool: %+v", response)
		}
		if strings.Contains(string(response.Result), "cookie") || strings.Contains(string(response.Result), "profile") || strings.Contains(string(response.Result), "cdp") {
			t.Fatalf("browser control leaked internals: %s", string(response.Result))
		}
	}
	if browserRuntime.clickRequest.Ref != "@e1" {
		t.Fatalf("unexpected click request: %+v", browserRuntime.clickRequest)
	}
	if browserRuntime.fillRequest.Target != "@e2" || browserRuntime.fillRequest.Text != "hello" {
		t.Fatalf("unexpected fill request: %+v", browserRuntime.fillRequest)
	}
	if browserRuntime.selectRequest.Selector != "select[name=team]" || browserRuntime.selectRequest.Value != "ops" {
		t.Fatalf("unexpected select request: %+v", browserRuntime.selectRequest)
	}
	if browserRuntime.pressRequest.Key != "Enter" {
		t.Fatalf("unexpected press request: %+v", browserRuntime.pressRequest)
	}
	if browserRuntime.waitRequest.Milliseconds != 250 {
		t.Fatalf("unexpected wait request: %+v", browserRuntime.waitRequest)
	}
}

func TestBrowserControlFailureReturnsSnapshotForRecovery(t *testing.T) {
	browserRuntime := &fakeBrowserRuntime{
		actionError: errors.New("target is detached"),
		observeResult: browserruntime.ObserveResult{
			URL:             "https://console.cloud.google.com/apis/credentials?project=internkim-7373e2a4",
			Title:           "Credentials",
			SnapshotText:    "- button \"Create credential\" [ref=e27]",
			InteractiveRefs: []string{"@e27"},
			CapturedAt:      "2026-05-06T14:20:00Z",
		},
	}
	executor := Executor{BrowserRuntime: browserRuntime}

	response, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "browser.click",
		Input:    json.RawMessage(`{"target":"@old"}`),
	})
	if errorValue != nil {
		t.Fatalf("expected recoverable browser action result: %v", errorValue)
	}

	var result BrowserActionFailureResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.Status != "error" || result.Status != "recoverable_error" {
		t.Fatalf("expected recoverable error response, response=%+v result=%+v", response, result)
	}
	if result.URL != browserRuntime.observeResult.URL || result.InteractiveRefs[0] != "@e27" || !strings.Contains(result.Guidance, "반복하지 마세요") {
		t.Fatalf("expected snapshot recovery guidance, got %+v", result)
	}
	if !strings.Contains(response.Content, "Create credential") {
		t.Fatalf("expected snapshot in response content, got %s", response.Content)
	}
	if browserRuntime.observeCount != 1 {
		t.Fatalf("expected one failure snapshot, got %d", browserRuntime.observeCount)
	}
}

func TestBrowserControlFailureReportsSnapshotFailure(t *testing.T) {
	browserRuntime := &fakeBrowserRuntime{
		actionError:  errors.New("target is detached"),
		observeError: errors.New("snapshot unavailable"),
	}
	executor := Executor{BrowserRuntime: browserRuntime}

	response, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "browser.fill",
		Input:    json.RawMessage(`{"target":"@old","text":"hello"}`),
	})
	if errorValue != nil {
		t.Fatalf("expected recoverable browser action result: %v", errorValue)
	}

	var result BrowserActionFailureResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || result.SnapshotError != "snapshot unavailable" {
		t.Fatalf("expected snapshot failure in recoverable result, response=%+v result=%+v", response, result)
	}
}

func TestBrowserHandoffPausesRuntimeAndResumesAfterUserCompletion(t *testing.T) {
	handoffStore := NewBrowserHandoffStore()
	browserRuntime := &fakeBrowserRuntime{observeResult: browserruntime.ObserveResult{
		URL:             "https://example.com/app",
		Title:           "Example",
		SnapshotText:    "- button \"Continue\" [ref=e1]",
		InteractiveRefs: []string{"@e1"},
		CapturedAt:      "2026-05-07T00:00:01Z",
	}}
	executor := Executor{BrowserRuntime: browserRuntime, HandoffStore: handoffStore}
	completionErrors := completeActiveHandoffWhenReady(handoffStore, "https://example.com/app")

	response, errorValue := executor.ExecuteJob(context.Background(), JobEnvelope{JobID: "job-1", ToolName: "browser.handoff"}, capabilities.ToolInvokeRequest{
		ToolName: "browser.handoff",
		Input:    json.RawMessage(`{"url":"https://example.com/login","message":"로그인 후 완료를 눌러주세요.","successCriteria":{"textIncludesAny":["Continue"]}}`),
	})
	if completionError := <-completionErrors; completionError != nil {
		t.Fatalf("expected handoff completion: %v", completionError)
	}
	if errorValue != nil {
		t.Fatalf("expected completed handoff response: %v", errorValue)
	}

	var result BrowserHandoffResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != HandoffStateCompleted || result.State != HandoffStateCompleted || !result.CompletedByUser {
		t.Fatalf("unexpected handoff result: response=%+v result=%+v", response, result)
	}
	if result.HandoffID == "" || result.URL != "https://example.com/app" || handoffStore.Snapshot().Active {
		t.Fatalf("expected completed handoff, got %+v snapshot=%+v", result, handoffStore.Snapshot())
	}
	if result.SnapshotText == "" || len(result.InteractiveRefs) != 1 {
		t.Fatalf("expected resumed automation snapshot in handoff result, got %+v", result)
	}
	if browserRuntime.pauseCount != 1 || browserRuntime.resumeCount != 1 {
		t.Fatalf("expected exactly one pause and one resume, got pause=%d resume=%d", browserRuntime.pauseCount, browserRuntime.resumeCount)
	}
	if browserRuntime.startCount != 0 {
		t.Fatalf("expected no second browser window to open, got %d", browserRuntime.startCount)
	}
}

func TestBrowserHandoffTimeoutResumesRuntime(t *testing.T) {
	handoffStore := NewBrowserHandoffStore()
	browserRuntime := &fakeBrowserRuntime{}
	executor := Executor{BrowserRuntime: browserRuntime, HandoffStore: handoffStore}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, errorValue := executor.ExecuteJob(ctx, JobEnvelope{JobID: "job-1", ToolName: "browser.handoff"}, capabilities.ToolInvokeRequest{
		ToolName: "browser.handoff",
		Input:    json.RawMessage(`{"url":"https://example.com/login"}`),
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "timed out") {
		t.Fatalf("expected browser handoff timeout error, got %v", errorValue)
	}
	if browserRuntime.pauseCount != 1 || browserRuntime.resumeCount != 1 {
		t.Fatalf("expected pause then resume even on timeout, pause=%d resume=%d", browserRuntime.pauseCount, browserRuntime.resumeCount)
	}
	if handoffStore.Snapshot().Active {
		t.Fatal("expected handoff store to clear the active handoff after timeout")
	}
}

func TestBrowserHandoffSurfacesPauseFailureWithoutStrandingTheHandoff(t *testing.T) {
	handoffStore := NewBrowserHandoffStore()
	browserRuntime := &fakeBrowserRuntime{pauseError: errors.New("runtime does not support a human handoff")}
	executor := Executor{BrowserRuntime: browserRuntime, HandoffStore: handoffStore}

	_, errorValue := executor.ExecuteJob(context.Background(), JobEnvelope{JobID: "job-1", ToolName: "browser.handoff"}, capabilities.ToolInvokeRequest{
		ToolName: "browser.handoff",
		Input:    json.RawMessage(`{"url":"https://example.com/login"}`),
	})
	if errorValue == nil {
		t.Fatal("expected the handoff to fail when the runtime cannot pause")
	}
	if browserRuntime.pauseCount != 1 || browserRuntime.resumeCount != 0 {
		t.Fatalf("expected exactly one failed pause attempt and no resume, pause=%d resume=%d", browserRuntime.pauseCount, browserRuntime.resumeCount)
	}
	if handoffStore.Snapshot().Active {
		t.Fatal("expected the handoff store to clear the handoff rather than leaving it stranded as active")
	}
}

func TestBrowserHandoffRejectsLinuxWayland(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux Wayland detection only applies on Linux")
	}
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv("DISPLAY", "")
	executor := Executor{BrowserRuntime: &fakeBrowserRuntime{}, HandoffStore: NewBrowserHandoffStore()}

	_, errorValue := executor.ExecuteJob(context.Background(), JobEnvelope{JobID: "job-1", ToolName: "browser.handoff"}, capabilities.ToolInvokeRequest{
		ToolName: "browser.handoff",
		Input:    json.RawMessage(`{"url":"https://example.com/login"}`),
	})

	if errorValue == nil || !strings.Contains(errorValue.Error(), "Linux Wayland") {
		t.Fatalf("expected Linux Wayland unsupported error, got %v", errorValue)
	}
}

func TestBrowserHandoffSkipsPauseWhenAlreadySignedIn(t *testing.T) {
	handoffStore := NewBrowserHandoffStore()
	browserRuntime := &fakeBrowserRuntime{navigateResults: []browserruntime.NavigateResult{{
		URL:             "https://console.cloud.google.com/project",
		Title:           "Google Cloud",
		SnapshotText:    "- button \"Create\" [ref=e1]",
		InteractiveRefs: []string{"@e1"},
		CapturedAt:      "2026-05-07T00:00:00Z",
	}}}
	executor := Executor{BrowserRuntime: browserRuntime, HandoffStore: handoffStore}

	response, errorValue := executor.ExecuteJob(context.Background(), JobEnvelope{JobID: "job-1", ToolName: "browser.handoff"}, capabilities.ToolInvokeRequest{
		ToolName: "browser.handoff",
		Input:    json.RawMessage(`{"url":"https://console.cloud.google.com/project","message":"로그인이 필요하면 계속을 눌러주세요."}`),
	})
	if errorValue != nil {
		t.Fatalf("expected signed-in automation handoff response: %v", errorValue)
	}

	var result BrowserHandoffResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != HandoffStateCompleted || result.CompletedByUser || result.SnapshotText == "" {
		t.Fatalf("expected completed automation result without user handoff, response=%+v result=%+v", response, result)
	}
	if browserRuntime.pauseCount != 0 || browserRuntime.navigateCount != 1 || handoffStore.Snapshot().Active {
		t.Fatalf("expected an automation probe only, pause=%d navigate=%d snapshot=%+v", browserRuntime.pauseCount, browserRuntime.navigateCount, handoffStore.Snapshot())
	}
}

func TestBrowserHandoffPausesWhenGoogleProfileNeedsLogin(t *testing.T) {
	handoffStore := NewBrowserHandoffStore()
	browserRuntime := &fakeBrowserRuntime{
		navigateResults: []browserruntime.NavigateResult{{
			URL:          "https://accounts.google.com/v3/signin/identifier",
			Title:        "Sign in",
			SnapshotText: "Sign in with Google",
			CapturedAt:   "2026-05-07T00:00:00Z",
		}},
		observeResult: browserruntime.ObserveResult{
			URL:             "https://console.cloud.google.com/project",
			Title:           "Google Cloud",
			SnapshotText:    "- button \"Create\" [ref=e1]",
			InteractiveRefs: []string{"@e1"},
			CapturedAt:      "2026-05-07T00:00:01Z",
		},
	}
	executor := Executor{BrowserRuntime: browserRuntime, HandoffStore: handoffStore}
	completionErrors := completeActiveHandoffWhenReady(handoffStore, "https://console.cloud.google.com")

	response, errorValue := executor.ExecuteJob(context.Background(), JobEnvelope{JobID: "job-1", ToolName: "browser.handoff"}, capabilities.ToolInvokeRequest{
		ToolName: "browser.handoff",
		Input:    json.RawMessage(`{"url":"https://console.cloud.google.com/project","message":"로그인 후 계속을 눌러주세요."}`),
	})
	if completionError := <-completionErrors; completionError != nil {
		t.Fatalf("expected handoff completion: %v", completionError)
	}
	if errorValue != nil {
		t.Fatalf("expected paused handoff response: %v", errorValue)
	}

	var result BrowserHandoffResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !result.CompletedByUser || result.SnapshotText == "" {
		t.Fatalf("expected user-completed handoff with resumed snapshot, got %+v", result)
	}
	if browserRuntime.pauseCount != 1 || browserRuntime.resumeCount != 1 || browserRuntime.startCount != 0 {
		t.Fatalf("expected the runtime to pause/resume in place without opening a second window, pause=%d resume=%d start=%d", browserRuntime.pauseCount, browserRuntime.resumeCount, browserRuntime.startCount)
	}
}

func TestBrowserHandoffReusesActiveHandoffWithoutReopeningBrowser(t *testing.T) {
	handoffStore := NewBrowserHandoffStore()
	activeHandoff, errorValue := handoffStore.Begin(BrowserHandoffRequest{URL: "https://example.com/login"}, "internkim")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	browserRuntime := &fakeBrowserRuntime{}
	executor := Executor{BrowserRuntime: browserRuntime, HandoffStore: handoffStore}
	completeActiveBrowserHandoffForExecutorTest(t, handoffStore, HandoffCompletion{
		HandoffID:  activeHandoff.HandoffID,
		SessionID:  activeHandoff.SessionID,
		URL:        "https://example.com/app",
		CapturedAt: time.Now().UTC().Format(time.RFC3339),
	})

	response, errorValue := executor.ExecuteJob(context.Background(), JobEnvelope{JobID: "job-2", ToolName: "browser.handoff"}, capabilities.ToolInvokeRequest{
		ToolName: "browser.handoff",
		Input:    json.RawMessage(`{"url":"https://example.com/projectselector2/iam-admin/serviceaccounts?supportedpurview=project","message":"로그인 후 완료를 눌러주세요."}`),
	})
	if errorValue != nil {
		t.Fatalf("expected active handoff reuse: %v", errorValue)
	}
	handoffStore.End(activeHandoff.HandoffID, HandoffStateCompleted)

	var result BrowserHandoffResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != HandoffStateCompleted || result.HandoffID != activeHandoff.HandoffID {
		t.Fatalf("expected completed response for active handoff, response=%+v result=%+v", response, result)
	}
	if browserRuntime.startCount != 0 {
		t.Fatalf("expected active handoff reuse not to reopen browser, got %d opens", browserRuntime.startCount)
	}
}

func completeActiveHandoffWhenReady(handoffStore *BrowserHandoffStore, pageURL string) <-chan error {
	completionErrors := make(chan error, 1)
	go func() {
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			snapshot := handoffStore.Snapshot()
			if snapshot.Active {
				completionErrors <- handoffStore.Complete(HandoffCompletion{
					HandoffID:  snapshot.HandoffID,
					SessionID:  snapshot.SessionID,
					URL:        pageURL,
					CapturedAt: time.Now().UTC().Format(time.RFC3339),
				})
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		completionErrors <- errors.New("timed out waiting for active handoff")
	}()
	return completionErrors
}

func TestHandoffBridgeCompletionCapturesSnapshot(t *testing.T) {
	handoffStore := NewBrowserHandoffStore()
	handoff, errorValue := handoffStore.Begin(BrowserHandoffRequest{URL: "https://example.com/login"}, "internkim")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	browserRuntime := &fakeBrowserRuntime{observeResult: browserruntime.ObserveResult{
		URL:             "https://example.com/app",
		Title:           "Example",
		SnapshotText:    "- button \"Continue\" [ref=e1]",
		InteractiveRefs: []string{"@e1"},
		CapturedAt:      "2026-04-27T00:00:00Z",
	}}
	var capturedCompletion HandoffCompletion
	handler := HandoffBridgeHandler{
		Store:          handoffStore,
		BrowserRuntime: browserRuntime,
		CompletionHandler: func(ctx context.Context, completion HandoffCompletion) error {
			_ = ctx
			capturedCompletion = completion
			return nil
		},
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/browser/handoff/complete", strings.NewReader(`{"handoffID":"`+handoff.HandoffID+`","sessionID":"`+handoff.SessionID+`","url":"https://example.com/login"}`))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected handoff completion success, got %d %s", response.Code, response.Body.String())
	}
	if capturedCompletion.URL != "https://example.com/app" || capturedCompletion.SnapshotText == "" || len(capturedCompletion.InteractiveRefs) != 1 {
		t.Fatalf("expected captured completion snapshot, got %+v", capturedCompletion)
	}
	if handoffStore.Snapshot().Active {
		t.Fatal("expected overlay to close after completion")
	}
	duplicateRequest := httptest.NewRequest(http.MethodPost, "/v1/browser/handoff/complete", strings.NewReader(`{"handoffID":"`+handoff.HandoffID+`","sessionID":"`+handoff.SessionID+`","url":"https://example.com/app"}`))
	duplicateResponse := httptest.NewRecorder()
	handler.ServeHTTP(duplicateResponse, duplicateRequest)
	if duplicateResponse.Code != http.StatusOK {
		t.Fatalf("expected duplicate completion success, got %d %s", duplicateResponse.Code, duplicateResponse.Body.String())
	}
}

func TestUserConfirmUsesPromptHandler(t *testing.T) {
	executor := Executor{
		PromptHandler: TerminalPromptHandler{
			Reader: strings.NewReader("yes\n"),
			Writer: &strings.Builder{},
		},
	}

	response, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "user.confirm",
		Input:    json.RawMessage(`{"message":"continue?"}`),
	})
	if errorValue != nil {
		t.Fatalf("expected confirm success: %v", errorValue)
	}
	var result struct {
		Confirmed bool `json:"confirmed"`
	}
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !result.Confirmed {
		t.Fatal("expected confirmation to be true")
	}
}

func TestExecutorRoutesTextLLMThroughChain(t *testing.T) {
	chain := &stubLLMChain{textResponse: llmbackend.Response{
		Provider:        "ollama",
		Model:           "gemma3:1b",
		Content:         "hello back",
		SelectedBackend: "ollama",
	}}
	executor := Executor{LLMChain: chain}

	requestBody, errorValue := json.Marshal(llmbackend.TextRequest{
		Messages: []llmbackend.Message{{Role: "user", Content: "ping"}},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	response, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "llm.text",
		Input:    requestBody,
	})
	if errorValue != nil {
		t.Fatalf("expected chain dispatch: %v", errorValue)
	}
	if chain.receivedText == nil || chain.receivedText.Messages[0].Content != "ping" {
		t.Fatalf("expected chain to receive prompt, got %+v", chain.receivedText)
	}
	var responseDocument llmbackend.Response
	if errorValue := json.Unmarshal(response.Result, &responseDocument); errorValue != nil {
		t.Fatal(errorValue)
	}
	if responseDocument.Content != "hello back" {
		t.Fatalf("expected chain content, got %q", responseDocument.Content)
	}
	if responseDocument.SelectedBackend != capabilities.LLMBackendCompanionLocal {
		t.Fatalf("expected companion-local selection, got %q", responseDocument.SelectedBackend)
	}
}

func TestExecutorRoutesStructuredLLMThroughChain(t *testing.T) {
	chain := &stubLLMChain{structuredResponse: llmbackend.Response{
		Provider:       "ollama",
		Content:        `{"reply":"ok"}`,
		ConstraintMode: "openai_json_schema",
	}}
	executor := Executor{LLMChain: chain}

	requestBody, errorValue := json.Marshal(llmbackend.StructuredRequest{
		Messages: []llmbackend.Message{{Role: "user", Content: "ping"}},
		StructuredOutputSchema: llmbackend.StructuredOutputSchema{
			Name:     "reply",
			Document: json.RawMessage(`{"type":"object","required":["reply"]}`),
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	response, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "llm.structured",
		Input:    requestBody,
	})
	if errorValue != nil {
		t.Fatalf("expected chain dispatch: %v", errorValue)
	}
	if chain.receivedStructured == nil || chain.receivedStructured.StructuredOutputSchema.Name != "reply" {
		t.Fatalf("expected chain to receive schema, got %+v", chain.receivedStructured)
	}
	var responseDocument llmbackend.Response
	if errorValue := json.Unmarshal(response.Result, &responseDocument); errorValue != nil {
		t.Fatal(errorValue)
	}
	if responseDocument.ConstraintMode != "openai_json_schema" {
		t.Fatalf("expected OpenAI JSON schema mode, got %q", responseDocument.ConstraintMode)
	}
}

func TestAttentionTriageUsesCompanionStructuredLLM(t *testing.T) {
	chain := &stubLLMChain{structuredResponse: llmbackend.Response{
		Provider: "stub",
		Model:    "stub-model",
		Content: `{
			"shouldEscalate":true,
			"importance":"high",
			"confidence":0.91,
			"reasonCodes":["blocked"],
			"summaryForRemote":"User confirmation is still pending.",
			"privacyClass":"user_input"
		}`,
	}}
	inputDocument, errorValue := json.Marshal(capabilities.AttentionTriageRequest{
		JobID:             "job-1",
		ToolName:          "user.confirm",
		Status:            "pending",
		RequesterEmail:    "alice@example.com",
		PrivacyClass:      "user_input",
		WatchAttemptCount: 1,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	executor := Executor{LLMChain: chain}
	response, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: capabilities.AttentionTriageToolName,
		Input:    inputDocument,
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "alice@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var decision capabilities.AttentionTriageDecision
	if errorValue := json.Unmarshal(response.Result, &decision); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !decision.ShouldEscalate || decision.Importance != "high" || decision.Confidence != 0.91 {
		t.Fatalf("unexpected attention decision: %+v", decision)
	}
	if chain.receivedStructured == nil || chain.receivedStructured.StructuredOutputSchema.Name != "companion_attention_triage" {
		t.Fatalf("expected attention triage schema, got %+v", chain.receivedStructured)
	}
}

func TestAttentionTriageReportsUnavailableLocalLLM(t *testing.T) {
	_, errorValue := Executor{}.Execute(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: capabilities.AttentionTriageToolName,
		Input:    json.RawMessage(`{"jobID":"job-1","toolName":"user.confirm","status":"pending"}`),
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "companion LLM is not configured") {
		t.Fatalf("expected local LLM configuration error, got %v", errorValue)
	}
}

func TestAttentionTriageReturnsBackendFailure(t *testing.T) {
	executor := Executor{LLMChain: &stubLLMChain{structuredError: errors.New("backend unavailable")}}
	_, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: capabilities.AttentionTriageToolName,
		Input:    json.RawMessage(`{"jobID":"job-1","toolName":"user.confirm","status":"pending"}`),
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "backend unavailable") {
		t.Fatalf("expected backend error, got %v", errorValue)
	}
}

func TestExecutorRoutesEmbeddingThroughChain(t *testing.T) {
	chain := &stubEmbeddingChain{response: llmbackend.EmbeddingResponse{
		Provider:        "ollama",
		Model:           llmbackend.DefaultEmbeddingGemmaModel,
		SelectedBackend: "ollama",
		Embedding:       []float64{0.1, 0.2},
	}}
	executor := Executor{EmbeddingChain: chain}

	requestBody, errorValue := json.Marshal(llmbackend.EmbeddingRequest{
		Input:     "ping",
		InputType: "query",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	response, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "embedding.create",
		Input:    requestBody,
	})
	if errorValue != nil {
		t.Fatalf("expected embedding chain dispatch: %v", errorValue)
	}
	if chain.receivedRequest == nil || chain.receivedRequest.Input != "ping" {
		t.Fatalf("expected chain to receive embedding request, got %+v", chain.receivedRequest)
	}
	var responseDocument llmbackend.EmbeddingResponse
	if errorValue := json.Unmarshal(response.Result, &responseDocument); errorValue != nil {
		t.Fatal(errorValue)
	}
	if responseDocument.SelectedBackend != capabilities.LLMBackendCompanionLocal {
		t.Fatalf("expected companion-local selection, got %q", responseDocument.SelectedBackend)
	}
	if len(responseDocument.Embedding) != 2 || responseDocument.Embedding[0] != 0.1 {
		t.Fatalf("unexpected embedding response: %+v", responseDocument)
	}
}

func TestExecutorTextLLMRequiresChainOrMockMode(t *testing.T) {
	executor := Executor{}
	_, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "llm.text",
		Input:    json.RawMessage(`{"messages":[]}`),
	})
	if errorValue == nil {
		t.Fatal("expected missing chain to fail")
	}
	if !strings.Contains(errorValue.Error(), "not configured") {
		t.Fatalf("expected configuration error, got %v", errorValue)
	}
}

func TestUserConfirmWithoutPromptHandlerFailsSafely(t *testing.T) {
	executor := Executor{}

	_, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{ToolName: "user.confirm"})
	if errorValue == nil {
		t.Fatal("expected missing prompt handler to fail")
	}
	if strings.Contains(errorValue.Error(), "token") || strings.Contains(errorValue.Error(), "secret") {
		t.Fatalf("unexpected sensitive error: %v", errorValue)
	}
}

func TestFilePickUploadsSelectedFile(t *testing.T) {
	filePath := writeExecutorTestFile(t, "report.pdf", "hello")
	uploader := &fakeFileUploader{}
	executor := Executor{
		FilePicker:   fakeFilePicker{pickedFile: PickedFile{Path: filePath}},
		FileUploader: uploader,
	}

	response, errorValue := executor.ExecuteJob(context.Background(), JobEnvelope{JobID: "job-1", ToolName: "file.pick"}, capabilities.ToolInvokeRequest{
		ToolName: "file.pick",
		Input:    json.RawMessage(`{"allowedExtensions":["pdf"],"ttlSeconds":600}`),
	})
	if errorValue != nil {
		t.Fatalf("expected file pick success: %v", errorValue)
	}
	if uploader.request.JobID != "job-1" || uploader.request.Filename != "report.pdf" || uploader.request.TTLSeconds != 600 {
		t.Fatalf("unexpected upload request: %+v", uploader.request)
	}
	var result UploadedFile
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if result.DevicePath != "/tmp/internkim-companion-files/report.pdf" {
		t.Fatalf("unexpected device path: %s", result.DevicePath)
	}
}

func TestFilePickCancelReturnsDenialObservation(t *testing.T) {
	executor := Executor{
		FilePicker:   fakeFilePicker{errorValue: ErrFilePickCanceled},
		FileUploader: &fakeFileUploader{},
	}

	_, errorValue := executor.ExecuteJob(context.Background(), JobEnvelope{JobID: "job-1", ToolName: "file.pick"}, capabilities.ToolInvokeRequest{ToolName: "file.pick"})
	denialError, ok := errorValue.(DenialError)
	if !ok {
		t.Fatalf("expected denial error, got %v", errorValue)
	}
	if denialError.Denial.Code != "user_cancelled" || denialError.Denial.ToolName != "file.pick" {
		t.Fatalf("unexpected denial: %+v", denialError.Denial)
	}
}

func TestFilePickValidatesExtensionAndSize(t *testing.T) {
	filePath := writeExecutorTestFile(t, "secret.txt", "hello")
	executor := Executor{
		FilePicker:   fakeFilePicker{pickedFile: PickedFile{Path: filePath}},
		FileUploader: &fakeFileUploader{},
	}

	_, extensionError := executor.ExecuteJob(context.Background(), JobEnvelope{JobID: "job-1", ToolName: "file.pick"}, capabilities.ToolInvokeRequest{
		ToolName: "file.pick",
		Input:    json.RawMessage(`{"allowedExtensions":["pdf"]}`),
	})
	if extensionError == nil {
		t.Fatal("expected extension validation error")
	}
	_, sizeError := executor.ExecuteJob(context.Background(), JobEnvelope{JobID: "job-1", ToolName: "file.pick"}, capabilities.ToolInvokeRequest{
		ToolName: "file.pick",
		Input:    json.RawMessage(`{"maxBytes":1}`),
	})
	if sizeError == nil {
		t.Fatal("expected size validation error")
	}
}

func TestShellBridgePromptHandlerConfirm(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/v1/user/confirm" {
			t.Fatalf("unexpected bridge path: %s", request.URL.Path)
		}
		if request.Header.Get("X-InternKim-Shell-Bridge-Token") != "bridge-token" {
			t.Fatalf("expected shell bridge token header")
		}
		return textResponse(http.StatusOK, `{"confirmed":true}`), nil
	})}
	handler := ShellBridgePromptHandler{
		BaseURL:    "http://127.0.0.1:1234",
		Token:      "bridge-token",
		HTTPClient: httpClient,
	}

	confirmed, errorValue := handler.Confirm(context.Background(), "continue?", false)
	if errorValue != nil {
		t.Fatalf("expected shell bridge confirm success: %v", errorValue)
	}
	if !confirmed {
		t.Fatal("expected shell bridge confirmation")
	}
}

func TestShellBridgePromptHandlerInput(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/v1/user/input" {
			t.Fatalf("unexpected bridge path: %s", request.URL.Path)
		}
		return textResponse(http.StatusOK, `{"text":"approved text"}`), nil
	})}
	handler := ShellBridgePromptHandler{
		BaseURL:    "http://127.0.0.1:1234",
		HTTPClient: httpClient,
	}

	text, errorValue := handler.Input(context.Background(), "value?")
	if errorValue != nil {
		t.Fatalf("expected shell bridge input success: %v", errorValue)
	}
	if text != "approved text" {
		t.Fatalf("unexpected shell bridge input: %s", text)
	}
}

func TestShellBridgePromptHandlerPickFile(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/v1/file/pick" {
			t.Fatalf("unexpected bridge path: %s", request.URL.Path)
		}
		return textResponse(http.StatusOK, `{"path":"/tmp/report.txt"}`), nil
	})}
	handler := ShellBridgePromptHandler{
		BaseURL:    "http://127.0.0.1:1234",
		HTTPClient: httpClient,
	}

	pickedFile, errorValue := handler.PickFile(context.Background(), FilePickRequest{Title: "Pick"})
	if errorValue != nil {
		t.Fatalf("expected shell bridge file pick success: %v", errorValue)
	}
	if pickedFile.Path != "/tmp/report.txt" {
		t.Fatalf("unexpected file path: %s", pickedFile.Path)
	}
}

func TestShellBridgePromptHandlerPickFileCancel(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return textResponse(http.StatusOK, `{"cancelled":true}`), nil
	})}
	handler := ShellBridgePromptHandler{
		BaseURL:    "http://127.0.0.1:1234",
		HTTPClient: httpClient,
	}

	_, errorValue := handler.PickFile(context.Background(), FilePickRequest{})
	if !errors.Is(errorValue, ErrFilePickCanceled) {
		t.Fatalf("expected cancel error, got %v", errorValue)
	}
}

func TestShellBridgePromptHandlerRejectsNonLocalURL(t *testing.T) {
	handler := ShellBridgePromptHandler{BaseURL: "https://device.example.test"}

	_, errorValue := handler.Confirm(context.Background(), "continue?", false)
	if errorValue == nil {
		t.Fatal("expected non-local bridge to fail")
	}
	if strings.Contains(errorValue.Error(), "token") || strings.Contains(errorValue.Error(), "secret") {
		t.Fatalf("unexpected sensitive error: %v", errorValue)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func textResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestMockStructuredLLMUsesSchemaRequiredKeys(t *testing.T) {
	executor := Executor{DevMockLLM: true}

	response, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "llm.structured",
		Input:    json.RawMessage(`{"structuredOutputSchema":{"document":{"required":["reply"]}}}`),
	})
	if errorValue != nil {
		t.Fatalf("expected mock structured success: %v", errorValue)
	}
	var llmResponse struct {
		Content string `json:"content"`
	}
	if errorValue := json.Unmarshal(response.Result, &llmResponse); errorValue != nil {
		t.Fatal(errorValue)
	}
	if llmResponse.Content != `{"reply":"ok"}` {
		t.Fatalf("unexpected mock content: %s", llmResponse.Content)
	}
}

func writeExecutorTestFile(t *testing.T, filename string, document string) string {
	t.Helper()
	path := t.TempDir() + "/" + filename
	if errorValue := os.WriteFile(path, []byte(document), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return path
}
