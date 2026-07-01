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
	selectedCalendarID := strings.TrimSpace(calendarID)
	selectedCalendarURL := strings.TrimSpace(calendarURL)
	isSameSelectedCalendar := strings.TrimSpace(account.SelectedCalendarID) == selectedCalendarID && strings.TrimSpace(account.SelectedCalendarURL) == selectedCalendarURL
	if !isSameSelectedCalendar {
		account.DefaultCalendarCTag = ""
		account.InitialSyncCompletedAt = ""
	}
	account.SelectedCalendarID = selectedCalendarID
	account.SelectedCalendarSummary = strings.TrimSpace(summary)
	account.SelectedCalendarAccessRole = strings.TrimSpace(accessRole)
	account.SelectedCalendarURL = selectedCalendarURL
	account.SelectedCalendarSelectedAt = selectedAt.UTC().Format(time.RFC3339Nano)
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return remoteCalendarAccount{}, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return remoteCalendarAccount{}, errorValue
	}
	updated, errorValue := upsertRemoteCalendarAccountWithRunner(ctx, transaction, account)
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
	shouldSignalSync, errorValue = enqueueCalendarBackfillOutboxWithRunner(ctx, transaction, updated)
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
	if isInitialSyncCompleted {
		account.InitialSyncCompletedAt = completedAt.UTC().Format(time.RFC3339Nano)
	}
	return service.upsertRemoteCalendarAccount(ctx, account)
}
