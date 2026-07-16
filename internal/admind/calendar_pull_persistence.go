package admind

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

func (service *Service) writePulledCalendarEventLocked(ctx context.Context, event calendarEvent, deferredProjections *calendarPullDeferredProjectionQueue) error {
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
	if errorValue := persistCalendarEventWithTransaction(ctx, transaction, event, updatedAt); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return errorValue
	}
	deferredProjections.add(event.ID)
	return nil
}

func (service *Service) writePulledCalendarEventAndRetainOutboxLocked(ctx context.Context, accountID string, targetCalendarURL string, event calendarEvent, retainedFields []string, deferredProjections *calendarPullDeferredProjectionQueue) error {
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
	if errorValue := persistCalendarEventWithTransaction(ctx, transaction, event, updatedAt); errorValue != nil {
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

func (service *Service) writePulledCalendarEventAndDeleteOutboxLocked(ctx context.Context, accountID string, targetCalendarURL string, event calendarEvent, deferredProjections *calendarPullDeferredProjectionQueue) error {
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
	if errorValue := persistCalendarEventWithTransaction(ctx, transaction, event, updatedAt); errorValue != nil {
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

func (service *Service) softDeleteCalendarEventForPullLocked(ctx context.Context, eventID string, deferredProjections *calendarPullDeferredProjectionQueue) error {
	event, found, errorValue := service.readCalendarEventByID(ctx, eventID)
	if errorValue != nil {
		return errorValue
	}
	if !found {
		return sql.ErrNoRows
	}
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
	result, errorValue := transaction.ExecContext(ctx, "UPDATE calendar_events SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at = ''", deletedAt, deletedAt, strings.TrimSpace(eventID))
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
	if errorValue := enqueueCalendarChannelProjection(ctx, transaction, eventID); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return errorValue
	}
	deferredProjections.add(event.ID)
	return nil
}
