package admind

import (
	"context"
	"database/sql"
	"fmt"
)

const flowTasksMadeFromCalendarEvents = `
SELECT id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, size, status, status_rank, start_date, end_date, mattermost_post_id, calendar_event_id, created_at
FROM flow_tasks
WHERE id IN (SELECT uid FROM calendar_events)`

// The flow mirror once read the whole of public.task, where calendar events sit
// beside tasks carrying the device's *event* uid as their mirror, and made a flow
// task out of every meeting. A flow task the device wrote carries a stableFlowID;
// one whose identifier is an event uid this device already holds can only be that
// mistake. Removing it must not queue a central delete, because the row that
// delete would remove is the calendar event itself.
func (service *Service) removeFlowTasksMadeFromCalendarEvents(ctx context.Context) (int, error) {
	if errorValue := service.ensureCalendarTablesExist(ctx); errorValue != nil {
		return 0, errorValue
	}
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return 0, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return 0, errorValue
	}
	removed, errorValue := removeFlowTasksMadeFromCalendarEventsInTransaction(ctx, transaction)
	if errorValue != nil {
		_ = transaction.Rollback()
		return 0, errorValue
	}
	return removed, transaction.Commit()
}

func removeFlowTasksMadeFromCalendarEventsInTransaction(ctx context.Context, transaction *sql.Tx) (int, error) {
	tasks, errorValue := readFlowTasksMadeFromCalendarEvents(ctx, transaction)
	if errorValue != nil || len(tasks) == 0 {
		return 0, errorValue
	}
	const selection = "SELECT id FROM flow_tasks WHERE id IN (SELECT uid FROM calendar_events)"
	for _, statement := range []string{
		"DELETE FROM flow_central_identity WHERE task_id IN (" + selection + ")",
		"DELETE FROM flow_channel_outbox WHERE task_id IN (" + selection + ")",
		"DELETE FROM flow_central_outbox WHERE task_id IN (" + selection + ")",
		"DELETE FROM flow_tasks WHERE id IN (SELECT uid FROM calendar_events)",
	} {
		if _, errorValue := transaction.ExecContext(ctx, statement); errorValue != nil {
			return 0, errorValue
		}
	}
	if errorValue := incrementFlowSummarySourceRevisions(ctx, transaction, flowTasksSummarySourceKeys(tasks)); errorValue != nil {
		return 0, errorValue
	}
	return len(tasks), nil
}

func readFlowTasksMadeFromCalendarEvents(ctx context.Context, transaction *sql.Tx) ([]flowTask, error) {
	rows, errorValue := transaction.QueryContext(ctx, flowTasksMadeFromCalendarEvents)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	tasks := []flowTask{}
	for rows.Next() {
		task, errorValue := scanFlowTask(rows)
		if errorValue != nil {
			return nil, errorValue
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (service *Service) reportFlowTasksMadeFromCalendarEvents(ctx context.Context) sshRecoveryCommandResult {
	removed, errorValue := service.removeFlowTasksMadeFromCalendarEvents(ctx)
	if errorValue != nil {
		return sshRecoveryCommandResult{
			Name:   "drop flow tasks made from calendar events",
			Status: "error",
			Output: errorValue.Error(),
		}
	}
	return sshRecoveryCommandResult{
		Name:   "drop flow tasks made from calendar events",
		Status: "ok",
		Output: fmt.Sprintf("%d removed", removed),
	}
}

// Both stores live in one SQLite file under two schema-ensure functions, and the
// join below reads the calendar's table through the flow handle.
func (service *Service) ensureCalendarTablesExist(ctx context.Context) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	return database.Close()
}
