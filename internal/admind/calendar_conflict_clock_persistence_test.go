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

func TestLocalCalendarDeleteAdvancesTheLogicalClockPastAnEarlierRevision(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	event := newLocalTestCalendarEvent("logical-clock-delete", "Delete")
	if errorValue := service.writeCalendarEvent(contextValue, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	laterThanTheWallClock := time.Date(2036, 7, 16, 9, 0, 0, 0, time.UTC)
	updateCalendarEventConflictTimes(t, service, event.ID, laterThanTheWallClock, time.Time{})

	if errorValue := service.softDeleteCalendarEvent(contextValue, event.ID); errorValue != nil {
		t.Fatal(errorValue)
	}

	projection, found, errorValue := service.readCalendarEventProjectionByID(contextValue, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("projection found=%v error=%v", found, errorValue)
	}
	deletedAt := parseCalendarConflictTime(projection.DeletedAt)
	if !deletedAt.After(laterThanTheWallClock) {
		t.Fatalf("deleted_at=%s must beat the row's own updated_at=%s, or a clock correction makes a delete look older than the event", deletedAt, laterThanTheWallClock)
	}
	if projection.Event.UpdatedAt != projection.DeletedAt {
		t.Fatalf("updated_at=%q deleted_at=%q, and one delete allocates one logical time", projection.Event.UpdatedAt, projection.DeletedAt)
	}
	if logicalTime := readCalendarConflictLogicalTime(t, service, event.UID); logicalTime != deletedAt {
		t.Fatalf("logical time=%s deleted_at=%s", logicalTime, deletedAt)
	}
}
