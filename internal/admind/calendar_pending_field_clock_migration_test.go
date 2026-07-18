package admind

import (
	"context"
	"testing"
	"time"
)

func TestCalendarFieldClockMigrationKeepsPendingActionOrderDuringClockRollback(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	baselineAt := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	firstActionAt := baselineAt.Add(2 * time.Hour)
	secondActionAt := baselineAt.Add(time.Hour)
	event := newLocalTestCalendarEvent("pending-clock-rollback-migration", "Current")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteHref = account.DefaultCalendarURL + event.UID + ".ics"
	event.RemoteETag = `"etag-original"`
	event.RawICS = string(encodeCalendarTestEventWithLastModified(t, event, baselineAt))
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	for index, changedAt := range []time.Time{firstActionAt, secondActionAt} {
		changedField := calendarFieldTitle
		if index == 1 {
			changedField = calendarFieldDescription
		}
		if errorValue := enqueueCalendarOutboxWithRunner(ctx, transaction, calendarOutboxRow{
			AccountID:     account.ID,
			EventID:       event.ID,
			EventUID:      event.UID,
			Operation:     calendarOutboxOperationPut,
			ChangedFields: []string{changedField},
		}, changedAt.Format(time.RFC3339Nano)); errorValue != nil {
			transaction.Rollback()
			database.Close()
			t.Fatal(errorValue)
		}
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		database.Close()
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
	fieldClock := readCalendarFieldClocksForEventTest(t, service, event.UID)[calendarFieldDescription]
	if !fieldClock.After(firstActionAt) {
		t.Fatalf("description clock=%s first action=%s second wall clock=%s", fieldClock, firstActionAt, secondActionAt)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	batches := aggregateCalendarOutboxRows(rows)
	if len(batches) != 1 {
		t.Fatalf("batches=%+v", batches)
	}
	if !batches[0].FieldChangedAt[calendarFieldDescription].Equal(fieldClock) {
		t.Fatalf("aggregated description clock=%s want %s", batches[0].FieldChangedAt[calendarFieldDescription], fieldClock)
	}
	remoteModifiedAt := secondActionAt.Add(30 * time.Minute)
	localWinningFields := selectCalendarLocalWinningFields(
		batches[0].ChangedFields,
		[]string{calendarFieldDescription},
		batches[0].FieldChangedAt,
		remoteModifiedAt,
	)
	if !calendarFieldListIncludes(localWinningFields, calendarFieldDescription) {
		t.Fatalf("later local description lost to remote modified at %s", remoteModifiedAt)
	}
}
