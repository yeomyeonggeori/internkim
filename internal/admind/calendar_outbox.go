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
	calendarSourceLocal                 = "local"
	calendarSourcePull                  = "pull"
	calendarOutboxActiveStatusPredicate = "status IN (?, ?)"
)

type calendarOutboxRow struct {
	ID                int64
	AccountID         string
	EventID           string
	EventUID          string
	Operation         string
	PayloadICS        string
	IfMatchETag       string
	RemoteHref        string
	TargetCalendarURL string
	ChangedFields     []string
	AttemptCount      int
	LastError         string
	CreatedAt         string
	LastAttemptedAt   string
	Status            string
	FailedAt          string
	SourceRowIDs      []int64
	FieldChangedAt    map[string]time.Time
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
	return enqueueCalendarOutboxWithRunner(ctx, database, row, time.Now().UTC().Format(time.RFC3339Nano))
}

func enqueueCalendarOutboxWithRunner(ctx context.Context, queryRunner calendarSQLRunner, row calendarOutboxRow, createdAt string) error {
	row, errorValue := prepareCalendarOutboxTargetWithRunner(ctx, queryRunner, row)
	if errorValue != nil {
		return errorValue
	}
	_, errorValue = queryRunner.ExecContext(ctx, `
INSERT INTO calendar_outbox (account_id, event_id, event_uid, operation, payload_ics, if_match_etag, remote_href, target_calendar_url, changed_fields, attempt_count, last_error, created_at, last_attempted_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, '', ?, '')`,
		strings.TrimSpace(row.AccountID),
		strings.TrimSpace(row.EventID),
		strings.TrimSpace(row.EventUID),
		strings.TrimSpace(row.Operation),
		row.PayloadICS,
		strings.TrimSpace(row.IfMatchETag),
		strings.TrimSpace(row.RemoteHref),
		normalizeCalendarOutboxTargetURL(row.TargetCalendarURL),
		encodeChangedFields(row.ChangedFields),
		strings.TrimSpace(createdAt),
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
	query := `SELECT id, account_id, event_id, event_uid, operation, payload_ics, if_match_etag, remote_href, target_calendar_url, changed_fields, attempt_count, last_error, created_at, last_attempted_at, status, failed_at FROM calendar_outbox`
	arguments := []any{}
	conditions := []string{}
	if strings.TrimSpace(accountID) != "" {
		conditions = append(conditions, "account_id = ?")
		arguments = append(arguments, strings.TrimSpace(accountID))
	}
	if includeBlocked {
		conditions = append(conditions, calendarOutboxActiveStatusPredicate)
		arguments = append(arguments, calendarOutboxStatusPending, calendarOutboxStatusBlocked)
	} else {
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
			&row.TargetCalendarURL,
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

func (service *Service) hasPendingCalendarLocalDelete(ctx context.Context, accountID string, targetCalendarURL string, eventUID string) (bool, error) {
	trimmedEventUID := strings.TrimSpace(eventUID)
	if trimmedEventUID == "" {
		return false, nil
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return false, errorValue
	}
	defer database.Close()
	query := `SELECT operation FROM calendar_outbox WHERE account_id = ? AND target_calendar_url = ? AND event_uid = ? AND ` + calendarOutboxActiveStatusPredicate + ` ORDER BY id DESC LIMIT 1`
	var operation string
	errorValue = database.QueryRowContext(ctx, query,
		strings.TrimSpace(accountID),
		normalizeCalendarOutboxTargetURL(targetCalendarURL),
		trimmedEventUID,
		calendarOutboxStatusPending,
		calendarOutboxStatusBlocked,
	).Scan(&operation)
	if errorValue == nil {
		return operation == calendarOutboxOperationDelete, nil
	}
	if errors.Is(errorValue, sql.ErrNoRows) {
		return false, nil
	}
	return false, errorValue
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
	query := `SELECT target_calendar_url FROM calendar_outbox WHERE operation = ? AND target_calendar_url = ? AND ` + calendarOutboxActiveStatusPredicate
	arguments := []any{calendarOutboxOperationPut, normalizeCalendarOutboxTargetURL(target.CalendarURL), calendarOutboxStatusPending, calendarOutboxStatusBlocked}
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
		var targetCalendarURL string
		if errorValue := rows.Scan(&targetCalendarURL); errorValue != nil {
			return false, errorValue
		}
		if normalizeCalendarOutboxTargetURL(targetCalendarURL) == normalizeCalendarOutboxTargetURL(target.CalendarURL) {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (service *Service) updatePendingCalendarOutboxRemoteState(ctx context.Context, accountID string, targetCalendarURL string, eventUID string, remoteHref string, remoteETag string) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx,
		`UPDATE calendar_outbox SET if_match_etag = ?, remote_href = ? WHERE account_id = ? AND target_calendar_url = ? AND event_uid = ? AND operation = ? AND `+calendarOutboxActiveStatusPredicate,
		strings.TrimSpace(remoteETag),
		strings.TrimSpace(remoteHref),
		strings.TrimSpace(accountID),
		normalizeCalendarOutboxTargetURL(targetCalendarURL),
		strings.TrimSpace(eventUID),
		calendarOutboxOperationPut,
		calendarOutboxStatusPending,
		calendarOutboxStatusBlocked,
	)
	return errorValue
}

func (service *Service) updatePendingCalendarDeleteRemoteState(ctx context.Context, accountID string, targetCalendarURL string, eventUID string, remoteHref string, remoteETag string) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx,
		`UPDATE calendar_outbox SET if_match_etag = ?, remote_href = ? WHERE account_id = ? AND target_calendar_url = ? AND event_uid = ? AND operation = ? AND `+calendarOutboxActiveStatusPredicate,
		strings.TrimSpace(remoteETag),
		strings.TrimSpace(remoteHref),
		strings.TrimSpace(accountID),
		normalizeCalendarOutboxTargetURL(targetCalendarURL),
		strings.TrimSpace(eventUID),
		calendarOutboxOperationDelete,
		calendarOutboxStatusPending,
		calendarOutboxStatusBlocked,
	)
	return errorValue
}

func deleteCalendarOutboxForEventUIDWithRunner(ctx context.Context, queryRunner calendarSQLRunner, accountID string, targetCalendarURL string, eventUID string) error {
	_, errorValue := queryRunner.ExecContext(ctx, `DELETE FROM calendar_outbox WHERE account_id = ? AND target_calendar_url = ? AND event_uid = ? AND `+calendarOutboxActiveStatusPredicate, strings.TrimSpace(accountID), normalizeCalendarOutboxTargetURL(targetCalendarURL), strings.TrimSpace(eventUID), calendarOutboxStatusPending, calendarOutboxStatusBlocked)
	return errorValue
}
