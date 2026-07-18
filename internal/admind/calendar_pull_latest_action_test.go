package admind

import (
	"context"
	"testing"
	"time"
)

func TestPullConflictDeleteThenPutUsesLatestLocalOperation(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("pull-delete-then-put", "Original")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-original"`
	event.RemoteHref = "/calendars/me/pull-delete-then-put.ics"
	originalICS, errorValue := encodeEventToICS(event)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event.RawICS = string(originalICS)
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.softDeleteCalendarEvent(ctx, event.ID); errorValue != nil {
		t.Fatal(errorValue)
	}

	recreatedEvent := event
	recreatedEvent.Title = "Local Recreated"
	if errorValue := service.writeCalendarEvent(ctx, recreatedEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	rows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(rows) != 2 {
		t.Fatalf("outbox rows=%d want 2", len(rows))
	}
	deleteAt := time.Date(2026, 7, 15, 3, 0, 0, 0, time.UTC)
	putAt := deleteAt.Add(2 * time.Minute)
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE calendar_outbox SET created_at = ? WHERE id = ?`, deleteAt.Format(time.RFC3339Nano), rows[0].ID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE calendar_outbox SET created_at = ? WHERE id = ?`, putAt.Format(time.RFC3339Nano), rows[1].ID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	remoteEvent := event
	remoteEvent.Title = "Remote Later Edit"
	remoteICS := encodeCalendarTestEventWithLastModified(t, remoteEvent, putAt.Add(time.Minute))
	if errorValue := service.reconcileGoogleCalendarPull(ctx, account, []calDAVCalendarObject{{
		Path: event.RemoteHref,
		ETag: `"etag-remote-edit"`,
		Data: remoteICS,
	}}, nil); errorValue != nil {
		t.Fatal(errorValue)
	}

	finalEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("final event found=%v error=%v", found, errorValue)
	}
	if finalEvent.Title != "Remote Later Edit" {
		t.Fatalf("title=%q want Remote Later Edit", finalEvent.Title)
	}
	remainingRows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	batches := aggregateCalendarOutboxRows(remainingRows)
	if len(batches) != 1 || batches[0].Operation != calendarOutboxOperationPut {
		t.Fatalf("outbox batches=%+v want one PUT", batches)
	}
	if calendarFieldListIncludes(batches[0].ChangedFields, calendarFieldTitle) {
		t.Fatalf("remote-winning title remained pending: %+v", batches[0].ChangedFields)
	}
}

func TestPullConflictDeleteThenPutRemovesSupersededDeleteWhenRemoteWins(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("pull-delete-then-put-remote-wins", "Original")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-original"`
	event.RemoteHref = "/calendars/me/pull-delete-then-put-remote-wins.ics"
	originalICS, errorValue := encodeEventToICS(event)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event.RawICS = string(originalICS)
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.softDeleteCalendarEvent(ctx, event.ID); errorValue != nil {
		t.Fatal(errorValue)
	}

	recreatedEvent := event
	recreatedEvent.Title = "Local Recreated"
	if errorValue := service.writeCalendarEvent(ctx, recreatedEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	rows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil || len(rows) != 2 {
		t.Fatalf("outbox rows=%d error=%v", len(rows), errorValue)
	}
	deleteAt := time.Date(2026, 7, 15, 3, 0, 0, 0, time.UTC)
	putAt := deleteAt.Add(2 * time.Minute)
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE calendar_outbox SET created_at = ? WHERE id = ?`, deleteAt.Format(time.RFC3339Nano), rows[0].ID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE calendar_outbox SET changed_fields = ?, created_at = ? WHERE id = ?`, encodeChangedFields([]string{calendarFieldTitle}), putAt.Format(time.RFC3339Nano), rows[1].ID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	remoteEvent := event
	remoteEvent.Title = "Remote Later Edit"
	remoteICS := encodeCalendarTestEventWithLastModified(t, remoteEvent, putAt.Add(time.Minute))
	if errorValue := service.reconcileGoogleCalendarPull(ctx, account, []calDAVCalendarObject{{
		Path: event.RemoteHref,
		ETag: `"etag-remote-edit"`,
		Data: remoteICS,
	}}, nil); errorValue != nil {
		t.Fatal(errorValue)
	}

	remainingRows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(remainingRows) != 0 {
		t.Fatalf("remaining outbox rows=%+v want none", remainingRows)
	}
}
