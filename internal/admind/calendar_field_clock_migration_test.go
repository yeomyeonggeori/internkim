package admind

import (
	"context"
	"testing"
	"time"
)

func TestCalendarEventFieldClockMigrationPreservesRemoteBaselineAndPendingEdit(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	remoteModifiedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	event := newLocalTestCalendarEvent("field-clock-migration-edit", "Remote title")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteHref = account.DefaultCalendarURL + event.UID + ".ics"
	event.RemoteETag = `"etag-remote"`
	event.RawICS = string(encodeCalendarTestEventWithLastModified(t, event, remoteModifiedAt))
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	event.Title = "Local title"
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil || len(rows) != 1 {
		t.Fatalf("outbox rows=%+v error=%v", rows, errorValue)
	}
	localChangedAt := parseCalendarConflictTime(rows[0].CreatedAt)
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
	if !fieldClocks[calendarFieldTitle].Equal(localChangedAt) {
		t.Fatalf("title clock=%s want %s", fieldClocks[calendarFieldTitle], localChangedAt)
	}
	if !fieldClocks[calendarFieldDescription].Equal(remoteModifiedAt) {
		t.Fatalf("description clock=%s want %s", fieldClocks[calendarFieldDescription], remoteModifiedAt)
	}
}

func TestCalendarEventFieldClockMigrationRestoresPendingLocalDeletion(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("field-clock-migration-delete", "Delete")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteHref = account.DefaultCalendarURL + event.UID + ".ics"
	event.RemoteETag = `"etag-remote"`
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.softDeleteCalendarEvent(ctx, event.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	projection, found, errorValue := service.readCalendarEventProjectionByID(ctx, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("projection found=%v error=%v", found, errorValue)
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
	if !fieldClocks[calendarEventDeletionClockField].Equal(parseCalendarConflictTime(projection.DeletedAt)) {
		t.Fatalf("deletion clock=%s want %s", fieldClocks[calendarEventDeletionClockField], projection.DeletedAt)
	}
}

func TestCalendarTargetFieldAcknowledgementMigrationExcludesPendingFields(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	remoteModifiedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	event := newLocalTestCalendarEvent("target-ack-migration", "Remote title")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteHref = account.DefaultCalendarURL + event.UID + ".ics"
	event.RemoteETag = `"etag-remote"`
	event.RawICS = string(encodeCalendarTestEventWithLastModified(t, event, remoteModifiedAt))
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.upsertCalendarRemoteEventState(ctx, calendarRemoteEventState{AccountID: account.ID, CalendarURL: account.DefaultCalendarURL, EventUID: event.UID, RemoteModifiedAt: remoteModifiedAt.Format(time.RFC3339Nano), LastSeenAt: time.Now().UTC().Format(time.RFC3339Nano)}); errorValue != nil {
		t.Fatal(errorValue)
	}
	event.Title = "Local title"
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `DELETE FROM calendar_target_field_acknowledgements WHERE event_uid = ?`, event.UID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `DELETE FROM calendar_settings WHERE key = ?`, calendarTargetFieldAcknowledgementMigrationKey); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := migrateCalendarTargetFieldAcknowledgements(ctx, database); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	acknowledgements, errorValue := readCalendarTargetFieldAcknowledgements(ctx, database, account.ID, account.DefaultCalendarURL, event.UID)
	if errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	if !acknowledgements[calendarFieldTitle].IsZero() {
		t.Fatalf("pending title was acknowledged at %s", acknowledgements[calendarFieldTitle])
	}
	if !acknowledgements[calendarFieldDescription].Equal(remoteModifiedAt) {
		t.Fatalf("description acknowledgement=%s want %s", acknowledgements[calendarFieldDescription], remoteModifiedAt)
	}
}

func TestCalendarTargetFieldAcknowledgementMigrationPreservesHistoricalTargetEvidence(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	calendarA := account.DefaultCalendarURL
	calendarB := "/calendars/b/"
	observedAtA := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	observedAtB := observedAtA.Add(time.Hour)
	event := newLocalTestCalendarEvent("target-ack-migration-history", "Remote B")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteHref = calendarB + event.UID + ".ics"
	event.RemoteETag = `"etag-b"`
	event.RawICS = string(encodeCalendarTestEventWithLastModified(t, event, observedAtB))
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, state := range []calendarRemoteEventState{
		{AccountID: account.ID, CalendarURL: calendarA, EventUID: event.UID, RemoteModifiedAt: observedAtA.Format(time.RFC3339Nano), LastSeenAt: observedAtA.Format(time.RFC3339Nano)},
		{AccountID: account.ID, CalendarURL: calendarB, EventUID: event.UID, RemoteModifiedAt: observedAtB.Format(time.RFC3339Nano), LastSeenAt: observedAtB.Format(time.RFC3339Nano)},
	} {
		if errorValue := service.upsertCalendarRemoteEventState(ctx, state); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if errorValue := service.softDeleteCalendarEventWithSource(ctx, event.ID, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `DELETE FROM calendar_target_field_acknowledgements WHERE event_uid = ?`, event.UID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `DELETE FROM calendar_settings WHERE key = ?`, calendarTargetFieldAcknowledgementMigrationKey); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := migrateCalendarTargetFieldAcknowledgements(ctx, database); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	acknowledgementsA, errorValue := readCalendarTargetFieldAcknowledgements(ctx, database, account.ID, calendarA, event.UID)
	if errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	acknowledgementsB, errorValue := readCalendarTargetFieldAcknowledgements(ctx, database, account.ID, calendarB, event.UID)
	database.Close()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !acknowledgementsA[calendarFieldTitle].Equal(observedAtA) {
		t.Fatalf("calendar A acknowledgement=%s want %s", acknowledgementsA[calendarFieldTitle], observedAtA)
	}
	if !acknowledgementsB[calendarFieldTitle].Equal(observedAtB) {
		t.Fatalf("calendar B acknowledgement=%s want %s", acknowledgementsB[calendarFieldTitle], observedAtB)
	}
}
