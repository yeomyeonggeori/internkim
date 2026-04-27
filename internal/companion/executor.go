package companion

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"runtime"
	"strings"

	"github.com/anthropic-lab/internkim/internal/capabilities"
)

type BrowserOpener interface {
	OpenBrowser(ctx context.Context, targetURL string) error
}

type PromptHandler interface {
	Confirm(ctx context.Context, message string, defaultValue bool) (bool, error)
	Input(ctx context.Context, message string) (string, error)
}

type Executor struct {
	DevMockLLM      bool
	BrowserOpener   BrowserOpener
	PromptHandler   PromptHandler
	ApprovalHandler ApprovalHandler
	GrantStore      *MemoryGrantStore
}

type SystemBrowserOpener struct{}

type TerminalPromptHandler struct {
	Reader io.Reader
	Writer io.Writer
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
	switch request.ToolName {
	case "llm.text":
		return executor.executeTextLLM(request)
	case "llm.structured":
		return executor.executeStructuredLLM(request)
	case "browser.session.start":
		return executor.executeBrowserSessionStart(ctx, request)
	case "browser.navigate":
		return executor.executeBrowserNavigate(ctx, request)
	case "user.confirm":
		return executor.executeUserConfirm(ctx, request)
	case "user.input":
		return executor.executeUserInput(ctx, request)
	case "browser.observe", "browser.screenshot", "file.pick":
		return capabilities.ToolInvokeResponse{}, fmt.Errorf("companion capability is not implemented yet: %s", request.ToolName)
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
	var input struct {
		URL      string `json:"url"`
		StartURL string `json:"startURL"`
	}
	if errorValue := decodeInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	targetURL := firstNonEmpty(input.URL, input.StartURL)
	opened := false
	if targetURL != "" {
		if errorValue := executor.openBrowser(ctx, targetURL); errorValue != nil {
			return capabilities.ToolInvokeResponse{}, errorValue
		}
		opened = true
	}
	return toolResponse(request.ToolName, map[string]any{
		"sessionID": "default",
		"opened":    opened,
		"url":       targetURL,
	})
}

func (executor Executor) executeBrowserNavigate(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	var input struct {
		URL string `json:"url"`
	}
	if errorValue := decodeInput(request.Input, &input); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if errorValue := executor.openBrowser(ctx, input.URL); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return toolResponse(request.ToolName, map[string]any{
		"opened": true,
		"url":    input.URL,
	})
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

func (executor Executor) openBrowser(ctx context.Context, targetURL string) error {
	if executor.BrowserOpener == nil {
		return errors.New("browser opener is not configured")
	}
	if errorValue := validateBrowserURL(targetURL); errorValue != nil {
		return errorValue
	}
	return executor.BrowserOpener.OpenBrowser(ctx, targetURL)
}

func (SystemBrowserOpener) OpenBrowser(ctx context.Context, targetURL string) error {
	commandName, arguments := browserOpenCommand(targetURL)
	return exec.CommandContext(ctx, commandName, arguments...).Start()
}

func browserOpenCommand(targetURL string) (string, []string) {
	switch runtime.GOOS {
	case "darwin":
		return "open", []string{targetURL}
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", targetURL}
	default:
		return "xdg-open", []string{targetURL}
	}
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

func validateBrowserURL(value string) error {
	parsedURL, errorValue := url.Parse(strings.TrimSpace(value))
	if errorValue != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return errors.New("browser URL must be absolute")
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errors.New("browser URL must use http or https")
	}
	return nil
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
