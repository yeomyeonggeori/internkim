package admind

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

func (service *Service) applyCalendarDeleteConflictResolution(ctx context.Context, account remoteCalendarAccount, row calendarOutboxRow, remoteEvent calendarEvent, observedAt time.Time) (bool, error) {
	reservedObservedAt, errorValue := service.reserveCalendarConflictCandidateTime(ctx, observedAt)
	if errorValue != nil {
		return false, errorValue
	}
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
	isActiveTarget, errorValue := service.calendarOutboxTargetIsActive(ctx, row)
	if errorValue != nil || !isActiveTarget {
		return false, errorValue
	}
	projection, found, errorValue := service.readCalendarEventProjectionByID(ctx, row.EventID)
	if errorValue != nil || !found {
		return false, errorValue
	}
	if !projection.IsDeleted {
		currentEvent := projection.Event
		currentEvent.RemoteSource = remoteEvent.RemoteSource
		currentEvent.RemoteHref = remoteEvent.RemoteHref
		currentEvent.RemoteETag = remoteEvent.RemoteETag
		currentEvent.RawICS = remoteEvent.RawICS
		return false, service.persistCalendarDeleteConflictResolutionLocked(ctx, account, row, currentEvent, remoteEvent, reservedObservedAt, true)
	}
	localDeletedAt := parseCalendarConflictTime(projection.DeletedAt)
	remoteModifiedAt := parseCalendarConflictTime(remoteEvent.RemoteModifiedAt)
	if resolveCalendarLocalDeletion(localDeletedAt, remoteModifiedAt) == calendarConflictWinnerLocal {
		return true, nil
	}
	restoredEvent := restoreCalendarRemoteEventFromProjection(remoteEvent, projection.Event)
	return false, service.persistCalendarDeleteConflictResolutionLocked(ctx, account, row, restoredEvent, remoteEvent, reservedObservedAt, false)
}

func (service *Service) persistCalendarDeleteConflictResolutionLocked(ctx context.Context, account remoteCalendarAccount, row calendarOutboxRow, event calendarEvent, observedEvent calendarEvent, observedAt time.Time, retainPendingLocalWrite bool) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	remoteState, errorValue := service.persistObservedCalendarRemoteEventStateWithTransaction(ctx, transaction, account.ID, row.TargetCalendarURL, observedEvent, observedAt)
	if errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	updatedAt := remoteState.LastSeenAt
	if errorValue := persistCalendarDeleteConflictResolutionWithTransaction(ctx, transaction, account.ID, row, event, updatedAt, retainPendingLocalWrite); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return errorValue
	}
	event.UpdatedAt = updatedAt
	service.runCalendarStoreSideEffectUnlocked(func() {
		service.finishCalendarEventPersistence(ctx, event)
	})
	return nil
}

func persistCalendarDeleteConflictResolutionWithTransaction(ctx context.Context, transaction *sql.Tx, accountID string, row calendarOutboxRow, event calendarEvent, updatedAt string, retainPendingLocalWrite bool) error {
	if errorValue := persistCalendarEventWithTransaction(ctx, transaction, event, updatedAt); errorValue != nil {
		return errorValue
	}
	if retainPendingLocalWrite {
		if errorValue := updatePendingCalendarPutRemoteStateWithRunner(ctx, transaction, accountID, row.TargetCalendarURL, event.UID, event.RemoteHref, event.RemoteETag); errorValue != nil {
			return errorValue
		}
	}
	return deleteCalendarOutboxBatchWithRunner(ctx, transaction, row)
}

func updatePendingCalendarPutRemoteStateWithRunner(ctx context.Context, queryRunner calendarSQLRunner, accountID string, targetCalendarURL string, eventUID string, remoteHref string, remoteETag string) error {
	_, errorValue := queryRunner.ExecContext(ctx,
		`UPDATE calendar_outbox SET if_match_etag = ?, remote_href = ? WHERE account_id = ? AND target_calendar_url = ? AND event_uid = ? AND operation = ? AND status = ?`,
		strings.TrimSpace(remoteETag),
		strings.TrimSpace(remoteHref),
		strings.TrimSpace(accountID),
		normalizeCalendarOutboxTargetURL(targetCalendarURL),
		strings.TrimSpace(eventUID),
		calendarOutboxOperationPut,
		calendarOutboxStatusPending,
	)
	return errorValue
}
