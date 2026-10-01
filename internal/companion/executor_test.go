package companion

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	browserruntime "gitlab.com/eastriver/internkim/internal/browser"
	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/internal/llmbackend"
	capabilityschema "gitlab.com/eastriver/internkim/pkg/capabilityprotocol/jsonschema"
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
	observeCount    int
	startCount      int
	navigateCount   int
	closeCount      int
	screenshot      browserruntime.ScreenshotResult
	errorValue      error
	actionError     error
	observeError    error
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
		ToolName: "browser_open",
		Input:    json.RawMessage(`{"url":"https://example.com/path"}`),
	})
	if errorValue != nil {
		t.Fatalf("expected browser navigate success: %v", errorValue)
	}
	if response.ToolName != "browser_open" || browserRuntime.navigateRequest.URL != "https://example.com/path" {
		t.Fatalf("unexpected browser result: response=%+v request=%+v", response, browserRuntime.navigateRequest)
	}
}

func TestBrowserNavigateRejectsNonHTTPURL(t *testing.T) {
	executor := Executor{BrowserRuntime: browserruntime.AgentBrowserRuntime{}}

	_, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "browser_open",
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

	response, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{ToolName: "browser_snapshot"})
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

	response, errorValue := executor.ExecuteJob(context.Background(), JobEnvelope{JobID: "job-1", ToolName: "browser_screenshot"}, capabilities.ToolInvokeRequest{
		ToolName: "browser_screenshot",
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
	if response.Outcome != capabilities.ToolOutcomeSucceeded || response.Status != "ok" {
		t.Fatalf("unexpected screenshot identity: %+v", response)
	}
}

func TestBrowserControlToolsUseRuntime(t *testing.T) {
	browserRuntime := &fakeBrowserRuntime{}
	executor := Executor{BrowserRuntime: browserRuntime}

	requests := []capabilities.ToolInvokeRequest{
		{ToolName: "browser_click", Input: json.RawMessage(`{"ref":"@e1"}`)},
		{ToolName: "browser_fill", Input: json.RawMessage(`{"target":"@e2","text":"hello"}`)},
		{ToolName: "browser_select", Input: json.RawMessage(`{"selector":"select[name=team]","value":"ops"}`)},
		{ToolName: "browser_press", Input: json.RawMessage(`{"key":"Enter"}`)},
		{ToolName: "browser_wait", Input: json.RawMessage(`{"milliseconds":250}`)},
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
		ToolName: "browser_click",
		Input:    json.RawMessage(`{"target":"@old"}`),
	})
	if errorValue != nil {
		t.Fatalf("expected recoverable browser action result: %v", errorValue)
	}

	var result BrowserActionFailureResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.Status != "error" || response.Outcome != capabilities.ToolOutcomeFailed || result.Status != "recoverable_error" {
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

func TestBrowserControlAnswersHoldTheirResultContracts(t *testing.T) {
	executor := Executor{BrowserRuntime: browserruntime.AgentBrowserRuntime{
		CommandPath: "agent-browser",
		SessionName: "internkim",
		Runner:      silentCommandRunner{},
	}}
	cases := []struct {
		toolName string
		input    string
	}{
		{toolName: "browser_click", input: `{"ref":"@e1"}`},
		{toolName: "browser_fill", input: `{"ref":"@e1","text":"hello"}`},
		{toolName: "browser_select", input: `{"selector":"#city","value":"Seoul"}`},
		{toolName: "browser_press", input: `{"key":"Enter"}`},
		{toolName: "browser_wait", input: `{"target":"@e2"}`},
		{toolName: "browser_wait", input: `{"milliseconds":500}`},
	}
	for _, testCase := range cases {
		t.Run(testCase.toolName, func(t *testing.T) {
			response, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{
				ToolName: testCase.toolName,
				Input:    json.RawMessage(testCase.input),
			})
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if response.Outcome != capabilities.ToolOutcomeSucceeded || response.IsError {
				t.Fatalf("response = %+v result=%s", response, response.Result)
			}
			expectResultHoldsCompanionContract(t, testCase.toolName, response.Result)
		})
	}
}

type silentCommandRunner struct{}

func (silentCommandRunner) Run(context.Context, string, []string) ([]byte, error) {
	return nil, nil
}

func expectResultHoldsCompanionContract(t *testing.T, toolName string, result json.RawMessage) {
	t.Helper()
	for _, descriptor := range capabilities.CompanionToolDescriptors() {
		if descriptor.Name != toolName {
			continue
		}
		if descriptor.ResultContract == nil {
			t.Fatalf("%s has no result contract", toolName)
		}
		check, errorValue := capabilityschema.ValidateResult(descriptor.ResultContract.Schema, result)
		if errorValue != nil {
			t.Fatalf("%s answered outside its contract: %v result=%s", toolName, errorValue, result)
		}
		if len(check.UnknownFields) > 0 {
			t.Fatalf("%s answered fields its contract does not name: %v", toolName, check.UnknownFields)
		}
		return
	}
	t.Fatalf("%s is not a companion tool", toolName)
}

func TestBrowserControlFailureReportsSnapshotFailure(t *testing.T) {
	browserRuntime := &fakeBrowserRuntime{
		actionError:  errors.New("target is detached"),
		observeError: errors.New("snapshot unavailable"),
	}
	executor := Executor{BrowserRuntime: browserRuntime}

	response, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "browser_fill",
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
		ToolName: "llm_text",
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
		ToolName: "llm_structured",
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
		ToolName:          "user_confirm",
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
		Input:    json.RawMessage(`{"jobID":"job-1","toolName":"user_confirm","status":"pending"}`),
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "companion LLM is not configured") {
		t.Fatalf("expected local LLM configuration error, got %v", errorValue)
	}
}

func TestAttentionTriageReturnsBackendFailure(t *testing.T) {
	executor := Executor{LLMChain: &stubLLMChain{structuredError: errors.New("backend unavailable")}}
	_, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: capabilities.AttentionTriageToolName,
		Input:    json.RawMessage(`{"jobID":"job-1","toolName":"user_confirm","status":"pending"}`),
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "backend unavailable") {
		t.Fatalf("expected backend error, got %v", errorValue)
	}
}

func TestExecutorRoutesEmbeddingThroughChain(t *testing.T) {
	chain := &stubEmbeddingChain{response: llmbackend.EmbeddingResponse{
		Provider:        "ollama",
		Model:           llmbackend.DefaultEmbeddingModelName,
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
		ToolName: "embedding_create",
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
		ToolName: "llm_text",
		Input:    json.RawMessage(`{"messages":[]}`),
	})
	if errorValue == nil {
		t.Fatal("expected missing chain to fail")
	}
	if !strings.Contains(errorValue.Error(), "not configured") {
		t.Fatalf("expected configuration error, got %v", errorValue)
	}
}

func TestMockStructuredLLMUsesSchemaRequiredKeys(t *testing.T) {
	executor := Executor{DevMockLLM: true}

	response, errorValue := executor.Execute(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "llm_structured",
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
