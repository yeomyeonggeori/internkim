package capabilityd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	browserruntime "gitlab.com/eastriver/internkim/internal/browser"
	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type deviceBrowserFileMetadata struct {
	FileID      string    `json:"fileID"`
	Filename    string    `json:"filename"`
	DevicePath  string    `json:"devicePath"`
	ExpiresAt   time.Time `json:"expiresAt"`
	CompanionID string    `json:"companionID"`
	JobID       string    `json:"jobID"`
}

func (service Service) invokeDeviceBrowserTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	browserRuntime := service.deviceBrowserRuntime()
	var result any
	var errorValue error

	switch request.ToolName {
	case "browser.open":
		var input browserruntime.NavigateRequest
		errorValue = decodeBrowserToolInput(request.Input, &input)
		if errorValue == nil {
			result, errorValue = browserRuntime.Navigate(ctx, input)
		}
	case "browser.snapshot":
		var input browserruntime.ObserveRequest
		errorValue = decodeBrowserToolInput(request.Input, &input)
		if errorValue == nil {
			observation, observationError := browserRuntime.Observe(ctx, input)
			if observationError != nil {
				errorValue = observationError
				break
			}
			if blockedResponse, isBlocked := captchaBlockedResponse(request.ToolName, observation.SnapshotText); isBlocked {
				return blockedResponse, nil
			}
			result = observation
		}
	case "browser.screenshot":
		return capabilityUnavailableResponse(request.ToolName, capabilities.CapabilityNotConnected), nil
	case "browser.click":
		var input browserruntime.ClickRequest
		errorValue = decodeBrowserToolInput(request.Input, &input)
		if errorValue == nil {
			result, errorValue = browserRuntime.Click(ctx, input)
		}
	case "browser.fill":
		var input browserruntime.FillRequest
		errorValue = decodeBrowserToolInput(request.Input, &input)
		if errorValue == nil {
			result, errorValue = browserRuntime.Fill(ctx, input)
		}
	case "browser.select":
		var input browserruntime.SelectRequest
		errorValue = decodeBrowserToolInput(request.Input, &input)
		if errorValue == nil {
			result, errorValue = browserRuntime.Select(ctx, input)
		}
	case "browser.press":
		var input browserruntime.PressRequest
		errorValue = decodeBrowserToolInput(request.Input, &input)
		if errorValue == nil {
			result, errorValue = browserRuntime.Press(ctx, input)
		}
	case "browser.wait":
		var input browserruntime.WaitRequest
		errorValue = decodeBrowserToolInput(request.Input, &input)
		if errorValue == nil {
			result, errorValue = browserRuntime.Wait(ctx, input)
		}
	default:
		errorValue = errors.New("device browser capability is not configured: " + request.ToolName)
	}
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	document, errorValue := json.Marshal(result)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilities.ToolInvokeResponse{
		Provider:        "device",
		SelectedBackend: capabilities.LLMBackendDevice,
		ToolName:        request.ToolName,
		Outcome:         capabilities.ToolOutcomeSucceeded,
		Status:          "ok",
		Result:          document,
	}, nil
}

func (service Service) deviceBrowserRuntime() browserruntime.AgentBrowserRuntime {
	configuration := service.Configuration.WithDefaults()
	return browserruntime.AgentBrowserRuntime{
		CommandPath:          configuration.AgentBrowserPath,
		Engine:               browserruntime.BrowserEngineLightpanda,
		EngineExecutablePath: configuration.DeviceBrowserPath,
		SessionName:          "internkim-device",
		Runner:               service.browserCommandRunner(),
	}
}

func (service Service) browserCommandRunner() browserruntime.CommandRunner {
	if service.RunCommand == nil {
		return nil
	}
	return serviceBrowserCommandRunner{RunCommand: service.RunCommand}
}

type serviceBrowserCommandRunner struct {
	RunCommand func(context.Context, string, []string, []byte) ([]byte, error)
}

func (runner serviceBrowserCommandRunner) Run(ctx context.Context, commandPath string, arguments []string) ([]byte, error) {
	return runner.RunCommand(ctx, commandPath, arguments, nil)
}

func (service Service) captureDeviceBrowserScreenshot(ctx context.Context, browserRuntime browserruntime.AgentBrowserRuntime, request capabilities.ToolInvokeRequest) (map[string]any, error) {
	var input struct {
		TTLSeconds int `json:"ttlSeconds,omitempty"`
	}
	if errorValue := decodeBrowserToolInput(request.Input, &input); errorValue != nil {
		return nil, errorValue
	}
	screenshot, errorValue := browserRuntime.Screenshot(ctx, browserruntime.ScreenshotRequest{})
	if errorValue != nil {
		return nil, errorValue
	}
	devicePath, expiresAt, fileID, errorValue := service.publishDeviceBrowserFile(screenshot, input.TTLSeconds)
	if errorValue != nil {
		return nil, errorValue
	}
	return map[string]any{
		"fileID":      fileID,
		"filename":    screenshot.Filename,
		"sizeBytes":   screenshot.SizeBytes,
		"contentType": screenshot.ContentType,
		"devicePath":  devicePath,
		"expiresAt":   expiresAt.Format(time.RFC3339),
		"capturedAt":  screenshot.CapturedAt,
	}, nil
}

func (service Service) publishDeviceBrowserFile(screenshot browserruntime.ScreenshotResult, ttlSeconds int) (string, time.Time, string, error) {
	configuration := service.Configuration.WithDefaults()
	filename := safeDeviceBrowserFilename(screenshot.Filename)
	if filename == "" {
		filename = "browser-screenshot.png"
	}
	if errorValue := os.MkdirAll(configuration.CompanionFileDirectory, 0o700); errorValue != nil {
		return "", time.Time{}, "", errorValue
	}
	devicePath := filepath.Join(configuration.CompanionFileDirectory, filename)
	if errorValue := copyFile(screenshot.LocalPath, devicePath); errorValue != nil {
		return "", time.Time{}, "", errorValue
	}
	expiresAt := time.Now().UTC().Add(clampDeviceBrowserFileTTL(ttlSeconds))
	fileID := randomFileID()
	metadata := deviceBrowserFileMetadata{
		FileID:      fileID,
		Filename:    filename,
		DevicePath:  devicePath,
		ExpiresAt:   expiresAt,
		CompanionID: "device",
		JobID:       "capabilityd-browser",
	}
	if errorValue := writeDeviceBrowserFileMetadata(metadata); errorValue != nil {
		return "", time.Time{}, "", errorValue
	}
	_ = os.Remove(screenshot.LocalPath)
	return devicePath, expiresAt, fileID, nil
}

func captchaBlockedResponse(toolName string, snapshotText string) (capabilities.ToolInvokeResponse, bool) {
	isBlocked, matchedSignature := browserruntime.IsCaptchaBlocked(snapshotText)
	if !isBlocked {
		return capabilities.ToolInvokeResponse{}, false
	}
	log.Printf("tool.browser.blocked_by_captcha: tool=%s signature=%q", toolName, matchedSignature)
	message := fmt.Sprintf(
		"blocked_by_captcha: this URL returned a bot-detection wall (matched: %q). "+
			"Do NOT pretend you have the information from this page. "+
			"Tell the user that the page needs the speaker's Companion browser, then stop.",
		matchedSignature,
	)
	return capabilities.ToolInvokeResponse{
		Provider:        "device",
		SelectedBackend: capabilities.LLMBackendDevice,
		ToolName:        toolName,
		Outcome:         capabilities.ToolOutcomeFailed,
		Status:          "error",
		Content:         message,
		IsError:         true,
	}, true
}

func decodeBrowserToolInput(input json.RawMessage, value any) error {
	if len(input) == 0 {
		return nil
	}
	if errorValue := json.Unmarshal(input, value); errorValue != nil {
		return errors.New("browser tool input is not valid json: " + errorValue.Error())
	}
	return nil
}

func safeDeviceBrowserFilename(filename string) string {
	name := filepath.Base(strings.TrimSpace(filename))
	builder := strings.Builder{}
	for _, value := range name {
		if unicode.IsLetter(value) || unicode.IsDigit(value) || value == '.' || value == '-' || value == '_' {
			builder.WriteRune(value)
		}
	}
	return builder.String()
}

func copyFile(sourcePath string, destinationPath string) error {
	source, errorValue := os.Open(sourcePath)
	if errorValue != nil {
		return errorValue
	}
	defer source.Close()

	destination, errorValue := os.OpenFile(destinationPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if errorValue != nil {
		return errorValue
	}
	defer destination.Close()

	_, errorValue = io.Copy(destination, source)
	return errorValue
}

func clampDeviceBrowserFileTTL(ttlSeconds int) time.Duration {
	if ttlSeconds < 5*60 {
		ttlSeconds = 24 * 60 * 60
	}
	if ttlSeconds > 7*24*60*60 {
		ttlSeconds = 7 * 24 * 60 * 60
	}
	return time.Duration(ttlSeconds) * time.Second
}

func writeDeviceBrowserFileMetadata(metadata deviceBrowserFileMetadata) error {
	document, errorValue := json.MarshalIndent(metadata, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	return os.WriteFile(metadata.DevicePath+".internkim-meta.json", document, 0o600)
}

func randomFileID() string {
	buffer := make([]byte, 16)
	if _, errorValue := rand.Read(buffer); errorValue != nil {
		return "device-browser-file"
	}
	return hex.EncodeToString(buffer)
}
