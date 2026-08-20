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

// A refusal can be permanent: a task whose owner has no messenger account here
// has nobody to write it as, and asking again every minute neither issues that
// session nor lets the log show anything else. Past this many attempts the entry
// is held back with its last error, and `flow-central-held` is where a person
// reads what the queue is holding.
const flowCentralOutboxAttemptLimit = 12

type flowCentralOutboxEntry struct {
	TaskID          string
	Intent          string
	AttemptCount    int
	LastError       string
	LastAttemptedAt string
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
// the link was down has nothing to send but its removal. Either way the intent is
// new work, so a held-back entry gets its attempts back and drains again.
func enqueueFlowCentralIntent(ctx context.Context, executor sqlContextExecutor, taskID string, intent string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, errorValue := executor.ExecContext(ctx, `
	INSERT INTO flow_central_outbox (task_id, intent, attempt_count, last_error, created_at, updated_at, last_attempted_at)
	VALUES (?, ?, 0, '', ?, ?, '')
	ON CONFLICT(task_id) DO UPDATE SET
		intent = excluded.intent,
		attempt_count = 0,
		last_error = '',
		updated_at = excluded.updated_at`,
		taskID,
		intent,
		now,
		now,
	)
	return errorValue
}

func (service *Service) readFlowCentralOutbox(ctx context.Context, limit int) ([]flowCentralOutboxEntry, error) {
	return service.queryFlowCentralOutbox(ctx, `
	SELECT task_id, intent, attempt_count, last_error, last_attempted_at
	FROM flow_central_outbox
	WHERE attempt_count < ?
	ORDER BY created_at
	LIMIT ?`, flowCentralOutboxAttemptLimit, limit)
}

func (service *Service) readFlowCentralOutboxHeldBack(ctx context.Context) ([]flowCentralOutboxEntry, error) {
	return service.queryFlowCentralOutbox(ctx, `
	SELECT task_id, intent, attempt_count, last_error, last_attempted_at
	FROM flow_central_outbox
	WHERE attempt_count >= ?
	ORDER BY created_at`, flowCentralOutboxAttemptLimit)
}

func (service *Service) queryFlowCentralOutbox(ctx context.Context, statement string, arguments ...any) ([]flowCentralOutboxEntry, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, statement, arguments...)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	entries := []flowCentralOutboxEntry{}
	for rows.Next() {
		var entry flowCentralOutboxEntry
		if errorValue := rows.Scan(&entry.TaskID, &entry.Intent, &entry.AttemptCount, &entry.LastError, &entry.LastAttemptedAt); errorValue != nil {
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

// The device removed the task and the central plane has not been told yet, so
// the row the mirror is reading is one this device already deleted. Writing it
// back would undo the delete a moment before the drain carries it out.
func (service *Service) flowTaskIsQueuedForDeletion(ctx context.Context, taskID string) (bool, error) {
	if strings.TrimSpace(taskID) == "" {
		return false, nil
	}
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return false, errorValue
	}
	defer database.Close()
	intent := ""
	errorValue = database.QueryRowContext(ctx,
		"SELECT intent FROM flow_central_outbox WHERE task_id = ?", taskID).Scan(&intent)
	if errorValue == sql.ErrNoRows {
		return false, nil
	}
	return intent == flowCentralDeleteIntent, errorValue
}
