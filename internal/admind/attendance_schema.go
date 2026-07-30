package admind

import (
	"context"
	"database/sql"
)

func ensureAttendanceSchema(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS attendance_events (
	id TEXT PRIMARY KEY,
	mattermost_user_id TEXT NOT NULL,
	mattermost_username TEXT NOT NULL,
	email TEXT NOT NULL,
	display_name TEXT NOT NULL,
	kind TEXT NOT NULL,
	occurred_at TEXT NOT NULL,
	local_date TEXT NOT NULL,
	local_time TEXT NOT NULL,
	time_zone_at_event TEXT NOT NULL,
	source TEXT NOT NULL,
	team_id TEXT NOT NULL,
	channel_id TEXT NOT NULL,
	action_post_id TEXT NOT NULL,
	result_post_id TEXT NOT NULL,
	location_id TEXT NOT NULL DEFAULT '',
	location_name TEXT NOT NULL DEFAULT '',
	canceled_at TEXT NOT NULL,
	cancel_reason TEXT NOT NULL,
	repeated_click_at TEXT NOT NULL
)`)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := ensureAttendanceColumn(ctx, database, "location_id", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureAttendanceColumn(ctx, database, "location_name", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if _, errorValue := database.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS attendance_absence_ranges (
		id TEXT PRIMARY KEY,
		email TEXT NOT NULL,
		kind TEXT NOT NULL,
		start_date TEXT NOT NULL,
		end_date TEXT NOT NULL,
		reason TEXT NOT NULL,
		created_by TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		canceled_at TEXT NOT NULL,
		replaced_by TEXT NOT NULL DEFAULT ''
	)`); errorValue != nil {
		return errorValue
	}
	if _, errorValue := database.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS attendance_absence_occurrences (
		id TEXT PRIMARY KEY,
		range_id TEXT NOT NULL,
		email TEXT NOT NULL,
		date TEXT NOT NULL,
		created_at TEXT NOT NULL,
		canceled_at TEXT NOT NULL
	)`); errorValue != nil {
		return errorValue
	}
	if _, errorValue := database.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS attendance_event_overrides (
		id TEXT PRIMARY KEY,
		event_id TEXT NOT NULL,
		edited_by TEXT NOT NULL,
		edited_at TEXT NOT NULL,
		reason TEXT NOT NULL,
		original_occurred_at TEXT NOT NULL,
		original_local_date TEXT NOT NULL,
		original_local_time TEXT NOT NULL,
		original_location_id TEXT NOT NULL,
		original_location_name TEXT NOT NULL,
		override_occurred_at TEXT NOT NULL,
		override_local_date TEXT NOT NULL,
		override_local_time TEXT NOT NULL,
		override_location_id TEXT NOT NULL,
		override_location_name TEXT NOT NULL
	)`); errorValue != nil {
		return errorValue
	}
	if _, errorValue := database.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS attendance_events_user_date ON attendance_events(email, local_date)"); errorValue != nil {
		return errorValue
	}
	if _, errorValue := database.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS attendance_events_local_date ON attendance_events(local_date)"); errorValue != nil {
		return errorValue
	}
	if _, errorValue := database.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS attendance_events_result_post ON attendance_events(result_post_id)"); errorValue != nil {
		return errorValue
	}
	if _, errorValue := database.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS attendance_event_overrides_event_edited ON attendance_event_overrides(event_id, edited_at)"); errorValue != nil {
		return errorValue
	}
	if _, errorValue := database.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS attendance_event_overrides_date ON attendance_event_overrides(override_local_date)"); errorValue != nil {
		return errorValue
	}
	if _, errorValue := database.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS attendance_absence_ranges_active_dates ON attendance_absence_ranges(canceled_at, start_date, end_date)"); errorValue != nil {
		return errorValue
	}
	if _, errorValue := database.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS attendance_absence_ranges_active_user_dates ON attendance_absence_ranges(email, canceled_at, start_date, end_date)"); errorValue != nil {
		return errorValue
	}
	if _, errorValue := database.ExecContext(ctx, "CREATE INDEX IF NOT EXISTS attendance_absence_occurrences_range ON attendance_absence_occurrences(range_id, canceled_at)"); errorValue != nil {
		return errorValue
	}
	if _, errorValue = database.ExecContext(ctx, "CREATE UNIQUE INDEX IF NOT EXISTS attendance_absence_occurrences_active_user_date ON attendance_absence_occurrences(email, date) WHERE canceled_at = ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureAttendanceSummaryCacheSchema(ctx, database); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureAttendanceLeaveLedgerSchema(ctx, database); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureAttendanceLeaveRequestSchema(ctx, database); errorValue != nil {
		return errorValue
	}
	return nil
}

func ensureAttendanceColumn(ctx context.Context, database *sql.DB, columnName string, columnDefinition string) error {
	rows, errorValue := database.QueryContext(ctx, "PRAGMA table_info(attendance_events)")
	if errorValue != nil {
		return errorValue
	}
	defer rows.Close()
	for rows.Next() {
		var columnIndex int
		var name string
		var columnType string
		var isNotNull int
		var defaultValue any
		var primaryKey int
		if errorValue := rows.Scan(&columnIndex, &name, &columnType, &isNotNull, &defaultValue, &primaryKey); errorValue != nil {
			return errorValue
		}
		if name == columnName {
			return rows.Err()
		}
	}
	if errorValue := rows.Err(); errorValue != nil {
		return errorValue
	}
	_, errorValue = database.ExecContext(ctx, "ALTER TABLE attendance_events ADD COLUMN "+columnName+" "+columnDefinition)
	return errorValue
}
