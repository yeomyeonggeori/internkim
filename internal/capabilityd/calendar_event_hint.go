package capabilityd

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type calendarEventHintFailure struct {
	ErrorCode    string                       `json:"errorCode"`
	FailureStage string                       `json:"failureStage"`
	Message      string                       `json:"message"`
	Candidates   []calendarEventHintCandidate `json:"candidates,omitempty"`
	Retryable    bool                         `json:"retryable"`
	SafeRetry    bool                         `json:"safeRetry"`
}

type calendarEventHintCandidate struct {
	EventID  string `json:"eventID"`
	Title    string `json:"title"`
	StartsAt string `json:"startsAt,omitempty"`
}

func (service Service) fetchCalendarAllEvents(ctx context.Context, requesterEmail string) ([]calendarEventForTool, error) {
	result, errorValue := service.sendCalendarToolRequest(ctx, http.MethodGet, "/calendar/api/events", nil, requesterEmail)
	if errorValue != nil {
		return nil, errorValue
	}
	events, errorValue := normalizeCalendarEventsResult(result)
	if errorValue != nil {
		return nil, errorValue
	}
	return events.Events, nil
}

func (service Service) resolveCalendarEventHintTarget(ctx context.Context, request capabilities.ToolInvokeRequest, eventHint string) (string, *calendarEventHintFailure, error) {
	events, errorValue := service.fetchCalendarAllEvents(ctx, request.Context.RequesterEmail)
	if errorValue != nil {
		return "", nil, errorValue
	}
	eventID, failure := resolveCalendarEventHint(eventHint, request.Context.RequesterEmail, events)
	return eventID, failure, nil
}

func (event calendarEventForTool) hintID() string { return event.EventID }

func (event calendarEventForTool) hintTitle() string { return event.Title }

func resolveCalendarEventHint(eventHint string, requesterEmail string, events []calendarEventForTool) (string, *calendarEventHintFailure) {
	resolution := resolveHint(eventHint, events, calendarEventParticipation(requesterEmail))
	if resolution.IsResolved {
		return resolution.Match.EventID, nil
	}
	failure := calendarEventHintUnresolvedFailure(resolution)
	return "", &failure
}

func calendarEventParticipation(requesterEmail string) func(calendarEventForTool) bool {
	normalizedRequesterEmail := strings.ToLower(strings.TrimSpace(requesterEmail))
	if normalizedRequesterEmail == "" {
		return nil
	}
	return func(event calendarEventForTool) bool {
		for _, participant := range event.Participants {
			if strings.ToLower(strings.TrimSpace(participant.Email)) == normalizedRequesterEmail {
				return true
			}
		}
		return false
	}
}

func calendarEventHintUnresolvedFailure(resolution hintResolution[calendarEventForTool]) calendarEventHintFailure {
	return calendarEventHintFailure{
		ErrorCode:    "calendar_event_hint_unresolved",
		FailureStage: "target_resolution",
		Message:      unresolvedHintMessage("calendar event", "eventHint", "eventID", resolution.IsAmbiguous),
		Candidates:   calendarEventHintCandidates(resolution.Candidates),
		Retryable:    true,
		SafeRetry:    true,
	}
}

func calendarEventHintCandidates(events []calendarEventForTool) []calendarEventHintCandidate {
	candidates := make([]calendarEventHintCandidate, 0, len(events))
	for _, event := range events {
		candidates = append(candidates, calendarEventHintCandidate{EventID: event.EventID, Title: event.Title, StartsAt: event.StartISO})
	}
	return candidates
}

func calendarEventHintFailureResponse(toolName string, failure calendarEventHintFailure) capabilities.ToolInvokeResponse {
	result, _ := json.Marshal(failure)
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        toolName,
		Outcome:         capabilities.ToolOutcomeFailed,
		Status:          "error",
		Content:         failure.Message,
		IsError:         true,
		Message:         failure.Message,
		ErrorCode:       failure.ErrorCode,
		FailureStage:    failure.FailureStage,
		Retryable:       failure.Retryable,
		SafeRetry:       failure.SafeRetry,
		Result:          result,
	}
}
