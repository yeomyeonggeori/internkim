package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCalendarUpdateStoresMutationOriginForResultingVersion(t *testing.T) {
	service := newCalendarTestService(t)
	event := seedCalendarDeleteIntentEvent(t, service, "mutation-origin-resulting-version")

	response := sendCalendarMutationUpdate(t, service, event, "Browser update", event.UpdatedAt, "page-a", 7, true, true)
	if response.Code != http.StatusOK {
		t.Fatalf("update status = %d body = %s", response.Code, response.Body.String())
	}
	var updatedEvent calendarEvent
	if errorValue := json.Unmarshal(response.Body.Bytes(), &updatedEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	origin := readCalendarMutationOrigin(t, service, event.ID)
	if origin.resultingUpdatedAt != updatedEvent.UpdatedAt || origin.clientID != "page-a" || origin.sequence != 7 {
		t.Fatalf("origin = %+v updatedAt = %q", origin, updatedEvent.UpdatedAt)
	}
}

func TestCalendarNoOpUpdateDoesNotReplaceMutationOrigin(t *testing.T) {
	service := newCalendarTestService(t)
	event := seedCalendarDeleteIntentEvent(t, service, "mutation-origin-no-op")
	changedResponse := sendCalendarMutationUpdate(t, service, event, "Browser update", event.UpdatedAt, "page-a", 1, true, true)
	if changedResponse.Code != http.StatusOK {
		t.Fatalf("changed update status = %d body = %s", changedResponse.Code, changedResponse.Body.String())
	}
	var changedEvent calendarEvent
	if errorValue := json.Unmarshal(changedResponse.Body.Bytes(), &changedEvent); errorValue != nil {
		t.Fatal(errorValue)
	}

	noOpResponse := sendCalendarMutationUpdate(t, service, changedEvent, changedEvent.Title, changedEvent.UpdatedAt, "page-a", 2, true, true)
	if noOpResponse.Code != http.StatusOK {
		t.Fatalf("no-op update status = %d body = %s", noOpResponse.Code, noOpResponse.Body.String())
	}
	origin := readCalendarMutationOrigin(t, service, event.ID)
	if origin.resultingUpdatedAt != changedEvent.UpdatedAt || origin.clientID != "page-a" || origin.sequence != 1 {
		t.Fatalf("origin after no-op = %+v", origin)
	}
}

func TestCalendarRemoteWriteClearsBrowserMutationOrigin(t *testing.T) {
	service := newCalendarTestService(t)
	event := seedCalendarDeleteIntentEvent(t, service, "mutation-origin-remote-clear")
	response := sendCalendarMutationUpdate(t, service, event, "Browser update", event.UpdatedAt, "page-a", 1, true, true)
	if response.Code != http.StatusOK {
		t.Fatalf("browser update status = %d body = %s", response.Code, response.Body.String())
	}
	var browserEvent calendarEvent
	if errorValue := json.Unmarshal(response.Body.Bytes(), &browserEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	readCalendarMutationOrigin(t, service, event.ID)

	remoteEvent := browserEvent
	remoteEvent.Title = "Remote update"
	if errorValue := service.writeCalendarEventWithSource(context.Background(), remoteEvent, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	if count := calendarMutationOriginCount(t, service, event.ID); count != 0 {
		t.Fatalf("origin count after remote write = %d, want 0", count)
	}
}

func TestCalendarUpdateWithoutMutationIdentityClearsOriginAndConflictsPendingIntent(t *testing.T) {
	service := newCalendarTestService(t)
	event := seedCalendarDeleteIntentEvent(t, service, "mutation-origin-identityless-update")
	browserResponse := sendCalendarMutationUpdate(t, service, event, "Browser update", event.UpdatedAt, "page-a", 1, true, true)
	if browserResponse.Code != http.StatusOK {
		t.Fatalf("browser update status = %d body = %s", browserResponse.Code, browserResponse.Body.String())
	}
	var browserEvent calendarEvent
	if errorValue := json.Unmarshal(browserResponse.Body.Bytes(), &browserEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	intent := createCalendarDeleteIntentForTest(t, service, browserEvent, "identityless-update-operation", "page-a", 2, time.Now().UTC().Add(-calendarDeleteIntentDelay-time.Second))

	identitylessResponse := sendCalendarMutationUpdate(t, service, browserEvent, "Identityless update", browserEvent.UpdatedAt, "", 0, false, false)
	if identitylessResponse.Code != http.StatusOK {
		t.Fatalf("identityless update status = %d body = %s", identitylessResponse.Code, identitylessResponse.Body.String())
	}
	if count := calendarMutationOriginCount(t, service, event.ID); count != 0 {
		t.Fatalf("origin count after identityless update = %d, want 0", count)
	}
	if errorValue := service.processDueCalendarDeleteIntents(context.Background(), time.Now().UTC()); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, found, errorValue := service.readCalendarEventByID(context.Background(), event.ID); errorValue != nil || !found {
		t.Fatalf("event after identityless conflict found = %v error = %v", found, errorValue)
	}
	assertCalendarDeleteIntentStatus(t, service, intent.OperationID, calendarDeleteIntentStatusConflicted)
}

func TestCalendarEventMutationWithOriginRevalidatesExpectedVersionInWriteTransaction(t *testing.T) {
	service := newCalendarTestService(t)
	event := seedCalendarDeleteIntentEvent(t, service, "mutation-origin-transaction-revalidation")
	newerEvent := event
	newerEvent.Title = "Newer event"
	if errorValue := service.writeCalendarEventWithSource(context.Background(), newerEvent, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}

	staleEvent := event
	staleEvent.Title = "Stale browser event"
	candidateUpdatedAt, errorValue := service.reserveCalendarConflictCandidateTime(context.Background(), time.Now().UTC())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	service.calendarStoreWriteMutex.Lock()
	errorValue = service.writeCalendarEventWithSourceLockedAndOriginIfCurrent(
		context.Background(),
		staleEvent,
		calendarSourceLocal,
		candidateUpdatedAt,
		&calendarMutationOrigin{ClientID: "page-a", Sequence: 1},
		event.UpdatedAt,
	)
	service.calendarStoreWriteMutex.Unlock()
	if !errors.Is(errorValue, errCalendarEventVersionConflict) {
		t.Fatalf("stale transaction write error = %v", errorValue)
	}
	storedEvent := readRequiredCalendarEvent(t, service, event.ID)
	if storedEvent.Title != newerEvent.Title {
		t.Fatalf("stored title = %q, want %q", storedEvent.Title, newerEvent.Title)
	}
}

func TestCalendarUpdateValidatesMutationIdentityPair(t *testing.T) {
	service := newCalendarTestService(t)
	testCases := []struct {
		name            string
		includeClientID bool
		includeSequence bool
		clientID        string
		sequence        int64
	}{
		{name: "client only", includeClientID: true, clientID: "page-a"},
		{name: "sequence only", includeSequence: true, sequence: 1},
		{name: "zero sequence", includeClientID: true, includeSequence: true, clientID: "page-a", sequence: 0},
		{name: "negative sequence", includeClientID: true, includeSequence: true, clientID: "page-a", sequence: -1},
		{name: "empty client", includeClientID: true, includeSequence: true, clientID: "", sequence: 1},
	}
	for index, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			event := seedCalendarDeleteIntentEvent(t, service, "mutation-origin-validation-"+string(rune('a'+index)))
			response := sendCalendarMutationUpdate(t, service, event, "Changed", event.UpdatedAt, testCase.clientID, testCase.sequence, testCase.includeClientID, testCase.includeSequence)
			assertCalendarErrorCode(t, response, http.StatusBadRequest, "calendar_mutation_invalid_request")
		})
	}
}

func TestCalendarMutationErrorsUseStableCodes(t *testing.T) {
	service := newCalendarTestService(t)
	event := seedCalendarDeleteIntentEvent(t, service, "mutation-origin-stable-errors")

	malformedRequest := httptest.NewRequest(http.MethodPut, "/calendar/api/events/"+event.ID, strings.NewReader(`{`))
	malformedRequest.Header.Set("Content-Type", "application/json")
	malformedRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	malformedResponse := httptest.NewRecorder()
	service.router().ServeHTTP(malformedResponse, malformedRequest)
	assertCalendarErrorCode(t, malformedResponse, http.StatusBadRequest, "calendar_mutation_invalid_request")

	invalidExpectedResponse := sendCalendarMutationUpdate(t, service, event, "Changed", "invalid", "page-a", 1, true, true)
	assertCalendarErrorCode(t, invalidExpectedResponse, http.StatusBadRequest, "calendar_mutation_invalid_request")

	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(context.Background(), `CREATE TRIGGER fail_mutation_origin_insert BEFORE INSERT ON calendar_event_mutation_origins BEGIN SELECT RAISE(ABORT, 'sensitive mutation sqlite detail'); END`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	database.Close()
	internalResponse := sendCalendarMutationUpdate(t, service, event, "Changed", event.UpdatedAt, "page-a", 1, true, true)
	assertCalendarErrorCode(t, internalResponse, http.StatusInternalServerError, "calendar_internal_error")
	if strings.Contains(internalResponse.Body.String(), "sensitive mutation sqlite detail") {
		t.Fatalf("response exposed sqlite detail: %s", internalResponse.Body.String())
	}
}

type calendarMutationOriginRecord struct {
	resultingUpdatedAt string
	clientID           string
	sequence           int64
}

func sendCalendarMutationUpdate(t *testing.T, service *Service, event calendarEvent, title string, expectedUpdatedAt string, clientID string, sequence int64, includeClientID bool, includeSequence bool) *httptest.ResponseRecorder {
	t.Helper()
	payload := map[string]any{
		"eventID":           event.ID,
		"title":             title,
		"description":       event.Description,
		"location":          event.Location,
		"startISO":          event.StartISO,
		"endISO":            event.EndISO,
		"timeZone":          event.TimeZone,
		"isAllDay":          event.IsAllDay,
		"color":             event.Color,
		"participants":      calendarParticipantIdentities(event.Participants),
		"reminderLeadHours": event.ReminderLeadHours,
		"expectedUpdatedAt": expectedUpdatedAt,
	}
	if includeClientID {
		payload["mutationClientID"] = clientID
	}
	if includeSequence {
		payload["mutationSequence"] = sequence
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

func readCalendarMutationOrigin(t *testing.T, service *Service, eventID string) calendarMutationOriginRecord {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var origin calendarMutationOriginRecord
	errorValue = database.QueryRowContext(context.Background(), `
SELECT resulting_updated_at, client_id, sequence
FROM calendar_event_mutation_origins
WHERE event_id = ?`, eventID).Scan(&origin.resultingUpdatedAt, &origin.clientID, &origin.sequence)
	if errorValue != nil {
		if errorValue == sql.ErrNoRows {
			t.Fatalf("mutation origin for event %q not found", eventID)
		}
		t.Fatal(errorValue)
	}
	return origin
}

func calendarMutationOriginCount(t *testing.T, service *Service, eventID string) int {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var count int
	if errorValue := database.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM calendar_event_mutation_origins WHERE event_id = ?`, eventID).Scan(&count); errorValue != nil {
		t.Fatal(errorValue)
	}
	return count
}
