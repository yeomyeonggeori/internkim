package admind

import (
	"context"
	"errors"
	"strings"
	"testing"
)

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
