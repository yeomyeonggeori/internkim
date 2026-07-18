package admind

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

func (service *Service) saveGoogleOAuthTokenAndAccount(ctx context.Context, token *oauth2.Token, email string) (remoteCalendarAccount, error) {
	unlock := lockGoogleOAuthAccountState(&service.calendarRemoteMutex, &service.calendarOAuthTokenMutex)
	defer unlock()
	if errorValue := service.recoverGoogleOAuthTokenFilesLocked(ctx); errorValue != nil {
		return remoteCalendarAccount{}, errorValue
	}
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	existingAccounts, errorValue := service.listRemoteCalendarAccountsByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		return remoteCalendarAccount{}, errorValue
	}
	existing, found := findRemoteCalendarAccountByEmail(existingAccounts, normalizedEmail)
	accountID := newGoogleOAuthAccountID(normalizedEmail)
	tokenPath := service.calendarTokenFilePath(accountID)
	if found {
		accountID = existing.ID
		if strings.TrimSpace(existing.TokenFilePath) != "" {
			tokenPath = existing.TokenFilePath
		}
	}
	if !service.canDeleteCalendarTokenFile(tokenPath) {
		return remoteCalendarAccount{}, fmt.Errorf("google calendar token path is outside the managed secrets directory")
	}
	key, errorValue := service.loadOrCreateCalendarTokenEncryptionKey()
	if errorValue != nil {
		return remoteCalendarAccount{}, errorValue
	}
	payload := oauthTokenPayloadFromOAuth2Token(token)
	pendingTokenPath := tokenPath + calendarTokenAccountPendingSuffix
	if errorValue := writeCalendarTokenFile(pendingTokenPath, key, payload); errorValue != nil {
		return remoteCalendarAccount{}, errorValue
	}
	if errorValue := validateCalendarTokenFile(pendingTokenPath, key); errorValue != nil {
		return remoteCalendarAccount{}, errors.Join(
			fmt.Errorf("validate pending google calendar token file: %w", errorValue),
			deleteCalendarTokenFile(pendingTokenPath),
		)
	}
	tokenBackupPath, errorValue := service.activatePendingGoogleOAuthToken(pendingTokenPath, tokenPath, key)
	if errorValue != nil {
		return remoteCalendarAccount{}, errors.Join(errorValue, deleteCalendarTokenFile(pendingTokenPath))
	}
	account := remoteCalendarAccount{
		ID:            accountID,
		Provider:      remoteCalendarProviderGoogle,
		AccountEmail:  normalizedEmail,
		TokenFilePath: tokenPath,
	}
	if found {
		account.PrincipalURL = existing.PrincipalURL
		account.HomeSetURL = existing.HomeSetURL
		account.DefaultCalendarURL = existing.DefaultCalendarURL
		account.DefaultCalendarCTag = existing.DefaultCalendarCTag
		account.SelectedCalendarID = existing.SelectedCalendarID
		account.SelectedCalendarSummary = existing.SelectedCalendarSummary
		account.SelectedCalendarAccessRole = existing.SelectedCalendarAccessRole
		account.SelectedCalendarURL = existing.SelectedCalendarURL
		account.SelectedCalendarSelectedAt = existing.SelectedCalendarSelectedAt
		account.InitialSyncCompletedAt = existing.InitialSyncCompletedAt
	}
	savedAccount, errorValue := service.replaceGoogleOAuthAccounts(ctx, account, existingAccounts)
	if errorValue != nil {
		return remoteCalendarAccount{}, service.rollbackGoogleOAuthTokenActivation(tokenPath, pendingTokenPath, tokenBackupPath, errorValue)
	}
	if errorValue := deleteCalendarTokenFile(pendingTokenPath); errorValue != nil {
		slog.WarnContext(ctx, "committed google calendar pending token cleanup deferred", "account_id", savedAccount.ID, "error", errorValue)
	}
	if tokenBackupPath != "" {
		if errorValue := service.removeCalendarTokenResetQuarantine(tokenBackupPath); errorValue != nil {
			slog.WarnContext(ctx, "replaced google calendar token backup cleanup deferred", "account_id", savedAccount.ID, "error", errorValue)
		}
	}
	return savedAccount, nil
}

func newGoogleOAuthAccountID(email string) string {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	digest := sha256.Sum256([]byte(normalizedEmail))
	return googleOAuthAccountPrefix + sanitizeCalendarSecretComponent(normalizedEmail) + "_" + fmt.Sprintf("%x", digest[:6])
}

func (service *Service) activatePendingGoogleOAuthToken(pendingTokenPath string, tokenPath string, key []byte) (string, error) {
	if !service.canDeleteCalendarTokenFile(tokenPath) {
		return "", fmt.Errorf("google calendar token path is outside the managed secrets directory")
	}
	backupPath, errorValue := service.quarantineCalendarTokenForReset(tokenPath)
	if errorValue != nil {
		return "", fmt.Errorf("back up current google calendar token file: %w", errorValue)
	}
	if errorValue := service.promotePendingGoogleOAuthToken(pendingTokenPath, tokenPath); errorValue != nil {
		return "", errors.Join(
			fmt.Errorf("activate google calendar token file: %w", errorValue),
			restoreCalendarTokenFromResetQuarantine(tokenPath, backupPath),
		)
	}
	if errorValue := validateCalendarTokenFile(tokenPath, key); errorValue != nil {
		return "", errors.Join(
			fmt.Errorf("validate active google calendar token file: %w", errorValue),
			deleteCalendarTokenFile(tokenPath),
			restoreCalendarTokenFromResetQuarantine(tokenPath, backupPath),
		)
	}
	return backupPath, nil
}

func (service *Service) rollbackGoogleOAuthTokenActivation(tokenPath string, pendingTokenPath string, backupPath string, cause error) error {
	rollbackError := deleteCalendarTokenFile(tokenPath)
	if errorValue := restoreCalendarTokenFromResetQuarantine(tokenPath, backupPath); errorValue != nil {
		rollbackError = errors.Join(rollbackError, fmt.Errorf("restore previous google calendar token file: %w", errorValue))
	}
	if errorValue := deleteCalendarTokenFile(pendingTokenPath); errorValue != nil {
		rollbackError = errors.Join(rollbackError, fmt.Errorf("delete pending google calendar token file: %w", errorValue))
	}
	return errors.Join(cause, rollbackError)
}

func findRemoteCalendarAccountByEmail(accounts []remoteCalendarAccount, email string) (remoteCalendarAccount, bool) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	for _, account := range accounts {
		if strings.EqualFold(strings.TrimSpace(account.AccountEmail), normalizedEmail) {
			return account, true
		}
	}
	return remoteCalendarAccount{}, false
}

func (service *Service) loadGoogleOAuthTokenForAccount(account remoteCalendarAccount) (*oauth2.Token, error) {
	key, errorValue := service.loadOrCreateCalendarTokenEncryptionKey()
	if errorValue != nil {
		return nil, errorValue
	}
	payload, errorValue := readCalendarTokenFile(account.TokenFilePath, key)
	if errorValue != nil {
		return nil, errorValue
	}
	return oauth2TokenFromPayload(payload), nil
}

func (service *Service) googleOAuthTokenSource(ctx context.Context, account remoteCalendarAccount, redirectURI string) (oauth2.TokenSource, error) {
	initialToken, errorValue := service.loadGoogleOAuthTokenForAccount(account)
	if errorValue != nil {
		return nil, errorValue
	}
	configuration, errorValue := service.buildGoogleOAuthConfig(redirectURI)
	if errorValue != nil {
		return nil, errorValue
	}
	clientCtx := context.WithValue(ctx, oauth2.HTTPClient, service.googleOAuthHTTPClient())
	baseSource := configuration.TokenSource(clientCtx, initialToken)
	return &persistingGoogleOAuthTokenSource{
		service: service,
		account: account,
		inner:   baseSource,
		latest:  initialToken,
	}, nil
}

type persistingGoogleOAuthTokenSource struct {
	service *Service
	account remoteCalendarAccount
	inner   oauth2.TokenSource
	latest  *oauth2.Token
	mutex   sync.Mutex
}

func (source *persistingGoogleOAuthTokenSource) Token() (*oauth2.Token, error) {
	source.mutex.Lock()
	defer source.mutex.Unlock()
	token, errorValue := source.inner.Token()
	if errorValue != nil {
		return nil, errorValue
	}
	if source.latest != nil && token.AccessToken == source.latest.AccessToken && token.Expiry.Equal(source.latest.Expiry) {
		return token, nil
	}
	source.service.calendarOAuthTokenMutex.Lock()
	defer source.service.calendarOAuthTokenMutex.Unlock()
	hasAccount, errorValue := source.service.hasGoogleOAuthAccount(context.Background(), source.account.ID)
	if errorValue != nil {
		return token, errorValue
	}
	if !hasAccount {
		return token, fmt.Errorf("google calendar account %s was removed before token persistence", source.account.ID)
	}
	key, errorValue := source.service.loadOrCreateCalendarTokenEncryptionKey()
	if errorValue != nil {
		return token, errorValue
	}
	payload := oauthTokenPayloadFromOAuth2Token(token)
	if errorValue := writeCalendarTokenFile(source.account.TokenFilePath, key, payload); errorValue != nil {
		return token, errorValue
	}
	source.latest = token
	return token, nil
}

func (service *Service) hasGoogleOAuthAccount(ctx context.Context, accountID string) (bool, error) {
	accounts, errorValue := service.listRemoteCalendarAccountsByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		return false, errorValue
	}
	for _, account := range accounts {
		if strings.TrimSpace(account.ID) == strings.TrimSpace(accountID) {
			return true, nil
		}
	}
	return false, nil
}

func oauthTokenPayloadFromOAuth2Token(token *oauth2.Token) oauthTokenPayload {
	expiresAt := ""
	if !token.Expiry.IsZero() {
		expiresAt = token.Expiry.UTC().Format(time.RFC3339Nano)
	}
	return oauthTokenPayload{
		AccessToken:      token.AccessToken,
		RefreshToken:     token.RefreshToken,
		TokenType:        token.TokenType,
		ExpiresAtRFC3339: expiresAt,
	}
}

func oauth2TokenFromPayload(payload oauthTokenPayload) *oauth2.Token {
	token := &oauth2.Token{
		AccessToken:  payload.AccessToken,
		RefreshToken: payload.RefreshToken,
		TokenType:    payload.TokenType,
	}
	if payload.ExpiresAtRFC3339 != "" {
		if parsed, errorValue := time.Parse(time.RFC3339Nano, payload.ExpiresAtRFC3339); errorValue == nil {
			token.Expiry = parsed
		}
	}
	return token
}
