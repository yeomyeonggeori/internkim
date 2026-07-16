package admind

import (
	"context"
	"testing"
	"time"
)

func TestCalendarFieldClockMigrationRestoresCompletedLocalDeletionForHistoricalTarget(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	calendarB := "/calendars/b/"
	remoteModifiedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	event := newLocalTestCalendarEvent("completed-delete-migration", "Delete")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteHref = account.DefaultCalendarURL + event.UID + ".ics"
	event.RemoteETag = `"etag-a"`
	event.RawICS = string(encodeCalendarTestEventWithLastModified(t, event, remoteModifiedAt))
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, calendarURL := range []string{account.DefaultCalendarURL, calendarB} {
		if errorValue := service.upsertCalendarRemoteEventState(ctx, calendarRemoteEventState{
			AccountID:        account.ID,
			CalendarURL:      calendarURL,
			EventUID:         event.UID,
			RemoteModifiedAt: remoteModifiedAt.Format(time.RFC3339Nano),
			LastSeenAt:       remoteModifiedAt.Format(time.RFC3339Nano),
		}); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if errorValue := service.softDeleteCalendarEvent(ctx, event.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	projection, found, errorValue := service.readCalendarEventProjectionByID(ctx, event.ID)
	if errorValue != nil || !found || !projection.IsDeleted {
		t.Fatalf("deleted projection found=%v deleted=%v error=%v", found, projection.IsDeleted, errorValue)
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, statement := range []string{
		`DELETE FROM calendar_outbox WHERE event_uid = ?`,
		`DELETE FROM calendar_event_field_clocks WHERE event_uid = ?`,
		`DELETE FROM calendar_target_field_acknowledgements WHERE event_uid = ?`,
	} {
		if _, errorValue := database.ExecContext(ctx, statement, event.UID); errorValue != nil {
			database.Close()
			t.Fatal(errorValue)
		}
	}
	for _, key := range []string{calendarEventFieldClockMigrationKey, calendarTargetFieldAcknowledgementMigrationKey} {
		if _, errorValue := database.ExecContext(ctx, `DELETE FROM calendar_settings WHERE key = ?`, key); errorValue != nil {
			database.Close()
			t.Fatal(errorValue)
		}
	}
	if errorValue := migrateCalendarEventFieldClocks(ctx, database); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := migrateCalendarTargetFieldAcknowledgements(ctx, database); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	fieldClocks := readCalendarFieldClocksForEventTest(t, service, event.UID)
	if !fieldClocks[calendarEventDeletionClockField].Equal(parseCalendarConflictTime(projection.DeletedAt)) {
		t.Fatalf("deletion clock=%s want %s", fieldClocks[calendarEventDeletionClockField], projection.DeletedAt)
	}
	if _, errorValue := service.saveSelectedCalendar(ctx, account, "calendar-b", "B", "writer", calendarB, time.Now()); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertCalendarTargetOutboxOperationTest(t, service, account.ID, calendarB, event.UID, calendarOutboxOperationDelete)
}

func TestCalendarFieldClockMigrationDoesNotConvertRemoteDeletionToLocalTombstone(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	deletedAt := time.Now().UTC().Truncate(time.Second)
	event := newLocalTestCalendarEvent("remote-delete-migration", "Remote delete")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteHref = account.DefaultCalendarURL + event.UID + ".ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.upsertCalendarRemoteEventState(ctx, calendarRemoteEventState{
		AccountID:         account.ID,
		CalendarURL:       account.DefaultCalendarURL,
		EventUID:          event.UID,
		LastSeenAt:        deletedAt.Add(-time.Minute).Format(time.RFC3339Nano),
		MissingDetectedAt: deletedAt.Format(time.RFC3339Nano),
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.softDeleteCalendarEventWithSource(ctx, event.ID, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `DELETE FROM calendar_event_field_clocks WHERE event_uid = ?`, event.UID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `DELETE FROM calendar_settings WHERE key = ?`, calendarEventFieldClockMigrationKey); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := migrateCalendarEventFieldClocks(ctx, database); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	fieldClocks := readCalendarFieldClocksForEventTest(t, service, event.UID)
	if !fieldClocks[calendarEventDeletionClockField].IsZero() {
		t.Fatalf("remote deletion became local tombstone=%s", fieldClocks[calendarEventDeletionClockField])
	}
}

func TestCalendarFieldClockMigrationUsesLatestDuplicateTargetStateForDeletion(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	olderStateAt := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	newerStateAt := olderStateAt.Add(time.Hour)
	event := newLocalTestCalendarEvent("duplicate-target-delete-migration", "Remote delete")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteHref = account.DefaultCalendarURL + event.UID + ".ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.softDeleteCalendarEventWithSource(ctx, event.ID, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	for _, state := range []struct {
		calendarURL       string
		missingDetectedAt string
		updatedAt         string
	}{
		{calendarURL: account.DefaultCalendarURL, updatedAt: olderStateAt.Format(time.RFC3339Nano)},
		{calendarURL: "https://calendar.google.com" + account.DefaultCalendarURL, missingDetectedAt: newerStateAt.Format(time.RFC3339Nano), updatedAt: newerStateAt.Format(time.RFC3339Nano)},
	} {
		if _, errorValue := database.ExecContext(ctx, `
INSERT INTO calendar_remote_event_sync_state(
	account_id, calendar_url, event_uid, remote_modified_at, last_seen_at, missing_detected_at, updated_at
) VALUES(?, ?, ?, '', '', ?, ?)`, account.ID, state.calendarURL, event.UID, state.missingDetectedAt, state.updatedAt); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if _, errorValue := database.ExecContext(ctx, `DELETE FROM calendar_event_field_clocks WHERE event_uid = ?`, event.UID); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `DELETE FROM calendar_settings WHERE key = ?`, calendarEventFieldClockMigrationKey); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := migrateCalendarEventFieldClocks(ctx, database); errorValue != nil {
		t.Fatal(errorValue)
	}
	fieldClocks, errorValue := readCalendarEventFieldClocksForUID(ctx, database, event.UID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !fieldClocks[calendarEventDeletionClockField].IsZero() {
		t.Fatalf("newer remote deletion state became local tombstone=%s", fieldClocks[calendarEventDeletionClockField])
	}
}
