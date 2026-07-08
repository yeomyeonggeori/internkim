package admind

import (
	"context"
	"testing"
	"time"
)

func TestSelectedCalendarBackfillRetargetsPendingPutFromPreviousCalendar(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	event := newLocalTestCalendarEvent("existing-retarget", "Existing Retarget")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-old"`
	event.RemoteHref = "/calendars/default/existing-retarget.ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatalf("write existing event: %v", errorValue)
	}
	account, errorValue := service.upsertRemoteCalendarAccount(ctx, remoteCalendarAccount{
		ID:                 "account-google-1",
		Provider:           remoteCalendarProviderGoogle,
		AccountEmail:       "user@example.com",
		DefaultCalendarURL: "/calendars/default/",
	})
	if errorValue != nil {
		t.Fatalf("seed account: %v", errorValue)
	}
	if errorValue := service.enqueueCalendarOutbox(ctx, calendarOutboxRow{
		AccountID:     account.ID,
		EventID:       event.ID,
		EventUID:      event.UID,
		Operation:     calendarOutboxOperationPut,
		IfMatchETag:   event.RemoteETag,
		RemoteHref:    event.RemoteHref,
		ChangedFields: []string{calendarFieldTitle},
	}); errorValue != nil {
		t.Fatalf("enqueue old target put: %v", errorValue)
	}

	selectedAccount, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "Company", "writer", "/calendars/company/", time.Now())
	if errorValue != nil {
		t.Fatalf("save selected calendar: %v", errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil {
		t.Fatalf("list outbox: %v", errorValue)
	}
	freshRows := 0
	oldRows := 0
	for _, row := range rows {
		if row.Operation != calendarOutboxOperationPut || row.EventUID != event.UID {
			continue
		}
		if row.RemoteHref == "" {
			freshRows++
		}
		if row.RemoteHref == event.RemoteHref {
			oldRows++
		}
	}
	if freshRows != 1 || oldRows != 1 {
		t.Fatalf("pending rows after backfill: fresh=%d old=%d rows=%+v", freshRows, oldRows, rows)
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, selectedAccount, &fakeCalDAVPushClient{}); errorValue != nil {
		t.Fatalf("push before initial pull: %v", errorValue)
	}
	rowsBeforePull, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil {
		t.Fatalf("list outbox before pull: %v", errorValue)
	}
	if len(rowsBeforePull) != len(rows) {
		t.Fatalf("initial export should wait for selected calendar pull: got %d rows, want %d", len(rowsBeforePull), len(rows))
	}
	if _, errorValue := service.runGoogleCalendarPull(ctx, selectedAccount, &fakeCalDAVPullClient{ctag: `"selected-ctag"`}); errorValue != nil {
		t.Fatalf("initial pull: %v", errorValue)
	}
	selectedAccount, _, errorValue = service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("read selected account: %v", errorValue)
	}

	expectedPath := "/calendars/company/" + event.UID + ".ics"
	client := &fakeCalDAVPushClient{
		putETags: map[string]string{expectedPath: `"etag-company"`},
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, selectedAccount, client); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}
	if len(client.putCalls) != 1 {
		t.Fatalf("put calls: got %d calls=%+v", len(client.putCalls), client.putCalls)
	}
	if client.putCalls[0].Path != expectedPath {
		t.Fatalf("put path: got %q, want %q", client.putCalls[0].Path, expectedPath)
	}
	remaining, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil {
		t.Fatalf("list remaining outbox: %v", errorValue)
	}
	if len(remaining) != 0 {
		t.Fatalf("outbox should be empty after stale cleanup and selected push: %+v", remaining)
	}
}

func TestSelectedCalendarBackfillDoesNotCreateFreshPutForImportedUID(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	event := newLocalTestCalendarEvent("existing-imported", "Existing Imported")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("write existing event: %v", errorValue)
	}
	account := seedAccountWithDiscovery(t, service)
	selectedAccount, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "Company", "writer", "/calendars/company/", time.Now())
	if errorValue != nil {
		t.Fatalf("save selected calendar: %v", errorValue)
	}
	remoteObject := fakeRemoteObject(t, event.UID, `"etag-imported"`, "Existing Imported")
	remoteObject.Path = "/calendars/company/" + event.UID + ".ics"
	if _, errorValue := service.runGoogleCalendarPull(ctx, selectedAccount, &fakeCalDAVPullClient{
		ctag:    `"selected-ctag"`,
		objects: []calDAVCalendarObject{remoteObject},
	}); errorValue != nil {
		t.Fatalf("initial pull: %v", errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, selectedAccount.ID)
	if errorValue != nil {
		t.Fatalf("list outbox: %v", errorValue)
	}
	freshRows := 0
	selectedUpdateRows := 0
	for _, row := range rows {
		if row.EventUID != event.UID || row.Operation != calendarOutboxOperationPut {
			continue
		}
		if row.RemoteHref == "" {
			freshRows++
		}
		if row.RemoteHref == remoteObject.Path && row.IfMatchETag == `"etag-imported"` {
			selectedUpdateRows++
		}
	}
	if freshRows != 0 {
		t.Fatalf("imported UID should not keep fresh duplicate export rows: %+v", rows)
	}
	if selectedUpdateRows != 1 {
		t.Fatalf("imported UID should retarget pending put to selected remote identity: got %d rows=%+v", selectedUpdateRows, rows)
	}
}
