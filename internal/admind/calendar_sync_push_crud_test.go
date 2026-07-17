package admind

import (
	"context"
	"testing"
	"time"
)

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
