package admind

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

func (service *Service) resetGoogleOAuthAccountConnection(ctx context.Context) error {
	return service.resetGoogleOAuthAccountConnectionsExcept(ctx, "")
}

func (service *Service) resetGoogleOAuthAccountConnectionsExcept(ctx context.Context, keptAccountID string) error {
	accounts, errorValue := service.listRemoteCalendarAccountsByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		return fmt.Errorf("read google calendar accounts before reset: %w", errorValue)
	}
	for _, account := range accounts {
		if isKeptGoogleOAuthAccount(account, keptAccountID) {
			continue
		}
		if service.canDeleteCalendarTokenFile(account.TokenFilePath) {
			if errorValue := deleteCalendarTokenFile(account.TokenFilePath); errorValue != nil {
				return fmt.Errorf("delete google calendar token file for %s: %w", account.AccountEmail, errorValue)
			}
		}
	}
	for _, account := range accounts {
		if isKeptGoogleOAuthAccount(account, keptAccountID) {
			continue
		}
		if errorValue := service.deleteRemoteCalendarAccount(ctx, account.ID); errorValue != nil {
			return fmt.Errorf("delete google calendar account %s: %w", account.AccountEmail, errorValue)
		}
	}
	return nil
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
