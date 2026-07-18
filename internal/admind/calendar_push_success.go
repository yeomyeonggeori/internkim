package admind

import (
	"context"
	"strings"
	"time"
)

func (service *Service) applyCalendarPushSuccess(ctx context.Context, row calendarOutboxRow, snapshotEvent calendarEvent, pushedEvent calendarEvent, objectPath string, newETag string, rawICS []byte) error {
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
	pushedEvent.RemoteSource = remoteCalendarProviderGoogle
	pushedEvent.RemoteHref = objectPath
	pushedEvent.RemoteETag = newETag
	pushedEvent.RawICS = string(rawICS)
	if calendarEventRevisionMatches(projection.Event, snapshotEvent) {
		return service.writeCalendarEventWithSourceLocked(ctx, pushedEvent, calendarSourcePull, time.Now().UTC())
	}
	currentEvent := projection.Event
	currentEvent.RemoteSource = pushedEvent.RemoteSource
	currentEvent.RemoteHref = pushedEvent.RemoteHref
	currentEvent.RemoteETag = pushedEvent.RemoteETag
	currentEvent.RawICS = pushedEvent.RawICS
	if errorValue := service.writeCalendarEventWithSourceLocked(ctx, currentEvent, calendarSourcePull, time.Now().UTC()); errorValue != nil {
		return errorValue
	}
	return service.updatePendingCalendarOutboxRemoteState(ctx, row.AccountID, row.TargetCalendarURL, currentEvent.UID, objectPath, newETag)
}

func (service *Service) persistCalendarPushSuccessForDeletedEvent(ctx context.Context, row calendarOutboxRow, eventID string, eventUID string, remoteHref string, remoteETag string, rawICS []byte) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	if _, errorValue := transaction.ExecContext(ctx, `UPDATE calendar_events SET remote_source = ?, remote_href = ?, remote_etag = ?, raw_ics = ? WHERE id = ? AND deleted_at <> ''`, remoteCalendarProviderGoogle, strings.TrimSpace(remoteHref), strings.TrimSpace(remoteETag), string(rawICS), strings.TrimSpace(eventID)); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := replaceCalendarMutationOrigin(ctx, transaction, eventID, "", nil); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if _, errorValue := transaction.ExecContext(ctx, `UPDATE calendar_outbox SET if_match_etag = ?, remote_href = ? WHERE account_id = ? AND target_calendar_url = ? AND event_uid = ? AND operation = ? AND status = ?`, strings.TrimSpace(remoteETag), strings.TrimSpace(remoteHref), strings.TrimSpace(row.AccountID), normalizeCalendarOutboxTargetURL(row.TargetCalendarURL), strings.TrimSpace(eventUID), calendarOutboxOperationDelete, calendarOutboxStatusPending); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

func calendarEventRevisionMatches(currentEvent calendarEvent, snapshotEvent calendarEvent) bool {
	return strings.TrimSpace(currentEvent.UpdatedAt) == strings.TrimSpace(snapshotEvent.UpdatedAt)
}
