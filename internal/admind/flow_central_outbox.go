package admind

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// The device writes to its own store first and queues the central plane, because
// the uplink drops and an agent that must reach Supabase before it can record
// anything stops working whenever the link does. See
// docs/internal/task-sync-direction.md §3.

const (
	flowCentralWriteIntent  = "write"
	flowCentralDeleteIntent = "delete"
)

type flowCentralOutboxEntry struct {
	TaskID       string
	Intent       string
	AttemptCount int
	LastError    string
}

func ensureFlowCentralOutboxTable(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
	CREATE TABLE IF NOT EXISTS flow_central_outbox (
		task_id TEXT PRIMARY KEY,
		intent TEXT NOT NULL DEFAULT 'write',
		attempt_count INTEGER NOT NULL DEFAULT 0,
		last_error TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		last_attempted_at TEXT NOT NULL DEFAULT ''
	)`)
	return errorValue
}

func enqueueFlowCentralWrite(ctx context.Context, executor sqlContextExecutor, taskID string) error {
	return enqueueFlowCentralIntent(ctx, executor, taskID, flowCentralWriteIntent)
}

func enqueueFlowCentralDelete(ctx context.Context, executor sqlContextExecutor, taskID string) error {
	return enqueueFlowCentralIntent(ctx, executor, taskID, flowCentralDeleteIntent)
}

// A delete replaces whatever the row said: a task written and then removed while
// the link was down has nothing to send but its removal.
func enqueueFlowCentralIntent(ctx context.Context, executor sqlContextExecutor, taskID string, intent string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, errorValue := executor.ExecContext(ctx, `
	INSERT INTO flow_central_outbox (task_id, intent, attempt_count, last_error, created_at, updated_at, last_attempted_at)
	VALUES (?, ?, 0, '', ?, ?, '')
	ON CONFLICT(task_id) DO UPDATE SET intent = excluded.intent, updated_at = excluded.updated_at`,
		taskID,
		intent,
		now,
		now,
	)
	return errorValue
}

func (service *Service) readFlowCentralOutbox(ctx context.Context, limit int) ([]flowCentralOutboxEntry, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
	SELECT task_id, intent, attempt_count, last_error
	FROM flow_central_outbox
	ORDER BY created_at
	LIMIT ?`, limit)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	entries := []flowCentralOutboxEntry{}
	for rows.Next() {
		var entry flowCentralOutboxEntry
		if errorValue := rows.Scan(&entry.TaskID, &entry.Intent, &entry.AttemptCount, &entry.LastError); errorValue != nil {
			return nil, errorValue
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (service *Service) markFlowCentralOutboxAttempt(ctx context.Context, taskID string, failure error) error {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, errorValue = database.ExecContext(ctx, `
	UPDATE flow_central_outbox
	SET attempt_count = attempt_count + 1, last_error = ?, updated_at = ?, last_attempted_at = ?
	WHERE task_id = ?`,
		strings.TrimSpace(failure.Error()),
		now,
		now,
		taskID,
	)
	return errorValue
}

func (service *Service) deleteFlowCentralOutbox(ctx context.Context, taskID string) error {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, "DELETE FROM flow_central_outbox WHERE task_id = ?", taskID)
	return errorValue
}
