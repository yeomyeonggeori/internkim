package admind

import (
	"context"
	"database/sql"
	"time"
)

const taskNotifyBaselineName = "task-notify"

func ensureTaskNotifySchema(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS task_notify_mark (
	task_run_id TEXT PRIMARY KEY,
	notified_status TEXT NOT NULL,
	marked_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS task_notify_baseline (
	name TEXT PRIMARY KEY,
	seeded_at TEXT NOT NULL
)`)
	return errorValue
}

func (service *Service) openTaskNotifyDatabase(ctx context.Context) (*sql.DB, error) {
	options := sqliteDatabaseOptions{transactionLock: "immediate"}
	return service.openStateDatabase(ctx, "task-notify", ensureTaskNotifySchema, options)
}

func (service *Service) taskNotifyBaselineSeeded(ctx context.Context) (bool, error) {
	database, errorValue := service.openTaskNotifyDatabase(ctx)
	if errorValue != nil {
		return false, errorValue
	}
	defer database.Close()

	seededAt := ""
	errorValue = database.QueryRowContext(ctx,
		"SELECT seeded_at FROM task_notify_baseline WHERE name = ?", taskNotifyBaselineName).Scan(&seededAt)
	if errorValue == sql.ErrNoRows {
		return false, nil
	}
	return seededAt != "", errorValue
}

func (service *Service) readTaskNotifyMarks(ctx context.Context) (map[string]string, error) {
	database, errorValue := service.openTaskNotifyDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()

	rows, errorValue := database.QueryContext(ctx, "SELECT task_run_id, notified_status FROM task_notify_mark")
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()

	marks := map[string]string{}
	for rows.Next() {
		taskRunID := ""
		status := ""
		if errorValue := rows.Scan(&taskRunID, &status); errorValue != nil {
			return nil, errorValue
		}
		marks[taskRunID] = status
	}
	return marks, rows.Err()
}

func (service *Service) writeTaskNotifyMarks(ctx context.Context, statusByTaskRunID map[string]string, at time.Time) error {
	if len(statusByTaskRunID) == 0 {
		return nil
	}
	database, errorValue := service.openTaskNotifyDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()

	marked := at.UTC().Format(time.RFC3339)
	for taskRunID, status := range statusByTaskRunID {
		if _, errorValue := database.ExecContext(ctx, `
INSERT INTO task_notify_mark (task_run_id, notified_status, marked_at) VALUES (?, ?, ?)
ON CONFLICT (task_run_id) DO UPDATE SET notified_status = excluded.notified_status, marked_at = excluded.marked_at`,
			taskRunID, status, marked); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func (service *Service) writeTaskNotifyBaseline(ctx context.Context, at time.Time) error {
	database, errorValue := service.openTaskNotifyDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()

	_, errorValue = database.ExecContext(ctx, `
INSERT INTO task_notify_baseline (name, seeded_at) VALUES (?, ?)
ON CONFLICT (name) DO NOTHING`, taskNotifyBaselineName, at.UTC().Format(time.RFC3339))
	return errorValue
}

func (service *Service) forgetStaleTaskNotifyMarks(ctx context.Context, before time.Time) error {
	database, errorValue := service.openTaskNotifyDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()

	_, errorValue = database.ExecContext(ctx,
		"DELETE FROM task_notify_mark WHERE marked_at < ?", before.UTC().Format(time.RFC3339))
	return errorValue
}
