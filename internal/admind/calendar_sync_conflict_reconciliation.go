package admind

import (
	"context"
	"strings"
	"time"
)

func (service *Service) reconcileCalendarRemoteDeletionDuringPush(ctx context.Context, account remoteCalendarAccount, client calDAVPushClient, row calendarOutboxRow, localEvent calendarEvent, detectedAt time.Time) (bool, error) {
	targetCalendarURL := row.TargetCalendarURL
	remoteState, found, errorValue := service.readCalendarRemoteEventState(ctx, account.ID, targetCalendarURL, localEvent.UID)
	if errorValue != nil {
		return false, errorValue
	}
	if !found || strings.TrimSpace(remoteState.LastSeenAt) == "" {
		return false, errCalDAVObjectNotFound
	}
	remoteState, errorValue = service.markCalendarRemoteEventMissing(ctx, account.ID, targetCalendarURL, localEvent.UID, detectedAt)
	if errorValue != nil {
		return false, errorValue
	}
	winner := resolveCalendarRemoteDeletion(
		parseCalendarConflictTime(row.CreatedAt),
		parseCalendarConflictTime(remoteState.LastSeenAt),
		parseCalendarConflictTime(remoteState.MissingDetectedAt),
	)
	if winner == calendarConflictWinnerRemote {
		deleted, errorValue := service.softDeleteCalendarEventIfRevisionMatches(ctx, localEvent, calendarSourcePull)
		if errorValue != nil {
			return false, errorValue
		}
		if !deleted {
			return false, nil
		}
		return false, service.deleteCalendarOutboxBatch(ctx, row)
	}
	objectPath := strings.TrimRight(targetCalendarURL, "/") + "/" + localEvent.UID + ".ics"
	encoded, errorValue := encodeEventToICS(localEvent)
	if errorValue != nil {
		return false, errorValue
	}
	newETag, errorValue := service.guardedCalendarPut(ctx, client, row, objectPath, encoded, "", caldavWildcardETag)
	if errorValue != nil {
		return false, errorValue
	}
	if errorValue := service.applyCalendarPushSuccess(ctx, row, localEvent, localEvent, objectPath, newETag, encoded); errorValue != nil {
		return false, errorValue
	}
	observedEvent := localEvent
	observedEvent.RemoteHref = objectPath
	observedEvent.RemoteETag = newETag
	if errorValue := service.markCalendarRemoteEventObserved(ctx, account.ID, targetCalendarURL, observedEvent, detectedAt); errorValue != nil {
		return false, errorValue
	}
	return true, nil
}

func (service *Service) reconcileCalendarLocalDeletionDuringPush(ctx context.Context, account remoteCalendarAccount, client calDAVPushClient, row calendarOutboxRow, observedAt time.Time) error {
	remoteObject, errorValue := service.guardedCalendarGet(ctx, client, row, row.RemoteHref)
	if errorValue != nil {
		if isCalDAVObjectNotFound(errorValue) {
			return nil
		}
		return errorValue
	}
	remoteEvent, errorValue := decodeRemoteCalendarObject(remoteObject, account.AccountEmail)
	if errorValue != nil {
		return errorValue
	}
	shouldDeleteRemote, errorValue := service.applyCalendarDeleteConflictResolution(ctx, account, row, remoteEvent, observedAt)
	if errorValue != nil {
		return errorValue
	}
	if shouldDeleteRemote {
		return service.guardedCalendarDelete(ctx, client, row, remoteObject.Path, remoteObject.ETag)
	}
	return nil
}

func (service *Service) reconcilePulledRemoteEventWithPendingLocalDeleteLocked(ctx context.Context, account remoteCalendarAccount, localEvent calendarEvent, remoteEvent calendarEvent, deferredProjections *calendarPullDeferredProjectionQueue) error {
	targetCalendarURL := activeRemoteCalendarTarget(account).CalendarURL
	projection, found, errorValue := service.readCalendarEventProjectionByID(ctx, localEvent.ID)
	if errorValue != nil || !found {
		return errorValue
	}
	localDeletedAt := parseCalendarConflictTime(projection.DeletedAt)
	remoteModifiedAt := parseCalendarConflictTime(remoteEvent.RemoteModifiedAt)
	if resolveCalendarLocalDeletion(localDeletedAt, remoteModifiedAt) == calendarConflictWinnerLocal {
		return service.updatePendingCalendarDeleteRemoteState(ctx, account.ID, targetCalendarURL, localEvent.UID, remoteEvent.RemoteHref, remoteEvent.RemoteETag)
	}
	remoteEvent = restoreCalendarRemoteEventFromProjection(remoteEvent, projection.Event)
	return service.writePulledCalendarEventAndDeleteOutboxLocked(ctx, account.ID, targetCalendarURL, remoteEvent, deferredProjections)
}

func restoreCalendarRemoteEventFromProjection(remoteEvent calendarEvent, projectionEvent calendarEvent) calendarEvent {
	remoteEvent.ID = projectionEvent.ID
	remoteEvent = preserveCalendarInternalParticipants(remoteEvent, projectionEvent, nil)
	remoteEvent.CreatedByEmail = projectionEvent.CreatedByEmail
	remoteEvent.CreatedByName = projectionEvent.CreatedByName
	remoteEvent.UpdatedByEmail = projectionEvent.UpdatedByEmail
	remoteEvent.UpdatedByName = projectionEvent.UpdatedByName
	remoteEvent.UpdatedByAt = projectionEvent.UpdatedByAt
	remoteEvent.MattermostPostID = projectionEvent.MattermostPostID
	return remoteEvent
}
