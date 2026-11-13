package capabilityd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
	"github.com/yeomyeonggeori/internkim/internal/desktop"
)

const (
	desktopReadToolName = "desktop_read"
	desktopActToolName  = "desktop_act"
)

var desktopKindByToolName = map[string]desktop.Kind{
	desktopReadToolName: desktop.KindRead,
	desktopActToolName:  desktop.KindAct,
}

type desktopToolInput struct {
	Program   string   `json:"program"`
	Arguments []string `json:"arguments"`
	Text      string   `json:"text"`
}

type desktopToolResult struct {
	Output      string                     `json:"output"`
	Attachments []workspaceImageAttachment `json:"attachments,omitempty"`
}

const desktopUnreachableMessage = "desktop_unreachable: no computer of the requester's is taking desktop work right now. " +
	"A computer takes it only while the internkim desktop app is open there, signed in as the requester, with \"Let internkim use this computer\" turned on, " +
	"and no other person's computer and no company computer can take it in their place."

func (service Service) invokeDesktopTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	owner := strings.ToLower(strings.TrimSpace(request.Context.RequesterEmail))
	if owner == "" {
		return desktopFailedResponse(request.ToolName, "a desktop call needs a requester, because it runs on the requester's own computer"), nil
	}
	var input desktopToolInput
	if errorValue := json.Unmarshal(request.Input, &input); errorValue != nil {
		return desktopFailedResponse(request.ToolName, "desktop tool input is not valid json: "+errorValue.Error()), nil
	}
	call, errorValue := desktop.NewCall(desktopKindByToolName[request.ToolName], input.Program, input.Arguments, input.Text)
	if errorValue != nil {
		return desktopFailedResponse(request.ToolName, errorValue.Error()), nil
	}
	answer, errorValue := service.runOnTheDesktopOf(ctx, owner, call)
	if errors.Is(errorValue, errNoDesktopTookTheWork) {
		return desktopFailedResponse(request.ToolName, desktopUnreachableMessage), nil
	}
	if errors.Is(errorValue, errDesktopUnavailableOnThisHost) {
		return desktopFailedResponse(request.ToolName, errorValue.Error()), nil
	}
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return desktopResponseOf(request.ToolName, answer, time.Now())
}

func (service Service) runOnTheDesktopOf(ctx context.Context, owner string, call desktop.Call) (desktop.Answer, error) {
	document, errorValue := service.handDesktopWorkToTheRelay(ctx, desktopWork{RequestID: newWorkID(), DesktopOwner: owner, Work: call})
	if errorValue != nil {
		return desktop.Answer{}, errorValue
	}
	var answer desktop.Answer
	if errorValue := json.Unmarshal(document, &answer); errorValue != nil {
		return desktop.Answer{}, errors.New("the desktop answered something other than an output or a refusal: " + errorValue.Error())
	}
	return answer, nil
}

func desktopResponseOf(toolName string, answer desktop.Answer, capturedAt time.Time) (capabilities.ToolInvokeResponse, error) {
	if answer.Refusal != "" {
		return desktopFailedResponse(toolName, "the requester's computer refused this call: "+answer.Refusal), nil
	}
	envelope, errorValue := desktop.ReadEnvelope(answer.Output)
	if errorValue != nil {
		return desktopFailedResponse(toolName, errorValue.Error()), nil
	}
	if !envelope.Succeeded() {
		return desktopFailedResponse(toolName, strings.TrimSpace(answer.Output)), nil
	}
	result := desktopResultOf(answer.Output, capturedAt)
	document, errorValue := json.Marshal(result)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilitySuccessResponseFrom(toolName, "ok", document, capabilityResponseOrigin{
		Provider:        "desktop",
		SelectedBackend: capabilities.LLMBackendDevice,
		Content:         result.Output,
	})
}

func desktopResultOf(output string, capturedAt time.Time) desktopToolResult {
	screenshot, isScreenshot := desktop.ScreenshotOf(output)
	if !isScreenshot {
		return desktopToolResult{Output: strings.TrimSpace(output)}
	}
	picture, errorValue := base64.StdEncoding.DecodeString(screenshot.PNGBase64)
	if errorValue != nil {
		return desktopToolResult{Output: strings.TrimSpace(output)}
	}
	return desktopToolResult{
		Output: screenshot.Remainder,
		Attachments: []workspaceImageAttachment{{
			Filename:      "desktop-" + capturedAt.UTC().Format("20060102T150405Z") + ".png",
			ContentType:   "image/png",
			SizeBytes:     int64(len(picture)),
			ContentBase64: screenshot.PNGBase64,
		}},
	}
}

func desktopFailedResponse(toolName string, message string) capabilities.ToolInvokeResponse {
	return capabilities.ToolInvokeResponse{
		Provider:        "desktop",
		SelectedBackend: capabilities.LLMBackendDevice,
		ToolName:        toolName,
		Outcome:         capabilities.ToolOutcomeFailed,
		Status:          "error",
		Content:         message,
		IsError:         true,
	}
}
