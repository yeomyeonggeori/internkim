package admind

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

type calDAVPullClient interface {
	discoverPrincipalURL(ctx context.Context) (string, error)
	discoverHomeSetURL(ctx context.Context, principalURL string) (string, error)
	listCalendars(ctx context.Context, homeSetURL string) ([]calDAVCalendarInfo, error)
	fetchCalendarCTag(ctx context.Context, calendarPath string) (string, error)
	queryAllCalendarEvents(ctx context.Context, calendarPath string) ([]calDAVCalendarObject, error)
}

func (service *Service) pullGoogleCalendarChanges(ctx context.Context) (bool, error) {
	return service.pullCalendarChangesForProvider(ctx, googleCalendarProvider{}, false, nil)
}

func (service *Service) pullGoogleCalendarChangesWithProtection(ctx context.Context, protectedUIDs map[string]struct{}) (bool, error) {
	return service.pullCalendarChangesForProvider(ctx, googleCalendarProvider{}, false, protectedUIDs)
}

func (service *Service) pullCalendarChangesForProvider(ctx context.Context, provider calendarProvider, forceQuery bool, protectedUIDs map[string]struct{}) (bool, error) {
	account, found, errorValue := service.readRemoteCalendarAccountByProvider(ctx, provider.Name())
	if errorValue != nil {
		return false, errorValue
	}
	if !found {
		return false, nil
	}
	if selectedRemoteCalendarTarget(account).CalendarURL == "" {
		return false, nil
	}
	httpClient, errorValue := provider.BuildHTTPClient(ctx, service, account)
	if errorValue != nil {
		service.markRemoteCalendarAccountAuthError(ctx, account, errorValue)
		return false, fmt.Errorf("build http client: %w", errorValue)
	}
	client, errorValue := newOutboundCalDAVClient(provider.Endpoint(account), httpClient)
	if errorValue != nil {
		return false, errorValue
	}
	changed, errorValue := service.runCalendarPull(ctx, provider, account, client, forceQuery, protectedUIDs)
	if errorValue != nil {
		if isCalendarAuthError(errorValue) {
			service.markRemoteCalendarAccountAuthError(ctx, account, errorValue)
		}
		return false, errorValue
	}
	service.clearRemoteCalendarAccountAuthError(ctx, account)
	return changed, nil
}

func (service *Service) runGoogleCalendarPull(ctx context.Context, account remoteCalendarAccount, client calDAVPullClient) (bool, error) {
	return service.runCalendarPull(ctx, googleCalendarProvider{}, account, client, false, nil)
}

func (service *Service) runCalendarPull(ctx context.Context, provider calendarProvider, account remoteCalendarAccount, client calDAVPullClient, forceQuery bool, protectedUIDs map[string]struct{}) (bool, error) {
	if strings.TrimSpace(account.DefaultCalendarURL) == "" && strings.TrimSpace(account.SelectedCalendarURL) == "" {
		discovered, errorValue := provider.Discover(ctx, service, account)
		if errorValue != nil {
			return false, errorValue
		}
		account = discovered
	}
	target := activeRemoteCalendarTarget(account)
	if target.CalendarURL == "" {
		return false, errors.New("calendar URL required for pull")
	}
	serverCTag, errorValue := client.fetchCalendarCTag(ctx, target.CalendarURL)
	if errorValue != nil {
		return false, fmt.Errorf("fetch ctag: %w", errorValue)
	}
	fencedUIDs, errorValue := service.listCalendarPushObservationFenceUIDs(ctx, account.ID, target.CalendarURL)
	if errorValue != nil {
		return false, fmt.Errorf("list calendar push observation fences: %w", errorValue)
	}
	if !forceQuery && len(fencedUIDs) == 0 && serverCTag != "" && serverCTag == target.CalendarCTag && !target.NeedsInitialSyncCompletion {
		isFinished, errorValue := service.finishUnchangedCalendarPullIfCurrent(ctx, account, target)
		if errorValue != nil {
			return false, errorValue
		}
		if isFinished {
			return false, nil
		}
	}
	objects, errorValue := client.queryAllCalendarEvents(ctx, target.CalendarURL)
	if errorValue != nil {
		return false, fmt.Errorf("query calendar: %w", errorValue)
	}
	snapshotCompletedAt, errorValue := service.reserveCalendarConflictCandidateTime(ctx, time.Now().UTC())
	if errorValue != nil {
		return false, fmt.Errorf("reserve calendar pull snapshot boundary: %w", errorValue)
	}
	reconciliationResult, isCurrentTarget, errorValue := service.applyCalendarPullCycleIfCurrent(ctx, account, target, objects, protectedUIDs, serverCTag, snapshotCompletedAt)
	if errorValue != nil {
		return false, errorValue
	}
	if !isCurrentTarget {
		return false, nil
	}
	if !reconciliationResult.IsComplete {
		return true, nil
	}
	return true, nil
}

func (service *Service) finishUnchangedCalendarPullIfCurrent(ctx context.Context, account remoteCalendarAccount, target remoteCalendarTarget) (bool, error) {
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
	isCurrentTarget, errorValue := service.calendarPullTargetIsCurrentLocked(ctx, account, target)
	if errorValue != nil || !isCurrentTarget {
		return true, errorValue
	}
	fencedUIDs, errorValue := service.listCalendarPushObservationFenceUIDs(ctx, account.ID, target.CalendarURL)
	if errorValue != nil {
		return false, fmt.Errorf("list calendar push observation fences: %w", errorValue)
	}
	if len(fencedUIDs) > 0 {
		return false, nil
	}
	if errorValue := service.markCalendarRemoteTargetLastSeen(ctx, account, target, time.Now().UTC()); errorValue != nil {
		return false, errorValue
	}
	return true, nil
}

func (service *Service) applyCalendarPullCycleIfCurrent(ctx context.Context, account remoteCalendarAccount, target remoteCalendarTarget, objects []calDAVCalendarObject, protectedUIDs map[string]struct{}, serverCTag string, snapshotCompletedAt time.Time) (calendarPullReconciliationResult, bool, error) {
	deferredProjections := &calendarPullDeferredProjectionQueue{}
	var reconciliationResult calendarPullReconciliationResult
	var isCurrentTarget bool
	var errorValue error
	service.runCalendarPullLocked(ctx, deferredProjections, func() {
		reconciliationResult, isCurrentTarget, errorValue = service.applyCalendarPullCycleLocked(ctx, account, target, objects, protectedUIDs, serverCTag, snapshotCompletedAt, deferredProjections)
	})
	return reconciliationResult, isCurrentTarget, errorValue
}

func (service *Service) applyCalendarPullCycleLocked(ctx context.Context, account remoteCalendarAccount, target remoteCalendarTarget, objects []calDAVCalendarObject, protectedUIDs map[string]struct{}, serverCTag string, snapshotCompletedAt time.Time, deferredProjections *calendarPullDeferredProjectionQueue) (calendarPullReconciliationResult, bool, error) {
	isCurrentTarget, errorValue := service.calendarPullTargetIsCurrentLocked(ctx, account, target)
	if errorValue != nil || !isCurrentTarget {
		return calendarPullReconciliationResult{}, isCurrentTarget, errorValue
	}
	fencedUIDs, errorValue := service.listCalendarPushObservationFenceUIDs(ctx, account.ID, target.CalendarURL)
	if errorValue != nil {
		return calendarPullReconciliationResult{}, false, fmt.Errorf("list calendar push observation fences: %w", errorValue)
	}
	reconciliationResult, errorValue := service.reconcileCalendarPullSnapshotWithFencesLocked(ctx, account, objects, protectedUIDs, fencedUIDs, snapshotCompletedAt, deferredProjections)
	if errorValue != nil || !reconciliationResult.IsComplete {
		return reconciliationResult, true, errorValue
	}
	shouldSaveCTag := serverCTag != "" && serverCTag != target.CalendarCTag
	if !shouldSaveCTag && !target.NeedsInitialSyncCompletion {
		return reconciliationResult, true, nil
	}
	nextCTag := target.CalendarCTag
	if serverCTag != "" {
		nextCTag = serverCTag
	}
	if _, errorValue := service.saveCalendarPullState(ctx, account, nextCTag, time.Now(), target.NeedsInitialSyncCompletion); errorValue != nil {
		if target.NeedsInitialSyncCompletion {
			return calendarPullReconciliationResult{}, true, fmt.Errorf("update calendar pull state: %w", errorValue)
		}
		log.Printf("update calendar ctag: %v", errorValue)
	}
	return reconciliationResult, true, nil
}

func (service *Service) calendarPullTargetIsCurrentLocked(ctx context.Context, account remoteCalendarAccount, target remoteCalendarTarget) (bool, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return false, errorValue
	}
	defer database.Close()
	persistedTarget, errorValue := readActiveCalendarOutboxTargetURLWithRunner(ctx, database, account.ID)
	if errorValue != nil {
		return false, errorValue
	}
	return persistedTarget == canonicalCalendarTargetURL(target.CalendarURL), nil
}

func (service *Service) reconcileGoogleCalendarPull(ctx context.Context, account remoteCalendarAccount, remoteObjects []calDAVCalendarObject, protectedUIDs map[string]struct{}) error {
	_, errorValue := service.reconcileCalendarPullSnapshot(ctx, account, remoteObjects, protectedUIDs)
	return errorValue
}

func (service *Service) applyPulledRemoteEvents(ctx context.Context, account remoteCalendarAccount, remoteObjects []calDAVCalendarObject, existingByUID map[string]calendarEvent) (map[string]struct{}, error) {
	return service.applyPulledRemoteEventsAt(ctx, account, activeRemoteCalendarTarget(account), remoteObjects, existingByUID, time.Now().UTC())
}

func (service *Service) applyPulledRemoteEventsAt(ctx context.Context, account remoteCalendarAccount, target remoteCalendarTarget, remoteObjects []calDAVCalendarObject, existingByUID map[string]calendarEvent, observedAt time.Time) (map[string]struct{}, error) {
	result, errorValue := service.applyCalendarPullSnapshot(ctx, account, target, remoteObjects, existingByUID, observedAt)
	return result.RemoteUIDs, errorValue
}

func (service *Service) applyPulledRemoteEvent(ctx context.Context, account remoteCalendarAccount, previousEvent calendarEvent, hasPreviousEvent bool, remoteEvent calendarEvent) error {
	deferredProjections := &calendarPullDeferredProjectionQueue{}
	var errorValue error
	service.runCalendarPullLocked(ctx, deferredProjections, func() {
		errorValue = service.applyPulledRemoteEventLocked(ctx, account, previousEvent, hasPreviousEvent, remoteEvent, deferredProjections)
	})
	return errorValue
}

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
		remoteEvent = preserveCalendarInternalParticipants(remoteEvent, currentEvent, nil)
	} else {
		remoteEvent.ID = randomHex(16)
	}
	if errorValue := service.writePulledCalendarEventLocked(ctx, remoteEvent, deferredProjections); errorValue != nil {
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
	mergedEvent.ID = localEvent.ID
	mergedEvent.CreatedByEmail = localEvent.CreatedByEmail
	mergedEvent.CreatedByName = localEvent.CreatedByName
	mergedEvent.UpdatedByEmail = localEvent.UpdatedByEmail
	mergedEvent.UpdatedByName = localEvent.UpdatedByName
	mergedEvent.UpdatedByAt = localEvent.UpdatedByAt
	mergedEvent.MattermostPostID = localEvent.MattermostPostID
	if errorValue := service.writePulledCalendarEventAndRetainOutboxLocked(ctx, account.ID, targetCalendarURL, mergedEvent, localWinningFields, deferredProjections); errorValue != nil {
		return fmt.Errorf("write pulled pending local event %s: %w", mergedEvent.UID, errorValue)
	}
	return nil
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
	errorValue = nil
	service.runCalendarPullLocked(ctx, deferredProjections, func() {
		errorValue = service.softDeleteMissingRemoteEventsWithConflictStateLocked(ctx, accountID, target, allEvents, remoteUIDs, protectedUIDs, pendingLocalChanges, reservedDetectedAt, deferredProjections)
	})
	return errorValue
}

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

func (service *Service) softDeleteMissingRemoteEvent(ctx context.Context, accountID string, target remoteCalendarTarget, event calendarEvent, pendingLocalChange pendingCalendarLocalChange, detectedAt time.Time) error {
	reservedDetectedAt, errorValue := service.reserveCalendarConflictCandidateTime(ctx, detectedAt)
	if errorValue != nil {
		return errorValue
	}
	deferredProjections := &calendarPullDeferredProjectionQueue{}
	errorValue = nil
	service.runCalendarPullLocked(ctx, deferredProjections, func() {
		errorValue = service.softDeleteMissingRemoteEventLocked(ctx, accountID, target, event, pendingLocalChange, reservedDetectedAt, deferredProjections)
	})
	return errorValue
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
		if errorValue := service.softDeleteCalendarEventForPullLocked(ctx, currentEvent.ID, deferredProjections); errorValue != nil {
			return errorValue
		}
		return service.deleteCalendarOutboxForEventUID(ctx, accountID, target.CalendarURL, currentEvent.UID)
	}
	return service.softDeleteCalendarEventForPullLocked(ctx, currentEvent.ID, deferredProjections)
}
