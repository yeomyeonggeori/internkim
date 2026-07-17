package admind

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPushCalendarOutboxKeepsNewCreateAfterPutNotFound(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("push-create-not-found", "Create Not Found")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("write: %v", errorValue)
	}
	expectedPath := account.DefaultCalendarURL + event.UID + ".ics"
	client := &fakeCalDAVPushClient{
		putErrors: map[string]error{expectedPath: errCalDAVObjectNotFound},
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}

	storedEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil {
		t.Fatalf("read event: %v", errorValue)
	}
	if !found || storedEvent.Title != event.Title {
		t.Fatalf("new create event should remain retryable: found=%v event=%+v", found, storedEvent)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil {
		t.Fatalf("list outbox: %v", errorValue)
	}
	if len(rows) != 1 {
		t.Fatalf("new create outbox should remain pending, got %d", len(rows))
	}
	if rows[0].AttemptCount != 1 || !strings.Contains(rows[0].LastError, "object not found") {
		t.Fatalf("retry metadata not recorded: %+v", rows[0])
	}
}

func TestPushCalendarOutboxDefersUpdateNotFoundWithoutRemoteObservation(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("push-update-not-found-unobserved", "Original")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-original"`
	event.RemoteHref = "/calendars/me/push-update-not-found-unobserved.ics"
	encoded, errorValue := encodeEventToICS(event)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event.RawICS = string(encoded)
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed event: %v", errorValue)
	}
	event.Title = "Local Edit"
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("local update: %v", errorValue)
	}
	client := &fakeCalDAVPushClient{
		putErrors: map[string]error{event.RemoteHref: errCalDAVObjectNotFound},
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}

	storedEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil {
		t.Fatalf("read event: %v", errorValue)
	}
	if !found || storedEvent.Title != event.Title {
		t.Fatalf("unobserved update should remain pending: found=%v event=%+v", found, storedEvent)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil {
		t.Fatalf("list outbox: %v", errorValue)
	}
	if len(rows) != 1 || rows[0].AttemptCount != 1 {
		t.Fatalf("unobserved update should remain retryable: %+v", rows)
	}
	if _, found, errorValue := service.readCalendarRemoteEventState(ctx, account.ID, activeRemoteCalendarTarget(account).CalendarURL, event.UID); errorValue != nil || found {
		t.Fatalf("unobserved 404 should not persist missing state: found=%v error=%v", found, errorValue)
	}
}

func TestPushCalendarOutboxTreatsObservedUpdateNotFoundAsRemoteDeletion(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("push-update-not-found-observed", "Original")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-original"`
	event.RemoteHref = "/calendars/me/push-update-not-found-observed.ics"
	encoded, errorValue := encodeEventToICS(event)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event.RawICS = string(encoded)
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed event: %v", errorValue)
	}
	if errorValue := service.upsertCalendarRemoteEventState(ctx, calendarRemoteEventState{
		AccountID:   account.ID,
		CalendarURL: activeRemoteCalendarTarget(account).CalendarURL,
		EventUID:    event.UID,
		LastSeenAt:  time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano),
	}); errorValue != nil {
		t.Fatalf("seed observation: %v", errorValue)
	}
	event.Title = "Local Edit"
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("local update: %v", errorValue)
	}
	client := &fakeCalDAVPushClient{
		putErrors: map[string]error{event.RemoteHref: errCalDAVObjectNotFound},
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}

	if _, found, errorValue := service.readCalendarEventByID(ctx, event.ID); errorValue != nil || found {
		t.Fatalf("observed update should follow remote deletion: found=%v error=%v", found, errorValue)
	}
	rows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil {
		t.Fatalf("list outbox: %v", errorValue)
	}
	if len(rows) != 0 {
		t.Fatalf("resolved observed deletion should clear outbox: %+v", rows)
	}
}

func TestPushCalendarOutboxDefersSuccessWithoutCanonicalETag(t *testing.T) {
	testCases := []struct {
		name       string
		getStatus  int
		getETag    string
		getPayload string
	}{
		{name: "GET error", getStatus: http.StatusInternalServerError},
		{name: "GET not found", getStatus: http.StatusNotFound},
		{name: "GET empty ETag", getStatus: http.StatusOK, getPayload: "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := newCalendarTestService(t)
			ctx := context.Background()
			account := seedAccountWithDiscovery(t, service)
			event := newLocalTestCalendarEvent("push-empty-etag-"+strings.ReplaceAll(testCase.name, " ", "-"), "Canonical ETag")
			if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
				t.Fatalf("write: %v", errorValue)
			}
			expectedPath := account.DefaultCalendarURL + event.UID + ".ics"
			requestMethods := []string{}
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				requestMethods = append(requestMethods, request.Method)
				if request.URL.EscapedPath() != expectedPath {
					t.Errorf("path: got %q, want %q", request.URL.EscapedPath(), expectedPath)
				}
				if request.Method == http.MethodPut {
					writer.WriteHeader(http.StatusNoContent)
					return
				}
				if request.Method != http.MethodGet {
					t.Fatalf("unexpected method: %s", request.Method)
				}
				writer.Header().Set("ETag", testCase.getETag)
				writer.WriteHeader(testCase.getStatus)
				_, _ = writer.Write([]byte(testCase.getPayload))
			}))
			defer server.Close()
			client := newTestOutboundCalDAVClient(t, server)

			if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
				t.Fatalf("push: %v", errorValue)
			}
			storedEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
			if errorValue != nil {
				t.Fatalf("read event: %v", errorValue)
			}
			if !found || storedEvent.RemoteHref != "" || storedEvent.RemoteETag != "" {
				t.Fatalf("push success committed without canonical ETag: found=%v event=%+v", found, storedEvent)
			}
			rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
			if errorValue != nil {
				t.Fatalf("list outbox: %v", errorValue)
			}
			if len(rows) != 1 || rows[0].AttemptCount != 1 {
				t.Fatalf("outbox should remain retryable: %+v", rows)
			}
			if strings.Join(requestMethods, ",") != http.MethodPut+","+http.MethodGet {
				t.Fatalf("request methods: %v", requestMethods)
			}
		})
	}
}

func TestPushCalendarOutboxDefersObservedUpdateWhenCanonicalETagGetReturnsNotFound(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("push-update-canonical-get-not-found", "Original")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-original"`
	event.RemoteHref = "/calendars/me/push-update-canonical-get-not-found.ics"
	encoded, errorValue := encodeEventToICS(event)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event.RawICS = string(encoded)
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed event: %v", errorValue)
	}
	if errorValue := service.upsertCalendarRemoteEventState(ctx, calendarRemoteEventState{
		AccountID:   account.ID,
		CalendarURL: activeRemoteCalendarTarget(account).CalendarURL,
		EventUID:    event.UID,
		LastSeenAt:  time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano),
	}); errorValue != nil {
		t.Fatalf("seed observation: %v", errorValue)
	}
	event.Title = "Local Edit"
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("local update: %v", errorValue)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPut {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		writer.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	client := newTestOutboundCalDAVClient(t, server)

	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}
	storedEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil {
		t.Fatalf("read event: %v", errorValue)
	}
	if !found || storedEvent.Title != event.Title {
		t.Fatalf("canonical GET 404 should not become remote deletion: found=%v event=%+v", found, storedEvent)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil {
		t.Fatalf("list outbox: %v", errorValue)
	}
	if len(rows) != 1 || rows[0].AttemptCount != 1 {
		t.Fatalf("update should remain retryable: %+v", rows)
	}
}
