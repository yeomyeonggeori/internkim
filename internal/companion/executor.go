package companion

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	browserruntime "gitlab.com/eastriver/internkim/internal/browser"
	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/internal/llmbackend"
)

type PromptHandler interface {
	Confirm(ctx context.Context, message string, defaultValue bool) (bool, error)
	Input(ctx context.Context, message string) (string, error)
}

type FilePicker interface {
	PickFile(ctx context.Context, request FilePickRequest) (PickedFile, error)
}

type DirectoryPicker interface {
	PickDirectory(ctx context.Context, request DirectoryPickRequest) (PickedDirectory, error)
}

// browserHandoffPauser is implemented by browser.ExtensionInputRuntime.
// Pausing stops OS-level input synthesis on the runtime's already-open
// browser window so a human can take over without a second window being
// opened; resuming restores normal automation once the handoff completes.
type browserHandoffPauser interface {
	Pause(ctx context.Context) error
	Resume(ctx context.Context) error
}

type BrowserActionFailureResult struct {
	Status          string   `json:"status"`
	ToolName        string   `json:"toolName"`
	Error           string   `json:"error"`
	Guidance        string   `json:"guidance"`
	URL             string   `json:"url,omitempty"`
	Title           string   `json:"title,omitempty"`
	SnapshotText    string   `json:"snapshotText,omitempty"`
	InteractiveRefs []string `json:"interactiveRefs,omitempty"`
	CapturedAt      string   `json:"capturedAt,omitempty"`
	SnapshotError   string   `json:"snapshotError,omitempty"`
}

type FileUploader interface {
	UploadFile(ctx context.Context, request FileUploadRequest) (UploadedFile, error)
}

type Executor struct {
	DevMockLLM      bool
	LLMChain        llmbackend.Provider
	EmbeddingChain  llmbackend.EmbeddingProvider
	BrowserRuntime  browserruntime.Runtime
	HandoffStore    *BrowserHandoffStore
	PromptHandler   PromptHandler
	FilePicker      FilePicker
	DirectoryPicker DirectoryPicker
	FileUploader    FileUploader
	MountStore      *MountStore
	ApprovalHandler ApprovalHandler
	GrantStore      *MemoryGrantStore
}

// NewExecutor builds an Executor from the dependencies known at companion
// startup. PromptHandler, ApprovalHandler, FilePicker, DirectoryPicker, and
// FileUploader are intentionally left unset here: the caller wires them in
// afterward only when the matching optional CLI flag (--allow-stdin-prompts,
// --development-auto-approve-browser, --shell-bridge-url) is present.
func NewExecutor(
	devMockLLM bool,
	llmChain llmbackend.Provider,
	embeddingChain llmbackend.EmbeddingProvider,
	browserRuntime browserruntime.Runtime,
	handoffStore *BrowserHandoffStore,
	mountStore *MountStore,
	grantStore *MemoryGrantStore,
) Executor {
	return Executor{
		DevMockLLM:     devMockLLM,
		LLMChain:       llmChain,
		EmbeddingChain: embeddingChain,
		BrowserRuntime: browserRuntime,
		HandoffStore:   handoffStore,
		MountStore:     mountStore,
		GrantStore:     grantStore,
	}
}

type TerminalPromptHandler struct {
	Reader io.Reader
	Writer io.Writer
}

type FilePickRequest struct {
	Title             string   `json:"title,omitempty"`
	AllowedExtensions []string `json:"allowedExtensions,omitempty"`
	MaxBytes          int64    `json:"maxBytes,omitempty"`
	TTLSeconds        int      `json:"ttlSeconds,omitempty"`
}

type PickedFile struct {
	Path        string `json:"path"`
	Filename    string `json:"filename,omitempty"`
	SizeBytes   int64  `json:"sizeBytes,omitempty"`
	ContentType string `json:"contentType,omitempty"`
}

type FileUploadRequest struct {
	JobID       string
	Path        string
	Filename    string
	SizeBytes   int64
	ContentType string
	TTLSeconds  int
}

type UploadedFile struct {
	FileID      string `json:"fileID"`
	Filename    string `json:"filename"`
	SizeBytes   int64  `json:"sizeBytes"`
	ContentType string `json:"contentType"`
	DevicePath  string `json:"devicePath"`
	ExpiresAt   string `json:"expiresAt"`
}

var ErrFilePickCanceled = errors.New("file pick canceled")

type executorToolHandler func(Executor, context.Context, JobEnvelope, capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error)

var executorToolHandlers = map[string]executorToolHandler{
	"llm.text":                           executorRequestHandler(Executor.executeTextLLM),
	"llm.structured":                     executorRequestHandler(Executor.executeStructuredLLM),
	"embedding.create":                   executorRequestHandler(Executor.executeEmbedding),
	capabilities.AttentionTriageToolName: executorRequestHandler(Executor.executeAttentionTriage),
	"browser.open":                       executorRequestHandler(Executor.executeBrowserNavigate),
	"browser.snapshot":                   executorRequestHandler(Executor.executeBrowserObserve),
	"browser.screenshot":                 Executor.executeBrowserScreenshot,
	"browser.handoff":                    Executor.executeBrowserHandoff,
	"browser.click":                      executorRequestHandler(Executor.executeBrowserClick),
	"browser.fill":                       executorRequestHandler(Executor.executeBrowserFill),
	"browser.select":                     executorRequestHandler(Executor.executeBrowserSelect),
	"browser.press":                      executorRequestHandler(Executor.executeBrowserPress),
	"browser.wait":                       executorRequestHandler(Executor.executeBrowserWait),
	"user.confirm":                       executorRequestHandler(Executor.executeUserConfirm),
	"user.input":                         executorRequestHandler(Executor.executeUserInput),
	"file.pick":                          Executor.executeFilePick,
	"filesystem.mount.create":            executorRequestHandler(Executor.executeMountCreate),
	"filesystem.mount.list":              executorSimpleHandler(Executor.executeMountList),
	"filesystem.mount.pause":             executorSimpleHandler(Executor.executeMountPause),
	"filesystem.mount.resume":            executorSimpleHandler(Executor.executeMountResume),
	"filesystem.mount.revoke":            executorSimpleHandler(Executor.executeMountRevoke),
	"filesystem.mount.status":            executorSimpleHandler(Executor.executeMountStatus),
	"filesystem.mount.stat":              executorSimpleHandler(Executor.executeMountStat),
	"filesystem.mount.list_directory":    executorSimpleHandler(Executor.executeMountListDirectory),
	"filesystem.mount.read":              executorSimpleHandler(Executor.executeMountRead),
	"filesystem.mount.write":             executorSimpleHandler(Executor.executeMountWrite),
	"filesystem.mount.mkdir":             executorSimpleHandler(Executor.executeMountMakeDirectory),
	"filesystem.mount.rename":            executorSimpleHandler(Executor.executeMountRename),
	"filesystem.mount.delete":            executorSimpleHandler(Executor.executeMountDelete),
	"filesystem.mount.truncate":          executorSimpleHandler(Executor.executeMountTruncate),
	"filesystem.mount.chmod":             executorSimpleHandler(Executor.executeMountChangeMode),
	"filesystem.mount.watch":             executorSimpleHandler(Executor.executeMountWatch),
}

func executorRequestHandler(handler func(Executor, context.Context, capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error)) executorToolHandler {
	return func(executor Executor, ctx context.Context, envelope JobEnvelope, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
		return handler(executor, ctx, request)
	}
}

func executorSimpleHandler(handler func(Executor, capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error)) executorToolHandler {
	return func(executor Executor, ctx context.Context, envelope JobEnvelope, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
		return handler(executor, request)
	}
}

func (executor Executor) Execute(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	return executor.ExecuteJob(ctx, JobEnvelope{ToolName: request.ToolName, ResourceScope: request.ResourceScope}, request)
}

func (executor Executor) ExecuteJob(ctx context.Context, envelope JobEnvelope, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if executor.GrantStore != nil {
		if errorValue := executor.GrantStore.Authorize(ctx, envelope, request, executor.ApprovalHandler); errorValue != nil {
			return capabilities.ToolInvokeResponse{}, errorValue
		}
	}
	handler, hasHandler := executorToolHandlers[request.ToolName]
	if !hasHandler {
		return capabilities.ToolInvokeResponse{}, fmt.Errorf("companion capability is not configured: %s", request.ToolName)
	}
	return handler(executor, ctx, envelope, request)
}

func (executor Executor) executeTextLLM(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if executor.DevMockLLM {
		return toolResponse(request.ToolName, map[string]any{
			"provider":        "companion",
			"model":           "mock-local",
			"selectedBackend": capabilities.LLMBackendCompanionLocal,
			"content":         "ok",
		})
	}
	if executor.LLMChain == nil {
		return capabilities.ToolInvokeResponse{}, errors.New("companion LLM is not configured")
	}
	var textRequest llmbackend.TextRequest
	if errorValue := decodeInput(request.Input, &textRequest); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	response, errorValue := executor.LLMChain.CompleteText(ctx, textRequest)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	response.SelectedBackend = capabilities.LLMBackendCompanionLocal
	return toolResponse(request.ToolName, response)
}

func (executor Executor) executeStructuredLLM(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if executor.DevMockLLM {
		return toolResponse(request.ToolName, map[string]any{
			"provider":        "companion",
			"model":           "mock-local",
			"selectedBackend": capabilities.LLMBackendCompanionLocal,
			"constraintMode":  llmbackend.ConstraintModeOpenAIJSONSchema,
			"content":         MockStructuredContent(request.Input),
		})
	}
	if executor.LLMChain == nil {
		return capabilities.ToolInvokeResponse{}, errors.New("companion LLM is not configured")
	}
	var structuredRequest llmbackend.StructuredRequest
	if errorValue := decodeInput(request.Input, &structuredRequest); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	response, errorValue := executor.LLMChain.CompleteStructured(ctx, structuredRequest)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	response.SelectedBackend = capabilities.LLMBackendCompanionLocal
	return toolResponse(request.ToolName, response)
}

func (executor Executor) executeEmbedding(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if executor.DevMockLLM {
		return toolResponse(request.ToolName, map[string]any{
			"provider":        "companion",
			"model":           llmbackend.DefaultEmbeddingModelName,
			"selectedBackend": capabilities.LLMBackendCompanionLocal,
			"embedding":       []float64{1, 0, 0},
		})
	}
	if executor.EmbeddingChain == nil {
		return capabilities.ToolInvokeResponse{}, errors.New("companion embedding is not configured")
	}
	var embeddingRequest llmbackend.EmbeddingRequest
	if errorValue := decodeInput(request.Input, &embeddingRequest); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	response, errorValue := executor.EmbeddingChain.CreateEmbedding(ctx, embeddingRequest)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	response.SelectedBackend = capabilities.LLMBackendCompanionLocal
	return toolResponse(request.ToolName, response)
}

func (executor Executor) executeAttentionTriage(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	var triageRequest capabilities.AttentionTriageRequest
	if errorValue := decodeInput(request.Input, &triageRequest); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if executor.DevMockLLM {
		return toolResponse(request.ToolName, mockAttentionTriageDecision(triageRequest))
	}
	if executor.LLMChain == nil {
		return capabilities.ToolInvokeResponse{}, errors.New("companion LLM is not configured")
	}
	response, errorValue := executor.LLMChain.CompleteStructured(ctx, attentionTriageStructuredRequest(request, triageRequest))
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	decision, errorValue := parseAttentionTriageDecision(response.Content, triageRequest.PrivacyClass)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, decision)
}

func attentionTriageStructuredRequest(request capabilities.ToolInvokeRequest, triageRequest capabilities.AttentionTriageRequest) llmbackend.StructuredRequest {
	return llmbackend.StructuredRequest{
		ExecutionMode: capabilities.ExecutionModeCompanion,
		Context: llmbackend.RequestContext{
			RequesterPersonID:       request.Context.RequesterPersonID,
			RequesterEmail:          request.Context.RequesterEmail,
			RequesterName:           request.Context.RequesterName,
			RequesterPlatformUserID: request.Context.RequesterPlatformUserID,
			ConversationID:          request.Context.ConversationID,
			Platform:                request.Context.Platform,
		},
		Messages: []llmbackend.Message{
			{Role: "system", Content: "Decide whether a pending user-local Companion job deserves remote attention. Return JSON only. Escalate only when silence would likely leave the user blocked or confused."},
			{Role: "user", Content: attentionTriagePrompt(triageRequest)},
		},
		StructuredOutputSchema: llmbackend.StructuredOutputSchema{
			Name:               "companion_attention_triage",
			Document:           json.RawMessage(attentionTriageSchema),
			IsStrictlyEnforced: true,
		},
		RequireParameters:     true,
		EnableResponseHealing: true,
	}
}

func attentionTriagePrompt(request capabilities.AttentionTriageRequest) string {
	document, errorValue := json.Marshal(request)
	if errorValue != nil {
		return "{}"
	}
	return string(document)
}

func parseAttentionTriageDecision(content string, fallbackPrivacyClass string) (capabilities.AttentionTriageDecision, error) {
	var decision capabilities.AttentionTriageDecision
	if errorValue := json.Unmarshal([]byte(strings.TrimSpace(content)), &decision); errorValue != nil {
		return capabilities.AttentionTriageDecision{}, errorValue
	}
	decision.Importance = normalizeAttentionImportance(decision.Importance)
	decision.Confidence = clampAttentionConfidence(decision.Confidence)
	decision.PrivacyClass = firstNonEmpty(decision.PrivacyClass, fallbackPrivacyClass)
	decision.SummaryForRemote = sanitizeAttentionSummary(decision.SummaryForRemote)
	decision.ReasonCodes = normalizeAttentionReasonCodes(decision.ReasonCodes)
	return decision, nil
}

func mockAttentionTriageDecision(request capabilities.AttentionTriageRequest) capabilities.AttentionTriageDecision {
	return capabilities.AttentionTriageDecision{
		ShouldEscalate:   request.Status == "failed" || request.Status == "denied" || request.Status == "expired",
		Importance:       "medium",
		Confidence:       0.8,
		ReasonCodes:      []string{"mock_triage"},
		SummaryForRemote: "Companion job " + request.JobID + " is " + request.Status + ".",
		PrivacyClass:     request.PrivacyClass,
	}
}

func normalizeAttentionImportance(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "low", "medium", "high":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "low"
	}
}

func clampAttentionConfidence(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func sanitizeAttentionSummary(value string) string {
	trimmedValue := strings.TrimSpace(value)
	if len(trimmedValue) > 600 {
		return trimmedValue[:600]
	}
	return trimmedValue
}

func normalizeAttentionReasonCodes(values []string) []string {
	reasonCodes := []string{}
	for _, value := range values {
		normalizedValue := strings.ToLower(strings.TrimSpace(value))
		if normalizedValue != "" {
			reasonCodes = append(reasonCodes, normalizedValue)
		}
		if len(reasonCodes) == 6 {
			return reasonCodes
		}
	}
	return reasonCodes
}

const attentionTriageSchema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["shouldEscalate", "importance", "confidence", "reasonCodes", "summaryForRemote", "privacyClass"],
  "properties": {
    "shouldEscalate": {"type": "boolean"},
    "importance": {"type": "string", "enum": ["low", "medium", "high"]},
    "confidence": {"type": "number", "minimum": 0, "maximum": 1},
    "reasonCodes": {"type": "array", "items": {"type": "string"}, "maxItems": 6},
    "summaryForRemote": {"type": "string", "maxLength": 600},
    "privacyClass": {"type": "string"}
  }
}`

func (executor Executor) executeBrowserSessionStart(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	var input browserruntime.SessionStartRequest
	if errorValue := decodeInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if executor.BrowserRuntime == nil {
		return capabilities.ToolInvokeResponse{}, errors.New("companion browser runtime unavailable")
	}
	result, errorValue := executor.BrowserRuntime.StartSession(ctx, input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, result)
}

func (executor Executor) executeBrowserNavigate(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	var input browserruntime.NavigateRequest
	if errorValue := decodeInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if executor.BrowserRuntime == nil {
		return capabilities.ToolInvokeResponse{}, errors.New("companion browser runtime unavailable")
	}
	result, errorValue := executor.BrowserRuntime.Navigate(ctx, input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, result)
}

func (executor Executor) executeBrowserObserve(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if executor.BrowserRuntime == nil {
		return capabilities.ToolInvokeResponse{}, errors.New("companion browser runtime unavailable")
	}
	var input browserruntime.ObserveRequest
	if errorValue := decodeInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	result, errorValue := executor.BrowserRuntime.Observe(ctx, input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, result)
}

func (executor Executor) executeBrowserScreenshot(ctx context.Context, envelope JobEnvelope, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if executor.BrowserRuntime == nil {
		return capabilities.ToolInvokeResponse{}, errors.New("companion browser runtime unavailable")
	}
	if executor.FileUploader == nil {
		return capabilities.ToolInvokeResponse{}, errors.New("browser screenshot upload requires device broker")
	}
	var input struct {
		TTLSeconds int `json:"ttlSeconds,omitempty"`
	}
	if errorValue := decodeInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	screenshot, errorValue := executor.BrowserRuntime.Screenshot(ctx, browserruntime.ScreenshotRequest{})
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	uploadedFile, errorValue := executor.FileUploader.UploadFile(ctx, FileUploadRequest{
		JobID:       envelope.JobID,
		Path:        screenshot.LocalPath,
		Filename:    screenshot.Filename,
		SizeBytes:   screenshot.SizeBytes,
		ContentType: screenshot.ContentType,
		TTLSeconds:  input.TTLSeconds,
	})
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, map[string]any{
		"fileID":      uploadedFile.FileID,
		"filename":    uploadedFile.Filename,
		"sizeBytes":   uploadedFile.SizeBytes,
		"contentType": uploadedFile.ContentType,
		"devicePath":  uploadedFile.DevicePath,
		"expiresAt":   uploadedFile.ExpiresAt,
		"capturedAt":  screenshot.CapturedAt,
	})
}

func (executor Executor) executeBrowserHandoff(ctx context.Context, envelope JobEnvelope, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if executor.BrowserRuntime == nil {
		return capabilities.ToolInvokeResponse{}, errors.New("companion browser runtime unavailable")
	}
	if isUnsupportedWaylandSession() {
		return capabilities.ToolInvokeResponse{}, errors.New("browser handoff native overlay is not supported on Linux Wayland")
	}
	if executor.HandoffStore == nil {
		return capabilities.ToolInvokeResponse{}, errors.New("browser handoff bridge is unavailable")
	}
	var input BrowserHandoffRequest
	if errorValue := decodeInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if strings.TrimSpace(input.URL) == "" {
		return capabilities.ToolInvokeResponse{}, errors.New("browser handoff url is required")
	}
	if !executor.HandoffStore.Snapshot().Active {
		if response, ok := executor.probeBrowserHandoffAutomation(ctx, request.ToolName, input); ok {
			return response, nil
		}
	}
	handoff, reused, errorValue := executor.HandoffStore.BeginOrReuse(input, firstNonEmpty(input.SessionID, "internkim"))
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if !reused {
		if errorValue := executor.pauseBrowserRuntimeForHandoff(ctx); errorValue != nil {
			executor.HandoffStore.End(handoff.HandoffID, HandoffStateDenied)
			return capabilities.ToolInvokeResponse{}, errorValue
		}
	}
	return executor.waitForBrowserHandoff(ctx, request.ToolName, input, handoff)
}

// probeBrowserHandoffAutomation navigates the already-open, already-driven
// browser session directly to the target URL: if that session's profile
// turns out to already be signed in (no Google login wall), the handoff is
// skipped entirely instead of pausing for a human who has nothing to do.
func (executor Executor) probeBrowserHandoffAutomation(ctx context.Context, toolName string, input BrowserHandoffRequest) (capabilities.ToolInvokeResponse, bool) {
	if !shouldProbeBrowserHandoffAutomation(input.URL) {
		return capabilities.ToolInvokeResponse{}, false
	}
	result, errorValue := executor.BrowserRuntime.Navigate(ctx, browserruntime.NavigateRequest{URL: input.URL})
	if errorValue != nil || browserHandoffNeedsNativeLogin(result.URL, result.Title, result.SnapshotText) {
		return capabilities.ToolInvokeResponse{}, false
	}
	sessionID := firstNonEmpty(input.SessionID, "internkim")
	response, errorValue := browserHandoffAutomationResponse(toolName, input.URL, sessionID, result)
	return response, errorValue == nil
}

func (executor Executor) pauseBrowserRuntimeForHandoff(ctx context.Context) error {
	pauser, ok := executor.BrowserRuntime.(browserHandoffPauser)
	if !ok {
		return errors.New("companion browser runtime does not support a human handoff")
	}
	return pauser.Pause(ctx)
}

func (executor Executor) resumeBrowserRuntimeAfterHandoff(ctx context.Context) {
	pauser, ok := executor.BrowserRuntime.(browserHandoffPauser)
	if !ok {
		return
	}
	_ = pauser.Resume(ctx)
}

func isUnsupportedWaylandSession() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("XDG_SESSION_TYPE")), "wayland") {
		return true
	}
	return strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) != "" && strings.TrimSpace(os.Getenv("DISPLAY")) == ""
}

func (executor Executor) waitForBrowserHandoff(ctx context.Context, toolName string, input BrowserHandoffRequest, handoff HandoffSnapshot) (capabilities.ToolInvokeResponse, error) {
	waitContext, cancel := handoffTimeoutContext(ctx, input.TimeoutSeconds)
	defer cancel()
	completion, errorValue := executor.HandoffStore.Wait(waitContext, handoff.HandoffID)
	if errorValue != nil {
		executor.HandoffStore.End(handoff.HandoffID, HandoffStateTimedOut)
		executor.resumeBrowserRuntimeAfterHandoff(ctx)
		return capabilities.ToolInvokeResponse{}, errors.New("browser handoff timed out")
	}
	completion = executor.resumeBrowserAutomationAfterHandoff(ctx, completion)
	return browserHandoffCompletedResponse(toolName, handoff, completion)
}

// resumeBrowserAutomationAfterHandoff resumes OS input synthesis on the same
// runtime the human just used, then re-observes the page so the completion
// result reflects whatever state the human left the browser in.
func (executor Executor) resumeBrowserAutomationAfterHandoff(ctx context.Context, completion HandoffCompletion) HandoffCompletion {
	executor.resumeBrowserRuntimeAfterHandoff(ctx)
	if executor.BrowserRuntime == nil {
		return completion
	}
	observation, errorValue := executor.BrowserRuntime.Observe(ctx, browserruntime.ObserveRequest{})
	if errorValue != nil {
		return completion
	}
	completion.URL = firstNonEmpty(observation.URL, completion.URL)
	completion.Title = firstNonEmpty(observation.Title, completion.Title)
	completion.SnapshotText = firstNonEmpty(observation.SnapshotText, completion.SnapshotText)
	completion.InteractiveRefs = firstNonEmptyStringSlice(observation.InteractiveRefs, completion.InteractiveRefs)
	completion.CapturedAt = firstNonEmpty(observation.CapturedAt, completion.CapturedAt)
	return completion
}

func browserHandoffAutomationResponse(toolName string, pageURL string, sessionID string, result browserruntime.NavigateResult) (capabilities.ToolInvokeResponse, error) {
	response, errorValue := toolResponse(toolName, BrowserHandoffResult{
		SessionID:       sessionID,
		URL:             firstNonEmpty(result.URL, pageURL),
		Origin:          webOriginOrEmpty(firstNonEmpty(result.URL, pageURL)),
		Title:           result.Title,
		SnapshotText:    result.SnapshotText,
		InteractiveRefs: result.InteractiveRefs,
		State:           HandoffStateCompleted,
		CompletedByUser: false,
		CapturedAt:      firstNonEmpty(result.CapturedAt, time.Now().UTC().Format(time.RFC3339)),
	})
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	response.Status = HandoffStateCompleted
	response.Content = "이미 로그인된 브라우저 세션을 사용합니다."
	return response, nil
}

func browserHandoffCompletedResponse(toolName string, handoff HandoffSnapshot, completion HandoffCompletion) (capabilities.ToolInvokeResponse, error) {
	response, errorValue := toolResponse(toolName, BrowserHandoffResult{
		HandoffID:       handoff.HandoffID,
		SessionID:       firstNonEmpty(completion.SessionID, handoff.SessionID),
		URL:             completion.URL,
		Origin:          handoff.Origin,
		Title:           completion.Title,
		SnapshotText:    completion.SnapshotText,
		InteractiveRefs: completion.InteractiveRefs,
		State:           HandoffStateCompleted,
		CompletedByUser: true,
		CapturedAt:      firstNonEmpty(completion.CapturedAt, time.Now().UTC().Format(time.RFC3339)),
	})
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	response.Status = HandoffStateCompleted
	response.Content = "브라우저 handoff가 완료되었습니다."
	return response, nil
}

func browserHandoffWaitingResponse(toolName string, pageURL string, handoff HandoffSnapshot) (capabilities.ToolInvokeResponse, error) {
	response, errorValue := toolResponse(toolName, BrowserHandoffResult{
		HandoffID:       handoff.HandoffID,
		SessionID:       handoff.SessionID,
		URL:             pageURL,
		Origin:          handoff.Origin,
		State:           HandoffStateWaitingForUser,
		CompletedByUser: false,
		CapturedAt:      time.Now().UTC().Format(time.RFC3339),
	})
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	response.Status = HandoffStateWaitingForUser
	response.Content = "브라우저에서 필요한 작업을 마친 뒤 완료 버튼을 눌러주세요."
	return response, nil
}

func shouldProbeBrowserHandoffAutomation(pageURL string) bool {
	origin, errorValue := url.Parse(strings.TrimSpace(pageURL))
	if errorValue != nil || origin.Hostname() == "" {
		return false
	}
	return isGoogleServiceHost(origin.Hostname()) && !isGoogleAuthenticationURL(origin)
}

func browserHandoffNeedsNativeLogin(pageURL string, title string, snapshotText string) bool {
	origin, errorValue := url.Parse(strings.TrimSpace(pageURL))
	if errorValue == nil && isGoogleAuthenticationURL(origin) {
		return true
	}
	text := strings.ToLower(title + "\n" + snapshotText)
	for _, fragment := range []string{
		"couldn't sign you in",
		"couldn’t sign you in",
		"this browser or app may not be secure",
		"choose an account",
		"use another account",
		"remove an account",
		"sign in with google",
	} {
		if strings.Contains(text, fragment) {
			return true
		}
	}
	return false
}

func isGoogleAuthenticationURL(origin *url.URL) bool {
	if origin == nil {
		return false
	}
	host := strings.ToLower(origin.Hostname())
	path := strings.ToLower(origin.EscapedPath())
	if host == "accounts.google.com" {
		return true
	}
	return strings.Contains(path, "/signin/") ||
		strings.Contains(path, "/challenge/") ||
		strings.Contains(path, "/rejected")
}

func isGoogleServiceHost(host string) bool {
	normalizedHost := strings.ToLower(strings.TrimSpace(host))
	return normalizedHost == "google.com" ||
		strings.HasSuffix(normalizedHost, ".google.com") ||
		normalizedHost == "youtube.com" ||
		strings.HasSuffix(normalizedHost, ".youtube.com")
}

func (executor Executor) executeBrowserClick(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if executor.BrowserRuntime == nil {
		return capabilities.ToolInvokeResponse{}, errors.New("companion browser runtime unavailable")
	}
	var input browserruntime.ClickRequest
	if errorValue := decodeInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	result, errorValue := executor.BrowserRuntime.Click(ctx, input)
	if errorValue != nil {
		return executor.browserActionFailureResponse(ctx, request, errorValue)
	}
	return toolResponse(request.ToolName, result)
}

func (executor Executor) executeBrowserFill(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if executor.BrowserRuntime == nil {
		return capabilities.ToolInvokeResponse{}, errors.New("companion browser runtime unavailable")
	}
	var input browserruntime.FillRequest
	if errorValue := decodeInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	result, errorValue := executor.BrowserRuntime.Fill(ctx, input)
	if errorValue != nil {
		return executor.browserActionFailureResponse(ctx, request, errorValue)
	}
	return toolResponse(request.ToolName, result)
}

func (executor Executor) executeBrowserSelect(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if executor.BrowserRuntime == nil {
		return capabilities.ToolInvokeResponse{}, errors.New("companion browser runtime unavailable")
	}
	var input browserruntime.SelectRequest
	if errorValue := decodeInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	result, errorValue := executor.BrowserRuntime.Select(ctx, input)
	if errorValue != nil {
		return executor.browserActionFailureResponse(ctx, request, errorValue)
	}
	return toolResponse(request.ToolName, result)
}

func (executor Executor) executeBrowserPress(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if executor.BrowserRuntime == nil {
		return capabilities.ToolInvokeResponse{}, errors.New("companion browser runtime unavailable")
	}
	var input browserruntime.PressRequest
	if errorValue := decodeInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	result, errorValue := executor.BrowserRuntime.Press(ctx, input)
	if errorValue != nil {
		return executor.browserActionFailureResponse(ctx, request, errorValue)
	}
	return toolResponse(request.ToolName, result)
}

func (executor Executor) executeBrowserWait(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if executor.BrowserRuntime == nil {
		return capabilities.ToolInvokeResponse{}, errors.New("companion browser runtime unavailable")
	}
	var input browserruntime.WaitRequest
	if errorValue := decodeInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	result, errorValue := executor.BrowserRuntime.Wait(ctx, input)
	if errorValue != nil {
		return executor.browserActionFailureResponse(ctx, request, errorValue)
	}
	return toolResponse(request.ToolName, result)
}

func (executor Executor) browserActionFailureResponse(ctx context.Context, request capabilities.ToolInvokeRequest, failure error) (capabilities.ToolInvokeResponse, error) {
	result := BrowserActionFailureResult{
		Status:   "recoverable_error",
		ToolName: request.ToolName,
		Error:    firstNonEmpty(errorString(failure), "browser action failed"),
		Guidance: "현재 스냅샷을 기준으로 다음 클릭/입력 대상을 다시 선택하거나, 화면에서 사용자가 해야 할 일이 보이면 사용자에게 안내하세요. 같은 실패 동작을 그대로 반복하지 마세요.",
	}
	observation, observeError := executor.BrowserRuntime.Observe(ctx, browserruntime.ObserveRequest{})
	if observeError != nil {
		result.SnapshotError = firstNonEmpty(errorString(observeError), "browser snapshot failed")
	} else {
		result.URL = observation.URL
		result.Title = observation.Title
		result.SnapshotText = observation.SnapshotText
		result.InteractiveRefs = observation.InteractiveRefs
		result.CapturedAt = observation.CapturedAt
	}
	response, errorValue := toolResponse(request.ToolName, result)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	response.Status = "error"
	response.IsError = true
	response.Content = string(response.Result)
	return response, nil
}

func (executor Executor) executeUserConfirm(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if executor.PromptHandler == nil {
		return capabilities.ToolInvokeResponse{}, errors.New("user confirmation requires companion UI or --allow-stdin-prompts")
	}
	var input struct {
		Message string `json:"message"`
		Default bool   `json:"default"`
	}
	if errorValue := decodeInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	confirmed, errorValue := executor.PromptHandler.Confirm(ctx, firstNonEmpty(input.Message, "Continue?"), input.Default)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, map[string]bool{"confirmed": confirmed})
}

func (executor Executor) executeUserInput(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if executor.PromptHandler == nil {
		return capabilities.ToolInvokeResponse{}, errors.New("user input requires companion UI or --allow-stdin-prompts")
	}
	var input struct {
		Message string `json:"message"`
		Prompt  string `json:"prompt"`
	}
	if errorValue := decodeInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	text, errorValue := executor.PromptHandler.Input(ctx, firstNonEmpty(input.Message, input.Prompt, "Input"))
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, map[string]string{"text": text})
}

func (executor Executor) executeFilePick(ctx context.Context, envelope JobEnvelope, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if executor.FilePicker == nil {
		return capabilities.ToolInvokeResponse{}, errors.New("file picker requires companion UI")
	}
	if executor.FileUploader == nil {
		return capabilities.ToolInvokeResponse{}, errors.New("file upload requires device broker")
	}
	var input FilePickRequest
	if errorValue := decodeInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	pickedFile, errorValue := executor.FilePicker.PickFile(ctx, input)
	if errors.Is(errorValue, ErrFilePickCanceled) {
		return capabilities.ToolInvokeResponse{}, DenialError{Denial: capabilities.DenialResult{
			Status:              "denied",
			Code:                "user_cancelled",
			JobID:               envelope.JobID,
			ToolName:            request.ToolName,
			ResourceScope:       firstResourceScope(envelope.ResourceScope, request.ResourceScope),
			SuggestedConstraint: "file selection was cancelled",
		}}
	}
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	fileUploadRequest, errorValue := fileUploadRequestFromPickedFile(envelope, input, pickedFile)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	uploadedFile, errorValue := executor.FileUploader.UploadFile(ctx, fileUploadRequest)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, uploadedFile)
}

func (executor Executor) executeMountCreate(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	mountStore, errorValue := executor.requireMountStore()
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	var input struct {
		Path        string `json:"path"`
		DisplayName string `json:"displayName"`
		Title       string `json:"title"`
	}
	if errorValue := decodeInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	localPath := strings.TrimSpace(input.Path)
	if localPath == "" {
		if executor.DirectoryPicker == nil {
			return capabilities.ToolInvokeResponse{}, errors.New("directory picker requires companion UI")
		}
		pickedDirectory, errorValue := executor.DirectoryPicker.PickDirectory(ctx, DirectoryPickRequest{Title: firstNonEmpty(input.Title, "Choose a folder for the connected agent")})
		if errorValue != nil {
			return capabilities.ToolInvokeResponse{}, errorValue
		}
		localPath = pickedDirectory.Path
	}
	mount, errorValue := mountStore.Create(localPath, input.DisplayName)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, mount)
}

func (executor Executor) executeMountList(request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	mountStore, errorValue := executor.requireMountStore()
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, map[string]any{"mounts": mountStore.List()})
}

func (executor Executor) executeMountRevoke(request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	mountStore, input, errorValue := executor.mountInput(request)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	mount, errorValue := mountStore.Revoke(input.MountID)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, mount)
}

func (executor Executor) executeMountPause(request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	mountStore, input, errorValue := executor.mountInput(request)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	mount, errorValue := mountStore.Pause(input.MountID)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, mount)
}

func (executor Executor) executeMountResume(request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	mountStore, input, errorValue := executor.mountInput(request)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	mount, errorValue := mountStore.Resume(input.MountID)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, mount)
}

func (executor Executor) executeMountStatus(request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	mountStore, input, errorValue := executor.mountInput(request)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if strings.TrimSpace(input.MountID) == "" {
		return toolResponse(request.ToolName, map[string]any{"mounts": mountStore.List()})
	}
	mount, errorValue := mountStore.Status(input.MountID)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, mount)
}

func (executor Executor) executeMountStat(request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	mountStore, input, errorValue := executor.mountInput(request)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	information, errorValue := mountStore.Stat(input.MountID, input.Path)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, information)
}

func (executor Executor) executeMountListDirectory(request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	mountStore, input, errorValue := executor.mountInput(request)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	entries, errorValue := mountStore.ListDirectory(input.MountID, input.Path)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, map[string]any{"entries": entries})
}

func (executor Executor) executeMountRead(request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	mountStore, input, errorValue := executor.mountInput(request)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	result, errorValue := mountStore.ReadFile(input.MountID, input.Path)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, result)
}

func (executor Executor) executeMountWrite(request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	mountStore, input, errorValue := executor.mountInput(request)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	document, errorValue := mountWriteDocument(input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	information, errorValue := mountStore.WriteFile(input.MountID, input.Path, document)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, information)
}

func (executor Executor) executeMountMakeDirectory(request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	mountStore, input, errorValue := executor.mountInput(request)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	information, errorValue := mountStore.MakeDirectory(input.MountID, input.Path)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, information)
}

func (executor Executor) executeMountRename(request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	mountStore, input, errorValue := executor.mountInput(request)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	information, errorValue := mountStore.Rename(input.MountID, input.Path, input.ToPath)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, information)
}

func (executor Executor) executeMountDelete(request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	mountStore, input, errorValue := executor.mountInput(request)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	result, errorValue := mountStore.Delete(input.MountID, input.Path, input.Recursive)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, result)
}

func (executor Executor) executeMountTruncate(request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	mountStore, input, errorValue := executor.mountInput(request)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	information, errorValue := mountStore.Truncate(input.MountID, input.Path, input.SizeBytes)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, information)
}

func (executor Executor) executeMountChangeMode(request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	mountStore, input, errorValue := executor.mountInput(request)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	information, errorValue := mountStore.ChangeMode(input.MountID, input.Path, os.FileMode(input.Mode))
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, information)
}

func (executor Executor) executeMountWatch(request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	mountStore, input, errorValue := executor.mountInput(request)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	entries, errorValue := mountStore.ChangedSince(input.MountID, input.Path, time.Unix(0, input.SinceUnixNano).UTC())
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, map[string]any{"entries": entries, "sinceUnixNano": input.SinceUnixNano})
}

type mountToolInput struct {
	MountID       string `json:"mountID"`
	Path          string `json:"path"`
	ToPath        string `json:"toPath"`
	Content       string `json:"content"`
	ContentBase64 string `json:"contentBase64"`
	Recursive     bool   `json:"recursive"`
	SizeBytes     int64  `json:"sizeBytes"`
	Mode          uint32 `json:"mode"`
	SinceUnixNano int64  `json:"sinceUnixNano"`
}

func (executor Executor) mountInput(request capabilities.ToolInvokeRequest) (*MountStore, mountToolInput, error) {
	mountStore, errorValue := executor.requireMountStore()
	if errorValue != nil {
		return nil, mountToolInput{}, errorValue
	}
	var input mountToolInput
	if errorValue := decodeInput(request.Input, &input); errorValue != nil {
		return nil, mountToolInput{}, errorValue
	}
	if strings.TrimSpace(input.MountID) == "" && request.ResourceScope.Kind == MountResourceScopeKind {
		input.MountID = request.ResourceScope.Value
	}
	if strings.TrimSpace(input.MountID) == "" && request.ToolName != "filesystem.mount.status" {
		return nil, mountToolInput{}, errors.New("mountID is required")
	}
	return mountStore, input, nil
}

func (executor Executor) requireMountStore() (*MountStore, error) {
	if executor.MountStore == nil {
		return nil, errors.New("filesystem mount store is unavailable")
	}
	return executor.MountStore, nil
}

func mountWriteDocument(input mountToolInput) ([]byte, error) {
	if strings.TrimSpace(input.ContentBase64) != "" {
		return base64.StdEncoding.DecodeString(strings.TrimSpace(input.ContentBase64))
	}
	return []byte(input.Content), nil
}

func fileUploadRequestFromPickedFile(envelope JobEnvelope, request FilePickRequest, pickedFile PickedFile) (FileUploadRequest, error) {
	trimmedPath := strings.TrimSpace(pickedFile.Path)
	if trimmedPath == "" {
		return FileUploadRequest{}, errors.New("selected file path is missing")
	}
	information, errorValue := os.Stat(trimmedPath)
	if errorValue != nil || information.IsDir() {
		return FileUploadRequest{}, errors.New("selected file cannot be read")
	}
	if request.MaxBytes > 0 && information.Size() > request.MaxBytes {
		return FileUploadRequest{}, errors.New("selected file is larger than the requested limit")
	}
	filename := firstNonEmpty(pickedFile.Filename, filepath.Base(trimmedPath))
	if !extensionAllowed(filename, request.AllowedExtensions) {
		return FileUploadRequest{}, errors.New("selected file extension is not allowed")
	}
	contentType := firstNonEmpty(pickedFile.ContentType, mime.TypeByExtension(strings.ToLower(filepath.Ext(filename))), "application/octet-stream")
	return FileUploadRequest{
		JobID:       envelope.JobID,
		Path:        trimmedPath,
		Filename:    filename,
		SizeBytes:   information.Size(),
		ContentType: contentType,
		TTLSeconds:  request.TTLSeconds,
	}, nil
}

func extensionAllowed(filename string, allowedExtensions []string) bool {
	if len(allowedExtensions) == 0 {
		return true
	}
	extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	for _, allowedExtension := range allowedExtensions {
		normalizedExtension := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(allowedExtension)), ".")
		if normalizedExtension != "" && normalizedExtension == extension {
			return true
		}
	}
	return false
}

func (handler TerminalPromptHandler) Confirm(ctx context.Context, message string, defaultValue bool) (bool, error) {
	_ = ctx
	if handler.Reader == nil {
		return false, errors.New("terminal prompt reader is not configured")
	}
	writer := handler.Writer
	if writer == nil {
		writer = io.Discard
	}
	reader := bufio.NewReader(handler.Reader)
	_, _ = fmt.Fprintf(writer, "%s ", message)
	if defaultValue {
		_, _ = fmt.Fprint(writer, "[Y/n] ")
	} else {
		_, _ = fmt.Fprint(writer, "[y/N] ")
	}
	line, errorValue := reader.ReadString('\n')
	if errorValue != nil && !errors.Is(errorValue, io.EOF) {
		return false, errorValue
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer == "" {
		return defaultValue, nil
	}
	return answer == "y" || answer == "yes", nil
}

func (handler TerminalPromptHandler) Input(ctx context.Context, message string) (string, error) {
	_ = ctx
	if handler.Reader == nil {
		return "", errors.New("terminal prompt reader is not configured")
	}
	writer := handler.Writer
	if writer == nil {
		writer = io.Discard
	}
	reader := bufio.NewReader(handler.Reader)
	_, _ = fmt.Fprintf(writer, "%s ", message)
	line, errorValue := reader.ReadString('\n')
	if errorValue != nil && !errors.Is(errorValue, io.EOF) {
		return "", errorValue
	}
	return strings.TrimSpace(line), nil
}

func MockStructuredContent(document []byte) string {
	var request struct {
		StructuredOutputSchema struct {
			Document struct {
				Required []string `json:"required"`
			} `json:"document"`
		} `json:"structuredOutputSchema"`
	}
	if errorValue := json.NewDecoder(bytes.NewReader(document)).Decode(&request); errorValue != nil {
		return `{"content":"ok"}`
	}
	values := map[string]string{}
	for _, key := range request.StructuredOutputSchema.Document.Required {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey != "" {
			values[trimmedKey] = "ok"
		}
	}
	if len(values) == 0 {
		values["content"] = "ok"
	}
	response, errorValue := json.Marshal(values)
	if errorValue != nil {
		return `{"content":"ok"}`
	}
	return string(response)
}

func toolResponse(toolName string, result any) (capabilities.ToolInvokeResponse, error) {
	document, errorValue := json.Marshal(result)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilities.ToolInvokeResponse{
		Provider:        "companion",
		SelectedBackend: capabilities.LLMBackendCompanionLocal,
		ToolName:        toolName,
		Result:          document,
	}, nil
}

func decodeInput(document []byte, output any) error {
	if len(bytes.TrimSpace(document)) == 0 {
		return nil
	}
	return json.Unmarshal(document, output)
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

func firstNonEmptyStringSlice(values ...[]string) []string {
	for _, value := range values {
		if len(value) != 0 {
			return value
		}
	}
	return nil
}

func errorString(errorValue error) string {
	if errorValue == nil {
		return ""
	}
	return strings.TrimSpace(errorValue.Error())
}
