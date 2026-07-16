package admind

import (
	"context"
	"fmt"
	"time"
)

func (service *Service) softDeleteMissingRemoteEventsWithConflictStateLocked(ctx context.Context, accountID string, target remoteCalendarTarget, allEvents []calendarEvent, remoteUIDs map[string]struct{}, protectedUIDs map[string]struct{}, pendingLocalChanges map[string]pendingCalendarLocalChange, detectedAt time.Time, deferredProjections *calendarPullDeferredProjectionQueue) error {
	for _, event := range allEvents {
		if event.RemoteSource != remoteCalendarProviderGoogle {
			continue
		}
		if !remoteCalendarEventBelongsToTarget(event, target) {
			continue
		}
		if _, kept := remoteUIDs[event.UID]; kept {
			continue
		}
		if _, protected := protectedUIDs[event.UID]; protected {
			continue
		}
		if errorValue := service.softDeleteMissingRemoteEventLocked(ctx, accountID, target, event, pendingLocalChanges[event.UID], detectedAt, deferredProjections); errorValue != nil {
			return fmt.Errorf("soft delete %s: %w", event.ID, errorValue)
		}
	}
	return nil
}

func (service *Service) softDeleteMissingRemoteEventLocked(ctx context.Context, accountID string, target remoteCalendarTarget, event calendarEvent, pendingLocalChange pendingCalendarLocalChange, detectedAt time.Time, deferredProjections *calendarPullDeferredProjectionQueue) error {
	currentEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil {
		return errorValue
	}
	if !found || currentEvent.RemoteSource != remoteCalendarProviderGoogle {
		return nil
	}
	if hasPendingLocalDelete, errorValue := service.hasPendingCalendarLocalDelete(ctx, accountID, target.CalendarURL, currentEvent.UID); errorValue != nil {
		return errorValue
	} else if hasPendingLocalDelete {
		return nil
	}
	refreshedPendingLocalChange, hasPendingLocalChange, errorValue := service.readPendingCalendarLocalChange(ctx, accountID, target.CalendarURL, currentEvent.UID)
	if errorValue != nil {
		return errorValue
	}
	if hasPendingLocalChange {
		pendingLocalChange = refreshedPendingLocalChange
	} else {
		pendingLocalChange = pendingCalendarLocalChange{}
	}
	localChangedAt := latestCalendarPendingChangeAt(pendingLocalChange)
	remoteState, errorValue := service.markCalendarRemoteEventMissingAtReservedBoundary(ctx, accountID, target.CalendarURL, currentEvent.UID, detectedAt)
	if errorValue != nil {
		return errorValue
	}
	if localChangedAt.After(parseCalendarConflictTime(remoteState.MissingDetectedAt)) {
		return service.updatePendingCalendarOutboxRemoteState(ctx, accountID, target.CalendarURL, currentEvent.UID, "", "")
	}
	if len(pendingLocalChange.ChangedFields) > 0 {
		winner := resolveCalendarRemoteDeletion(
			localChangedAt,
			parseCalendarConflictTime(remoteState.LastSeenAt),
			parseCalendarConflictTime(remoteState.MissingDetectedAt),
		)
		if winner == calendarConflictWinnerLocal {
			return service.updatePendingCalendarOutboxRemoteState(ctx, accountID, target.CalendarURL, currentEvent.UID, "", "")
		}
		return service.acceptMissingCalendarRemoteDeletionLocked(ctx, accountID, target.CalendarURL, currentEvent, deferredProjections)
	}
	return service.acceptMissingCalendarRemoteDeletionLocked(ctx, accountID, target.CalendarURL, currentEvent, deferredProjections)
}
