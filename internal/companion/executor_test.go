package companion

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	browserruntime "github.com/anthropic-lab/internkim/internal/browser"
	"github.com/anthropic-lab/internkim/internal/capabilities"
)

type fakeBrowserRuntime struct {
	startRequest    browserruntime.SessionStartRequest
	navigateRequest browserruntime.NavigateRequest
	observeResult   browserruntime.ObserveResult
	screenshot      browserruntime.ScreenshotResult
	errorValue      error
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
	if runtime.errorValue != nil {
		return browserruntime.SessionStartResult{}, runtime.errorValue
	}
	return browserruntime.SessionStartResult{SessionID: "internkim", Opened: true, URL: firstNonEmpty(request.URL, request.StartURL)}, nil
}

func (runtime *fakeBrowserRuntime) Navigate(ctx context.Context, request browserruntime.NavigateRequest) (browserruntime.NavigateResult, error) {
	_ = ctx
	runtime.navigateRequest = request
	if runtime.errorValue != nil {
		return browserruntime.NavigateResult{}, runtime.errorValue
	}
	return browserruntime.NavigateResult{URL: request.URL}, nil
}

func (runtime *fakeBrowserRuntime) Observe(ctx context.Context, request browserruntime.ObserveRequest) (browserruntime.ObserveResult, error) {
	_ = ctx
	_ = request
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

func TestBrowserNavigateOpensValidatedURL(t *testing.T) {
	browserRuntime := &fakeBrowserRuntime{}
	executor := Executor{BrowserRuntime: browserRuntime}

	response, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "browser.navigate",
		Input:    json.RawMessage(`{"url":"https://example.com/path"}`),
	})
	if errorValue != nil {
		t.Fatalf("expected browser navigate success: %v", errorValue)
	}
	if response.ToolName != "browser.navigate" || browserRuntime.navigateRequest.URL != "https://example.com/path" {
		t.Fatalf("unexpected browser result: response=%+v request=%+v", response, browserRuntime.navigateRequest)
	}
}

func TestBrowserNavigateRejectsNonHTTPURL(t *testing.T) {
	executor := Executor{BrowserRuntime: browserruntime.AgentBrowserRuntime{}}

	_, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "browser.navigate",
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

	response, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{ToolName: "browser.observe"})
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
		return textResponse(http.StatusOK, `{"confirmed":true}`), nil
	})}
	handler := ShellBridgePromptHandler{
		BaseURL:    "http://127.0.0.1:1234",
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
	handler := ShellBridgePromptHandler{BaseURL: "https://device.intern.kim"}

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
