package capabilityd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type calendarResolvedEventTarget struct {
	EventID   string
	UpdatedAt string
	Event     calendarEventForTool
}

func (service Service) resolveCalendarEventTarget(ctx context.Context, request capabilities.ToolInvokeRequest, eventID string) (calendarResolvedEventTarget, *capabilities.ToolInvokeResponse, error) {
	return service.resolveCalendarEventTargetByID(ctx, request, strings.TrimSpace(eventID))
}

func (service Service) resolveCalendarEventTargetByID(ctx context.Context, request capabilities.ToolInvokeRequest, eventID string) (calendarResolvedEventTarget, *capabilities.ToolInvokeResponse, error) {
	path := "/calendar/api/events/" + url.PathEscape(eventID)
	result, errorValue := service.sendCalendarToolRequest(ctx, http.MethodGet, path, nil, request.Context.RequesterEmail)
	if errorValue != nil {
		var requestError *calendarToolRequestError
		if errors.As(errorValue, &requestError) && requestError.statusCode == http.StatusNotFound {
			failure := calendarResolutionFailure(request.ToolName, "calendar_event_not_found", "calendar event was not found")
			return calendarResolvedEventTarget{}, &failure, nil
		}
		return calendarResolvedEventTarget{}, nil, errorValue
	}
	target, errorValue := resolveCalendarEventResponse(result, eventID)
	if errorValue != nil {
		return calendarResolvedEventTarget{}, nil, errorValue
	}
	return target, nil, nil
}

func resolveCalendarEventResponse(result json.RawMessage, expectedEventID string) (calendarResolvedEventTarget, error) {
	_, event, errorValue := normalizeCalendarEventResult(result, expectedEventID)
	if errorValue != nil {
		return calendarResolvedEventTarget{}, errorValue
	}
	updatedAt := strings.TrimSpace(event.UpdatedAt)
	if _, errorValue := time.Parse(time.RFC3339Nano, updatedAt); errorValue != nil {
		return calendarResolvedEventTarget{}, fmt.Errorf("calendar event lookup returned invalid updatedAt for event %q: %w", event.EventID, errorValue)
	}
	event.UpdatedAt = updatedAt
	return calendarResolvedEventTarget{EventID: event.EventID, UpdatedAt: updatedAt, Event: event}, nil
}

func calendarResolutionFailure(toolName string, errorCode string, message string) capabilities.ToolInvokeResponse {
	result, _ := json.Marshal(map[string]string{"errorCode": errorCode, "message": message})
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        toolName,
		Outcome:         capabilities.ToolOutcomeFailed,
		Status:          "error",
		Content:         message,
		IsError:         true,
		Message:         message,
		ErrorCode:       errorCode,
		FailureStage:    "resolution",
		Result:          result,
	}
}
