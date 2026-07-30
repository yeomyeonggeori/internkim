package admind

import (
	"context"
	"testing"
	"time"
)

func TestRemoteCalendarAccountCRUD(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := remoteCalendarAccount{
		ID:                         "account-google-1",
		Provider:                   remoteCalendarProviderGoogle,
		AccountEmail:               "user@example.com",
		PrincipalURL:               "https://apidata.googleusercontent.com/caldav/v2/user@example.com/user",
		HomeSetURL:                 "https://apidata.googleusercontent.com/caldav/v2/user@example.com/",
		DefaultCalendarURL:         "https://apidata.googleusercontent.com/caldav/v2/user@example.com/events/",
		DefaultCalendarCTag:        "ctag-initial",
		TokenFilePath:              "/tmp/example.token.enc",
		SelectedCalendarID:         "company@example.com",
		SelectedCalendarSummary:    "회사 일정",
		SelectedCalendarAccessRole: "writer",
		SelectedCalendarURL:        "https://apidata.googleusercontent.com/caldav/v2/user@example.com/company/events/",
		SelectedCalendarSelectedAt: "2026-07-01T10:00:00Z",
		InitialSyncCompletedAt:     "2026-07-01T10:05:00Z",
	}
	saved, errorValue := service.upsertRemoteCalendarAccount(ctx, account)
	if errorValue != nil {
		t.Fatalf("upsert insert: %v", errorValue)
	}
	if saved.CreatedAt == "" || saved.UpdatedAt == "" {
		t.Fatalf("expected timestamps populated, got %+v", saved)
	}

	loaded, found, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("read: %v", errorValue)
	}
	if !found {
		t.Fatal("expected account found")
	}
	if loaded.AccountEmail != account.AccountEmail {
		t.Errorf("AccountEmail: got %q, want %q", loaded.AccountEmail, account.AccountEmail)
	}
	if loaded.DefaultCalendarCTag != "ctag-initial" {
		t.Errorf("DefaultCalendarCTag: got %q, want %q", loaded.DefaultCalendarCTag, "ctag-initial")
	}
	if loaded.SelectedCalendarID != account.SelectedCalendarID {
		t.Errorf("SelectedCalendarID: got %q, want %q", loaded.SelectedCalendarID, account.SelectedCalendarID)
	}
	if loaded.SelectedCalendarSummary != account.SelectedCalendarSummary {
		t.Errorf("SelectedCalendarSummary: got %q, want %q", loaded.SelectedCalendarSummary, account.SelectedCalendarSummary)
	}
	if loaded.SelectedCalendarAccessRole != account.SelectedCalendarAccessRole {
		t.Errorf("SelectedCalendarAccessRole: got %q, want %q", loaded.SelectedCalendarAccessRole, account.SelectedCalendarAccessRole)
	}
	if loaded.SelectedCalendarURL != account.SelectedCalendarURL {
		t.Errorf("SelectedCalendarURL: got %q, want %q", loaded.SelectedCalendarURL, account.SelectedCalendarURL)
	}
	if loaded.SelectedCalendarSelectedAt != account.SelectedCalendarSelectedAt {
		t.Errorf("SelectedCalendarSelectedAt: got %q, want %q", loaded.SelectedCalendarSelectedAt, account.SelectedCalendarSelectedAt)
	}
	if loaded.InitialSyncCompletedAt != account.InitialSyncCompletedAt {
		t.Errorf("InitialSyncCompletedAt: got %q, want %q", loaded.InitialSyncCompletedAt, account.InitialSyncCompletedAt)
	}

	if _, errorValue := service.saveCalendarPullState(ctx, loaded, "ctag-updated", time.Time{}, false); errorValue != nil {
		t.Fatalf("save ctag: %v", errorValue)
	}
	reloaded, _, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("reload: %v", errorValue)
	}
	if reloaded.DefaultCalendarCTag != "ctag-updated" {
		t.Errorf("ctag not updated: got %q", reloaded.DefaultCalendarCTag)
	}

	if errorValue := service.deleteRemoteCalendarAccount(ctx, account.ID); errorValue != nil {
		t.Fatalf("delete: %v", errorValue)
	}
	_, found, errorValue = service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("read after delete: %v", errorValue)
	}
	if found {
		t.Fatal("expected account not found after delete")
	}
}

func TestSaveSelectedCalendarResetsPullStateWhenCalendarChanges(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := remoteCalendarAccount{
		ID:                     "account-google-1",
		Provider:               remoteCalendarProviderGoogle,
		AccountEmail:           "user@example.com",
		DefaultCalendarURL:     "/calendars/default/",
		DefaultCalendarCTag:    "old-selected-ctag",
		SelectedCalendarID:     "old-company@example.com",
		SelectedCalendarURL:    "/calendars/old-company/",
		InitialSyncCompletedAt: "2026-07-01T10:05:00Z",
	}
	saved, errorValue := service.upsertRemoteCalendarAccount(ctx, account)
	if errorValue != nil {
		t.Fatalf("seed account: %v", errorValue)
	}

	updated, errorValue := service.saveSelectedCalendar(ctx, saved, "new-company@example.com", "New Company", "writer", "/calendars/new-company/", time.Now())
	if errorValue != nil {
		t.Fatalf("save selected calendar: %v", errorValue)
	}
	if updated.DefaultCalendarCTag != "" {
		t.Errorf("DefaultCalendarCTag should reset when selected calendar changes: %q", updated.DefaultCalendarCTag)
	}
	if updated.InitialSyncCompletedAt != "" {
		t.Errorf("InitialSyncCompletedAt should reset when selected calendar changes: %q", updated.InitialSyncCompletedAt)
	}
}

func TestInitialSyncStateDoesNotOverwriteChangedSelection(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account, errorValue := service.upsertRemoteCalendarAccount(ctx, remoteCalendarAccount{
		ID:                              "account-google-1",
		Provider:                        remoteCalendarProviderGoogle,
		AccountEmail:                    "user@example.com",
		DefaultCalendarURL:              "/calendars/default/",
		SelectedCalendarID:              "company@example.com",
		SelectedCalendarSummary:         "Company",
		SelectedCalendarAccessRole:      "writer",
		SelectedCalendarURL:             "/calendars/company/",
		SelectedCalendarReadinessStatus: calendarReadinessStatusInitialExportPending,
	})
	if errorValue != nil {
		t.Fatalf("seed account: %v", errorValue)
	}
	if _, errorValue := service.saveSelectedCalendar(ctx, account, "new-company@example.com", "New Company", "writer", "/calendars/new-company/", time.Now()); errorValue != nil {
		t.Fatalf("change selection: %v", errorValue)
	}
	if _, errorValue := service.saveCalendarPullState(ctx, account, `"old-selected-ctag"`, time.Now(), true); errorValue != nil {
		t.Fatalf("save stale pull state: %v", errorValue)
	}
	if errorValue := service.completeCalendarInitialSyncIfReady(ctx, account, time.Now()); errorValue != nil {
		t.Fatalf("complete stale initial sync: %v", errorValue)
	}
	loaded, found, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("read account: %v", errorValue)
	}
	if !found {
		t.Fatal("account should exist")
	}
	if loaded.SelectedCalendarID != "new-company@example.com" {
		t.Errorf("SelectedCalendarID: got %q", loaded.SelectedCalendarID)
	}
	if loaded.SelectedCalendarURL != "/calendars/new-company/" {
		t.Errorf("SelectedCalendarURL: got %q", loaded.SelectedCalendarURL)
	}
	if loaded.DefaultCalendarCTag != "" {
		t.Errorf("DefaultCalendarCTag should not be set by stale pull: %q", loaded.DefaultCalendarCTag)
	}
	if loaded.InitialSyncCompletedAt != "" {
		t.Errorf("InitialSyncCompletedAt should not be set by stale completion: %q", loaded.InitialSyncCompletedAt)
	}
	if loaded.SelectedCalendarReadinessStatus != calendarReadinessStatusInitialSyncPending {
		t.Errorf("SelectedCalendarReadinessStatus: got %q", loaded.SelectedCalendarReadinessStatus)
	}
}

func TestSelectedCalendarPullStateDoesNotOverwriteChangedSelection(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account, errorValue := service.upsertRemoteCalendarAccount(ctx, remoteCalendarAccount{
		ID:                              "account-google-1",
		Provider:                        remoteCalendarProviderGoogle,
		AccountEmail:                    "user@example.com",
		DefaultCalendarURL:              "/calendars/default/",
		DefaultCalendarCTag:             `"old-selected-ctag"`,
		SelectedCalendarID:              "company@example.com",
		SelectedCalendarSummary:         "Company",
		SelectedCalendarAccessRole:      "writer",
		SelectedCalendarURL:             "/calendars/company/",
		SelectedCalendarReadinessStatus: calendarReadinessStatusSyncReady,
		InitialSyncCompletedAt:          "2026-07-01T10:05:00Z",
	})
	if errorValue != nil {
		t.Fatalf("seed account: %v", errorValue)
	}
	if _, errorValue := service.saveSelectedCalendar(ctx, account, "new-company@example.com", "New Company", "writer", "/calendars/new-company/", time.Now()); errorValue != nil {
		t.Fatalf("change selection: %v", errorValue)
	}
	if _, errorValue := service.saveCalendarPullState(ctx, account, `"stale-selected-ctag"`, time.Now(), false); errorValue != nil {
		t.Fatalf("save stale pull state: %v", errorValue)
	}
	loaded, found, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("read account: %v", errorValue)
	}
	if !found {
		t.Fatal("account should exist")
	}
	if loaded.SelectedCalendarID != "new-company@example.com" {
		t.Errorf("SelectedCalendarID: got %q", loaded.SelectedCalendarID)
	}
	if loaded.SelectedCalendarURL != "/calendars/new-company/" {
		t.Errorf("SelectedCalendarURL: got %q", loaded.SelectedCalendarURL)
	}
	if loaded.DefaultCalendarCTag != "" {
		t.Errorf("DefaultCalendarCTag should not be set by stale pull: %q", loaded.DefaultCalendarCTag)
	}
	if loaded.InitialSyncCompletedAt != "" {
		t.Errorf("InitialSyncCompletedAt should not be restored by stale pull: %q", loaded.InitialSyncCompletedAt)
	}
	if loaded.SelectedCalendarReadinessStatus != calendarReadinessStatusInitialSyncPending {
		t.Errorf("SelectedCalendarReadinessStatus: got %q", loaded.SelectedCalendarReadinessStatus)
	}
}

func TestSaveSelectedCalendarBackfillsExistingEventsWhenWritable(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	event := newLocalTestCalendarEvent("existing-local", "Existing Local")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("write existing event: %v", errorValue)
	}
	account, errorValue := service.upsertRemoteCalendarAccount(ctx, remoteCalendarAccount{
		ID:                 "account-google-1",
		Provider:           remoteCalendarProviderGoogle,
		AccountEmail:       "user@example.com",
		DefaultCalendarURL: "/calendars/default/",
	})
	if errorValue != nil {
		t.Fatalf("seed account: %v", errorValue)
	}

	if _, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "Company", "writer", "/calendars/company/", time.Now()); errorValue != nil {
		t.Fatalf("save selected calendar: %v", errorValue)
	}

	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil {
		t.Fatalf("list outbox: %v", errorValue)
	}
	if len(rows) != 1 {
		t.Fatalf("outbox rows: got %d, want 1", len(rows))
	}
	if rows[0].EventID != event.ID || rows[0].EventUID != event.UID {
		t.Fatalf("outbox event mismatch: %+v", rows[0])
	}
	if rows[0].Operation != calendarOutboxOperationPut {
		t.Fatalf("operation: got %q", rows[0].Operation)
	}
	if rows[0].RemoteHref != "" || rows[0].IfMatchETag != "" {
		t.Fatalf("backfill should create a fresh remote object, got href=%q ifMatch=%q", rows[0].RemoteHref, rows[0].IfMatchETag)
	}
}

func TestSaveSelectedCalendarSkipsBackfillWhenReadOnly(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	event := newLocalTestCalendarEvent("existing-readonly", "Existing Readonly")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("write existing event: %v", errorValue)
	}
	account, errorValue := service.upsertRemoteCalendarAccount(ctx, remoteCalendarAccount{
		ID:                 "account-google-1",
		Provider:           remoteCalendarProviderGoogle,
		AccountEmail:       "user@example.com",
		DefaultCalendarURL: "/calendars/default/",
	})
	if errorValue != nil {
		t.Fatalf("seed account: %v", errorValue)
	}

	if _, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "Company", "reader", "/calendars/company/", time.Now()); errorValue != nil {
		t.Fatalf("save selected calendar: %v", errorValue)
	}

	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil {
		t.Fatalf("list outbox: %v", errorValue)
	}
	if len(rows) != 0 {
		t.Fatalf("read-only calendar should not receive backfill outbox rows: %+v", rows)
	}
}

func TestSaveSelectedCalendarBackfillsWhenSameCalendarBecomesWritable(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	event := newLocalTestCalendarEvent("existing-permission-upgrade", "Existing Permission Upgrade")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("write existing event: %v", errorValue)
	}
	account, errorValue := service.upsertRemoteCalendarAccount(ctx, remoteCalendarAccount{
		ID:                 "account-google-1",
		Provider:           remoteCalendarProviderGoogle,
		AccountEmail:       "user@example.com",
		DefaultCalendarURL: "/calendars/default/",
	})
	if errorValue != nil {
		t.Fatalf("seed account: %v", errorValue)
	}

	selectedAccount, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "Company", "reader", "/calendars/company/", time.Now())
	if errorValue != nil {
		t.Fatalf("save read-only selected calendar: %v", errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil {
		t.Fatalf("list read-only outbox: %v", errorValue)
	}
	if len(rows) != 0 {
		t.Fatalf("read-only selected calendar should not enqueue backfill: %+v", rows)
	}

	if _, errorValue := service.saveSelectedCalendar(ctx, selectedAccount, "company@example.com", "Company", "writer", "/calendars/company/", time.Now()); errorValue != nil {
		t.Fatalf("save writable selected calendar: %v", errorValue)
	}
	rows, errorValue = service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil {
		t.Fatalf("list writable outbox: %v", errorValue)
	}
	if len(rows) != 1 {
		t.Fatalf("writable permission upgrade should enqueue one backfill row, got %d", len(rows))
	}
	if rows[0].EventID != event.ID {
		t.Fatalf("backfill event: got %q, want %q", rows[0].EventID, event.ID)
	}
}

func TestSaveSelectedCalendarRetriesMissingBackfillForWritableSelection(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	event := newLocalTestCalendarEvent("existing-retry-backfill", "Existing Retry Backfill")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("write existing event: %v", errorValue)
	}
	account, errorValue := service.upsertRemoteCalendarAccount(ctx, remoteCalendarAccount{
		ID:                         "account-google-1",
		Provider:                   remoteCalendarProviderGoogle,
		AccountEmail:               "user@example.com",
		DefaultCalendarURL:         "/calendars/default/",
		SelectedCalendarID:         "company@example.com",
		SelectedCalendarSummary:    "Company",
		SelectedCalendarAccessRole: "writer",
		SelectedCalendarURL:        "/calendars/company/",
		SelectedCalendarSelectedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})
	if errorValue != nil {
		t.Fatalf("seed selected account: %v", errorValue)
	}

	if _, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "Company", "writer", "/calendars/company/", time.Now()); errorValue != nil {
		t.Fatalf("save selected calendar: %v", errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil {
		t.Fatalf("list outbox: %v", errorValue)
	}
	if len(rows) != 1 {
		t.Fatalf("same writable selection should repair missing backfill, got %d", len(rows))
	}
	if rows[0].EventID != event.ID {
		t.Fatalf("backfill event: got %q, want %q", rows[0].EventID, event.ID)
	}
}

func TestSaveSelectedCalendarWakesExistingSameTargetOutbox(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account, errorValue := service.upsertRemoteCalendarAccount(ctx, remoteCalendarAccount{
		ID:                         "account-google-1",
		Provider:                   remoteCalendarProviderGoogle,
		AccountEmail:               "user@example.com",
		DefaultCalendarURL:         "/calendars/default/",
		SelectedCalendarID:         "company@example.com",
		SelectedCalendarSummary:    "Company",
		SelectedCalendarAccessRole: "writer",
		SelectedCalendarURL:        "/calendars/company/",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event := newLocalTestCalendarEvent("same-target-wake", "Same Target Wake")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	select {
	case <-service.calendarSyncWakeUp:
	default:
		t.Fatal("initial event write did not signal sync")
	}
	if _, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "Company", "writer", "/calendars/company/", time.Now()); errorValue != nil {
		t.Fatal(errorValue)
	}
	select {
	case <-service.calendarSyncWakeUp:
	case <-time.After(time.Second):
		t.Fatal("same-target selection did not wake the existing outbox")
	}
}

func TestSaveSelectedCalendarPreservesInitialExportPendingForSameSelection(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account, errorValue := service.upsertRemoteCalendarAccount(ctx, remoteCalendarAccount{
		ID:                              "account-google-1",
		Provider:                        remoteCalendarProviderGoogle,
		AccountEmail:                    "user@example.com",
		DefaultCalendarURL:              "/calendars/default/",
		DefaultCalendarCTag:             `"selected-ctag"`,
		SelectedCalendarID:              "company@example.com",
		SelectedCalendarSummary:         "Company",
		SelectedCalendarAccessRole:      "writer",
		SelectedCalendarURL:             "/calendars/company/",
		SelectedCalendarSelectedAt:      time.Now().UTC().Format(time.RFC3339Nano),
		SelectedCalendarReadinessStatus: calendarReadinessStatusInitialExportPending,
	})
	if errorValue != nil {
		t.Fatalf("seed account: %v", errorValue)
	}
	event := newLocalTestCalendarEvent("existing-export-pending", "Existing Export Pending")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("write existing event: %v", errorValue)
	}

	updated, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "Company", "writer", "/calendars/company/", time.Now())
	if errorValue != nil {
		t.Fatalf("save same selected calendar: %v", errorValue)
	}
	if updated.SelectedCalendarReadinessStatus != calendarReadinessStatusInitialExportPending {
		t.Fatalf("SelectedCalendarReadinessStatus: got %q", updated.SelectedCalendarReadinessStatus)
	}

	client := &fakeCalDAVPushClient{
		putETags: map[string]string{
			"/calendars/company/" + event.UID + ".ics": `"etag-exported"`,
		},
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, updated, client); errorValue != nil {
		t.Fatalf("push pending export: %v", errorValue)
	}
	if len(client.putCalls) != 1 {
		t.Fatalf("pending export should push after same selection save, got %d calls", len(client.putCalls))
	}
}

func TestSaveSelectedCalendarPreservesCompletedInitialSyncForStaleSameSelection(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	staleAccount, errorValue := service.upsertRemoteCalendarAccount(ctx, remoteCalendarAccount{
		ID:                              "account-google-1",
		Provider:                        remoteCalendarProviderGoogle,
		AccountEmail:                    "user@example.com",
		DefaultCalendarURL:              "/calendars/default/",
		SelectedCalendarID:              "company@example.com",
		SelectedCalendarSummary:         "Company",
		SelectedCalendarAccessRole:      "writer",
		SelectedCalendarURL:             "/calendars/company/",
		SelectedCalendarReadinessStatus: calendarReadinessStatusInitialExportPending,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	completedAt := time.Now().UTC().Add(-time.Second)
	if errorValue := service.markSelectedCalendarInitialSyncCompleted(ctx, staleAccount, completedAt); errorValue != nil {
		t.Fatal(errorValue)
	}
	updated, errorValue := service.saveSelectedCalendar(ctx, staleAccount, "company@example.com", "Company", "writer", "/calendars/company/", time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if updated.SelectedCalendarReadinessStatus != calendarReadinessStatusSyncReady {
		t.Fatalf("readiness status=%q want %q", updated.SelectedCalendarReadinessStatus, calendarReadinessStatusSyncReady)
	}
	if updated.InitialSyncCompletedAt != completedAt.Format(time.RFC3339Nano) {
		t.Fatalf("initial sync completed at=%q want %q", updated.InitialSyncCompletedAt, completedAt.Format(time.RFC3339Nano))
	}
}

func TestRemoteCalendarAccountSchemaMigratesLegacyColumns(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	database, errorValue := service.openSQLiteDatabase(ctx, service.stateDatabasePath(), nil)
	if errorValue != nil {
		t.Fatalf("open legacy database: %v", errorValue)
	}
	_, errorValue = database.ExecContext(ctx, `
CREATE TABLE calendar_remote_accounts (
	id TEXT PRIMARY KEY,
	provider TEXT NOT NULL,
	account_email TEXT NOT NULL,
	principal_url TEXT NOT NULL DEFAULT '',
	home_set_url TEXT NOT NULL DEFAULT '',
	default_calendar_url TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(provider, account_email)
)`)
	if errorValue != nil {
		t.Fatalf("create legacy remote accounts: %v", errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatalf("close legacy database: %v", errorValue)
	}

	account := remoteCalendarAccount{
		ID:                         "legacy-account-google",
		Provider:                   remoteCalendarProviderGoogle,
		AccountEmail:               "legacy@example.com",
		DefaultCalendarCTag:        "legacy-ctag",
		TokenFilePath:              "/tmp/legacy-token.enc",
		LastAuthError:              "expired token",
		LastAuthErrorAt:            "2026-06-01T00:00:00Z",
		SelectedCalendarID:         "legacy-company@example.com",
		SelectedCalendarSummary:    "Legacy Company",
		SelectedCalendarAccessRole: "owner",
		SelectedCalendarURL:        "https://apidata.googleusercontent.com/caldav/v2/legacy@example.com/company/events/",
		SelectedCalendarSelectedAt: "2026-07-01T11:00:00Z",
		InitialSyncCompletedAt:     "2026-07-01T11:10:00Z",
	}
	if _, errorValue := service.upsertRemoteCalendarAccount(ctx, account); errorValue != nil {
		t.Fatalf("upsert migrated account: %v", errorValue)
	}
	loaded, found, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("read migrated account: %v", errorValue)
	}
	if !found {
		t.Fatal("expected migrated account found")
	}
	if loaded.DefaultCalendarCTag != account.DefaultCalendarCTag {
		t.Errorf("DefaultCalendarCTag: got %q, want %q", loaded.DefaultCalendarCTag, account.DefaultCalendarCTag)
	}
	if loaded.TokenFilePath != account.TokenFilePath {
		t.Errorf("TokenFilePath: got %q, want %q", loaded.TokenFilePath, account.TokenFilePath)
	}
	if loaded.LastAuthError != account.LastAuthError {
		t.Errorf("LastAuthError: got %q, want %q", loaded.LastAuthError, account.LastAuthError)
	}
	if loaded.LastAuthErrorAt != account.LastAuthErrorAt {
		t.Errorf("LastAuthErrorAt: got %q, want %q", loaded.LastAuthErrorAt, account.LastAuthErrorAt)
	}
	if loaded.SelectedCalendarID != account.SelectedCalendarID {
		t.Errorf("SelectedCalendarID: got %q, want %q", loaded.SelectedCalendarID, account.SelectedCalendarID)
	}
	if loaded.SelectedCalendarURL != account.SelectedCalendarURL {
		t.Errorf("SelectedCalendarURL: got %q, want %q", loaded.SelectedCalendarURL, account.SelectedCalendarURL)
	}
	if loaded.InitialSyncCompletedAt != account.InitialSyncCompletedAt {
		t.Errorf("InitialSyncCompletedAt: got %q, want %q", loaded.InitialSyncCompletedAt, account.InitialSyncCompletedAt)
	}
}
