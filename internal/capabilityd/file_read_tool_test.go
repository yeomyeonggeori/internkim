package capabilityd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileReadReturnsMarkdownFromHelper(t *testing.T) {
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

	response, errorValue := service.invokeCapabilityTool(context.Background(), "file.read", strings.NewReader(`{"input":{"path":"/workspace/docs/report.pdf","ocrMode":"auto"}}`))
	if errorValue != nil {
		t.Fatalf("expected file.read: %v", errorValue)
	}
	if response.IsError || response.Content != "# Report\n\nBody" {
		t.Fatalf("unexpected response: %+v", response)
	}
	if helperRequest.Path != sourcePath || helperRequest.OpenRouterAPIKey != "sk-file" || helperRequest.OpenRouterBaseURL != "https://openrouter.test/api/v1" || helperRequest.OpenRouterModel != "openrouter/vision-model" {
		t.Fatalf("unexpected helper request: %+v", helperRequest)
	}
	var result fileReadResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if result.Path != "/workspace/docs/report.pdf" || result.Format != "markdown" || result.Backend != "openrouter" || len(result.Warnings) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestFileReadRejectsUnsafeWorkspacePaths(t *testing.T) {
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
		response, errorValue := service.invokeCapabilityTool(context.Background(), "file.read", strings.NewReader(input))
		if errorValue != nil {
			t.Fatalf("expected structured error response: %v", errorValue)
		}
		if !response.IsError || response.FailureStage != "path_validation" || wasCalled {
			t.Fatalf("expected path validation failure, response=%+v called=%v", response, wasCalled)
		}
	}
}

func TestFileReadAllowsNoOCRWithoutOpenRouterKey(t *testing.T) {
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

	response, errorValue := service.invokeCapabilityTool(context.Background(), "file.read", strings.NewReader(`{"input":{"path":"/workspace/notes.txt","ocrMode":"never"}}`))
	if errorValue != nil {
		t.Fatalf("expected file.read without OCR: %v", errorValue)
	}
	if response.IsError || response.Content != "hello" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestFileReadRequiresOpenRouterKeyForOCR(t *testing.T) {
	workspacePath := t.TempDir()
	writeFileReadTestFile(t, filepath.Join(workspacePath, "scan.pdf"), "pdf")
	service := Service{Configuration: Configuration{
		OpenRouterKeyPath:     filepath.Join(t.TempDir(), "missing"),
		BlueclawWorkspacePath: workspacePath,
	}.WithDefaults()}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "file.read", strings.NewReader(`{"input":{"path":"/workspace/scan.pdf"}}`))
	if errorValue != nil {
		t.Fatalf("expected structured error response: %v", errorValue)
	}
	if !response.IsError || response.ErrorCode != "missing_openrouter_key" || response.FailureStage != "openrouter_configuration" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestFileReadTruncatesLargeOutput(t *testing.T) {
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

	response, errorValue := service.invokeCapabilityTool(context.Background(), "file.read", strings.NewReader(`{"input":{"path":"/workspace/large.txt","maxOutputBytes":1200}}`))
	if errorValue != nil {
		t.Fatalf("expected file.read: %v", errorValue)
	}
	var result fileReadResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !result.Truncated || len([]byte(result.Content)) > 1200 || !strings.HasSuffix(result.Content, "한") {
		t.Fatalf("expected unicode-safe truncation, got bytes=%d result=%+v", len([]byte(result.Content)), result)
	}
}

func TestFileReadHelperFailureReturnsStructuredFacts(t *testing.T) {
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

	response, errorValue := service.invokeCapabilityTool(context.Background(), "file.read", strings.NewReader(`{"input":{"path":"/workspace/broken.pdf"}}`))
	if errorValue != nil {
		t.Fatalf("expected structured error response: %v", errorValue)
	}
	if !response.IsError || response.ErrorCode != "file_read_failed" || response.FailureStage != "markitdown_conversion" || !strings.Contains(response.Message, "markitdown failed") {
		t.Fatalf("unexpected response: %+v", response)
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
