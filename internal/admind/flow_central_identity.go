package admind

import (
	"context"
	"database/sql"
	"strings"
)

// What the central plane called a task when it last took it. The device mints its
// own identifiers and the central plane mints uuids, so without this every drain
// would create a second copy. It is kept beside the task rather than on it: a
// removal has to travel after the task row is gone, and this row outlives it
// until the drain has carried the removal across.
func ensureFlowCentralIdentityTable(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
	CREATE TABLE IF NOT EXISTS flow_central_identity (
		task_id TEXT PRIMARY KEY,
		central_task_id TEXT NOT NULL
	)`)
	return errorValue
}

func rememberFlowCentralIdentity(ctx context.Context, executor sqlContextExecutor, taskID string, centralTaskID string) error {
	if strings.TrimSpace(taskID) == "" || strings.TrimSpace(centralTaskID) == "" {
		return nil
	}
	_, errorValue := executor.ExecContext(ctx, `
	INSERT INTO flow_central_identity (task_id, central_task_id)
	VALUES (?, ?)
	ON CONFLICT(task_id) DO UPDATE SET central_task_id = excluded.central_task_id`,
		taskID,
		centralTaskID,
	)
	return errorValue
}

type sqlContextQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func readFlowCentralIdentity(ctx context.Context, querier sqlContextQuerier, taskID string) (string, error) {
	centralTaskID := ""
	errorValue := querier.QueryRowContext(ctx,
		"SELECT central_task_id FROM flow_central_identity WHERE task_id = ?", taskID).Scan(&centralTaskID)
	if errorValue == sql.ErrNoRows {
		return "", nil
	}
	return centralTaskID, errorValue
}

func readFlowTaskIDCarrying(ctx context.Context, querier sqlContextQuerier, centralTaskID string) (string, error) {
	taskID := ""
	errorValue := querier.QueryRowContext(ctx,
		"SELECT task_id FROM flow_central_identity WHERE central_task_id = ?", centralTaskID).Scan(&taskID)
	if errorValue == sql.ErrNoRows {
		return "", nil
	}
	return taskID, errorValue
}

func forgetFlowCentralIdentity(ctx context.Context, executor sqlContextExecutor, taskID string) error {
	_, errorValue := executor.ExecContext(ctx, "DELETE FROM flow_central_identity WHERE task_id = ?", taskID)
	return errorValue
}

func (service *Service) readFlowCentralIdentityByTaskID(ctx context.Context, taskID string) (string, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return "", errorValue
	}
	defer database.Close()
	return readFlowCentralIdentity(ctx, database, taskID)
}
