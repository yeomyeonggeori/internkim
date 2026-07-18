package admind

import (
	"context"
	"testing"
)

func TestWriteCalendarEventLocalEnqueuesPutOutbox(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("local-new-1", "Local New")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("write: %v", errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, "")
	if errorValue != nil {
		t.Fatalf("list outbox: %v", errorValue)
	}
	if len(rows) != 1 {
		t.Fatalf("outbox rows: got %d, want 1", len(rows))
	}
	if rows[0].Operation != calendarOutboxOperationPut {
		t.Errorf("operation: got %q", rows[0].Operation)
	}
	if rows[0].EventID != event.ID {
		t.Errorf("event id: got %q", rows[0].EventID)
	}
	if rows[0].RemoteHref != "" {
		t.Errorf("remote href should be empty for new event: %q", rows[0].RemoteHref)
	}
}

func TestWriteCalendarEventFromPullSkipsOutbox(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("pull-1", "Pulled")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-1"`
	event.RemoteHref = "/calendars/me/pull-1.ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatalf("write pull: %v", errorValue)
	}
	rows, _ := service.listPendingCalendarOutbox(ctx, "")
	if len(rows) != 0 {
		t.Errorf("pull source should not enqueue, got %d rows", len(rows))
	}
}

func TestWriteCalendarEventLocalSkipsOutboxWhenAccountMissing(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	event := newLocalTestCalendarEvent("orphan-1", "No account")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("write: %v", errorValue)
	}
	rows, _ := service.listPendingCalendarOutbox(ctx, "")
	if len(rows) != 0 {
		t.Errorf("should not enqueue without account, got %d rows", len(rows))
	}
}

func TestSoftDeleteCalendarEventLocalEnqueuesDeleteOutbox(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("local-del", "Local Delete")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-del"`
	event.RemoteHref = "/calendars/me/local-del.ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed write: %v", errorValue)
	}
	if errorValue := service.softDeleteCalendarEvent(ctx, event.ID); errorValue != nil {
		t.Fatalf("soft delete: %v", errorValue)
	}
	rows, _ := service.listPendingCalendarOutbox(ctx, "")
	if len(rows) != 1 {
		t.Fatalf("outbox rows: got %d, want 1", len(rows))
	}
	if rows[0].Operation != calendarOutboxOperationDelete {
		t.Errorf("operation: got %q", rows[0].Operation)
	}
	if rows[0].RemoteHref != event.RemoteHref {
		t.Errorf("remote href: got %q", rows[0].RemoteHref)
	}
	if rows[0].IfMatchETag != event.RemoteETag {
		t.Errorf("if-match etag: got %q", rows[0].IfMatchETag)
	}
}
