package capabilityd

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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

type documentReadInput struct {
	Path           string `json:"path"`
	MaxPages       int    `json:"maxPages"`
	MaxOutputBytes int    `json:"maxOutputBytes"`
}

type documentReadInputDocument struct {
	Path           string `json:"path"`
	MaxPages       *int   `json:"maxPages"`
	MaxOutputBytes *int   `json:"maxOutputBytes"`
}

type imageReadInput struct {
	Path string `json:"path"`
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

type documentReadResult struct {
	Status    string   `json:"status"`
	Path      string   `json:"path"`
	Format    string   `json:"format"`
	Content   string   `json:"content"`
	Backend   string   `json:"backend,omitempty"`
	Model     string   `json:"model,omitempty"`
	Warnings  []string `json:"warnings"`
	Truncated bool     `json:"truncated"`
}

type imageReadAttachment struct {
	DevicePath    string `json:"devicePath"`
	Filename      string `json:"filename"`
	ContentType   string `json:"contentType"`
	SizeBytes     int64  `json:"sizeBytes"`
	ContentBase64 string `json:"contentBase64"`
}

type imageReadResult struct {
	Status      string                `json:"status"`
	Path        string                `json:"path"`
	Attachments []imageReadAttachment `json:"attachments"`
}

type documentConversionAttempt struct {
	Backend string
	Model   string
	Request fileReadHelperRequest
}

func (service Service) invokeDocumentReadTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeDocumentReadInput(request.Input)
	if errorValue != nil {
		return fileReadErrorResponse(request.ToolName, errorValue.Error(), "invalid_input", "input_validation", false), nil
	}
	hostPath, agentPath, errorValue := service.resolveFileReadPath(input.Path)
	if errorValue != nil {
		return fileReadErrorResponse(request.ToolName, errorValue.Error(), "invalid_workspace_path", "path_validation", false), nil
	}
	contentType := detectWorkspaceFileContentType(hostPath)
	if strings.HasPrefix(contentType, "image/") {
		return fileReadErrorResponse(request.ToolName, "image files must be read with image_read", "use_image_read", "content_type", false), nil
	}
	helperResponse, backend, model, errorValue := service.convertDocument(ctx, hostPath, input.MaxPages)
	if errorValue != nil {
		return fileReadErrorResponse(request.ToolName, errorValue.Error(), "document_read_failed", "markitdown_conversion", true), nil
	}
	content := strings.TrimSpace(helperResponse.Content)
	if content == "" {
		return fileReadErrorResponse(request.ToolName, "converted file content was empty", "document_read_empty", "markitdown_conversion", false), nil
	}
	content, isTruncated := truncateTextByBytes(content, input.MaxOutputBytes)
	warnings := helperResponse.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	result := documentReadResult{
		Status:    "ok",
		Path:      agentPath,
		Format:    "markdown",
		Content:   content,
		Backend:   backend,
		Model:     model,
		Warnings:  warnings,
		Truncated: isTruncated,
	}
	resultDocument, errorValue := json.Marshal(result)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilitySuccessResponseFrom(request.ToolName, "ok", resultDocument, capabilityResponseOrigin{
		Provider:        "markitdown",
		SelectedBackend: selectedDocumentBackend(backend),
		Content:         content,
	})
}

func (service Service) invokeImageReadTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeImageReadInput(request.Input)
	if errorValue != nil {
		return fileReadErrorResponse(request.ToolName, errorValue.Error(), "invalid_input", "input_validation", false), nil
	}
	hostPath, agentPath, errorValue := service.resolveFileReadPath(input.Path)
	if errorValue != nil {
		return fileReadErrorResponse(request.ToolName, errorValue.Error(), "invalid_workspace_path", "path_validation", false), nil
	}
	contentType := detectWorkspaceFileContentType(hostPath)
	if !strings.HasPrefix(contentType, "image/") {
		return fileReadErrorResponse(request.ToolName, "non-image files must be read with document_read or file_read", "use_document_read", "content_type", false), nil
	}
	fileInformation, errorValue := os.Stat(hostPath)
	if errorValue != nil {
		return fileReadErrorResponse(request.ToolName, errorValue.Error(), "image_stat_failed", "path_validation", false), nil
	}
	if fileInformation.Size() > maximumInputImagePartBytes {
		return fileReadErrorResponse(request.ToolName, "image is larger than the model input limit", "image_too_large", "image_read", false), nil
	}
	document, errorValue := os.ReadFile(hostPath)
	if errorValue != nil {
		return fileReadErrorResponse(request.ToolName, errorValue.Error(), "image_read_failed", "image_read", true), nil
	}
	result := imageReadResult{
		Status: "ok",
		Path:   agentPath,
		Attachments: []imageReadAttachment{{
			DevicePath:    agentPath,
			Filename:      filepath.Base(agentPath),
			ContentType:   contentType,
			SizeBytes:     fileInformation.Size(),
			ContentBase64: base64.StdEncoding.EncodeToString(document),
		}},
	}
	resultDocument, errorValue := json.Marshal(result)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilitySuccessResponseFrom(request.ToolName, "ok", resultDocument, capabilityResponseOrigin{
		Provider:        "workspace",
		SelectedBackend: capabilities.LLMBackendDevice,
		Content:         "image loaded",
	})
}

func decodeDocumentReadInput(document json.RawMessage) (documentReadInput, error) {
	var documentInput documentReadInputDocument
	if errorValue := decodeStrictFileReadInput(document, &documentInput); errorValue != nil {
		return documentReadInput{}, errorValue
	}
	path := strings.TrimSpace(documentInput.Path)
	if path == "" {
		return documentReadInput{}, errors.New("path is required")
	}
	maxPages := 0
	if documentInput.MaxPages != nil {
		if *documentInput.MaxPages < 1 || *documentInput.MaxPages > 500 {
			return documentReadInput{}, errors.New("maxPages must be between 1 and 500")
		}
		maxPages = *documentInput.MaxPages
	}
	maxOutputBytes := defaultFileReadMaximumOutputBytes
	if documentInput.MaxOutputBytes != nil {
		maxOutputBytes = *documentInput.MaxOutputBytes
	}
	if maxOutputBytes < 1024 || maxOutputBytes > hardFileReadMaximumOutputBytes {
		return documentReadInput{}, fmt.Errorf("maxOutputBytes must be between 1024 and %d", hardFileReadMaximumOutputBytes)
	}
	return documentReadInput{Path: path, MaxPages: maxPages, MaxOutputBytes: maxOutputBytes}, nil
}

func decodeImageReadInput(document json.RawMessage) (imageReadInput, error) {
	var input imageReadInput
	if errorValue := decodeStrictFileReadInput(document, &input); errorValue != nil {
		return imageReadInput{}, errorValue
	}
	input.Path = strings.TrimSpace(input.Path)
	if input.Path == "" {
		return imageReadInput{}, errors.New("path is required")
	}
	return input, nil
}

func decodeStrictFileReadInput(document json.RawMessage, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.DisallowUnknownFields()
	if errorValue := decoder.Decode(destination); errorValue != nil {
		return errorValue
	}
	var trailingDocument json.RawMessage
	if errorValue := decoder.Decode(&trailingDocument); errorValue != io.EOF {
		if errorValue == nil {
			return errors.New("input must contain one JSON value")
		}
		return errorValue
	}
	return nil
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
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		command.Stdout = &stdout
		command.Stderr = &stderr
		errorValue = command.Run()
		output = stdout.Bytes()
		if errorValue != nil {
			return fileReadHelperResponse{}, fmt.Errorf("%w: %s", errorValue, strings.TrimSpace(stderr.String()))
		}
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

func (service Service) convertDocument(ctx context.Context, hostPath string, maxPages int) (fileReadHelperResponse, string, string, error) {
	attempts := service.documentConversionAttempts(hostPath, maxPages)
	failures := []string{}
	for _, attempt := range attempts {
		response, errorValue := service.runFileReadHelper(ctx, attempt.Request)
		if errorValue == nil {
			return response, attempt.Backend, attempt.Model, nil
		}
		failures = append(failures, attempt.Backend+": "+errorValue.Error())
	}
	fallbackResponse, errorValue := service.runFileReadHelper(ctx, fileReadHelperRequest{
		Path:     hostPath,
		OCRMode:  "never",
		MaxPages: maxPages,
	})
	if errorValue != nil {
		return fileReadHelperResponse{}, "", "", errors.New(strings.Join(append(failures, "no_ocr: "+errorValue.Error()), "; "))
	}
	fallbackResponse.Warnings = append(fallbackResponse.Warnings, "OCR failed; returned non-OCR extraction. "+strings.Join(failures, "; "))
	return fallbackResponse, "markitdown", "no_ocr", nil
}

func (service Service) documentConversionAttempts(hostPath string, maxPages int) []documentConversionAttempt {
	configuration := service.Configuration.WithDefaults()
	attempts := []documentConversionAttempt{}
	apiKey := readSecretValue(configuration.OpenRouterKeyPath)
	if documentConversionShouldTryOCR(hostPath) && !configuration.LocalOnly && strings.TrimSpace(apiKey) != "" && !isPlaceholderOpenRouterKey(apiKey) {
		attempts = append(attempts, documentConversionAttempt{
			Backend: "openrouter",
			Model:   configuration.OpenRouterModel,
			Request: fileReadHelperRequest{
				Path:              hostPath,
				OCRMode:           "always",
				MaxPages:          maxPages,
				OpenRouterAPIKey:  apiKey,
				OpenRouterBaseURL: openRouterClientBaseURL(configuration.OpenRouterBaseURL),
				OpenRouterModel:   configuration.OpenRouterModel,
			},
		})
	}
	return attempts
}

func documentConversionShouldTryOCR(hostPath string) bool {
	return strings.EqualFold(filepath.Ext(strings.TrimSpace(hostPath)), ".pdf")
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

func selectedDocumentBackend(backend string) string {
	if strings.TrimSpace(backend) == "local" {
		return capabilities.LLMBackendDevice
	}
	return capabilities.LLMBackendRemote
}

func detectWorkspaceFileContentType(path string) string {
	file, errorValue := os.Open(path)
	if errorValue != nil {
		return "application/octet-stream"
	}
	defer file.Close()
	buffer := make([]byte, 512)
	byteCount, _ := file.Read(buffer)
	return http.DetectContentType(buffer[:byteCount])
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
	provider, selectedBackend := fileReadErrorIdentity(toolName)
	return capabilities.ToolInvokeResponse{
		Provider:        provider,
		SelectedBackend: selectedBackend,
		ToolName:        toolName,
		Outcome:         capabilities.ToolOutcomeFailed,
		Effects:         []capabilities.ResourceEffect{},
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

func fileReadErrorIdentity(toolName string) (string, string) {
	if strings.TrimSpace(toolName) == "image_read" {
		return "workspace", capabilities.LLMBackendDevice
	}
	return "markitdown", capabilities.LLMBackendRemote
}
