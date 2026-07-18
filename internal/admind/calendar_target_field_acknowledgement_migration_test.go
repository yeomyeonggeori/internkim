package admind

import (
	"context"
	"testing"
	"time"
)

func TestCalendarTargetFieldAcknowledgementMigrationFallsBackToCurrentTargetEvent(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	account.SelectedCalendarID = "selected"
	account.SelectedCalendarURL = "/calendars/selected/"
	account, errorValue := service.upsertRemoteCalendarAccount(ctx, account)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	remoteModifiedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	event := newLocalTestCalendarEvent("target-ack-fallback", "Remote title")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteHref = account.SelectedCalendarURL + event.UID + ".ics"
	event.RemoteETag = `"etag-remote"`
	event.RawICS = string(encodeCalendarTestEventWithLastModified(t, event, remoteModifiedAt))
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
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
	defer database.Close()
	if _, errorValue := database.ExecContext(ctx, `DELETE FROM calendar_target_field_acknowledgements WHERE event_uid = ?`, event.UID); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `DELETE FROM calendar_settings WHERE key = ?`, calendarTargetFieldAcknowledgementMigrationKey); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := migrateCalendarTargetFieldAcknowledgements(ctx, database); errorValue != nil {
		t.Fatal(errorValue)
	}
	acknowledgements, errorValue := readCalendarTargetFieldAcknowledgements(ctx, database, account.ID, account.SelectedCalendarURL, event.UID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !acknowledgements[calendarFieldTitle].IsZero() {
		t.Fatalf("pending title was acknowledged at %s", acknowledgements[calendarFieldTitle])
	}
	if !acknowledgements[calendarFieldDescription].Equal(remoteModifiedAt) {
		t.Fatalf("description acknowledgement=%s want %s", acknowledgements[calendarFieldDescription], remoteModifiedAt)
	}
}

func TestCalendarTargetFieldAcknowledgementMigrationDoesNotReplaceRemoteStateEvidence(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	remoteModifiedAt := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	rawModifiedAt := remoteModifiedAt.Add(time.Hour)
	event := newLocalTestCalendarEvent("target-ack-state-evidence", "Remote title")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteHref = account.DefaultCalendarURL + event.UID + ".ics"
	event.RemoteETag = `"etag-remote"`
	event.RawICS = string(encodeCalendarTestEventWithLastModified(t, event, rawModifiedAt))
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.upsertCalendarRemoteEventState(ctx, calendarRemoteEventState{
		AccountID:        account.ID,
		CalendarURL:      account.DefaultCalendarURL,
		EventUID:         event.UID,
		RemoteModifiedAt: remoteModifiedAt.Format(time.RFC3339Nano),
		LastSeenAt:       rawModifiedAt.Format(time.RFC3339Nano),
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.ExecContext(ctx, `DELETE FROM calendar_target_field_acknowledgements WHERE event_uid = ?`, event.UID); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `DELETE FROM calendar_settings WHERE key = ?`, calendarTargetFieldAcknowledgementMigrationKey); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := migrateCalendarTargetFieldAcknowledgements(ctx, database); errorValue != nil {
		t.Fatal(errorValue)
	}
	acknowledgements, errorValue := readCalendarTargetFieldAcknowledgements(ctx, database, account.ID, account.DefaultCalendarURL, event.UID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !acknowledgements[calendarFieldTitle].Equal(remoteModifiedAt) {
		t.Fatalf("title acknowledgement=%s want state evidence %s", acknowledgements[calendarFieldTitle], remoteModifiedAt)
	}
}

func TestCalendarTargetFieldAcknowledgementMigrationFallsBackToEventUpdatedAt(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("target-ack-updated-at-fallback", "Remote title")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteHref = account.DefaultCalendarURL + event.UID + ".ics"
	event.RemoteETag = `"etag-remote"`
	encodedEvent, errorValue := encodeEventToICS(event)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event.RawICS = string(encodedEvent)
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	persistedEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("persisted event found=%v error=%v", found, errorValue)
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.ExecContext(ctx, `DELETE FROM calendar_target_field_acknowledgements WHERE event_uid = ?`, event.UID); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `DELETE FROM calendar_settings WHERE key = ?`, calendarTargetFieldAcknowledgementMigrationKey); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := migrateCalendarTargetFieldAcknowledgements(ctx, database); errorValue != nil {
		t.Fatal(errorValue)
	}
	acknowledgements, errorValue := readCalendarTargetFieldAcknowledgements(ctx, database, account.ID, account.DefaultCalendarURL, event.UID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	expectedAcknowledgement := parseCalendarConflictTime(persistedEvent.UpdatedAt)
	if !acknowledgements[calendarFieldTitle].Equal(expectedAcknowledgement) {
		t.Fatalf("title acknowledgement=%s want updated_at %s", acknowledgements[calendarFieldTitle], expectedAcknowledgement)
	}
}

func TestCalendarTargetFieldAcknowledgementFallbackRollsBackWithMigrationMarker(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	remoteModifiedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	event := newLocalTestCalendarEvent("target-ack-fallback-rollback", "Remote title")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteHref = account.DefaultCalendarURL + event.UID + ".ics"
	event.RemoteETag = `"etag-remote"`
	event.RawICS = string(encodeCalendarTestEventWithLastModified(t, event, remoteModifiedAt))
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.ExecContext(ctx, `DELETE FROM calendar_target_field_acknowledgements WHERE event_uid = ?`, event.UID); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `DELETE FROM calendar_settings WHERE key = ?`, calendarTargetFieldAcknowledgementMigrationKey); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `
CREATE TRIGGER fail_target_ack_fallback
BEFORE INSERT ON calendar_target_field_acknowledgements
WHEN NEW.event_uid = 'target-ack-fallback-rollback@internkim'
BEGIN
	SELECT RAISE(FAIL, 'forced target acknowledgement failure');
END`); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := migrateCalendarTargetFieldAcknowledgements(ctx, database); errorValue == nil {
		t.Fatal("expected migration failure")
	}
	if _, errorValue := database.ExecContext(ctx, `DROP TRIGGER fail_target_ack_fallback`); errorValue != nil {
		t.Fatal(errorValue)
	}
	var acknowledgementCount int
	if errorValue := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM calendar_target_field_acknowledgements WHERE event_uid = ?`, event.UID).Scan(&acknowledgementCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	var markerCount int
	if errorValue := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM calendar_settings WHERE key = ?`, calendarTargetFieldAcknowledgementMigrationKey).Scan(&markerCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if acknowledgementCount != 0 || markerCount != 0 {
		t.Fatalf("migration rollback acknowledgements=%d markers=%d", acknowledgementCount, markerCount)
	}
}
