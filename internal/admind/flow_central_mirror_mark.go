package admind

import (
	"context"
	"database/sql"
	"strings"
)

// How far into the board's changes this device has read. Kept so a restart asks
// for what it has not seen rather than for everything.
func ensureFlowMirrorMarkTable(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
	CREATE TABLE IF NOT EXISTS flow_central_mirror_mark (
		name TEXT PRIMARY KEY,
		seen_up_to TEXT NOT NULL
	)`)
	return errorValue
}

func (service *Service) readFlowMirrorMark(ctx context.Context) (string, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return "", errorValue
	}
	defer database.Close()
	seenUpTo := ""
	errorValue = database.QueryRowContext(ctx,
		"SELECT seen_up_to FROM flow_central_mirror_mark WHERE name = ?", flowMirrorMark).Scan(&seenUpTo)
	if errorValue == sql.ErrNoRows {
		return "", nil
	}
	return seenUpTo, errorValue
}

func (service *Service) writeFlowMirrorMark(ctx context.Context, seenUpTo string) error {
	if strings.TrimSpace(seenUpTo) == "" {
		return nil
	}
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `
	INSERT INTO flow_central_mirror_mark (name, seen_up_to)
	VALUES (?, ?)
	ON CONFLICT(name) DO UPDATE SET seen_up_to = excluded.seen_up_to`,
		flowMirrorMark,
		seenUpTo,
	)
	return errorValue
}

func (service *Service) deviceTaskIDForCentralTask(ctx context.Context, centralTaskID string) (string, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return "", errorValue
	}
	defer database.Close()
	taskID := ""
	errorValue = database.QueryRowContext(ctx,
		"SELECT task_id FROM flow_central_identity WHERE central_task_id = ?", centralTaskID).Scan(&taskID)
	if errorValue == sql.ErrNoRows {
		return "", nil
	}
	return taskID, errorValue
}

// The row carries when it was last written; flowTask does not, and the drain
// needs it only to decide which side of an offline edit wins.
func (service *Service) flowTaskWrittenAt(ctx context.Context, taskID string) (string, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return "", errorValue
	}
	defer database.Close()
	writtenAt := ""
	errorValue = database.QueryRowContext(ctx,
		"SELECT updated_at FROM flow_tasks WHERE id = ?", taskID).Scan(&writtenAt)
	if errorValue == sql.ErrNoRows {
		return "", nil
	}
	return writtenAt, errorValue
}
