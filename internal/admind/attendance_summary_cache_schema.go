package admind

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func ensureAttendanceSummaryCacheSchema(ctx context.Context, database *sql.DB) error {
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return fmt.Errorf("begin attendance summary cache schema: %w", errorValue)
	}
	defer transaction.Rollback()
	rebuild, errorValue := attendanceSummaryCacheSchemaNeedsRebuild(ctx, transaction)
	if errorValue != nil {
		return errorValue
	}
	if rebuild {
		if errorValue := dropAttendanceSummaryCacheTables(ctx, transaction); errorValue != nil {
			return errorValue
		}
	}
	if errorValue := createAttendanceSummaryCacheTables(ctx, transaction); errorValue != nil {
		return errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return fmt.Errorf("commit attendance summary cache schema: %w", errorValue)
	}
	return nil
}

func attendanceSummaryCacheSchemaNeedsRebuild(ctx context.Context, transaction *sql.Tx) (bool, error) {
	tables := []struct {
		name       string
		columns    string
		primaryKey string
	}{
		{
			name:       "attendance_summary_cache_revisions",
			columns:    "cache_kind,month,revision",
			primaryKey: "cache_kind,month",
		},
		{
			name:       "attendance_summary_cache_entries",
			columns:    "cache_kind,month,generation,source_revision,schema_version,payload_json,cached_at",
			primaryKey: "cache_kind,month",
		},
	}
	for _, table := range tables {
		exists, columns, primaryKey, errorValue := readAttendanceSummaryCacheTableShape(ctx, transaction, table.name)
		if errorValue != nil {
			return false, errorValue
		}
		if exists && (columns != table.columns || primaryKey != table.primaryKey) {
			return true, nil
		}
	}
	return false, nil
}

func readAttendanceSummaryCacheTableShape(
	ctx context.Context,
	transaction *sql.Tx,
	tableName string,
) (bool, string, string, error) {
	rows, errorValue := transaction.QueryContext(ctx, "SELECT name, pk FROM pragma_table_info(?) ORDER BY cid", tableName)
	if errorValue != nil {
		return false, "", "", fmt.Errorf("read attendance summary cache table %s: %w", tableName, errorValue)
	}
	defer rows.Close()
	columns := []string{}
	primaryKeyColumns := map[int]string{}
	for rows.Next() {
		var columnName string
		var primaryKeyPosition int
		if errorValue := rows.Scan(&columnName, &primaryKeyPosition); errorValue != nil {
			return false, "", "", fmt.Errorf("scan attendance summary cache table %s: %w", tableName, errorValue)
		}
		columns = append(columns, columnName)
		if primaryKeyPosition > 0 {
			primaryKeyColumns[primaryKeyPosition] = columnName
		}
	}
	if errorValue := rows.Err(); errorValue != nil {
		return false, "", "", fmt.Errorf("read attendance summary cache table %s: %w", tableName, errorValue)
	}
	primaryKey := make([]string, 0, len(primaryKeyColumns))
	for position := 1; position <= len(primaryKeyColumns); position++ {
		primaryKey = append(primaryKey, primaryKeyColumns[position])
	}
	return len(columns) > 0, strings.Join(columns, ","), strings.Join(primaryKey, ","), nil
}

func dropAttendanceSummaryCacheTables(ctx context.Context, transaction *sql.Tx) error {
	for _, tableName := range []string{"attendance_summary_cache_entries", "attendance_summary_cache_revisions"} {
		if _, errorValue := transaction.ExecContext(ctx, "DROP TABLE IF EXISTS "+tableName); errorValue != nil {
			return fmt.Errorf("drop attendance summary cache table %s: %w", tableName, errorValue)
		}
	}
	return nil
}

func createAttendanceSummaryCacheTables(ctx context.Context, transaction *sql.Tx) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS attendance_summary_cache_revisions (
			cache_kind TEXT NOT NULL,
			month TEXT NOT NULL,
			revision INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (cache_kind, month)
		)`,
		`CREATE TABLE IF NOT EXISTS attendance_summary_cache_entries (
			cache_kind TEXT NOT NULL,
			month TEXT NOT NULL,
			generation TEXT NOT NULL,
			source_revision INTEGER NOT NULL,
			schema_version INTEGER NOT NULL,
			payload_json TEXT NOT NULL,
			cached_at TEXT NOT NULL,
			PRIMARY KEY (cache_kind, month)
		)`,
	}
	for _, statement := range statements {
		if _, errorValue := transaction.ExecContext(ctx, statement); errorValue != nil {
			return fmt.Errorf("create attendance summary cache schema: %w", errorValue)
		}
	}
	return nil
}
