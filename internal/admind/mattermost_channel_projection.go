package admind

import (
	"context"
	"database/sql"
	"log"
	"strings"
	"time"
)

type sqlContextExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func enqueueFlowChannelProjection(ctx context.Context, executor sqlContextExecutor, taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, errorValue := executor.ExecContext(ctx, `
	INSERT INTO flow_channel_outbox (task_id, attempt_count, last_error, created_at, updated_at, last_attempted_at)
	VALUES (?, 0, '', ?, ?, '')
	ON CONFLICT(task_id) DO UPDATE SET updated_at = excluded.updated_at`,
		taskID,
		now,
		now,
	)
	return errorValue
}

func enqueueCalendarChannelProjection(ctx context.Context, executor sqlContextExecutor, eventID string) error {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, errorValue := executor.ExecContext(ctx, `
	INSERT INTO calendar_channel_outbox (event_id, generation, lease_owner, lease_generation, attempt_count, last_error, created_at, updated_at, last_attempted_at)
	VALUES (?, 1, '', 0, 0, '', ?, ?, '')
	ON CONFLICT(event_id) DO UPDATE SET generation = calendar_channel_outbox.generation + 1, updated_at = excluded.updated_at`,
		eventID,
		now,
		now,
	)
	return errorValue
}

func (service *Service) drainMattermostManagedChannelProjections(ctx context.Context) {
	service.drainFlowMattermostProjectionOutbox(ctx)
	service.drainCalendarMattermostProjectionOutbox(ctx)
}

func (service *Service) drainFlowMattermostProjectionOutbox(ctx context.Context) {
	taskIDs, errorValue := service.pendingFlowMattermostProjectionTaskIDs(ctx)
	if errorValue != nil {
		log.Printf("Flow Mattermost projection outbox read failed: %v", errorValue)
		return
	}
	for _, taskID := range taskIDs {
		if errorValue := service.applyFlowMattermostProjectionByID(ctx, taskID); errorValue != nil {
			log.Printf("Flow Mattermost projection failed for %s: %v", taskID, errorValue)
		}
	}
}

func (service *Service) drainCalendarMattermostProjectionOutbox(ctx context.Context) {
	eventIDs, errorValue := service.pendingCalendarMattermostProjectionEventIDs(ctx)
	if errorValue != nil {
		log.Printf("calendar Mattermost projection outbox read failed: %v", errorValue)
		return
	}
	for _, eventID := range eventIDs {
		if errorValue := service.applyCalendarMattermostProjectionByID(ctx, eventID); errorValue != nil {
			log.Printf("calendar Mattermost projection failed for %s: %v", eventID, errorValue)
		}
	}
}

func (service *Service) pendingFlowMattermostProjectionTaskIDs(ctx context.Context) ([]string, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, "SELECT task_id FROM flow_channel_outbox ORDER BY updated_at, task_id")
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	var taskIDs []string
	for rows.Next() {
		var taskID string
		if errorValue := rows.Scan(&taskID); errorValue != nil {
			return nil, errorValue
		}
		taskIDs = append(taskIDs, strings.TrimSpace(taskID))
	}
	return uniqueNonEmpty(taskIDs), rows.Err()
}

func (service *Service) pendingCalendarMattermostProjectionEventIDs(ctx context.Context) ([]string, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, "SELECT event_id FROM calendar_channel_outbox ORDER BY updated_at, event_id")
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	var eventIDs []string
	for rows.Next() {
		var eventID string
		if errorValue := rows.Scan(&eventID); errorValue != nil {
			return nil, errorValue
		}
		eventIDs = append(eventIDs, strings.TrimSpace(eventID))
	}
	return uniqueNonEmpty(eventIDs), rows.Err()
}

func (service *Service) applyFlowMattermostProjection(ctx context.Context, task flowTask) flowTask {
	nextTask, errorValue := service.trySyncFlowMattermostNotification(ctx, task)
	if errorValue != nil {
		_ = service.markFlowMattermostProjectionAttempt(ctx, task.ID, errorValue)
		return task
	}
	_ = service.deleteFlowMattermostProjectionOutbox(ctx, task.ID)
	return nextTask
}

func (service *Service) applyFlowMattermostProjectionByID(ctx context.Context, taskID string) error {
	task, found, errorValue := service.readFlowTaskByID(ctx, taskID)
	if errorValue != nil {
		return errorValue
	}
	if !found {
		return service.deleteFlowMattermostProjectionOutbox(ctx, taskID)
	}
	nextTask, errorValue := service.trySyncFlowMattermostNotification(ctx, task)
	if errorValue != nil {
		if markError := service.markFlowMattermostProjectionAttempt(ctx, taskID, errorValue); markError != nil {
			return markError
		}
		return errorValue
	}
	if nextTask.MattermostPostID != task.MattermostPostID {
		task = nextTask
	}
	return service.deleteFlowMattermostProjectionOutbox(ctx, task.ID)
}

func (service *Service) applyCalendarMattermostProjection(ctx context.Context, event calendarEvent) calendarEvent {
	if errorValue := service.applyCalendarMattermostProjectionByID(ctx, event.ID); errorValue != nil {
		return event
	}
	nextEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil || !found {
		return event
	}
	return nextEvent
}

func (service *Service) applyCalendarMattermostProjectionByID(ctx context.Context, eventID string) error {
	lease, acquired, errorValue := service.acquireCalendarProjectionLease(ctx, eventID)
	if errorValue != nil || !acquired {
		return errorValue
	}
	for {
		errorValue = service.applyCalendarMattermostProjectionGeneration(ctx, eventID)
		if errorValue != nil {
			nextLease, hasNextGeneration, releaseError := service.releaseCalendarProjectionLeaseAfterFailure(ctx, lease, errorValue)
			if releaseError != nil {
				return releaseError
			}
			if !hasNextGeneration {
				return errorValue
			}
			lease = nextLease
			continue
		}
		nextLease, hasNextGeneration, errorValue := service.completeCalendarProjectionGeneration(ctx, lease)
		if errorValue != nil {
			return errorValue
		}
		if !hasNextGeneration {
			return nil
		}
		lease = nextLease
	}
}

func (service *Service) applyCalendarMattermostProjectionGeneration(ctx context.Context, eventID string) error {
	projection, found, errorValue := service.readCalendarEventProjectionByID(ctx, eventID)
	if errorValue != nil {
		return errorValue
	}
	if !found {
		return nil
	}
	if projection.IsDeleted {
		if errorValue := service.tryDeleteCalendarMattermostLog(ctx, projection.Event); errorValue != nil {
			return errorValue
		}
		if errorValue := service.updateCalendarEventMattermostPostID(ctx, projection.Event.ID, ""); errorValue != nil {
			return errorValue
		}
		return nil
	}
	_, errorValue = service.trySyncCalendarMattermostLog(ctx, projection.Event)
	if errorValue != nil {
		return errorValue
	}
	return nil
}

func (service *Service) markFlowMattermostProjectionAttempt(ctx context.Context, taskID string, cause error) error {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `
	UPDATE flow_channel_outbox
	SET attempt_count = attempt_count + 1, last_error = ?, last_attempted_at = ?
	WHERE task_id = ?`,
		cause.Error(),
		time.Now().UTC().Format(time.RFC3339Nano),
		strings.TrimSpace(taskID),
	)
	return errorValue
}

func (service *Service) markCalendarMattermostProjectionAttempt(ctx context.Context, eventID string, cause error) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `
	UPDATE calendar_channel_outbox
	SET attempt_count = attempt_count + 1, last_error = ?, last_attempted_at = ?
	WHERE event_id = ?`,
		cause.Error(),
		time.Now().UTC().Format(time.RFC3339Nano),
		strings.TrimSpace(eventID),
	)
	return errorValue
}

func (service *Service) deleteFlowMattermostProjectionOutbox(ctx context.Context, taskID string) error {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, "DELETE FROM flow_channel_outbox WHERE task_id = ?", strings.TrimSpace(taskID))
	return errorValue
}

func (service *Service) deleteCalendarMattermostProjectionOutbox(ctx context.Context, eventID string) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, "DELETE FROM calendar_channel_outbox WHERE event_id = ?", strings.TrimSpace(eventID))
	return errorValue
}
