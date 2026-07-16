package admind

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

const (
	calendarEventFieldClockMigrationKey = "calendar_event_field_clocks_v1"
	calendarEventDeletionClockField     = "__deleted__"
)

func ensureCalendarEventFieldClocksTable(ctx context.Context, database *sql.DB) error {
	if _, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS calendar_event_field_clocks (
	event_uid TEXT NOT NULL,
	field TEXT NOT NULL,
	changed_at TEXT NOT NULL,
	PRIMARY KEY(event_uid, field)
)`); errorValue != nil {
		return errorValue
	}
	return migrateCalendarEventFieldClocks(ctx, database)
}

func migrateCalendarEventFieldClocks(ctx context.Context, database *sql.DB) error {
	var migrationValue string
	errorValue := database.QueryRowContext(ctx, "SELECT value FROM calendar_settings WHERE key = ?", calendarEventFieldClockMigrationKey).Scan(&migrationValue)
	if errorValue == nil {
		return nil
	}
	if !errors.Is(errorValue, sql.ErrNoRows) {
		return errorValue
	}
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := seedExistingCalendarFieldClocks(ctx, transaction); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := seedCompletedLocalDeletionClocks(ctx, transaction); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := seedPendingCalendarFieldClocks(ctx, transaction); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if _, errorValue := transaction.ExecContext(ctx, "INSERT OR IGNORE INTO calendar_settings(key, value) VALUES(?, ?)", calendarEventFieldClockMigrationKey, "complete"); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

func seedExistingCalendarFieldClocks(ctx context.Context, transaction *sql.Tx) error {
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT uid, raw_ics, remote_href, updated_at
FROM calendar_events
WHERE deleted_at = ''`)
	if errorValue != nil {
		return errorValue
	}
	type existingEvent struct {
		uid        string
		rawICS     string
		remoteHref string
		updatedAt  string
	}
	events := []existingEvent{}
	for rows.Next() {
		var event existingEvent
		if errorValue := rows.Scan(&event.uid, &event.rawICS, &event.remoteHref, &event.updatedAt); errorValue != nil {
			rows.Close()
			return errorValue
		}
		events = append(events, event)
	}
	if errorValue := rows.Close(); errorValue != nil {
		return errorValue
	}
	for _, event := range events {
		baseline := calendarExistingEventFieldClockBaseline(event.rawICS, event.remoteHref, event.updatedAt)
		if baseline.IsZero() {
			continue
		}
		if errorValue := persistCalendarEventFieldClocks(ctx, transaction, event.uid, calendarAllUserEditableFields(), baseline.Format(time.RFC3339Nano)); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func seedPendingCalendarFieldClocks(ctx context.Context, transaction *sql.Tx) error {
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT id, event_uid, operation, changed_fields, created_at
FROM calendar_outbox
WHERE operation IN (?, ?) AND status IN (?, ?)
ORDER BY id`, calendarOutboxOperationPut, calendarOutboxOperationDelete, calendarOutboxStatusPending, calendarOutboxStatusBlocked)
	if errorValue != nil {
		return errorValue
	}
	type pendingChange struct {
		id               int64
		eventUID         string
		operation        string
		changedFieldsRaw string
		changedAt        string
	}
	changes := []pendingChange{}
	for rows.Next() {
		var change pendingChange
		if errorValue := rows.Scan(&change.id, &change.eventUID, &change.operation, &change.changedFieldsRaw, &change.changedAt); errorValue != nil {
			rows.Close()
			return errorValue
		}
		changes = append(changes, change)
	}
	if errorValue := rows.Close(); errorValue != nil {
		return errorValue
	}
	for _, change := range changes {
		changedAt, errorValue := nextMigratedCalendarActionTime(ctx, transaction, change.eventUID, parseCalendarConflictTime(change.changedAt))
		if errorValue != nil {
			return errorValue
		}
		fields := []string{calendarEventDeletionClockField}
		if change.operation == calendarOutboxOperationPut {
			if errorValue := deleteCalendarEventDeletionClock(ctx, transaction, change.eventUID); errorValue != nil {
				return errorValue
			}
			fields = normalizeCalendarOutboxPutChangedFields(decodeChangedFields(change.changedFieldsRaw))
		}
		if errorValue := persistCalendarEventFieldClocks(ctx, transaction, change.eventUID, fields, changedAt.Format(time.RFC3339Nano)); errorValue != nil {
			return errorValue
		}
		if !changedAt.Equal(parseCalendarConflictTime(change.changedAt)) {
			if _, errorValue := transaction.ExecContext(ctx, `UPDATE calendar_outbox SET created_at = ? WHERE id = ?`, changedAt.Format(time.RFC3339Nano), change.id); errorValue != nil {
				return errorValue
			}
		}
	}
	return nil
}

func nextMigratedCalendarActionTime(ctx context.Context, queryRunner calendarSQLRunner, eventUID string, candidate time.Time) (time.Time, error) {
	fieldClocks, errorValue := readCalendarEventFieldClocksForUID(ctx, queryRunner, eventUID)
	if errorValue != nil {
		return time.Time{}, errorValue
	}
	latest := time.Time{}
	for _, changedAt := range fieldClocks {
		if changedAt.After(latest) {
			latest = changedAt
		}
	}
	if candidate.After(latest) {
		return candidate, nil
	}
	return latest.Add(time.Nanosecond), nil
}

func calendarExistingEventFieldClockBaseline(rawICS string, remoteHref string, updatedAt string) time.Time {
	remoteEvent := decodeCalendarEventFromRawICS(rawICS, remoteHref, "")
	remoteModifiedAt := parseCalendarConflictTime(remoteEvent.RemoteModifiedAt)
	if !remoteModifiedAt.IsZero() {
		return remoteModifiedAt
	}
	return parseCalendarConflictTime(updatedAt)
}

func persistCalendarEventFieldClocks(ctx context.Context, transaction *sql.Tx, eventUID string, fields []string, changedAt string) error {
	if strings.TrimSpace(eventUID) == "" || len(fields) == 0 {
		return nil
	}
	values := map[string]string{}
	for _, field := range fields {
		values[field] = strings.TrimSpace(changedAt)
	}
	return persistCalendarEventFieldClockValues(ctx, transaction, eventUID, values)
}

func persistAdvancedCalendarEventFieldClocks(ctx context.Context, transaction *sql.Tx, eventUID string, fields []string, candidate time.Time, floors map[string]time.Time) (map[string]time.Time, error) {
	existing, errorValue := readCalendarEventFieldClocksForUID(ctx, transaction, eventUID)
	if errorValue != nil {
		return nil, errorValue
	}
	changedAtByField := map[string]time.Time{}
	values := map[string]string{}
	for _, field := range fields {
		floor := existing[field]
		if floors[field].After(floor) {
			floor = floors[field]
		}
		changedAt := candidate
		if !changedAt.After(floor) {
			changedAt = floor.Add(time.Nanosecond)
		}
		changedAtByField[field] = changedAt
		values[field] = changedAt.Format(time.RFC3339Nano)
	}
	if errorValue := persistCalendarEventFieldClockValues(ctx, transaction, eventUID, values); errorValue != nil {
		return nil, errorValue
	}
	return changedAtByField, nil
}

func persistCalendarEventFieldClockValues(ctx context.Context, transaction *sql.Tx, eventUID string, values map[string]string) error {
	if strings.TrimSpace(eventUID) == "" || len(values) == 0 {
		return nil
	}
	statement, errorValue := transaction.PrepareContext(ctx, `
INSERT INTO calendar_event_field_clocks(event_uid, field, changed_at)
VALUES(?, ?, ?)
ON CONFLICT(event_uid, field) DO UPDATE SET changed_at = excluded.changed_at`)
	if errorValue != nil {
		return errorValue
	}
	defer statement.Close()
	for field, changedAt := range values {
		if _, errorValue := statement.ExecContext(ctx, strings.TrimSpace(eventUID), field, changedAt); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func deleteCalendarEventDeletionClock(ctx context.Context, transaction *sql.Tx, eventUID string) error {
	_, errorValue := transaction.ExecContext(ctx, `DELETE FROM calendar_event_field_clocks WHERE event_uid = ? AND field = ?`, strings.TrimSpace(eventUID), calendarEventDeletionClockField)
	return errorValue
}

func readCalendarEventFieldClocks(ctx context.Context, queryRunner calendarSQLRunner) (map[string]map[string]time.Time, error) {
	rows, errorValue := queryRunner.QueryContext(ctx, `
SELECT event_uid, field, changed_at
FROM calendar_event_field_clocks
ORDER BY event_uid, field`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	result := map[string]map[string]time.Time{}
	for rows.Next() {
		var eventUID string
		var field string
		var changedAt string
		if errorValue := rows.Scan(&eventUID, &field, &changedAt); errorValue != nil {
			return nil, errorValue
		}
		if result[eventUID] == nil {
			result[eventUID] = map[string]time.Time{}
		}
		result[eventUID][field] = parseCalendarConflictTime(changedAt)
	}
	return result, rows.Err()
}

func readCalendarEventFieldClocksForUID(ctx context.Context, queryRunner calendarSQLRunner, eventUID string) (map[string]time.Time, error) {
	rows, errorValue := queryRunner.QueryContext(ctx, `
SELECT field, changed_at
FROM calendar_event_field_clocks
WHERE event_uid = ?`, strings.TrimSpace(eventUID))
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	result := map[string]time.Time{}
	for rows.Next() {
		var field string
		var changedAt string
		if errorValue := rows.Scan(&field, &changedAt); errorValue != nil {
			return nil, errorValue
		}
		result[field] = parseCalendarConflictTime(changedAt)
	}
	return result, rows.Err()
}
