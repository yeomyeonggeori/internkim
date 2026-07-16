package admind

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWriteCalendarEventLocalEnqueuesPutOutbox(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("local-new-1", "Local New")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("write: %v", errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, "")
	if errorValue != nil {
		t.Fatalf("list outbox: %v", errorValue)
	}
	if len(rows) != 1 {
		t.Fatalf("outbox rows: got %d, want 1", len(rows))
	}
	if rows[0].Operation != calendarOutboxOperationPut {
		t.Errorf("operation: got %q", rows[0].Operation)
	}
	if rows[0].EventID != event.ID {
		t.Errorf("event id: got %q", rows[0].EventID)
	}
	if rows[0].RemoteHref != "" {
		t.Errorf("remote href should be empty for new event: %q", rows[0].RemoteHref)
	}
}

func TestWriteCalendarEventFromPullSkipsOutbox(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("pull-1", "Pulled")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-1"`
	event.RemoteHref = "/calendars/me/pull-1.ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatalf("write pull: %v", errorValue)
	}
	rows, _ := service.listPendingCalendarOutbox(ctx, "")
	if len(rows) != 0 {
		t.Errorf("pull source should not enqueue, got %d rows", len(rows))
	}
}

func TestWriteCalendarEventLocalSkipsOutboxWhenAccountMissing(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	event := newLocalTestCalendarEvent("orphan-1", "No account")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("write: %v", errorValue)
	}
	rows, _ := service.listPendingCalendarOutbox(ctx, "")
	if len(rows) != 0 {
		t.Errorf("should not enqueue without account, got %d rows", len(rows))
	}
}

func TestSoftDeleteCalendarEventLocalEnqueuesDeleteOutbox(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("local-del", "Local Delete")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-del"`
	event.RemoteHref = "/calendars/me/local-del.ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed write: %v", errorValue)
	}
	if errorValue := service.softDeleteCalendarEvent(ctx, event.ID); errorValue != nil {
		t.Fatalf("soft delete: %v", errorValue)
	}
	rows, _ := service.listPendingCalendarOutbox(ctx, "")
	if len(rows) != 1 {
		t.Fatalf("outbox rows: got %d, want 1", len(rows))
	}
	if rows[0].Operation != calendarOutboxOperationDelete {
		t.Errorf("operation: got %q", rows[0].Operation)
	}
	if rows[0].RemoteHref != event.RemoteHref {
		t.Errorf("remote href: got %q", rows[0].RemoteHref)
	}
	if rows[0].IfMatchETag != event.RemoteETag {
		t.Errorf("if-match etag: got %q", rows[0].IfMatchETag)
	}
}

func TestPushCalendarOutboxCreatesNewObject(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("push-new", "Push New")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("write: %v", errorValue)
	}
	client := &fakeCalDAVPushClient{
		putETags: map[string]string{
			account.DefaultCalendarURL + event.UID + ".ics": `"etag-server-1"`,
		},
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}
	if len(client.putCalls) != 1 {
		t.Fatalf("put calls: got %d", len(client.putCalls))
	}
	call := client.putCalls[0]
	if call.IfNoneMatch != caldavWildcardETag {
		t.Errorf("If-None-Match: got %q, want %q", call.IfNoneMatch, caldavWildcardETag)
	}
	if call.IfMatch != "" {
		t.Errorf("If-Match should be empty for create: got %q", call.IfMatch)
	}
	remaining, _ := service.listPendingCalendarOutbox(ctx, "")
	if len(remaining) != 0 {
		t.Errorf("outbox should be empty after success: %d", len(remaining))
	}
	stored, _, _ := service.readCalendarEventByID(ctx, event.ID)
	if stored.RemoteSource != remoteCalendarProviderGoogle {
		t.Errorf("RemoteSource: got %q", stored.RemoteSource)
	}
	if stored.RemoteETag != `"etag-server-1"` {
		t.Errorf("RemoteETag: got %q", stored.RemoteETag)
	}
	if stored.RemoteHref != call.Path {
		t.Errorf("RemoteHref: got %q, want %q", stored.RemoteHref, call.Path)
	}
}

func TestPushCalendarOutboxCreatesNewObjectInSelectedCalendar(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	selectedCalendarURL := "/calendars/company/"
	selectedAccount, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "Company", "writer", selectedCalendarURL, time.Now())
	if errorValue != nil {
		t.Fatalf("select calendar: %v", errorValue)
	}

	event := newLocalTestCalendarEvent("push-selected", "Push Selected")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("write: %v", errorValue)
	}
	if _, errorValue := service.runGoogleCalendarPull(ctx, selectedAccount, &fakeCalDAVPullClient{ctag: `"selected-ctag"`}); errorValue != nil {
		t.Fatalf("initial pull: %v", errorValue)
	}
	selectedAccount, _, errorValue = service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("read selected account: %v", errorValue)
	}
	expectedPath := selectedCalendarURL + event.UID + ".ics"
	client := &fakeCalDAVPushClient{
		putETags: map[string]string{expectedPath: `"etag-selected-push"`},
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, selectedAccount, client); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}
	if len(client.putCalls) != 1 {
		t.Fatalf("put calls: got %d", len(client.putCalls))
	}
	if client.putCalls[0].Path != expectedPath {
		t.Fatalf("put path: got %q, want %q", client.putCalls[0].Path, expectedPath)
	}
	stored, _, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil {
		t.Fatalf("read event: %v", errorValue)
	}
	if stored.RemoteHref != expectedPath {
		t.Errorf("RemoteHref: got %q, want %q", stored.RemoteHref, expectedPath)
	}
}

func TestPushCalendarOutboxSkipsReadOnlySelectedCalendar(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	selectedAccount, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "Company", "reader", "/calendars/company/", time.Now())
	if errorValue != nil {
		t.Fatalf("select calendar: %v", errorValue)
	}

	event := newLocalTestCalendarEvent("push-readonly", "Push Readonly")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("write: %v", errorValue)
	}
	client := &fakeCalDAVPushClient{}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, selectedAccount, client); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}
	if len(client.putCalls) != 0 {
		t.Fatalf("read-only selected calendar should not receive PUT calls: %+v", client.putCalls)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, selectedAccount.ID)
	if errorValue != nil {
		t.Fatalf("list outbox: %v", errorValue)
	}
	if len(rows) != 1 {
		t.Fatalf("read-only skip should keep pending row, got %d", len(rows))
	}
}

func TestPushCalendarOutboxUpdatesObjectWithIfMatch(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("push-upd", "Push Update")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-old"`
	event.RemoteHref = "/calendars/me/push-upd.ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed write: %v", errorValue)
	}
	event.Title = "Push Update modified"
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("local update: %v", errorValue)
	}
	client := &fakeCalDAVPushClient{
		putETags: map[string]string{event.RemoteHref: `"etag-new"`},
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}
	if len(client.putCalls) != 1 {
		t.Fatalf("put calls: got %d", len(client.putCalls))
	}
	if client.putCalls[0].IfMatch != `"etag-old"` {
		t.Errorf("If-Match: got %q", client.putCalls[0].IfMatch)
	}
	if client.putCalls[0].IfNoneMatch != "" {
		t.Errorf("If-None-Match should be empty on update: got %q", client.putCalls[0].IfNoneMatch)
	}
	stored, _, _ := service.readCalendarEventByID(ctx, event.ID)
	if stored.RemoteETag != `"etag-new"` {
		t.Errorf("RemoteETag: got %q", stored.RemoteETag)
	}
}

func TestPushCalendarOutboxDeletesObjectWithIfMatch(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("push-del", "Push Delete")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-del"`
	event.RemoteHref = "/calendars/me/push-del.ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed write: %v", errorValue)
	}
	if errorValue := service.softDeleteCalendarEvent(ctx, event.ID); errorValue != nil {
		t.Fatalf("soft delete: %v", errorValue)
	}
	remoteObject := fakeRemoteObject(t, event.UID, event.RemoteETag, event.Title)
	remoteObject.Path = event.RemoteHref
	client := &fakeCalDAVPushClient{getObjects: map[string]calDAVCalendarObject{event.RemoteHref: remoteObject}}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}
	if len(client.deleteCalls) != 1 {
		t.Fatalf("delete calls: got %d", len(client.deleteCalls))
	}
	if client.deleteCalls[0].Path != event.RemoteHref {
		t.Errorf("delete path: got %q", client.deleteCalls[0].Path)
	}
	if client.deleteCalls[0].IfMatch != event.RemoteETag {
		t.Errorf("If-Match: got %q", client.deleteCalls[0].IfMatch)
	}
}

func TestPushCalendarOutboxClearsAuthErrorAfterDeleteSuccess(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	service.markRemoteCalendarAccountAuthError(ctx, account, errors.New("caldav delete status 401: Unauthorized"))
	reloaded, _, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("read account: %v", errorValue)
	}

	event := newLocalTestCalendarEvent("push-del-auth-clear", "Push Delete Auth Clear")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-del-auth-clear"`
	event.RemoteHref = "/calendars/me/push-del-auth-clear.ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed write: %v", errorValue)
	}
	if errorValue := service.softDeleteCalendarEvent(ctx, event.ID); errorValue != nil {
		t.Fatalf("soft delete: %v", errorValue)
	}

	remoteObject := fakeRemoteObject(t, event.UID, event.RemoteETag, event.Title)
	remoteObject.Path = event.RemoteHref
	client := &fakeCalDAVPushClient{getObjects: map[string]calDAVCalendarObject{event.RemoteHref: remoteObject}}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, reloaded, client); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}
	refreshed, _, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("read refreshed account: %v", errorValue)
	}
	if refreshed.LastAuthError != "" || refreshed.LastAuthErrorAt != "" {
		t.Fatalf("auth error should clear after delete success: %+v", refreshed)
	}
}

func TestPushCalendarOutboxKeepsAuthErrorAfterNoopRow(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	service.markRemoteCalendarAccountAuthError(ctx, account, errors.New("caldav put status 401: Unauthorized"))
	reloaded, _, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("read account: %v", errorValue)
	}
	if errorValue := service.enqueueCalendarOutbox(ctx, calendarOutboxRow{
		AccountID: account.ID,
		EventID:   "missing-event",
		EventUID:  "missing-event@internkim",
		Operation: calendarOutboxOperationPut,
	}); errorValue != nil {
		t.Fatalf("enqueue outbox: %v", errorValue)
	}

	client := &fakeCalDAVPushClient{}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, reloaded, client); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}
	refreshed, _, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("read refreshed account: %v", errorValue)
	}
	if refreshed.LastAuthError == "" || refreshed.LastAuthErrorAt == "" {
		t.Fatalf("auth error should remain after noop row: %+v", refreshed)
	}
	if len(client.putCalls) != 0 || len(client.deleteCalls) != 0 {
		t.Fatalf("noop row should not call remote client: put=%d delete=%d", len(client.putCalls), len(client.deleteCalls))
	}
}

func TestPushCalendarOutboxBlocksRowAfterMaxAttempts(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("push-cap", "Hit attempt cap")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("write: %v", errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, "")
	if errorValue != nil {
		t.Fatalf("list outbox: %v", errorValue)
	}
	if len(rows) != 1 {
		t.Fatalf("seed rows: %d", len(rows))
	}
	for index := 0; index < calendarOutboxMaxAttempts; index++ {
		if errorValue := service.markCalendarOutboxBatchAttempt(ctx, rows[0], "simulated failure"); errorValue != nil {
			t.Fatalf("mark attempt %d: %v", index, errorValue)
		}
	}
	client := &fakeCalDAVPushClient{}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}
	if len(client.putCalls) != 0 {
		t.Errorf("client should not be called when attempt cap reached, got %d put calls", len(client.putCalls))
	}
	pending, _ := service.listPendingCalendarOutbox(ctx, "")
	if len(pending) != 0 {
		t.Errorf("blocked row should not remain pending, got %d", len(pending))
	}
	stored, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(stored) != 1 {
		t.Fatalf("failed row should be preserved, got %d", len(stored))
	}
	if stored[0].Status != calendarOutboxStatusBlocked || stored[0].FailedAt == "" {
		t.Fatalf("failed row not blocked: %+v", stored[0])
	}
}

func TestPushCalendarOutboxKeepsRowOnTransientError(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("push-transient", "Transient")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("write: %v", errorValue)
	}
	expectedPath := account.DefaultCalendarURL + event.UID + ".ics"
	client := &fakeCalDAVPushClient{
		putErrors: map[string]error{expectedPath: errors.New("network unreachable")},
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}
	rows, _ := service.listPendingCalendarOutbox(ctx, "")
	if len(rows) != 1 {
		t.Fatalf("transient error should keep outbox row, got %d", len(rows))
	}
	if rows[0].AttemptCount != 1 {
		t.Errorf("attempt count: got %d", rows[0].AttemptCount)
	}
	if !strings.Contains(rows[0].LastError, "network unreachable") {
		t.Errorf("last error not captured: %q", rows[0].LastError)
	}
}

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

func TestPushCalendarOutboxMarksAccountAuthErrorOnCalDAVUnauthorized(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("push-auth", "Auth Failure")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("write: %v", errorValue)
	}
	expectedPath := account.DefaultCalendarURL + event.UID + ".ics"
	client := &fakeCalDAVPushClient{
		putErrors: map[string]error{expectedPath: errors.New("caldav put status 401: Unauthorized")},
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}
	reloaded, _, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("read account: %v", errorValue)
	}
	if !strings.Contains(reloaded.LastAuthError, "401") {
		t.Fatalf("last auth error: got %q", reloaded.LastAuthError)
	}
	if reloaded.LastAuthErrorAt == "" {
		t.Fatal("last auth error timestamp should be set")
	}
}
