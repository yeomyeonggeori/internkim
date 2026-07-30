package admind

import (
	"context"
	"database/sql"
)

func (service *Service) openAttendanceLegacyAbsenceMigrationDatabase(
	ctx context.Context,
	mutation bool,
) (*sql.DB, error) {
	var database *sql.DB
	var errorValue error
	if mutation {
		database, errorValue = service.openAttendanceLeaveMutationDatabase(ctx)
	} else {
		database, errorValue = service.openAttendanceDatabase(ctx)
	}
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := ensureAttendanceLegacyAbsenceMigrationSchema(ctx, database); errorValue != nil {
		database.Close()
		return nil, errorValue
	}
	return database, nil
}

func ensureAttendanceLegacyAbsenceMigrationSchema(
	ctx context.Context,
	database *sql.DB,
) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS attendance_legacy_absence_migration_batches (
			id TEXT PRIMARY KEY,
			status TEXT NOT NULL CHECK (status IN ('applied', 'rolledBack')),
			actor_email TEXT NOT NULL,
			leave_count INTEGER NOT NULL CHECK (leave_count >= 0),
			other_count INTEGER NOT NULL CHECK (other_count >= 0),
			created_at TEXT NOT NULL,
			rolled_back_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS attendance_legacy_absence_migration_items (
			batch_id TEXT NOT NULL REFERENCES attendance_legacy_absence_migration_batches(id),
			range_id TEXT NOT NULL REFERENCES attendance_absence_ranges(id),
			request_id TEXT NOT NULL,
			migrated_at TEXT NOT NULL,
			rolled_back_at TEXT NOT NULL,
			PRIMARY KEY (batch_id, range_id)
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS attendance_legacy_absence_migration_active_range
			ON attendance_legacy_absence_migration_items(range_id)
			WHERE rolled_back_at = ''`,
		`CREATE INDEX IF NOT EXISTS attendance_legacy_absence_migration_batch
			ON attendance_legacy_absence_migration_items(batch_id, rolled_back_at)`,
	}
	for _, statement := range statements {
		if _, errorValue := database.ExecContext(ctx, statement); errorValue != nil {
			return errorValue
		}
	}
	return nil
}
