package capabilityd

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	capabilityschema "gitlab.com/eastriver/internkim/pkg/capabilityprotocol/jsonschema"
)

type capabilityToolTarget struct {
	InputField string `json:"inputField,omitempty"`
	ID         string `json:"id,omitempty"`
	Title      string `json:"title,omitempty"`
	StartsAt   string `json:"startsAt,omitempty"`
}

type capabilityToolTargetRoute struct {
	ToolName string
	Resolver capabilityToolHandler
}

var capabilityToolTargetRoutes = []capabilityToolTargetRoute{
	{ToolName: "event_delete", Resolver: Service.resolveCalendarEventDeleteTarget},
	{ToolName: "task_delete", Resolver: Service.resolveFlowTaskDeleteTarget},
}

func capabilityToolTargetRouteFor(toolName string) (capabilityToolTargetRoute, bool) {
	trimmedToolName := strings.TrimSpace(toolName)
	for _, route := range capabilityToolTargetRoutes {
		if route.ToolName == trimmedToolName {
			return route, true
		}
	}
	return capabilityToolTargetRoute{}, false
}

func (service Service) resolveCapabilityToolTarget(ctx context.Context, toolName string, reader io.Reader) (capabilities.ToolInvokeResponse, error) {
	request, errorValue := decodeToolInvokeRequest(toolName, reader)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	descriptor, hasDescriptor := capabilityToolDescriptorFor(request.ToolName)
	targetRoute, hasTargetRoute := capabilityToolTargetRouteFor(request.ToolName)
	if !hasDescriptor || !hasTargetRoute {
		return capabilityToolWithoutTargetResponse(request.ToolName), nil
	}
	if errorValue := capabilityschema.Validate(descriptor.InputSchema, request.Input); errorValue != nil {
		return capabilityInvalidInputResponse(request.ToolName, errorValue), nil
	}
	return targetRoute.Resolver(service, ctx, request)
}

func (service Service) resolveCalendarEventDeleteTarget(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeCalendarEventDeleteInput(request.Input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	event, hintFailure, errorValue := service.resolveCalendarEventHintTarget(ctx, request, input.EventHint)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if hintFailure != nil {
		return calendarEventHintFailureResponse(request.ToolName, *hintFailure), nil
	}
	return capabilityToolTargetResponse(request.ToolName, capabilityToolTarget{
		InputField: "eventHint",
		ID:         event.EventID,
		Title:      event.Title,
		StartsAt:   event.StartsAt,
	}), nil
}

func (service Service) resolveFlowTaskDeleteTarget(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeFlowTaskDeleteInput(request.Input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	summary, errorValue := service.fetchFlowAllTasks(ctx, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	task, hintFailure := resolveFlowTaskHint(input.TaskHint, service.requesterFlowOwnerID(ctx, request.Context.RequesterEmail, summary.Members), summary.Tasks)
	if hintFailure != nil {
		return flowFailureResponse(request.ToolName, *hintFailure), nil
	}
	return capabilityToolTargetResponse(request.ToolName, capabilityToolTarget{
		InputField: "taskHint",
		ID:         task.ID,
		Title:      task.Content,
		StartsAt:   task.StartDate,
	}), nil
}

func capabilityToolTargetResponse(toolName string, target capabilityToolTarget) capabilities.ToolInvokeResponse {
	result, _ := json.Marshal(target)
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        strings.TrimSpace(toolName),
		Outcome:         capabilities.ToolOutcomeSucceeded,
		Status:          "resolved",
		Content:         target.Title,
		Result:          result,
	}
}

func capabilityToolWithoutTargetResponse(toolName string) capabilities.ToolInvokeResponse {
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        strings.TrimSpace(toolName),
		Outcome:         capabilities.ToolOutcomeSucceeded,
		Status:          "no_target",
		Result:          json.RawMessage(`{}`),
	}
}
