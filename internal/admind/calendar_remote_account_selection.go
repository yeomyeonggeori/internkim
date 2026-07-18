package admind

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func updateSelectedRemoteCalendarAccountWithRunner(ctx context.Context, queryRunner calendarSQLRunner, account remoteCalendarAccount) (remoteCalendarAccount, error) {
	accountID := strings.TrimSpace(account.ID)
	updatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	result, errorValue := queryRunner.ExecContext(ctx, `
UPDATE calendar_remote_accounts
SET default_calendar_ctag = ?,
	selected_calendar_id = ?,
	selected_calendar_summary = ?,
	selected_calendar_access_role = ?,
	selected_calendar_url = ?,
	selected_calendar_selected_at = ?,
	selected_calendar_readiness_status = CASE
		WHEN selected_calendar_id = ? AND selected_calendar_url = ? AND initial_sync_completed_at <> '' THEN ?
		ELSE ?
	END,
	initial_sync_completed_at = CASE
		WHEN selected_calendar_id = ? AND selected_calendar_url = ? AND initial_sync_completed_at <> '' THEN initial_sync_completed_at
		ELSE ?
	END,
	updated_at = ?
WHERE id = ?`,
		strings.TrimSpace(account.DefaultCalendarCTag),
		strings.TrimSpace(account.SelectedCalendarID),
		strings.TrimSpace(account.SelectedCalendarSummary),
		strings.TrimSpace(account.SelectedCalendarAccessRole),
		strings.TrimSpace(account.SelectedCalendarURL),
		strings.TrimSpace(account.SelectedCalendarSelectedAt),
		strings.TrimSpace(account.SelectedCalendarID),
		strings.TrimSpace(account.SelectedCalendarURL),
		calendarReadinessStatusSyncReady,
		strings.TrimSpace(account.SelectedCalendarReadinessStatus),
		strings.TrimSpace(account.SelectedCalendarID),
		strings.TrimSpace(account.SelectedCalendarURL),
		strings.TrimSpace(account.InitialSyncCompletedAt),
		updatedAt,
		accountID,
	)
	if errorValue != nil {
		return remoteCalendarAccount{}, fmt.Errorf("update selected remote calendar account %q: %w", accountID, errorValue)
	}
	affectedRows, errorValue := result.RowsAffected()
	if errorValue != nil {
		return remoteCalendarAccount{}, fmt.Errorf("read selected remote calendar account %q update count: %w", accountID, errorValue)
	}
	if affectedRows == 0 {
		return remoteCalendarAccount{}, sql.ErrNoRows
	}
	rows, errorValue := queryRunner.QueryContext(ctx, `
SELECT id, provider, account_email, principal_url, home_set_url, default_calendar_url, default_calendar_ctag, selected_calendar_id, selected_calendar_summary, selected_calendar_access_role, selected_calendar_url, selected_calendar_selected_at, selected_calendar_readiness_status, initial_sync_completed_at, token_file_path, last_auth_error, last_auth_error_at, created_at, updated_at
FROM calendar_remote_accounts
WHERE id = ?`, accountID)
	if errorValue != nil {
		return remoteCalendarAccount{}, fmt.Errorf("read selected remote calendar account %q: %w", accountID, errorValue)
	}
	defer rows.Close()
	if !rows.Next() {
		if errorValue := rows.Err(); errorValue != nil {
			return remoteCalendarAccount{}, fmt.Errorf("read selected remote calendar account %q: %w", accountID, errorValue)
		}
		return remoteCalendarAccount{}, sql.ErrNoRows
	}
	updatedAccount, errorValue := scanRemoteCalendarAccount(rows)
	if errorValue != nil {
		return remoteCalendarAccount{}, fmt.Errorf("scan selected remote calendar account %q: %w", accountID, errorValue)
	}
	return updatedAccount, nil
}
