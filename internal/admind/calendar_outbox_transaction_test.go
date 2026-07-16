package admind

import (
	"context"
	"strings"
	"testing"
)

func TestWriteCalendarEventRollsBackWhenOutboxInsertFails(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("outbox-write-rollback", "Original")
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	createCalendarOutboxInsertFailureTrigger(t, service, ctx)

	updatedEvent := event
	updatedEvent.Title = "Unsynced Update"
	errorValue := service.writeCalendarEvent(ctx, updatedEvent)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "forced outbox insert failure") {
		t.Fatalf("error=%v", errorValue)
	}

	storedEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("stored event found=%v error=%v", found, errorValue)
	}
	if storedEvent.Title != "Original" {
		t.Fatalf("title=%q want Original", storedEvent.Title)
	}
}

func TestSoftDeleteCalendarEventRollsBackWhenOutboxInsertFails(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("outbox-delete-rollback", "Keep Event")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-delete-rollback"`
	event.RemoteHref = "/calendars/me/outbox-delete-rollback.ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	createCalendarOutboxInsertFailureTrigger(t, service, ctx)

	errorValue := service.softDeleteCalendarEvent(ctx, event.ID)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "forced outbox insert failure") {
		t.Fatalf("error=%v", errorValue)
	}

	projection, found, errorValue := service.readCalendarEventProjectionByID(ctx, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("projection found=%v error=%v", found, errorValue)
	}
	if projection.IsDeleted {
		t.Fatal("event should remain active when delete outbox insert fails")
	}
}

func TestLocalCalendarOutboxUsesEventRevisionTimestamp(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("outbox-revision-time", "Revision Time")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatal(errorValue)
	}

	storedEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("stored event found=%v error=%v", found, errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil || len(rows) != 1 {
		t.Fatalf("rows=%d error=%v", len(rows), errorValue)
	}
	if rows[0].CreatedAt != storedEvent.UpdatedAt {
		t.Fatalf("outbox created_at=%q event updated_at=%q", rows[0].CreatedAt, storedEvent.UpdatedAt)
	}
}

func createCalendarOutboxInsertFailureTrigger(t *testing.T, service *Service, ctx context.Context) {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `
CREATE TRIGGER fail_calendar_outbox_insert
BEFORE INSERT ON calendar_outbox
BEGIN
	SELECT RAISE(FAIL, 'forced outbox insert failure');
END`)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
}
