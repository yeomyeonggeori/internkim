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

type calendarEventTarget struct {
	EventID string `json:"eventID"`
	Query   string `json:"query"`
	Title   string `json:"title"`
}

type calendarResolvedEventTarget struct {
	ID        string
	UpdatedAt string
	Event     calendarEventForTool
}

const calendarEventResolutionLimit = 2

func (service Service) resolveCalendarEventTarget(ctx context.Context, request capabilities.ToolInvokeRequest) (calendarResolvedEventTarget, *capabilities.ToolInvokeResponse, error) {
	target := decodeCalendarEventTarget(request.Input)
	if target.EventID == "" && target.Query == "" {
		return calendarResolvedEventTarget{}, nil, fmt.Errorf("eventID, query, or unchanged title is required")
	}
	if target.EventID != "" {
		return service.resolveCalendarEventTargetByID(ctx, request, target.EventID)
	}
	return service.resolveCalendarEventTargetByQuery(ctx, request, target.Query)
}

func decodeCalendarEventTarget(document json.RawMessage) calendarEventTarget {
	var target calendarEventTarget
	_ = json.Unmarshal(document, &target)
	target.EventID = strings.TrimSpace(target.EventID)
	target.Query = strings.TrimSpace(target.Query)
	target.Title = strings.TrimSpace(target.Title)
	if target.Query == "" {
		target.Query = target.Title
	}
	return target
}

func (service Service) resolveCalendarEventTargetByID(ctx context.Context, request capabilities.ToolInvokeRequest, eventID string) (calendarResolvedEventTarget, *capabilities.ToolInvokeResponse, error) {
	path := "/calendar/api/events/" + url.PathEscape(eventID)
	result, errorValue := service.sendCalendarToolRequest(ctx, http.MethodGet, path, nil, request.Context.RequesterEmail)
	if errorValue != nil {
		var requestError *calendarToolRequestError
		if errors.As(errorValue, &requestError) && requestError.statusCode == http.StatusNotFound {
			failure := calendarResolutionFailure(request.ToolName, "calendar_event_not_found", "no calendar event matched the query", nil)
			return calendarResolvedEventTarget{}, &failure, nil
		}
		return calendarResolvedEventTarget{}, nil, errorValue
	}
	var event calendarEventForTool
	if errorValue := json.Unmarshal(result, &event); errorValue != nil {
		return calendarResolvedEventTarget{}, nil, errorValue
	}
	target, errorValue := resolveCalendarEventResponse(event, eventID)
	if errorValue != nil {
		return calendarResolvedEventTarget{}, nil, errorValue
	}
	return target, nil, nil
}

func (service Service) resolveCalendarEventTargetByQuery(ctx context.Context, request capabilities.ToolInvokeRequest, query string) (calendarResolvedEventTarget, *capabilities.ToolInvokeResponse, error) {
	searchQuery := url.Values{}
	searchQuery.Set("query", query)
	result, errorValue := service.sendCalendarToolRequest(ctx, http.MethodGet, "/calendar/api/events/search?"+searchQuery.Encode(), nil, request.Context.RequesterEmail)
	isLegacyFallback := false
	if errorValue != nil {
		var requestError *calendarToolRequestError
		if !errors.As(errorValue, &requestError) || requestError.statusCode != http.StatusNotFound {
			return calendarResolvedEventTarget{}, nil, errorValue
		}
		result, errorValue = service.sendCalendarToolRequest(ctx, http.MethodGet, "/calendar/api/events", nil, request.Context.RequesterEmail)
		if errorValue != nil {
			return calendarResolvedEventTarget{}, nil, errorValue
		}
		isLegacyFallback = true
	}
	var response calendarEventsForTool
	if errorValue := json.Unmarshal(result, &response); errorValue != nil {
		return calendarResolvedEventTarget{}, nil, errorValue
	}
	matches := response.Events
	if isLegacyFallback {
		matches = filterCalendarEventSearchFallback(matches, query)
	}
	if len(matches) == 0 {
		failure := calendarResolutionFailure(request.ToolName, "calendar_event_not_found", "no calendar event matched the query", nil)
		return calendarResolvedEventTarget{}, &failure, nil
	}
	if len(matches) > 1 {
		failure := calendarResolutionFailure(request.ToolName, "calendar_event_ambiguous", "multiple calendar events matched the query", matches)
		return calendarResolvedEventTarget{}, &failure, nil
	}
	target, errorValue := resolveCalendarEventResponse(matches[0], "")
	if errorValue != nil {
		return calendarResolvedEventTarget{}, nil, errorValue
	}
	return target, nil, nil
}

func filterCalendarEventSearchFallback(events []calendarEventForTool, query string) []calendarEventForTool {
	matches := make([]calendarEventForTool, 0, calendarEventResolutionLimit)
	for _, event := range events {
		if calendarEventMatchesQuery(event, query) {
			matches = append(matches, event)
			if len(matches) == calendarEventResolutionLimit {
				break
			}
		}
	}
	return matches
}

func resolveCalendarEventResponse(event calendarEventForTool, expectedEventID string) (calendarResolvedEventTarget, error) {
	eventID := strings.TrimSpace(event.ID)
	if eventID == "" {
		return calendarResolvedEventTarget{}, fmt.Errorf("calendar event lookup returned an empty event ID")
	}
	if expectedEventID != "" && eventID != strings.TrimSpace(expectedEventID) {
		return calendarResolvedEventTarget{}, fmt.Errorf("calendar event lookup returned event %q for requested event %q", eventID, strings.TrimSpace(expectedEventID))
	}
	updatedAt := strings.TrimSpace(event.UpdatedAt)
	if _, errorValue := time.Parse(time.RFC3339Nano, updatedAt); errorValue != nil {
		return calendarResolvedEventTarget{}, fmt.Errorf("calendar event lookup returned invalid updatedAt for event %q: %w", eventID, errorValue)
	}
	event.ID = eventID
	event.UpdatedAt = updatedAt
	return calendarResolvedEventTarget{ID: eventID, UpdatedAt: updatedAt, Event: event}, nil
}

func calendarResolutionFailure(toolName string, errorCode string, message string, candidates []calendarEventForTool) capabilities.ToolInvokeResponse {
	result, _ := json.Marshal(map[string]any{"errorCode": errorCode, "message": message, "candidates": candidates})
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        toolName,
		Status:          "error",
		Content:         message,
		IsError:         true,
		Message:         message,
		ErrorCode:       errorCode,
		FailureStage:    "resolution",
		Result:          result,
	}
}
