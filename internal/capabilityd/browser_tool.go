package capabilityd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"unicode"

	browserruntime "github.com/yeomyeonggeori/internkim/internal/browser"
	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

func (service Service) invokeDeviceBrowserTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	browserRuntime, errorValue := service.deviceBrowserRuntime(ctx, request.Context)
	if errors.Is(errorValue, browserruntime.ErrDeviceBrowsersFull) {
		return service.deviceBrowsersFullResponse(request.ToolName), nil
	}
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	var result any

	switch request.ToolName {
	case "browser_open":
		var input browserruntime.NavigateRequest
		errorValue = decodeBrowserToolInput(request.Input, &input)
		if errorValue == nil {
			result, errorValue = browserRuntime.Navigate(ctx, input)
		}
	case "browser_snapshot":
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
	case "browser_screenshot":
		screenshot, screenshotError := browserRuntime.Screenshot(ctx, browserruntime.ScreenshotRequest{})
		result, errorValue = browserScreenshotResultOf(screenshot), screenshotError
	case "browser_click":
		var input browserruntime.ClickRequest
		errorValue = decodeBrowserToolInput(request.Input, &input)
		if errorValue == nil {
			result, errorValue = browserRuntime.Click(ctx, input)
		}
	case "browser_fill":
		var input browserruntime.FillRequest
		errorValue = decodeBrowserToolInput(request.Input, &input)
		if errorValue == nil {
			result, errorValue = browserRuntime.Fill(ctx, input)
		}
	case "browser_select":
		var input browserruntime.SelectRequest
		errorValue = decodeBrowserToolInput(request.Input, &input)
		if errorValue == nil {
			result, errorValue = browserRuntime.Select(ctx, input)
		}
	case "browser_press":
		var input browserruntime.PressRequest
		errorValue = decodeBrowserToolInput(request.Input, &input)
		if errorValue == nil {
			result, errorValue = browserRuntime.Press(ctx, input)
		}
	case "browser_wait":
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
	return capabilitySuccessResponseFrom(request.ToolName, "ok", document, capabilityResponseOrigin{
		Provider:        "device",
		SelectedBackend: capabilities.LLMBackendDevice,
	})
}

func (service Service) deviceBrowserRuntime(ctx context.Context, requestContext capabilities.ToolInvokeContext) (browserruntime.AgentBrowserRuntime, error) {
	if service.DeviceBrowsers == nil {
		return browserruntime.AgentBrowserRuntime{}, errors.New("capabilityd runs no device browsers")
	}
	browser, errorValue := service.DeviceBrowsers.BrowserFor(ctx, requestContext.RequesterEmail)
	if errorValue != nil {
		return browserruntime.AgentBrowserRuntime{}, errorValue
	}
	return browserruntime.AgentBrowserRuntime{
		CommandPath: service.Configuration.WithDefaults().AgentBrowserPath,
		Engine:      browserruntime.BrowserEngineMoli,
		CDPURL:      browser.DevtoolsURL,
		SessionName: browser.SessionName,
		Runner:      service.browserCommandRunner(),
	}, nil
}

func (configuration Configuration) newDeviceBrowsers() *browserruntime.DeviceBrowsers {
	return browserruntime.NewDeviceBrowsers(browserruntime.DeviceBrowserSettings{
		ExecutablePath: configuration.DeviceBrowserExecutablePath,
		StateDirectory: configuration.DeviceBrowserStateDirectory,
		FirstPort:      configuration.DeviceBrowserFirstPort,
		Capacity:       configuration.DeviceBrowserCapacity,
		UserName:       configuration.DeviceBrowserUserName,
	})
}

func (service Service) deviceBrowsersFullResponse(toolName string) capabilities.ToolInvokeResponse {
	capacity := service.Configuration.WithDefaults().DeviceBrowserCapacity
	return deviceBrowserFailedResponse(toolName, fmt.Sprintf(
		"all %d device browsers are in use by other people right now; tell the requester the browser is busy and try again in a few minutes", capacity))
}

func deviceBrowserFailedResponse(toolName string, message string) capabilities.ToolInvokeResponse {
	return capabilities.ToolInvokeResponse{
		Provider:        "device",
		SelectedBackend: capabilities.LLMBackendDevice,
		ToolName:        toolName,
		Outcome:         capabilities.ToolOutcomeFailed,
		Status:          "error",
		Content:         message,
		IsError:         true,
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

func captchaBlockedResponse(toolName string, snapshotText string) (capabilities.ToolInvokeResponse, bool) {
	isBlocked, matchedSignature := browserruntime.IsCaptchaBlocked(snapshotText)
	if !isBlocked {
		return capabilities.ToolInvokeResponse{}, false
	}
	log.Printf("tool.browser.blocked_by_captcha: tool=%s signature=%q", toolName, matchedSignature)
	message := fmt.Sprintf(
		"blocked_by_captcha: this URL returned a bot-detection wall (matched: %q). "+
			"Do NOT pretend you have the information from this page. "+
			"Tell the person who asked that this page is behind a bot check you cannot pass, then stop.",
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

type browserScreenshotResult struct {
	OK          bool                       `json:"ok"`
	Action      string                     `json:"action"`
	Attachments []workspaceImageAttachment `json:"attachments"`
	CapturedAt  string                     `json:"capturedAt"`
}

func browserScreenshotResultOf(screenshot browserruntime.ScreenshotResult) browserScreenshotResult {
	return browserScreenshotResult{
		OK:     true,
		Action: "screenshot",
		Attachments: []workspaceImageAttachment{{
			DevicePath:    screenshot.Filename,
			Filename:      screenshot.Filename,
			ContentType:   screenshot.ContentType,
			SizeBytes:     screenshot.SizeBytes,
			ContentBase64: base64.StdEncoding.EncodeToString(screenshot.Content),
		}},
		CapturedAt: screenshot.CapturedAt,
	}
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
