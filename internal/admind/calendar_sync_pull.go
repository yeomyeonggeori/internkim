package admind

import (
	"context"
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
	if strings.TrimSpace(account.DefaultCalendarURL) == "" {
		discovered, errorValue := provider.Discover(ctx, service, account)
		if errorValue != nil {
			return false, errorValue
		}
		account = discovered
	}
	serverCTag, errorValue := client.fetchCalendarCTag(ctx, account.DefaultCalendarURL)
	if errorValue != nil {
		return false, fmt.Errorf("fetch ctag: %w", errorValue)
	}
	if !forceQuery && serverCTag != "" && serverCTag == account.DefaultCalendarCTag {
		return false, nil
	}
	objects, errorValue := client.queryAllCalendarEvents(ctx, account.DefaultCalendarURL)
	if errorValue != nil {
		return false, fmt.Errorf("query calendar: %w", errorValue)
	}
	if errorValue := service.reconcileGoogleCalendarPull(ctx, account, objects, protectedUIDs); errorValue != nil {
		return false, errorValue
	}
	if serverCTag != "" && serverCTag != account.DefaultCalendarCTag {
		account.DefaultCalendarCTag = serverCTag
		if _, errorValue := service.upsertRemoteCalendarAccount(ctx, account); errorValue != nil {
			log.Printf("update calendar ctag: %v", errorValue)
		}
	}
	return true, nil
}

func (service *Service) reconcileGoogleCalendarPull(ctx context.Context, account remoteCalendarAccount, remoteObjects []calDAVCalendarObject, protectedUIDs map[string]struct{}) error {
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
	remoteUIDs, errorValue := service.applyPulledRemoteEvents(ctx, account, remoteObjects, existingByUID)
	if errorValue != nil {
		return errorValue
	}
	service.markCalendarPushUIDsObserved(remoteUIDs)
	return service.softDeleteMissingRemoteEvents(ctx, account.ID, activeEvents, remoteUIDs, mergeCalendarProtectedUIDs(protectedUIDs, pendingLocalChanges))
}

func (service *Service) applyPulledRemoteEvents(ctx context.Context, account remoteCalendarAccount, remoteObjects []calDAVCalendarObject, existingByUID map[string]calendarEvent) (map[string]struct{}, error) {
	remoteUIDs := map[string]struct{}{}
	for _, object := range remoteObjects {
		event, errorValue := decodeRemoteCalendarObject(object, account.AccountEmail)
		if errorValue != nil {
			log.Printf("calendar pull decode failed for %s: %v", object.Path, errorValue)
			continue
		}
		remoteUIDs[event.UID] = struct{}{}
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
		return nil
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
	mergedEvent := mergeCalendarEventChanges(remoteEvent, localEvent, pendingLocalChange.ChangedFields)
	mergedEvent = preserveCalendarInternalParticipants(mergedEvent, localEvent, pendingLocalChange.ChangedFields)
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
	return service.updatePendingCalendarOutboxRemoteState(ctx, account.ID, mergedEvent.UID, remoteEvent.RemoteHref, remoteEvent.RemoteETag)
}

func mergeCalendarProtectedUIDs(protectedUIDs map[string]struct{}, pendingLocalChanges map[string]pendingCalendarLocalChange) map[string]struct{} {
	if len(pendingLocalChanges) == 0 {
		return protectedUIDs
	}
	result := map[string]struct{}{}
	for eventUID := range protectedUIDs {
		result[eventUID] = struct{}{}
	}
	for eventUID := range pendingLocalChanges {
		result[eventUID] = struct{}{}
	}
	return result
}

func (service *Service) softDeleteMissingRemoteEvents(ctx context.Context, accountID string, allEvents []calendarEvent, remoteUIDs map[string]struct{}, protectedUIDs map[string]struct{}) error {
	for _, event := range allEvents {
		if event.RemoteSource != remoteCalendarProviderGoogle {
			continue
		}
		if _, kept := remoteUIDs[event.UID]; kept {
			continue
		}
		if _, protected := protectedUIDs[event.UID]; protected {
			continue
		}
		if errorValue := service.softDeleteMissingRemoteEvent(ctx, accountID, event); errorValue != nil {
			return fmt.Errorf("soft delete %s: %w", event.ID, errorValue)
		}
	}
	return nil
}

func (service *Service) softDeleteMissingRemoteEvent(ctx context.Context, accountID string, event calendarEvent) error {
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
	if _, hasPendingLocalChange, errorValue := service.readPendingCalendarLocalChange(ctx, accountID, currentEvent.UID); errorValue != nil {
		return errorValue
	} else if hasPendingLocalChange {
		return nil
	}
	return service.softDeleteCalendarEventWithSourceLocked(ctx, currentEvent.ID, calendarSourcePull)
}
