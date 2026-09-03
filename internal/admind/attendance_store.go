package admind

import (
	"context"
	"database/sql"
	"log/slog"
)

const (
	attendanceKindClockIn  = "clock_in"
	attendanceKindClockOut = "clock_out"
)

// Caches and projections of rows held elsewhere in this same store. Nothing in
// them is a record of anything, so they go whether or not the company holds
// what they were derived from.
var derivedAttendanceTables = []string{
	"attendance_summary_cache_entries",
	"attendance_summary_cache_revisions",
	"attendance_absence_occurrences",
	"attendance_leave_request_occurrences",
	"attendance_leave_request_absence_ranges",
	"attendance_leave_operations",
}

// What a device recorded before the company did. These drop only once the
// record holds all of it.
var carriedAttendanceTables = []string{
	"attendance_events",
	"attendance_event_overrides",
	"attendance_leave_requests",
	"attendance_leave_request_events",
	"attendance_carried_rows",
}

// The record has no place for these yet, so a device holding any of them keeps
// its store and says which decision is missing.
var attendanceTablesWithNowhereToGo = map[string]string{
	"attendance_leave_request_attachments": "https://github.com/yeomyeonggeori/internkim/issues/1394",
	"attendance_leave_grant_lots":          "https://github.com/yeomyeonggeori/internkim/issues/1396",
	"attendance_leave_ledger_entries":      "https://github.com/yeomyeonggeori/internkim/issues/1396",
	"attendance_absence_ranges":            "https://github.com/yeomyeonggeori/internkim/issues/1396",
}

func (service *Service) openAttendanceDatabase(ctx context.Context) (*sql.DB, error) {
	return service.openStateDatabase(ctx, "attendance", ensureAttendanceSchema, sqliteDatabaseOptions{})
}

// The store is not created any more, only opened to be let go of. A device that
// never held attendance has nothing here and the schema stays as it was found.
func ensureAttendanceSchema(context.Context, *sql.DB) error {
	return nil
}

func (service *Service) startAttendanceSweep(ctx context.Context) {
	go service.sweepTheAttendanceTheCompanyNowHolds(ctx)
}

func dropAttendanceTables(ctx context.Context, database *sql.DB, tableNames []string) error {
	for _, tableName := range tableNames {
		rowCount, held := countRowsInAttendanceTable(ctx, database, tableName)
		if !held {
			continue
		}
		if _, errorValue := database.ExecContext(ctx, "DROP TABLE IF EXISTS "+tableName); errorValue != nil {
			return errorValue
		}
		slog.InfoContext(ctx, "dropped an attendance table the company now holds",
			"table", tableName, "rows", rowCount)
	}
	return nil
}

func countRowsInAttendanceTable(ctx context.Context, database *sql.DB, tableName string) (int, bool) {
	var rowCount int
	errorValue := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+tableName).Scan(&rowCount)
	if errorValue != nil {
		return 0, false
	}
	return rowCount, true
}
