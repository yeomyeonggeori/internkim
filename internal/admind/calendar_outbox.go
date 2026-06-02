package admind

import (
	"context"
	"encoding/json"
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
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, errorValue = database.ExecContext(ctx, `
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
		result = append(result, row)
	}
	return result, rows.Err()
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
