package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	calendarSourceLocal = "local"
	calendarSourcePull  = "pull"
)

type calendarOutboxRow struct {
	ID              int64
	AccountID       string
	EventID         string
	EventUID        string
	Operation       string
	PayloadICS      string
	IfMatchETag     string
	RemoteHref      string
	ChangedFields   []string
	AttemptCount    int
	LastError       string
	CreatedAt       string
	LastAttemptedAt string
}

type pendingCalendarLocalChange struct {
	EventUID      string
	ChangedFields []string
}

func encodeChangedFields(fields []string) string {
	if len(fields) == 0 {
		return ""
	}
	encoded, errorValue := json.Marshal(fields)
	if errorValue != nil {
		return ""
	}
	return string(encoded)
}

func decodeChangedFields(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	var fields []string
	if errorValue := json.Unmarshal([]byte(trimmed), &fields); errorValue != nil {
		return nil
	}
	return fields
}

func (service *Service) enqueueCalendarOutbox(ctx context.Context, row calendarOutboxRow) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	return enqueueCalendarOutboxWithRunner(ctx, database, row)
}

func enqueueCalendarOutboxWithRunner(ctx context.Context, queryRunner calendarSQLRunner, row calendarOutboxRow) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, errorValue := queryRunner.ExecContext(ctx, `
INSERT INTO calendar_outbox (account_id, event_id, event_uid, operation, payload_ics, if_match_etag, remote_href, changed_fields, attempt_count, last_error, created_at, last_attempted_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, '', ?, '')`,
		strings.TrimSpace(row.AccountID),
		strings.TrimSpace(row.EventID),
		strings.TrimSpace(row.EventUID),
		strings.TrimSpace(row.Operation),
		row.PayloadICS,
		strings.TrimSpace(row.IfMatchETag),
		strings.TrimSpace(row.RemoteHref),
		encodeChangedFields(row.ChangedFields),
		now,
	)
	return errorValue
}

func (service *Service) listPendingCalendarOutbox(ctx context.Context, accountID string) ([]calendarOutboxRow, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	query := `SELECT id, account_id, event_id, event_uid, operation, payload_ics, if_match_etag, remote_href, changed_fields, attempt_count, last_error, created_at, last_attempted_at FROM calendar_outbox`
	arguments := []any{}
	if strings.TrimSpace(accountID) != "" {
		query += " WHERE account_id = ?"
		arguments = append(arguments, strings.TrimSpace(accountID))
	}
	query += " ORDER BY id"
	rows, errorValue := database.QueryContext(ctx, query, arguments...)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	result := []calendarOutboxRow{}
	for rows.Next() {
		var row calendarOutboxRow
		var changedFieldsRaw string
		if errorValue := rows.Scan(
			&row.ID,
			&row.AccountID,
			&row.EventID,
			&row.EventUID,
			&row.Operation,
			&row.PayloadICS,
			&row.IfMatchETag,
			&row.RemoteHref,
			&changedFieldsRaw,
			&row.AttemptCount,
			&row.LastError,
			&row.CreatedAt,
			&row.LastAttemptedAt,
		); errorValue != nil {
			return nil, errorValue
		}
		row.ChangedFields = decodeChangedFields(changedFieldsRaw)
		if row.Operation == calendarOutboxOperationPut {
			row.ChangedFields = normalizeCalendarOutboxPutChangedFields(row.ChangedFields)
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func normalizeCalendarOutboxPutChangedFields(fields []string) []string {
	if len(fields) == 0 {
		return calendarAllUserEditableFields()
	}
	return fields
}

func (service *Service) listPendingCalendarLocalChanges(ctx context.Context, accountID string) (map[string]pendingCalendarLocalChange, error) {
	rows, errorValue := service.listPendingCalendarOutbox(ctx, accountID)
	if errorValue != nil {
		return nil, errorValue
	}
	result := map[string]pendingCalendarLocalChange{}
	for _, row := range rows {
		eventUID := strings.TrimSpace(row.EventUID)
		if row.Operation != calendarOutboxOperationPut || eventUID == "" {
			continue
		}
		change := result[eventUID]
		change.EventUID = eventUID
		change.ChangedFields = mergeCalendarFieldLists(change.ChangedFields, row.ChangedFields)
		result[eventUID] = change
	}
	return result, nil
}

func (service *Service) readPendingCalendarLocalChange(ctx context.Context, accountID string, eventUID string) (pendingCalendarLocalChange, bool, error) {
	trimmedEventUID := strings.TrimSpace(eventUID)
	if trimmedEventUID == "" {
		return pendingCalendarLocalChange{}, false, nil
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return pendingCalendarLocalChange{}, false, errorValue
	}
	defer database.Close()
	query := `SELECT changed_fields FROM calendar_outbox WHERE event_uid = ? AND operation = ?`
	arguments := []any{trimmedEventUID, calendarOutboxOperationPut}
	if strings.TrimSpace(accountID) != "" {
		query += " AND account_id = ?"
		arguments = append(arguments, strings.TrimSpace(accountID))
	}
	query += " ORDER BY id"
	rows, errorValue := database.QueryContext(ctx, query, arguments...)
	if errorValue != nil {
		return pendingCalendarLocalChange{}, false, errorValue
	}
	defer rows.Close()
	change := pendingCalendarLocalChange{EventUID: trimmedEventUID}
	found := false
	for rows.Next() {
		var changedFieldsRaw string
		if errorValue := rows.Scan(&changedFieldsRaw); errorValue != nil {
			return pendingCalendarLocalChange{}, false, errorValue
		}
		change.ChangedFields = mergeCalendarFieldLists(change.ChangedFields, normalizeCalendarOutboxPutChangedFields(decodeChangedFields(changedFieldsRaw)))
		found = true
	}
	return change, found, rows.Err()
}

func (service *Service) hasPendingCalendarLocalDelete(ctx context.Context, accountID string, eventUID string) (bool, error) {
	trimmedEventUID := strings.TrimSpace(eventUID)
	if trimmedEventUID == "" {
		return false, nil
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return false, errorValue
	}
	defer database.Close()
	query := `SELECT 1 FROM calendar_outbox WHERE event_uid = ? AND operation = ?`
	arguments := []any{trimmedEventUID, calendarOutboxOperationDelete}
	if strings.TrimSpace(accountID) != "" {
		query += " AND account_id = ?"
		arguments = append(arguments, strings.TrimSpace(accountID))
	}
	query += " LIMIT 1"
	var marker int
	errorValue = database.QueryRowContext(ctx, query, arguments...).Scan(&marker)
	if errorValue == nil {
		return true, nil
	}
	if errors.Is(errorValue, sql.ErrNoRows) {
		return false, nil
	}
	return false, errorValue
}

func (service *Service) hasPendingCalendarPutForTarget(ctx context.Context, accountID string, eventUID string, target remoteCalendarTarget) (bool, error) {
	trimmedEventUID := strings.TrimSpace(eventUID)
	if trimmedEventUID == "" {
		return false, nil
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return false, errorValue
	}
	defer database.Close()
	return hasPendingCalendarPutForTargetWithRunner(ctx, database, accountID, trimmedEventUID, target)
}

func hasPendingCalendarPutForTargetWithRunner(ctx context.Context, queryRunner calendarSQLRunner, accountID string, eventUID string, target remoteCalendarTarget) (bool, error) {
	trimmedEventUID := strings.TrimSpace(eventUID)
	if trimmedEventUID == "" {
		return false, nil
	}
	query := `SELECT remote_href FROM calendar_outbox WHERE event_uid = ? AND operation = ?`
	arguments := []any{trimmedEventUID, calendarOutboxOperationPut}
	if strings.TrimSpace(accountID) != "" {
		query += " AND account_id = ?"
		arguments = append(arguments, strings.TrimSpace(accountID))
	}
	query += " ORDER BY id"
	rows, errorValue := queryRunner.QueryContext(ctx, query, arguments...)
	if errorValue != nil {
		return false, errorValue
	}
	defer rows.Close()
	for rows.Next() {
		var remoteHref string
		if errorValue := rows.Scan(&remoteHref); errorValue != nil {
			return false, errorValue
		}
		if calendarOutboxPutTargetsRemoteTarget(calendarOutboxRow{
			Operation:  calendarOutboxOperationPut,
			RemoteHref: remoteHref,
		}, target) {
			return true, nil
		}
	}
	return false, rows.Err()
}

func mergeCalendarFieldLists(left []string, right []string) []string {
	if len(left) == 0 {
		return append([]string(nil), right...)
	}
	result := append([]string(nil), left...)
	seen := map[string]struct{}{}
	for _, field := range result {
		seen[field] = struct{}{}
	}
	for _, field := range right {
		if _, found := seen[field]; found {
			continue
		}
		result = append(result, field)
		seen[field] = struct{}{}
	}
	return result
}

func (service *Service) updatePendingCalendarOutboxRemoteState(ctx context.Context, accountID string, eventUID string, remoteHref string, remoteETag string) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx,
		`UPDATE calendar_outbox SET if_match_etag = ?, remote_href = ? WHERE account_id = ? AND event_uid = ? AND operation = ?`,
		strings.TrimSpace(remoteETag),
		strings.TrimSpace(remoteHref),
		strings.TrimSpace(accountID),
		strings.TrimSpace(eventUID),
		calendarOutboxOperationPut,
	)
	return errorValue
}

func (service *Service) markCalendarOutboxAttempt(ctx context.Context, rowID int64, lastError string) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, errorValue = database.ExecContext(ctx,
		`UPDATE calendar_outbox SET attempt_count = attempt_count + 1, last_error = ?, last_attempted_at = ? WHERE id = ?`,
		lastError, now, rowID)
	return errorValue
}

func (service *Service) deleteCalendarOutbox(ctx context.Context, rowID int64) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `DELETE FROM calendar_outbox WHERE id = ?`, rowID)
	return errorValue
}
