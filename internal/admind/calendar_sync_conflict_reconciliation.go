package admind

import (
	"context"
	"strings"
	"time"
)

func (service *Service) reconcileCalendarRemoteDeletionDuringPush(ctx context.Context, account remoteCalendarAccount, client calDAVPushClient, row calendarOutboxRow, localEvent calendarEvent, detectedAt time.Time) (bool, error) {
	target := activeRemoteCalendarTarget(account)
	remoteState, errorValue := service.markCalendarRemoteEventMissing(ctx, account.ID, target.CalendarURL, localEvent.UID, detectedAt)
	if errorValue != nil {
		return false, errorValue
	}
	winner := resolveCalendarRemoteDeletion(
		parseCalendarConflictTime(row.CreatedAt),
		parseCalendarConflictTime(remoteState.LastSeenAt),
		parseCalendarConflictTime(remoteState.MissingDetectedAt),
	)
	if winner == calendarConflictWinnerRemote {
		if errorValue := service.softDeleteCalendarEventWithSource(ctx, localEvent.ID, calendarSourcePull); errorValue != nil {
			return false, errorValue
		}
		return false, service.deleteCalendarOutboxForEventUID(ctx, account.ID, localEvent.UID)
	}
	objectPath := strings.TrimRight(target.CalendarURL, "/") + "/" + localEvent.UID + ".ics"
	encoded, errorValue := encodeEventToICS(localEvent)
	if errorValue != nil {
		return false, errorValue
	}
	newETag, errorValue := client.putCalendarObject(ctx, objectPath, encoded, "", caldavWildcardETag)
	if errorValue != nil {
		return false, errorValue
	}
	if errorValue := service.applyCalendarPushSuccess(ctx, localEvent, objectPath, newETag, encoded); errorValue != nil {
		return false, errorValue
	}
	observedEvent := localEvent
	observedEvent.RemoteHref = objectPath
	observedEvent.RemoteETag = newETag
	if errorValue := service.markCalendarRemoteEventObserved(ctx, account.ID, target.CalendarURL, observedEvent, detectedAt); errorValue != nil {
		return false, errorValue
	}
	return true, nil
}

func (service *Service) reconcileCalendarLocalDeletionDuringPush(ctx context.Context, account remoteCalendarAccount, client calDAVPushClient, row calendarOutboxRow, observedAt time.Time) error {
	remoteObject, errorValue := client.getCalendarObject(ctx, row.RemoteHref)
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
	projection, found, errorValue := service.readCalendarEventProjectionByID(ctx, row.EventID)
	if errorValue != nil || !found {
		return errorValue
	}
	localDeletedAt := parseCalendarConflictTime(projection.DeletedAt)
	remoteModifiedAt := parseCalendarConflictTime(remoteEvent.RemoteModifiedAt)
	if resolveCalendarLocalDeletion(localDeletedAt, remoteModifiedAt) == calendarConflictWinnerLocal {
		return client.deleteCalendarObject(ctx, remoteObject.Path, remoteObject.ETag)
	}
	remoteEvent = restoreCalendarRemoteEventFromProjection(remoteEvent, projection.Event)
	target := activeRemoteCalendarTarget(account)
	if errorValue := service.markCalendarRemoteEventObserved(ctx, account.ID, target.CalendarURL, remoteEvent, observedAt); errorValue != nil {
		return errorValue
	}
	if errorValue := service.deleteCalendarOutboxForEventUID(ctx, account.ID, remoteEvent.UID); errorValue != nil {
		return errorValue
	}
	return service.writeCalendarEventWithSource(ctx, remoteEvent, calendarSourcePull)
}

func (service *Service) reconcilePulledRemoteEventWithPendingLocalDeleteLocked(ctx context.Context, account remoteCalendarAccount, localEvent calendarEvent, remoteEvent calendarEvent) error {
	projection, found, errorValue := service.readCalendarEventProjectionByID(ctx, localEvent.ID)
	if errorValue != nil || !found {
		return errorValue
	}
	localDeletedAt := parseCalendarConflictTime(projection.DeletedAt)
	remoteModifiedAt := parseCalendarConflictTime(remoteEvent.RemoteModifiedAt)
	if resolveCalendarLocalDeletion(localDeletedAt, remoteModifiedAt) == calendarConflictWinnerLocal {
		return service.updatePendingCalendarDeleteRemoteState(ctx, account.ID, localEvent.UID, remoteEvent.RemoteHref, remoteEvent.RemoteETag)
	}
	remoteEvent = restoreCalendarRemoteEventFromProjection(remoteEvent, projection.Event)
	if errorValue := service.deleteCalendarOutboxForEventUID(ctx, account.ID, remoteEvent.UID); errorValue != nil {
		return errorValue
	}
	if errorValue := service.writeCalendarEventWithSourceLocked(ctx, remoteEvent, calendarSourcePull); errorValue != nil {
		return errorValue
	}
	return nil
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
