package capabilityd

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestDeviceBrowserToolRunsThroughLightpandaRuntime(t *testing.T) {
	type commandCall struct {
		path      string
		arguments []string
	}
	var calls []commandCall
	service := Service{
		Configuration: Configuration{AgentBrowserPath: "agent-browser-test", DeviceBrowserPath: "/usr/local/bin/lightpanda", DeviceBrowserProfilePath: "/profile"},
		RunCommand: func(_ context.Context, path string, commandArguments []string, _ []byte) ([]byte, error) {
			calls = append(calls, commandCall{path: path, arguments: append([]string{}, commandArguments...)})
			if slices.Contains(commandArguments, "get") && slices.Contains(commandArguments, "url") {
				return []byte("https://example.com\n"), nil
			}
			return nil, nil
		},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "browser_open", strings.NewReader(`{"input":{"url":"https://example.com"}}`))
	if errorValue != nil {
		t.Fatalf("expected device browser response: %v", errorValue)
	}
	if response.Provider != "device" || response.ToolName != "browser_open" || response.Outcome != capabilities.ToolOutcomeSucceeded {
		t.Fatalf("unexpected response: %+v", response)
	}
	if len(calls) != 3 {
		t.Fatalf("expected 3 calls (open + url check + snapshot), got %d: %+v", len(calls), calls)
	}
	if calls[0].path != "agent-browser-test" {
		t.Fatalf("unexpected open command path: %s", calls[0].path)
	}
	expectedOpenArguments := []string{"--session", "internkim-device", "--engine", "lightpanda", "--executable-path", "/usr/local/bin/lightpanda", "--session-name", "internkim-device", "open", "https://example.com"}
	if !slices.Equal(calls[0].arguments, expectedOpenArguments) {
		t.Fatalf("unexpected open arguments: %+v", calls[0].arguments)
	}
	if calls[1].arguments[len(calls[1].arguments)-2] != "get" || calls[1].arguments[len(calls[1].arguments)-1] != "url" {
		t.Fatalf("expected second call to be url check, got %+v", calls[1].arguments)
	}
	if calls[2].arguments[len(calls[2].arguments)-4] != "snapshot" ||
		calls[2].arguments[len(calls[2].arguments)-3] != "-i" {
		t.Fatalf("expected third call to be snapshot, got %+v", calls[2].arguments)
	}
}

func TestDeviceBrowserScreenshotRequiresCompanion(t *testing.T) {
	commandWasCalled := false
	service := Service{RunCommand: func(_ context.Context, _ string, _ []string, _ []byte) ([]byte, error) {
		commandWasCalled = true
		return nil, nil
	}}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "browser_screenshot", strings.NewReader(`{"input":{"ttlSeconds":600}}`))
	if errorValue != nil {
		t.Fatalf("expected structured screenshot denial: %v", errorValue)
	}
	var result struct {
		Status string `json:"status"`
		Code   string `json:"code"`
	}
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatalf("expected denial result: %v", errorValue)
	}
	if !response.IsError || response.Status != "denied" || result.Code != capabilities.CapabilityNotConnected {
		t.Fatalf("expected companion-required screenshot denial, got response=%+v result=%+v", response, result)
	}
	if commandWasCalled {
		t.Fatal("expected device screenshot denial not to run agent-browser")
	}
}
