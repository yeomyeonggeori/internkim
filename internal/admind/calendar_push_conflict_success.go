package admind

import (
	"context"
	"time"
)

func (service *Service) applyCalendarPushConflictSuccess(ctx context.Context, row calendarOutboxRow, snapshotEvent calendarEvent, pushedEvent calendarEvent, remoteEvent calendarEvent, remoteWinningFields []string, objectPath string, newETag string, rawICS []byte) error {
	fallbackChangedAt := time.Time{}
	if len(remoteWinningFields) > 0 && parseCalendarConflictTime(remoteEvent.RemoteModifiedAt).IsZero() {
		var errorValue error
		fallbackChangedAt, errorValue = service.reserveCalendarConflictCandidateTime(ctx, time.Now().UTC())
		if errorValue != nil {
			return errorValue
		}
	}
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
	isActiveTarget, errorValue := service.calendarOutboxTargetIsActive(ctx, row)
	if errorValue != nil || !isActiveTarget {
		return errorValue
	}
	projection, found, errorValue := service.readCalendarEventProjectionByID(ctx, snapshotEvent.ID)
	if errorValue != nil || !found {
		return errorValue
	}
	if projection.IsDeleted {
		return service.persistCalendarPushSuccessForDeletedEvent(ctx, row, projection.Event.ID, snapshotEvent.UID, objectPath, newETag, rawICS)
	}
	persistedEvent, hasConcurrentRevision := calendarPushConflictPersistedEvent(projection.Event, snapshotEvent, pushedEvent, objectPath, newETag, rawICS)
	persistedEvent.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if errorValue := service.persistCalendarPushConflictSuccessLocked(ctx, row, persistedEvent, remoteEvent, remoteWinningFields, fallbackChangedAt, hasConcurrentRevision); errorValue != nil {
		return errorValue
	}
	service.runCalendarStoreSideEffectUnlocked(func() {
		service.finishCalendarEventPersistence(ctx, persistedEvent)
	})
	return nil
}

func (service *Service) persistCalendarPushConflictSuccessLocked(ctx context.Context, row calendarOutboxRow, event calendarEvent, remoteEvent calendarEvent, remoteWinningFields []string, fallbackChangedAt time.Time, hasConcurrentRevision bool) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := service.persistCalendarEventMutationWithTransaction(ctx, transaction, event, event.UpdatedAt); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := service.persistCalendarPushConflictFieldClocksWithTransaction(ctx, transaction, row, remoteEvent, remoteWinningFields, fallbackChangedAt); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if hasConcurrentRevision {
		if errorValue := updatePendingCalendarOutboxRemoteStateWithRunner(ctx, transaction, row.AccountID, row.TargetCalendarURL, event.UID, event.RemoteHref, event.RemoteETag); errorValue != nil {
			_ = transaction.Rollback()
			return errorValue
		}
	}
	return transaction.Commit()
}

func calendarPushConflictPersistedEvent(currentEvent calendarEvent, snapshotEvent calendarEvent, pushedEvent calendarEvent, objectPath string, newETag string, rawICS []byte) (calendarEvent, bool) {
	pushedEvent.RemoteSource = remoteCalendarProviderGoogle
	pushedEvent.RemoteHref = objectPath
	pushedEvent.RemoteETag = newETag
	pushedEvent.RawICS = string(rawICS)
	if calendarEventRevisionMatches(currentEvent, snapshotEvent) {
		return pushedEvent, false
	}
	currentEvent.RemoteSource = pushedEvent.RemoteSource
	currentEvent.RemoteHref = pushedEvent.RemoteHref
	currentEvent.RemoteETag = pushedEvent.RemoteETag
	currentEvent.RawICS = pushedEvent.RawICS
	return currentEvent, true
}
