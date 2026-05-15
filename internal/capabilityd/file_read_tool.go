package capabilityd

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

//go:embed file_read_helper.py
var fileReadHelperDocument string

const defaultFileReadMaximumOutputBytes = 200000
const hardFileReadMaximumOutputBytes = 1000000

type fileReadInput struct {
	Path           string `json:"path"`
	OCRMode        string `json:"ocrMode"`
	MaxPages       int    `json:"maxPages"`
	MaxOutputBytes int    `json:"maxOutputBytes"`
}

type fileReadHelperRequest struct {
	Path              string `json:"path"`
	OCRMode           string `json:"ocrMode"`
	MaxPages          int    `json:"maxPages,omitempty"`
	OpenRouterAPIKey  string `json:"openRouterAPIKey,omitempty"`
	OpenRouterBaseURL string `json:"openRouterBaseURL,omitempty"`
	OpenRouterModel   string `json:"openRouterModel,omitempty"`
}

type fileReadHelperResponse struct {
	Content  string   `json:"content"`
	Warnings []string `json:"warnings,omitempty"`
}

type fileReadResult struct {
	Status    string   `json:"status"`
	Path      string   `json:"path"`
	Format    string   `json:"format"`
	Content   string   `json:"content"`
	Backend   string   `json:"backend,omitempty"`
	Model     string   `json:"model,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
	Truncated bool     `json:"truncated,omitempty"`
}

func isFileReadTool(toolName string) bool {
	return strings.TrimSpace(toolName) == "file.read"
}

func (service Service) invokeFileReadTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeFileReadInput(request.Input)
	if errorValue != nil {
		return fileReadErrorResponse(request.ToolName, errorValue.Error(), "invalid_input", "input_validation", false), nil
	}
	configuration := service.Configuration.WithDefaults()
	hostPath, agentPath, errorValue := service.resolveFileReadPath(input.Path)
	if errorValue != nil {
		return fileReadErrorResponse(request.ToolName, errorValue.Error(), "invalid_workspace_path", "path_validation", false), nil
	}
	if input.OCRMode != "never" && configuration.LocalOnly {
		return fileReadErrorResponse(request.ToolName, "remote OCR is disabled in local-only mode", "local_only", "openrouter_configuration", false), nil
	}
	apiKey := readSecretValue(configuration.OpenRouterKeyPath)
	if input.OCRMode != "never" && (strings.TrimSpace(apiKey) == "" || isPlaceholderOpenRouterKey(apiKey)) {
		return fileReadErrorResponse(request.ToolName, "OpenRouter API key is not configured", "missing_openrouter_key", "openrouter_configuration", false), nil
	}
	helperResponse, errorValue := service.runFileReadHelper(ctx, fileReadHelperRequest{
		Path:              hostPath,
		OCRMode:           input.OCRMode,
		MaxPages:          input.MaxPages,
		OpenRouterAPIKey:  apiKey,
		OpenRouterBaseURL: openRouterClientBaseURL(configuration.OpenRouterBaseURL),
		OpenRouterModel:   configuration.OpenRouterModel,
	})
	if errorValue != nil {
		return fileReadErrorResponse(request.ToolName, errorValue.Error(), "file_read_failed", "markitdown_conversion", true), nil
	}
	content := strings.TrimSpace(helperResponse.Content)
	if content == "" {
		return fileReadErrorResponse(request.ToolName, "converted file content was empty", "file_read_empty", "markitdown_conversion", false), nil
	}
	content, isTruncated := truncateTextByBytes(content, input.MaxOutputBytes)
	result := fileReadResult{
		Status:    "ok",
		Path:      agentPath,
		Format:    "markdown",
		Content:   content,
		Backend:   "openrouter",
		Model:     configuration.OpenRouterModel,
		Warnings:  helperResponse.Warnings,
		Truncated: isTruncated,
	}
	resultDocument, errorValue := json.Marshal(result)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilities.ToolInvokeResponse{
		Provider:        "markitdown",
		SelectedBackend: capabilities.LLMBackendRemote,
		ToolName:        request.ToolName,
		Status:          "ok",
		Content:         content,
		Result:          resultDocument,
	}, nil
}

func decodeFileReadInput(document json.RawMessage) (fileReadInput, error) {
	var input fileReadInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return fileReadInput{}, errorValue
	}
	input.Path = strings.TrimSpace(input.Path)
	input.OCRMode = strings.ToLower(strings.TrimSpace(input.OCRMode))
	if input.OCRMode == "" {
		input.OCRMode = "auto"
	}
	if input.Path == "" {
		return fileReadInput{}, errors.New("path is required")
	}
	if input.OCRMode != "auto" && input.OCRMode != "always" && input.OCRMode != "never" {
		return fileReadInput{}, errors.New("ocrMode must be auto, always, or never")
	}
	if input.MaxPages < 0 || input.MaxPages > 500 {
		return fileReadInput{}, errors.New("maxPages must be between 0 and 500")
	}
	if input.MaxOutputBytes == 0 {
		input.MaxOutputBytes = defaultFileReadMaximumOutputBytes
	}
	if input.MaxOutputBytes < 1024 || input.MaxOutputBytes > hardFileReadMaximumOutputBytes {
		return fileReadInput{}, fmt.Errorf("maxOutputBytes must be between 1024 and %d", hardFileReadMaximumOutputBytes)
	}
	return input, nil
}

func (service Service) resolveFileReadPath(path string) (string, string, error) {
	agentPath, errorValue := cleanFileReadAgentPath(path)
	if errorValue != nil {
		return "", "", errorValue
	}
	workspacePath := service.Configuration.WithDefaults().BlueclawWorkspacePath
	hostPath := filepath.Join(workspacePath, strings.TrimPrefix(agentPath, "/workspace/"))
	if agentPath == "/workspace" {
		hostPath = workspacePath
	}
	hostPath, errorValue = cleanHostWorkspacePath(workspacePath, hostPath)
	if errorValue != nil {
		return "", "", errorValue
	}
	fileInformation, errorValue := os.Stat(hostPath)
	if errorValue != nil {
		return "", "", errorValue
	}
	if !fileInformation.Mode().IsRegular() {
		return "", "", errors.New("path must point to a regular file")
	}
	return hostPath, agentPath, nil
}

func cleanFileReadAgentPath(path string) (string, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return "", errors.New("path is required")
	}
	if !filepath.IsAbs(trimmedPath) {
		return "", errors.New("path must be an absolute /workspace path")
	}
	cleanPath := filepath.ToSlash(filepath.Clean(trimmedPath))
	if cleanPath != "/workspace" && !strings.HasPrefix(cleanPath, "/workspace/") {
		return "", errors.New("path must stay under /workspace")
	}
	if cleanPath == "/workspace" || cleanPath == "/workspace/.blueclaw" || strings.HasPrefix(cleanPath, "/workspace/.blueclaw/") {
		return "", errors.New("path cannot read Blueclaw internal files")
	}
	return cleanPath, nil
}

func cleanHostWorkspacePath(workspacePath string, hostPath string) (string, error) {
	cleanWorkspacePath := filepath.Clean(workspacePath)
	cleanHostPath := filepath.Clean(hostPath)
	relativePath, errorValue := filepath.Rel(cleanWorkspacePath, cleanHostPath)
	if errorValue != nil {
		return "", errorValue
	}
	if relativePath == ".." || strings.HasPrefix(relativePath, "../") {
		return "", errors.New("path must stay under the workspace root")
	}
	resolvedWorkspacePath, workspaceError := filepath.EvalSymlinks(cleanWorkspacePath)
	resolvedHostPath, hostError := filepath.EvalSymlinks(cleanHostPath)
	if workspaceError == nil && hostError == nil {
		relativeResolvedPath, errorValue := filepath.Rel(resolvedWorkspacePath, resolvedHostPath)
		if errorValue != nil {
			return "", errorValue
		}
		if relativeResolvedPath == ".." || strings.HasPrefix(relativeResolvedPath, "../") {
			return "", errors.New("path must stay under the workspace root")
		}
	}
	return cleanHostPath, nil
}

func (service Service) runFileReadHelper(ctx context.Context, request fileReadHelperRequest) (fileReadHelperResponse, error) {
	requestDocument, errorValue := json.Marshal(request)
	if errorValue != nil {
		return fileReadHelperResponse{}, errorValue
	}
	helperPath, cleanup, errorValue := writeFileReadHelper()
	if errorValue != nil {
		return fileReadHelperResponse{}, errorValue
	}
	defer cleanup()
	pythonPath := service.Configuration.WithDefaults().FileReadPythonPath
	var output []byte
	if service.RunCommand != nil {
		output, errorValue = service.RunCommand(ctx, pythonPath, []string{helperPath}, requestDocument)
	} else {
		command := exec.CommandContext(ctx, pythonPath, helperPath)
		command.Stdin = bytes.NewReader(requestDocument)
		output, errorValue = command.CombinedOutput()
	}
	if errorValue != nil {
		return fileReadHelperResponse{}, fmt.Errorf("%w: %s", errorValue, strings.TrimSpace(string(output)))
	}
	var response fileReadHelperResponse
	if errorValue := json.Unmarshal(output, &response); errorValue != nil {
		return fileReadHelperResponse{}, errorValue
	}
	return response, nil
}

func writeFileReadHelper() (string, func(), error) {
	file, errorValue := os.CreateTemp("", "internkim-file-read-*.py")
	if errorValue != nil {
		return "", func() {}, errorValue
	}
	path := file.Name()
	_, writeError := file.WriteString(fileReadHelperDocument)
	closeError := file.Close()
	if writeError != nil {
		_ = os.Remove(path)
		return "", func() {}, writeError
	}
	if closeError != nil {
		_ = os.Remove(path)
		return "", func() {}, closeError
	}
	return path, func() { _ = os.Remove(path) }, nil
}

func openRouterClientBaseURL(value string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(value), "/")
	baseURL = strings.TrimSuffix(baseURL, "/chat/completions")
	return strings.TrimRight(baseURL, "/")
}

func truncateTextByBytes(value string, maximumBytes int) (string, bool) {
	if len(value) <= maximumBytes {
		return value, false
	}
	byteCount := 0
	endIndex := 0
	for endIndex < len(value) {
		runeValue, width := utf8.DecodeRuneInString(value[endIndex:])
		if runeValue == utf8.RuneError && width == 0 {
			break
		}
		if byteCount+width > maximumBytes {
			break
		}
		byteCount += width
		endIndex += width
	}
	return value[:endIndex], true
}

func fileReadErrorResponse(toolName string, message string, code string, stage string, retryable bool) capabilities.ToolInvokeResponse {
	result, _ := json.Marshal(map[string]any{
		"status":       "error",
		"message":      message,
		"errorCode":    code,
		"failureStage": stage,
		"retryable":    retryable,
		"safeRetry":    retryable,
	})
	return capabilities.ToolInvokeResponse{
		Provider:        "markitdown",
		SelectedBackend: capabilities.LLMBackendRemote,
		ToolName:        toolName,
		Status:          "error",
		IsError:         true,
		Message:         message,
		ErrorCode:       code,
		FailureStage:    stage,
		Retryable:       retryable,
		SafeRetry:       retryable,
		Result:          result,
	}
}
