package capabilityd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestDocumentReadReturnsMarkdownFromHelper(t *testing.T) {
	workspacePath := t.TempDir()
	sourcePath := filepath.Join(workspacePath, "docs", "report.pdf")
	writeFileReadTestFile(t, sourcePath, "pdf")
	secretPath := writeOpenRouterSecretForWebToolTest(t, "sk-file")
	var helperRequest fileReadHelperRequest
	service := Service{
		Configuration: Configuration{
			OpenRouterKeyPath:     secretPath,
			OpenRouterBaseURL:     "https://openrouter.test/api/v1/chat/completions",
			OpenRouterModel:       "openrouter/vision-model",
			BlueclawWorkspacePath: workspacePath,
			FileReadPythonPath:    "/test/python",
		}.WithDefaults(),
		RunCommand: func(ctx context.Context, executable string, arguments []string, input []byte) ([]byte, error) {
			if executable != "/test/python" {
				t.Fatalf("unexpected executable: %s", executable)
			}
			if errorValue := json.Unmarshal(input, &helperRequest); errorValue != nil {
				t.Fatal(errorValue)
			}
			return []byte(`{"content":"# Report\n\nBody","warnings":["ok"]}`), nil
		},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "document.read", strings.NewReader(`{"input":{"path":"/workspace/docs/report.pdf"}}`))
	if errorValue != nil {
		t.Fatalf("expected document.read: %v", errorValue)
	}
	if response.IsError || response.Content != "# Report\n\nBody" {
		t.Fatalf("unexpected response: %+v", response)
	}
	if response.Provider != "markitdown" || response.SelectedBackend != capabilities.LLMBackendRemote || response.ToolName != "document.read" || response.Outcome != capabilities.ToolOutcomeSucceeded || response.Effects == nil || len(response.Effects) != 0 {
		t.Fatalf("unexpected response identity: %+v", response)
	}
	if helperRequest.Path != sourcePath || helperRequest.OpenRouterAPIKey != "sk-file" || helperRequest.OpenRouterBaseURL != "https://openrouter.test/api/v1" || helperRequest.OpenRouterModel != "openrouter/vision-model" {
		t.Fatalf("unexpected helper request: %+v", helperRequest)
	}
	var result documentReadResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if result.Path != "/workspace/docs/report.pdf" || result.Format != "markdown" || result.Backend != "openrouter" || len(result.Warnings) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Status != "ok" || result.Truncated {
		t.Fatalf("unexpected result status: %+v", result)
	}
}

func TestDocumentReadRejectsUnsupportedInputFields(t *testing.T) {
	service := Service{Configuration: Configuration{BlueclawWorkspacePath: t.TempDir()}.WithDefaults()}
	for _, input := range []string{
		`{"input":{"path":"/workspace/report.pdf","materialID":"material-1"}}`,
		`{"input":{"path":"/workspace/report.pdf","unexpected":true}}`,
		`{"input":{"path":"/workspace/report.pdf","maxPages":0}}`,
		`{"input":{"path":"/workspace/report.pdf","maxOutputBytes":0}}`,
	} {
		response, errorValue := service.invokeCapabilityTool(context.Background(), "document.read", strings.NewReader(input))
		if errorValue != nil {
			t.Fatalf("expected structured input error: %v", errorValue)
		}
		if !response.IsError || response.ErrorCode != "invalid_input" {
			t.Fatalf("expected invalid input response, got %+v", response)
		}
		if response.Provider != "markitdown" || response.SelectedBackend != capabilities.LLMBackendRemote || response.ToolName != "document.read" || response.Outcome != capabilities.ToolOutcomeFailed || response.Effects == nil || len(response.Effects) != 0 {
			t.Fatalf("unexpected document error identity: %+v", response)
		}
	}
}

func TestDocumentReadRejectsUnsafeWorkspacePaths(t *testing.T) {
	workspacePath := t.TempDir()
	secretPath := writeOpenRouterSecretForWebToolTest(t, "sk-file")
	wasCalled := false
	service := Service{
		Configuration: Configuration{
			OpenRouterKeyPath:     secretPath,
			BlueclawWorkspacePath: workspacePath,
		}.WithDefaults(),
		RunCommand: func(ctx context.Context, executable string, arguments []string, input []byte) ([]byte, error) {
			wasCalled = true
			return nil, nil
		},
	}

	for _, input := range []string{
		`{"input":{"path":"/etc/passwd"}}`,
		`{"input":{"path":"/workspace/../etc/passwd"}}`,
		`{"input":{"path":"/workspace/.blueclaw/config/runtime.json"}}`,
	} {
		response, errorValue := service.invokeCapabilityTool(context.Background(), "document.read", strings.NewReader(input))
		if errorValue != nil {
			t.Fatalf("expected structured error response: %v", errorValue)
		}
		if !response.IsError || response.FailureStage != "path_validation" || wasCalled {
			t.Fatalf("expected path validation failure, response=%+v called=%v", response, wasCalled)
		}
	}
}

func TestDocumentReadFallsBackToNoOCRWithoutOpenRouterKey(t *testing.T) {
	workspacePath := t.TempDir()
	sourcePath := filepath.Join(workspacePath, "notes.txt")
	writeFileReadTestFile(t, sourcePath, "hello")
	service := Service{
		Configuration: Configuration{
			OpenRouterKeyPath:     filepath.Join(t.TempDir(), "missing"),
			BlueclawWorkspacePath: workspacePath,
			FileReadPythonPath:    "/test/python",
		}.WithDefaults(),
		RunCommand: func(ctx context.Context, executable string, arguments []string, input []byte) ([]byte, error) {
			return []byte(`{"content":"hello"}`), nil
		},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "document.read", strings.NewReader(`{"input":{"path":"/workspace/notes.txt"}}`))
	if errorValue != nil {
		t.Fatalf("expected document.read without OCR: %v", errorValue)
	}
	if response.IsError || response.Content != "hello" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestDocumentReadUsesNoOCRForHTMLWithOpenRouterKey(t *testing.T) {
	workspacePath := t.TempDir()
	sourcePath := filepath.Join(workspacePath, "index.html")
	writeFileReadTestFile(t, sourcePath, "<h1>HTML Title</h1>")
	secretPath := writeOpenRouterSecretForWebToolTest(t, "sk-file")
	var helperRequest fileReadHelperRequest
	service := Service{
		Configuration: Configuration{
			OpenRouterKeyPath:     secretPath,
			OpenRouterBaseURL:     "https://openrouter.test/api/v1/chat/completions",
			OpenRouterModel:       "openrouter/vision-model",
			BlueclawWorkspacePath: workspacePath,
			FileReadPythonPath:    "/test/python",
		}.WithDefaults(),
		RunCommand: func(ctx context.Context, executable string, arguments []string, input []byte) ([]byte, error) {
			if errorValue := json.Unmarshal(input, &helperRequest); errorValue != nil {
				t.Fatal(errorValue)
			}
			return []byte(`{"content":"# HTML Title"}`), nil
		},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "document.read", strings.NewReader(`{"input":{"path":"/workspace/index.html"}}`))
	if errorValue != nil {
		t.Fatalf("expected document.read: %v", errorValue)
	}
	if response.IsError || response.Content != "# HTML Title" {
		t.Fatalf("unexpected response: %+v", response)
	}
	if helperRequest.Path != sourcePath || helperRequest.OCRMode != "never" || helperRequest.OpenRouterAPIKey != "" {
		t.Fatalf("expected HTML to use no-OCR conversion, got %+v", helperRequest)
	}
}

func TestFileReadHelperIgnoresStderrNoise(t *testing.T) {
	workspacePath := t.TempDir()
	pythonPath := filepath.Join(workspacePath, "fake-python")
	writeFileReadTestFile(t, pythonPath, "#!/bin/sh\nprintf '\\033[32mnoise\\033[0m\\n' >&2\nprintf '{\"content\":\"# Clean\"}'\n")
	if errorValue := os.Chmod(pythonPath, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	service := Service{
		Configuration: Configuration{
			BlueclawWorkspacePath: workspacePath,
			FileReadPythonPath:    pythonPath,
		}.WithDefaults(),
	}

	response, errorValue := service.runFileReadHelper(context.Background(), fileReadHelperRequest{Path: filepath.Join(workspacePath, "document.html")})
	if errorValue != nil {
		t.Fatalf("expected stderr noise to be ignored: %v", errorValue)
	}
	if response.Content != "# Clean" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestDocumentReadUsesNoOCRFallbackWhenOCRAttemptFails(t *testing.T) {
	workspacePath := t.TempDir()
	writeFileReadTestFile(t, filepath.Join(workspacePath, "scan.pdf"), "pdf")
	secretPath := writeOpenRouterSecretForWebToolTest(t, "sk-file")
	callCount := 0
	service := Service{
		Configuration: Configuration{
			OpenRouterKeyPath:     secretPath,
			BlueclawWorkspacePath: workspacePath,
			FileReadPythonPath:    "/test/python",
		}.WithDefaults(),
		RunCommand: func(ctx context.Context, executable string, arguments []string, input []byte) ([]byte, error) {
			callCount++
			if callCount == 1 {
				return []byte("ocr failed"), errors.New("exit status 1")
			}
			return []byte(`{"content":"fallback text"}`), nil
		},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "document.read", strings.NewReader(`{"input":{"path":"/workspace/scan.pdf"}}`))
	if errorValue != nil {
		t.Fatalf("expected document.read: %v", errorValue)
	}
	if response.IsError || response.Content != "fallback text" || callCount != 2 {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestDocumentReadTruncatesLargeOutput(t *testing.T) {
	workspacePath := t.TempDir()
	writeFileReadTestFile(t, filepath.Join(workspacePath, "large.txt"), "large")
	secretPath := writeOpenRouterSecretForWebToolTest(t, "sk-file")
	service := Service{
		Configuration: Configuration{
			OpenRouterKeyPath:     secretPath,
			BlueclawWorkspacePath: workspacePath,
			FileReadPythonPath:    "/test/python",
		}.WithDefaults(),
		RunCommand: func(ctx context.Context, executable string, arguments []string, input []byte) ([]byte, error) {
			content, _ := json.Marshal(map[string]string{"content": strings.Repeat("한", 900)})
			return content, nil
		},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "document.read", strings.NewReader(`{"input":{"path":"/workspace/large.txt","maxOutputBytes":1200}}`))
	if errorValue != nil {
		t.Fatalf("expected document.read: %v", errorValue)
	}
	var result documentReadResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !result.Truncated || len([]byte(result.Content)) > 1200 || !strings.HasSuffix(result.Content, "한") {
		t.Fatalf("expected unicode-safe truncation, got bytes=%d result=%+v", len([]byte(result.Content)), result)
	}
}

func TestDocumentReadHelperFailureReturnsStructuredFacts(t *testing.T) {
	workspacePath := t.TempDir()
	writeFileReadTestFile(t, filepath.Join(workspacePath, "broken.pdf"), "pdf")
	secretPath := writeOpenRouterSecretForWebToolTest(t, "sk-file")
	service := Service{
		Configuration: Configuration{
			OpenRouterKeyPath:     secretPath,
			BlueclawWorkspacePath: workspacePath,
			FileReadPythonPath:    "/test/python",
		}.WithDefaults(),
		RunCommand: func(ctx context.Context, executable string, arguments []string, input []byte) ([]byte, error) {
			return []byte("markitdown failed"), errors.New("exit status 1")
		},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "document.read", strings.NewReader(`{"input":{"path":"/workspace/broken.pdf"}}`))
	if errorValue != nil {
		t.Fatalf("expected structured error response: %v", errorValue)
	}
	if !response.IsError || response.ErrorCode != "document_read_failed" || response.FailureStage != "markitdown_conversion" || !strings.Contains(response.Message, "markitdown failed") {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestImageReadReturnsImageAttachment(t *testing.T) {
	workspacePath := t.TempDir()
	imageDocument := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	imagePath := filepath.Join(workspacePath, "uploads", "screen.png")
	writeFileReadTestFile(t, imagePath, string(imageDocument))
	service := Service{Configuration: Configuration{
		BlueclawWorkspacePath: workspacePath,
	}.WithDefaults()}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "image.read", strings.NewReader(`{"input":{"path":"/workspace/uploads/screen.png"}}`))
	if errorValue != nil {
		t.Fatalf("expected image.read: %v", errorValue)
	}
	var result imageReadResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.IsError || len(result.Attachments) != 1 || response.Provider != "workspace" || response.SelectedBackend != capabilities.LLMBackendDevice || response.ToolName != "image.read" || response.Outcome != capabilities.ToolOutcomeSucceeded || response.Effects == nil || len(response.Effects) != 0 {
		t.Fatalf("unexpected response: %+v", response)
	}
	attachment := result.Attachments[0]
	if result.Status != "ok" || result.Path != "/workspace/uploads/screen.png" || attachment.DevicePath != "/workspace/uploads/screen.png" || attachment.Filename != "screen.png" || attachment.ContentType != "image/png" || attachment.SizeBytes != int64(len(imageDocument)) || attachment.ContentBase64 != base64.StdEncoding.EncodeToString(imageDocument) {
		t.Fatalf("unexpected image attachment: %+v", attachment)
	}
}

func TestImageReadRejectsUnsupportedInputFields(t *testing.T) {
	service := Service{Configuration: Configuration{BlueclawWorkspacePath: t.TempDir()}.WithDefaults()}
	for _, input := range []string{
		`{"input":{"path":"/workspace/screen.png","materialID":"material-1"}}`,
		`{"input":{"path":"/workspace/screen.png","unexpected":true}}`,
	} {
		response, errorValue := service.invokeCapabilityTool(context.Background(), "image.read", strings.NewReader(input))
		if errorValue != nil {
			t.Fatalf("expected structured input error: %v", errorValue)
		}
		if !response.IsError || response.ErrorCode != "invalid_input" {
			t.Fatalf("expected invalid input response, got %+v", response)
		}
		if response.Provider != "workspace" || response.SelectedBackend != capabilities.LLMBackendDevice || response.ToolName != "image.read" || response.Outcome != capabilities.ToolOutcomeFailed || response.Effects == nil || len(response.Effects) != 0 {
			t.Fatalf("unexpected image error identity: %+v", response)
		}
	}
}

func writeFileReadTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(path, []byte(content), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
}
