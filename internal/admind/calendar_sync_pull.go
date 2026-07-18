package admind

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	return service.pullCalendarChangesForProvider(ctx, googleCalendarProvider{}, false)
}

func (service *Service) pullCalendarChangesForProvider(ctx context.Context, provider calendarProvider, forceQuery bool) (bool, error) {
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
	changed, errorValue := service.runCalendarPull(ctx, provider, account, client, forceQuery)
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
	return service.runCalendarPull(ctx, googleCalendarProvider{}, account, client, false)
}

func (service *Service) runCalendarPull(ctx context.Context, provider calendarProvider, account remoteCalendarAccount, client calDAVPullClient, forceQuery bool) (bool, error) {
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
	reconciliationResult, isCurrentTarget, errorValue := service.applyCalendarPullCycleIfCurrent(ctx, account, target, objects, serverCTag, snapshotCompletedAt)
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

func (service *Service) applyCalendarPullCycleIfCurrent(ctx context.Context, account remoteCalendarAccount, target remoteCalendarTarget, objects []calDAVCalendarObject, serverCTag string, snapshotCompletedAt time.Time) (calendarPullReconciliationResult, bool, error) {
	deferredProjections := &calendarPullDeferredProjectionQueue{}
	var reconciliationResult calendarPullReconciliationResult
	var isCurrentTarget bool
	var errorValue error
	service.runCalendarPullLocked(ctx, deferredProjections, func() {
		reconciliationResult, isCurrentTarget, errorValue = service.applyCalendarPullCycleLocked(ctx, account, target, objects, serverCTag, snapshotCompletedAt, deferredProjections)
	})
	return reconciliationResult, isCurrentTarget, errorValue
}

func (service *Service) applyCalendarPullCycleLocked(ctx context.Context, account remoteCalendarAccount, target remoteCalendarTarget, objects []calDAVCalendarObject, serverCTag string, snapshotCompletedAt time.Time, deferredProjections *calendarPullDeferredProjectionQueue) (calendarPullReconciliationResult, bool, error) {
	isCurrentTarget, errorValue := service.calendarPullTargetIsCurrentLocked(ctx, account, target)
	if errorValue != nil || !isCurrentTarget {
		return calendarPullReconciliationResult{}, isCurrentTarget, errorValue
	}
	fencedUIDs, errorValue := service.listCalendarPushObservationFenceUIDs(ctx, account.ID, target.CalendarURL)
	if errorValue != nil {
		return calendarPullReconciliationResult{}, false, fmt.Errorf("list calendar push observation fences: %w", errorValue)
	}
	reconciliationResult, errorValue := service.reconcileCalendarPullSnapshotWithFencesLocked(ctx, account, objects, fencedUIDs, snapshotCompletedAt, deferredProjections)
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
		slog.WarnContext(ctx, "calendar ctag update failed", "account_id", account.ID, "calendar_url", target.CalendarURL, "error", errorValue)
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
