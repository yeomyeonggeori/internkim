package companion

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"

	browserruntime "github.com/anthropic-lab/internkim/internal/browser"
	"github.com/anthropic-lab/internkim/internal/capabilities"
)

type PromptHandler interface {
	Confirm(ctx context.Context, message string, defaultValue bool) (bool, error)
	Input(ctx context.Context, message string) (string, error)
}

type FilePicker interface {
	PickFile(ctx context.Context, request FilePickRequest) (PickedFile, error)
}

type FileUploader interface {
	UploadFile(ctx context.Context, request FileUploadRequest) (UploadedFile, error)
}

type Executor struct {
	DevMockLLM      bool
	BrowserRuntime  browserruntime.Runtime
	PromptHandler   PromptHandler
	FilePicker      FilePicker
	FileUploader    FileUploader
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
		return executor.executeTextLLM(request)
	case "llm.structured":
		return executor.executeStructuredLLM(request)
	case "browser.session.start":
		return executor.executeBrowserSessionStart(ctx, request)
	case "browser.navigate":
		return executor.executeBrowserNavigate(ctx, request)
	case "browser.observe":
		return executor.executeBrowserObserve(ctx, request)
	case "browser.screenshot":
		return executor.executeBrowserScreenshot(ctx, envelope, request)
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
	default:
		return capabilities.ToolInvokeResponse{}, fmt.Errorf("companion capability is not configured: %s", request.ToolName)
	}
}

func (executor Executor) executeTextLLM(request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if !executor.DevMockLLM {
		return capabilities.ToolInvokeResponse{}, errors.New("companion LLM is not configured")
	}
	return toolResponse(request.ToolName, map[string]any{
		"provider":        "companion",
		"model":           "mock-local",
		"selectedBackend": capabilities.LLMBackendCompanionLocal,
		"content":         "ok",
	})
}

func (executor Executor) executeStructuredLLM(request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if !executor.DevMockLLM {
		return capabilities.ToolInvokeResponse{}, errors.New("companion LLM is not configured")
	}
	return toolResponse(request.ToolName, map[string]any{
		"provider":        "companion",
		"model":           "mock-local",
		"selectedBackend": capabilities.LLMBackendCompanionLocal,
		"constraintMode":  "prompt_validation",
		"content":         MockStructuredContent(request.Input),
	})
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
