package admind

import (
	"context"
	"log/slog"
	"time"
)

type calendarPullReconciliationResult struct {
	RemoteUIDs map[string]struct{}
	IsComplete bool
}

func (service *Service) reconcileCalendarPullSnapshot(ctx context.Context, account remoteCalendarAccount, remoteObjects []calDAVCalendarObject) (calendarPullReconciliationResult, error) {
	target := activeRemoteCalendarTarget(account)
	fencedUIDs, errorValue := service.listCalendarPushObservationFenceUIDs(ctx, account.ID, target.CalendarURL)
	if errorValue != nil {
		return calendarPullReconciliationResult{}, errorValue
	}
	return service.reconcileCalendarPullSnapshotWithFences(ctx, account, remoteObjects, fencedUIDs)
}

func (service *Service) reconcileCalendarPullSnapshotWithFences(ctx context.Context, account remoteCalendarAccount, remoteObjects []calDAVCalendarObject, fencedUIDs map[string]struct{}) (calendarPullReconciliationResult, error) {
	deferredProjections := &calendarPullDeferredProjectionQueue{}
	var result calendarPullReconciliationResult
	var errorValue error
	snapshotCompletedAt, errorValue := service.reserveCalendarConflictCandidateTime(ctx, time.Now().UTC())
	if errorValue != nil {
		return calendarPullReconciliationResult{}, errorValue
	}
	service.runCalendarPullLocked(ctx, deferredProjections, func() {
		result, errorValue = service.reconcileCalendarPullSnapshotWithFencesLocked(ctx, account, remoteObjects, fencedUIDs, snapshotCompletedAt, deferredProjections)
	})
	return result, errorValue
}

func (service *Service) reconcileCalendarPullSnapshotWithFencesLocked(ctx context.Context, account remoteCalendarAccount, remoteObjects []calDAVCalendarObject, fencedUIDs map[string]struct{}, snapshotCompletedAt time.Time, deferredProjections *calendarPullDeferredProjectionQueue) (calendarPullReconciliationResult, error) {
	target := activeRemoteCalendarTarget(account)
	activeEvents, errorValue := service.readCalendarEvents(ctx, time.Time{}, time.Time{})
	if errorValue != nil {
		return calendarPullReconciliationResult{}, errorValue
	}
	softDeletedEvents, errorValue := service.readSoftDeletedCalendarEvents(ctx)
	if errorValue != nil {
		return calendarPullReconciliationResult{}, errorValue
	}
	existingByUID := calendarEventsByUID(activeEvents, softDeletedEvents)
	pendingLocalChanges, errorValue := service.listPendingCalendarLocalChanges(ctx, account.ID, target.CalendarURL)
	if errorValue != nil {
		return calendarPullReconciliationResult{}, errorValue
	}
	result, errorValue := service.applyCalendarPullSnapshotLocked(ctx, account, target, remoteObjects, existingByUID, snapshotCompletedAt, deferredProjections)
	if errorValue != nil {
		return calendarPullReconciliationResult{}, errorValue
	}
	if !result.IsComplete {
		return result, nil
	}
	protectedUIDs, errorValue := service.protectMissingCalendarPushObservationFences(ctx, account.ID, target.CalendarURL, fencedUIDs, result.RemoteUIDs, pendingLocalChanges, snapshotCompletedAt)
	if errorValue != nil {
		return calendarPullReconciliationResult{}, errorValue
	}
	if !target.NeedsInitialSyncCompletion {
		if errorValue := service.softDeleteMissingRemoteEventsWithConflictStateLocked(ctx, account.ID, target, activeEvents, result.RemoteUIDs, protectedUIDs, pendingLocalChanges, snapshotCompletedAt, deferredProjections); errorValue != nil {
			return calendarPullReconciliationResult{}, errorValue
		}
	}
	if errorValue := service.clearObservedCalendarPushObservationFences(ctx, account.ID, target.CalendarURL, fencedUIDs, result.RemoteUIDs); errorValue != nil {
		return calendarPullReconciliationResult{}, errorValue
	}
	if len(protectedUIDs) > 0 {
		result.IsComplete = false
	}
	return result, nil
}

func calendarEventsByUID(activeEvents []calendarEvent, softDeletedEvents []calendarEvent) map[string]calendarEvent {
	existingByUID := map[string]calendarEvent{}
	for _, event := range activeEvents {
		existingByUID[event.UID] = event
	}
	for _, event := range softDeletedEvents {
		if _, alreadyActive := existingByUID[event.UID]; !alreadyActive {
			existingByUID[event.UID] = event
		}
	}
	return existingByUID
}

func (service *Service) applyCalendarPullSnapshot(ctx context.Context, account remoteCalendarAccount, target remoteCalendarTarget, remoteObjects []calDAVCalendarObject, existingByUID map[string]calendarEvent, observedAt time.Time) (calendarPullReconciliationResult, error) {
	reservedObservedAt, errorValue := service.reserveCalendarConflictCandidateTime(ctx, observedAt)
	if errorValue != nil {
		return calendarPullReconciliationResult{}, errorValue
	}
	deferredProjections := &calendarPullDeferredProjectionQueue{}
	var result calendarPullReconciliationResult
	errorValue = nil
	service.runCalendarPullLocked(ctx, deferredProjections, func() {
		result, errorValue = service.applyCalendarPullSnapshotLocked(ctx, account, target, remoteObjects, existingByUID, reservedObservedAt, deferredProjections)
	})
	return result, errorValue
}

func (service *Service) applyCalendarPullSnapshotLocked(ctx context.Context, account remoteCalendarAccount, target remoteCalendarTarget, remoteObjects []calDAVCalendarObject, existingByUID map[string]calendarEvent, observedAt time.Time, deferredProjections *calendarPullDeferredProjectionQueue) (calendarPullReconciliationResult, error) {
	decodedEvents, result := decodeCalendarPullSnapshot(remoteObjects, account.AccountEmail)
	if errorValue := service.persistObservedCalendarRemoteEventStateBatch(ctx, account.ID, target.CalendarURL, decodedEvents, observedAt); errorValue != nil {
		return calendarPullReconciliationResult{}, errorValue
	}
	for _, event := range decodedEvents {
		previous, found := existingByUID[event.UID]
		if found && previous.RemoteETag == event.RemoteETag && previous.RemoteETag != "" {
			continue
		}
		if errorValue := service.applyPulledRemoteEventLocked(ctx, account, previous, found, event, deferredProjections); errorValue != nil {
			return calendarPullReconciliationResult{}, errorValue
		}
	}
	return result, nil
}

func decodeCalendarPullSnapshot(remoteObjects []calDAVCalendarObject, accountEmail string) ([]calendarEvent, calendarPullReconciliationResult) {
	decodedEvents := make([]calendarEvent, 0, len(remoteObjects))
	result := calendarPullReconciliationResult{RemoteUIDs: map[string]struct{}{}, IsComplete: true}
	for _, object := range remoteObjects {
		event, errorValue := decodeRemoteCalendarObject(object, accountEmail)
		if errorValue != nil {
			slog.Warn("calendar pull decode failed", "path", object.Path, "error", errorValue)
			result.IsComplete = false
			continue
		}
		decodedEvents = append(decodedEvents, event)
		result.RemoteUIDs[event.UID] = struct{}{}
	}
	return decodedEvents, result
}
