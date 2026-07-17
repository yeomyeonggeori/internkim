package admind

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

func (service *Service) persistCalendarEventDeletionWithTransaction(
	ctx context.Context,
	transaction *sql.Tx,
	event calendarEvent,
	source string,
	candidateDeletedAt time.Time,
	outboxRow calendarOutboxRow,
	shouldSignalSync bool,
) (string, error) {
	actionAt := candidateDeletedAt.UTC()
	storageRevision := actionAt
	if source == calendarSourceLocal {
		logicalTime, errorValue := service.allocateCalendarConflictTime(ctx, transaction, event.UID, candidateDeletedAt)
		if errorValue != nil {
			return "", errorValue
		}
		actionAt = logicalTime
		storageRevision = logicalTime
	}
	return service.persistCalendarEventDeletionAtTimesWithTransaction(ctx, transaction, event, source, actionAt, storageRevision, outboxRow, shouldSignalSync)
}

func (service *Service) persistCalendarDeleteIntentEventDeletionWithTransaction(
	ctx context.Context,
	transaction *sql.Tx,
	event calendarEvent,
	requestedAt time.Time,
	outboxRow calendarOutboxRow,
	shouldSignalSync bool,
) (string, error) {
	storageRevision, errorValue := service.allocateCalendarConflictTime(ctx, transaction, event.UID, requestedAt)
	if errorValue != nil {
		return "", errorValue
	}
	return service.persistCalendarEventDeletionAtTimesWithTransaction(ctx, transaction, event, calendarSourceLocal, requestedAt.UTC(), storageRevision, outboxRow, shouldSignalSync)
}

func (service *Service) persistCalendarEventDeletionAtTimesWithTransaction(
	ctx context.Context,
	transaction *sql.Tx,
	event calendarEvent,
	source string,
	actionAt time.Time,
	storageRevision time.Time,
	outboxRow calendarOutboxRow,
	shouldSignalSync bool,
) (string, error) {
	deletedAt := actionAt.UTC().Format(time.RFC3339Nano)
	updatedAt := storageRevision.UTC().Format(time.RFC3339Nano)
	if source == calendarSourceLocal {
		if errorValue := persistCalendarEventFieldClocks(ctx, transaction, event.UID, []string{calendarEventDeletionClockField}, deletedAt); errorValue != nil {
			return "", errorValue
		}
	}
	result, errorValue := transaction.ExecContext(ctx, `
UPDATE calendar_events
SET deleted_at = ?, updated_at = ?
WHERE id = ? AND updated_at = ? AND deleted_at = ''`,
		deletedAt,
		updatedAt,
		strings.TrimSpace(event.ID),
		strings.TrimSpace(event.UpdatedAt),
	)
	if errorValue != nil {
		return "", errorValue
	}
	affectedRows, errorValue := result.RowsAffected()
	if errorValue != nil {
		return "", errorValue
	}
	if affectedRows == 0 {
		return "", sql.ErrNoRows
	}
	if errorValue := replaceCalendarMutationOrigin(ctx, transaction, event.ID, updatedAt, nil); errorValue != nil {
		return "", errorValue
	}
	if errorValue := enqueueCalendarChannelProjection(ctx, transaction, event.ID); errorValue != nil {
		return "", errorValue
	}
	if errorValue := service.invalidateCalendarEventWindowCache(ctx, transaction, event); errorValue != nil {
		return "", errorValue
	}
	if shouldSignalSync {
		if errorValue := enqueueCalendarOutboxWithRunner(ctx, transaction, outboxRow, deletedAt); errorValue != nil {
			return "", errorValue
		}
	}
	return deletedAt, nil
}
