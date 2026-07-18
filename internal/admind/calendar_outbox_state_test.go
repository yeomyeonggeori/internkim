package admind

import (
	"context"
	"testing"
)

func TestCalendarOutboxBlockedStatePersists(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("blocked-event", "Blocked")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil || len(rows) != 1 {
		t.Fatalf("pending rows: rows=%+v error=%v", rows, errorValue)
	}
	failedAt := "2026-07-15T02:00:00Z"
	if errorValue := service.markCalendarOutboxBatchBlocked(ctx, rows[0], failedAt, "remote object missing"); errorValue != nil {
		t.Fatal(errorValue)
	}
	blockedRows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(blockedRows) != 1 {
		t.Fatalf("blocked rows: got %d want 1", len(blockedRows))
	}
	if blockedRows[0].Status != calendarOutboxStatusBlocked || blockedRows[0].FailedAt != failedAt {
		t.Fatalf("blocked state mismatch: %+v", blockedRows[0])
	}
}
