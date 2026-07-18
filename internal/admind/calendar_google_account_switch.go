package admind

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

type quarantinedCalendarToken struct {
	originalPath   string
	quarantinePath string
	account        remoteCalendarAccount
}

func (service *Service) replaceGoogleOAuthAccounts(ctx context.Context, account remoteCalendarAccount, existingAccounts []remoteCalendarAccount) (remoteCalendarAccount, error) {
	quarantinedTokens, errorValue := service.quarantineReplacedGoogleAccountTokens(existingAccounts, account.ID)
	if errorValue != nil {
		return remoteCalendarAccount{}, errorValue
	}
	savedAccount, errorValue := service.replaceGoogleOAuthAccountsInDatabase(ctx, account, existingAccounts)
	if errorValue != nil {
		return remoteCalendarAccount{}, errors.Join(errorValue, restoreQuarantinedCalendarTokens(quarantinedTokens))
	}
	service.removeReplacedGoogleAccountTokens(ctx, quarantinedTokens)
	return savedAccount, nil
}

func (service *Service) quarantineReplacedGoogleAccountTokens(accounts []remoteCalendarAccount, keptAccountID string) ([]quarantinedCalendarToken, error) {
	quarantinedTokens := []quarantinedCalendarToken{}
	for _, account := range accounts {
		if isKeptGoogleOAuthAccount(account, keptAccountID) {
			continue
		}
		quarantinePath, errorValue := service.quarantineCalendarTokenForReset(account.TokenFilePath)
		if errorValue != nil {
			return nil, errors.Join(errorValue, restoreQuarantinedCalendarTokens(quarantinedTokens))
		}
		quarantinedTokens = append(quarantinedTokens, quarantinedCalendarToken{
			originalPath:   account.TokenFilePath,
			quarantinePath: quarantinePath,
			account:        account,
		})
	}
	return quarantinedTokens, nil
}

func (service *Service) replaceGoogleOAuthAccountsInDatabase(ctx context.Context, account remoteCalendarAccount, existingAccounts []remoteCalendarAccount) (remoteCalendarAccount, error) {
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return remoteCalendarAccount{}, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return remoteCalendarAccount{}, errorValue
	}
	for _, existingAccount := range existingAccounts {
		if isKeptGoogleOAuthAccount(existingAccount, account.ID) {
			continue
		}
		if errorValue := deleteRemoteCalendarAccountWithRunner(ctx, transaction, existingAccount.ID); errorValue != nil {
			_ = transaction.Rollback()
			return remoteCalendarAccount{}, fmt.Errorf("delete replaced google calendar account %s: %w", existingAccount.AccountEmail, errorValue)
		}
	}
	savedAccount, errorValue := upsertRemoteCalendarAccountWithRunner(ctx, transaction, account)
	if errorValue != nil {
		_ = transaction.Rollback()
		return remoteCalendarAccount{}, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return remoteCalendarAccount{}, errorValue
	}
	return savedAccount, nil
}

func restoreQuarantinedCalendarTokens(tokens []quarantinedCalendarToken) error {
	var restoreError error
	for _, token := range tokens {
		if errorValue := restoreCalendarTokenFromResetQuarantine(token.originalPath, token.quarantinePath); errorValue != nil {
			restoreError = errors.Join(restoreError, fmt.Errorf("restore google calendar token file for %s: %w", token.account.AccountEmail, errorValue))
		}
	}
	return restoreError
}

func (service *Service) removeReplacedGoogleAccountTokens(ctx context.Context, tokens []quarantinedCalendarToken) {
	for _, token := range tokens {
		if token.quarantinePath == "" {
			continue
		}
		if errorValue := service.removeCalendarTokenResetQuarantine(token.quarantinePath); errorValue != nil {
			slog.WarnContext(ctx, "replaced google calendar token cleanup deferred", "account_id", token.account.ID, "error", errorValue)
		}
	}
}
