package admind

import (
	"context"
	"testing"
)

func TestLatestCalendarIdentityValueUsesTimestampOrderAndDeterministicFallbacks(t *testing.T) {
	testCases := []struct {
		name   string
		first  string
		second string
		want   string
	}{
		{name: "fractional second", first: "2026-07-16T00:00:00.1Z", second: "2026-07-16T00:00:00Z", want: "2026-07-16T00:00:00.1Z"},
		{name: "equal instant preserves canonical", first: "2026-07-16T01:00:00+01:00", second: "2026-07-16T00:00:00Z", want: "2026-07-16T00:00:00Z"},
		{name: "valid beats invalid", first: "invalid", second: "2026-07-16T00:00:00Z", want: "2026-07-16T00:00:00Z"},
		{name: "invalid beats empty", first: "invalid", second: "", want: "invalid"},
		{name: "invalid fallback is lexical", first: "alpha", second: "beta", want: "beta"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := latestCalendarIdentityValue(testCase.first, testCase.second); got != testCase.want {
				t.Fatalf("latest value=%q want %q", got, testCase.want)
			}
		})
	}
}

func TestMergeCalendarRemoteEventStateIdentityRowsPreservesLatestMissingSemantics(t *testing.T) {
	testCases := []struct {
		name             string
		legacyUpdatedAt  string
		legacyMissingAt  string
		canonicalUpdated string
		canonicalMissing string
		wantMissing      string
	}{
		{name: "newer canonical clear wins", legacyUpdatedAt: "2026-07-16T00:00:00Z", legacyMissingAt: "2026-07-15T23:00:00Z", canonicalUpdated: "2026-07-16T00:00:01Z", canonicalMissing: "", wantMissing: ""},
		{name: "newer legacy missing wins", legacyUpdatedAt: "2026-07-16T00:00:02Z", legacyMissingAt: "2026-07-16T00:00:02Z", canonicalUpdated: "2026-07-16T00:00:01Z", canonicalMissing: "", wantMissing: "2026-07-16T00:00:02Z"},
		{name: "equal updated at preserves canonical clear", legacyUpdatedAt: "2026-07-16T01:00:00+01:00", legacyMissingAt: "2026-07-15T23:00:00Z", canonicalUpdated: "2026-07-16T00:00:00Z", canonicalMissing: "", wantMissing: ""},
		{name: "invalid updated at uses deterministic order", legacyUpdatedAt: "z-invalid", legacyMissingAt: "legacy-missing", canonicalUpdated: "a-invalid", canonicalMissing: "", wantMissing: "legacy-missing"},
		{name: "equal invalid updated at preserves canonical", legacyUpdatedAt: "invalid", legacyMissingAt: "legacy-missing", canonicalUpdated: "invalid", canonicalMissing: "", wantMissing: ""},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			legacyRow := calendarRemoteEventStateIdentityRow{
				AccountID:         "account-1",
				CalendarURL:       "https://calendar.example.com/calendars/company",
				EventUID:          "event-1",
				RemoteModifiedAt:  "2026-07-16T00:00:03Z",
				LastSeenAt:        "2026-07-16T00:00:00Z",
				MissingDetectedAt: testCase.legacyMissingAt,
				UpdatedAt:         testCase.legacyUpdatedAt,
			}
			canonicalRow := calendarRemoteEventStateIdentityRow{
				AccountID:         "account-1",
				CalendarURL:       "/calendars/company/",
				EventUID:          "event-1",
				RemoteModifiedAt:  "2026-07-16T00:00:01Z",
				LastSeenAt:        "2026-07-16T00:00:04Z",
				MissingDetectedAt: testCase.canonicalMissing,
				UpdatedAt:         testCase.canonicalUpdated,
			}
			mergedRow := mergeCalendarRemoteEventStateIdentityRows(legacyRow, canonicalRow, "/calendars/company/")
			if mergedRow.MissingDetectedAt != testCase.wantMissing {
				t.Fatalf("missing detected at=%q want %q", mergedRow.MissingDetectedAt, testCase.wantMissing)
			}
			if mergedRow.RemoteModifiedAt != legacyRow.RemoteModifiedAt {
				t.Fatalf("remote modified at=%q want %q", mergedRow.RemoteModifiedAt, legacyRow.RemoteModifiedAt)
			}
			if mergedRow.LastSeenAt != canonicalRow.LastSeenAt {
				t.Fatalf("last seen at=%q want %q", mergedRow.LastSeenAt, canonicalRow.LastSeenAt)
			}
		})
	}
}

func TestCalendarTargetIdentityMigrationMergesCanonicalDuplicatesOnce(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = database.ExecContext(ctx, `
INSERT INTO calendar_push_observation_fences (account_id, calendar_url, event_uid, created_at)
VALUES
	('account-1', '/calendars/company/', 'event-1', '2026-07-16T00:00:00Z'),
	('account-1', 'https://calendar.example.com/calendars/company', 'event-1', '2026-07-16T00:00:01Z');
INSERT INTO calendar_remote_event_sync_state (
	account_id, calendar_url, event_uid, remote_modified_at, last_seen_at, missing_detected_at, updated_at
) VALUES
	('account-1', '/calendars/company/', 'event-1', '2026-07-15T23:59:00Z', '2026-07-16T00:00:00Z', '', '2026-07-16T00:00:00Z'),
	('account-1', 'https://calendar.example.com/calendars/company', 'event-1', '', '2026-07-16T00:00:01Z', '2026-07-16T00:00:02Z', '2026-07-16T00:00:02Z');
CREATE TABLE calendar_target_identity_migration_audit (write_count INTEGER NOT NULL);
INSERT INTO calendar_target_identity_migration_audit (write_count) VALUES (0);
CREATE TRIGGER calendar_fence_identity_insert_audit AFTER INSERT ON calendar_push_observation_fences BEGIN
	UPDATE calendar_target_identity_migration_audit SET write_count = write_count + 1;
END;
CREATE TRIGGER calendar_fence_identity_delete_audit AFTER DELETE ON calendar_push_observation_fences BEGIN
	UPDATE calendar_target_identity_migration_audit SET write_count = write_count + 1;
END;
CREATE TRIGGER calendar_remote_state_identity_insert_audit AFTER INSERT ON calendar_remote_event_sync_state BEGIN
	UPDATE calendar_target_identity_migration_audit SET write_count = write_count + 1;
END;
CREATE TRIGGER calendar_remote_state_identity_update_audit AFTER UPDATE ON calendar_remote_event_sync_state BEGIN
	UPDATE calendar_target_identity_migration_audit SET write_count = write_count + 1;
END;
CREATE TRIGGER calendar_remote_state_identity_delete_audit AFTER DELETE ON calendar_remote_event_sync_state BEGIN
	UPDATE calendar_target_identity_migration_audit SET write_count = write_count + 1;
END;`)
	database.Close()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	service.databaseSchemas = newAdminDatabaseSchemas()
	database, errorValue = service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var fenceCount int
	var fenceCalendarURL string
	var fenceCreatedAt string
	if errorValue := database.QueryRowContext(ctx, `
SELECT COUNT(*), calendar_url, created_at
FROM calendar_push_observation_fences
WHERE account_id = ? AND event_uid = ?`, "account-1", "event-1").Scan(&fenceCount, &fenceCalendarURL, &fenceCreatedAt); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if fenceCount != 1 || fenceCalendarURL != "/calendars/company/" || fenceCreatedAt != "2026-07-16T00:00:01Z" {
		database.Close()
		t.Fatalf("migrated fence count=%d url=%q created=%q", fenceCount, fenceCalendarURL, fenceCreatedAt)
	}
	var remoteStateCount int
	var remoteStateCalendarURL string
	var remoteModifiedAt string
	var lastSeenAt string
	var missingDetectedAt string
	var updatedAt string
	if errorValue := database.QueryRowContext(ctx, `
SELECT COUNT(*), calendar_url, remote_modified_at, last_seen_at, missing_detected_at, updated_at
FROM calendar_remote_event_sync_state
WHERE account_id = ? AND event_uid = ?`, "account-1", "event-1").Scan(&remoteStateCount, &remoteStateCalendarURL, &remoteModifiedAt, &lastSeenAt, &missingDetectedAt, &updatedAt); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if remoteStateCount != 1 || remoteStateCalendarURL != "/calendars/company/" || remoteModifiedAt != "2026-07-15T23:59:00Z" || lastSeenAt != "2026-07-16T00:00:01Z" || missingDetectedAt != "2026-07-16T00:00:02Z" || updatedAt != "2026-07-16T00:00:02Z" {
		database.Close()
		t.Fatalf("migrated remote state count=%d url=%q modified=%q seen=%q missing=%q updated=%q", remoteStateCount, remoteStateCalendarURL, remoteModifiedAt, lastSeenAt, missingDetectedAt, updatedAt)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE calendar_target_identity_migration_audit SET write_count = 0`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	database.Close()
	service.databaseSchemas = newAdminDatabaseSchemas()
	database, errorValue = service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var repeatedWriteCount int
	if errorValue := database.QueryRowContext(ctx, `SELECT write_count FROM calendar_target_identity_migration_audit`).Scan(&repeatedWriteCount); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	database.Close()
	if repeatedWriteCount != 0 {
		t.Fatalf("repeated migration writes=%d want 0", repeatedWriteCount)
	}
	fencedUIDs, errorValue := service.listCalendarPushObservationFenceUIDs(ctx, "account-1", "https://calendar.example.com/calendars/company")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, found := fencedUIDs["event-1"]; !found {
		t.Fatalf("migrated fence missing through absolute target lookup: %+v", fencedUIDs)
	}
	remoteState, found, errorValue := service.readCalendarRemoteEventState(ctx, "account-1", "https://calendar.example.com/calendars/company", "event-1")
	if errorValue != nil || !found || remoteState.LastSeenAt != "2026-07-16T00:00:01Z" {
		t.Fatalf("migrated remote state found=%v state=%+v error=%v", found, remoteState, errorValue)
	}
	if errorValue := service.clearObservedCalendarPushObservationFences(ctx, "account-1", "https://calendar.example.com/calendars/company", fencedUIDs, map[string]struct{}{"event-1": {}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	fencedUIDs, errorValue = service.listCalendarPushObservationFenceUIDs(ctx, "account-1", "/calendars/company/")
	if errorValue != nil || len(fencedUIDs) != 0 {
		t.Fatalf("canonical fence remains after absolute clear: %+v error=%v", fencedUIDs, errorValue)
	}
}

func TestCalendarTargetIdentityMigrationPreservesLoneLegacyState(t *testing.T) {
	testCases := []struct {
		name      string
		updatedAt string
	}{
		{name: "empty updated at", updatedAt: ""},
		{name: "invalid updated at", updatedAt: "invalid"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := newCalendarTestService(t)
			ctx := context.Background()
			database, errorValue := service.openCalendarDatabase(ctx)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			_, errorValue = database.ExecContext(ctx, `
INSERT INTO calendar_remote_event_sync_state (
	account_id, calendar_url, event_uid, remote_modified_at, last_seen_at, missing_detected_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?)`,
				"account-lone",
				"https://calendar.example.com/calendars/company",
				"event-lone",
				"remote-modified",
				"last-seen",
				"missing-detected",
				testCase.updatedAt,
			)
			database.Close()
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			service.databaseSchemas = newAdminDatabaseSchemas()
			database, errorValue = service.openCalendarDatabase(ctx)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			defer database.Close()
			var calendarURL string
			var remoteModifiedAt string
			var lastSeenAt string
			var missingDetectedAt string
			var updatedAt string
			errorValue = database.QueryRowContext(ctx, `
SELECT calendar_url, remote_modified_at, last_seen_at, missing_detected_at, updated_at
FROM calendar_remote_event_sync_state
WHERE account_id = ? AND event_uid = ?`, "account-lone", "event-lone").Scan(
				&calendarURL,
				&remoteModifiedAt,
				&lastSeenAt,
				&missingDetectedAt,
				&updatedAt,
			)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if calendarURL != "/calendars/company/" || remoteModifiedAt != "remote-modified" || lastSeenAt != "last-seen" || missingDetectedAt != "missing-detected" || updatedAt != testCase.updatedAt {
				t.Fatalf("url=%q modified=%q seen=%q missing=%q updated=%q", calendarURL, remoteModifiedAt, lastSeenAt, missingDetectedAt, updatedAt)
			}
		})
	}
}

func TestCalendarTargetIdentityMigrationRollsBackBothTablesOnFailure(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = database.ExecContext(ctx, `
INSERT INTO calendar_push_observation_fences (account_id, calendar_url, event_uid, created_at)
VALUES
	('account-rollback', '/calendars/company/', 'event-rollback', '2026-07-16T00:00:00Z'),
	('account-rollback', 'https://calendar.example.com/calendars/company', 'event-rollback', '2026-07-16T00:00:01Z');
INSERT INTO calendar_remote_event_sync_state (
	account_id, calendar_url, event_uid, remote_modified_at, last_seen_at, missing_detected_at, updated_at
) VALUES
	('account-rollback', '/calendars/company/', 'event-rollback', '', '2026-07-16T00:00:00Z', '', '2026-07-16T00:00:00Z'),
	('account-rollback', 'https://calendar.example.com/calendars/company', 'event-rollback', '', '2026-07-16T00:00:01Z', '', '2026-07-16T00:00:01Z');
CREATE TRIGGER fail_calendar_remote_state_identity_migration
BEFORE DELETE ON calendar_remote_event_sync_state
WHEN OLD.calendar_url = 'https://calendar.example.com/calendars/company'
BEGIN
	SELECT RAISE(ABORT, 'forced target identity migration failure');
END;`)
	database.Close()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	service.databaseSchemas = newAdminDatabaseSchemas()
	if database, errorValue = service.openCalendarDatabase(ctx); errorValue == nil {
		database.Close()
		t.Fatal("migration unexpectedly succeeded")
	}
	database, errorValue = service.openSQLiteDatabase(ctx, service.stateDatabasePath(), nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var fenceCount int
	if errorValue := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM calendar_push_observation_fences WHERE account_id = ?`, "account-rollback").Scan(&fenceCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	var remoteStateCount int
	if errorValue := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM calendar_remote_event_sync_state WHERE account_id = ?`, "account-rollback").Scan(&remoteStateCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if fenceCount != 2 || remoteStateCount != 2 {
		t.Fatalf("migration rollback fence_count=%d remote_state_count=%d", fenceCount, remoteStateCount)
	}
}
