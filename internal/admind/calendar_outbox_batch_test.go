package admind

import (
	"context"
	"testing"
	"time"
)

func TestPushCalendarOutboxUsesLatestFieldChangeAcrossQueuedRows(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	baselineEvent := newLocalTestCalendarEvent("queued-latest-field", "Original")
	baselineEvent.RemoteSource = remoteCalendarProviderGoogle
	baselineEvent.RemoteETag = `"etag-base"`
	baselineEvent.RemoteHref = "/calendars/me/queued-latest-field.ics"
	baselineICS, errorValue := encodeEventToICS(baselineEvent)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	baselineEvent.RawICS = string(baselineICS)
	if errorValue := service.writeCalendarEventWithSource(ctx, baselineEvent, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}

	firstLocalEvent := baselineEvent
	firstLocalEvent.Title = "First Local"
	if errorValue := service.writeCalendarEvent(ctx, firstLocalEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	latestLocalEvent := firstLocalEvent
	latestLocalEvent.Title = "Latest Local"
	if errorValue := service.writeCalendarEvent(ctx, latestLocalEvent); errorValue != nil {
		t.Fatal(errorValue)
	}

	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(rows) != 2 {
		t.Fatalf("queued rows=%d want 2", len(rows))
	}
	firstChangedAt := time.Date(2026, 7, 15, 1, 0, 0, 0, time.UTC)
	latestChangedAt := firstChangedAt.Add(2 * time.Minute)
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE calendar_outbox SET created_at = ? WHERE id = ?`, firstChangedAt.Format(time.RFC3339Nano), rows[0].ID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE calendar_outbox SET created_at = ? WHERE id = ?`, latestChangedAt.Format(time.RFC3339Nano), rows[1].ID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	remoteEvent := baselineEvent
	remoteEvent.Title = "Remote Middle"
	remoteICS := encodeCalendarTestEventWithLastModified(t, remoteEvent, firstChangedAt.Add(time.Minute))
	client := &fakeCalDAVPushClient{
		putErrorsQueue: map[string][]error{baselineEvent.RemoteHref: {errCalDAVPreconditionFailed}},
		putETags:       map[string]string{baselineEvent.RemoteHref: `"etag-latest-local"`},
		getObjects: map[string]calDAVCalendarObject{
			baselineEvent.RemoteHref: {Path: baselineEvent.RemoteHref, ETag: `"etag-remote-middle"`, Data: remoteICS},
		},
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatal(errorValue)
	}

	finalEvent, found, errorValue := service.readCalendarEventByID(ctx, baselineEvent.ID)
	if errorValue != nil || !found {
		t.Fatalf("final event found=%v error=%v", found, errorValue)
	}
	if finalEvent.Title != "Latest Local" {
		t.Fatalf("title=%q want Latest Local", finalEvent.Title)
	}
	remainingRows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(remainingRows) != 0 {
		t.Fatalf("remaining rows=%d want 0", len(remainingRows))
	}
}

func TestAggregateCalendarOutboxUsesInsertionOrderWhenClockMovesBackward(t *testing.T) {
	rows := []calendarOutboxRow{
		{
			ID:        1,
			AccountID: "account-1",
			EventID:   "event-1",
			EventUID:  "event-1@example.com",
			Operation: calendarOutboxOperationPut,
			CreatedAt: "2026-07-15T12:00:00Z",
		},
		{
			ID:        2,
			AccountID: "account-1",
			EventID:   "event-1",
			EventUID:  "event-1@example.com",
			Operation: calendarOutboxOperationDelete,
			CreatedAt: "2026-07-15T11:00:00Z",
		},
	}

	batches := aggregateCalendarOutboxRows(rows)
	if len(batches) != 1 {
		t.Fatalf("batches=%d want 1", len(batches))
	}
	if batches[0].Operation != calendarOutboxOperationDelete {
		t.Fatalf("operation=%q want %q", batches[0].Operation, calendarOutboxOperationDelete)
	}
	if len(batches[0].SourceRowIDs) != 2 || batches[0].SourceRowIDs[0] != 1 || batches[0].SourceRowIDs[1] != 2 {
		t.Fatalf("source row IDs=%v want [1 2]", batches[0].SourceRowIDs)
	}
}
