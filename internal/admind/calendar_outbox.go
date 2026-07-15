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
	Status          string
	FailedAt        string
}

type pendingCalendarLocalChange struct {
	EventUID       string
	ChangedFields  []string
	FieldChangedAt map[string]time.Time
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
	return service.listCalendarOutbox(ctx, accountID, false)
}

func (service *Service) listCalendarOutbox(ctx context.Context, accountID string, includeBlocked bool) ([]calendarOutboxRow, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	query := `SELECT id, account_id, event_id, event_uid, operation, payload_ics, if_match_etag, remote_href, changed_fields, attempt_count, last_error, created_at, last_attempted_at, status, failed_at FROM calendar_outbox`
	arguments := []any{}
	conditions := []string{}
	if strings.TrimSpace(accountID) != "" {
		conditions = append(conditions, "account_id = ?")
		arguments = append(arguments, strings.TrimSpace(accountID))
	}
	if !includeBlocked {
		conditions = append(conditions, "status = ?")
		arguments = append(arguments, calendarOutboxStatusPending)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
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
			&row.Status,
			&row.FailedAt,
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

func (service *Service) markCalendarOutboxBlocked(ctx context.Context, rowID int64, failedAt string, failure string) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `
UPDATE calendar_outbox
SET status = ?, failed_at = ?, last_error = ?
WHERE id = ?`, calendarOutboxStatusBlocked, strings.TrimSpace(failedAt), strings.TrimSpace(failure), rowID)
	return errorValue
}

func normalizeCalendarOutboxPutChangedFields(fields []string) []string {
	if len(fields) == 0 {
		return calendarAllUserEditableFields()
	}
	return fields
}

func (service *Service) listPendingCalendarLocalChanges(ctx context.Context, accountID string) (map[string]pendingCalendarLocalChange, error) {
	rows, errorValue := service.listCalendarOutbox(ctx, accountID, true)
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
		if change.FieldChangedAt == nil {
			change.FieldChangedAt = map[string]time.Time{}
		}
		change.ChangedFields = mergeCalendarFieldLists(change.ChangedFields, row.ChangedFields)
		changedAt := parseCalendarConflictTime(row.CreatedAt)
		for _, field := range row.ChangedFields {
			if changedAt.After(change.FieldChangedAt[field]) {
				change.FieldChangedAt[field] = changedAt
			}
		}
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
	query := `SELECT changed_fields, created_at FROM calendar_outbox WHERE event_uid = ? AND operation = ?`
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
	change := pendingCalendarLocalChange{EventUID: trimmedEventUID, FieldChangedAt: map[string]time.Time{}}
	found := false
	for rows.Next() {
		var changedFieldsRaw string
		var createdAtRaw string
		if errorValue := rows.Scan(&changedFieldsRaw, &createdAtRaw); errorValue != nil {
			return pendingCalendarLocalChange{}, false, errorValue
		}
		changedFields := normalizeCalendarOutboxPutChangedFields(decodeChangedFields(changedFieldsRaw))
		change.ChangedFields = mergeCalendarFieldLists(change.ChangedFields, changedFields)
		changedAt := parseCalendarConflictTime(createdAtRaw)
		for _, field := range changedFields {
			if changedAt.After(change.FieldChangedAt[field]) {
				change.FieldChangedAt[field] = changedAt
			}
		}
		found = true
	}
	return change, found, rows.Err()
}

func (service *Service) retainPendingCalendarOutboxFields(ctx context.Context, accountID string, eventUID string, retainedFields []string, remoteHref string, remoteETag string) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, changed_fields
FROM calendar_outbox
WHERE account_id = ? AND event_uid = ? AND operation = ?
ORDER BY id`, strings.TrimSpace(accountID), strings.TrimSpace(eventUID), calendarOutboxOperationPut)
	if errorValue != nil {
		return errorValue
	}
	type retainedCalendarOutboxRow struct {
		ID     int64
		Fields []string
	}
	retainedRows := []retainedCalendarOutboxRow{}
	for rows.Next() {
		var rowID int64
		var changedFieldsRaw string
		if errorValue := rows.Scan(&rowID, &changedFieldsRaw); errorValue != nil {
			rows.Close()
			return errorValue
		}
		changedFields := normalizeCalendarOutboxPutChangedFields(decodeChangedFields(changedFieldsRaw))
		retainedRows = append(retainedRows, retainedCalendarOutboxRow{ID: rowID, Fields: intersectCalendarFields(changedFields, retainedFields)})
	}
	if errorValue := rows.Close(); errorValue != nil {
		return errorValue
	}
	for _, row := range retainedRows {
		if len(row.Fields) == 0 {
			if _, errorValue := database.ExecContext(ctx, `DELETE FROM calendar_outbox WHERE id = ?`, row.ID); errorValue != nil {
				return errorValue
			}
			continue
		}
		if _, errorValue := database.ExecContext(ctx, `
UPDATE calendar_outbox
SET changed_fields = ?, if_match_etag = ?, remote_href = ?, status = ?, failed_at = '', attempt_count = 0, last_error = ''
WHERE id = ?`, encodeChangedFields(row.Fields), strings.TrimSpace(remoteETag), strings.TrimSpace(remoteHref), calendarOutboxStatusPending, row.ID); errorValue != nil {
			return errorValue
		}
	}
	return nil
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
	return hasPendingCalendarPutsForTargetWithRunner(ctx, queryRunner, accountID, trimmedEventUID, target)
}

func (service *Service) hasPendingCalendarPutsForTarget(ctx context.Context, accountID string, target remoteCalendarTarget) (bool, error) {
	if target.CalendarURL == "" {
		return false, nil
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return false, errorValue
	}
	defer database.Close()
	return hasPendingCalendarPutsForTargetWithRunner(ctx, database, accountID, "", target)
}

func hasPendingCalendarPutsForTargetWithRunner(ctx context.Context, queryRunner calendarSQLRunner, accountID string, eventUID string, target remoteCalendarTarget) (bool, error) {
	query := `SELECT remote_href FROM calendar_outbox WHERE operation = ?`
	arguments := []any{calendarOutboxOperationPut}
	if strings.TrimSpace(eventUID) != "" {
		query += " AND event_uid = ?"
		arguments = append(arguments, strings.TrimSpace(eventUID))
	}
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

func (service *Service) updatePendingCalendarDeleteRemoteState(ctx context.Context, accountID string, eventUID string, remoteHref string, remoteETag string) error {
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
		calendarOutboxOperationDelete,
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

func (service *Service) deleteCalendarOutboxForEventUID(ctx context.Context, accountID string, eventUID string) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `DELETE FROM calendar_outbox WHERE account_id = ? AND event_uid = ?`, strings.TrimSpace(accountID), strings.TrimSpace(eventUID))
	return errorValue
}
