package admind

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSelectedCalendarSwitchPreservesLatestSameUIDEditForNewTarget(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	event := newLocalTestCalendarEvent("switch-latest-edit", "Original")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-default"`
	event.RemoteHref = "/calendars/default/switch-latest-edit.ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	account, errorValue := service.upsertRemoteCalendarAccount(ctx, remoteCalendarAccount{
		ID:                 "account-google-1",
		Provider:           remoteCalendarProviderGoogle,
		AccountEmail:       "user@example.com",
		DefaultCalendarURL: "/calendars/default/",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	firstEdit := event
	firstEdit.Title = "Edited For Default"
	if errorValue := service.writeCalendarEvent(ctx, firstEdit); errorValue != nil {
		t.Fatal(errorValue)
	}
	selectedAccount, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "Company", "writer", "/calendars/company/", time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	latestEdit := firstEdit
	latestEdit.Title = "Latest For Company"
	if errorValue := service.writeCalendarEvent(ctx, latestEdit); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.runGoogleCalendarPull(ctx, selectedAccount, &fakeCalDAVPullClient{ctag: `"company-ctag"`}); errorValue != nil {
		t.Fatal(errorValue)
	}
	selectedAccount, _, errorValue = service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	expectedPath := "/calendars/company/" + event.UID + ".ics"
	client := &fakeCalDAVPushClient{putETags: map[string]string{expectedPath: `"etag-company"`}}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, selectedAccount, client); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(client.putCalls) != 1 {
		t.Fatalf("put calls=%d want 1", len(client.putCalls))
	}
	call := client.putCalls[0]
	if call.Path != expectedPath || call.IfMatch != "" || call.IfNoneMatch != caldavWildcardETag {
		t.Fatalf("put call=%+v want fresh create at %q", call, expectedPath)
	}
	if !strings.Contains(string(call.Data), "SUMMARY:Latest For Company") {
		t.Fatalf("pushed ICS does not contain latest edit: %s", call.Data)
	}
	remaining, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(remaining) != 0 {
		t.Fatalf("remaining outbox rows=%+v", remaining)
	}
}

func TestStaleCalendarTargetCleanupPreservesNewTargetSourceRows(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	event := newLocalTestCalendarEvent("switch-source-rows", "Original")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-default"`
	event.RemoteHref = "/calendars/default/switch-source-rows.ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	account, errorValue := service.upsertRemoteCalendarAccount(ctx, remoteCalendarAccount{
		ID:                 "account-google-1",
		Provider:           remoteCalendarProviderGoogle,
		AccountEmail:       "user@example.com",
		DefaultCalendarURL: "/calendars/default/",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	firstEdit := event
	firstEdit.Title = "Default Edit"
	if errorValue := service.writeCalendarEvent(ctx, firstEdit); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "Company", "writer", "/calendars/company/", time.Now()); errorValue != nil {
		t.Fatal(errorValue)
	}
	latestEdit := firstEdit
	latestEdit.Description = "Company Edit"
	if errorValue := service.writeCalendarEvent(ctx, latestEdit); errorValue != nil {
		t.Fatal(errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	batches := aggregateCalendarOutboxRows(rows)
	var staleBatch calendarOutboxRow
	for _, batch := range batches {
		if batch.RemoteHref == event.RemoteHref {
			staleBatch = batch
		}
	}
	if staleBatch.ID == 0 {
		t.Fatalf("stale target batch not found: %+v", batches)
	}
	if errorValue := service.deleteCalendarOutboxBatch(ctx, staleBatch); errorValue != nil {
		t.Fatal(errorValue)
	}
	remaining, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(remaining) != 2 {
		t.Fatalf("new target source rows=%d want 2 rows=%+v", len(remaining), remaining)
	}
	for _, row := range remaining {
		if row.RemoteHref != "" || row.IfMatchETag != "" {
			t.Fatalf("new target row retained stale remote state: %+v", row)
		}
	}
}

func TestStaleCalendarTargetSuccessDoesNotOverwriteNewTargetRemoteMapping(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	event := newLocalTestCalendarEvent("switch-stale-success", "Original")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-default"`
	event.RemoteHref = "/calendars/default/switch-stale-success.ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	account, errorValue := service.upsertRemoteCalendarAccount(ctx, remoteCalendarAccount{
		ID:                 "account-google-1",
		Provider:           remoteCalendarProviderGoogle,
		AccountEmail:       "user@example.com",
		DefaultCalendarURL: "/calendars/default/",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defaultEdit := event
	defaultEdit.Title = "Default Edit"
	if errorValue := service.writeCalendarEvent(ctx, defaultEdit); errorValue != nil {
		t.Fatal(errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil || len(rows) != 1 {
		t.Fatalf("default target rows=%+v error=%v", rows, errorValue)
	}
	staleRow := rows[0]
	if _, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "Company", "writer", "/calendars/company/", time.Now()); errorValue != nil {
		t.Fatal(errorValue)
	}
	companyEvent := defaultEdit
	companyEvent.RemoteHref = "/calendars/company/switch-stale-success.ics"
	companyEvent.RemoteETag = `"etag-company"`
	companyICS, errorValue := encodeEventToICS(companyEvent)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	companyEvent.RawICS = string(companyICS)
	if errorValue := service.writeCalendarEventWithSource(ctx, companyEvent, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	stalePushedEvent := defaultEdit
	stalePushedEvent.RemoteHref = "/calendars/default/switch-stale-success.ics"
	stalePushedEvent.RemoteETag = `"etag-default-new"`
	staleICS, errorValue := encodeEventToICS(stalePushedEvent)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.applyCalendarPushSuccess(ctx, staleRow, defaultEdit, stalePushedEvent, stalePushedEvent.RemoteHref, stalePushedEvent.RemoteETag, staleICS); errorValue != nil {
		t.Fatal(errorValue)
	}
	storedEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("stored event found=%v error=%v", found, errorValue)
	}
	if storedEvent.RemoteHref != companyEvent.RemoteHref || storedEvent.RemoteETag != companyEvent.RemoteETag {
		t.Fatalf("remote mapping href=%q etag=%q want href=%q etag=%q", storedEvent.RemoteHref, storedEvent.RemoteETag, companyEvent.RemoteHref, companyEvent.RemoteETag)
	}
}

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
