package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCalendarDeleteIntentSchemaIsCreated(t *testing.T) {
	service := newCalendarTestService(t)
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()

	for _, tableName := range []string{"calendar_delete_intents", "calendar_event_mutation_origins"} {
		var count int
		if errorValue := database.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, tableName).Scan(&count); errorValue != nil {
			t.Fatal(errorValue)
		}
		if count != 1 {
			t.Fatalf("table %q count = %d, want 1", tableName, count)
		}
	}

	rows, errorValue := database.QueryContext(context.Background(), `PRAGMA table_info(calendar_delete_intents)`)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var columnID int
		var name string
		var columnType string
		var notNull int
		var defaultValue any
		var primaryKey int
		if errorValue := rows.Scan(&columnID, &name, &columnType, &notNull, &defaultValue, &primaryKey); errorValue != nil {
			t.Fatal(errorValue)
		}
		columns[name] = true
	}
	for _, columnName := range []string{"operation_id", "event_id", "client_id", "sequence", "expected_updated_at", "requested_at", "execute_at", "status", "resolved_at", "resolution_sequence", "next_attempt_at", "attempt_count", "last_error"} {
		if !columns[columnName] {
			t.Fatalf("calendar_delete_intents missing column %q", columnName)
		}
	}
}

func TestCalendarDeleteIntentCreateIsDurableAndIdempotent(t *testing.T) {
	service := newCalendarTestService(t)
	event := seedCalendarDeleteIntentEvent(t, service, "delete-intent-idempotent")
	payload := `{"clientID":"page-a","sequence":2,"expectedUpdatedAt":"` + event.UpdatedAt + `"}`

	firstResponse := sendCalendarDeleteIntentHTTPRequest(service, http.MethodPut, event.ID, "operation-a", payload)
	if firstResponse.Code != http.StatusAccepted {
		t.Fatalf("first create status = %d body = %s", firstResponse.Code, firstResponse.Body.String())
	}
	var firstBody struct {
		OperationID string `json:"operationID"`
		ExecuteAt   string `json:"executeAt"`
	}
	if errorValue := json.Unmarshal(firstResponse.Body.Bytes(), &firstBody); errorValue != nil {
		t.Fatal(errorValue)
	}
	if firstBody.OperationID != "operation-a" {
		t.Fatalf("operationID = %q", firstBody.OperationID)
	}
	if _, errorValue := time.Parse(time.RFC3339Nano, firstBody.ExecuteAt); errorValue != nil {
		t.Fatalf("executeAt = %q error = %v", firstBody.ExecuteAt, errorValue)
	}

	secondResponse := sendCalendarDeleteIntentHTTPRequest(service, http.MethodPut, event.ID, "operation-a", payload)
	if secondResponse.Code != http.StatusAccepted {
		t.Fatalf("duplicate create status = %d body = %s", secondResponse.Code, secondResponse.Body.String())
	}
	var secondBody struct {
		ExecuteAt string `json:"executeAt"`
	}
	if errorValue := json.Unmarshal(secondResponse.Body.Bytes(), &secondBody); errorValue != nil {
		t.Fatal(errorValue)
	}
	if secondBody.ExecuteAt != firstBody.ExecuteAt {
		t.Fatalf("duplicate executeAt = %q, want %q", secondBody.ExecuteAt, firstBody.ExecuteAt)
	}
	if count := calendarDeleteIntentRowCount(t, service, "operation-a"); count != 1 {
		t.Fatalf("delete intent row count = %d, want 1", count)
	}

	mismatchResponse := sendCalendarDeleteIntentHTTPRequest(service, http.MethodPut, event.ID, "operation-a", `{"clientID":"page-a","sequence":3,"expectedUpdatedAt":"`+event.UpdatedAt+`"}`)
	if mismatchResponse.Code != http.StatusConflict {
		t.Fatalf("operation mismatch status = %d body = %s", mismatchResponse.Code, mismatchResponse.Body.String())
	}
}

func TestCalendarDeleteIntentCancelIsIdempotent(t *testing.T) {
	service := newCalendarTestService(t)
	event := seedCalendarDeleteIntentEvent(t, service, "delete-intent-cancel")
	createPayload := `{"clientID":"page-a","sequence":2,"expectedUpdatedAt":"` + event.UpdatedAt + `"}`
	createResponse := sendCalendarDeleteIntentHTTPRequest(service, http.MethodPut, event.ID, "operation-cancel", createPayload)
	if createResponse.Code != http.StatusAccepted {
		t.Fatalf("create status = %d body = %s", createResponse.Code, createResponse.Body.String())
	}

	for attempt := 0; attempt < 2; attempt++ {
		cancelResponse := sendCalendarDeleteIntentHTTPRequest(service, http.MethodDelete, event.ID, "operation-cancel", `{"clientID":"page-a","sequence":3}`)
		if cancelResponse.Code != http.StatusNoContent {
			t.Fatalf("cancel attempt %d status = %d body = %s", attempt+1, cancelResponse.Code, cancelResponse.Body.String())
		}
	}

	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var status string
	var resolutionSequence int64
	if errorValue := database.QueryRowContext(context.Background(), `SELECT status, resolution_sequence FROM calendar_delete_intents WHERE operation_id = ?`, "operation-cancel").Scan(&status, &resolutionSequence); errorValue != nil {
		t.Fatal(errorValue)
	}
	if status != "canceled" || resolutionSequence != 3 {
		t.Fatalf("canceled intent status = %q resolution sequence = %d", status, resolutionSequence)
	}
	if _, found, errorValue := service.readCalendarEventByID(context.Background(), event.ID); errorValue != nil || !found {
		t.Fatalf("event after cancel found = %v error = %v", found, errorValue)
	}
}

func TestCalendarDeleteIntentCancelRejectsWrongIdentityOrOlderSequence(t *testing.T) {
	service := newCalendarTestService(t)
	event := seedCalendarDeleteIntentEvent(t, service, "delete-intent-cancel-ordering")
	createResponse := sendCalendarDeleteIntentHTTPRequest(service, http.MethodPut, event.ID, "operation-cancel-ordering", `{"clientID":"page-a","sequence":3,"expectedUpdatedAt":"`+event.UpdatedAt+`"}`)
	if createResponse.Code != http.StatusAccepted {
		t.Fatalf("create status = %d body = %s", createResponse.Code, createResponse.Body.String())
	}

	testCases := []struct {
		name    string
		payload string
	}{
		{name: "wrong client", payload: `{"clientID":"page-b","sequence":4}`},
		{name: "same sequence", payload: `{"clientID":"page-a","sequence":3}`},
		{name: "older sequence", payload: `{"clientID":"page-a","sequence":2}`},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			response := sendCalendarDeleteIntentHTTPRequest(service, http.MethodDelete, event.ID, "operation-cancel-ordering", testCase.payload)
			if response.Code != http.StatusConflict {
				t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
			}
			assertCalendarDeleteIntentStatus(t, service, "operation-cancel-ordering", calendarDeleteIntentStatusPending)
		})
	}
}

func TestCalendarDeleteIntentCreateAcceptsSameClientEarlierCurrentVersion(t *testing.T) {
	service := newCalendarTestService(t)
	event := seedCalendarDeleteIntentEvent(t, service, "delete-intent-create-after-put")
	updateResponse := sendCalendarMutationUpdate(t, service, event, "PUT completed first", event.UpdatedAt, "page-a", 1, true, true)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d body = %s", updateResponse.Code, updateResponse.Body.String())
	}

	createResponse := sendCalendarDeleteIntentHTTPRequest(service, http.MethodPut, event.ID, "operation-after-put", `{"clientID":"page-a","sequence":2,"expectedUpdatedAt":"`+event.UpdatedAt+`"}`)
	if createResponse.Code != http.StatusAccepted {
		t.Fatalf("create after same-client PUT status = %d body = %s", createResponse.Code, createResponse.Body.String())
	}
}

func TestCalendarDeleteIntentHTTPValidation(t *testing.T) {
	service := newCalendarTestService(t)
	event := seedCalendarDeleteIntentEvent(t, service, "delete-intent-validation")
	testCases := []struct {
		name       string
		method     string
		operation  string
		payload    string
		wantStatus int
	}{
		{name: "missing client", method: http.MethodPut, operation: "missing-client", payload: `{"sequence":1,"expectedUpdatedAt":"` + event.UpdatedAt + `"}`, wantStatus: http.StatusBadRequest},
		{name: "zero sequence", method: http.MethodPut, operation: "zero-sequence", payload: `{"clientID":"page-a","sequence":0,"expectedUpdatedAt":"` + event.UpdatedAt + `"}`, wantStatus: http.StatusBadRequest},
		{name: "invalid expected version", method: http.MethodPut, operation: "invalid-version", payload: `{"clientID":"page-a","sequence":1,"expectedUpdatedAt":"bad"}`, wantStatus: http.StatusBadRequest},
		{name: "stale expected version", method: http.MethodPut, operation: "stale-version", payload: `{"clientID":"page-a","sequence":1,"expectedUpdatedAt":"2026-01-01T00:00:00Z"}`, wantStatus: http.StatusConflict},
		{name: "cancel missing client", method: http.MethodDelete, operation: "missing-cancel-client", payload: `{"sequence":2}`, wantStatus: http.StatusBadRequest},
		{name: "cancel zero sequence", method: http.MethodDelete, operation: "zero-cancel-sequence", payload: `{"clientID":"page-a","sequence":0}`, wantStatus: http.StatusBadRequest},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			response := sendCalendarDeleteIntentHTTPRequest(service, testCase.method, event.ID, testCase.operation, testCase.payload)
			if response.Code != testCase.wantStatus {
				t.Fatalf("status = %d, want %d body = %s", response.Code, testCase.wantStatus, response.Body.String())
			}
		})
	}

	notFoundResponse := sendCalendarDeleteIntentHTTPRequest(service, http.MethodPut, "missing-event", "missing-event-operation", `{"clientID":"page-a","sequence":1,"expectedUpdatedAt":"`+event.UpdatedAt+`"}`)
	if notFoundResponse.Code != http.StatusNotFound {
		t.Fatalf("missing event status = %d body = %s", notFoundResponse.Code, notFoundResponse.Body.String())
	}
}

func TestParseCalendarDeleteIntentPathRejectsNonEventPrefix(t *testing.T) {
	if _, _, found := parseCalendarDeleteIntentPath("/foo/delete-intents/operation-a"); found {
		t.Fatal("non-event delete intent path was accepted")
	}
}

func seedCalendarDeleteIntentEvent(t *testing.T, service *Service, eventID string) calendarEvent {
	t.Helper()
	event := newLocalTestCalendarEvent(eventID, "Delete intent")
	if errorValue := service.writeCalendarEventWithSource(context.Background(), event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	return readRequiredCalendarEvent(t, service, event.ID)
}

func sendCalendarDeleteIntentHTTPRequest(service *Service, method string, eventID string, operationID string, payload string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, "/calendar/api/events/"+eventID+"/delete-intents/"+operationID, strings.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	return response
}

func calendarDeleteIntentRowCount(t *testing.T, service *Service, operationID string) int {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var count int
	if errorValue := database.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM calendar_delete_intents WHERE operation_id = ?`, operationID).Scan(&count); errorValue != nil {
		t.Fatal(errorValue)
	}
	return count
}
