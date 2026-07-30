package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

func (service *Service) writeFlowTask(ctx context.Context, task flowTask) error {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := writeFlowTaskAndInvalidateSummaryInTransaction(ctx, transaction, task); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

func (service *Service) writeFlowTaskAtStatusEnd(ctx context.Context, task flowTask) (flowTask, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return flowTask{}, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return flowTask{}, errorValue
	}
	statusRank, errorValue := nextFlowTaskStatusRankInTransaction(ctx, transaction, task.Status)
	if errorValue != nil {
		_ = transaction.Rollback()
		return flowTask{}, errorValue
	}
	task.StatusRank = statusRank
	task = flowTaskWithCreatedAt(task)
	if errorValue := writeFlowTaskAndInvalidateSummaryInTransaction(ctx, transaction, task); errorValue != nil {
		_ = transaction.Rollback()
		return flowTask{}, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return flowTask{}, errorValue
	}
	return task, nil
}

func writeFlowTaskAndInvalidateSummaryInTransaction(ctx context.Context, transaction *sql.Tx, task flowTask) error {
	existingTask, found, errorValue := readFlowTaskByIDInTransaction(ctx, transaction, task.ID)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := writeFlowTaskInTransaction(ctx, transaction, task); errorValue != nil {
		return errorValue
	}
	sourceTasks := []flowTask{task}
	if found {
		sourceTasks = append(sourceTasks, existingTask)
	}
	sourceKeys := flowTasksSummarySourceKeys(sourceTasks)
	return incrementFlowSummarySourceRevisions(ctx, transaction, sourceKeys)
}

func flowTaskWithCreatedAt(task flowTask) flowTask {
	if strings.TrimSpace(task.CreatedAt) != "" {
		return task
	}
	task.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	return task
}

func writeFlowTaskInTransaction(ctx context.Context, transaction *sql.Tx, task flowTask) error {
	participantIDs, errorValue := json.Marshal(task.ParticipantIDs)
	if errorValue != nil {
		return errorValue
	}
	participantNames, errorValue := json.Marshal(task.ParticipantNames)
	if errorValue != nil {
		return errorValue
	}
	now := time.Now().UTC().Format(time.RFC3339)
	createdAt := strings.TrimSpace(task.CreatedAt)
	if createdAt == "" {
		createdAt = now
	}
	_, errorValue = transaction.ExecContext(ctx, `
INSERT INTO flow_tasks (
		id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, goal, size, status, status_rank, start_date, end_date, flag, request_reason, decision_reason, mattermost_post_id, calendar_event_id, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	week_code = excluded.week_code,
	owner_id = excluded.owner_id,
	owner_name = excluded.owner_name,
	participant_ids = excluded.participant_ids,
	participant_names = excluded.participant_names,
	business = excluded.business,
	type = excluded.type,
	content = excluded.content,
	goal = excluded.goal,
	size = excluded.size,
	status = excluded.status,
	status_rank = excluded.status_rank,
	start_date = excluded.start_date,
	end_date = excluded.end_date,
	flag = excluded.flag,
	request_reason = excluded.request_reason,
	decision_reason = excluded.decision_reason,
	mattermost_post_id = excluded.mattermost_post_id,
	calendar_event_id = excluded.calendar_event_id,
	updated_at = excluded.updated_at`,
		task.ID,
		task.WeekCode,
		task.OwnerID,
		task.OwnerName,
		string(participantIDs),
		string(participantNames),
		task.Business,
		task.Type,
		task.Content,
		task.Goal,
		task.Size,
		task.Status,
		task.StatusRank,
		task.StartDate,
		task.EndDate,
		task.Flag,
		task.RequestReason,
		task.DecisionReason,
		task.MattermostPostID,
		task.CalendarEventID,
		createdAt,
		now,
	)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := enqueueFlowChannelProjection(ctx, transaction, task.ID); errorValue != nil {
		return errorValue
	}
	return nil
}

func writeFlowTaskStatusRankInTransaction(ctx context.Context, transaction *sql.Tx, task flowTask) error {
	_, errorValue := transaction.ExecContext(ctx, "UPDATE flow_tasks SET status_rank = ? WHERE id = ?", task.StatusRank, task.ID)
	return errorValue
}

func nextFlowTaskStatusRankInTransaction(ctx context.Context, transaction *sql.Tx, status string) (int, error) {
	var rank int
	errorValue := transaction.QueryRowContext(ctx, "SELECT COALESCE(MAX(status_rank), 0) + 1024 FROM flow_tasks WHERE status = ?", cleanFlowStatus(status)).Scan(&rank)
	return rank, errorValue
}

func (service *Service) updateFlowTaskMattermostPostID(ctx context.Context, taskID string, postID string) error {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	existingTask, found, errorValue := readFlowTaskByIDInTransaction(ctx, transaction, taskID)
	if errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	trimmedPostID := strings.TrimSpace(postID)
	if found && existingTask.MattermostPostID == trimmedPostID {
		return transaction.Commit()
	}
	postCreatedAt := ""
	if trimmedPostID != "" {
		postCreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if _, errorValue = transaction.ExecContext(ctx, "UPDATE flow_tasks SET mattermost_post_id = ?, mattermost_post_created_at = ?, updated_at = ? WHERE id = ?", trimmedPostID, postCreatedAt, time.Now().UTC().Format(time.RFC3339), taskID); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if found && existingTask.MattermostPostID != trimmedPostID {
		weekKey, errorValue := flowTaskWeekSummarySourceKey(existingTask)
		if errorValue == nil {
			if errorValue := incrementFlowSummarySourceRevisions(ctx, transaction, []flowSummarySourceKey{weekKey}); errorValue != nil {
				_ = transaction.Rollback()
				return errorValue
			}
		}
	}
	return transaction.Commit()
}

func (service *Service) deleteFlowTaskByID(ctx context.Context, taskID string) error {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	existingTask, found, errorValue := readFlowTaskByIDInTransaction(ctx, transaction, taskID)
	if errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if _, errorValue = transaction.ExecContext(ctx, "DELETE FROM flow_channel_outbox WHERE task_id = ?", taskID); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if _, errorValue = transaction.ExecContext(ctx, "DELETE FROM flow_tasks WHERE id = ?", taskID); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if found {
		sourceKeys := flowTaskSummarySourceKeys(existingTask)
		if errorValue := incrementFlowSummarySourceRevisions(ctx, transaction, sourceKeys); errorValue != nil {
			_ = transaction.Rollback()
			return errorValue
		}
		if errorValue := clearFlowSummaryCacheEntries(ctx, transaction); errorValue != nil {
			_ = transaction.Rollback()
			return errorValue
		}
	}
	return transaction.Commit()
}
