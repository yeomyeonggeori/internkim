package admind

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
)

func (service *Service) resetGoogleOAuthAccountConnection(ctx context.Context) error {
	return service.resetGoogleOAuthAccountConnectionsExcept(ctx, "")
}

func (service *Service) recoverGoogleOAuthTokenResetState(ctx context.Context) error {
	service.calendarOAuthTokenMutex.Lock()
	defer service.calendarOAuthTokenMutex.Unlock()
	return service.recoverGoogleOAuthTokenFilesLocked(ctx)
}

func (service *Service) resetGoogleOAuthAccountConnectionsExcept(ctx context.Context, keptAccountID string) error {
	unlock := lockGoogleOAuthAccountState(&service.calendarRemoteMutex, &service.calendarOAuthTokenMutex)
	defer unlock()
	return service.resetGoogleOAuthAccountConnectionsExceptLocked(ctx, keptAccountID)
}

func (service *Service) resetGoogleOAuthAccountConnectionsExceptLocked(ctx context.Context, keptAccountID string) error {
	if errorValue := service.recoverGoogleOAuthTokenFilesLocked(ctx); errorValue != nil {
		return errorValue
	}
	accounts, errorValue := service.listRemoteCalendarAccountsByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		return fmt.Errorf("read google calendar accounts before reset: %w", errorValue)
	}
	for _, account := range accounts {
		if isKeptGoogleOAuthAccount(account, keptAccountID) {
			continue
		}
		quarantinePath, errorValue := service.quarantineCalendarTokenForReset(account.TokenFilePath)
		if errorValue != nil {
			return fmt.Errorf("quarantine google calendar token file for %s: %w", account.AccountEmail, errorValue)
		}
		if errorValue := service.deleteRemoteCalendarAccountLocked(ctx, account.ID); errorValue != nil {
			restoreError := restoreCalendarTokenFromResetQuarantine(account.TokenFilePath, quarantinePath)
			if restoreError != nil {
				return errors.Join(
					fmt.Errorf("delete google calendar account %s: %w", account.AccountEmail, errorValue),
					fmt.Errorf("restore google calendar token file for %s: %w", account.AccountEmail, restoreError),
				)
			}
			return fmt.Errorf("delete google calendar account %s: %w", account.AccountEmail, errorValue)
		}
		if quarantinePath != "" {
			if errorValue := service.removeCalendarTokenResetQuarantine(quarantinePath); errorValue != nil {
				slog.WarnContext(ctx, "quarantined google calendar token cleanup deferred", "account_id", account.ID, "error", errorValue)
			}
		}
	}
	return nil
}

func lockGoogleOAuthAccountState(remoteLock sync.Locker, tokenLock sync.Locker) func() {
	remoteLock.Lock()
	tokenLock.Lock()
	return func() {
		tokenLock.Unlock()
		remoteLock.Unlock()
	}
}

func isKeptGoogleOAuthAccount(account remoteCalendarAccount, keptAccountID string) bool {
	return strings.TrimSpace(keptAccountID) != "" && strings.TrimSpace(account.ID) == strings.TrimSpace(keptAccountID)
}

func (service *Service) canDeleteCalendarTokenFile(path string) bool {
	candidatePath := strings.TrimSpace(path)
	if candidatePath == "" || !strings.HasSuffix(candidatePath, calendarTokenFileExtension) {
		return false
	}
	secretsDirectory, errorValue := filepath.Abs(service.calendarSecretsDirectory())
	if errorValue != nil {
		return false
	}
	tokenPath, errorValue := filepath.Abs(candidatePath)
	if errorValue != nil {
		return false
	}
	relativePath, errorValue := filepath.Rel(secretsDirectory, tokenPath)
	if errorValue != nil {
		return false
	}
	if relativePath == "." {
		return false
	}
	return !strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) && relativePath != ".."
}
