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
	if !forceQuery && serverCTag != "" && serverCTag == target.CalendarCTag && !target.NeedsInitialSyncCompletion {
		if errorValue := service.markCalendarRemoteTargetLastSeen(ctx, account, target, time.Now().UTC()); errorValue != nil {
			return false, errorValue
		}
		return false, nil
	}
	objects, errorValue := client.queryAllCalendarEvents(ctx, target.CalendarURL)
	if errorValue != nil {
		return false, fmt.Errorf("query calendar: %w", errorValue)
	}
	if errorValue := service.reconcileGoogleCalendarPull(ctx, account, objects, protectedUIDs); errorValue != nil {
		return false, errorValue
	}
	shouldSaveCTag := serverCTag != "" && serverCTag != target.CalendarCTag
	if shouldSaveCTag || target.NeedsInitialSyncCompletion {
		nextCTag := target.CalendarCTag
		if serverCTag != "" {
			nextCTag = serverCTag
		}
		if _, errorValue := service.saveCalendarPullState(ctx, account, nextCTag, time.Now(), target.NeedsInitialSyncCompletion); errorValue != nil {
			if target.NeedsInitialSyncCompletion {
				return false, fmt.Errorf("update calendar pull state: %w", errorValue)
			}
			log.Printf("update calendar ctag: %v", errorValue)
		}
	}
	return true, nil
}

func (service *Service) reconcileGoogleCalendarPull(ctx context.Context, account remoteCalendarAccount, remoteObjects []calDAVCalendarObject, protectedUIDs map[string]struct{}) error {
	target := activeRemoteCalendarTarget(account)
	observedAt := time.Now().UTC()
	activeEvents, errorValue := service.readCalendarEvents(ctx, time.Time{}, time.Time{})
	if errorValue != nil {
		return errorValue
	}
	softDeletedEvents, errorValue := service.readSoftDeletedCalendarEvents(ctx)
	if errorValue != nil {
		return errorValue
	}
	existingByUID := map[string]calendarEvent{}
	for _, event := range activeEvents {
		existingByUID[event.UID] = event
	}
	for _, event := range softDeletedEvents {
		if _, alreadyActive := existingByUID[event.UID]; alreadyActive {
			continue
		}
		existingByUID[event.UID] = event
	}
	pendingLocalChanges, errorValue := service.listPendingCalendarLocalChanges(ctx, account.ID)
	if errorValue != nil {
		return errorValue
	}
	remoteUIDs, errorValue := service.applyPulledRemoteEventsAt(ctx, account, target, remoteObjects, existingByUID, observedAt)
	if errorValue != nil {
		return errorValue
	}
	service.markCalendarPushUIDsObserved(remoteUIDs)
	if target.NeedsInitialSyncCompletion {
		return nil
	}
	return service.softDeleteMissingRemoteEventsWithConflictState(ctx, account.ID, target, activeEvents, remoteUIDs, protectedUIDs, pendingLocalChanges, observedAt)
}

func (service *Service) applyPulledRemoteEvents(ctx context.Context, account remoteCalendarAccount, remoteObjects []calDAVCalendarObject, existingByUID map[string]calendarEvent) (map[string]struct{}, error) {
	return service.applyPulledRemoteEventsAt(ctx, account, activeRemoteCalendarTarget(account), remoteObjects, existingByUID, time.Now().UTC())
}

func (service *Service) applyPulledRemoteEventsAt(ctx context.Context, account remoteCalendarAccount, target remoteCalendarTarget, remoteObjects []calDAVCalendarObject, existingByUID map[string]calendarEvent, observedAt time.Time) (map[string]struct{}, error) {
	remoteUIDs := map[string]struct{}{}
	for _, object := range remoteObjects {
		event, errorValue := decodeRemoteCalendarObject(object, account.AccountEmail)
		if errorValue != nil {
			log.Printf("calendar pull decode failed for %s: %v", object.Path, errorValue)
			continue
		}
		remoteUIDs[event.UID] = struct{}{}
		if errorValue := service.markCalendarRemoteEventObserved(ctx, account.ID, target.CalendarURL, event, observedAt); errorValue != nil {
			return nil, errorValue
		}
		previous, found := existingByUID[event.UID]
		if found {
			if previous.RemoteETag == event.RemoteETag && previous.RemoteETag != "" {
				continue
			}
		}
		if errorValue := service.applyPulledRemoteEvent(ctx, account, previous, found, event); errorValue != nil {
			return nil, errorValue
		}
	}
	return remoteUIDs, nil
}

func (service *Service) applyPulledRemoteEvent(ctx context.Context, account remoteCalendarAccount, previousEvent calendarEvent, hasPreviousEvent bool, remoteEvent calendarEvent) error {
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
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
	hasPendingLocalDelete, errorValue := service.hasPendingCalendarLocalDelete(ctx, account.ID, remoteEvent.UID)
	if errorValue != nil {
		return errorValue
	}
	if hasPendingLocalDelete {
		return service.reconcilePulledRemoteEventWithPendingLocalDeleteLocked(ctx, account, currentEvent, remoteEvent)
	}
	if hasCurrentEvent {
		if currentEvent.RemoteETag == remoteEvent.RemoteETag && currentEvent.RemoteETag != "" {
			return nil
		}
		remoteEvent.ID = currentEvent.ID
		pendingLocalChange, hasPendingLocalChange, errorValue := service.readPendingCalendarLocalChange(ctx, account.ID, remoteEvent.UID)
		if errorValue != nil {
			return errorValue
		}
		if hasPendingLocalChange {
			return service.applyPulledRemoteEventWithPendingLocalChangeLocked(ctx, account, currentEvent, remoteEvent, pendingLocalChange)
		}
		remoteEvent = preserveCalendarInternalParticipants(remoteEvent, currentEvent, nil)
	} else {
		remoteEvent.ID = randomHex(16)
	}
	if errorValue := service.writeCalendarEventWithSourceLocked(ctx, remoteEvent, calendarSourcePull); errorValue != nil {
		return fmt.Errorf("write pulled event %s: %w", remoteEvent.UID, errorValue)
	}
	return nil
}

func (service *Service) applyPulledRemoteEventWithPendingLocalChangeLocked(ctx context.Context, account remoteCalendarAccount, localEvent calendarEvent, remoteEvent calendarEvent, pendingLocalChange pendingCalendarLocalChange) error {
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
	if errorValue := service.writeCalendarEventWithSourceLocked(ctx, mergedEvent, calendarSourcePull); errorValue != nil {
		return fmt.Errorf("write pulled pending local event %s: %w", mergedEvent.UID, errorValue)
	}
	return service.retainPendingCalendarOutboxFields(ctx, account.ID, mergedEvent.UID, localWinningFields, remoteEvent.RemoteHref, remoteEvent.RemoteETag)
}

func (service *Service) softDeleteMissingRemoteEvents(ctx context.Context, accountID string, target remoteCalendarTarget, allEvents []calendarEvent, remoteUIDs map[string]struct{}, protectedUIDs map[string]struct{}) error {
	pendingLocalChanges, errorValue := service.listPendingCalendarLocalChanges(ctx, accountID)
	if errorValue != nil {
		return errorValue
	}
	return service.softDeleteMissingRemoteEventsWithConflictState(ctx, accountID, target, allEvents, remoteUIDs, protectedUIDs, pendingLocalChanges, time.Now().UTC())
}

func (service *Service) softDeleteMissingRemoteEventsWithConflictState(ctx context.Context, accountID string, target remoteCalendarTarget, allEvents []calendarEvent, remoteUIDs map[string]struct{}, protectedUIDs map[string]struct{}, pendingLocalChanges map[string]pendingCalendarLocalChange, detectedAt time.Time) error {
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
		if errorValue := service.softDeleteMissingRemoteEvent(ctx, accountID, target, event, pendingLocalChanges[event.UID], detectedAt); errorValue != nil {
			return fmt.Errorf("soft delete %s: %w", event.ID, errorValue)
		}
	}
	return nil
}

func (service *Service) softDeleteMissingRemoteEvent(ctx context.Context, accountID string, target remoteCalendarTarget, event calendarEvent, pendingLocalChange pendingCalendarLocalChange, detectedAt time.Time) error {
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
	currentEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil {
		return errorValue
	}
	if !found || currentEvent.RemoteSource != remoteCalendarProviderGoogle {
		return nil
	}
	if hasPendingLocalDelete, errorValue := service.hasPendingCalendarLocalDelete(ctx, accountID, currentEvent.UID); errorValue != nil {
		return errorValue
	} else if hasPendingLocalDelete {
		return nil
	}
	refreshedPendingLocalChange, hasPendingLocalChange, errorValue := service.readPendingCalendarLocalChange(ctx, accountID, currentEvent.UID)
	if errorValue != nil {
		return errorValue
	}
	if hasPendingLocalChange {
		pendingLocalChange = refreshedPendingLocalChange
	} else {
		pendingLocalChange = pendingCalendarLocalChange{}
	}
	remoteState, errorValue := service.markCalendarRemoteEventMissing(ctx, accountID, target.CalendarURL, currentEvent.UID, detectedAt)
	if errorValue != nil {
		return errorValue
	}
	if len(pendingLocalChange.ChangedFields) > 0 {
		winner := resolveCalendarRemoteDeletion(
			latestCalendarPendingChangeAt(pendingLocalChange),
			parseCalendarConflictTime(remoteState.LastSeenAt),
			parseCalendarConflictTime(remoteState.MissingDetectedAt),
		)
		if winner == calendarConflictWinnerLocal {
			return service.updatePendingCalendarOutboxRemoteState(ctx, accountID, currentEvent.UID, "", "")
		}
		if errorValue := service.softDeleteCalendarEventWithSourceLocked(ctx, currentEvent.ID, calendarSourcePull); errorValue != nil {
			return errorValue
		}
		return service.deleteCalendarOutboxForEventUID(ctx, accountID, currentEvent.UID)
	}
	return service.softDeleteCalendarEventWithSourceLocked(ctx, currentEvent.ID, calendarSourcePull)
}
