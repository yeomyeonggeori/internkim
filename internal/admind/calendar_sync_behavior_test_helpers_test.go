package admind

import (
	"context"
	"time"
)

func (service *Service) reconcileGoogleCalendarPull(ctx context.Context, account remoteCalendarAccount, remoteObjects []calDAVCalendarObject, _ map[string]struct{}) error {
	_, errorValue := service.reconcileCalendarPullSnapshot(ctx, account, remoteObjects)
	return errorValue
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
	snapshotCompletedAt, errorValue := service.reserveCalendarConflictCandidateTime(ctx, time.Now().UTC())
	if errorValue != nil {
		return calendarPullReconciliationResult{}, errorValue
	}
	service.runCalendarPullLocked(ctx, deferredProjections, func() {
		result, errorValue = service.reconcileCalendarPullSnapshotWithFencesLocked(ctx, account, remoteObjects, fencedUIDs, snapshotCompletedAt, deferredProjections)
	})
	return result, errorValue
}

func (service *Service) applyPulledRemoteEvents(ctx context.Context, account remoteCalendarAccount, remoteObjects []calDAVCalendarObject, existingByUID map[string]calendarEvent) (map[string]struct{}, error) {
	return service.applyPulledRemoteEventsAt(ctx, account, activeRemoteCalendarTarget(account), remoteObjects, existingByUID, time.Now().UTC())
}

func (service *Service) applyPulledRemoteEventsAt(ctx context.Context, account remoteCalendarAccount, target remoteCalendarTarget, remoteObjects []calDAVCalendarObject, existingByUID map[string]calendarEvent, observedAt time.Time) (map[string]struct{}, error) {
	result, errorValue := service.applyCalendarPullSnapshot(ctx, account, target, remoteObjects, existingByUID, observedAt)
	return result.RemoteUIDs, errorValue
}

func (service *Service) applyCalendarPullSnapshot(ctx context.Context, account remoteCalendarAccount, target remoteCalendarTarget, remoteObjects []calDAVCalendarObject, existingByUID map[string]calendarEvent, observedAt time.Time) (calendarPullReconciliationResult, error) {
	reservedObservedAt, errorValue := service.reserveCalendarConflictCandidateTime(ctx, observedAt)
	if errorValue != nil {
		return calendarPullReconciliationResult{}, errorValue
	}
	deferredProjections := &calendarPullDeferredProjectionQueue{}
	var result calendarPullReconciliationResult
	service.runCalendarPullLocked(ctx, deferredProjections, func() {
		result, errorValue = service.applyCalendarPullSnapshotLocked(ctx, account, target, remoteObjects, existingByUID, reservedObservedAt, deferredProjections)
	})
	return result, errorValue
}

func (service *Service) applyPulledRemoteEvent(ctx context.Context, account remoteCalendarAccount, previousEvent calendarEvent, hasPreviousEvent bool, remoteEvent calendarEvent) error {
	deferredProjections := &calendarPullDeferredProjectionQueue{}
	var errorValue error
	service.runCalendarPullLocked(ctx, deferredProjections, func() {
		errorValue = service.applyPulledRemoteEventLocked(ctx, account, previousEvent, hasPreviousEvent, remoteEvent, deferredProjections)
	})
	return errorValue
}

func (service *Service) softDeleteMissingRemoteEvents(ctx context.Context, accountID string, target remoteCalendarTarget, allEvents []calendarEvent, remoteUIDs map[string]struct{}, protectedUIDs map[string]struct{}) error {
	pendingLocalChanges, errorValue := service.listPendingCalendarLocalChanges(ctx, accountID, target.CalendarURL)
	if errorValue != nil {
		return errorValue
	}
	return service.softDeleteMissingRemoteEventsWithConflictState(ctx, accountID, target, allEvents, remoteUIDs, protectedUIDs, pendingLocalChanges, time.Now().UTC())
}

func (service *Service) softDeleteMissingRemoteEventsWithConflictState(ctx context.Context, accountID string, target remoteCalendarTarget, allEvents []calendarEvent, remoteUIDs map[string]struct{}, protectedUIDs map[string]struct{}, pendingLocalChanges map[string]pendingCalendarLocalChange, detectedAt time.Time) error {
	reservedDetectedAt, errorValue := service.reserveCalendarConflictCandidateTime(ctx, detectedAt)
	if errorValue != nil {
		return errorValue
	}
	deferredProjections := &calendarPullDeferredProjectionQueue{}
	service.runCalendarPullLocked(ctx, deferredProjections, func() {
		errorValue = service.softDeleteMissingRemoteEventsWithConflictStateLocked(ctx, accountID, target, allEvents, remoteUIDs, protectedUIDs, pendingLocalChanges, reservedDetectedAt, deferredProjections)
	})
	return errorValue
}

func (service *Service) softDeleteMissingRemoteEvent(ctx context.Context, accountID string, target remoteCalendarTarget, event calendarEvent, pendingLocalChange pendingCalendarLocalChange, detectedAt time.Time) error {
	reservedDetectedAt, errorValue := service.reserveCalendarConflictCandidateTime(ctx, detectedAt)
	if errorValue != nil {
		return errorValue
	}
	deferredProjections := &calendarPullDeferredProjectionQueue{}
	service.runCalendarPullLocked(ctx, deferredProjections, func() {
		errorValue = service.softDeleteMissingRemoteEventLocked(ctx, accountID, target, event, pendingLocalChange, reservedDetectedAt, deferredProjections)
	})
	return errorValue
}

func (service *Service) softDeleteCalendarEventIfRevisionMatches(ctx context.Context, snapshotEvent calendarEvent, source string) (bool, error) {
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
	currentEvent, found, errorValue := service.readCalendarEventByID(ctx, snapshotEvent.ID)
	if errorValue != nil || !found {
		return false, errorValue
	}
	if !calendarEventRevisionMatches(currentEvent, snapshotEvent) {
		return false, nil
	}
	if errorValue := service.softDeleteCalendarEventWithSourceLocked(ctx, currentEvent.ID, source, time.Now().UTC()); errorValue != nil {
		return false, errorValue
	}
	return true, nil
}
