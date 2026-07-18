package admind

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	legacyCalendarOutboxDeleteTargetUnknown  = "legacy calendar outbox delete target is unknown"
	legacyCalendarOutboxPutTargetUnavailable = "legacy calendar outbox PUT target unavailable; reconnect account or select a complete writable calendar"
)

var errCalendarTargetUnavailable = errors.New("calendar target unavailable")

type legacyCalendarOutboxTargetRow struct {
	ID        int64
	AccountID string
	Operation string
}

func canonicalCalendarTargetURL(calendarURL string) string {
	normalized := strings.TrimRight(calDAVPathOnly(strings.TrimSpace(calendarURL)), "/")
	if normalized == "" {
		return ""
	}
	return normalized + "/"
}

func normalizeCalendarOutboxTargetURL(calendarURL string) string {
	return canonicalCalendarTargetURL(calendarURL)
}

func calendarRemoteHrefBelongsToTargetURL(remoteHref string, targetCalendarURL string) bool {
	normalizedTarget := normalizeCalendarOutboxTargetURL(targetCalendarURL)
	normalizedRemoteHref := calDAVPathOnly(strings.TrimSpace(remoteHref))
	return normalizedTarget != "" && normalizedRemoteHref != "" && strings.HasPrefix(normalizedRemoteHref, normalizedTarget)
}

func prepareCalendarOutboxTargetWithRunner(ctx context.Context, queryRunner calendarSQLRunner, row calendarOutboxRow) (calendarOutboxRow, error) {
	targetCalendarURL := normalizeCalendarOutboxTargetURL(row.TargetCalendarURL)
	if targetCalendarURL == "" {
		var errorValue error
		targetCalendarURL, errorValue = readActiveCalendarOutboxTargetURLWithRunner(ctx, queryRunner, row.AccountID)
		if errorValue != nil {
			return calendarOutboxRow{}, errorValue
		}
	}
	if targetCalendarURL == "" {
		return calendarOutboxRow{}, calendarOutboxTargetUnavailableError(row.AccountID)
	}
	row.TargetCalendarURL = targetCalendarURL
	if strings.TrimSpace(row.RemoteHref) != "" && !calendarRemoteHrefBelongsToTargetURL(row.RemoteHref, targetCalendarURL) {
		row.RemoteHref = ""
		row.IfMatchETag = ""
	}
	return row, nil
}

func calendarOutboxTargetUnavailableError(accountID string) error {
	return fmt.Errorf("%w for account %q; reconnect the calendar account or select a complete writable calendar", errCalendarTargetUnavailable, strings.TrimSpace(accountID))
}

func readActiveCalendarOutboxTargetURLWithRunner(ctx context.Context, queryRunner calendarSQLRunner, accountID string) (string, error) {
	rows, errorValue := queryRunner.QueryContext(ctx, `SELECT selected_calendar_id, selected_calendar_url, default_calendar_url FROM calendar_remote_accounts WHERE id = ?`, strings.TrimSpace(accountID))
	if errorValue != nil {
		return "", errorValue
	}
	defer rows.Close()
	if !rows.Next() {
		return "", rows.Err()
	}
	var selectedCalendarID string
	var selectedCalendarURL string
	var defaultCalendarURL string
	if errorValue := rows.Scan(&selectedCalendarID, &selectedCalendarURL, &defaultCalendarURL); errorValue != nil {
		return "", errorValue
	}
	return resolveCalendarOutboxTargetURL(selectedCalendarID, selectedCalendarURL, defaultCalendarURL), nil
}

func resolveCalendarOutboxTargetURL(selectedCalendarID string, selectedCalendarURL string, defaultCalendarURL string) string {
	if strings.TrimSpace(selectedCalendarID) != "" && strings.TrimSpace(selectedCalendarURL) != "" {
		return normalizeCalendarOutboxTargetURL(selectedCalendarURL)
	}
	if strings.TrimSpace(selectedCalendarID) != "" || strings.TrimSpace(selectedCalendarURL) != "" {
		return ""
	}
	return normalizeCalendarOutboxTargetURL(defaultCalendarURL)
}

func backfillLegacyCalendarOutboxTargets(ctx context.Context, databaseRunner calendarSQLRunner) error {
	legacyRows, errorValue := readLegacyCalendarOutboxTargetRows(ctx, databaseRunner)
	if errorValue != nil {
		return errorValue
	}
	for _, row := range legacyRows {
		if errorValue := backfillLegacyCalendarOutboxTarget(ctx, databaseRunner, row); errorValue != nil {
			return fmt.Errorf("backfill calendar outbox row %d target: %w", row.ID, errorValue)
		}
	}
	return nil
}

func readLegacyCalendarOutboxTargetRows(ctx context.Context, queryRunner calendarSQLRunner) ([]legacyCalendarOutboxTargetRow, error) {
	rows, errorValue := queryRunner.QueryContext(ctx, `
SELECT id, account_id, operation
FROM calendar_outbox
WHERE target_calendar_url = ''
	AND NOT (
		status = ?
		AND (
			(operation = ? AND last_error = ?)
			OR (operation = ? AND last_error = ?)
		)
	)
ORDER BY id`,
		calendarOutboxStatusBlocked,
		calendarOutboxOperationDelete,
		legacyCalendarOutboxDeleteTargetUnknown,
		calendarOutboxOperationPut,
		legacyCalendarOutboxPutTargetUnavailable,
	)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	result := []legacyCalendarOutboxTargetRow{}
	for rows.Next() {
		var row legacyCalendarOutboxTargetRow
		if errorValue := rows.Scan(&row.ID, &row.AccountID, &row.Operation); errorValue != nil {
			return nil, errorValue
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func backfillLegacyCalendarOutboxTarget(ctx context.Context, queryRunner calendarSQLRunner, row legacyCalendarOutboxTargetRow) error {
	if row.Operation == calendarOutboxOperationDelete {
		_, errorValue := queryRunner.ExecContext(ctx, `UPDATE calendar_outbox SET status = ?, last_error = ? WHERE id = ?`, calendarOutboxStatusBlocked, legacyCalendarOutboxDeleteTargetUnknown, row.ID)
		return errorValue
	}
	activeTarget, errorValue := readActiveCalendarOutboxTargetURLWithRunner(ctx, queryRunner, row.AccountID)
	if errorValue != nil {
		return errorValue
	}
	if row.Operation == calendarOutboxOperationPut && activeTarget != "" {
		_, errorValue = queryRunner.ExecContext(ctx, `UPDATE calendar_outbox SET target_calendar_url = ?, remote_href = '', if_match_etag = '' WHERE id = ?`, activeTarget, row.ID)
		return errorValue
	}
	_, errorValue = queryRunner.ExecContext(ctx, `UPDATE calendar_outbox SET status = ?, last_error = ? WHERE id = ?`, calendarOutboxStatusBlocked, legacyCalendarOutboxPutTargetUnavailable, row.ID)
	return errorValue
}

func (service *Service) calendarOutboxTargetIsActive(ctx context.Context, row calendarOutboxRow) (bool, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return false, errorValue
	}
	defer database.Close()
	activeTarget, errorValue := readActiveCalendarOutboxTargetURLWithRunner(ctx, database, row.AccountID)
	if errorValue != nil {
		return false, errorValue
	}
	return normalizeCalendarOutboxTargetURL(row.TargetCalendarURL) == activeTarget, nil
}
