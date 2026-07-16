package admind

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestDeleteConflictRemoteRestoreRollsBackAllStateOnPersistenceFailure(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("delete-conflict-rollback", "Original")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-original"`
	event.RemoteHref = "/calendars/me/delete-conflict-rollback.ics"
	encoded, errorValue := encodeEventToICS(event)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event.RawICS = string(encoded)
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.softDeleteCalendarEvent(ctx, event.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	deletedAt := time.Date(2026, 7, 15, 4, 0, 0, 0, time.UTC)
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE calendar_events SET deleted_at = ? WHERE id = ?`, deletedAt.Format(time.RFC3339Nano), event.ID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `CREATE TRIGGER fail_delete_conflict_outbox_cleanup BEFORE DELETE ON calendar_outbox BEGIN SELECT RAISE(ABORT, 'outbox cleanup blocked'); END`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil || len(rows) != 1 {
		t.Fatalf("rows=%d error=%v", len(rows), errorValue)
	}
	remoteEvent := event
	remoteEvent.Title = "Remote Edit"
	remoteICS := encodeCalendarTestEventWithLastModified(t, remoteEvent, deletedAt.Add(time.Minute))
	client := &fakeCalDAVPushClient{
		getObjects: map[string]calDAVCalendarObject{
			event.RemoteHref: {Path: event.RemoteHref, ETag: `"etag-remote-edit"`, Data: remoteICS},
		},
	}
	observedAt := deletedAt.Add(2 * time.Minute)
	if errorValue := service.reconcileCalendarLocalDeletionDuringPush(ctx, account, client, rows[0], observedAt); errorValue == nil {
		t.Fatal("expected persistence failure")
	}

	projection, found, errorValue := service.readCalendarEventProjectionByID(ctx, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("projection found=%v error=%v", found, errorValue)
	}
	if !projection.IsDeleted {
		t.Fatal("event restore should roll back")
	}
	remainingRows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil || len(remainingRows) != 1 {
		t.Fatalf("remaining rows=%d error=%v", len(remainingRows), errorValue)
	}
	target := activeRemoteCalendarTarget(account)
	if _, found, errorValue := service.readCalendarRemoteEventState(ctx, account.ID, target.CalendarURL, event.UID); errorValue != nil || found {
		t.Fatalf("remote state found=%v error=%v", found, errorValue)
	}
}

func TestPullDeleteConflictRestorationRollsBackEventAndOutboxTogether(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("pull-delete-conflict-rollback", "Original")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-original"`
	event.RemoteHref = "/calendars/me/pull-delete-conflict-rollback.ics"
	encoded, errorValue := encodeEventToICS(event)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event.RawICS = string(encoded)
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.softDeleteCalendarEvent(ctx, event.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	deletedAt := time.Date(2026, 7, 15, 5, 0, 0, 0, time.UTC)
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE calendar_events SET deleted_at = ? WHERE id = ?`, deletedAt.Format(time.RFC3339Nano), event.ID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `CREATE TRIGGER fail_pull_delete_conflict_restore BEFORE UPDATE ON calendar_events BEGIN SELECT RAISE(ABORT, 'event restore blocked'); END`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	remoteEvent := event
	remoteEvent.Title = "Remote Edit"
	remoteEvent.RemoteETag = `"etag-remote-edit"`
	remoteEvent.RemoteModifiedAt = deletedAt.Add(time.Minute).Format(time.RFC3339Nano)
	remoteEvent.RawICS = string(encodeCalendarTestEventWithLastModified(t, remoteEvent, deletedAt.Add(time.Minute)))
	errorValue = service.applyPulledRemoteEvent(ctx, account, event, true, remoteEvent)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "event restore blocked") {
		t.Fatalf("error=%v", errorValue)
	}

	projection, found, errorValue := service.readCalendarEventProjectionByID(ctx, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("projection found=%v error=%v", found, errorValue)
	}
	if !projection.IsDeleted {
		t.Fatal("event restore should roll back")
	}
	remainingRows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil || len(remainingRows) != 1 {
		t.Fatalf("remaining rows=%d error=%v", len(remainingRows), errorValue)
	}
}
