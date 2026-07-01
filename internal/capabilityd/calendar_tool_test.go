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
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
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
	if len(people) != 2 || people[0] != "동하" || people[1] != "수민" || payload["reminderLeadHours"] != float64(48) {
		t.Fatalf("calendar metadata payload = %#v", payload)
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
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method != http.MethodDelete || request.URL.String() != "http://admind.local/calendar/api/events/event-1" {
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			}
			requesterEmail = request.Header.Get("CF-Access-Authenticated-User-Email")
			return calendarToolJSONResponse(`{"deleted":true}`), nil
		})},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "calendar.delete", strings.NewReader(`{"input":{"eventID":"event-1"},"context":{"requesterPersonID":"person-1","requesterEmail":"Staff@Example.com","isScheduledRun":true}}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "deleted" || response.IsError {
		t.Fatalf("expected scheduled delete to execute, got %+v", response)
	}
	if requesterEmail != "staff@example.com" {
		t.Fatalf("requesterEmail = %q", requesterEmail)
	}
}

func calendarToolJSONResponse(document string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(document)),
	}
}
