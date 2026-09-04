package admind

import (
	"context"
	"database/sql"
	"log/slog"
)

// Caches, queues and projections of rows held elsewhere. Nothing in them is a
// record of anything, so they go whether or not the company holds their source.
var derivedTaskTables = []string{
	"flow_summary_cache_entries",
	"flow_summary_source_revisions",
	"flow_definition_meta",
	"flow_channel_outbox",
}

// What a device recorded before the company did. These drop only once the
// record holds all of it.
var carriedTaskTables = []string{
	"flow_tasks",
	"task_carried_rows",
}

// The words a task may carry have been the company's since #1440, and nothing
// on a device has written these since. A company that answers a vocabulary of
// its own covers them.
var vocabularyTaskTables = []string{
	"flow_definitions",
	"flow_size_definitions",
}

func (service *Service) openTaskDatabase(ctx context.Context) (*sql.DB, error) {
	return service.openStateDatabase(ctx, "flow", ensureTaskSchema, sqliteDatabaseOptions{})
}

// The store is not created any more, only opened to be let go of. A device that
// never held a task has nothing here and the schema stays as it was found.
func ensureTaskSchema(context.Context, *sql.DB) error {
	return nil
}

func (service *Service) startTaskSweep(ctx context.Context) {
	go service.sweepTheTasksTheCompanyNowHolds(ctx)
}

func dropTaskTables(ctx context.Context, database *sql.DB, tableNames []string) error {
	for _, tableName := range tableNames {
		rowCount, held := countRowsInTaskTable(ctx, database, tableName)
		if !held {
			continue
		}
		if _, errorValue := database.ExecContext(ctx, "DROP TABLE IF EXISTS "+tableName); errorValue != nil {
			return errorValue
		}
		slog.InfoContext(ctx, "dropped a task table the company now holds",
			"table", tableName, "rows", rowCount)
	}
	return nil
}

func countRowsInTaskTable(ctx context.Context, database *sql.DB, tableName string) (int, bool) {
	var rowCount int
	errorValue := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+tableName).Scan(&rowCount)
	if errorValue != nil {
		return 0, false
	}
	return rowCount, true
}
