package admind

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type calendarPushObservationFenceIdentityRow struct {
	AccountID   string
	CalendarURL string
	EventUID    string
	CreatedAt   string
}

type calendarRemoteEventStateIdentityRow struct {
	AccountID         string
	CalendarURL       string
	EventUID          string
	RemoteModifiedAt  string
	LastSeenAt        string
	MissingDetectedAt string
	UpdatedAt         string
}

func migrateCalendarTargetIdentityKeys(ctx context.Context, database *sql.DB) error {
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := migrateCalendarPushObservationFenceIdentityKeys(ctx, transaction); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := migrateCalendarRemoteEventStateIdentityKeys(ctx, transaction); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

func migrateCalendarPushObservationFenceIdentityKeys(ctx context.Context, transaction *sql.Tx) error {
	rows, errorValue := transaction.QueryContext(ctx, `SELECT account_id, calendar_url, event_uid, created_at FROM calendar_push_observation_fences`)
	if errorValue != nil {
		return errorValue
	}
	legacyRows := []calendarPushObservationFenceIdentityRow{}
	for rows.Next() {
		var row calendarPushObservationFenceIdentityRow
		if errorValue := rows.Scan(&row.AccountID, &row.CalendarURL, &row.EventUID, &row.CreatedAt); errorValue != nil {
			rows.Close()
			return errorValue
		}
		if canonicalCalendarTargetURL(row.CalendarURL) != row.CalendarURL {
			legacyRows = append(legacyRows, row)
		}
	}
	if errorValue := rows.Err(); errorValue != nil {
		rows.Close()
		return errorValue
	}
	rows.Close()
	for _, row := range legacyRows {
		if errorValue := migrateCalendarPushObservationFenceIdentityRow(ctx, transaction, row); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func migrateCalendarPushObservationFenceIdentityRow(ctx context.Context, transaction *sql.Tx, row calendarPushObservationFenceIdentityRow) error {
	canonicalURL := canonicalCalendarTargetURL(row.CalendarURL)
	if canonicalURL == "" {
		return nil
	}
	var canonicalCreatedAt string
	errorValue := transaction.QueryRowContext(ctx, `
SELECT created_at
FROM calendar_push_observation_fences
WHERE account_id = ? AND calendar_url = ? AND event_uid = ?`, row.AccountID, canonicalURL, row.EventUID).Scan(&canonicalCreatedAt)
	if errorValue != nil && !errors.Is(errorValue, sql.ErrNoRows) {
		return errorValue
	}
	mergedCreatedAt := latestCalendarIdentityValue(row.CreatedAt, canonicalCreatedAt)
	if errors.Is(errorValue, sql.ErrNoRows) {
		_, errorValue = transaction.ExecContext(ctx, `
INSERT INTO calendar_push_observation_fences (account_id, calendar_url, event_uid, created_at)
VALUES (?, ?, ?, ?)`, row.AccountID, canonicalURL, row.EventUID, mergedCreatedAt)
	} else if mergedCreatedAt != canonicalCreatedAt {
		_, errorValue = transaction.ExecContext(ctx, `
UPDATE calendar_push_observation_fences
SET created_at = ?
WHERE account_id = ? AND calendar_url = ? AND event_uid = ?`, mergedCreatedAt, row.AccountID, canonicalURL, row.EventUID)
	}
	if errorValue != nil {
		return errorValue
	}
	_, errorValue = transaction.ExecContext(ctx, `
DELETE FROM calendar_push_observation_fences
WHERE account_id = ? AND calendar_url = ? AND event_uid = ?`, row.AccountID, row.CalendarURL, row.EventUID)
	return errorValue
}

func migrateCalendarRemoteEventStateIdentityKeys(ctx context.Context, transaction *sql.Tx) error {
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT account_id, calendar_url, event_uid, remote_modified_at, last_seen_at, missing_detected_at, updated_at
FROM calendar_remote_event_sync_state`)
	if errorValue != nil {
		return errorValue
	}
	legacyRows := []calendarRemoteEventStateIdentityRow{}
	for rows.Next() {
		var row calendarRemoteEventStateIdentityRow
		if errorValue := rows.Scan(&row.AccountID, &row.CalendarURL, &row.EventUID, &row.RemoteModifiedAt, &row.LastSeenAt, &row.MissingDetectedAt, &row.UpdatedAt); errorValue != nil {
			rows.Close()
			return errorValue
		}
		if canonicalCalendarTargetURL(row.CalendarURL) != row.CalendarURL {
			legacyRows = append(legacyRows, row)
		}
	}
	if errorValue := rows.Err(); errorValue != nil {
		rows.Close()
		return errorValue
	}
	rows.Close()
	for _, row := range legacyRows {
		if errorValue := migrateCalendarRemoteEventStateIdentityRow(ctx, transaction, row); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func migrateCalendarRemoteEventStateIdentityRow(ctx context.Context, transaction *sql.Tx, row calendarRemoteEventStateIdentityRow) error {
	canonicalURL := canonicalCalendarTargetURL(row.CalendarURL)
	if canonicalURL == "" {
		return nil
	}
	canonicalRow := calendarRemoteEventStateIdentityRow{}
	errorValue := transaction.QueryRowContext(ctx, `
SELECT account_id, calendar_url, event_uid, remote_modified_at, last_seen_at, missing_detected_at, updated_at
FROM calendar_remote_event_sync_state
WHERE account_id = ? AND calendar_url = ? AND event_uid = ?`, row.AccountID, canonicalURL, row.EventUID).Scan(
		&canonicalRow.AccountID,
		&canonicalRow.CalendarURL,
		&canonicalRow.EventUID,
		&canonicalRow.RemoteModifiedAt,
		&canonicalRow.LastSeenAt,
		&canonicalRow.MissingDetectedAt,
		&canonicalRow.UpdatedAt,
	)
	if errorValue != nil && !errors.Is(errorValue, sql.ErrNoRows) {
		return errorValue
	}
	mergedRow := row
	mergedRow.CalendarURL = canonicalURL
	if errorValue == nil {
		mergedRow = mergeCalendarRemoteEventStateIdentityRows(row, canonicalRow, canonicalURL)
	}
	if errors.Is(errorValue, sql.ErrNoRows) {
		_, errorValue = transaction.ExecContext(ctx, `
INSERT INTO calendar_remote_event_sync_state (
	account_id, calendar_url, event_uid, remote_modified_at, last_seen_at, missing_detected_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			mergedRow.AccountID,
			mergedRow.CalendarURL,
			mergedRow.EventUID,
			mergedRow.RemoteModifiedAt,
			mergedRow.LastSeenAt,
			mergedRow.MissingDetectedAt,
			mergedRow.UpdatedAt,
		)
	} else if mergedRow != canonicalRow {
		_, errorValue = transaction.ExecContext(ctx, `
UPDATE calendar_remote_event_sync_state
SET remote_modified_at = ?, last_seen_at = ?, missing_detected_at = ?, updated_at = ?
WHERE account_id = ? AND calendar_url = ? AND event_uid = ?`,
			mergedRow.RemoteModifiedAt,
			mergedRow.LastSeenAt,
			mergedRow.MissingDetectedAt,
			mergedRow.UpdatedAt,
			mergedRow.AccountID,
			mergedRow.CalendarURL,
			mergedRow.EventUID,
		)
	}
	if errorValue != nil {
		return fmt.Errorf("merge calendar remote event state identity: %w", errorValue)
	}
	_, errorValue = transaction.ExecContext(ctx, `
DELETE FROM calendar_remote_event_sync_state
WHERE account_id = ? AND calendar_url = ? AND event_uid = ?`, row.AccountID, row.CalendarURL, row.EventUID)
	return errorValue
}

func mergeCalendarRemoteEventStateIdentityRows(legacyRow calendarRemoteEventStateIdentityRow, canonicalRow calendarRemoteEventStateIdentityRow, canonicalURL string) calendarRemoteEventStateIdentityRow {
	missingDetectedAt := canonicalRow.MissingDetectedAt
	if calendarIdentityFirstIsNewer(legacyRow.UpdatedAt, canonicalRow.UpdatedAt) {
		missingDetectedAt = legacyRow.MissingDetectedAt
	}
	return calendarRemoteEventStateIdentityRow{
		AccountID:         legacyRow.AccountID,
		CalendarURL:       canonicalURL,
		EventUID:          legacyRow.EventUID,
		RemoteModifiedAt:  latestCalendarIdentityValue(legacyRow.RemoteModifiedAt, canonicalRow.RemoteModifiedAt),
		LastSeenAt:        latestCalendarIdentityValue(legacyRow.LastSeenAt, canonicalRow.LastSeenAt),
		MissingDetectedAt: missingDetectedAt,
		UpdatedAt:         latestCalendarIdentityValue(legacyRow.UpdatedAt, canonicalRow.UpdatedAt),
	}
}

func latestCalendarIdentityValue(first string, second string) string {
	trimmedFirst := strings.TrimSpace(first)
	trimmedSecond := strings.TrimSpace(second)
	if calendarIdentityFirstIsNewer(trimmedFirst, trimmedSecond) {
		return trimmedFirst
	}
	return trimmedSecond
}

func calendarIdentityFirstIsNewer(first string, second string) bool {
	trimmedFirst := strings.TrimSpace(first)
	trimmedSecond := strings.TrimSpace(second)
	firstTime, firstError := time.Parse(time.RFC3339Nano, trimmedFirst)
	secondTime, secondError := time.Parse(time.RFC3339Nano, trimmedSecond)
	if firstError == nil && secondError == nil {
		return firstTime.After(secondTime)
	}
	if firstError == nil {
		return true
	}
	if secondError == nil {
		return false
	}
	return trimmedFirst > trimmedSecond
}
