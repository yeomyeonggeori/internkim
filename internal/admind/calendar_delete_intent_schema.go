package admind

import (
	"context"
	"database/sql"
)

func ensureCalendarDeleteIntentSchema(ctx context.Context, database *sql.DB) error {
	if _, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS calendar_delete_intents (
	operation_id TEXT PRIMARY KEY,
	event_id TEXT NOT NULL,
	client_id TEXT NOT NULL,
	sequence INTEGER NOT NULL,
	expected_updated_at TEXT NOT NULL,
	requested_at TEXT NOT NULL,
	execute_at TEXT NOT NULL,
	status TEXT NOT NULL,
	resolved_at TEXT NOT NULL DEFAULT '',
	resolution_sequence INTEGER NOT NULL DEFAULT 0,
	next_attempt_at TEXT NOT NULL,
	attempt_count INTEGER NOT NULL DEFAULT 0,
	last_error TEXT NOT NULL DEFAULT ''
)`); errorValue != nil {
		return errorValue
	}
	if _, errorValue := database.ExecContext(ctx, `
CREATE INDEX IF NOT EXISTS calendar_delete_intents_pending_at_idx
ON calendar_delete_intents(status, execute_at, next_attempt_at)`); errorValue != nil {
		return errorValue
	}
	_, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS calendar_event_mutation_origins (
	event_id TEXT NOT NULL,
	resulting_updated_at TEXT NOT NULL,
	client_id TEXT NOT NULL,
	sequence INTEGER NOT NULL,
	PRIMARY KEY(event_id, resulting_updated_at)
)`)
	return errorValue
}
