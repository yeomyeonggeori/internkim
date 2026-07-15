package admind

import (
	"context"
	"database/sql"
	"strings"
)

func (service *Service) openFlowDatabase(ctx context.Context) (*sql.DB, error) {
	options := sqliteDatabaseOptions{transactionLock: "immediate"}
	return service.openSQLiteDatabaseWithOptions(ctx, service.Configuration.FlowDatabasePath, ensureFlowSchema, options)
}

func ensureFlowSchema(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS flow_tasks (
	id TEXT PRIMARY KEY,
	week_code TEXT NOT NULL,
	owner_id TEXT NOT NULL,
	owner_name TEXT NOT NULL,
	participant_ids TEXT NOT NULL,
	participant_names TEXT NOT NULL,
	business TEXT NOT NULL,
	type TEXT NOT NULL,
	content TEXT NOT NULL,
	goal TEXT NOT NULL,
	size TEXT NOT NULL,
	status TEXT NOT NULL,
	start_date TEXT NOT NULL,
	end_date TEXT NOT NULL,
	flag INTEGER NOT NULL,
	request_reason TEXT NOT NULL,
	decision_reason TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
)`)
	if errorValue != nil {
		return errorValue
	}
	_, errorValue = database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS flow_definitions (
	kind TEXT NOT NULL,
	value TEXT NOT NULL,
	position INTEGER NOT NULL,
	PRIMARY KEY(kind, value)
)`)
	if errorValue != nil {
		return errorValue
	}
	_, errorValue = database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS flow_definition_meta (
	kind TEXT PRIMARY KEY,
	initialized INTEGER NOT NULL
)`)
	if errorValue != nil {
		return errorValue
	}
	_, errorValue = database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS flow_size_definitions (
	name TEXT PRIMARY KEY,
	distance_km INTEGER NOT NULL,
	max_hours INTEGER NOT NULL,
	development_example TEXT NOT NULL,
	other_example TEXT NOT NULL,
	note TEXT NOT NULL,
	position INTEGER NOT NULL
)`)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := ensureFlowColumn(ctx, database, "flow_tasks", "mattermost_post_id", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureFlowColumn(ctx, database, "flow_tasks", "mattermost_post_created_at", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureFlowColumn(ctx, database, "flow_tasks", "status_rank", "INTEGER NOT NULL DEFAULT 0"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureFlowColumn(ctx, database, "flow_tasks", "created_at", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := backfillFlowTaskCreatedAt(ctx, database); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureFlowTaskIndexes(ctx, database); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureFlowChannelOutboxTable(ctx, database); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureFlowSummaryCacheSchema(ctx, database); errorValue != nil {
		return errorValue
	}
	return seedFlowDefinitions(ctx, database)
}

func backfillFlowTaskCreatedAt(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, "UPDATE flow_tasks SET created_at = updated_at WHERE created_at = '' AND updated_at != ''")
	return errorValue
}

func ensureFlowTaskIndexes(ctx context.Context, database *sql.DB) error {
	for _, statement := range []string{
		"CREATE INDEX IF NOT EXISTS flow_tasks_week_code_idx ON flow_tasks(week_code)",
		"CREATE INDEX IF NOT EXISTS flow_tasks_start_date_idx ON flow_tasks(start_date)",
		"CREATE INDEX IF NOT EXISTS flow_tasks_end_date_idx ON flow_tasks(end_date)",
	} {
		if _, errorValue := database.ExecContext(ctx, statement); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func ensureFlowChannelOutboxTable(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
	CREATE TABLE IF NOT EXISTS flow_channel_outbox (
		task_id TEXT PRIMARY KEY,
		attempt_count INTEGER NOT NULL DEFAULT 0,
		last_error TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		last_attempted_at TEXT NOT NULL DEFAULT ''
	)`)
	return errorValue
}

func ensureFlowColumn(ctx context.Context, database *sql.DB, tableName string, columnName string, definition string) error {
	rows, errorValue := database.QueryContext(ctx, "PRAGMA table_info("+tableName+")")
	if errorValue != nil {
		return errorValue
	}
	defer rows.Close()
	for rows.Next() {
		var columnIndex int
		var existingColumnName string
		var columnType string
		var isNotNull int
		var defaultValue sql.NullString
		var primaryKey int
		if errorValue := rows.Scan(&columnIndex, &existingColumnName, &columnType, &isNotNull, &defaultValue, &primaryKey); errorValue != nil {
			return errorValue
		}
		if strings.EqualFold(existingColumnName, columnName) {
			return rows.Err()
		}
	}
	if errorValue := rows.Err(); errorValue != nil {
		return errorValue
	}
	_, errorValue = database.ExecContext(ctx, "ALTER TABLE "+tableName+" ADD COLUMN "+columnName+" "+definition)
	return errorValue
}

func seedFlowDefinitions(ctx context.Context, database *sql.DB) error {
	if errorValue := seedFlowDefinitionKind(ctx, database, "category", []string{}); errorValue != nil {
		return errorValue
	}
	if errorValue := seedFlowDefinitionKind(ctx, database, "type", defaultFlowTypes()); errorValue != nil {
		return errorValue
	}
	return seedFlowSizeDefinitions(ctx, database)
}

func seedFlowDefinitionKind(ctx context.Context, database *sql.DB, kind string, values []string) error {
	var initialized int
	errorValue := database.QueryRowContext(ctx, "SELECT initialized FROM flow_definition_meta WHERE kind = ?", kind).Scan(&initialized)
	if errorValue == nil && initialized == 1 {
		return nil
	}
	if errorValue != nil && errorValue != sql.ErrNoRows {
		return errorValue
	}
	for index, value := range values {
		if _, errorValue := database.ExecContext(ctx, "INSERT OR IGNORE INTO flow_definitions(kind, value, position) VALUES(?, ?, ?)", kind, value, index); errorValue != nil {
			return errorValue
		}
	}
	_, errorValue = database.ExecContext(ctx, "INSERT INTO flow_definition_meta(kind, initialized) VALUES(?, 1) ON CONFLICT(kind) DO UPDATE SET initialized = 1", kind)
	return errorValue
}

func seedFlowSizeDefinitions(ctx context.Context, database *sql.DB) error {
	var initialized int
	errorValue := database.QueryRowContext(ctx, "SELECT initialized FROM flow_definition_meta WHERE kind = ?", "size").Scan(&initialized)
	if errorValue == nil && initialized == 1 {
		return nil
	}
	if errorValue != nil && errorValue != sql.ErrNoRows {
		return errorValue
	}
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := replaceFlowSizeDefinitions(ctx, transaction, defaultFlowSizeDefinitions()); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}
