package admind

import (
	"context"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

func (service *Service) saveGoogleOAuthTokenAndAccount(ctx context.Context, token *oauth2.Token, email string) (remoteCalendarAccount, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	existing, found, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		return remoteCalendarAccount{}, errorValue
	}
	accountID := googleOAuthAccountPrefix + sanitizeCalendarSecretComponent(normalizedEmail)
	tokenPath := service.calendarTokenFilePath(accountID)
	if found && strings.EqualFold(strings.TrimSpace(existing.AccountEmail), normalizedEmail) {
		accountID = existing.ID
		if strings.TrimSpace(existing.TokenFilePath) != "" {
			tokenPath = existing.TokenFilePath
		}
	}
	key, errorValue := service.loadOrCreateCalendarTokenEncryptionKey()
	if errorValue != nil {
		return remoteCalendarAccount{}, errorValue
	}
	payload := oauthTokenPayloadFromOAuth2Token(token)
	if errorValue := writeCalendarTokenFile(tokenPath, key, payload); errorValue != nil {
		return remoteCalendarAccount{}, errorValue
	}
	account := remoteCalendarAccount{
		ID:            accountID,
		Provider:      remoteCalendarProviderGoogle,
		AccountEmail:  normalizedEmail,
		TokenFilePath: tokenPath,
	}
	if found && strings.EqualFold(strings.TrimSpace(existing.AccountEmail), normalizedEmail) {
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
	return service.upsertRemoteCalendarAccount(ctx, account)
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
}

func (source *persistingGoogleOAuthTokenSource) Token() (*oauth2.Token, error) {
	token, errorValue := source.inner.Token()
	if errorValue != nil {
		return nil, errorValue
	}
	if source.latest != nil && token.AccessToken == source.latest.AccessToken && token.Expiry.Equal(source.latest.Expiry) {
		return token, nil
	}
	source.latest = token
	key, errorValue := source.service.loadOrCreateCalendarTokenEncryptionKey()
	if errorValue != nil {
		return token, errorValue
	}
	payload := oauthTokenPayloadFromOAuth2Token(token)
	if errorValue := writeCalendarTokenFile(source.account.TokenFilePath, key, payload); errorValue != nil {
		return token, errorValue
	}
	return token, nil
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
