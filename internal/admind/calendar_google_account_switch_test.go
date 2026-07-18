package admind

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestSaveGoogleOAuthTokenAndAccountResetsDiscoveryForDifferentEmail(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	existing := remoteCalendarAccount{
		ID:                  googleOAuthAccountPrefix + "old_example_com",
		Provider:            remoteCalendarProviderGoogle,
		AccountEmail:        "old@example.com",
		PrincipalURL:        "https://apidata.googleusercontent.com/caldav/v2/old@example.com/user",
		HomeSetURL:          "https://apidata.googleusercontent.com/caldav/v2/old@example.com/",
		DefaultCalendarURL:  "https://apidata.googleusercontent.com/caldav/v2/old@example.com/events/",
		DefaultCalendarCTag: "old-ctag",
		TokenFilePath:       "/tmp/old-google-token.enc",
	}
	if _, errorValue := service.upsertRemoteCalendarAccount(ctx, existing); errorValue != nil {
		t.Fatalf("seed account: %v", errorValue)
	}
	token := &oauth2.Token{
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().UTC().Add(time.Hour),
	}

	account, errorValue := service.saveGoogleOAuthTokenAndAccount(ctx, token, " NEW@Example.COM ")
	if errorValue != nil {
		t.Fatalf("save account: %v", errorValue)
	}

	if account.AccountEmail != "new@example.com" {
		t.Fatalf("account email: got %q", account.AccountEmail)
	}
	if account.ID != newGoogleOAuthAccountID("new@example.com") {
		t.Fatalf("account id: got %q", account.ID)
	}
	if account.PrincipalURL != "" || account.HomeSetURL != "" || account.DefaultCalendarURL != "" || account.DefaultCalendarCTag != "" {
		t.Fatalf("discovery fields should reset for a different email: %+v", account)
	}
}

func TestDifferentGoogleEmailsWithSameSanitizedValueUseDifferentAccountIDs(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	firstAccount, errorValue := service.saveGoogleOAuthTokenAndAccount(ctx, &oauth2.Token{
		AccessToken: "collision-first-access-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().UTC().Add(time.Hour),
	}, "a.b@example.com")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	secondAccount, errorValue := service.saveGoogleOAuthTokenAndAccount(ctx, &oauth2.Token{
		AccessToken: "collision-second-access-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().UTC().Add(time.Hour),
	}, "a_b@example.com")
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if firstAccount.ID == secondAccount.ID || firstAccount.TokenFilePath == secondAccount.TokenFilePath {
		t.Fatalf("colliding accounts first=%+v second=%+v", firstAccount, secondAccount)
	}
	if _, errorValue := os.Stat(firstAccount.TokenFilePath); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("replaced first token file error=%v", errorValue)
	}
	storedAccount, found, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil || !found || storedAccount.ID != secondAccount.ID {
		t.Fatalf("stored account found=%v id=%q error=%v", found, storedAccount.ID, errorValue)
	}
}

func TestDifferentGoogleAccountSelectionBackfillsExistingEvents(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	existingEvent := newLocalTestCalendarEvent("existing-from-old-account", "Existing From Old Account")
	existingEvent.RemoteSource = remoteCalendarProviderGoogle
	existingEvent.RemoteHref = "/calendars/old-account/existing-from-old-account.ics"
	existingEvent.RemoteETag = `"old-etag"`
	if errorValue := service.writeCalendarEventWithSource(ctx, existingEvent, calendarSourcePull); errorValue != nil {
		t.Fatalf("write existing event: %v", errorValue)
	}
	oldToken := &oauth2.Token{
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().UTC().Add(time.Hour),
	}
	oldAccount, errorValue := service.saveGoogleOAuthTokenAndAccount(ctx, oldToken, "old@example.com")
	if errorValue != nil {
		t.Fatalf("save old account: %v", errorValue)
	}
	oldTokenPath := oldAccount.TokenFilePath
	oldAccount.SelectedCalendarID = "old@example.com"
	oldAccount.SelectedCalendarAccessRole = "writer"
	oldAccount.SelectedCalendarURL = "/calendars/old-account/"
	if _, errorValue := service.upsertRemoteCalendarAccount(ctx, oldAccount); errorValue != nil {
		t.Fatalf("update old account selection: %v", errorValue)
	}
	token := &oauth2.Token{
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().UTC().Add(time.Hour),
	}
	newAccount, errorValue := service.saveGoogleOAuthTokenAndAccount(ctx, token, "new@example.com")
	if errorValue != nil {
		t.Fatalf("save new account: %v", errorValue)
	}

	if _, errorValue := service.saveSelectedCalendar(ctx, newAccount, "company@example.com", "Company", "writer", "/calendars/new-company/", time.Now()); errorValue != nil {
		t.Fatalf("save new selected calendar: %v", errorValue)
	}

	rows, errorValue := service.listPendingCalendarOutbox(ctx, newAccount.ID)
	if errorValue != nil {
		t.Fatalf("list new account outbox: %v", errorValue)
	}
	if len(rows) != 1 {
		t.Fatalf("new account backfill rows: got %d, want 1", len(rows))
	}
	if rows[0].EventID != existingEvent.ID || rows[0].RemoteHref != "" || rows[0].IfMatchETag != "" {
		t.Fatalf("new account should add existing event as fresh put: %+v", rows[0])
	}
	accountEmails := readRemoteCalendarAccountEmailsForTest(t, service, ctx, remoteCalendarProviderGoogle)
	if len(accountEmails) != 1 || accountEmails[0] != "new@example.com" {
		t.Fatalf("google accounts = %#v, want only new@example.com", accountEmails)
	}
	if _, errorValue := os.Stat(oldTokenPath); !os.IsNotExist(errorValue) {
		t.Fatalf("old token file should be deleted, stat error = %v", errorValue)
	}
}

func TestSaveGoogleOAuthTokenAndAccountPreservesSelectedCalendarForSameEmail(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	existing := remoteCalendarAccount{
		ID:                         googleOAuthAccountPrefix + "User_example_com",
		Provider:                   remoteCalendarProviderGoogle,
		AccountEmail:               "user@example.com",
		PrincipalURL:               "https://apidata.googleusercontent.com/caldav/v2/user@example.com/user",
		HomeSetURL:                 "https://apidata.googleusercontent.com/caldav/v2/user@example.com/",
		DefaultCalendarURL:         "https://apidata.googleusercontent.com/caldav/v2/user@example.com/events/",
		DefaultCalendarCTag:        "existing-ctag",
		TokenFilePath:              service.calendarTokenFilePath(googleOAuthAccountPrefix + "User_example_com"),
		SelectedCalendarID:         "company@example.com",
		SelectedCalendarSummary:    "회사 일정",
		SelectedCalendarAccessRole: "writer",
		SelectedCalendarURL:        "https://apidata.googleusercontent.com/caldav/v2/user@example.com/company/events/",
		SelectedCalendarSelectedAt: "2026-07-01T10:00:00Z",
		InitialSyncCompletedAt:     "2026-07-01T10:05:00Z",
	}
	if _, errorValue := service.upsertRemoteCalendarAccount(ctx, existing); errorValue != nil {
		t.Fatalf("seed account: %v", errorValue)
	}
	token := &oauth2.Token{
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().UTC().Add(time.Hour),
	}

	account, errorValue := service.saveGoogleOAuthTokenAndAccount(ctx, token, "USER@example.com")
	if errorValue != nil {
		t.Fatalf("save account: %v", errorValue)
	}

	if account.ID != existing.ID {
		t.Errorf("ID: got %q, want %q", account.ID, existing.ID)
	}
	if account.AccountEmail != "user@example.com" {
		t.Errorf("AccountEmail: got %q", account.AccountEmail)
	}
	if account.TokenFilePath != existing.TokenFilePath {
		t.Errorf("TokenFilePath: got %q, want %q", account.TokenFilePath, existing.TokenFilePath)
	}
	if account.SelectedCalendarID != existing.SelectedCalendarID {
		t.Errorf("SelectedCalendarID: got %q, want %q", account.SelectedCalendarID, existing.SelectedCalendarID)
	}
	if account.SelectedCalendarSummary != existing.SelectedCalendarSummary {
		t.Errorf("SelectedCalendarSummary: got %q, want %q", account.SelectedCalendarSummary, existing.SelectedCalendarSummary)
	}
	if account.SelectedCalendarAccessRole != existing.SelectedCalendarAccessRole {
		t.Errorf("SelectedCalendarAccessRole: got %q, want %q", account.SelectedCalendarAccessRole, existing.SelectedCalendarAccessRole)
	}
	if account.SelectedCalendarURL != existing.SelectedCalendarURL {
		t.Errorf("SelectedCalendarURL: got %q, want %q", account.SelectedCalendarURL, existing.SelectedCalendarURL)
	}
	if account.SelectedCalendarSelectedAt != existing.SelectedCalendarSelectedAt {
		t.Errorf("SelectedCalendarSelectedAt: got %q, want %q", account.SelectedCalendarSelectedAt, existing.SelectedCalendarSelectedAt)
	}
	if account.InitialSyncCompletedAt != existing.InitialSyncCompletedAt {
		t.Errorf("InitialSyncCompletedAt: got %q, want %q", account.InitialSyncCompletedAt, existing.InitialSyncCompletedAt)
	}
}

func readRemoteCalendarAccountEmailsForTest(t *testing.T, service *Service, ctx context.Context, provider string) []string {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatalf("open calendar database: %v", errorValue)
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT account_email
FROM calendar_remote_accounts
WHERE provider = ?
ORDER BY account_email`, provider)
	if errorValue != nil {
		t.Fatalf("query calendar accounts: %v", errorValue)
	}
	defer rows.Close()
	emails := []string{}
	for rows.Next() {
		var email string
		if errorValue := rows.Scan(&email); errorValue != nil {
			t.Fatalf("scan calendar account email: %v", errorValue)
		}
		emails = append(emails, email)
	}
	if errorValue := rows.Err(); errorValue != nil {
		t.Fatalf("scan calendar account emails: %v", errorValue)
	}
	return emails
}
