package capabilityd

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeviceBrowserToolRunsThroughLightpandaRuntime(t *testing.T) {
	var commandPath string
	var arguments []string
	service := Service{
		Configuration: Configuration{AgentBrowserPath: "agent-browser-test"},
		RunCommand: func(_ context.Context, path string, commandArguments []string, _ []byte) ([]byte, error) {
			commandPath = path
			arguments = append([]string{}, commandArguments...)
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
	if commandPath != "agent-browser-test" {
		t.Fatalf("unexpected command path: %s", commandPath)
	}
	expectedArguments := []string{"--engine", "lightpanda", "--session-name", "internkim-device", "open", "https://example.com"}
	if strings.Join(arguments, "\x00") != strings.Join(expectedArguments, "\x00") {
		t.Fatalf("unexpected command arguments: %+v", arguments)
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
