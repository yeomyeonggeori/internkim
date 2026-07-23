package capabilityd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestCalendarEventAddPostsToAdmind(t *testing.T) {
	var requesterEmail string
	var payload map[string]any
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local", BlueclawBaseURL: "http://blueclaw.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method == http.MethodPost && request.URL.String() == "http://blueclaw.local/admin/api/identity/resolve-recipient" {
				return calendarToolJSONResponse(`{"status":"not_found"}`), nil
			}
			if request.Method != http.MethodPost || request.URL.String() != "http://admind.local/calendar/api/events" {
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			}
			requesterEmail = request.Header.Get("CF-Access-Authenticated-User-Email")
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			return calendarToolEventResponse(payload["eventID"].(string), "Demo", "2026-05-08T01:00:00Z", "2026-05-08T02:00:00Z"), nil
		})},
	}

	response, errorValue := service.invokeCalendarEventAdd(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.add",
		Input:    []byte(`{"title":"Demo","startISO":"2026-05-08T10:00:00+09:00","endISO":"2026-05-08T11:00:00+09:00","location":"Office","people":["샘플","수민"],"reminderLeadHours":48}`),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail: "Staff@Example.com",
			RequesterName:  "Staff",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "created" {
		t.Fatalf("status = %q", response.Status)
	}
	assertCalendarMutationEffect(t, response, payload["eventID"].(string), "created")
	var result map[string]any
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if result["eventID"] != payload["eventID"] || result["id"] != nil {
		t.Fatalf("canonical result = %#v", result)
	}
	if requesterEmail != "staff@example.com" {
		t.Fatalf("requesterEmail = %q", requesterEmail)
	}
	if payload["title"] != "Demo" || payload["location"] != "Office" {
		t.Fatalf("payload = %#v", payload)
	}
	if payload["eventID"] == "" {
		t.Fatalf("eventID was not generated: %#v", payload)
	}
	people, _ := payload["people"].([]any)
	if len(people) != 3 || people[0] != "Staff" || people[1] != "샘플" || people[2] != "수민" || payload["reminderLeadHours"] != float64(48) {
		t.Fatalf("calendar metadata payload = %#v", payload)
	}
}

func TestCalendarEventAddResolvesPeopleHintsAndIncludesRequester(t *testing.T) {
	var payload map[string]any
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local", BlueclawBaseURL: "http://blueclaw.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodPost && request.URL.String() == "http://blueclaw.local/admin/api/identity/resolve-recipient":
				var requestBody map[string]string
				if errorValue := json.NewDecoder(request.Body).Decode(&requestBody); errorValue != nil {
					t.Fatal(errorValue)
				}
				switch requestBody["hint"] {
				case "staff@example.com":
					return calendarToolJSONResponse(`{"status":"resolved","recipient":{"personID":"person-staff","displayName":"김표본","emails":["staff@example.com"],"externalUserID":"user-staff","username":"pyobon"}}`), nil
				case "테스트":
					return calendarToolJSONResponse(`{"status":"resolved","recipient":{"personID":"person-rain","displayName":"김테스트","emails":["rain@example.com"],"externalUserID":"user-rain","username":"rain"}}`), nil
				default:
					return calendarToolJSONResponse(`{"status":"not_found"}`), nil
				}
			case request.Method == http.MethodPost && request.URL.String() == "http://admind.local/calendar/api/events":
				if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
					t.Fatal(errorValue)
				}
				return calendarToolEventResponse(payload["eventID"].(string), "경산 일정", "2026-05-08T05:00:00+09:00", "2026-05-08T06:00:00+09:00"), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeCalendarEventAdd(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.add",
		Input:    []byte(`{"title":"경산 일정","startISO":"2026-05-08T05:00:00+09:00","endISO":"2026-05-08T06:00:00+09:00","people":["테스트"]}`),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:    "staff@example.com",
			RequesterName:     "김표본",
			RequesterPersonID: "person-staff",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "created" {
		t.Fatalf("status = %q", response.Status)
	}
	people, _ := payload["people"].([]any)
	participants, _ := payload["participants"].([]any)
	if len(people) != 2 || people[0] != "김표본" || people[1] != "김테스트" {
		t.Fatalf("people = %#v payload=%#v", people, payload)
	}
	if len(participants) != 2 {
		t.Fatalf("participants = %#v payload=%#v", participants, payload)
	}
	firstParticipant, _ := participants[0].(map[string]any)
	secondParticipant, _ := participants[1].(map[string]any)
	if firstParticipant["personID"] != "person-staff" || secondParticipant["personID"] != "person-rain" {
		t.Fatalf("participants = %#v", participants)
	}
}

func TestCalendarToolShouldIncludeRequesterPolicy(t *testing.T) {
	excludeRequester := false
	includeRequester := true
	testCases := []struct {
		name                    string
		input                   calendarEventWriteInput
		includeRequesterDefault bool
		expected                bool
	}{
		{
			name:                    "calendar add default includes requester",
			input:                   calendarEventWriteInput{People: calendarToolPeopleInput{"테스트"}},
			includeRequesterDefault: true,
			expected:                true,
		},
		{
			name:                    "calendar update default excludes requester",
			input:                   calendarEventWriteInput{People: calendarToolPeopleInput{"테스트"}},
			includeRequesterDefault: false,
			expected:                false,
		},
		{
			name:                    "explicit false excludes requester",
			input:                   calendarEventWriteInput{People: calendarToolPeopleInput{"테스트"}, IncludeRequester: &excludeRequester},
			includeRequesterDefault: true,
			expected:                false,
		},
		{
			name:                    "explicit true includes requester",
			input:                   calendarEventWriteInput{People: calendarToolPeopleInput{"테스트"}, IncludeRequester: &includeRequester},
			includeRequesterDefault: false,
			expected:                true,
		},
		{
			name:                    "all hands excludes requester",
			input:                   calendarEventWriteInput{People: calendarToolPeopleInput{"전체"}},
			includeRequesterDefault: true,
			expected:                false,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual := calendarToolShouldIncludeRequester(testCase.input, testCase.includeRequesterDefault)
			if actual != testCase.expected {
				t.Fatalf("include requester = %v, want %v", actual, testCase.expected)
			}
		})
	}
}

func TestCalendarEventAddGeneratesStableEventID(t *testing.T) {
	first, firstError := decodeCalendarEventWriteInput([]byte(`{"title":"휴가","startISO":"2026-05-10T00:00:00Z","endISO":"2026-05-13T00:00:00Z","isAllDay":true}`))
	second, secondError := decodeCalendarEventWriteInput([]byte(`{"title":"휴가","startISO":"2026-05-10T00:00:00Z","endISO":"2026-05-13T00:00:00Z","isAllDay":true}`))
	if firstError != nil || secondError != nil {
		t.Fatalf("decode errors: %v %v", firstError, secondError)
	}
	if first.EventID == "" || first.EventID != second.EventID {
		t.Fatalf("event IDs = %q %q", first.EventID, second.EventID)
	}
}

func TestCalendarInputsRejectUnknownTrailingAndLegacyAliases(t *testing.T) {
	tests := []struct {
		name   string
		decode func() error
	}{
		{
			name: "add unknown internal field",
			decode: func() error {
				_, errorValue := decodeCalendarEventWriteInput([]byte(`{"title":"Demo","startISO":"2026-05-08T10:00:00+09:00","endISO":"2026-05-08T11:00:00+09:00","participants":[]}`))
				return errorValue
			},
		},
		{
			name: "add comma-delimited people",
			decode: func() error {
				_, errorValue := decodeCalendarEventWriteInput([]byte(`{"title":"Demo","startISO":"2026-05-08T10:00:00+09:00","endISO":"2026-05-08T11:00:00+09:00","people":"Alice,Bob"}`))
				return errorValue
			},
		},
		{
			name: "add hidden duplicate override",
			decode: func() error {
				_, errorValue := decodeCalendarEventWriteInput([]byte(`{"title":"Demo","startISO":"2026-05-08T10:00:00+09:00","endISO":"2026-05-08T11:00:00+09:00","allowDuplicate":true}`))
				return errorValue
			},
		},
		{
			name: "list trailing data",
			decode: func() error {
				_, errorValue := decodeCalendarEventListInput([]byte(`{} {}`))
				return errorValue
			},
		},
		{
			name: "update internal participants",
			decode: func() error {
				_, errorValue := decodeCalendarEventUpdateInput([]byte(`{"eventHint":"event-1","participants":[]}`))
				return errorValue
			},
		},
		{
			name: "delete unknown query",
			decode: func() error {
				_, errorValue := decodeCalendarEventDeleteInput([]byte(`{"eventHint":"event-1","query":"Demo"}`))
				return errorValue
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if errorValue := testCase.decode(); errorValue == nil {
				t.Fatal("expected strict calendar input error")
			}
		})
	}
}

func TestCalendarListRequiresPositiveWholeNumberLimit(t *testing.T) {
	for _, document := range []string{`{"limit":0}`, `{"limit":-1}`, `{"limit":1.5}`} {
		if _, errorValue := decodeCalendarEventListInput([]byte(document)); errorValue == nil {
			t.Fatalf("expected list limit validation error for %s", document)
		}
	}
	if _, errorValue := decodeCalendarEventListInput([]byte(`{"limit":2}`)); errorValue != nil {
		t.Fatalf("expected whole-number limit to pass: %v", errorValue)
	}
}

func TestCalendarUpdateRequiresPatchAndAllowedReminder(t *testing.T) {
	for _, document := range []string{
		`{"eventHint":"event-1"}`,
		`{"eventHint":"event-1","reminderLeadHours":0}`,
		`{"eventHint":"event-1","reminderLeadHours":5}`,
	} {
		if _, errorValue := decodeCalendarEventUpdateInput([]byte(document)); errorValue == nil {
			t.Fatalf("expected update validation error for %s", document)
		}
	}
	if _, errorValue := decodeCalendarEventWriteInput([]byte(`{"title":"Demo","startISO":"2026-05-08T10:00:00+09:00","endISO":"2026-05-08T11:00:00+09:00","reminderLeadHours":5}`)); errorValue == nil {
		t.Fatal("expected add reminder validation error")
	}
}

func TestCalendarEventListFiltersQueryAndLimit(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			expectedURL := "http://admind.local/calendar/api/events?endISO=2026-05-09T00%3A00%3A00Z&startISO=2026-05-08T00%3A00%3A00Z"
			if request.Method != http.MethodGet || request.URL.String() != expectedURL {
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			}
			firstEvent := calendarToolEventDocument("event-1", "Design review", "2026-05-08T01:00:00Z", "2026-05-08T02:00:00Z")
			firstEvent["location"] = "Office"
			secondEvent := calendarToolEventDocument("event-2", "Lunch", "2026-05-08T03:00:00Z", "2026-05-08T04:00:00Z")
			secondEvent["location"] = "Cafe"
			return calendarToolEventsResponse(firstEvent, secondEvent), nil
		})},
	}

	response, errorValue := service.invokeCalendarEventList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.list",
		Input:    []byte(`{"startISO":"2026-05-08T00:00:00Z","endISO":"2026-05-09T00:00:00Z","query":"design","limit":1}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var document calendarEventsForTool
	if errorValue := json.Unmarshal(response.Result, &document); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(document.Events) != 1 || document.Events[0].EventID != "event-1" {
		t.Fatalf("events = %#v", document.Events)
	}
	if response.Outcome != capabilities.ToolOutcomeSucceeded || len(response.Effects) != 0 {
		t.Fatalf("list response = %+v", response)
	}
}

func TestCalendarEventResultsRequireAndNormalizeIdentity(t *testing.T) {
	event := calendarToolEventDocument("event-1", "Demo", "2026-05-08T10:00:00+09:00", "2026-05-08T11:00:00+09:00")
	event["createdByEmail"] = "creator@example.com"
	document, _ := json.Marshal(event)
	normalized, normalizedEvent, errorValue := normalizeCalendarEventResult(document, "event-1")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var normalizedDocument map[string]any
	if errorValue := json.Unmarshal(normalized, &normalizedDocument); errorValue != nil {
		t.Fatal(errorValue)
	}
	if normalizedEvent.EventID != "event-1" || normalizedDocument["eventID"] != "event-1" {
		t.Fatalf("normalized result = %#v", normalizedDocument)
	}
	if normalizedDocument["id"] != nil || normalizedDocument["createdByEmail"] != nil {
		t.Fatalf("non-canonical result fields survived: %#v", normalizedDocument)
	}

	delete(event, "id")
	document, _ = json.Marshal(event)
	if _, _, errorValue := normalizeCalendarEventResult(document, ""); errorValue == nil {
		t.Fatal("expected missing result identity error")
	}
	event["id"] = "event-2"
	document, _ = json.Marshal(event)
	if _, _, errorValue := normalizeCalendarEventResult(document, "event-1"); errorValue == nil {
		t.Fatal("expected mismatched result identity error")
	}
}

func TestCalendarEventListRejectsEventWithoutIdentity(t *testing.T) {
	event := calendarToolEventDocument("", "Demo", "2026-05-08T10:00:00+09:00", "2026-05-08T11:00:00+09:00")
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return calendarToolEventsResponse(event), nil
		})},
	}
	if _, errorValue := service.invokeCalendarEventList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.list",
		Input:    []byte(`{}`),
	}); errorValue == nil {
		t.Fatal("expected list result identity error")
	}
}

func TestCalendarEventAddPreservesDuplicateControlWithoutSuccessEffect(t *testing.T) {
	var allowDuplicate bool
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			var payload map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			allowDuplicate, _ = payload["allowDuplicate"].(bool)
			if !allowDuplicate {
				return calendarToolJSONResponse(`{"status":"duplicate_candidate","candidates":[{"id":"event-existing","title":"Demo"}]}`), nil
			}
			return calendarToolEventResponse(payload["eventID"].(string), "Demo", "2026-05-08T10:00:00+09:00", "2026-05-08T11:00:00+09:00"), nil
		})},
	}
	baseInput := `{"title":"Demo","startISO":"2026-05-08T10:00:00+09:00","endISO":"2026-05-08T11:00:00+09:00"`
	response, errorValue := service.invokeCalendarEventAdd(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.add",
		Input:    []byte(baseInput + `}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed || !response.IsError || response.Status != "duplicate_candidate" || len(response.Effects) != 0 {
		t.Fatalf("duplicate candidate response = %+v", response)
	}
	if !strings.Contains(string(response.Result), `"candidates"`) {
		t.Fatalf("duplicate candidate result = %s", response.Result)
	}

	response, errorValue = service.invokeCalendarEventAdd(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.add",
		Input:    []byte(baseInput + `}`),
		Context: capabilities.ToolInvokeContext{
			ConflictResolution: capabilities.ToolConflictResolutionAllowDuplicate,
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !allowDuplicate {
		t.Fatal("allowDuplicate control did not reach admind")
	}
	assertCalendarMutationEffect(t, response, response.Effects[0].ID, "created")
}

func TestCalendarEventListDefaultsToUpcomingWindow(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.String() != "http://admind.local/calendar/api/events?window=upcoming" {
				t.Fatalf("unexpected request %s", request.URL.String())
			}
			return calendarToolJSONResponse(`{"events":[]}`), nil
		})},
	}

	if _, errorValue := service.invokeCalendarEventList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.list",
		Input:    []byte(`{}`),
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestCalendarMutationInputsRequireEventHint(t *testing.T) {
	tests := []struct {
		name   string
		invoke func(Service) error
	}{
		{
			name: "update query",
			invoke: func(service Service) error {
				_, errorValue := service.invokeCalendarEventUpdate(context.Background(), capabilities.ToolInvokeRequest{
					ToolName: "calendar.update",
					Input:    []byte(`{"query":"비용 테스트 일정","startISO":"2026-07-16T14:00:00+09:00"}`),
				})
				return errorValue
			},
		},
		{
			name: "update title without hint",
			invoke: func(service Service) error {
				_, errorValue := service.invokeCalendarEventUpdate(context.Background(), capabilities.ToolInvokeRequest{
					ToolName: "calendar.update",
					Input:    []byte(`{"title":"비용 테스트 일정"}`),
				})
				return errorValue
			},
		},
		{
			name: "delete query",
			invoke: func(service Service) error {
				_, errorValue := service.invokeCalendarEventDelete(context.Background(), capabilities.ToolInvokeRequest{
					ToolName: "calendar.delete",
					Input:    []byte(`{"query":"비용 테스트 일정"}`),
				})
				return errorValue
			},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			service := Service{HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				t.Fatalf("mutation input reached HTTP boundary: %s", request.URL.String())
				return nil, nil
			})}}
			if errorValue := testCase.invoke(service); errorValue == nil {
				t.Fatal("expected eventHint validation error")
			}
		})
	}
}

func TestCalendarEventUpdatePreservesOmittedFields(t *testing.T) {
	requestCount := 0
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestCount++
			switch requestCount {
			case 1:
				return calendarToolEventsResponse(calendarToolEventDocument("event-1", "Original title", "2026-07-16T14:00:00+09:00", "2026-07-16T15:00:00+09:00")), nil
			case 2:
				return calendarToolJSONResponse(`{
					"id":"event-1",
					"title":"Original title",
					"description":"Original description",
					"location":"Original location",
					"startISO":"2026-07-16T14:00:00+09:00",
					"endISO":"2026-07-16T15:00:00+09:00",
					"timeZone":"Asia/Seoul",
					"isAllDay":false,
					"color":"blueberry",
					"people":["Alice"],
					"participants":[{"personID":"person-alice","name":"Alice","email":"alice@example.com"}],
					"reminderLeadHours":6,
					"updatedAt":"2026-07-16T00:00:00Z"
				}`), nil
			case 3:
				var payload map[string]any
				if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
					t.Fatal(errorValue)
				}
				if payload["title"] != "Changed title" || payload["description"] != "Original description" || payload["location"] != "Original location" {
					t.Fatalf("text fields = %#v", payload)
				}
				if payload["startISO"] != "2026-07-16T14:00:00+09:00" || payload["endISO"] != "2026-07-16T15:00:00+09:00" || payload["timeZone"] != "Asia/Seoul" {
					t.Fatalf("time fields = %#v", payload)
				}
				if payload["color"] != "blueberry" || payload["reminderLeadHours"] != float64(6) {
					t.Fatalf("presentation fields = %#v", payload)
				}
				participants, found := payload["participants"].([]any)
				if !found || len(participants) != 1 {
					t.Fatalf("participants = %#v", payload["participants"])
				}
				if payload["expectedUpdatedAt"] != "2026-07-16T00:00:00Z" {
					t.Fatalf("expectedUpdatedAt = %#v", payload["expectedUpdatedAt"])
				}
				return calendarToolEventResponse("event-1", "Changed title", "2026-07-16T14:00:00+09:00", "2026-07-16T15:00:00+09:00"), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeCalendarEventUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.update",
		Input:    []byte(`{"eventHint":"event-1","title":"Changed title"}`),
	})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "updated" || requestCount != 3 {
		t.Fatalf("response = %+v requests = %d", response, requestCount)
	}
	assertCalendarMutationEffect(t, response, "event-1", "updated")
}

func TestCalendarEventUpdateAppliesExplicitEmptyAndFalseFields(t *testing.T) {
	current := calendarEventForTool{
		EventID:           "event-1",
		Title:             "Original title",
		Description:       "Original description",
		Location:          "Original location",
		StartISO:          "2026-07-16T14:00:00+09:00",
		EndISO:            "2026-07-16T15:00:00+09:00",
		TimeZone:          "Asia/Seoul",
		IsAllDay:          true,
		Participants:      []calendarToolParticipant{{PersonID: "person-alice", Name: "Alice"}},
		ReminderLeadHours: 6,
	}
	update, errorValue := decodeCalendarEventUpdateInput([]byte(`{
		"eventHint":"event-1",
		"description":"",
		"location":"",
		"isAllDay":false,
		"people":[],
		"reminderLeadHours":1,
		"includeRequester":false
	}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	input := mergeCalendarEventUpdateInput(update, current)
	if input.Description != "" || input.Location != "" || input.IsAllDay {
		t.Fatalf("explicit empty and false fields were not applied: %+v", input)
	}
	if len(input.People) != 0 || len(input.Participants) != 0 {
		t.Fatalf("explicit empty participants were not applied: %+v", input.Participants)
	}
	if input.ReminderLeadHours != 1 {
		t.Fatalf("explicit reminder = %d, expected 1", input.ReminderLeadHours)
	}
	if input.IncludeRequester == nil || *input.IncludeRequester {
		t.Fatalf("explicit includeRequester = %#v, expected false", input.IncludeRequester)
	}
}

func TestCalendarEventUpdateReturnsVersionConflict(t *testing.T) {
	requestCount := 0
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestCount++
			switch requestCount {
			case 1:
				return calendarToolEventsResponse(calendarToolEventDocument("event-1", "Conflicted event", "2026-07-16T14:00:00+09:00", "2026-07-16T15:00:00+09:00")), nil
			case 2:
				if request.URL.String() != "http://admind.local/calendar/api/events/event-1" {
					t.Fatalf("unexpected lookup request %s", request.URL.String())
				}
				return calendarToolEventResponse("event-1", "Conflicted event", "2026-07-16T14:00:00+09:00", "2026-07-16T15:00:00+09:00"), nil
			default:
				return calendarToolJSONStatusResponse(http.StatusConflict, `{"code":"calendar_event_version_conflict"}`), nil
			}
		})},
	}

	response, errorValue := service.invokeCalendarEventUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.update",
		Input:    []byte(`{"eventHint":"event-1","title":"Conflicted event","startISO":"2026-07-16T14:00:00+09:00","endISO":"2026-07-16T15:00:00+09:00"}`),
	})

	assertCalendarToolVersionConflict(t, response, errorValue)
	if requestCount != 3 {
		t.Fatalf("request count = %d", requestCount)
	}
}

func TestCalendarEventUpdateRejectsInvalidDirectLookupContract(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		response string
	}{
		{name: "not found", status: http.StatusNotFound, response: `{"code":"not_found"}`},
		{name: "mismatched ID", status: http.StatusOK, response: `{"id":"event-2","updatedAt":"2026-07-16T00:00:00Z"}`},
		{name: "missing version", status: http.StatusOK, response: `{"id":"event-1"}`},
		{name: "invalid version", status: http.StatusOK, response: `{"id":"event-1","updatedAt":"not-a-time"}`},
		{name: "malformed response", status: http.StatusOK, response: `{`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requestCount := 0
			service := Service{
				Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
				HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					requestCount++
					if requestCount == 1 {
						return calendarToolEventsResponse(calendarToolEventDocument("event-1", "Original title", "2026-07-16T14:00:00+09:00", "2026-07-16T15:00:00+09:00")), nil
					}
					return calendarToolJSONStatusResponse(test.status, test.response), nil
				})},
			}

			response, errorValue := service.invokeCalendarEventUpdate(context.Background(), capabilities.ToolInvokeRequest{
				ToolName: "calendar.update",
				Input:    []byte(`{"eventHint":"event-1","title":"Changed","startISO":"2026-07-16T14:00:00+09:00","endISO":"2026-07-16T15:00:00+09:00"}`),
			})

			if test.status == http.StatusNotFound {
				if errorValue != nil || response.ErrorCode != "calendar_event_not_found" {
					t.Fatalf("response=%+v error=%v", response, errorValue)
				}
			} else if errorValue == nil {
				t.Fatalf("expected contract error, response=%+v", response)
			}
			if requestCount != 2 {
				t.Fatalf("request count = %d", requestCount)
			}
		})
	}
}

func TestCalendarEventUpdateResolvesByExactEventIDAcrossAllEvents(t *testing.T) {
	requestCount := 0
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestCount++
			switch requestCount {
			case 1:
				return calendarToolEventsResponse(
					calendarToolEventDocument("event-other", "10분 회의", "2026-07-16T13:00:00+09:00", "2026-07-16T13:10:00+09:00"),
					calendarToolEventDocument("event-1", "IR 미팅", "2026-07-16T14:00:00+09:00", "2026-07-16T15:00:00+09:00"),
				), nil
			case 2:
				return calendarToolEventResponse("event-1", "IR 미팅", "2026-07-16T14:00:00+09:00", "2026-07-16T15:00:00+09:00"), nil
			case 3:
				return calendarToolEventResponse("event-1", "IR 미팅 완료", "2026-07-16T14:00:00+09:00", "2026-07-16T15:00:00+09:00"), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeCalendarEventUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.update",
		Input:    []byte(`{"eventHint":"event-1","title":"IR 미팅 완료"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.IsError {
		t.Fatalf("expected exact eventID hint to resolve, got error response %+v", response)
	}
	assertCalendarMutationEffect(t, response, "event-1", "updated")
}

func TestCalendarEventUpdateResolvesByExactUniqueTitle(t *testing.T) {
	requestCount := 0
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestCount++
			switch requestCount {
			case 1:
				return calendarToolEventsResponse(calendarToolEventDocument("event-1", "IR 미팅", "2026-07-16T14:00:00+09:00", "2026-07-16T15:00:00+09:00")), nil
			case 2:
				return calendarToolEventResponse("event-1", "IR 미팅", "2026-07-16T14:00:00+09:00", "2026-07-16T15:00:00+09:00"), nil
			case 3:
				return calendarToolEventResponse("event-1", "IR 미팅 완료", "2026-07-16T14:00:00+09:00", "2026-07-16T15:00:00+09:00"), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeCalendarEventUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.update",
		Input:    []byte(`{"eventHint":"IR 미팅","title":"IR 미팅 완료"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.IsError {
		t.Fatalf("expected exact unique title hint to resolve, got error response %+v", response)
	}
	assertCalendarMutationEffect(t, response, "event-1", "updated")
}

func TestCalendarEventUpdateAmbiguousTitleReturnsCandidatesWithoutWrite(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method != http.MethodGet {
				t.Fatalf("unexpected write request %s %s", request.Method, request.URL.String())
			}
			return calendarToolEventsResponse(
				calendarToolEventDocument("event-1", "IR 미팅", "2026-07-16T14:00:00+09:00", "2026-07-16T15:00:00+09:00"),
				calendarToolEventDocument("event-2", "IR 미팅", "2026-07-17T14:00:00+09:00", "2026-07-17T15:00:00+09:00"),
			), nil
		})},
	}

	response, errorValue := service.invokeCalendarEventUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.update",
		Input:    []byte(`{"eventHint":"IR 미팅","title":"IR 미팅 완료"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "calendar_event_hint_unresolved" || !response.SafeRetry {
		t.Fatalf("response = %+v", response)
	}
	if !strings.Contains(string(response.Result), "event-1") || !strings.Contains(string(response.Result), "event-2") {
		t.Fatalf("expected both ambiguous candidates, got result = %s", response.Result)
	}
}

func TestCalendarEventUpdateUnresolvedHintReturnsCandidatesWithoutWrite(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method != http.MethodGet {
				t.Fatalf("unexpected write request %s %s", request.Method, request.URL.String())
			}
			return calendarToolEventsResponse(calendarToolEventDocument("event-1", "IR 미팅", "2026-07-16T14:00:00+09:00", "2026-07-16T15:00:00+09:00")), nil
		})},
	}

	response, errorValue := service.invokeCalendarEventUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.update",
		Input:    []byte(`{"eventHint":"missing-event","title":"IR 미팅 완료"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "calendar_event_hint_unresolved" || !response.SafeRetry {
		t.Fatalf("response = %+v", response)
	}
	if !strings.Contains(string(response.Result), "event-1") {
		t.Fatalf("expected the requester's current events as candidates, got result = %s", response.Result)
	}
}

func TestCalendarEventUpdateHintResolutionIsCaseSensitiveAfterTrim(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method != http.MethodGet {
				t.Fatalf("unexpected write request %s %s", request.Method, request.URL.String())
			}
			return calendarToolEventsResponse(calendarToolEventDocument("event-1", "IR Meeting", "2026-07-16T14:00:00+09:00", "2026-07-16T15:00:00+09:00")), nil
		})},
	}

	response, errorValue := service.invokeCalendarEventUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.update",
		Input:    []byte(`{"eventHint":"ir meeting","title":"IR Meeting Done"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "calendar_event_hint_unresolved" {
		t.Fatalf("expected case-mismatched title to stay unresolved, got response = %+v", response)
	}
}

func TestCalendarEventUpdateHintResolutionTrimsWhitespaceBeforeMatching(t *testing.T) {
	requestCount := 0
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestCount++
			switch requestCount {
			case 1:
				return calendarToolEventsResponse(calendarToolEventDocument("event-1", "IR 미팅", "2026-07-16T14:00:00+09:00", "2026-07-16T15:00:00+09:00")), nil
			case 2:
				return calendarToolEventResponse("event-1", "IR 미팅", "2026-07-16T14:00:00+09:00", "2026-07-16T15:00:00+09:00"), nil
			case 3:
				return calendarToolEventResponse("event-1", "IR 미팅 완료", "2026-07-16T14:00:00+09:00", "2026-07-16T15:00:00+09:00"), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeCalendarEventUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.update",
		Input:    []byte(`{"eventHint":" IR 미팅 ","title":"IR 미팅 완료"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.IsError {
		t.Fatalf("expected trimmed title hint to resolve, got error response %+v", response)
	}
}

func TestCalendarEventDeleteResolvesByExactUniqueTitle(t *testing.T) {
	requestCount := 0
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestCount++
			switch requestCount {
			case 1:
				return calendarToolEventsResponse(calendarToolEventDocument("event-1", "고객지원 분기 결산 검토", "2026-07-16T14:00:00+09:00", "2026-07-16T15:00:00+09:00")), nil
			case 2:
				return calendarToolEventResponse("event-1", "고객지원 분기 결산 검토", "2026-07-16T14:00:00+09:00", "2026-07-16T15:00:00+09:00"), nil
			case 3:
				return calendarToolJSONResponse(`{"deleted":true}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeCalendarEventDelete(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.delete",
		Input:    []byte(`{"eventHint":"고객지원 분기 결산 검토"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "deleted" {
		t.Fatalf("expected exact unique title hint to resolve and delete, got response = %+v", response)
	}
}

func TestCalendarEventDeleteAmbiguousTitleReturnsCandidatesWithoutDeleting(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method != http.MethodGet {
				t.Fatalf("unexpected write request %s %s", request.Method, request.URL.String())
			}
			return calendarToolEventsResponse(
				calendarToolEventDocument("event-1", "IR 미팅", "2026-07-16T14:00:00+09:00", "2026-07-16T15:00:00+09:00"),
				calendarToolEventDocument("event-2", "IR 미팅", "2026-07-17T14:00:00+09:00", "2026-07-17T15:00:00+09:00"),
			), nil
		})},
	}

	response, errorValue := service.invokeCalendarEventDelete(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.delete",
		Input:    []byte(`{"eventHint":"IR 미팅"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "calendar_event_hint_unresolved" || !response.SafeRetry {
		t.Fatalf("response = %+v", response)
	}
	if !strings.Contains(string(response.Result), "event-1") || !strings.Contains(string(response.Result), "event-2") {
		t.Fatalf("expected both ambiguous candidates, got result = %s", response.Result)
	}
}

func TestCalendarEventDeleteNoMatchReturnsCandidatesWithoutDeleting(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method != http.MethodGet {
				t.Fatalf("unexpected write request %s %s", request.Method, request.URL.String())
			}
			return calendarToolEventsResponse(calendarToolEventDocument("event-1", "회의", "2026-07-16T14:00:00+09:00", "2026-07-16T15:00:00+09:00")), nil
		})},
	}

	response, errorValue := service.invokeCalendarEventDelete(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.delete",
		Input:    []byte(`{"eventHint":"missing-event"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "calendar_event_hint_unresolved" || !response.SafeRetry {
		t.Fatalf("response = %+v", response)
	}
	if !strings.Contains(string(response.Result), "event-1") {
		t.Fatalf("expected the requester's current events as candidates, got result = %s", response.Result)
	}
}

func TestCalendarEventDeleteRequiresEventHint(t *testing.T) {
	_, errorValue := decodeCalendarEventDeleteInput([]byte(`{}`))
	if errorValue == nil || !strings.Contains(errorValue.Error(), "eventHint") {
		t.Fatalf("expected eventHint error, got %v", errorValue)
	}
}

func TestCalendarConnectionStartToolIsNotConfigured(t *testing.T) {
	service := Service{Configuration: Configuration{AdmindBaseURL: "http://admind.local"}}

	_, errorValue := service.invokeCapabilityTool(context.Background(), "calendar.connection.start", strings.NewReader(`{}`))

	if errorValue == nil || !strings.Contains(errorValue.Error(), "capability tool is not configured") {
		t.Fatalf("expected calendar connection start to be unavailable, got %v", errorValue)
	}
}

func TestCalendarEventDeleteScheduledRunBypassesApprovalGate(t *testing.T) {
	var requesterEmail string
	requestCount := 0
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestCount++
			switch requestCount {
			case 1:
				return calendarToolEventsResponse(calendarToolEventDocument("event-1", "Scheduled event", "2026-07-16T14:00:00+09:00", "2026-07-16T15:00:00+09:00")), nil
			case 2:
				if request.Method != http.MethodGet || request.URL.String() != "http://admind.local/calendar/api/events/event-1" {
					t.Fatalf("unexpected lookup request %s %s", request.Method, request.URL.String())
				}
				return calendarToolEventResponse("event-1", "Scheduled event", "2026-07-16T14:00:00+09:00", "2026-07-16T15:00:00+09:00"), nil
			case 3:
				if request.Method != http.MethodDelete || request.URL.String() != "http://admind.local/calendar/api/events/event-1" {
					t.Fatalf("unexpected delete request %s %s", request.Method, request.URL.String())
				}
				var payload map[string]any
				if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
					t.Fatal(errorValue)
				}
				if payload["expectedUpdatedAt"] != "2026-07-16T00:00:00Z" {
					t.Fatalf("missing expected delete version %#v", payload)
				}
				requesterEmail = request.Header.Get("CF-Access-Authenticated-User-Email")
				return calendarToolJSONResponse(`{"deleted":true}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "calendar.delete", strings.NewReader(`{"input":{"eventHint":"event-1"},"context":{"requesterPersonID":"person-1","requesterEmail":"Staff@Example.com","isScheduledRun":true}}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "deleted" || response.IsError || requestCount != 3 {
		t.Fatalf("expected scheduled delete to execute, got %+v", response)
	}
	assertCalendarMutationEffect(t, response, "event-1", "deleted")
	if requesterEmail != "staff@example.com" {
		t.Fatalf("requesterEmail = %q", requesterEmail)
	}
}

func TestCalendarEventDeleteReturnsVersionConflict(t *testing.T) {
	requestCount := 0
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestCount++
			switch requestCount {
			case 1:
				return calendarToolEventsResponse(calendarToolEventDocument("event-1", "Conflicted event", "2026-07-16T14:00:00+09:00", "2026-07-16T15:00:00+09:00")), nil
			case 2:
				if request.URL.String() != "http://admind.local/calendar/api/events/event-1" {
					t.Fatalf("unexpected lookup request %s", request.URL.String())
				}
				return calendarToolEventResponse("event-1", "Conflicted event", "2026-07-16T14:00:00+09:00", "2026-07-16T15:00:00+09:00"), nil
			default:
				return calendarToolJSONStatusResponse(http.StatusConflict, `{"code":"calendar_event_version_conflict"}`), nil
			}
		})},
	}

	response, errorValue := service.invokeCalendarEventDelete(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.delete",
		Input:    []byte(`{"eventHint":"event-1"}`),
	})

	assertCalendarToolVersionConflict(t, response, errorValue)
	if requestCount != 3 {
		t.Fatalf("request count = %d", requestCount)
	}
}

func calendarToolJSONResponse(document string) *http.Response {
	return calendarToolJSONStatusResponse(http.StatusOK, document)
}

func calendarToolEventResponse(eventID string, title string, startISO string, endISO string) *http.Response {
	document, _ := json.Marshal(calendarToolEventDocument(eventID, title, startISO, endISO))
	return calendarToolJSONResponse(string(document))
}

func calendarToolEventsResponse(events ...map[string]any) *http.Response {
	document, _ := json.Marshal(map[string]any{"events": events})
	return calendarToolJSONResponse(string(document))
}

func calendarToolEventDocument(eventID string, title string, startISO string, endISO string) map[string]any {
	return map[string]any{
		"id":                eventID,
		"title":             title,
		"description":       "",
		"location":          "",
		"startISO":          startISO,
		"endISO":            endISO,
		"timeZone":          "Asia/Seoul",
		"isAllDay":          false,
		"color":             "",
		"people":            []string{},
		"participants":      []calendarToolParticipant{},
		"reminderLeadHours": 24,
		"updatedAt":         "2026-07-16T00:00:00Z",
	}
}

func assertCalendarMutationEffect(t *testing.T, response capabilities.ToolInvokeResponse, eventID string, effect string) {
	t.Helper()
	if response.Outcome != capabilities.ToolOutcomeSucceeded || len(response.Effects) != 1 {
		t.Fatalf("mutation response = %+v", response)
	}
	resourceEffect := response.Effects[0]
	if resourceEffect.ObjectType != "calendar" || resourceEffect.Effect != effect || resourceEffect.ID != eventID {
		t.Fatalf("mutation effect = %+v", resourceEffect)
	}
}

func assertCalendarToolVersionConflict(t *testing.T, response capabilities.ToolInvokeResponse, errorValue error) {
	t.Helper()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "calendar_event_version_conflict" || response.FailureStage != "persistence" {
		t.Fatalf("response = %+v", response)
	}
	if !response.Retryable || response.SafeRetry {
		t.Fatalf("version conflict retry contract = %+v", response)
	}
}

func calendarToolJSONStatusResponse(statusCode int, document string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(document)),
	}
}

func TestCompleteCalendarListRangeDefaultsMissingEnd(t *testing.T) {
	startISO, endISO := completeCalendarListRange("2026-07-23T00:00:00+09:00", "")
	if startISO != "2026-07-23T00:00:00+09:00" || endISO != "2026-07-24T00:00:00+09:00" {
		t.Fatalf("unexpected range: %s %s", startISO, endISO)
	}
	startISO, endISO = completeCalendarListRange("", "2026-07-23T00:00:00+09:00")
	if startISO != "2026-07-22T00:00:00+09:00" || endISO != "2026-07-23T00:00:00+09:00" {
		t.Fatalf("unexpected backfilled range: %s %s", startISO, endISO)
	}
}
