package capabilityd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	EventID           string
	Title             string
	Description       string
	Location          string
	StartISO          string
	EndISO            string
	TimeZone          string
	IsAllDay          bool
	Color             string
	People            calendarToolPeopleInput
	Participants      []calendarToolParticipant
	ReminderLeadHours int
	AllowDuplicate    bool
	IncludeRequester  *bool
	ExpectedUpdatedAt string
	GeneratedEventID  bool
}

type calendarEventAddInput struct {
	Title             string                  `json:"title"`
	Description       string                  `json:"description"`
	Location          string                  `json:"location"`
	StartISO          string                  `json:"startISO"`
	EndISO            string                  `json:"endISO"`
	TimeZone          string                  `json:"timeZone"`
	IsAllDay          bool                    `json:"isAllDay"`
	Color             string                  `json:"color"`
	People            calendarToolPeopleInput `json:"people"`
	ReminderLeadHours *int                    `json:"reminderLeadHours"`
	IncludeRequester  *bool                   `json:"includeRequester"`
}

type calendarEventListInput struct {
	StartISO string   `json:"startISO"`
	EndISO   string   `json:"endISO"`
	Query    string   `json:"query"`
	Limit    *float64 `json:"limit"`
}

type calendarEventUpdateInput struct {
	EventID           string                   `json:"eventID"`
	Title             *string                  `json:"title"`
	Description       *string                  `json:"description"`
	Location          *string                  `json:"location"`
	StartISO          *string                  `json:"startISO"`
	EndISO            *string                  `json:"endISO"`
	TimeZone          *string                  `json:"timeZone"`
	IsAllDay          *bool                    `json:"isAllDay"`
	Color             *string                  `json:"color"`
	People            *calendarToolPeopleInput `json:"people"`
	ReminderLeadHours *int                     `json:"reminderLeadHours"`
	IncludeRequester  *bool                    `json:"includeRequester"`
}

type calendarEventDeleteInput struct {
	EventID string `json:"eventID"`
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
	EventID           string                    `json:"eventID"`
	Title             string                    `json:"title"`
	Description       string                    `json:"description"`
	Location          string                    `json:"location"`
	StartISO          string                    `json:"startISO"`
	EndISO            string                    `json:"endISO"`
	TimeZone          string                    `json:"timeZone"`
	IsAllDay          bool                      `json:"isAllDay"`
	Color             string                    `json:"color"`
	People            calendarToolPeopleInput   `json:"people"`
	Participants      []calendarToolParticipant `json:"participants"`
	ReminderLeadHours int                       `json:"reminderLeadHours"`
	UpdatedAt         string                    `json:"updatedAt"`
}

func (service Service) invokeCalendarTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	switch strings.TrimSpace(request.ToolName) {
	case "calendar.add":
		return service.invokeCalendarEventAdd(ctx, request)
	case "calendar.list":
		return service.invokeCalendarEventList(ctx, request)
	case "calendar.update":
		return service.invokeCalendarEventUpdate(ctx, request)
	case "calendar.delete":
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
	normalizedResult, _, errorValue := normalizeCalendarEventResult(result, input.EventID)
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
	return calendarToolResponse(request.ToolName, "ok", filteredResult), nil
}

func (service Service) invokeCalendarEventUpdate(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	updateInput, errorValue := decodeCalendarEventUpdateInput(request.Input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	target, failure, errorValue := service.resolveCalendarEventTarget(ctx, request, updateInput.EventID)
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
	target, failure, errorValue := service.resolveCalendarEventTarget(ctx, request, input.EventID)
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
		Title:            strings.TrimSpace(externalInput.Title),
		Description:      strings.TrimSpace(externalInput.Description),
		Location:         strings.TrimSpace(externalInput.Location),
		StartISO:         strings.TrimSpace(externalInput.StartISO),
		EndISO:           strings.TrimSpace(externalInput.EndISO),
		TimeZone:         strings.TrimSpace(externalInput.TimeZone),
		IsAllDay:         externalInput.IsAllDay,
		Color:            strings.TrimSpace(externalInput.Color),
		People:           normalizeCalendarToolPeople([]string(externalInput.People)),
		IncludeRequester: externalInput.IncludeRequester,
		GeneratedEventID: true,
	}
	reminderLeadHours, errorValue := calendarToolReminderLeadHours(externalInput.ReminderLeadHours)
	if errorValue != nil {
		return calendarEventWriteInput{}, errorValue
	}
	input.ReminderLeadHours = reminderLeadHours
	if input.Title == "" {
		return calendarEventWriteInput{}, fmt.Errorf("title is required")
	}
	if input.StartISO == "" || input.EndISO == "" {
		return calendarEventWriteInput{}, fmt.Errorf("startISO and endISO are required")
	}
	input.EventID = stableCalendarToolEventID(input)
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
	input.StartISO = strings.TrimSpace(input.StartISO)
	input.EndISO = strings.TrimSpace(input.EndISO)
	input.Query = strings.TrimSpace(input.Query)
	if input.Limit != nil && (*input.Limit <= 0 || math.Trunc(*input.Limit) != *input.Limit) {
		return calendarEventListInput{}, fmt.Errorf("limit must be a positive whole number")
	}
	if (input.StartISO == "") != (input.EndISO == "") {
		return calendarEventListInput{}, fmt.Errorf("startISO and endISO must be provided together")
	}
	return input, nil
}

func decodeCalendarEventUpdateInput(document json.RawMessage) (calendarEventUpdateInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return calendarEventUpdateInput{}, fmt.Errorf("calendar.update input is required")
	}
	var input calendarEventUpdateInput
	if errorValue := decodeStrictCalendarToolInput(document, &input); errorValue != nil {
		return calendarEventUpdateInput{}, errorValue
	}
	input.EventID = strings.TrimSpace(input.EventID)
	trimStringPointer(&input.Title)
	trimStringPointer(&input.Description)
	trimStringPointer(&input.Location)
	trimStringPointer(&input.StartISO)
	trimStringPointer(&input.EndISO)
	trimStringPointer(&input.TimeZone)
	trimStringPointer(&input.Color)
	if _, errorValue := calendarToolReminderLeadHours(input.ReminderLeadHours); errorValue != nil {
		return calendarEventUpdateInput{}, errorValue
	}
	if input.EventID == "" {
		return calendarEventUpdateInput{}, fmt.Errorf("eventID is required")
	}
	if !hasCalendarEventUpdatePatch(input) {
		return calendarEventUpdateInput{}, fmt.Errorf("calendar.update requires at least one mutable field")
	}
	return input, nil
}

func decodeCalendarEventDeleteInput(document json.RawMessage) (calendarEventDeleteInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return calendarEventDeleteInput{}, fmt.Errorf("eventID is required")
	}
	var input calendarEventDeleteInput
	if errorValue := decodeStrictCalendarToolInput(document, &input); errorValue != nil {
		return calendarEventDeleteInput{}, errorValue
	}
	input.EventID = strings.TrimSpace(input.EventID)
	if input.EventID == "" {
		return calendarEventDeleteInput{}, fmt.Errorf("eventID is required")
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
		input.Description != nil ||
		input.Location != nil ||
		input.StartISO != nil ||
		input.EndISO != nil ||
		input.TimeZone != nil ||
		input.IsAllDay != nil ||
		input.Color != nil ||
		input.People != nil ||
		input.ReminderLeadHours != nil ||
		input.IncludeRequester != nil
}

func calendarEventWritePayload(input calendarEventWriteInput) map[string]any {
	payload := map[string]any{
		"eventID":           input.EventID,
		"title":             input.Title,
		"description":       input.Description,
		"location":          input.Location,
		"startISO":          input.StartISO,
		"endISO":            input.EndISO,
		"timeZone":          input.TimeZone,
		"isAllDay":          input.IsAllDay,
		"color":             input.Color,
		"people":            []string(input.People),
		"reminderLeadHours": input.ReminderLeadHours,
		"allowDuplicate":    input.AllowDuplicate,
	}
	if input.ExpectedUpdatedAt != "" {
		payload["expectedUpdatedAt"] = input.ExpectedUpdatedAt
	}
	if len(input.Participants) > 0 {
		payload["participants"] = input.Participants
	}
	return payload
}

func stableCalendarToolEventID(input calendarEventWriteInput) string {
	document := strings.Join([]string{
		input.Title,
		input.Description,
		input.Location,
		input.StartISO,
		input.EndISO,
		input.TimeZone,
		fmt.Sprintf("%t", input.IsAllDay),
		strings.Join([]string(input.People), ","),
	}, "\x00")
	sum := sha256.Sum256([]byte(document))
	return "tool-" + hex.EncodeToString(sum[:])[:32]
}

func calendarEventListPath(input calendarEventListInput) string {
	if input.StartISO == "" && input.EndISO == "" {
		return "/calendar/api/events?window=upcoming"
	}
	query := url.Values{}
	query.Set("startISO", input.StartISO)
	query.Set("endISO", input.EndISO)
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
	var event calendarEventForTool
	if errorValue := json.Unmarshal(normalizedResult, &event); errorValue != nil {
		return nil, calendarEventForTool{}, errorValue
	}
	event.People = normalizeCalendarToolPeople(event.People)
	event.Participants = normalizeCalendarToolParticipants(event.Participants)
	if errorValue := validateCalendarEventResult(event); errorValue != nil {
		return nil, calendarEventForTool{}, errorValue
	}
	normalizedResult, errorValue = json.Marshal(event)
	if errorValue != nil {
		return nil, calendarEventForTool{}, errorValue
	}
	return normalizedResult, event, nil
}

func validateCalendarEventResult(event calendarEventForTool) error {
	requiredValues := []struct {
		field string
		value string
	}{
		{field: "eventID", value: event.EventID},
		{field: "title", value: event.Title},
		{field: "startISO", value: event.StartISO},
		{field: "endISO", value: event.EndISO},
		{field: "timeZone", value: event.TimeZone},
		{field: "updatedAt", value: event.UpdatedAt},
	}
	for _, requiredValue := range requiredValues {
		if strings.TrimSpace(requiredValue.value) == "" {
			return fmt.Errorf("calendar result requires %s", requiredValue.field)
		}
	}
	return nil
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
	searchText := strings.ToLower(strings.Join([]string{event.Title, event.Description, event.Location}, "\n"))
	return strings.Contains(searchText, normalizedQuery)
}

func calendarToolResponse(toolName string, status string, result json.RawMessage) capabilities.ToolInvokeResponse {
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        toolName,
		Outcome:         capabilities.ToolOutcomeSucceeded,
		Status:          status,
		Result:          result,
	}
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
