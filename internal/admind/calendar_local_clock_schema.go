package admind

import (
	"context"
	"database/sql"
)

func ensureCalendarEventLogicalClocksTable(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS calendar_event_logical_clocks (
	event_uid TEXT PRIMARY KEY,
	logical_time_unix_nano INTEGER NOT NULL
)`)
	return errorValue
}
