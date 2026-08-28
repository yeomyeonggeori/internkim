package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

func (service *Service) writeTask(ctx context.Context, task Task) error {
	database, errorValue := service.openTaskDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := writeTaskAndInvalidateSummaryInTransaction(ctx, transaction, task); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

// A task the central plane already holds is written without being queued back to
// it. Queueing it would send it straight out again, and the answer would arrive
// as another change, and so on.
func (service *Service) writeMirroredTask(ctx context.Context, task Task) error {
	database, errorValue := service.openTaskDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	existingTask, found, errorValue := readTaskByIDInTransaction(ctx, transaction, task.ID)
	if errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := writeTaskRowInTransaction(ctx, transaction, task); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	sourceTasks := []Task{task}
	if found {
		sourceTasks = append(sourceTasks, existingTask)
	}
	if errorValue := incrementTaskSummarySourceRevisions(ctx, transaction, tasksSummarySourceKeys(sourceTasks)); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

func (service *Service) writeTaskAtStatusEnd(ctx context.Context, task Task) (Task, error) {
	database, errorValue := service.openTaskDatabase(ctx)
	if errorValue != nil {
		return Task{}, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return Task{}, errorValue
	}
	statusRank, errorValue := nextTaskStatusRankInTransaction(ctx, transaction, task.Status)
	if errorValue != nil {
		_ = transaction.Rollback()
		return Task{}, errorValue
	}
	task.StatusRank = statusRank
	task = taskWithCreatedAt(taskWithIdentifier(task))
	if errorValue := writeTaskAndInvalidateSummaryInTransaction(ctx, transaction, task); errorValue != nil {
		_ = transaction.Rollback()
		return Task{}, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return Task{}, errorValue
	}
	return task, nil
}

func writeTaskAndInvalidateSummaryInTransaction(ctx context.Context, transaction *sql.Tx, task Task) error {
	existingTask, found, errorValue := readTaskByIDInTransaction(ctx, transaction, task.ID)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := writeTaskInTransaction(ctx, transaction, task); errorValue != nil {
		return errorValue
	}
	sourceTasks := []Task{task}
	if found {
		sourceTasks = append(sourceTasks, existingTask)
	}
	sourceKeys := tasksSummarySourceKeys(sourceTasks)
	return incrementTaskSummarySourceRevisions(ctx, transaction, sourceKeys)
}

func taskWithIdentifier(task Task) Task {
	if strings.TrimSpace(task.ID) != "" {
		return task
	}
	task.ID = newTaskID(task)
	return task
}

func newTaskID(task Task) string {
	return stableTaskID(task.WeekCode + task.OwnerID + task.Content + time.Now().UTC().Format(time.RFC3339Nano))
}

func taskWithCreatedAt(task Task) Task {
	if strings.TrimSpace(task.CreatedAt) != "" {
		return task
	}
	task.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	return task
}

func writeTaskInTransaction(ctx context.Context, transaction *sql.Tx, task Task) error {
	if errorValue := writeTaskRowInTransaction(ctx, transaction, task); errorValue != nil {
		return errorValue
	}
	if errorValue := enqueueTaskChannelProjection(ctx, transaction, task.ID); errorValue != nil {
		return errorValue
	}
	return nil
}

func writeTaskRowInTransaction(ctx context.Context, transaction *sql.Tx, task Task) error {
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
		id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, size, status, status_rank, start_date, end_date, mattermost_post_id, calendar_event_id, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	week_code = excluded.week_code,
	owner_id = excluded.owner_id,
	owner_name = excluded.owner_name,
	participant_ids = excluded.participant_ids,
	participant_names = excluded.participant_names,
	business = excluded.business,
	type = excluded.type,
	content = excluded.content,
	size = excluded.size,
	status = excluded.status,
	status_rank = excluded.status_rank,
	start_date = excluded.start_date,
	end_date = excluded.end_date,
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
		task.Size,
		task.Status,
		task.StatusRank,
		task.StartDate,
		task.EndDate,
		task.MattermostPostID,
		task.CalendarEventID,
		createdAt,
		now,
	)
	if errorValue != nil {
		return errorValue
	}
	return nil
}

func writeTaskStatusRankInTransaction(ctx context.Context, transaction *sql.Tx, task Task) error {
	_, errorValue := transaction.ExecContext(ctx, "UPDATE flow_tasks SET status_rank = ? WHERE id = ?", task.StatusRank, task.ID)
	return errorValue
}

func nextTaskStatusRankInTransaction(ctx context.Context, transaction *sql.Tx, status string) (int, error) {
	var rank int
	errorValue := transaction.QueryRowContext(ctx, "SELECT COALESCE(MAX(status_rank), 0) + 1024 FROM flow_tasks WHERE status = ?", cleanTaskStatus(status)).Scan(&rank)
	return rank, errorValue
}

func (service *Service) updateTaskMattermostPostID(ctx context.Context, taskID string, postID string) error {
	database, errorValue := service.openTaskDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	existingTask, found, errorValue := readTaskByIDInTransaction(ctx, transaction, taskID)
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
		weekKey, errorValue := taskWeekSummarySourceKey(existingTask)
		if errorValue == nil {
			if errorValue := incrementTaskSummarySourceRevisions(ctx, transaction, []taskSummarySourceKey{weekKey}); errorValue != nil {
				_ = transaction.Rollback()
				return errorValue
			}
		}
	}
	return transaction.Commit()
}

func (service *Service) deleteTaskByID(ctx context.Context, taskID string) error {
	database, errorValue := service.openTaskDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	existingTask, found, errorValue := readTaskByIDInTransaction(ctx, transaction, taskID)
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
		sourceKeys := taskSummarySourceKeys(existingTask)
		if errorValue := incrementTaskSummarySourceRevisions(ctx, transaction, sourceKeys); errorValue != nil {
			_ = transaction.Rollback()
			return errorValue
		}
		if errorValue := clearTaskSummaryCacheEntries(ctx, transaction); errorValue != nil {
			_ = transaction.Rollback()
			return errorValue
		}
	}
	return transaction.Commit()
}
