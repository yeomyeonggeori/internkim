package admind

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCalendarTargetUnavailableErrorSupportsErrorsIs(t *testing.T) {
	errorValue := calendarOutboxTargetUnavailableError("google-test")
	if !errors.Is(errorValue, errCalendarTargetUnavailable) {
		t.Fatalf("error = %v", errorValue)
	}
}

func TestCalendarEventMutationReportsTargetUnavailable(t *testing.T) {
	testCases := []struct {
		name          string
		method        string
		path          string
		eventID       string
		requestBody   string
		seedEvent     bool
		expectedFound bool
	}{
		{
			name:        "create",
			method:      http.MethodPost,
			path:        "/calendar/api/events",
			eventID:     "target-unavailable-create",
			requestBody: calendarTargetUnavailableRequestBody("target-unavailable-create", "Created title"),
		},
		{
			name:          "update",
			method:        http.MethodPut,
			path:          "/calendar/api/events/target-unavailable-update",
			eventID:       "target-unavailable-update",
			requestBody:   calendarTargetUnavailableRequestBody("target-unavailable-update", "Updated title"),
			seedEvent:     true,
			expectedFound: true,
		},
		{
			name:          "delete",
			method:        http.MethodDelete,
			path:          "/calendar/api/events/target-unavailable-delete",
			eventID:       "target-unavailable-delete",
			seedEvent:     true,
			expectedFound: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := newCalendarTestService(t)
			contextValue := context.Background()
			account := seedAccountWithDiscovery(t, service)
			account.SelectedCalendarID = "partial@example.com"
			account.SelectedCalendarURL = ""
			if _, errorValue := service.upsertRemoteCalendarAccount(contextValue, account); errorValue != nil {
				t.Fatal(errorValue)
			}
			if testCase.seedEvent {
				event := newLocalTestCalendarEvent(testCase.eventID, "Original title")
				if errorValue := service.writeCalendarEventWithSource(contextValue, event, calendarSourcePull); errorValue != nil {
					t.Fatal(errorValue)
				}
			}
			requestBody := testCase.requestBody
			if testCase.seedEvent {
				persistedEvent, found, errorValue := service.readCalendarEventByID(contextValue, testCase.eventID)
				if errorValue != nil || !found {
					t.Fatalf("seeded event found=%v error=%v", found, errorValue)
				}
				if testCase.method == http.MethodPut {
					var payload calendarEventWriteRequest
					if errorValue := json.Unmarshal([]byte(requestBody), &payload); errorValue != nil {
						t.Fatal(errorValue)
					}
					payload.ExpectedUpdatedAt = persistedEvent.UpdatedAt
					encodedPayload, errorValue := json.Marshal(payload)
					if errorValue != nil {
						t.Fatal(errorValue)
					}
					requestBody = string(encodedPayload)
				}
				if testCase.method == http.MethodDelete {
					encodedPayload, errorValue := json.Marshal(calendarEventDeleteRequest{ExpectedUpdatedAt: persistedEvent.UpdatedAt})
					if errorValue != nil {
						t.Fatal(errorValue)
					}
					requestBody = string(encodedPayload)
				}
			}

			request := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(requestBody))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
			response := httptest.NewRecorder()
			service.router().ServeHTTP(response, request)

			if response.Code != http.StatusConflict {
				t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
			}
			if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
				t.Fatalf("content type = %q", contentType)
			}
			if strings.Contains(response.Body.String(), account.ID) {
				t.Fatalf("response exposed account ID: %s", response.Body.String())
			}
			var payload struct {
				Code string `json:"code"`
			}
			if errorValue := json.Unmarshal(response.Body.Bytes(), &payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			if payload.Code != calendarTargetUnavailableErrorCode {
				t.Fatalf("code = %q", payload.Code)
			}

			event, found, errorValue := service.readCalendarEventByID(contextValue, testCase.eventID)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if found != testCase.expectedFound {
				t.Fatalf("found = %v event = %+v", found, event)
			}
			if found && event.Title != "Original title" {
				t.Fatalf("title = %q", event.Title)
			}
		})
	}
}

func calendarTargetUnavailableRequestBody(eventID string, title string) string {
	payload := calendarEventWriteRequest{
		EventID:  eventID,
		Title:    title,
		StartISO: "2026-07-16T01:00:00Z",
		EndISO:   "2026-07-16T02:00:00Z",
		TimeZone: "Asia/Seoul",
		Color:    "#2563eb",
	}
	encoded, errorValue := json.Marshal(payload)
	if errorValue != nil {
		panic(errorValue)
	}
	return string(encoded)
}
