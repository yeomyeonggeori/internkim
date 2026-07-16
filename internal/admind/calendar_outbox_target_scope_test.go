package admind

import (
	"context"
	"testing"
	"time"
)

func TestSelectedTargetPullPreservesStaleAndBlockedOutboxRows(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	event := newLocalTestCalendarEvent("target-scoped-pull", "Original")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteHref = "/calendars/default/target-scoped-pull.ics"
	event.RemoteETag = `"etag-default"`
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
	if errorValue := service.enqueueCalendarOutbox(ctx, calendarOutboxRow{
		AccountID:         account.ID,
		EventID:           event.ID,
		EventUID:          event.UID,
		Operation:         calendarOutboxOperationPut,
		TargetCalendarURL: "/calendars/default/",
		IfMatchETag:       event.RemoteETag,
		RemoteHref:        event.RemoteHref,
		ChangedFields:     []string{calendarFieldTitle},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.enqueueCalendarOutbox(ctx, calendarOutboxRow{
		AccountID:         account.ID,
		EventID:           event.ID,
		EventUID:          event.UID,
		Operation:         calendarOutboxOperationDelete,
		TargetCalendarURL: "/calendars/default/",
		IfMatchETag:       event.RemoteETag,
		RemoteHref:        event.RemoteHref,
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	selectedAccount, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "Company", "writer", "/calendars/company/", time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	latestEvent := event
	latestEvent.Description = "Company local edit"
	if errorValue := service.writeCalendarEvent(ctx, latestEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.enqueueCalendarOutbox(ctx, calendarOutboxRow{
		AccountID:         account.ID,
		EventID:           event.ID,
		EventUID:          event.UID,
		Operation:         calendarOutboxOperationPut,
		TargetCalendarURL: "/calendars/company/",
		ChangedFields:     []string{calendarFieldLocation},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	rowsBeforePull, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var staleDelete calendarOutboxRow
	var stalePut calendarOutboxRow
	var blockedNewTargetPut calendarOutboxRow
	for _, row := range rowsBeforePull {
		switch {
		case row.TargetCalendarURL == "/calendars/default/" && row.Operation == calendarOutboxOperationDelete:
			staleDelete = row
		case row.TargetCalendarURL == "/calendars/default/" && row.Operation == calendarOutboxOperationPut:
			stalePut = row
		case row.TargetCalendarURL == "/calendars/company/" && len(row.ChangedFields) == 1 && row.ChangedFields[0] == calendarFieldLocation:
			blockedNewTargetPut = row
		}
	}
	if staleDelete.ID == 0 || stalePut.ID == 0 || blockedNewTargetPut.ID == 0 {
		t.Fatalf("missing seeded rows: %+v", rowsBeforePull)
	}
	if errorValue := service.markCalendarOutboxBatchBlocked(ctx, blockedNewTargetPut, time.Now().UTC().Format(time.RFC3339Nano), "manual review required"); errorValue != nil {
		t.Fatal(errorValue)
	}
	remoteObject := fakeRemoteObject(t, event.UID, `"etag-company"`, "Company Remote")
	remoteObject.Path = "/calendars/company/target-scoped-pull.ics"
	if _, errorValue := service.runGoogleCalendarPull(ctx, selectedAccount, &fakeCalDAVPullClient{
		ctag:    `"company-ctag"`,
		objects: []calDAVCalendarObject{remoteObject},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	rowsAfterPull, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	rowsByID := map[int64]calendarOutboxRow{}
	for _, row := range rowsAfterPull {
		rowsByID[row.ID] = row
	}
	if _, found := rowsByID[staleDelete.ID]; !found {
		t.Fatalf("stale target delete was removed: before=%+v after=%+v", staleDelete, rowsAfterPull)
	}
	storedStalePut, found := rowsByID[stalePut.ID]
	if !found || storedStalePut.RemoteHref != stalePut.RemoteHref || storedStalePut.IfMatchETag != stalePut.IfMatchETag {
		t.Fatalf("stale target put was mutated: before=%+v after=%+v", stalePut, storedStalePut)
	}
	storedBlockedPut, found := rowsByID[blockedNewTargetPut.ID]
	if !found || storedBlockedPut.Status != calendarOutboxStatusBlocked || storedBlockedPut.RemoteHref != remoteObject.Path || storedBlockedPut.IfMatchETag != remoteObject.ETag {
		t.Fatalf("blocked target put was mutated: %+v", storedBlockedPut)
	}
}
