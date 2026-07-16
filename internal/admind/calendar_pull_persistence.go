package admind

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

func (service *Service) writePulledCalendarEventLocked(ctx context.Context, accountID string, targetCalendarURL string, event calendarEvent, remoteWinningFields []string, deferredProjections *calendarPullDeferredProjectionQueue) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	updatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	if errorValue := service.persistCalendarEventMutationWithTransaction(ctx, transaction, event, updatedAt); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := persistPulledCalendarFieldClocks(ctx, transaction, accountID, targetCalendarURL, event, remoteWinningFields, updatedAt); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return errorValue
	}
	deferredProjections.add(event.ID)
	return nil
}

func (service *Service) writePulledCalendarEventAndRetainOutboxLocked(ctx context.Context, accountID string, targetCalendarURL string, event calendarEvent, retainedFields []string, remoteWinningFields []string, deferredProjections *calendarPullDeferredProjectionQueue) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	updatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	if errorValue := service.persistCalendarEventMutationWithTransaction(ctx, transaction, event, updatedAt); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := persistPulledCalendarFieldClocks(ctx, transaction, accountID, targetCalendarURL, event, remoteWinningFields, updatedAt); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := retainPendingCalendarOutboxFieldsWithRunner(ctx, transaction, accountID, targetCalendarURL, event.UID, retainedFields, event.RemoteHref, event.RemoteETag); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return errorValue
	}
	event.UpdatedAt = updatedAt
	deferredProjections.add(event.ID)
	return nil
}

func (service *Service) writePulledCalendarEventAndDeleteOutboxLocked(ctx context.Context, accountID string, targetCalendarURL string, event calendarEvent, remoteWinningFields []string, deferredProjections *calendarPullDeferredProjectionQueue) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	updatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	if errorValue := service.persistCalendarEventMutationWithTransaction(ctx, transaction, event, updatedAt); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := persistPulledCalendarFieldClocks(ctx, transaction, accountID, targetCalendarURL, event, remoteWinningFields, updatedAt); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := deleteCalendarOutboxForEventUIDWithRunner(ctx, transaction, accountID, targetCalendarURL, event.UID); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return errorValue
	}
	event.UpdatedAt = updatedAt
	deferredProjections.add(event.ID)
	return nil
}

func persistPulledCalendarFieldClocks(ctx context.Context, transaction *sql.Tx, accountID string, targetCalendarURL string, event calendarEvent, remoteWinningFields []string, fallbackChangedAt string) error {
	changedAt := parseCalendarConflictTime(event.RemoteModifiedAt)
	if changedAt.IsZero() {
		changedAt = parseCalendarConflictTime(fallbackChangedAt)
	}
	if changedAt.IsZero() || len(remoteWinningFields) == 0 {
		return nil
	}
	existingAcknowledgements, errorValue := readCalendarTargetFieldAcknowledgements(ctx, transaction, accountID, targetCalendarURL, event.UID)
	if errorValue != nil {
		return errorValue
	}
	acknowledgements, errorValue := persistAdvancedCalendarEventFieldClocks(ctx, transaction, event.UID, remoteWinningFields, changedAt, existingAcknowledgements)
	if errorValue != nil {
		return errorValue
	}
	return persistCalendarTargetFieldAcknowledgements(ctx, transaction, accountID, targetCalendarURL, event.UID, acknowledgements)
}

func (service *Service) acceptMissingCalendarRemoteDeletionLocked(ctx context.Context, accountID string, targetCalendarURL string, event calendarEvent, deferredProjections *calendarPullDeferredProjectionQueue) error {
	deletedAt := time.Now().UTC().Format(time.RFC3339Nano)
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	result, errorValue := transaction.ExecContext(ctx, `
UPDATE calendar_events
SET deleted_at = ?, updated_at = ?
WHERE id = ? AND updated_at = ? AND deleted_at = ''`, deletedAt, deletedAt, strings.TrimSpace(event.ID), strings.TrimSpace(event.UpdatedAt))
	if errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	affectedRows, errorValue := result.RowsAffected()
	if errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if affectedRows == 0 {
		_ = transaction.Rollback()
		return sql.ErrNoRows
	}
	if errorValue := enqueueCalendarChannelProjection(ctx, transaction, event.ID); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := service.invalidateCalendarEventWindowCache(ctx, transaction, event); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := deleteCalendarOutboxForEventUIDWithRunner(ctx, transaction, accountID, targetCalendarURL, event.UID); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := deleteCalendarPushObservationFenceWithRunner(ctx, transaction, accountID, targetCalendarURL, event.UID); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return errorValue
	}
	deferredProjections.add(event.ID)
	return nil
}
