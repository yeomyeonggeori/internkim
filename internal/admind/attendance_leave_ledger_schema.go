package admind

import (
	"context"
	"database/sql"
)

func ensureAttendanceLeaveLedgerSchema(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS attendance_leave_operations (
	operation_key TEXT PRIMARY KEY,
	employee_email TEXT NOT NULL COLLATE NOCASE,
	user_id TEXT NOT NULL DEFAULT '',
	leave_type_id TEXT NOT NULL,
	kind TEXT NOT NULL CHECK (
		kind IN (
			'grant', 'reserve', 'use', 'release', 'expire', 'carryover',
			'legalCorrection', 'adjustment', 'untrackedUse'
		)
	),
	reference_id TEXT NOT NULL DEFAULT '',
	amount_milli_days INTEGER NOT NULL CHECK (amount_milli_days >= 0),
	effective_date TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS attendance_leave_grant_lots (
	id TEXT PRIMARY KEY,
	employee_email TEXT NOT NULL COLLATE NOCASE,
	user_id TEXT NOT NULL DEFAULT '',
	leave_type_id TEXT NOT NULL,
	source_operation_key TEXT NOT NULL UNIQUE REFERENCES attendance_leave_operations(operation_key),
	granted_on TEXT NOT NULL,
	expires_on TEXT,
	original_milli_days INTEGER NOT NULL CHECK (original_milli_days >= 0),
	available_milli_days INTEGER NOT NULL CHECK (available_milli_days >= 0),
	reserved_milli_days INTEGER NOT NULL CHECK (reserved_milli_days >= 0),
	used_milli_days INTEGER NOT NULL CHECK (used_milli_days >= 0),
	expired_milli_days INTEGER NOT NULL CHECK (expired_milli_days >= 0),
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	CHECK (
		available_milli_days + reserved_milli_days + used_milli_days + expired_milli_days
		= original_milli_days
	)
);

CREATE TABLE IF NOT EXISTS attendance_leave_ledger_entries (
	id TEXT PRIMARY KEY,
	operation_key TEXT NOT NULL REFERENCES attendance_leave_operations(operation_key),
	sequence INTEGER NOT NULL CHECK (sequence >= 0),
	grant_lot_id TEXT REFERENCES attendance_leave_grant_lots(id),
	kind TEXT NOT NULL CHECK (
		kind IN (
			'grant', 'reserve', 'use', 'release', 'expire', 'carryoverOut',
			'carryoverIn', 'legalCorrection', 'adjustment', 'untrackedUse'
		)
	),
	amount_milli_days INTEGER NOT NULL CHECK (amount_milli_days >= 0),
	available_delta_milli_days INTEGER NOT NULL,
	reserved_delta_milli_days INTEGER NOT NULL,
	used_delta_milli_days INTEGER NOT NULL,
	expired_delta_milli_days INTEGER NOT NULL,
	available_after_milli_days INTEGER NOT NULL CHECK (available_after_milli_days >= 0),
	effective_at TEXT NOT NULL,
	created_at TEXT NOT NULL,
	UNIQUE (operation_key, sequence)
);

CREATE INDEX IF NOT EXISTS attendance_leave_lots_employee_type_expiry
ON attendance_leave_grant_lots(employee_email, leave_type_id, expires_on, granted_on);

CREATE INDEX IF NOT EXISTS attendance_leave_ledger_operation_created
ON attendance_leave_ledger_entries(operation_key, created_at);

CREATE UNIQUE INDEX IF NOT EXISTS attendance_leave_reservation_reference
ON attendance_leave_operations(reference_id)
WHERE kind = 'reserve' AND reference_id <> '';
`)
	return errorValue
}
