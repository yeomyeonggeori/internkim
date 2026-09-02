package admind

import (
	"context"
	"testing"
	"time"
)

func TestLocalCalendarWriteAdvancesPastLegacyRevisionDuringWallClockRollback(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	event := newLocalTestCalendarEvent("logical-clock-legacy-write", "Original")
	if errorValue := service.writeCalendarEvent(contextValue, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	legacyRevision := time.Date(2036, 7, 16, 1, 0, 0, 0, time.UTC)
	updateCalendarEventConflictTimes(t, service, event.ID, legacyRevision, time.Time{})

	updatedEvent := event
	updatedEvent.Title = "Later Local Edit"
	if errorValue := service.writeCalendarEvent(contextValue, updatedEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	storedEvent, found, errorValue := service.readCalendarEventByID(contextValue, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("stored event found=%v error=%v", found, errorValue)
	}
	storedRevision := parseCalendarConflictTime(storedEvent.UpdatedAt)
	if !storedRevision.After(legacyRevision) {
		t.Fatalf("stored revision=%s legacy revision=%s", storedRevision, legacyRevision)
	}
	logicalTime := readCalendarConflictLogicalTime(t, service, event.UID)
	if logicalTime != storedRevision {
		t.Fatalf("logical time=%s event updated_at=%s", logicalTime, storedRevision)
	}
}
