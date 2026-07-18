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
			return calendarToolJSONResponse(`{"id":"event-1","title":"Demo","startISO":"2026-05-08T01:00:00Z","endISO":"2026-05-08T02:00:00Z"}`), nil
		})},
	}

	response, errorValue := service.invokeCalendarEventAdd(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.add",
		Input:    []byte(`{"title":"Demo","startISO":"2026-05-08T10:00:00+09:00","endISO":"2026-05-08T11:00:00+09:00","location":"Office","people":"동하, 수민","reminderLeadHours":48}`),
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
	if len(people) != 3 || people[0] != "Staff" || people[1] != "동하" || people[2] != "수민" || payload["reminderLeadHours"] != float64(48) {
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
					return calendarToolJSONResponse(`{"status":"resolved","recipient":{"personID":"person-staff","displayName":"김여명","emails":["staff@example.com"],"externalUserID":"user-staff","username":"yeomyeong"}}`), nil
				case "우경":
					return calendarToolJSONResponse(`{"status":"resolved","recipient":{"personID":"person-rain","displayName":"신우경","emails":["rain@example.com"],"externalUserID":"user-rain","username":"rain"}}`), nil
				default:
					return calendarToolJSONResponse(`{"status":"not_found"}`), nil
				}
			case request.Method == http.MethodPost && request.URL.String() == "http://admind.local/calendar/api/events":
				if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
					t.Fatal(errorValue)
				}
				return calendarToolJSONResponse(`{"id":"event-1","title":"경산 일정"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeCalendarEventAdd(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.add",
		Input:    []byte(`{"title":"경산 일정","startISO":"2026-05-08T05:00:00+09:00","endISO":"2026-05-08T06:00:00+09:00","people":["우경"]}`),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:    "staff@example.com",
			RequesterName:     "김여명",
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
	if len(people) != 2 || people[0] != "김여명" || people[1] != "신우경" {
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
			input:                   calendarEventWriteInput{People: calendarToolPeopleInput{"우경"}},
			includeRequesterDefault: true,
			expected:                true,
		},
		{
			name:                    "calendar update default excludes requester",
			input:                   calendarEventWriteInput{People: calendarToolPeopleInput{"우경"}},
			includeRequesterDefault: false,
			expected:                false,
		},
		{
			name:                    "explicit false excludes requester",
			input:                   calendarEventWriteInput{People: calendarToolPeopleInput{"우경"}, IncludeRequester: &excludeRequester},
			includeRequesterDefault: true,
			expected:                false,
		},
		{
			name:                    "explicit true includes requester",
			input:                   calendarEventWriteInput{People: calendarToolPeopleInput{"우경"}, IncludeRequester: &includeRequester},
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
	first, firstError := decodeCalendarEventWriteInput([]byte(`{"title":"휴가","startISO":"2026-05-10T00:00:00Z","endISO":"2026-05-13T00:00:00Z","isAllDay":true}`), false)
	second, secondError := decodeCalendarEventWriteInput([]byte(`{"title":"휴가","startISO":"2026-05-10T00:00:00Z","endISO":"2026-05-13T00:00:00Z","isAllDay":true}`), false)
	if firstError != nil || secondError != nil {
		t.Fatalf("decode errors: %v %v", firstError, secondError)
	}
	if first.EventID == "" || first.EventID != second.EventID {
		t.Fatalf("event IDs = %q %q", first.EventID, second.EventID)
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
			return calendarToolJSONResponse(`{"events":[{"id":"event-1","title":"Design review","description":"","location":"Office"},{"id":"event-2","title":"Lunch","description":"","location":"Cafe"}]}`), nil
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
	if len(document.Events) != 1 || document.Events[0].ID != "event-1" {
		t.Fatalf("events = %#v", document.Events)
	}
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

func TestCalendarEventUpdateUsesUnchangedTitleAsTarget(t *testing.T) {
	requestCount := 0
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestCount++
			switch requestCount {
			case 1:
				if request.Method != http.MethodGet || request.URL.String() != "http://admind.local/calendar/api/events/search?query=%EB%B9%84%EC%9A%A9+%ED%85%8C%EC%8A%A4%ED%8A%B8+%EC%9D%BC%EC%A0%95" {
					t.Fatalf("unexpected lookup request %s %s", request.Method, request.URL.String())
				}
				return calendarToolJSONResponse(`{"events":[{"id":"event-1","title":"비용 테스트 일정","updatedAt":"2026-07-16T00:00:00Z"}]}`), nil
			case 2:
				if request.Method != http.MethodPut || request.URL.String() != "http://admind.local/calendar/api/events/event-1" {
					t.Fatalf("unexpected update request %s %s", request.Method, request.URL.String())
				}
				var payload map[string]any
				if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
					t.Fatal(errorValue)
				}
				if payload["startISO"] != "2026-07-16T14:00:00+09:00" {
					t.Fatalf("unexpected update payload %#v", payload)
				}
				if payload["expectedUpdatedAt"] != "2026-07-16T00:00:00Z" {
					t.Fatalf("missing expected update version %#v", payload)
				}
				return calendarToolJSONResponse(`{"id":"event-1","title":"비용 테스트 일정","startISO":"2026-07-16T14:00:00+09:00","endISO":"2026-07-16T15:00:00+09:00"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeCalendarEventUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.update",
		Input:    []byte(`{"title":"비용 테스트 일정","startISO":"2026-07-16T14:00:00+09:00","endISO":"2026-07-16T15:00:00+09:00"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "updated" || requestCount != 2 {
		t.Fatalf("response = %+v requests = %d", response, requestCount)
	}
}

func TestCalendarEventQueryResolutionHandlesSearchResults(t *testing.T) {
	tests := []struct {
		name              string
		response          string
		expectedErrorCode string
	}{
		{name: "not found", response: `{"events":[]}`, expectedErrorCode: "calendar_event_not_found"},
		{name: "ambiguous", response: `{"events":[{"id":"event-1","title":"Alpha","updatedAt":"2026-07-16T00:00:00Z"},{"id":"event-2","title":"Alpha two","updatedAt":"2026-07-16T00:00:00Z"}]}`, expectedErrorCode: "calendar_event_ambiguous"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			service := Service{
				Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
				HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					if request.URL.String() != "http://admind.local/calendar/api/events/search?query=Alpha" {
						t.Fatalf("unexpected request %s", request.URL.String())
					}
					return calendarToolJSONResponse(testCase.response), nil
				})},
			}

			response, errorValue := service.invokeCalendarEventDelete(context.Background(), capabilities.ToolInvokeRequest{
				ToolName: "calendar.delete",
				Input:    []byte(`{"query":"Alpha"}`),
			})

			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if response.ErrorCode != testCase.expectedErrorCode {
				t.Fatalf("error code = %q, expected %q", response.ErrorCode, testCase.expectedErrorCode)
			}
		})
	}
}

func TestCalendarEventQueryResolutionFallsBackWhenSearchRouteIsUnavailable(t *testing.T) {
	requestCount := 0
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requestCount++
			if requestCount == 1 {
				if request.URL.String() != "http://admind.local/calendar/api/events/search?query=Legacy" {
					t.Fatalf("unexpected search request %s", request.URL.String())
				}
				return calendarToolJSONStatusResponse(http.StatusNotFound, `404 page not found`), nil
			}
			if requestCount == 2 {
				if request.URL.String() != "http://admind.local/calendar/api/events" {
					t.Fatalf("unexpected fallback request %s", request.URL.String())
				}
				return calendarToolJSONResponse(`{"events":[{"id":"event-1","title":"Legacy meeting","startISO":"2026-07-16T14:00:00+09:00","endISO":"2026-07-16T15:00:00+09:00","updatedAt":"2026-07-16T00:00:00Z"}]}`), nil
			}
			if request.Method != http.MethodDelete || request.URL.String() != "http://admind.local/calendar/api/events/event-1" {
				t.Fatalf("unexpected delete request %s %s", request.Method, request.URL.String())
			}
			return calendarToolJSONResponse(`{"deleted":true}`), nil
		})},
	}

	response, errorValue := service.invokeCalendarEventDelete(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.delete",
		Input:    []byte(`{"query":"Legacy"}`),
	})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "deleted" || requestCount != 3 {
		t.Fatalf("response = %+v request count = %d", response, requestCount)
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
			case 2:
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
				return calendarToolJSONResponse(`{"id":"event-1","title":"Changed title"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeCalendarEventUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.update",
		Input:    []byte(`{"eventID":"event-1","title":"Changed title"}`),
	})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "updated" || requestCount != 2 {
		t.Fatalf("response = %+v requests = %d", response, requestCount)
	}
}

func TestCalendarEventUpdateAppliesExplicitEmptyAndFalseFields(t *testing.T) {
	current := calendarEventForTool{
		ID:                "event-1",
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
	merged, errorValue := mergeCalendarEventUpdateInput([]byte(`{
		"description":"",
		"location":"",
		"isAllDay":false,
		"participants":[],
		"reminderLeadHours":0,
		"includeRequester":false
	}`), current)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	input, errorValue := decodeCalendarEventWriteInput(merged, true)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if input.Description != "" || input.Location != "" || input.IsAllDay {
		t.Fatalf("explicit empty and false fields were not applied: %+v", input)
	}
	if len(input.People) != 0 || len(input.Participants) != 0 {
		t.Fatalf("explicit empty participants were not applied: %+v", input.Participants)
	}
	if input.ReminderLeadHours != 24 {
		t.Fatalf("explicit default reminder = %d, expected 24", input.ReminderLeadHours)
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
			if requestCount == 1 {
				if request.URL.String() != "http://admind.local/calendar/api/events/event-1" {
					t.Fatalf("unexpected lookup request %s", request.URL.String())
				}
				return calendarToolJSONResponse(`{"id":"event-1","title":"Conflicted event","updatedAt":"2026-07-16T00:00:00Z"}`), nil
			}
			return calendarToolJSONStatusResponse(http.StatusConflict, `{"code":"calendar_event_version_conflict"}`), nil
		})},
	}

	response, errorValue := service.invokeCalendarEventUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.update",
		Input:    []byte(`{"eventID":"event-1","title":"Conflicted event","startISO":"2026-07-16T14:00:00+09:00","endISO":"2026-07-16T15:00:00+09:00"}`),
	})

	assertCalendarToolVersionConflict(t, response, errorValue)
	if requestCount != 2 {
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
					return calendarToolJSONStatusResponse(test.status, test.response), nil
				})},
			}

			response, errorValue := service.invokeCalendarEventUpdate(context.Background(), capabilities.ToolInvokeRequest{
				ToolName: "calendar.update",
				Input:    []byte(`{"eventID":"event-1","title":"Changed","startISO":"2026-07-16T14:00:00+09:00","endISO":"2026-07-16T15:00:00+09:00"}`),
			})

			if test.status == http.StatusNotFound {
				if errorValue != nil || response.ErrorCode != "calendar_event_not_found" {
					t.Fatalf("response=%+v error=%v", response, errorValue)
				}
			} else if errorValue == nil {
				t.Fatalf("expected contract error, response=%+v", response)
			}
			if requestCount != 1 {
				t.Fatalf("request count = %d", requestCount)
			}
		})
	}
}

func TestCalendarEventDeleteRequiresEventID(t *testing.T) {
	_, errorValue := decodeCalendarEventDeleteInput([]byte(`{}`))
	if errorValue == nil || !strings.Contains(errorValue.Error(), "eventID") {
		t.Fatalf("expected eventID error, got %v", errorValue)
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
				if request.Method != http.MethodGet || request.URL.String() != "http://admind.local/calendar/api/events/event-1" {
					t.Fatalf("unexpected lookup request %s %s", request.Method, request.URL.String())
				}
				return calendarToolJSONResponse(`{"id":"event-1","title":"Scheduled event","updatedAt":"2026-07-16T00:00:00Z"}`), nil
			case 2:
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

	response, errorValue := service.invokeCapabilityTool(context.Background(), "calendar.delete", strings.NewReader(`{"input":{"eventID":"event-1"},"context":{"requesterPersonID":"person-1","requesterEmail":"Staff@Example.com","isScheduledRun":true}}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "deleted" || response.IsError || requestCount != 2 {
		t.Fatalf("expected scheduled delete to execute, got %+v", response)
	}
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
			if requestCount == 1 {
				if request.URL.String() != "http://admind.local/calendar/api/events/event-1" {
					t.Fatalf("unexpected lookup request %s", request.URL.String())
				}
				return calendarToolJSONResponse(`{"id":"event-1","title":"Conflicted event","updatedAt":"2026-07-16T00:00:00Z"}`), nil
			}
			return calendarToolJSONStatusResponse(http.StatusConflict, `{"code":"calendar_event_version_conflict"}`), nil
		})},
	}

	response, errorValue := service.invokeCalendarEventDelete(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "calendar.delete",
		Input:    []byte(`{"eventID":"event-1"}`),
	})

	assertCalendarToolVersionConflict(t, response, errorValue)
	if requestCount != 2 {
		t.Fatalf("request count = %d", requestCount)
	}
}

func calendarToolJSONResponse(document string) *http.Response {
	return calendarToolJSONStatusResponse(http.StatusOK, document)
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
