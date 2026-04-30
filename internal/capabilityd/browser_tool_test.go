package capabilityd

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeviceBrowserToolRunsThroughChromeRuntime(t *testing.T) {
	type commandCall struct {
		path      string
		arguments []string
	}
	var calls []commandCall
	profileDirectory := t.TempDir()
	service := Service{
		Configuration: Configuration{AgentBrowserPath: "agent-browser-test", DeviceBrowserProfilePath: profileDirectory},
		RunCommand: func(_ context.Context, path string, commandArguments []string, _ []byte) ([]byte, error) {
			calls = append(calls, commandCall{path: path, arguments: append([]string{}, commandArguments...)})
			return nil, nil
		},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "browser.open", strings.NewReader(`{"input":{"url":"https://example.com"}}`))
	if errorValue != nil {
		t.Fatalf("expected device browser response: %v", errorValue)
	}
	if response.Provider != "device" || response.ToolName != "browser.open" {
		t.Fatalf("unexpected response: %+v", response)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls (open + stealth eval), got %d: %+v", len(calls), calls)
	}
	if calls[0].path != "agent-browser-test" {
		t.Fatalf("unexpected open command path: %s", calls[0].path)
	}
	expectedOpenArguments := []string{"--session", "internkim-device", "--engine", "chrome", "--executable-path", "/opt/internkim/device-browser/chromium", "--headed", "false", "--profile", profileDirectory, "--session-name", "internkim-device", "open", "https://example.com", "--headers"}
	if len(calls[0].arguments) < len(expectedOpenArguments)+1 {
		t.Fatalf("unexpected open arguments length: %+v", calls[0].arguments)
	}
	for index, expected := range expectedOpenArguments {
		if calls[0].arguments[index] != expected {
			t.Fatalf("unexpected open argument at index %d: got %q want %q (full: %+v)", index, calls[0].arguments[index], expected, calls[0].arguments)
		}
	}
	if !strings.Contains(calls[0].arguments[len(expectedOpenArguments)], "User-Agent") {
		t.Fatalf("expected stealth User-Agent header in open arguments, got %+v", calls[0].arguments)
	}
	if calls[1].arguments[len(calls[1].arguments)-2] != "eval" {
		t.Fatalf("expected second call to be eval, got %+v", calls[1].arguments)
	}
}

func TestDeviceBrowserScreenshotReturnsTemporaryDevicePath(t *testing.T) {
	fileDirectory := t.TempDir()
	service := Service{
		Configuration: Configuration{
			AgentBrowserPath:       "agent-browser-test",
			CompanionFileDirectory: fileDirectory,
		},
		RunCommand: func(_ context.Context, _ string, commandArguments []string, _ []byte) ([]byte, error) {
			if len(commandArguments) == 0 || commandArguments[len(commandArguments)-2] != "screenshot" {
				t.Fatalf("expected screenshot command, got %+v", commandArguments)
			}
			outputPath := commandArguments[len(commandArguments)-1]
			if errorValue := os.WriteFile(outputPath, []byte("png"), 0o600); errorValue != nil {
				t.Fatalf("failed to write fake screenshot: %v", errorValue)
			}
			return nil, nil
		},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "browser.screenshot", strings.NewReader(`{"input":{"ttlSeconds":600}}`))
	if errorValue != nil {
		t.Fatalf("expected screenshot response: %v", errorValue)
	}
	var result struct {
		DevicePath string `json:"devicePath"`
		Filename   string `json:"filename"`
	}
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatalf("expected screenshot result: %v", errorValue)
	}
	if !strings.HasPrefix(result.DevicePath, fileDirectory) {
		t.Fatalf("expected device temp path, got %q", result.DevicePath)
	}
	if strings.Contains(string(response.Result), "internkim-companion-browser") {
		t.Fatalf("response leaked local screenshot path: %s", response.Result)
	}
	if _, errorValue := os.Stat(result.DevicePath); errorValue != nil {
		t.Fatalf("expected device screenshot file: %v", errorValue)
	}
	if _, errorValue := os.Stat(result.DevicePath + ".internkim-meta.json"); errorValue != nil {
		t.Fatalf("expected screenshot metadata: %v", errorValue)
	}
	if filepath.Base(result.DevicePath) != result.Filename {
		t.Fatalf("expected filename to match device path, got %+v", result)
	}
}
