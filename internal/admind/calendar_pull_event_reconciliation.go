package admind

import (
	"context"
	"fmt"
)

func (service *Service) applyPulledRemoteEventLocked(ctx context.Context, account remoteCalendarAccount, previousEvent calendarEvent, hasPreviousEvent bool, remoteEvent calendarEvent, deferredProjections *calendarPullDeferredProjectionQueue) error {
	targetCalendarURL := activeRemoteCalendarTarget(account).CalendarURL
	currentEvent := previousEvent
	hasCurrentEvent := hasPreviousEvent
	refreshedEvent, refreshedFound, errorValue := service.readCalendarEventByUID(ctx, remoteEvent.UID)
	if errorValue != nil {
		return errorValue
	}
	if refreshedFound {
		currentEvent = refreshedEvent
		hasCurrentEvent = true
	}
	hasPendingLocalDelete, errorValue := service.hasPendingCalendarLocalDelete(ctx, account.ID, targetCalendarURL, remoteEvent.UID)
	if errorValue != nil {
		return errorValue
	}
	if hasPendingLocalDelete {
		return service.reconcilePulledRemoteEventWithPendingLocalDeleteLocked(ctx, account, currentEvent, remoteEvent, deferredProjections)
	}
	if hasCurrentEvent {
		if currentEvent.RemoteETag == remoteEvent.RemoteETag && currentEvent.RemoteETag != "" {
			return nil
		}
		remoteEvent.ID = currentEvent.ID
		pendingLocalChange, hasPendingLocalChange, errorValue := service.readPendingCalendarLocalChange(ctx, account.ID, targetCalendarURL, remoteEvent.UID)
		if errorValue != nil {
			return errorValue
		}
		if hasPendingLocalChange {
			return service.applyPulledRemoteEventWithPendingLocalChangeLocked(ctx, account, targetCalendarURL, currentEvent, remoteEvent, pendingLocalChange, deferredProjections)
		}
		previousRemote := decodeCalendarEventFromRawICS(currentEvent.RawICS, currentEvent.RemoteHref, currentEvent.CreatedByEmail)
		remoteWinningFields := diffCalendarEventFields(previousRemote, remoteEvent)
		mergedRemoteEvent := preserveCalendarInternalParticipants(remoteEvent, currentEvent, nil)
		remoteWinningFields = excludeCalendarFields(remoteWinningFields, diffCalendarEventFields(remoteEvent, mergedRemoteEvent))
		remoteEvent = mergedRemoteEvent
		if errorValue := service.writePulledCalendarEventLocked(ctx, account.ID, targetCalendarURL, remoteEvent, remoteWinningFields, deferredProjections); errorValue != nil {
			return fmt.Errorf("write pulled event %s: %w", remoteEvent.UID, errorValue)
		}
		return nil
	} else {
		remoteEvent.ID = randomHex(16)
	}
	if errorValue := service.writePulledCalendarEventLocked(ctx, account.ID, targetCalendarURL, remoteEvent, calendarAllUserEditableFields(), deferredProjections); errorValue != nil {
		return fmt.Errorf("write pulled event %s: %w", remoteEvent.UID, errorValue)
	}
	return nil
}

func (service *Service) applyPulledRemoteEventWithPendingLocalChangeLocked(ctx context.Context, account remoteCalendarAccount, targetCalendarURL string, localEvent calendarEvent, remoteEvent calendarEvent, pendingLocalChange pendingCalendarLocalChange, deferredProjections *calendarPullDeferredProjectionQueue) error {
	if errorValue := service.recordCalendarFieldConflicts(ctx, localEvent, remoteEvent, pendingLocalChange.ChangedFields); errorValue != nil {
		return errorValue
	}
	previousRemote := decodeCalendarEventFromRawICS(localEvent.RawICS, localEvent.RemoteHref, localEvent.CreatedByEmail)
	remoteChangedFields := diffCalendarEventFields(previousRemote, remoteEvent)
	localWinningFields := selectCalendarLocalWinningFields(pendingLocalChange.ChangedFields, remoteChangedFields, pendingLocalChange.FieldChangedAt, parseCalendarConflictTime(remoteEvent.RemoteModifiedAt))
	mergedEvent := mergeCalendarEventChanges(remoteEvent, localEvent, localWinningFields)
	mergedEvent = preserveCalendarInternalParticipants(mergedEvent, localEvent, localWinningFields)
	remoteWinningFields := excludeCalendarFields(remoteChangedFields, mergeCalendarFieldLists(localWinningFields, diffCalendarEventFields(remoteEvent, mergedEvent)))
	mergedEvent.ID = localEvent.ID
	mergedEvent.CreatedByEmail = localEvent.CreatedByEmail
	mergedEvent.CreatedByName = localEvent.CreatedByName
	mergedEvent.UpdatedByEmail = localEvent.UpdatedByEmail
	mergedEvent.UpdatedByName = localEvent.UpdatedByName
	mergedEvent.UpdatedByAt = localEvent.UpdatedByAt
	mergedEvent.MattermostPostID = localEvent.MattermostPostID
	if errorValue := service.writePulledCalendarEventAndRetainOutboxLocked(ctx, account.ID, targetCalendarURL, mergedEvent, localWinningFields, remoteWinningFields, deferredProjections); errorValue != nil {
		return fmt.Errorf("write pulled pending local event %s: %w", mergedEvent.UID, errorValue)
	}
	return nil
}
