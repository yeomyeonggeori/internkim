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
	"os"
	"path/filepath"
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

func (executor Executor) Execute(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	return executor.ExecuteJob(ctx, JobEnvelope{ToolName: request.ToolName, ResourceScope: request.ResourceScope}, request)
}

func (executor Executor) ExecuteJob(ctx context.Context, envelope JobEnvelope, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if executor.GrantStore != nil {
		if errorValue := executor.GrantStore.Authorize(ctx, envelope, request, executor.ApprovalHandler); errorValue != nil {
			return capabilities.ToolInvokeResponse{}, errorValue
		}
	}
	switch request.ToolName {
	case "llm.text":
		return executor.executeTextLLM(ctx, request)
	case "llm.structured":
		return executor.executeStructuredLLM(ctx, request)
	case "embedding.create":
		return executor.executeEmbedding(ctx, request)
	case "browser.open":
		return executor.executeBrowserNavigate(ctx, request)
	case "browser.snapshot":
		return executor.executeBrowserObserve(ctx, request)
	case "browser.screenshot":
		return executor.executeBrowserScreenshot(ctx, envelope, request)
	case "browser.handoff":
		return executor.executeBrowserHandoff(ctx, envelope, request)
	case "browser.click":
		return executor.executeBrowserClick(ctx, request)
	case "browser.fill":
		return executor.executeBrowserFill(ctx, request)
	case "browser.select":
		return executor.executeBrowserSelect(ctx, request)
	case "browser.press":
		return executor.executeBrowserPress(ctx, request)
	case "browser.wait":
		return executor.executeBrowserWait(ctx, request)
	case "user.confirm":
		return executor.executeUserConfirm(ctx, request)
	case "user.input":
		return executor.executeUserInput(ctx, request)
	case "file.pick":
		return executor.executeFilePick(ctx, envelope, request)
	case "filesystem.mount.create":
		return executor.executeMountCreate(ctx, request)
	case "filesystem.mount.list":
		return executor.executeMountList(request)
	case "filesystem.mount.pause":
		return executor.executeMountPause(request)
	case "filesystem.mount.resume":
		return executor.executeMountResume(request)
	case "filesystem.mount.revoke":
		return executor.executeMountRevoke(request)
	case "filesystem.mount.status":
		return executor.executeMountStatus(request)
	case "filesystem.mount.stat":
		return executor.executeMountStat(request)
	case "filesystem.mount.list_directory":
		return executor.executeMountListDirectory(request)
	case "filesystem.mount.read":
		return executor.executeMountRead(request)
	case "filesystem.mount.write":
		return executor.executeMountWrite(request)
	case "filesystem.mount.mkdir":
		return executor.executeMountMakeDirectory(request)
	case "filesystem.mount.rename":
		return executor.executeMountRename(request)
	case "filesystem.mount.delete":
		return executor.executeMountDelete(request)
	case "filesystem.mount.truncate":
		return executor.executeMountTruncate(request)
	case "filesystem.mount.chmod":
		return executor.executeMountChangeMode(request)
	case "filesystem.mount.watch":
		return executor.executeMountWatch(request)
	default:
		return capabilities.ToolInvokeResponse{}, fmt.Errorf("companion capability is not configured: %s", request.ToolName)
	}
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
			"model":           llmbackend.DefaultEmbeddingGemmaModel,
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
	startResult, errorValue := executor.BrowserRuntime.StartSession(ctx, browserruntime.SessionStartRequest{URL: input.URL})
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	handoff, errorValue := executor.HandoffStore.Begin(input, startResult.SessionID)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	handoffContext, cancel := handoffTimeoutContext(ctx, input.TimeoutSeconds)
	defer cancel()
	for validationAttempt := 0; validationAttempt < maxHandoffValidationAttempts; validationAttempt++ {
		_, waitError := executor.HandoffStore.Wait(handoffContext, handoff.HandoffID)
		if waitError != nil {
			executor.HandoffStore.End(handoff.HandoffID, HandoffStateTimedOut)
			return capabilities.ToolInvokeResponse{}, handoffDenial(envelope, request, "handoff_timeout", "browser handoff timed out")
		}
		_ = executor.HandoffStore.UpdateMessage(handoff.HandoffID, HandoffStateValidating, "확인 중입니다.")
		observation, observeError := executor.BrowserRuntime.Observe(ctx, browserruntime.ObserveRequest{})
		if observeError != nil {
			executor.HandoffStore.End(handoff.HandoffID, HandoffStateDenied)
			return capabilities.ToolInvokeResponse{}, observeError
		}
		if HandoffCriteriaEmpty(input.SuccessCriteria) || HandoffCriteriaSatisfied(input.SuccessCriteria, observation.URL, observation.SnapshotText) {
			executor.HandoffStore.End(handoff.HandoffID, HandoffStateCompleted)
			return toolResponse(request.ToolName, BrowserHandoffResult{
				SessionID:       handoff.SessionID,
				URL:             observation.URL,
				Origin:          firstNonEmpty(handoff.Origin, webOriginOrEmpty(observation.URL)),
				Title:           observation.Title,
				SnapshotText:    observation.SnapshotText,
				InteractiveRefs: observation.InteractiveRefs,
				State:           HandoffStateCompleted,
				CompletedByUser: true,
				CapturedAt:      observation.CapturedAt,
			})
		}
		_ = executor.HandoffStore.UpdateMessage(handoff.HandoffID, HandoffStateWaitingForUser, "아직 완료되지 않은 것 같아요. 브라우저에서 필요한 작업을 마친 뒤 완료를 눌러주세요.")
	}
	executor.HandoffStore.End(handoff.HandoffID, HandoffStateDenied)
	return capabilities.ToolInvokeResponse{}, handoffDenial(envelope, request, "validation_failed", "browser handoff validation failed")
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
		return capabilities.ToolInvokeResponse{}, errorValue
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
		return capabilities.ToolInvokeResponse{}, errorValue
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
		return capabilities.ToolInvokeResponse{}, errorValue
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
		return capabilities.ToolInvokeResponse{}, errorValue
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
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, result)
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
		pickedDirectory, errorValue := executor.DirectoryPicker.PickDirectory(ctx, DirectoryPickRequest{Title: firstNonEmpty(input.Title, "Choose a folder for Blueclaw")})
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
