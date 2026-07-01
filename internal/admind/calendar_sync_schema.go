package admind

import (
	"context"
	"database/sql"
)

const (
	remoteCalendarProviderGoogle  = "google"
	calendarOutboxOperationPut    = "put"
	calendarOutboxOperationDelete = "delete"
)

func ensureCalendarSyncSchema(ctx context.Context, database *sql.DB) error {
	if errorValue := ensureCalendarRemoteAccountsTable(ctx, database); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarSyncStateTable(ctx, database); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarOutboxTable(ctx, database); errorValue != nil {
		return errorValue
	}
	return ensureCalendarConflictsTable(ctx, database)
}

func ensureCalendarConflictsTable(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS calendar_conflicts (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	event_id TEXT NOT NULL,
	event_uid TEXT NOT NULL,
	field TEXT NOT NULL,
	local_value TEXT NOT NULL DEFAULT '',
	remote_value TEXT NOT NULL DEFAULT '',
	detected_at TEXT NOT NULL,
	dismissed_at TEXT NOT NULL DEFAULT ''
)`)
	if errorValue != nil {
		return errorValue
	}
	_, errorValue = database.ExecContext(ctx,
		"CREATE INDEX IF NOT EXISTS calendar_conflicts_event_active_idx ON calendar_conflicts(event_id, dismissed_at)")
	return errorValue
}

func ensureCalendarRemoteAccountsTable(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS calendar_remote_accounts (
	id TEXT PRIMARY KEY,
	provider TEXT NOT NULL,
	account_email TEXT NOT NULL,
	principal_url TEXT NOT NULL DEFAULT '',
	home_set_url TEXT NOT NULL DEFAULT '',
	default_calendar_url TEXT NOT NULL DEFAULT '',
	default_calendar_ctag TEXT NOT NULL DEFAULT '',
	selected_calendar_id TEXT NOT NULL DEFAULT '',
	selected_calendar_summary TEXT NOT NULL DEFAULT '',
	selected_calendar_access_role TEXT NOT NULL DEFAULT '',
	selected_calendar_url TEXT NOT NULL DEFAULT '',
	selected_calendar_selected_at TEXT NOT NULL DEFAULT '',
	initial_sync_completed_at TEXT NOT NULL DEFAULT '',
	token_file_path TEXT NOT NULL DEFAULT '',
	last_auth_error TEXT NOT NULL DEFAULT '',
	last_auth_error_at TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(provider, account_email)
)`)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_remote_accounts", "principal_url", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_remote_accounts", "home_set_url", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_remote_accounts", "default_calendar_url", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_remote_accounts", "default_calendar_ctag", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_remote_accounts", "selected_calendar_id", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_remote_accounts", "selected_calendar_summary", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_remote_accounts", "selected_calendar_access_role", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_remote_accounts", "selected_calendar_url", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_remote_accounts", "selected_calendar_selected_at", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_remote_accounts", "initial_sync_completed_at", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_remote_accounts", "token_file_path", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_remote_accounts", "last_auth_error", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	return ensureCalendarColumn(ctx, database, "calendar_remote_accounts", "last_auth_error_at", "TEXT NOT NULL DEFAULT ''")
}

func ensureCalendarSyncStateTable(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS calendar_sync_state (
	account_id TEXT NOT NULL,
	calendar_url TEXT NOT NULL,
	last_ctag TEXT NOT NULL DEFAULT '',
	last_synced_at TEXT NOT NULL DEFAULT '',
	last_error TEXT NOT NULL DEFAULT '',
	PRIMARY KEY(account_id, calendar_url)
)`)
	return errorValue
}

func ensureCalendarOutboxTable(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS calendar_outbox (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	account_id TEXT NOT NULL,
	event_id TEXT NOT NULL,
	event_uid TEXT NOT NULL,
	operation TEXT NOT NULL,
	payload_ics TEXT NOT NULL DEFAULT '',
	if_match_etag TEXT NOT NULL DEFAULT '',
	remote_href TEXT NOT NULL DEFAULT '',
	attempt_count INTEGER NOT NULL DEFAULT 0,
	last_error TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	last_attempted_at TEXT NOT NULL DEFAULT ''
)`)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_outbox", "remote_href", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_outbox", "changed_fields", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	_, errorValue = database.ExecContext(ctx,
		"CREATE INDEX IF NOT EXISTS calendar_outbox_account_created_idx ON calendar_outbox(account_id, created_at)")
	return errorValue
}
