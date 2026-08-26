package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

const calendarEventVersionConflictCode = "calendar_event_version_conflict"

type calendarToolRequestError struct {
	statusCode int
	errorCode  string
	message    string
}

func (errorValue *calendarToolRequestError) Error() string {
	return errorValue.message
}

type calendarEventWriteInput struct {
	EventID                   string
	Title                     string
	Note                      string
	Location                  string
	StartsAt                  string
	EndsAt                    string
	IsWholeDay                bool
	EveryoneAttends           bool
	People                    calendarToolPeopleInput
	Participants              []calendarToolParticipant
	IsRequestedOfSomebodyElse bool
	NotifyMinutesBefore       int
	AllowDuplicate            bool
	IncludeRequester          *bool
	ExpectedUpdatedAt         string
}

// An event is a task the company marked as one, so it is written with the task's
// own field names.
type calendarEventAddInput struct {
	Title                  string                  `json:"title"`
	Note                   string                  `json:"note"`
	Location               string                  `json:"location"`
	StartsAt               string                  `json:"startsAt"`
	EndsAt                 string                  `json:"endsAt"`
	IsWholeDay             bool                    `json:"isWholeDay"`
	ParticipantPersonHints calendarToolPeopleInput `json:"participantPersonHints"`
	EveryoneAttends        bool                    `json:"everyoneAttends"`
	NotifyMinutesBefore    *int                    `json:"notifyMinutesBefore"`
}

type calendarEventListInput struct {
	StartsAt string   `json:"startsAt"`
	EndsAt   string   `json:"endsAt"`
	WeekFrom *int     `json:"weekFrom"`
	WeekTo   *int     `json:"weekTo"`
	Query    string   `json:"query"`
	Limit    *float64 `json:"limit"`
}

type calendarEventUpdateInput struct {
	EventHint              string                   `json:"eventHint"`
	Title                  *string                  `json:"title"`
	Note                   *string                  `json:"note"`
	Location               *string                  `json:"location"`
	StartsAt               *string                  `json:"startsAt"`
	EndsAt                 *string                  `json:"endsAt"`
	IsWholeDay             *bool                    `json:"isWholeDay"`
	ParticipantPersonHints *calendarToolPeopleInput `json:"participantPersonHints"`
	EveryoneAttends        *bool                    `json:"everyoneAttends"`
	NotifyMinutesBefore    *int                     `json:"notifyMinutesBefore"`
}

type calendarEventDeleteInput struct {
	EventHint string `json:"eventHint"`
}

type calendarToolPeopleInput []string

type calendarToolParticipant struct {
	PersonID string `json:"personID,omitempty"`
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"`
}

type calendarEventsForTool struct {
	Events []calendarEventForTool `json:"events"`
}

type calendarEventForTool struct {
	EventID             string                    `json:"eventID"`
	Title               string                    `json:"title"`
	Note                string                    `json:"note"`
	Location            string                    `json:"location"`
	StartsAt            string                    `json:"startsAt"`
	EndsAt              string                    `json:"endsAt"`
	IsWholeDay          bool                      `json:"isWholeDay"`
	People              calendarToolPeopleInput   `json:"-"`
	TimeZone            string                    `json:"-"`
	Participants        []calendarToolParticipant `json:"participants"`
	NotifyMinutesBefore int                       `json:"notifyMinutesBefore,omitempty"`
	UpdatedAt           string                    `json:"updatedAt"`
}

func (service Service) invokeCalendarTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	switch strings.TrimSpace(request.ToolName) {
	case "event_add":
		return service.invokeCalendarEventAdd(ctx, request)
	case "event_list":
		return service.invokeCalendarEventList(ctx, request)
	case "event_update":
		return service.invokeCalendarEventUpdate(ctx, request)
	case "event_delete":
		return service.invokeCalendarEventDelete(ctx, request)
	default:
		return capabilities.ToolInvokeResponse{}, fmt.Errorf("calendar tool is not configured: %s", request.ToolName)
	}
}

func (service Service) invokeCalendarEventAdd(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeCalendarEventWriteInput(request.Input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if errorValue := applyCalendarConflictResolution(&input, request.Context.ConflictResolution); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	input, failure, hasFailure := service.prepareCalendarEventWriteInput(ctx, input, request.Context, true)
	if hasFailure {
		return calendarToolPersonResolveErrorResponse(request.ToolName, failure), nil
	}
	result, errorValue := service.sendCalendarToolRequest(ctx, http.MethodPost, "/calendar/api/events", calendarEventWritePayload(input), request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if isCalendarDuplicateCandidateResult(result) {
		return calendarToolDuplicateCandidateResponse(request.ToolName, result), nil
	}
	normalizedResult, _, errorValue := normalizeCalendarEventResult(result, "")
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilitySuccessResponse(request.ToolName, "created", normalizedResult)
}

func (service Service) invokeCalendarEventList(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeCalendarEventListInput(request.Input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	result, errorValue := service.sendCalendarToolRequest(ctx, http.MethodGet, calendarEventListPath(input), nil, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	events, errorValue := normalizeCalendarEventsResult(result)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	filteredResult, errorValue := filterCalendarEventsForTool(events, input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilitySuccessResponse(request.ToolName, "ok", filteredResult)
}

func (service Service) invokeCalendarEventUpdate(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	updateInput, errorValue := decodeCalendarEventUpdateInput(request.Input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	hintedEvent, hintFailure, errorValue := service.resolveCalendarEventHintTarget(ctx, request, updateInput.EventHint)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if hintFailure != nil {
		return calendarEventHintFailureResponse(request.ToolName, *hintFailure), nil
	}
	target, failure, errorValue := service.resolveCalendarEventTarget(ctx, request, hintedEvent.EventID)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if failure != nil {
		return *failure, nil
	}
	input := mergeCalendarEventUpdateInput(updateInput, target.Event)
	input, personFailure, hasFailure := service.prepareCalendarEventWriteInput(ctx, input, request.Context, false)
	if hasFailure {
		return calendarToolPersonResolveErrorResponse(request.ToolName, personFailure), nil
	}
	input.ExpectedUpdatedAt = target.UpdatedAt
	path := "/calendar/api/events/" + url.PathEscape(input.EventID)
	result, errorValue := service.sendCalendarToolRequest(ctx, http.MethodPut, path, calendarEventWritePayload(input), request.Context.RequesterEmail)
	if errorValue != nil {
		if response, isVersionConflict := calendarToolVersionConflictResponse(request.ToolName, errorValue); isVersionConflict {
			return response, nil
		}
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	normalizedResult, _, errorValue := normalizeCalendarEventResult(result, target.EventID)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilitySuccessResponse(request.ToolName, "updated", normalizedResult)
}

func (service Service) invokeCalendarEventDelete(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeCalendarEventDeleteInput(request.Input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	hintedEvent, hintFailure, errorValue := service.resolveCalendarEventHintTarget(ctx, request, input.EventHint)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if hintFailure != nil {
		return calendarEventHintFailureResponse(request.ToolName, *hintFailure), nil
	}
	target, failure, errorValue := service.resolveCalendarEventTarget(ctx, request, hintedEvent.EventID)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if failure != nil {
		return *failure, nil
	}
	path := "/calendar/api/events/" + url.PathEscape(target.EventID)
	payload := map[string]any{"expectedUpdatedAt": target.UpdatedAt}
	if _, errorValue := service.sendCalendarToolRequest(ctx, http.MethodDelete, path, payload, request.Context.RequesterEmail); errorValue != nil {
		if response, isVersionConflict := calendarToolVersionConflictResponse(request.ToolName, errorValue); isVersionConflict {
			return response, nil
		}
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	result, _ := json.Marshal(map[string]any{"eventID": target.EventID, "deleted": true})
	return capabilitySuccessResponse(request.ToolName, "deleted", result)
}

func decodeCalendarEventWriteInput(document json.RawMessage) (calendarEventWriteInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return calendarEventWriteInput{}, fmt.Errorf("calendar event input is required")
	}
	var externalInput calendarEventAddInput
	if errorValue := decodeStrictCalendarToolInput(document, &externalInput); errorValue != nil {
		return calendarEventWriteInput{}, errorValue
	}
	input := calendarEventWriteInput{
		Title:           strings.TrimSpace(externalInput.Title),
		Note:            strings.TrimSpace(externalInput.Note),
		Location:        strings.TrimSpace(externalInput.Location),
		StartsAt:        strings.TrimSpace(externalInput.StartsAt),
		EndsAt:          strings.TrimSpace(externalInput.EndsAt),
		IsWholeDay:      externalInput.IsWholeDay,
		EveryoneAttends: externalInput.EveryoneAttends,
		People:          normalizeCalendarToolPeople([]string(externalInput.ParticipantPersonHints)),
	}
	if externalInput.NotifyMinutesBefore != nil {
		if *externalInput.NotifyMinutesBefore <= 0 {
			return calendarEventWriteInput{}, fmt.Errorf("notifyMinutesBefore must be a positive number of minutes")
		}
		input.NotifyMinutesBefore = *externalInput.NotifyMinutesBefore
	}
	if input.Title == "" {
		return calendarEventWriteInput{}, fmt.Errorf("title is required")
	}
	if input.StartsAt == "" || input.EndsAt == "" {
		return calendarEventWriteInput{}, fmt.Errorf("startsAt and endsAt are required")
	}
	return input, nil
}

func applyCalendarConflictResolution(input *calendarEventWriteInput, resolution capabilities.ToolConflictResolution) error {
	switch resolution {
	case "":
		return nil
	case capabilities.ToolConflictResolutionAllowDuplicate:
		input.AllowDuplicate = true
		return nil
	default:
		return fmt.Errorf("calendar conflict resolution %q is not supported", resolution)
	}
}

func normalizeCalendarToolPeople(values []string) []string {
	people := []string{}
	seenPeople := map[string]bool{}
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue == "" {
			continue
		}
		normalizedValue := strings.ToLower(trimmedValue)
		if seenPeople[normalizedValue] {
			continue
		}
		seenPeople[normalizedValue] = true
		people = append(people, trimmedValue)
	}
	return people
}

func normalizeCalendarToolParticipants(values []calendarToolParticipant) []calendarToolParticipant {
	participants := []calendarToolParticipant{}
	seenParticipants := map[string]bool{}
	for _, value := range values {
		participant := calendarToolParticipant{
			PersonID: strings.TrimSpace(value.PersonID),
			Name:     strings.TrimSpace(value.Name),
			Email:    strings.ToLower(strings.TrimSpace(value.Email)),
		}
		key := calendarToolParticipantKey(participant)
		if key == "" || seenParticipants[key] {
			continue
		}
		seenParticipants[key] = true
		participants = append(participants, participant)
	}
	return participants
}

func calendarToolParticipantKey(participant calendarToolParticipant) string {
	if strings.TrimSpace(participant.PersonID) != "" {
		return "personID:" + strings.ToLower(strings.TrimSpace(participant.PersonID))
	}
	if strings.TrimSpace(participant.Email) != "" {
		return "email:" + strings.ToLower(strings.TrimSpace(participant.Email))
	}
	if strings.TrimSpace(participant.Name) != "" {
		return "name:" + strings.ToLower(strings.TrimSpace(participant.Name))
	}
	return ""
}

func calendarToolReminderLeadHours(value *int) (int, error) {
	if value == nil {
		return 24, nil
	}
	switch *value {
	case 1, 2, 3, 6, 12, 24, 48:
		return *value, nil
	default:
		return 0, fmt.Errorf("reminderLeadHours is not allowed")
	}
}

func decodeCalendarEventListInput(document json.RawMessage) (calendarEventListInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return calendarEventListInput{}, nil
	}
	var input calendarEventListInput
	if errorValue := decodeStrictCalendarToolInput(document, &input); errorValue != nil {
		return calendarEventListInput{}, errorValue
	}
	input.StartsAt = strings.TrimSpace(input.StartsAt)
	input.EndsAt = strings.TrimSpace(input.EndsAt)
	input.Query = strings.TrimSpace(input.Query)
	if input.Limit != nil && (*input.Limit <= 0 || math.Trunc(*input.Limit) != *input.Limit) {
		return calendarEventListInput{}, fmt.Errorf("limit must be a positive whole number")
	}
	if input.WeekFrom != nil || input.WeekTo != nil {
		input.StartsAt, input.EndsAt = calendarWeekWindow(input.WeekFrom, input.WeekTo, time.Now())
	}
	input.StartsAt, input.EndsAt = completeCalendarListRange(input.StartsAt, input.EndsAt)
	return input, nil
}

// A week said as an offset is the same window the task board answers, so asking
// for last week means the same seven days whichever of the two is asked.
func calendarWeekWindow(weekFrom *int, weekTo *int, now time.Time) (string, string) {
	firstOffset := 0
	if weekFrom != nil {
		firstOffset = *weekFrom
	}
	lastOffset := firstOffset
	if weekTo != nil {
		lastOffset = *weekTo
	}
	if lastOffset < firstOffset {
		firstOffset, lastOffset = lastOffset, firstOffset
	}
	monday := startOfWeekFor(now)
	start := monday.AddDate(0, 0, 7*firstOffset)
	end := monday.AddDate(0, 0, 7*(lastOffset+1))
	return start.Format(time.RFC3339), end.Format(time.RFC3339)
}

func startOfWeekFor(now time.Time) time.Time {
	daysSinceMonday := (int(now.Weekday()) + 6) % 7
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return midnight.AddDate(0, 0, -daysSinceMonday)
}

func completeCalendarListRange(startISO string, endISO string) (string, string) {
	if startISO != "" && endISO == "" {
		if startTime, errorValue := time.Parse(time.RFC3339, startISO); errorValue == nil {
			return startISO, startTime.Add(24 * time.Hour).Format(time.RFC3339)
		}
	}
	if startISO == "" && endISO != "" {
		if endTime, errorValue := time.Parse(time.RFC3339, endISO); errorValue == nil {
			return endTime.Add(-24 * time.Hour).Format(time.RFC3339), endISO
		}
	}
	return startISO, endISO
}

func decodeCalendarEventUpdateInput(document json.RawMessage) (calendarEventUpdateInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return calendarEventUpdateInput{}, fmt.Errorf("event_update input is required")
	}
	var input calendarEventUpdateInput
	if errorValue := decodeStrictCalendarToolInput(document, &input); errorValue != nil {
		return calendarEventUpdateInput{}, errorValue
	}
	input.EventHint = strings.TrimSpace(input.EventHint)
	trimStringPointer(&input.Title)
	trimStringPointer(&input.Note)
	trimStringPointer(&input.Location)
	trimStringPointer(&input.StartsAt)
	trimStringPointer(&input.EndsAt)
	if input.NotifyMinutesBefore != nil && *input.NotifyMinutesBefore <= 0 {
		return calendarEventUpdateInput{}, fmt.Errorf("notifyMinutesBefore must be a positive number of minutes")
	}
	if input.EventHint == "" {
		return calendarEventUpdateInput{}, fmt.Errorf("eventHint is required")
	}
	if !hasCalendarEventUpdatePatch(input) {
		return calendarEventUpdateInput{}, fmt.Errorf("event_update requires at least one mutable field")
	}
	return input, nil
}

func decodeCalendarEventDeleteInput(document json.RawMessage) (calendarEventDeleteInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return calendarEventDeleteInput{}, fmt.Errorf("eventHint is required")
	}
	var input calendarEventDeleteInput
	if errorValue := decodeStrictCalendarToolInput(document, &input); errorValue != nil {
		return calendarEventDeleteInput{}, errorValue
	}
	input.EventHint = strings.TrimSpace(input.EventHint)
	if input.EventHint == "" {
		return calendarEventDeleteInput{}, fmt.Errorf("eventHint is required")
	}
	return input, nil
}

func decodeStrictCalendarToolInput(document json.RawMessage, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.DisallowUnknownFields()
	if errorValue := decoder.Decode(value); errorValue != nil {
		return errorValue
	}
	if errorValue := decoder.Decode(&struct{}{}); errorValue != io.EOF {
		return fmt.Errorf("calendar input contains trailing data")
	}
	return nil
}

func hasCalendarEventUpdatePatch(input calendarEventUpdateInput) bool {
	return input.Title != nil ||
		input.Note != nil ||
		input.Location != nil ||
		input.StartsAt != nil ||
		input.EndsAt != nil ||
		input.IsWholeDay != nil ||
		input.ParticipantPersonHints != nil ||
		input.EveryoneAttends != nil ||
		input.NotifyMinutesBefore != nil
}

func calendarEventWritePayload(input calendarEventWriteInput) map[string]any {
	payload := map[string]any{
		"title":                     input.Title,
		"description":               input.Note,
		"location":                  input.Location,
		"startISO":                  input.StartsAt,
		"endISO":                    input.EndsAt,
		"isAllDay":                  input.IsWholeDay,
		"people":                    []string(input.People),
		"notifyMinutesBefore":       input.NotifyMinutesBefore,
		"isRequestedOfSomebodyElse": input.IsRequestedOfSomebodyElse,
		"allowDuplicate":            input.AllowDuplicate,
	}
	if input.ExpectedUpdatedAt != "" {
		payload["expectedUpdatedAt"] = input.ExpectedUpdatedAt
	}
	if len(input.Participants) > 0 {
		payload["participants"] = input.Participants
	}
	return payload
}

func calendarEventListPath(input calendarEventListInput) string {
	if input.StartsAt == "" && input.EndsAt == "" {
		return "/calendar/api/events?window=upcoming"
	}
	query := url.Values{}
	query.Set("startISO", input.StartsAt)
	query.Set("endISO", input.EndsAt)
	return "/calendar/api/events?" + query.Encode()
}

func (service Service) sendCalendarToolRequest(ctx context.Context, method string, path string, payload any, requesterEmail string) (json.RawMessage, error) {
	var body io.Reader
	if payload != nil {
		document, errorValue := json.Marshal(payload)
		if errorValue != nil {
			return nil, errorValue
		}
		body = bytes.NewReader(document)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	httpRequest, errorValue := http.NewRequestWithContext(ctx, method, strings.TrimRight(service.Configuration.AdmindBaseURL, "/")+path, body)
	if errorValue != nil {
		return nil, errorValue
	}
	if payload != nil {
		httpRequest.Header.Set("Content-Type", "application/json")
	}
	setCalendarRequesterEmailHeader(httpRequest, requesterEmail)
	httpResponse, errorValue := service.httpClient().Do(httpRequest)
	if errorValue != nil {
		return nil, errorValue
	}
	defer httpResponse.Body.Close()
	responseBody, readError := io.ReadAll(httpResponse.Body)
	if readError != nil {
		return nil, readError
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		message := strings.TrimSpace(string(responseBody))
		var failureDocument struct {
			Code string `json:"code"`
		}
		if errorValue := json.Unmarshal(responseBody, &failureDocument); errorValue == nil && strings.TrimSpace(failureDocument.Code) != "" {
			return nil, &calendarToolRequestError{
				statusCode: httpResponse.StatusCode,
				errorCode:  strings.TrimSpace(failureDocument.Code),
				message:    "calendar tool failed: " + message,
			}
		}
		return nil, &calendarToolRequestError{
			statusCode: httpResponse.StatusCode,
			message:    "calendar tool failed: " + message,
		}
	}
	return json.RawMessage(responseBody), nil
}

func calendarToolVersionConflictResponse(toolName string, errorValue error) (capabilities.ToolInvokeResponse, bool) {
	var requestError *calendarToolRequestError
	if !errors.As(errorValue, &requestError) || requestError.statusCode != http.StatusConflict || requestError.errorCode != calendarEventVersionConflictCode {
		return capabilities.ToolInvokeResponse{}, false
	}
	message := "calendar event changed after it was loaded"
	result, _ := json.Marshal(map[string]string{"errorCode": calendarEventVersionConflictCode, "message": message})
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        toolName,
		Outcome:         capabilities.ToolOutcomeFailed,
		Status:          "error",
		Content:         message,
		IsError:         true,
		Message:         message,
		ErrorCode:       calendarEventVersionConflictCode,
		FailureStage:    "persistence",
		Retryable:       true,
		SafeRetry:       false,
		Result:          result,
	}, true
}

func setCalendarRequesterEmailHeader(request *http.Request, requesterEmail string) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(requesterEmail))
	if normalizedEmail == "" {
		return
	}
	request.Header.Set("CF-Access-Authenticated-User-Email", normalizedEmail)
}

// The device calendar keeps its own spelling of an event. Reading it here is
// serialization: what the model is handed is the row's own vocabulary.
type storedCalendarEvent struct {
	EventID           string                    `json:"eventID"`
	Title             string                    `json:"title"`
	Description       string                    `json:"description"`
	Location          string                    `json:"location"`
	StartISO          string                    `json:"startISO"`
	EndISO            string                    `json:"endISO"`
	TimeZone          string                    `json:"timeZone"`
	IsAllDay          bool                      `json:"isAllDay"`
	People            calendarToolPeopleInput   `json:"people"`
	Participants      []calendarToolParticipant `json:"participants"`
	ReminderLeadHours int                       `json:"reminderLeadHours"`
	UpdatedAt         string                    `json:"updatedAt"`
}

func (stored storedCalendarEvent) eventForTool() calendarEventForTool {
	return calendarEventForTool{
		EventID:             stored.EventID,
		Title:               stored.Title,
		Note:                stored.Description,
		Location:            stored.Location,
		StartsAt:            stored.StartISO,
		EndsAt:              stored.EndISO,
		IsWholeDay:          stored.IsAllDay,
		People:              stored.People,
		TimeZone:            stored.TimeZone,
		Participants:        stored.Participants,
		NotifyMinutesBefore: stored.ReminderLeadHours * 60,
		UpdatedAt:           stored.UpdatedAt,
	}
}

func normalizeCalendarEventResult(result json.RawMessage, expectedEventID string) (json.RawMessage, calendarEventForTool, error) {
	var document map[string]json.RawMessage
	if errorValue := json.Unmarshal(result, &document); errorValue != nil {
		return nil, calendarEventForTool{}, errorValue
	}
	var eventID string
	if errorValue := json.Unmarshal(document["id"], &eventID); errorValue != nil {
		return nil, calendarEventForTool{}, fmt.Errorf("calendar result requires id: %w", errorValue)
	}
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return nil, calendarEventForTool{}, fmt.Errorf("calendar result requires id")
	}
	if expectedEventID != "" && eventID != strings.TrimSpace(expectedEventID) {
		return nil, calendarEventForTool{}, fmt.Errorf("calendar result returned event %q for requested event %q", eventID, strings.TrimSpace(expectedEventID))
	}
	eventIDDocument, _ := json.Marshal(eventID)
	document["eventID"] = eventIDDocument
	delete(document, "id")
	normalizedResult, errorValue := json.Marshal(document)
	if errorValue != nil {
		return nil, calendarEventForTool{}, errorValue
	}
	var stored storedCalendarEvent
	if errorValue := json.Unmarshal(normalizedResult, &stored); errorValue != nil {
		return nil, calendarEventForTool{}, errorValue
	}
	event := stored.eventForTool()
	event.People = normalizeCalendarToolPeople(event.People)
	event.Participants = normalizeCalendarToolParticipants(event.Participants)
	event = localizeCalendarEventTimes(event)
	normalizedResult, errorValue = json.Marshal(event)
	if errorValue != nil {
		return nil, calendarEventForTool{}, errorValue
	}
	return normalizedResult, event, nil
}

func localizeCalendarEventTimes(event calendarEventForTool) calendarEventForTool {
	location, errorValue := time.LoadLocation(strings.TrimSpace(event.TimeZone))
	if errorValue != nil {
		return event
	}
	event.StartsAt = localizeCalendarISOTime(event.StartsAt, location)
	event.EndsAt = localizeCalendarISOTime(event.EndsAt, location)
	return event
}

func localizeCalendarISOTime(value string, location *time.Location) string {
	parsed, errorValue := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if errorValue != nil {
		return value
	}
	return parsed.In(location).Format(time.RFC3339)
}

func normalizeCalendarEventsResult(result json.RawMessage) (calendarEventsForTool, error) {
	var document map[string]json.RawMessage
	if errorValue := json.Unmarshal(result, &document); errorValue != nil {
		return calendarEventsForTool{}, errorValue
	}
	eventsDocument, found := document["events"]
	if !found {
		return calendarEventsForTool{}, fmt.Errorf("calendar list result requires events")
	}
	var rawEvents []json.RawMessage
	if errorValue := json.Unmarshal(eventsDocument, &rawEvents); errorValue != nil {
		return calendarEventsForTool{}, errorValue
	}
	events := make([]calendarEventForTool, 0, len(rawEvents))
	for _, rawEvent := range rawEvents {
		_, event, errorValue := normalizeCalendarEventResult(rawEvent, "")
		if errorValue != nil {
			return calendarEventsForTool{}, errorValue
		}
		events = append(events, event)
	}
	return calendarEventsForTool{Events: events}, nil
}

func filterCalendarEventsForTool(response calendarEventsForTool, input calendarEventListInput) (json.RawMessage, error) {
	filteredEvents := make([]calendarEventForTool, 0, len(response.Events))
	for _, event := range response.Events {
		if !calendarEventMatchesQuery(event, input.Query) {
			continue
		}
		filteredEvents = append(filteredEvents, event)
		if input.Limit != nil && float64(len(filteredEvents)) >= *input.Limit {
			break
		}
	}
	document, errorValue := json.Marshal(calendarEventsForTool{Events: filteredEvents})
	if errorValue != nil {
		return nil, errorValue
	}
	return document, nil
}

func calendarEventMatchesQuery(event calendarEventForTool, query string) bool {
	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	if normalizedQuery == "" {
		return true
	}
	searchText := strings.ToLower(strings.Join([]string{event.Title, event.Note, event.Location}, "\n"))
	return strings.Contains(searchText, normalizedQuery)
}

func isCalendarDuplicateCandidateResult(result json.RawMessage) bool {
	var document struct {
		Status string `json:"status"`
	}
	return json.Unmarshal(result, &document) == nil && strings.TrimSpace(document.Status) == "duplicate_candidate"
}

func calendarToolDuplicateCandidateResponse(toolName string, result json.RawMessage) capabilities.ToolInvokeResponse {
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        toolName,
		Outcome:         capabilities.ToolOutcomeFailed,
		Status:          "duplicate_candidate",
		IsError:         true,
		ErrorCode:       "calendar_duplicate_candidate",
		FailureStage:    "resolution",
		Result:          result,
	}
}
