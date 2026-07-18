package admind

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestResetGoogleOAuthAccountPreservesTokenWhenAccountDeleteFails(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account, errorValue := service.saveGoogleOAuthTokenAndAccount(ctx, &oauth2.Token{
		AccessToken:  "reset-access-token",
		RefreshToken: "reset-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().UTC().Add(time.Hour),
	}, "reset-failure@example.com")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = database.ExecContext(ctx, `
CREATE TRIGGER fail_google_oauth_account_reset
BEFORE DELETE ON calendar_remote_accounts
BEGIN
	SELECT RAISE(ABORT, 'forced google oauth account reset failure');
END`)
	database.Close()
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	errorValue = service.resetGoogleOAuthAccountConnection(ctx)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "forced google oauth account reset failure") {
		t.Fatalf("reset error=%v", errorValue)
	}
	if _, errorValue := os.Stat(account.TokenFilePath); errorValue != nil {
		t.Fatalf("token file must remain while account exists: %v", errorValue)
	}
	storedAccount, found, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil || !found || storedAccount.ID != account.ID {
		t.Fatalf("account found=%v id=%q error=%v", found, storedAccount.ID, errorValue)
	}
}

func TestResetGoogleOAuthAccountRetriesQuarantinedTokenCleanup(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account, errorValue := service.saveGoogleOAuthTokenAndAccount(ctx, &oauth2.Token{
		AccessToken:  "reset-cleanup-access-token",
		RefreshToken: "reset-cleanup-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().UTC().Add(time.Hour),
	}, "reset-cleanup@example.com")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	service.removeTokenQuarantineFile = func(path string) error {
		return errors.New("forced token quarantine removal failure")
	}
	t.Cleanup(func() {
		service.removeTokenQuarantineFile = nil
	})

	if errorValue := service.resetGoogleOAuthAccountConnection(ctx); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := os.Stat(account.TokenFilePath); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("original token file error=%v", errorValue)
	}
	quarantinePath := account.TokenFilePath + calendarTokenResetQuarantineSuffix
	if _, errorValue := os.Stat(quarantinePath); errorValue != nil {
		t.Fatalf("quarantined token file error=%v", errorValue)
	}
	if _, found, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle); errorValue != nil || found {
		t.Fatalf("account found=%v error=%v", found, errorValue)
	}

	service.removeTokenQuarantineFile = nil
	if errorValue := service.resetGoogleOAuthAccountConnection(ctx); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := os.Stat(quarantinePath); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("quarantine cleanup error=%v", errorValue)
	}
}

func TestGoogleOAuthStartupPromotesPendingTokenForActiveAccount(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	accountID := googleOAuthAccountPrefix + "pending_active_example_com"
	tokenPath := service.calendarTokenFilePath(accountID)
	pendingPath := tokenPath + calendarTokenAccountPendingSuffix
	key, errorValue := service.loadOrCreateCalendarTokenEncryptionKey()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := writeCalendarTokenFile(pendingPath, key, oauthTokenPayload{AccessToken: "pending-active-token"}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.upsertRemoteCalendarAccount(ctx, remoteCalendarAccount{
		ID:            accountID,
		Provider:      remoteCalendarProviderGoogle,
		AccountEmail:  "pending-active@example.com",
		TokenFilePath: tokenPath,
	}); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := service.recoverGoogleOAuthTokenResetState(ctx); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := os.Stat(tokenPath); errorValue != nil {
		t.Fatalf("active token file error=%v", errorValue)
	}
	if _, errorValue := os.Stat(pendingPath); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("pending token file error=%v", errorValue)
	}
	payload, errorValue := readCalendarTokenFile(tokenPath, key)
	if errorValue != nil || payload.AccessToken != "pending-active-token" {
		t.Fatalf("active token payload=%+v error=%v", payload, errorValue)
	}
}

func TestGoogleOAuthStartupDeletesPendingTokenWithoutAccount(t *testing.T) {
	service := newCalendarTestService(t)
	tokenPath := service.calendarTokenFilePath(googleOAuthAccountPrefix + "pending_orphan_example_com")
	pendingPath := tokenPath + calendarTokenAccountPendingSuffix
	key, errorValue := service.loadOrCreateCalendarTokenEncryptionKey()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := writeCalendarTokenFile(pendingPath, key, oauthTokenPayload{AccessToken: "orphan-token"}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := writeCalendarTokenFile(tokenPath, key, oauthTokenPayload{AccessToken: "orphan-token"}); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := service.recoverGoogleOAuthTokenResetState(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := os.Stat(pendingPath); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("pending token file error=%v", errorValue)
	}
	if _, errorValue := os.Stat(tokenPath); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("orphan active token file error=%v", errorValue)
	}
}

func TestGoogleOAuthStartupCleansCorruptedPendingTokenWhenActiveTokenIsValid(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account, errorValue := service.saveGoogleOAuthTokenAndAccount(ctx, &oauth2.Token{
		AccessToken: "valid-active-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().UTC().Add(time.Hour),
	}, "corrupted-pending@example.com")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	pendingPath := account.TokenFilePath + calendarTokenAccountPendingSuffix
	if errorValue := os.WriteFile(pendingPath, []byte("corrupted-token"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := service.recoverGoogleOAuthTokenResetState(ctx); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.loadGoogleOAuthTokenForAccount(account); errorValue != nil {
		t.Fatalf("active token error=%v", errorValue)
	}
	if _, errorValue := os.Stat(pendingPath); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("corrupted pending token cleanup error=%v", errorValue)
	}
}

func TestGoogleOAuthStartupRecoversCorruptedActiveTokenFromPendingToken(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account, errorValue := service.saveGoogleOAuthTokenAndAccount(ctx, &oauth2.Token{
		AccessToken: "old-active-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().UTC().Add(time.Hour),
	}, "corrupted-active@example.com")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	key, errorValue := service.loadOrCreateCalendarTokenEncryptionKey()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	pendingPath := account.TokenFilePath + calendarTokenAccountPendingSuffix
	if errorValue := writeCalendarTokenFile(pendingPath, key, oauthTokenPayload{AccessToken: "recovered-active-token"}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(account.TokenFilePath, []byte("corrupted-token"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := service.recoverGoogleOAuthTokenResetState(ctx); errorValue != nil {
		t.Fatal(errorValue)
	}
	payload, errorValue := readCalendarTokenFile(account.TokenFilePath, key)
	if errorValue != nil || payload.AccessToken != "recovered-active-token" {
		t.Fatalf("active token payload=%+v error=%v", payload, errorValue)
	}
	if _, errorValue := os.Stat(pendingPath); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("pending token cleanup error=%v", errorValue)
	}
}

func TestGoogleOAuthResetRestoresQuarantinedTokenForKeptAccount(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account, errorValue := service.saveGoogleOAuthTokenAndAccount(ctx, &oauth2.Token{
		AccessToken: "recovery-access-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().UTC().Add(time.Hour),
	}, "recovery@example.com")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	quarantinePath := account.TokenFilePath + calendarTokenResetQuarantineSuffix
	if errorValue := os.Rename(account.TokenFilePath, quarantinePath); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := service.recoverGoogleOAuthTokenResetState(ctx); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := os.Stat(account.TokenFilePath); errorValue != nil {
		t.Fatalf("restored token file error=%v", errorValue)
	}
	if _, errorValue := os.Stat(quarantinePath); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("quarantine file error=%v", errorValue)
	}
}

func TestGoogleOAuthResetDeletesCorruptedQuarantineWhenActiveTokenIsValid(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account, errorValue := service.saveGoogleOAuthTokenAndAccount(ctx, &oauth2.Token{
		AccessToken: "valid-current-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().UTC().Add(time.Hour),
	}, "corrupted-quarantine@example.com")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	quarantinePath := account.TokenFilePath + calendarTokenResetQuarantineSuffix
	if errorValue := os.WriteFile(quarantinePath, []byte("corrupted-token"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := service.recoverGoogleOAuthTokenResetState(ctx); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.loadGoogleOAuthTokenForAccount(account); errorValue != nil {
		t.Fatalf("active token error=%v", errorValue)
	}
	if _, errorValue := os.Stat(quarantinePath); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("corrupted quarantine cleanup error=%v", errorValue)
	}
}
