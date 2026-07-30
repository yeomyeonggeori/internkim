package admind

import (
	"context"
	"database/sql"
)

func ensureAttendanceLeaveRequestSchema(ctx context.Context, database *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS attendance_leave_requests (
			id TEXT PRIMARY KEY,
			employee_email TEXT NOT NULL COLLATE NOCASE,
			user_id TEXT NOT NULL,
			leave_type_id TEXT NOT NULL,
			leave_type_name TEXT NOT NULL,
			balance_mode TEXT NOT NULL,
			unit TEXT NOT NULL,
			partial_period TEXT NOT NULL,
			start_date TEXT NOT NULL,
			end_date TEXT NOT NULL,
			start_time TEXT NOT NULL,
			reason TEXT NOT NULL,
			admin_response TEXT NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('pending', 'needsChanges', 'approved', 'rejected', 'cancelled')),
			total_deduction_milli_days INTEGER NOT NULL CHECK (total_deduction_milli_days > 0),
			revision INTEGER NOT NULL DEFAULT 1 CHECK (revision > 0),
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			cancelled_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS attendance_leave_request_occurrences (
			id TEXT PRIMARY KEY,
			request_id TEXT NOT NULL REFERENCES attendance_leave_requests(id) ON DELETE CASCADE,
			date TEXT NOT NULL,
			start_time TEXT NOT NULL,
			end_time TEXT NOT NULL,
			time_zone TEXT NOT NULL,
			deduction_milli_days INTEGER NOT NULL CHECK (deduction_milli_days > 0),
			created_at TEXT NOT NULL,
			UNIQUE (request_id, date)
		)`,
		`CREATE TABLE IF NOT EXISTS attendance_leave_request_events (
			id TEXT PRIMARY KEY,
			request_id TEXT NOT NULL REFERENCES attendance_leave_requests(id) ON DELETE CASCADE,
			kind TEXT NOT NULL,
			actor_email TEXT NOT NULL,
			response TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS attendance_leave_request_attachments (
			id TEXT PRIMARY KEY,
			request_id TEXT NOT NULL REFERENCES attendance_leave_requests(id) ON DELETE CASCADE,
			file_name TEXT NOT NULL,
			content_type TEXT NOT NULL,
			size_bytes INTEGER NOT NULL CHECK (size_bytes > 0),
			storage_key TEXT NOT NULL UNIQUE,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS attendance_leave_request_absence_ranges (
			request_id TEXT NOT NULL REFERENCES attendance_leave_requests(id) ON DELETE CASCADE,
			range_id TEXT NOT NULL REFERENCES attendance_absence_ranges(id),
			PRIMARY KEY (request_id, range_id)
		)`,
		`CREATE INDEX IF NOT EXISTS attendance_leave_requests_employee_updated ON attendance_leave_requests(employee_email, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS attendance_leave_requests_active_dates ON attendance_leave_requests(employee_email, status, start_date, end_date)`,
		`CREATE INDEX IF NOT EXISTS attendance_leave_request_occurrences_active_date ON attendance_leave_request_occurrences(date, request_id)`,
		`CREATE INDEX IF NOT EXISTS attendance_leave_request_events_request_created ON attendance_leave_request_events(request_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS attendance_leave_request_attachments_request ON attendance_leave_request_attachments(request_id, created_at)`,
	}
	for _, statement := range statements {
		if _, errorValue := database.ExecContext(ctx, statement); errorValue != nil {
			return errorValue
		}
	}
	return nil
}
