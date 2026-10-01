package capabilityd

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

func browserSnapshotFrom(requesterEmail string) string {
	return `{"input":{},"context":{"requesterEmail":"` + requesterEmail + `"}}`
}

func TestEachRequesterDrivesTheirOwnDeviceBrowser(t *testing.T) {
	var argumentsByCall [][]string
	service := Service{
		Configuration:  Configuration{AgentBrowserPath: "agent-browser-test"},
		DeviceBrowsers: fakeDeviceBrowsers(4, time.Now()),
		RunCommand: func(_ context.Context, _ string, commandArguments []string, _ []byte) ([]byte, error) {
			argumentsByCall = append(argumentsByCall, append([]string{}, commandArguments...))
			return []byte(`{"success":true,"data":{"snapshot":"","refs":{}}}`), nil
		},
	}

	for _, requesterEmail := range []string{"kim@example.test", "lee@example.test"} {
		response, errorValue := service.invokeCapabilityTool(context.Background(), "browser_snapshot", strings.NewReader(browserSnapshotFrom(requesterEmail)))
		if errorValue != nil || response.Outcome != capabilities.ToolOutcomeSucceeded {
			t.Fatalf("snapshot for %s: response=%+v error=%v", requesterEmail, response, errorValue)
		}
	}

	if len(argumentsByCall) != 2 {
		t.Fatalf("expected two snapshots, got %+v", argumentsByCall)
	}
	first := strings.Join(argumentsByCall[0], " ")
	second := strings.Join(argumentsByCall[1], " ")
	if !strings.Contains(first, "--cdp http://127.0.0.1:9230") || !strings.Contains(second, "--cdp http://127.0.0.1:9231") {
		t.Fatalf("expected each requester on their own browser, got %q and %q", first, second)
	}
	if argumentsByCall[0][1] == argumentsByCall[1][1] {
		t.Fatalf("expected separate agent-browser sessions, got %q and %q", first, second)
	}
}

func TestABusyDeviceTellsTheAgentTheBrowserIsTaken(t *testing.T) {
	browsers, launchStarted, finishLaunch := deviceBrowsersWhoseLaunchWaits(1)
	service := Service{
		Configuration:  Configuration{AgentBrowserPath: "agent-browser-test", DeviceBrowserCapacity: 1},
		DeviceBrowsers: browsers,
		RunCommand: func(context.Context, string, []string, []byte) ([]byte, error) {
			return []byte(`{"success":true,"data":{"snapshot":"","refs":{}}}`), nil
		},
	}
	firstDone := make(chan error, 1)
	go func() {
		_, errorValue := service.invokeCapabilityTool(context.Background(), "browser_snapshot", strings.NewReader(browserSnapshotFrom("kim@example.test")))
		firstDone <- errorValue
	}()
	<-launchStarted

	response, errorValue := service.invokeCapabilityTool(context.Background(), "browser_snapshot", strings.NewReader(browserSnapshotFrom("lee@example.test")))
	finishLaunch()

	if errorValue != nil {
		t.Fatalf("expected a failed response, got error %v", errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed || !strings.Contains(response.Content, "all 1 device browsers are in use") {
		t.Fatalf("response = %+v", response)
	}
	if errorValue := <-firstDone; errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestDeviceBrowserToolRunsThroughMoliRuntime(t *testing.T) {
	type commandCall struct {
		path      string
		arguments []string
	}
	var calls []commandCall
	service := Service{
		Configuration:  Configuration{AgentBrowserPath: "agent-browser-test"},
		DeviceBrowsers: fakeDeviceBrowsers(4, time.Now()),
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
	expectedOpenArguments := []string{"--session", "internkim-device-unattributed", "--cdp", "http://127.0.0.1:9230", "--session-name", "internkim-device-unattributed", "open", "https://example.com"}
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

func TestDeviceBrowserRefusesAScreenshot(t *testing.T) {
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
		t.Fatalf("expected a not-connected screenshot denial, got response=%+v result=%+v", response, result)
	}
	if commandWasCalled {
		t.Fatal("expected device screenshot denial not to run agent-browser")
	}
}
