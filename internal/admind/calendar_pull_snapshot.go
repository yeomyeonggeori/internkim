package admind

import (
	"context"
	"log"
	"time"
)

type calendarPullReconciliationResult struct {
	RemoteUIDs map[string]struct{}
	IsComplete bool
}

func (service *Service) reconcileCalendarPullSnapshot(ctx context.Context, account remoteCalendarAccount, remoteObjects []calDAVCalendarObject, protectedUIDs map[string]struct{}) (calendarPullReconciliationResult, error) {
	target := activeRemoteCalendarTarget(account)
	fencedUIDs, errorValue := service.listCalendarPushObservationFenceUIDs(ctx, account.ID, target.CalendarURL)
	if errorValue != nil {
		return calendarPullReconciliationResult{}, errorValue
	}
	return service.reconcileCalendarPullSnapshotWithFences(ctx, account, remoteObjects, protectedUIDs, fencedUIDs)
}

func (service *Service) reconcileCalendarPullSnapshotWithFences(ctx context.Context, account remoteCalendarAccount, remoteObjects []calDAVCalendarObject, protectedUIDs map[string]struct{}, fencedUIDs map[string]struct{}) (calendarPullReconciliationResult, error) {
	deferredProjections := &calendarPullDeferredProjectionQueue{}
	var result calendarPullReconciliationResult
	var errorValue error
	service.runCalendarPullLocked(ctx, deferredProjections, func() {
		result, errorValue = service.reconcileCalendarPullSnapshotWithFencesLocked(ctx, account, remoteObjects, protectedUIDs, fencedUIDs, deferredProjections)
	})
	return result, errorValue
}

func (service *Service) reconcileCalendarPullSnapshotWithFencesLocked(ctx context.Context, account remoteCalendarAccount, remoteObjects []calDAVCalendarObject, protectedUIDs map[string]struct{}, fencedUIDs map[string]struct{}, deferredProjections *calendarPullDeferredProjectionQueue) (calendarPullReconciliationResult, error) {
	target := activeRemoteCalendarTarget(account)
	observedAt := time.Now().UTC()
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
	result, errorValue := service.applyCalendarPullSnapshotLocked(ctx, account, target, remoteObjects, existingByUID, observedAt, deferredProjections)
	if errorValue != nil {
		return calendarPullReconciliationResult{}, errorValue
	}
	service.markCalendarPushUIDsObserved(result.RemoteUIDs)
	allProtectedUIDs := mergeCalendarUIDSets(protectedUIDs, fencedUIDs)
	if hasMissingCalendarPushObservationFence(fencedUIDs, result.RemoteUIDs) || hasProtectedMissingCalendarUID(activeEvents, target, result.RemoteUIDs, allProtectedUIDs) {
		result.IsComplete = false
	}
	if !result.IsComplete {
		return result, nil
	}
	if !target.NeedsInitialSyncCompletion {
		if errorValue := service.softDeleteMissingRemoteEventsWithConflictStateLocked(ctx, account.ID, target, activeEvents, result.RemoteUIDs, allProtectedUIDs, pendingLocalChanges, observedAt, deferredProjections); errorValue != nil {
			return calendarPullReconciliationResult{}, errorValue
		}
	}
	if errorValue := service.clearObservedCalendarPushObservationFences(ctx, account.ID, target.CalendarURL, fencedUIDs, result.RemoteUIDs); errorValue != nil {
		return calendarPullReconciliationResult{}, errorValue
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
	deferredProjections := &calendarPullDeferredProjectionQueue{}
	var result calendarPullReconciliationResult
	var errorValue error
	service.runCalendarPullLocked(ctx, deferredProjections, func() {
		result, errorValue = service.applyCalendarPullSnapshotLocked(ctx, account, target, remoteObjects, existingByUID, observedAt, deferredProjections)
	})
	return result, errorValue
}

func (service *Service) applyCalendarPullSnapshotLocked(ctx context.Context, account remoteCalendarAccount, target remoteCalendarTarget, remoteObjects []calDAVCalendarObject, existingByUID map[string]calendarEvent, observedAt time.Time, deferredProjections *calendarPullDeferredProjectionQueue) (calendarPullReconciliationResult, error) {
	result := calendarPullReconciliationResult{RemoteUIDs: map[string]struct{}{}, IsComplete: true}
	for _, object := range remoteObjects {
		event, errorValue := decodeRemoteCalendarObject(object, account.AccountEmail)
		if errorValue != nil {
			log.Printf("calendar pull decode failed for %s: %v", object.Path, errorValue)
			result.IsComplete = false
			continue
		}
		result.RemoteUIDs[event.UID] = struct{}{}
		if errorValue := service.markCalendarRemoteEventObserved(ctx, account.ID, target.CalendarURL, event, observedAt); errorValue != nil {
			return calendarPullReconciliationResult{}, errorValue
		}
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

func hasProtectedMissingCalendarUID(activeEvents []calendarEvent, target remoteCalendarTarget, remoteUIDs map[string]struct{}, protectedUIDs map[string]struct{}) bool {
	for _, event := range activeEvents {
		if event.RemoteSource != remoteCalendarProviderGoogle || !remoteCalendarEventBelongsToTarget(event, target) {
			continue
		}
		if _, isRemote := remoteUIDs[event.UID]; isRemote {
			continue
		}
		if _, isProtected := protectedUIDs[event.UID]; isProtected {
			return true
		}
	}
	return false
}
