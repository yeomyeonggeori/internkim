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
	candidateDeletedAt time.Time,
) (string, error) {
	logicalTime, errorValue := service.allocateCalendarConflictTime(ctx, transaction, event.UID, candidateDeletedAt)
	if errorValue != nil {
		return "", errorValue
	}
	return service.persistCalendarEventDeletionAtTimesWithTransaction(ctx, transaction, event, logicalTime, logicalTime)
}

func (service *Service) persistCalendarDeleteIntentEventDeletionWithTransaction(
	ctx context.Context,
	transaction *sql.Tx,
	event calendarEvent,
	requestedAt time.Time,
) (string, error) {
	storageRevision, errorValue := service.allocateCalendarConflictTime(ctx, transaction, event.UID, requestedAt)
	if errorValue != nil {
		return "", errorValue
	}
	return service.persistCalendarEventDeletionAtTimesWithTransaction(ctx, transaction, event, requestedAt.UTC(), storageRevision)
}

func (service *Service) persistCalendarEventDeletionAtTimesWithTransaction(
	ctx context.Context,
	transaction *sql.Tx,
	event calendarEvent,
	actionAt time.Time,
	storageRevision time.Time,
) (string, error) {
	deletedAt := actionAt.UTC().Format(time.RFC3339Nano)
	updatedAt := storageRevision.UTC().Format(time.RFC3339Nano)
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
	if errorValue := service.invalidateCalendarEventWindowCache(ctx, transaction, event); errorValue != nil {
		return "", errorValue
	}
	return deletedAt, nil
}
