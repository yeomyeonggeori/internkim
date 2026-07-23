package capabilityd

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

const calendarEventHintCandidateLimit = 20

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

func resolveCalendarEventHint(eventHint string, requesterEmail string, events []calendarEventForTool) (string, *calendarEventHintFailure) {
	trimmedHint := strings.TrimSpace(eventHint)
	if event, found := findCalendarEventByID(trimmedHint, events); found {
		return event.EventID, nil
	}
	titleMatches := findCalendarEventsByTitle(trimmedHint, events)
	if len(titleMatches) == 1 {
		return titleMatches[0].EventID, nil
	}
	if participatingMatch, isUnique := uniqueParticipatingCalendarEvent(titleMatches, requesterEmail); isUnique {
		return participatingMatch.EventID, nil
	}
	failure := calendarEventHintUnresolvedFailure(events)
	return "", &failure
}

func uniqueParticipatingCalendarEvent(events []calendarEventForTool, requesterEmail string) (calendarEventForTool, bool) {
	requesterEmail = strings.ToLower(strings.TrimSpace(requesterEmail))
	if requesterEmail == "" {
		return calendarEventForTool{}, false
	}
	participatingMatches := make([]calendarEventForTool, 0, 1)
	for _, event := range events {
		for _, participant := range event.Participants {
			if strings.ToLower(strings.TrimSpace(participant.Email)) == requesterEmail {
				participatingMatches = append(participatingMatches, event)
				break
			}
		}
	}
	if len(participatingMatches) != 1 {
		return calendarEventForTool{}, false
	}
	return participatingMatches[0], true
}

func findCalendarEventByID(eventID string, events []calendarEventForTool) (calendarEventForTool, bool) {
	for _, event := range events {
		if event.EventID == eventID {
			return event, true
		}
	}
	return calendarEventForTool{}, false
}

func findCalendarEventsByTitle(title string, events []calendarEventForTool) []calendarEventForTool {
	matches := make([]calendarEventForTool, 0, 1)
	for _, event := range events {
		if event.Title == title {
			matches = append(matches, event)
		}
	}
	return matches
}

func calendarEventHintUnresolvedFailure(events []calendarEventForTool) calendarEventHintFailure {
	return calendarEventHintFailure{
		ErrorCode:    "calendar_event_hint_unresolved",
		FailureStage: "target_resolution",
		Message:      "eventHint did not uniquely resolve to a calendar event; retry with the exact eventID or exact title from one of the candidates",
		Candidates:   calendarEventHintCandidates(events),
		Retryable:    true,
		SafeRetry:    true,
	}
}

func calendarEventHintCandidates(events []calendarEventForTool) []calendarEventHintCandidate {
	limit := calendarEventHintCandidateLimit
	if len(events) < limit {
		limit = len(events)
	}
	candidates := make([]calendarEventHintCandidate, 0, limit)
	for _, event := range events[:limit] {
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
