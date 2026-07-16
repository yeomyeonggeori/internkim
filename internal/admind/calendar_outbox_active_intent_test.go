package admind

import (
	"context"
	"testing"
	"time"
)

func TestBlockedCalendarOutboxPutPreservesLocalFieldDuringPull(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	baselineEvent := newLocalTestCalendarEvent("blocked-put-pull", "Original")
	baselineEvent.RemoteSource = remoteCalendarProviderGoogle
	baselineEvent.RemoteETag = `"etag-original"`
	baselineEvent.RemoteHref = "/calendars/me/blocked-put-pull.ics"
	encoded, errorValue := encodeEventToICS(baselineEvent)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	baselineEvent.RawICS = string(encoded)
	if errorValue := service.writeCalendarEventWithSource(ctx, baselineEvent, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}

	localEvent := baselineEvent
	localEvent.Title = "Local Pending"
	if errorValue := service.writeCalendarEvent(ctx, localEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil || len(rows) != 1 {
		t.Fatalf("pending rows=%d error=%v", len(rows), errorValue)
	}
	localChangedAt := time.Date(2026, 7, 16, 1, 0, 0, 0, time.UTC)
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

	remoteEvent := baselineEvent
	remoteEvent.Title = "Remote Stale"
	remoteICS := encodeCalendarTestEventWithLastModified(t, remoteEvent, localChangedAt.Add(-time.Minute))
	if errorValue := service.reconcileGoogleCalendarPull(ctx, account, []calDAVCalendarObject{{
		Path: baselineEvent.RemoteHref,
		ETag: `"etag-remote"`,
		Data: remoteICS,
	}}, nil); errorValue != nil {
		t.Fatal(errorValue)
	}

	finalEvent, found, errorValue := service.readCalendarEventByID(ctx, baselineEvent.ID)
	if errorValue != nil || !found {
		t.Fatalf("final event found=%v error=%v", found, errorValue)
	}
	if finalEvent.Title != "Local Pending" {
		t.Fatalf("title=%q want Local Pending", finalEvent.Title)
	}
	remainingRows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil || len(remainingRows) != 1 {
		t.Fatalf("active rows=%d error=%v", len(remainingRows), errorValue)
	}
	if remainingRows[0].Status != calendarOutboxStatusBlocked {
		t.Fatalf("status=%q want blocked", remainingRows[0].Status)
	}
}
