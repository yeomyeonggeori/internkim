package admind

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"time"
)

func (service *Service) acceptCalendarRemoteDeletion(ctx context.Context, row calendarOutboxRow, snapshotEvent calendarEvent) (bool, error) {
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return false, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return false, errorValue
	}
	deletedAt := time.Now().UTC().Format(time.RFC3339Nano)
	accepted, errorValue := service.acceptCalendarRemoteDeletionWithTransaction(ctx, transaction, row, snapshotEvent, deletedAt)
	if errorValue != nil || !accepted {
		_ = transaction.Rollback()
		return false, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return false, errorValue
	}
	service.runCalendarStoreSideEffectUnlocked(func() {
		if errorValue := service.cancelCalendarNotifications(ctx, snapshotEvent.ID); errorValue != nil {
			slog.WarnContext(ctx, "calendar notification cancel failed", "event_id", snapshotEvent.ID, "error", errorValue)
		}
		service.applyCalendarMattermostProjectionByID(ctx, snapshotEvent.ID)
	})
	return true, nil
}

func (service *Service) acceptCalendarRemoteDeletionWithTransaction(ctx context.Context, transaction *sql.Tx, row calendarOutboxRow, snapshotEvent calendarEvent, deletedAt string) (bool, error) {
	result, errorValue := transaction.ExecContext(ctx, `
UPDATE calendar_events
SET deleted_at = ?, updated_at = ?
WHERE id = ? AND updated_at = ? AND deleted_at = ''`,
		strings.TrimSpace(deletedAt),
		strings.TrimSpace(deletedAt),
		strings.TrimSpace(snapshotEvent.ID),
		strings.TrimSpace(snapshotEvent.UpdatedAt),
	)
	if errorValue != nil {
		return false, errorValue
	}
	affectedRows, errorValue := result.RowsAffected()
	if errorValue != nil || affectedRows == 0 {
		return false, errorValue
	}
	if errorValue := replaceCalendarMutationOrigin(ctx, transaction, snapshotEvent.ID, deletedAt, nil); errorValue != nil {
		return false, errorValue
	}
	if errorValue := enqueueCalendarChannelProjection(ctx, transaction, snapshotEvent.ID); errorValue != nil {
		return false, errorValue
	}
	if errorValue := service.invalidateCalendarEventWindowCache(ctx, transaction, snapshotEvent); errorValue != nil {
		return false, errorValue
	}
	if errorValue := deleteCalendarOutboxBatchWithRunner(ctx, transaction, row); errorValue != nil {
		return false, errorValue
	}
	if errorValue := deleteCalendarPushObservationFenceWithRunner(ctx, transaction, row.AccountID, row.TargetCalendarURL, row.EventUID); errorValue != nil {
		return false, errorValue
	}
	return true, nil
}
