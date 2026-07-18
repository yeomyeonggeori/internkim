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

func TestSaveGoogleOAuthTokenRemovesNewTokenWhenPreviousAccountResetFails(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	if _, errorValue := service.saveGoogleOAuthTokenAndAccount(ctx, &oauth2.Token{
		AccessToken: "existing-access-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().UTC().Add(time.Hour),
	}, "existing@example.com"); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = database.ExecContext(ctx, `
CREATE TRIGGER fail_previous_google_account_reset
BEFORE DELETE ON calendar_remote_accounts
BEGIN
	SELECT RAISE(ABORT, 'forced previous account reset failure');
END`)
	database.Close()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	newTokenPath := service.calendarTokenFilePath(newGoogleOAuthAccountID("new@example.com"))
	newPendingTokenPath := newTokenPath + calendarTokenAccountPendingSuffix

	_, errorValue = service.saveGoogleOAuthTokenAndAccount(ctx, &oauth2.Token{
		AccessToken: "new-access-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().UTC().Add(time.Hour),
	}, "new@example.com")

	if errorValue == nil || !strings.Contains(errorValue.Error(), "forced previous account reset failure") {
		t.Fatalf("save error=%v", errorValue)
	}
	if _, errorValue := os.Stat(newTokenPath); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("new token file error=%v", errorValue)
	}
	if _, errorValue := os.Stat(newPendingTokenPath); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("new pending token file error=%v", errorValue)
	}
}

func TestSaveGoogleOAuthTokenKeepsPreviousAccountWhenTokenActivationFails(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	oldAccount, errorValue := service.saveGoogleOAuthTokenAndAccount(ctx, &oauth2.Token{
		AccessToken: "activation-failure-old-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().UTC().Add(time.Hour),
	}, "activation-failure-old@example.com")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	service.promoteCalendarTokenFile = func(string, string) error {
		return errors.New("forced token activation failure")
	}
	t.Cleanup(func() {
		service.promoteCalendarTokenFile = nil
	})
	newTokenPath := service.calendarTokenFilePath(newGoogleOAuthAccountID("activation-failure-new@example.com"))
	newPendingTokenPath := newTokenPath + calendarTokenAccountPendingSuffix

	_, errorValue = service.saveGoogleOAuthTokenAndAccount(ctx, &oauth2.Token{
		AccessToken: "activation-failure-new-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().UTC().Add(time.Hour),
	}, "activation-failure-new@example.com")

	if errorValue == nil || !strings.Contains(errorValue.Error(), "forced token activation failure") {
		t.Fatalf("save error=%v", errorValue)
	}
	storedAccount, found, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil || !found || storedAccount.ID != oldAccount.ID {
		t.Fatalf("old account found=%v id=%q error=%v", found, storedAccount.ID, errorValue)
	}
	if _, errorValue := os.Stat(oldAccount.TokenFilePath); errorValue != nil {
		t.Fatalf("old token file error=%v", errorValue)
	}
	if _, errorValue := os.Stat(newTokenPath); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("new token file error=%v", errorValue)
	}
	if _, errorValue := os.Stat(newPendingTokenPath); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("new pending token file error=%v", errorValue)
	}
}

func TestSaveGoogleOAuthTokenRemovesNewTokenWhenAccountUpsertFails(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	oldAccount, errorValue := service.saveGoogleOAuthTokenAndAccount(ctx, &oauth2.Token{
		AccessToken: "upsert-failure-old-access-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().UTC().Add(time.Hour),
	}, "upsert-failure-old@example.com")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = database.ExecContext(ctx, `
CREATE TRIGGER fail_google_account_upsert
BEFORE INSERT ON calendar_remote_accounts
BEGIN
	SELECT RAISE(ABORT, 'forced google account upsert failure');
END`)
	database.Close()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	tokenPath := service.calendarTokenFilePath(newGoogleOAuthAccountID("upsert-failure@example.com"))
	pendingTokenPath := tokenPath + calendarTokenAccountPendingSuffix

	_, errorValue = service.saveGoogleOAuthTokenAndAccount(ctx, &oauth2.Token{
		AccessToken: "upsert-failure-access-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().UTC().Add(time.Hour),
	}, "upsert-failure@example.com")

	if errorValue == nil || !strings.Contains(errorValue.Error(), "forced google account upsert failure") {
		t.Fatalf("save error=%v", errorValue)
	}
	if _, errorValue := os.Stat(tokenPath); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("new token file error=%v", errorValue)
	}
	if _, errorValue := os.Stat(pendingTokenPath); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("new pending token file error=%v", errorValue)
	}
	storedAccount, found, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil || !found || storedAccount.ID != oldAccount.ID {
		t.Fatalf("old account found=%v id=%q error=%v", found, storedAccount.ID, errorValue)
	}
	if _, errorValue := os.Stat(oldAccount.TokenFilePath); errorValue != nil {
		t.Fatalf("old token file error=%v", errorValue)
	}
}
