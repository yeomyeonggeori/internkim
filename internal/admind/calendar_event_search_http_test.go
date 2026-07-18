package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestCalendarEventSearchReturnsAtMostTwoMatches(t *testing.T) {
	service := newCalendarTestService(t)
	for _, event := range []calendarEvent{
		calendarTestEvent("search-alpha-1", "Alpha planning", ""),
		calendarTestEvent("search-alpha-2", "Second", "Discuss ALPHA launch"),
		calendarTestEvent("search-alpha-3", "Third", ""),
	} {
		if event.ID == "search-alpha-3" {
			event.Location = "alpha room"
		}
		if errorValue := service.writeCalendarEvent(context.Background(), event); errorValue != nil {
			t.Fatal(errorValue)
		}
	}

	response := searchCalendarEventsForTest(t, service, "alpha")
	if len(response.Events) != 2 {
		t.Fatalf("event count = %d, expected 2", len(response.Events))
	}
}

func TestCalendarEventSearchTreatsWildcardsAsLiteralText(t *testing.T) {
	service := newCalendarTestService(t)
	literal := calendarTestEvent("search-literal", "Budget 100%_review", "")
	wildcardOnly := calendarTestEvent("search-wildcard", "Budget 100XXreview", "")
	for _, event := range []calendarEvent{literal, wildcardOnly} {
		if errorValue := service.writeCalendarEvent(context.Background(), event); errorValue != nil {
			t.Fatal(errorValue)
		}
	}

	response := searchCalendarEventsForTest(t, service, "%_")
	if len(response.Events) != 1 || response.Events[0].ID != literal.ID {
		t.Fatalf("events = %+v", response.Events)
	}
}

func TestCalendarEventSearchRejectsEmptyQuery(t *testing.T) {
	service := newCalendarTestService(t)
	request := httptest.NewRequest(http.MethodGet, "/calendar/api/events/search", nil)
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func searchCalendarEventsForTest(t *testing.T, service *Service, query string) calendarEventsResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/calendar/api/events/search?query="+url.QueryEscape(query), nil)
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	var document calendarEventsResponse
	if errorValue := json.NewDecoder(response.Body).Decode(&document); errorValue != nil {
		t.Fatal(errorValue)
	}
	return document
}
