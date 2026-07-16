package admind

import (
	"context"
	"testing"
	"time"
)

func TestLocalCalendarWriteAdvancesPastLegacyRevisionDuringWallClockRollback(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("logical-clock-legacy-write", "Original")
	if errorValue := service.writeCalendarEventWithSource(contextValue, event, calendarSourcePull); errorValue != nil {
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
	rows, errorValue := service.listPendingCalendarOutbox(contextValue, account.ID)
	if errorValue != nil || len(rows) != 1 {
		t.Fatalf("outbox rows=%d error=%v", len(rows), errorValue)
	}
	storedRevision := parseCalendarConflictTime(storedEvent.UpdatedAt)
	if !storedRevision.After(legacyRevision) {
		t.Fatalf("stored revision=%s legacy revision=%s", storedRevision, legacyRevision)
	}
	if rows[0].CreatedAt != storedEvent.UpdatedAt {
		t.Fatalf("outbox created_at=%q event updated_at=%q", rows[0].CreatedAt, storedEvent.UpdatedAt)
	}
	logicalTime := readCalendarConflictLogicalTime(t, service, event.UID)
	if logicalTime != storedRevision {
		t.Fatalf("logical time=%s event updated_at=%s", logicalTime, storedRevision)
	}
}

func TestLocalCalendarDeleteUsesOneLogicalTimeForEventAndOutbox(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("logical-clock-delete", "Delete")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteHref = "/calendars/me/logical-clock-delete.ics"
	if errorValue := service.writeCalendarEventWithSource(contextValue, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	legacyRevision := time.Date(2036, 7, 16, 9, 0, 0, 0, time.UTC)
	updateCalendarEventConflictTimes(t, service, event.ID, legacyRevision, time.Time{})
	if errorValue := service.softDeleteCalendarEvent(contextValue, event.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	projection, found, errorValue := service.readCalendarEventProjectionByID(contextValue, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("projection found=%v error=%v", found, errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(contextValue, account.ID)
	if errorValue != nil || len(rows) != 1 {
		t.Fatalf("outbox rows=%d error=%v", len(rows), errorValue)
	}
	deletedAt := parseCalendarConflictTime(projection.DeletedAt)
	if !deletedAt.After(legacyRevision) {
		t.Fatalf("deleted_at=%s legacy revision=%s", deletedAt, legacyRevision)
	}
	if rows[0].CreatedAt != projection.DeletedAt {
		t.Fatalf("outbox created_at=%q deleted_at=%q", rows[0].CreatedAt, projection.DeletedAt)
	}
	logicalTime := readCalendarConflictLogicalTime(t, service, event.UID)
	if logicalTime != deletedAt {
		t.Fatalf("logical time=%s deleted_at=%s", logicalTime, deletedAt)
	}
}

func TestLocalCalendarWriteRollsBackLogicalTimeWithOutbox(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("logical-clock-write-rollback", "Original")
	if errorValue := service.writeCalendarEventWithSource(contextValue, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	initialLogicalTime := allocateCommittedCalendarConflictTime(t, service, event.UID, time.Date(2036, 7, 16, 10, 0, 0, 0, time.UTC))
	createCalendarOutboxInsertFailureTrigger(t, service, contextValue)
	updatedEvent := event
	updatedEvent.Title = "Rollback"
	if errorValue := service.writeCalendarEvent(contextValue, updatedEvent); errorValue == nil {
		t.Fatal("expected outbox failure")
	}
	if logicalTime := readCalendarConflictLogicalTime(t, service, event.UID); logicalTime != initialLogicalTime {
		t.Fatalf("logical time=%s initial=%s", logicalTime, initialLogicalTime)
	}
	storedEvent, found, errorValue := service.readCalendarEventByID(contextValue, event.ID)
	if errorValue != nil || !found || storedEvent.Title != event.Title {
		t.Fatalf("stored event found=%v title=%q error=%v", found, storedEvent.Title, errorValue)
	}
}

func TestLocalCalendarDeleteRollsBackLogicalTimeWithOutbox(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("logical-clock-delete-rollback", "Original")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteHref = "/calendars/me/logical-clock-delete-rollback.ics"
	if errorValue := service.writeCalendarEventWithSource(contextValue, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	initialLogicalTime := allocateCommittedCalendarConflictTime(t, service, event.UID, time.Date(2036, 7, 16, 11, 0, 0, 0, time.UTC))
	createCalendarOutboxInsertFailureTrigger(t, service, contextValue)
	if errorValue := service.softDeleteCalendarEvent(contextValue, event.ID); errorValue == nil {
		t.Fatal("expected outbox failure")
	}
	if logicalTime := readCalendarConflictLogicalTime(t, service, event.UID); logicalTime != initialLogicalTime {
		t.Fatalf("logical time=%s initial=%s", logicalTime, initialLogicalTime)
	}
	projection, found, errorValue := service.readCalendarEventProjectionByID(contextValue, event.ID)
	if errorValue != nil || !found || projection.IsDeleted {
		t.Fatalf("projection found=%v deleted=%v error=%v", found, projection.IsDeleted, errorValue)
	}
}

func TestCalendarBackfillAdvancesPastLegacyEventRevision(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	event := newLocalTestCalendarEvent("logical-clock-backfill", "Backfill")
	if errorValue := service.writeCalendarEventWithSource(contextValue, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	legacyRevision := time.Date(2036, 7, 16, 12, 0, 0, 0, time.UTC)
	updateCalendarEventConflictTimes(t, service, event.ID, legacyRevision, time.Time{})
	account, errorValue := service.upsertRemoteCalendarAccount(contextValue, remoteCalendarAccount{
		ID:                 "logical-clock-backfill-account",
		Provider:           remoteCalendarProviderGoogle,
		AccountEmail:       "logical-clock@example.com",
		DefaultCalendarURL: "/calendars/default/",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.saveSelectedCalendar(contextValue, account, "selected", "Selected", "writer", "/calendars/selected/", time.Now()); errorValue != nil {
		t.Fatal(errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(contextValue, account.ID)
	if errorValue != nil || len(rows) != 1 {
		t.Fatalf("outbox rows=%d error=%v", len(rows), errorValue)
	}
	createdAt := parseCalendarConflictTime(rows[0].CreatedAt)
	if !createdAt.After(legacyRevision) {
		t.Fatalf("outbox created_at=%s legacy revision=%s", createdAt, legacyRevision)
	}
	logicalTime := readCalendarConflictLogicalTime(t, service, event.UID)
	if logicalTime != createdAt {
		t.Fatalf("logical time=%s outbox created_at=%s", logicalTime, createdAt)
	}
}

func TestCalendarBackfillAdvancesPastGlobalBoundaryDuringWallClockRollback(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	event := newLocalTestCalendarEvent("logical-clock-backfill-global", "Backfill")
	if errorValue := service.writeCalendarEventWithSource(contextValue, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	snapshotBoundary, errorValue := service.reserveCalendarConflictCandidateTime(contextValue, time.Date(2036, 7, 16, 13, 0, 0, 0, time.UTC))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	account, errorValue := service.upsertRemoteCalendarAccount(contextValue, remoteCalendarAccount{
		ID:                 "logical-clock-backfill-global-account",
		Provider:           remoteCalendarProviderGoogle,
		AccountEmail:       "logical-clock-global@example.com",
		DefaultCalendarURL: "/calendars/default/",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.saveSelectedCalendar(contextValue, account, "selected", "Selected", "writer", "/calendars/selected/", time.Now()); errorValue != nil {
		t.Fatal(errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(contextValue, account.ID)
	if errorValue != nil || len(rows) != 1 {
		t.Fatalf("outbox rows=%d error=%v", len(rows), errorValue)
	}
	createdAt := parseCalendarConflictTime(rows[0].CreatedAt)
	if !createdAt.After(snapshotBoundary) {
		t.Fatalf("outbox created at=%s snapshot boundary=%s", createdAt, snapshotBoundary)
	}
}
