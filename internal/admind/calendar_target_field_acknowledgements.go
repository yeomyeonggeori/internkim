package admind

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

const calendarTargetFieldAcknowledgementMigrationKey = "calendar_target_field_acknowledgements_v1"

func ensureCalendarTargetFieldAcknowledgementsTable(ctx context.Context, database *sql.DB) error {
	if _, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS calendar_target_field_acknowledgements (
	account_id TEXT NOT NULL,
	calendar_url TEXT NOT NULL,
	event_uid TEXT NOT NULL,
	field TEXT NOT NULL,
	acknowledged_at TEXT NOT NULL,
	PRIMARY KEY(account_id, calendar_url, event_uid, field)
)`); errorValue != nil {
		return errorValue
	}
	return migrateCalendarTargetFieldAcknowledgements(ctx, database)
}

func migrateCalendarTargetFieldAcknowledgements(ctx context.Context, database *sql.DB) error {
	var migrationValue string
	errorValue := database.QueryRowContext(ctx, "SELECT value FROM calendar_settings WHERE key = ?", calendarTargetFieldAcknowledgementMigrationKey).Scan(&migrationValue)
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
	if errorValue := seedCalendarTargetFieldAcknowledgements(ctx, transaction); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := seedCurrentCalendarTargetFieldAcknowledgementFallbacks(ctx, transaction); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if _, errorValue := transaction.ExecContext(ctx, "INSERT OR IGNORE INTO calendar_settings(key, value) VALUES(?, ?)", calendarTargetFieldAcknowledgementMigrationKey, "complete"); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

func seedCalendarTargetFieldAcknowledgements(ctx context.Context, transaction *sql.Tx) error {
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT state.account_id, state.calendar_url, state.event_uid, state.remote_modified_at, state.last_seen_at
FROM calendar_remote_event_sync_state state
WHERE EXISTS (
	SELECT 1 FROM calendar_events event WHERE event.uid = state.event_uid
)`)
	if errorValue != nil {
		return errorValue
	}
	type targetEvent struct {
		accountID        string
		calendarURL      string
		eventUID         string
		remoteModifiedAt string
		lastSeenAt       string
	}
	events := []targetEvent{}
	for rows.Next() {
		var event targetEvent
		if errorValue := rows.Scan(&event.accountID, &event.calendarURL, &event.eventUID, &event.remoteModifiedAt, &event.lastSeenAt); errorValue != nil {
			rows.Close()
			return errorValue
		}
		events = append(events, event)
	}
	if errorValue := rows.Close(); errorValue != nil {
		return errorValue
	}
	for _, event := range events {
		baseline := parseCalendarConflictTime(event.remoteModifiedAt)
		if baseline.IsZero() {
			baseline = parseCalendarConflictTime(event.lastSeenAt)
		}
		if baseline.IsZero() {
			continue
		}
		pendingFields, errorValue := readPendingCalendarTargetFields(ctx, transaction, event.accountID, event.calendarURL, event.eventUID)
		if errorValue != nil {
			return errorValue
		}
		acknowledgements := map[string]time.Time{}
		for _, field := range calendarAllUserEditableFields() {
			if !calendarFieldListIncludes(pendingFields, field) {
				acknowledgements[field] = baseline
			}
		}
		if errorValue := persistCalendarTargetFieldAcknowledgements(ctx, transaction, event.accountID, event.calendarURL, event.eventUID, acknowledgements); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func readPendingCalendarTargetFields(ctx context.Context, queryRunner calendarSQLRunner, accountID string, calendarURL string, eventUID string) ([]string, error) {
	rows, errorValue := queryRunner.QueryContext(ctx, `
SELECT changed_fields
FROM calendar_outbox
WHERE account_id = ? AND target_calendar_url = ? AND event_uid = ? AND operation = ? AND status IN (?, ?)
ORDER BY id`, strings.TrimSpace(accountID), normalizeCalendarOutboxTargetURL(calendarURL), strings.TrimSpace(eventUID), calendarOutboxOperationPut, calendarOutboxStatusPending, calendarOutboxStatusBlocked)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	fields := []string{}
	for rows.Next() {
		var encodedFields string
		if errorValue := rows.Scan(&encodedFields); errorValue != nil {
			return nil, errorValue
		}
		fields = mergeCalendarFieldLists(fields, normalizeCalendarOutboxPutChangedFields(decodeChangedFields(encodedFields)))
	}
	return fields, rows.Err()
}

func persistCalendarTargetFieldAcknowledgements(ctx context.Context, transaction *sql.Tx, accountID string, calendarURL string, eventUID string, acknowledgements map[string]time.Time) error {
	if len(acknowledgements) == 0 {
		return nil
	}
	existing, errorValue := readCalendarTargetFieldAcknowledgements(ctx, transaction, accountID, calendarURL, eventUID)
	if errorValue != nil {
		return errorValue
	}
	statement, errorValue := transaction.PrepareContext(ctx, `
INSERT INTO calendar_target_field_acknowledgements(account_id, calendar_url, event_uid, field, acknowledged_at)
VALUES(?, ?, ?, ?, ?)
ON CONFLICT(account_id, calendar_url, event_uid, field) DO UPDATE SET acknowledged_at = excluded.acknowledged_at`)
	if errorValue != nil {
		return errorValue
	}
	defer statement.Close()
	for field, acknowledgedAt := range acknowledgements {
		if acknowledgedAt.IsZero() || !acknowledgedAt.After(existing[field]) {
			continue
		}
		if _, errorValue := statement.ExecContext(ctx, strings.TrimSpace(accountID), canonicalCalendarTargetURL(calendarURL), strings.TrimSpace(eventUID), field, acknowledgedAt.UTC().Format(time.RFC3339Nano)); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func readCalendarTargetFieldAcknowledgements(ctx context.Context, queryRunner calendarSQLRunner, accountID string, calendarURL string, eventUID string) (map[string]time.Time, error) {
	rows, errorValue := queryRunner.QueryContext(ctx, `
SELECT field, acknowledged_at
FROM calendar_target_field_acknowledgements
WHERE account_id = ? AND calendar_url = ? AND event_uid = ?`, strings.TrimSpace(accountID), canonicalCalendarTargetURL(calendarURL), strings.TrimSpace(eventUID))
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	result := map[string]time.Time{}
	for rows.Next() {
		var field string
		var acknowledgedAt string
		if errorValue := rows.Scan(&field, &acknowledgedAt); errorValue != nil {
			return nil, errorValue
		}
		result[field] = parseCalendarConflictTime(acknowledgedAt)
	}
	return result, rows.Err()
}

func calendarFieldAcknowledgementsAt(fields []string, acknowledgedAt time.Time) map[string]time.Time {
	result := map[string]time.Time{}
	if acknowledgedAt.IsZero() {
		return result
	}
	for _, field := range fields {
		result[field] = acknowledgedAt
	}
	return result
}
