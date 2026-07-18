package admind

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCalendarCreateReturnsPersistedEventVersion(t *testing.T) {
	service := newCalendarTestService(t)
	payload := calendarEventWriteRequest{
		EventID:  "created-event-version",
		Title:    "Created title",
		StartISO: "2026-07-16T01:00:00Z",
		EndISO:   "2026-07-16T02:00:00Z",
		TimeZone: "UTC",
		Color:    "#2563eb",
	}
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPost, "/calendar/api/events", strings.NewReader(string(document)))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d body = %s", response.Code, response.Body.String())
	}
	var createdEvent calendarEvent
	if errorValue := json.Unmarshal(response.Body.Bytes(), &createdEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := time.Parse(time.RFC3339Nano, createdEvent.UpdatedAt); errorValue != nil {
		t.Fatalf("created event updatedAt = %q error=%v", createdEvent.UpdatedAt, errorValue)
	}
	storedEvent := readRequiredCalendarEvent(t, service, createdEvent.ID)
	if storedEvent.UpdatedAt != createdEvent.UpdatedAt {
		t.Fatalf("stored updatedAt = %q response updatedAt = %q", storedEvent.UpdatedAt, createdEvent.UpdatedAt)
	}
}

func TestCalendarGetEventReturnsPersistedVersion(t *testing.T) {
	service := newCalendarTestService(t)
	event := newLocalTestCalendarEvent("single-event-version", "Single event")
	if errorValue := service.writeCalendarEventWithSource(context.Background(), event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodGet, "/calendar/api/events/"+event.ID, nil)
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("get status = %d body = %s", response.Code, response.Body.String())
	}
	var returnedEvent calendarEvent
	if errorValue := json.Unmarshal(response.Body.Bytes(), &returnedEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	storedEvent := readRequiredCalendarEvent(t, service, event.ID)
	if returnedEvent.ID != event.ID || returnedEvent.UpdatedAt != storedEvent.UpdatedAt {
		t.Fatalf("returned event = %+v", returnedEvent)
	}
}

func TestCalendarGetEventReturnsNotFound(t *testing.T) {
	service := newCalendarTestService(t)
	request := httptest.NewRequest(http.MethodGet, "/calendar/api/events/missing-event", nil)
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("get status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestCalendarGetEventTreatsEscapedActorImageSuffixAsEventID(t *testing.T) {
	service := newCalendarTestService(t)
	event := newLocalTestCalendarEvent("event/actor-image", "Escaped actor image suffix")
	if errorValue := service.writeCalendarEventWithSource(context.Background(), event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodGet, "/calendar/api/events/event%2Factor-image", nil)
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("get status = %d body = %s", response.Code, response.Body.String())
	}
	var returnedEvent calendarEvent
	if errorValue := json.Unmarshal(response.Body.Bytes(), &returnedEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	if returnedEvent.ID != event.ID {
		t.Fatalf("returned event ID = %q", returnedEvent.ID)
	}
}

func TestCalendarUpdateRejectsStaleEventVersion(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	event := newLocalTestCalendarEvent("stale-web-update", "Original title")
	if errorValue := service.writeCalendarEventWithSource(contextValue, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	originalEvent := readRequiredCalendarEvent(t, service, event.ID)

	updatedEvent := updateCalendarEventThroughHTTP(t, service, originalEvent, "Newer title", originalEvent.UpdatedAt)
	staleResponse := sendCalendarEventUpdate(t, service, originalEvent, "Stale title", originalEvent.UpdatedAt)

	if staleResponse.Code != http.StatusConflict {
		t.Fatalf("stale update status = %d body = %s", staleResponse.Code, staleResponse.Body.String())
	}
	var responseBody struct {
		Code string `json:"code"`
	}
	if errorValue := json.Unmarshal(staleResponse.Body.Bytes(), &responseBody); errorValue != nil {
		t.Fatal(errorValue)
	}
	if responseBody.Code != calendarEventVersionConflictErrorCode {
		t.Fatalf("stale update code = %q", responseBody.Code)
	}
	storedEvent := readRequiredCalendarEvent(t, service, event.ID)
	if storedEvent.Title != updatedEvent.Title || storedEvent.UpdatedAt != updatedEvent.UpdatedAt {
		t.Fatalf("stored event after stale update = %+v", storedEvent)
	}
}

func TestCalendarStaleUpdateCannotRestoreDeletedEvent(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	event := newLocalTestCalendarEvent("deleted-before-web-update", "Original title")
	if errorValue := service.writeCalendarEventWithSource(contextValue, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	originalEvent := readRequiredCalendarEvent(t, service, event.ID)
	if errorValue := service.softDeleteCalendarEventWithSource(contextValue, event.ID, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	originalEvent.Title = "Stale title"

	errorValue := service.writeCalendarEventIfCurrentVersionWithOrigin(contextValue, originalEvent, originalEvent.UpdatedAt, nil)
	if !errors.Is(errorValue, errCalendarEventVersionConflict) {
		t.Fatalf("stale update error = %v", errorValue)
	}
	projection, found, errorValue := service.readCalendarEventProjectionByID(contextValue, event.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found || !projection.IsDeleted || projection.Event.Title != "Original title" {
		t.Fatalf("deleted event projection = %+v found=%v", projection, found)
	}
}

func TestCalendarDeleteRejectsStaleEventVersion(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	event := newLocalTestCalendarEvent("stale-web-delete", "Original title")
	if errorValue := service.writeCalendarEventWithSource(contextValue, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	originalEvent := readRequiredCalendarEvent(t, service, event.ID)
	updatedEvent := updateCalendarEventThroughHTTP(t, service, originalEvent, "Newer title", originalEvent.UpdatedAt)

	staleResponse := sendCalendarEventDelete(t, service, event.ID, originalEvent.UpdatedAt)
	if staleResponse.Code != http.StatusConflict {
		t.Fatalf("stale delete status = %d body = %s", staleResponse.Code, staleResponse.Body.String())
	}
	storedEvent := readRequiredCalendarEvent(t, service, event.ID)
	if storedEvent.Title != "Newer title" {
		t.Fatalf("stored event after stale delete = %+v", storedEvent)
	}

	currentResponse := sendCalendarEventDelete(t, service, event.ID, updatedEvent.UpdatedAt)
	if currentResponse.Code != http.StatusNoContent {
		t.Fatalf("current delete status = %d body = %s", currentResponse.Code, currentResponse.Body.String())
	}
	if _, found, errorValue := service.readCalendarEventByID(contextValue, event.ID); errorValue != nil || found {
		t.Fatalf("deleted event found=%v error=%v", found, errorValue)
	}
}

func TestCalendarUpdateAcceptsLegacyPayloadWithoutExpectedVersion(t *testing.T) {
	service := newCalendarTestService(t)
	event := newLocalTestCalendarEvent("legacy-web-update", "Original title")
	if errorValue := service.writeCalendarEventWithSource(context.Background(), event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	storedEvent := readRequiredCalendarEvent(t, service, event.ID)

	response := sendCalendarEventUpdate(t, service, storedEvent, "Legacy title", "")

	if response.Code != http.StatusOK {
		t.Fatalf("legacy update status = %d body = %s", response.Code, response.Body.String())
	}
	if updatedEvent := readRequiredCalendarEvent(t, service, event.ID); updatedEvent.Title != "Legacy title" {
		t.Fatalf("updated event = %+v", updatedEvent)
	}
}

func TestCalendarDeleteAcceptsLegacyRequestWithoutBody(t *testing.T) {
	service := newCalendarTestService(t)
	event := newLocalTestCalendarEvent("legacy-web-delete", "Original title")
	if errorValue := service.writeCalendarEventWithSource(context.Background(), event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodDelete, "/calendar/api/events/"+event.ID, nil)
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("legacy delete status = %d body = %s", response.Code, response.Body.String())
	}
	if _, found, errorValue := service.readCalendarEventByID(context.Background(), event.ID); errorValue != nil || found {
		t.Fatalf("deleted event found=%v error=%v", found, errorValue)
	}
}

func updateCalendarEventThroughHTTP(t *testing.T, service *Service, event calendarEvent, title string, expectedUpdatedAt string) calendarEvent {
	t.Helper()
	response := sendCalendarEventUpdate(t, service, event, title, expectedUpdatedAt)
	if response.Code != http.StatusOK {
		t.Fatalf("update status = %d body = %s", response.Code, response.Body.String())
	}
	var updatedEvent calendarEvent
	if errorValue := json.Unmarshal(response.Body.Bytes(), &updatedEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	return updatedEvent
}

func sendCalendarEventUpdate(t *testing.T, service *Service, event calendarEvent, title string, expectedUpdatedAt string) *httptest.ResponseRecorder {
	t.Helper()
	payload := calendarEventWriteRequest{
		EventID:           event.ID,
		Title:             title,
		Description:       event.Description,
		Location:          event.Location,
		StartISO:          event.StartISO,
		EndISO:            event.EndISO,
		TimeZone:          event.TimeZone,
		IsAllDay:          event.IsAllDay,
		Color:             event.Color,
		Participants:      calendarParticipantIdentities(event.Participants),
		ReminderLeadHours: event.ReminderLeadHours,
		ExpectedUpdatedAt: expectedUpdatedAt,
	}
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPut, "/calendar/api/events/"+event.ID, strings.NewReader(string(document)))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	return response
}

func sendCalendarEventDelete(t *testing.T, service *Service, eventID string, expectedUpdatedAt string) *httptest.ResponseRecorder {
	t.Helper()
	document, errorValue := json.Marshal(calendarEventDeleteRequest{ExpectedUpdatedAt: expectedUpdatedAt})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodDelete, "/calendar/api/events/"+eventID, strings.NewReader(string(document)))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	return response
}

func readRequiredCalendarEvent(t *testing.T, service *Service, eventID string) calendarEvent {
	t.Helper()
	event, found, errorValue := service.readCalendarEventByID(context.Background(), eventID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found {
		t.Fatalf("calendar event %q not found", eventID)
	}
	return event
}
