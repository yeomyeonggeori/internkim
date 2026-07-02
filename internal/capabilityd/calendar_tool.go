package capabilityd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type calendarEventWriteInput struct {
	EventID           string                  `json:"eventID"`
	Title             string                  `json:"title"`
	Description       string                  `json:"description"`
	Location          string                  `json:"location"`
	StartISO          string                  `json:"startISO"`
	EndISO            string                  `json:"endISO"`
	TimeZone          string                  `json:"timeZone"`
	IsAllDay          bool                    `json:"isAllDay"`
	Color             string                  `json:"color"`
	People            calendarToolPeopleInput `json:"people"`
	ReminderLeadHours int                     `json:"reminderLeadHours"`
	AllowDuplicate    bool                    `json:"allowDuplicate"`
}

type calendarEventListInput struct {
	StartISO string `json:"startISO"`
	EndISO   string `json:"endISO"`
	Query    string `json:"query"`
	Limit    int    `json:"limit"`
}

type calendarEventDeleteInput struct {
	EventID string `json:"eventID"`
	Query   string `json:"query"`
}

type calendarEventTarget struct {
	EventID string `json:"eventID"`
	Query   string `json:"query"`
}

type calendarToolPeopleInput []string

type calendarEventsForTool struct {
	Events []calendarEventForTool `json:"events"`
}

type calendarEventForTool struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Location    string `json:"location"`
	StartISO    string `json:"startISO"`
	EndISO      string `json:"endISO"`
	TimeZone    string `json:"timeZone"`
	IsAllDay    bool   `json:"isAllDay"`
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
	input, errorValue := decodeCalendarEventWriteInput(request.Input, false)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	result, errorValue := service.sendCalendarToolRequest(ctx, http.MethodPost, "/calendar/api/events", calendarEventWritePayload(input), request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return calendarToolResponse(request.ToolName, "created", result), nil
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
	filteredResult, errorValue := filterCalendarEventsForTool(result, input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return calendarToolResponse(request.ToolName, "ok", filteredResult), nil
}

func (service Service) invokeCalendarEventUpdate(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	eventID, failure, errorValue := service.resolveCalendarEventTarget(ctx, request)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if failure != nil {
		return *failure, nil
	}
	resolvedInput, errorValue := injectCalendarEventID(request.Input, eventID)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	input, errorValue := decodeCalendarEventWriteInput(resolvedInput, true)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	path := "/calendar/api/events/" + url.PathEscape(input.EventID)
	result, errorValue := service.sendCalendarToolRequest(ctx, http.MethodPut, path, calendarEventWritePayload(input), request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return calendarToolResponse(request.ToolName, "updated", result), nil
}

func (service Service) invokeCalendarEventDelete(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	eventID, failure, errorValue := service.resolveCalendarEventTarget(ctx, request)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if failure != nil {
		return *failure, nil
	}
	path := "/calendar/api/events/" + url.PathEscape(eventID)
	if _, errorValue := service.sendCalendarToolRequest(ctx, http.MethodDelete, path, nil, request.Context.RequesterEmail); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	result, _ := json.Marshal(map[string]any{"eventID": eventID, "deleted": true})
	return calendarToolResponse(request.ToolName, "deleted", result), nil
}

func (service Service) resolveCalendarEventTarget(ctx context.Context, request capabilities.ToolInvokeRequest) (string, *capabilities.ToolInvokeResponse, error) {
	var target calendarEventTarget
	_ = json.Unmarshal(request.Input, &target)
	target.EventID = strings.TrimSpace(target.EventID)
	target.Query = strings.TrimSpace(target.Query)
	if target.EventID != "" {
		return target.EventID, nil, nil
	}
	if target.Query == "" {
		return "", nil, fmt.Errorf("eventID or query is required")
	}
	result, errorValue := service.sendCalendarToolRequest(ctx, http.MethodGet, "/calendar/api/events", nil, request.Context.RequesterEmail)
	if errorValue != nil {
		return "", nil, errorValue
	}
	var response calendarEventsForTool
	if errorValue := json.Unmarshal(result, &response); errorValue != nil {
		return "", nil, errorValue
	}
	matches := make([]calendarEventForTool, 0, len(response.Events))
	for _, event := range response.Events {
		if calendarEventMatchesQuery(event, target.Query) {
			matches = append(matches, event)
		}
	}
	if len(matches) == 0 {
		failure := calendarResolutionFailure(request.ToolName, "calendar_event_not_found", "no calendar event matched the query", nil)
		return "", &failure, nil
	}
	if len(matches) > 1 {
		failure := calendarResolutionFailure(request.ToolName, "calendar_event_ambiguous", "multiple calendar events matched the query", matches)
		return "", &failure, nil
	}
	return matches[0].ID, nil, nil
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

func injectCalendarEventID(document json.RawMessage, eventID string) (json.RawMessage, error) {
	values := map[string]any{}
	if len(bytes.TrimSpace(document)) > 0 {
		if errorValue := json.Unmarshal(document, &values); errorValue != nil {
			return nil, errorValue
		}
	}
	values["eventID"] = eventID
	return json.Marshal(values)
}

func decodeCalendarEventWriteInput(document json.RawMessage, needsEventID bool) (calendarEventWriteInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return calendarEventWriteInput{}, fmt.Errorf("calendar event input is required")
	}
	var input calendarEventWriteInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return calendarEventWriteInput{}, errorValue
	}
	input.EventID = strings.TrimSpace(input.EventID)
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	input.Location = strings.TrimSpace(input.Location)
	input.StartISO = strings.TrimSpace(input.StartISO)
	input.EndISO = strings.TrimSpace(input.EndISO)
	input.TimeZone = strings.TrimSpace(input.TimeZone)
	input.Color = strings.TrimSpace(input.Color)
	input.People = normalizeCalendarToolPeople([]string(input.People))
	input.ReminderLeadHours = normalizeCalendarToolReminderLeadHours(input.ReminderLeadHours)
	if needsEventID && input.EventID == "" {
		return calendarEventWriteInput{}, fmt.Errorf("eventID is required")
	}
	if input.Title == "" {
		return calendarEventWriteInput{}, fmt.Errorf("title is required")
	}
	if input.StartISO == "" || input.EndISO == "" {
		return calendarEventWriteInput{}, fmt.Errorf("startISO and endISO are required")
	}
	if input.EventID == "" {
		input.EventID = stableCalendarToolEventID(input)
	}
	return input, nil
}

func (people *calendarToolPeopleInput) UnmarshalJSON(document []byte) error {
	trimmedDocument := bytes.TrimSpace(document)
	if len(trimmedDocument) == 0 || bytes.Equal(trimmedDocument, []byte("null")) {
		*people = nil
		return nil
	}
	var values []string
	if errorValue := json.Unmarshal(trimmedDocument, &values); errorValue == nil {
		*people = normalizeCalendarToolPeople(values)
		return nil
	}
	var value string
	if errorValue := json.Unmarshal(trimmedDocument, &value); errorValue != nil {
		return errorValue
	}
	*people = normalizeCalendarToolPeople(strings.Split(value, ","))
	return nil
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

func normalizeCalendarToolReminderLeadHours(value int) int {
	switch value {
	case 1, 2, 3, 6, 12, 24, 48:
		return value
	default:
		return 24
	}
}

func decodeCalendarEventListInput(document json.RawMessage) (calendarEventListInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return calendarEventListInput{}, nil
	}
	var input calendarEventListInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return calendarEventListInput{}, errorValue
	}
	input.StartISO = strings.TrimSpace(input.StartISO)
	input.EndISO = strings.TrimSpace(input.EndISO)
	input.Query = strings.TrimSpace(input.Query)
	if input.Limit < 0 {
		return calendarEventListInput{}, fmt.Errorf("limit must be positive")
	}
	if (input.StartISO == "") != (input.EndISO == "") {
		return calendarEventListInput{}, fmt.Errorf("startISO and endISO must be provided together")
	}
	return input, nil
}

func decodeCalendarEventDeleteInput(document json.RawMessage) (calendarEventDeleteInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return calendarEventDeleteInput{}, fmt.Errorf("eventID is required")
	}
	var input calendarEventDeleteInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return calendarEventDeleteInput{}, errorValue
	}
	input.EventID = strings.TrimSpace(input.EventID)
	if input.EventID == "" {
		return calendarEventDeleteInput{}, fmt.Errorf("eventID is required")
	}
	return input, nil
}

func calendarEventWritePayload(input calendarEventWriteInput) map[string]any {
	return map[string]any{
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
		return nil, fmt.Errorf("calendar tool failed: %s", strings.TrimSpace(string(responseBody)))
	}
	return json.RawMessage(responseBody), nil
}

func setCalendarRequesterEmailHeader(request *http.Request, requesterEmail string) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(requesterEmail))
	if normalizedEmail == "" {
		return
	}
	request.Header.Set("CF-Access-Authenticated-User-Email", normalizedEmail)
}

func filterCalendarEventsForTool(result json.RawMessage, input calendarEventListInput) (json.RawMessage, error) {
	if strings.TrimSpace(input.Query) == "" && input.Limit == 0 {
		return result, nil
	}
	var response calendarEventsForTool
	if errorValue := json.Unmarshal(result, &response); errorValue != nil {
		return nil, errorValue
	}
	filteredEvents := make([]calendarEventForTool, 0, len(response.Events))
	for _, event := range response.Events {
		if !calendarEventMatchesQuery(event, input.Query) {
			continue
		}
		filteredEvents = append(filteredEvents, event)
		if input.Limit > 0 && len(filteredEvents) >= input.Limit {
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
		Status:          status,
		Result:          result,
	}
}
