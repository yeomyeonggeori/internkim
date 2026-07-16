package admind

import (
	"context"
	"strings"
	"time"
)

func (service *Service) saveCalendarDiscovery(ctx context.Context, account remoteCalendarAccount, principalURL string, homeSetURL string, defaultCalendarURL string) (remoteCalendarAccount, error) {
	account.PrincipalURL = strings.TrimSpace(principalURL)
	account.HomeSetURL = strings.TrimSpace(homeSetURL)
	account.DefaultCalendarURL = strings.TrimSpace(defaultCalendarURL)
	return service.upsertRemoteCalendarAccount(ctx, account)
}

func (service *Service) saveSelectedCalendar(ctx context.Context, account remoteCalendarAccount, calendarID string, summary string, accessRole string, calendarURL string, selectedAt time.Time) (remoteCalendarAccount, error) {
	service.calendarSwitchWaiters.Add(1)
	service.calendarRemoteMutex.Lock()
	service.calendarSwitchWaiters.Add(-1)
	defer func() {
		service.calendarRemoteMutex.Unlock()
		service.signalCalendarSyncWakeUp()
	}()
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
	selectedCalendarID := strings.TrimSpace(calendarID)
	selectedCalendarURL := strings.TrimSpace(calendarURL)
	isSameSelectedCalendar := strings.TrimSpace(account.SelectedCalendarID) == selectedCalendarID && strings.TrimSpace(account.SelectedCalendarURL) == selectedCalendarURL
	previousReadinessStatus := strings.TrimSpace(account.SelectedCalendarReadinessStatus)
	if !isSameSelectedCalendar {
		account.DefaultCalendarCTag = ""
		account.InitialSyncCompletedAt = ""
	}
	account.SelectedCalendarID = selectedCalendarID
	account.SelectedCalendarSummary = strings.TrimSpace(summary)
	account.SelectedCalendarAccessRole = strings.TrimSpace(accessRole)
	account.SelectedCalendarURL = selectedCalendarURL
	account.SelectedCalendarSelectedAt = selectedAt.UTC().Format(time.RFC3339Nano)
	account.SelectedCalendarReadinessStatus = calendarReadinessStatusInitialSyncPending
	if isSameSelectedCalendar && previousReadinessStatus == calendarReadinessStatusInitialExportPending {
		account.SelectedCalendarReadinessStatus = calendarReadinessStatusInitialExportPending
	}
	if strings.TrimSpace(account.InitialSyncCompletedAt) != "" {
		account.SelectedCalendarReadinessStatus = calendarReadinessStatusSyncReady
	}
	backfillCandidate := time.Now().UTC()
	if remoteCalendarAccountCanWrite(account) {
		reservedBackfillCandidate, errorValue := service.reserveCalendarConflictCandidateTime(ctx, backfillCandidate)
		if errorValue != nil {
			return remoteCalendarAccount{}, errorValue
		}
		backfillCandidate = reservedBackfillCandidate
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return remoteCalendarAccount{}, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return remoteCalendarAccount{}, errorValue
	}
	updated, errorValue := updateSelectedRemoteCalendarAccountWithRunner(ctx, transaction, account)
	if errorValue != nil {
		_ = transaction.Rollback()
		return remoteCalendarAccount{}, errorValue
	}
	shouldSignalSync := false
	if !remoteCalendarAccountCanWrite(updated) {
		if errorValue := transaction.Commit(); errorValue != nil {
			return remoteCalendarAccount{}, errorValue
		}
		return updated, nil
	}
	shouldSignalSync, errorValue = enqueueCalendarBackfillOutboxWithRunner(ctx, transaction, &service.calendarCandidateClock, updated, backfillCandidate)
	if errorValue != nil {
		_ = transaction.Rollback()
		return remoteCalendarAccount{}, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return remoteCalendarAccount{}, errorValue
	}
	if shouldSignalSync {
		service.signalCalendarSyncWakeUp()
	}
	return updated, nil
}

func (service *Service) saveCalendarPullState(ctx context.Context, account remoteCalendarAccount, calendarCTag string, completedAt time.Time, isInitialSyncCompleted bool) (remoteCalendarAccount, error) {
	account.DefaultCalendarCTag = strings.TrimSpace(calendarCTag)
	selectedTarget := selectedRemoteCalendarTarget(account)
	if isInitialSyncCompleted {
		hasPendingBackfill, errorValue := service.hasPendingCalendarPutsForTarget(ctx, account.ID, selectedTarget)
		if errorValue != nil {
			return remoteCalendarAccount{}, errorValue
		}
		if hasPendingBackfill {
			account.SelectedCalendarReadinessStatus = calendarReadinessStatusInitialExportPending
		} else {
			account.InitialSyncCompletedAt = completedAt.UTC().Format(time.RFC3339Nano)
			account.SelectedCalendarReadinessStatus = calendarReadinessStatusSyncReady
		}
		if selectedTarget.IsSelectedCalendar {
			return account, service.updateSelectedCalendarInitialSyncState(ctx, account)
		}
	}
	if selectedTarget.IsSelectedCalendar {
		return account, service.updateSelectedCalendarCTag(ctx, account)
	}
	return service.upsertRemoteCalendarAccount(ctx, account)
}

func (service *Service) updateSelectedCalendarCTag(ctx context.Context, account remoteCalendarAccount) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `
	UPDATE calendar_remote_accounts
	SET default_calendar_ctag = ?, updated_at = ?
	WHERE id = ?
		AND selected_calendar_id = ?
		AND selected_calendar_url = ?`,
		strings.TrimSpace(account.DefaultCalendarCTag),
		time.Now().UTC().Format(time.RFC3339Nano),
		strings.TrimSpace(account.ID),
		strings.TrimSpace(account.SelectedCalendarID),
		strings.TrimSpace(account.SelectedCalendarURL),
	)
	return errorValue
}

func (service *Service) updateSelectedCalendarInitialSyncState(ctx context.Context, account remoteCalendarAccount) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `
	UPDATE calendar_remote_accounts
	SET default_calendar_ctag = ?, selected_calendar_readiness_status = ?, initial_sync_completed_at = ?, updated_at = ?
	WHERE id = ?
		AND selected_calendar_id = ?
		AND selected_calendar_url = ?
		AND COALESCE(initial_sync_completed_at, '') = ''`,
		strings.TrimSpace(account.DefaultCalendarCTag),
		strings.TrimSpace(account.SelectedCalendarReadinessStatus),
		strings.TrimSpace(account.InitialSyncCompletedAt),
		time.Now().UTC().Format(time.RFC3339Nano),
		strings.TrimSpace(account.ID),
		strings.TrimSpace(account.SelectedCalendarID),
		strings.TrimSpace(account.SelectedCalendarURL),
	)
	return errorValue
}

func (service *Service) completeCalendarInitialSyncIfReady(ctx context.Context, account remoteCalendarAccount, completedAt time.Time) error {
	if strings.TrimSpace(account.InitialSyncCompletedAt) != "" {
		return nil
	}
	if strings.TrimSpace(account.SelectedCalendarReadinessStatus) != calendarReadinessStatusInitialExportPending {
		return nil
	}
	hasPendingBackfill, errorValue := service.hasPendingCalendarPutsForTarget(ctx, account.ID, selectedRemoteCalendarTarget(account))
	if errorValue != nil {
		return errorValue
	}
	if hasPendingBackfill {
		return nil
	}
	return service.markSelectedCalendarInitialSyncCompleted(ctx, account, completedAt)
}

func (service *Service) markSelectedCalendarInitialSyncCompleted(ctx context.Context, account remoteCalendarAccount, completedAt time.Time) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `
	UPDATE calendar_remote_accounts
	SET initial_sync_completed_at = ?, selected_calendar_readiness_status = ?, updated_at = ?
	WHERE id = ?
		AND selected_calendar_id = ?
		AND selected_calendar_url = ?
		AND selected_calendar_readiness_status = ?
		AND COALESCE(initial_sync_completed_at, '') = ''`,
		completedAt.UTC().Format(time.RFC3339Nano),
		calendarReadinessStatusSyncReady,
		time.Now().UTC().Format(time.RFC3339Nano),
		strings.TrimSpace(account.ID),
		strings.TrimSpace(account.SelectedCalendarID),
		strings.TrimSpace(account.SelectedCalendarURL),
		calendarReadinessStatusInitialExportPending,
	)
	return errorValue
}
