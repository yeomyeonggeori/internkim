package admind

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestMissingPullWithoutLocalIntentDeletesEventAndObservationFence(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	target := activeRemoteCalendarTarget(account)
	event := newLocalTestCalendarEvent("pull-missing-no-intent", "Remote Event")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-before-delete"`
	event.RemoteHref = "/calendars/me/pull-missing-no-intent.ics"
	encoded, errorValue := encodeEventToICS(event)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event.RawICS = string(encoded)
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	seedCalendarPushObservationFenceForTest(t, service, account.ID, target.CalendarURL, event.UID)

	if errorValue := service.softDeleteMissingRemoteEvent(ctx, account.ID, target, event, pendingCalendarLocalChange{}, time.Now().UTC()); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, found, errorValue := service.readCalendarEventByID(ctx, event.ID); errorValue != nil || found {
		t.Fatalf("active event found=%v error=%v", found, errorValue)
	}
	projection, found, errorValue := service.readCalendarEventProjectionByID(ctx, event.ID)
	if errorValue != nil || !found || !projection.IsDeleted {
		t.Fatalf("deleted projection found=%v deleted=%v error=%v", found, projection.IsDeleted, errorValue)
	}
	fences, errorValue := service.listCalendarPushObservationFenceUIDs(ctx, account.ID, target.CalendarURL)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, found := fences[event.UID]; found {
		t.Fatal("accepted remote deletion should clear matching observation fence")
	}
}

func TestMissingPullRollsBackEventOutboxAndFenceWhenIntentDeleteFails(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	target := activeRemoteCalendarTarget(account)
	event := newLocalTestCalendarEvent("pull-missing-transaction", "Original")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-original"`
	event.RemoteHref = "/calendars/me/pull-missing-transaction.ics"
	encoded, errorValue := encodeEventToICS(event)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event.RawICS = string(encoded)
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}

	localEvent := event
	localEvent.Title = "Local Pending"
	if errorValue := service.writeCalendarEvent(ctx, localEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil || len(rows) != 1 {
		t.Fatalf("pending rows=%d error=%v", len(rows), errorValue)
	}
	localChangedAt := time.Date(2026, 7, 16, 3, 0, 0, 0, time.UTC)
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE calendar_outbox SET created_at = ? WHERE id = ?`, localChangedAt.Format(time.RFC3339Nano), rows[0].ID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.markCalendarOutboxBatchBlocked(ctx, rows[0], localChangedAt.Format(time.RFC3339Nano), "manual review required"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.upsertCalendarRemoteEventState(ctx, calendarRemoteEventState{
		AccountID:   account.ID,
		CalendarURL: target.CalendarURL,
		EventUID:    event.UID,
		LastSeenAt:  localChangedAt.Add(time.Minute).Format(time.RFC3339Nano),
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	seedCalendarPushObservationFenceForTest(t, service, account.ID, target.CalendarURL, event.UID)
	database, errorValue = service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `
CREATE TRIGGER fail_missing_pull_intent_delete
BEFORE DELETE ON calendar_outbox
BEGIN
	SELECT RAISE(FAIL, 'forced active intent delete failure');
END`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	errorValue = service.softDeleteMissingRemoteEvent(ctx, account.ID, target, event, pendingCalendarLocalChange{}, localChangedAt.Add(2*time.Minute))
	if errorValue == nil || !strings.Contains(errorValue.Error(), "forced active intent delete failure") {
		t.Fatalf("error=%v", errorValue)
	}
	finalEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("event should remain active: found=%v error=%v", found, errorValue)
	}
	if finalEvent.Title != "Local Pending" {
		t.Fatalf("title=%q want Local Pending", finalEvent.Title)
	}
	remainingRows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil || len(remainingRows) != 1 || remainingRows[0].Status != calendarOutboxStatusBlocked {
		t.Fatalf("active rows=%+v error=%v", remainingRows, errorValue)
	}
	fences, errorValue := service.listCalendarPushObservationFenceUIDs(ctx, account.ID, target.CalendarURL)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, found := fences[event.UID]; !found {
		t.Fatal("failed remote deletion should retain matching observation fence")
	}
}
