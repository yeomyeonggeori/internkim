package admind

import (
	"context"
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
	if account.ID != googleOAuthAccountPrefix+"new_example_com" {
		t.Fatalf("account id: got %q", account.ID)
	}
	if account.PrincipalURL != "" || account.HomeSetURL != "" || account.DefaultCalendarURL != "" || account.DefaultCalendarCTag != "" {
		t.Fatalf("discovery fields should reset for a different email: %+v", account)
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
		TokenFilePath:              "/tmp/existing-google-token.enc",
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
