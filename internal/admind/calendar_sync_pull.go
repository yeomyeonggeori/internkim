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
	remoteUIDs, errorValue := service.applyPulledRemoteEvents(ctx, account, remoteObjects, existingByUID)
	if errorValue != nil {
		return errorValue
	}
	service.markCalendarPushUIDsObserved(remoteUIDs)
	return service.softDeleteMissingRemoteEvents(ctx, activeEvents, remoteUIDs, protectedUIDs)
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
			event.ID = previous.ID
		} else {
			event.ID = randomHex(16)
		}
		if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
			return nil, fmt.Errorf("write pulled event %s: %w", event.UID, errorValue)
		}
	}
	return remoteUIDs, nil
}

func (service *Service) softDeleteMissingRemoteEvents(ctx context.Context, allEvents []calendarEvent, remoteUIDs map[string]struct{}, protectedUIDs map[string]struct{}) error {
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
		if errorValue := service.softDeleteCalendarEventWithSource(ctx, event.ID, calendarSourcePull); errorValue != nil {
			return fmt.Errorf("soft delete %s: %w", event.ID, errorValue)
		}
	}
	return nil
}
