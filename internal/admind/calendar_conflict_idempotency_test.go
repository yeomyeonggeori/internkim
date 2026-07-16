package admind

import (
	"context"
	"testing"
)

func TestRecordCalendarConflictSkipsDuplicateActiveConflict(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	for range 2 {
		if errorValue := service.recordCalendarConflict(ctx, "event-1", "event-1@example.com", calendarFieldTitle, "Local", "Remote"); errorValue != nil {
			t.Fatal(errorValue)
		}
	}

	conflicts, errorValue := service.listActiveCalendarConflicts(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(conflicts) != 1 {
		t.Fatalf("active conflicts=%d want 1", len(conflicts))
	}
	firstConflictID := conflicts[0].ID
	if errorValue := service.dismissCalendarConflict(ctx, firstConflictID); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.recordCalendarConflict(ctx, "event-1", "event-1@example.com", calendarFieldTitle, "Local", "Remote"); errorValue != nil {
		t.Fatal(errorValue)
	}
	conflicts, errorValue = service.listActiveCalendarConflicts(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(conflicts) != 1 {
		t.Fatalf("active conflicts after dismiss=%d want 1", len(conflicts))
	}
	if conflicts[0].ID == firstConflictID {
		t.Fatal("dismissed conflict was not replaced")
	}
}
