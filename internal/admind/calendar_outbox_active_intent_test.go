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
	database, errorValue = service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE calendar_outbox SET attempt_count = 4, last_attempted_at = ? WHERE id = ?`, localChangedAt.Format(time.RFC3339Nano), rows[0].ID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
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
	if remainingRows[0].AttemptCount != 4 || remainingRows[0].FailedAt != localChangedAt.Format(time.RFC3339Nano) || remainingRows[0].LastError != "manual review required" {
		t.Fatalf("retry state attempt_count=%d failed_at=%q last_error=%q", remainingRows[0].AttemptCount, remainingRows[0].FailedAt, remainingRows[0].LastError)
	}
	if remainingRows[0].IfMatchETag != `"etag-remote"` || remainingRows[0].RemoteHref != baselineEvent.RemoteHref {
		t.Fatalf("remote state etag=%q href=%q", remainingRows[0].IfMatchETag, remainingRows[0].RemoteHref)
	}
}

func TestMissingPullKeepsBlockedLocalPutAndClearsRemoteState(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	target := activeRemoteCalendarTarget(account)
	event := newLocalTestCalendarEvent("blocked-put-missing-pull", "Original")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-original"`
	event.RemoteHref = "/calendars/me/blocked-put-missing-pull.ics"
	encoded, errorValue := encodeEventToICS(event)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event.RawICS = string(encoded)
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	lastSeenAt := time.Date(2026, 7, 16, 1, 0, 0, 0, time.UTC)
	missingDetectedAt := lastSeenAt.Add(time.Hour)
	if errorValue := service.upsertCalendarRemoteEventState(ctx, calendarRemoteEventState{
		AccountID:         account.ID,
		CalendarURL:       target.CalendarURL,
		EventUID:          event.UID,
		LastSeenAt:        lastSeenAt.Format(time.RFC3339Nano),
		MissingDetectedAt: missingDetectedAt.Format(time.RFC3339Nano),
	}); errorValue != nil {
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
	localChangedAt := missingDetectedAt.Add(time.Hour)
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

	if errorValue := service.softDeleteMissingRemoteEvent(ctx, account.ID, target, event, pendingCalendarLocalChange{}, localChangedAt.Add(time.Hour)); errorValue != nil {
		t.Fatal(errorValue)
	}
	finalEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("local event found=%v error=%v", found, errorValue)
	}
	if finalEvent.Title != "Local Pending" {
		t.Fatalf("title=%q want Local Pending", finalEvent.Title)
	}
	remainingRows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil || len(remainingRows) != 1 {
		t.Fatalf("active rows=%d error=%v", len(remainingRows), errorValue)
	}
	if remainingRows[0].Status != calendarOutboxStatusBlocked || remainingRows[0].RemoteHref != "" || remainingRows[0].IfMatchETag != "" {
		t.Fatalf("blocked put state=%+v", remainingRows[0])
	}
}
