package admind

import (
	"context"
	"testing"
)

func TestCalendarOutboxSchemaBackfillsLegacyTargetlessRowsSafely(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	database, errorValue := service.openSQLiteDatabase(ctx, service.stateDatabasePath(), nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = database.ExecContext(ctx, `
CREATE TABLE calendar_remote_accounts (
	id TEXT PRIMARY KEY,
	provider TEXT NOT NULL,
	account_email TEXT NOT NULL,
	default_calendar_url TEXT NOT NULL DEFAULT '',
	selected_calendar_id TEXT NOT NULL DEFAULT '',
	selected_calendar_url TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(provider, account_email)
);
CREATE TABLE calendar_outbox (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	account_id TEXT NOT NULL,
	event_id TEXT NOT NULL,
	event_uid TEXT NOT NULL,
	operation TEXT NOT NULL,
	payload_ics TEXT NOT NULL DEFAULT '',
	if_match_etag TEXT NOT NULL DEFAULT '',
	remote_href TEXT NOT NULL DEFAULT '',
	attempt_count INTEGER NOT NULL DEFAULT 0,
	last_error TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	last_attempted_at TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'pending',
	failed_at TEXT NOT NULL DEFAULT ''
);
INSERT INTO calendar_remote_accounts (
	id, provider, account_email, default_calendar_url, selected_calendar_id, selected_calendar_url, created_at, updated_at
) VALUES (
	'legacy-account', 'google', 'legacy@example.com', '/calendars/default/', 'company@example.com', 'https://calendar.example.com/calendars/company', '2026-07-16T00:00:00Z', '2026-07-16T00:00:00Z'
), (
	'partial-account', 'google', 'partial@example.com', '/calendars/default/', 'partial@example.com', '', '2026-07-16T00:00:00Z', '2026-07-16T00:00:00Z'
);
INSERT INTO calendar_outbox (
	account_id, event_id, event_uid, operation, if_match_etag, remote_href, created_at
) VALUES
	('legacy-account', 'legacy-put', 'legacy-put@example.com', 'put', '"etag-default"', '/calendars/default/legacy-put.ics', '2026-07-16T00:00:00Z'),
	('legacy-account', 'legacy-delete', 'legacy-delete@example.com', 'delete', '"etag-default"', '/calendars/default/legacy-delete.ics', '2026-07-16T00:00:01Z'),
	('legacy-account', 'legacy-active-delete', 'legacy-active-delete@example.com', 'delete', '"etag-company"', '/calendars/company/legacy-active-delete.ics', '2026-07-16T00:00:02Z'),
	('missing-account', 'legacy-orphan-put', 'legacy-orphan-put@example.com', 'put', '', '', '2026-07-16T00:00:03Z'),
	('partial-account', 'legacy-partial-put', 'legacy-partial-put@example.com', 'put', '', '', '2026-07-16T00:00:04Z');`)
	if errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	database, errorValue = service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue = service.openSQLiteDatabase(ctx, service.stateDatabasePath(), nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = database.ExecContext(ctx, `
CREATE TABLE calendar_outbox_update_audit (update_count INTEGER NOT NULL);
INSERT INTO calendar_outbox_update_audit (update_count) VALUES (0);
CREATE TRIGGER calendar_outbox_update_audit_trigger
AFTER UPDATE ON calendar_outbox
BEGIN
	UPDATE calendar_outbox_update_audit SET update_count = update_count + 1;
END;`)
	if errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	service.databaseSchemas = newAdminDatabaseSchemas()
	database, errorValue = service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue = service.openSQLiteDatabase(ctx, service.stateDatabasePath(), nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var repeatedMigrationUpdateCount int
	if errorValue := database.QueryRowContext(ctx, `SELECT update_count FROM calendar_outbox_update_audit`).Scan(&repeatedMigrationUpdateCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if repeatedMigrationUpdateCount != 0 {
		t.Fatalf("repeated migration updates=%d want 0", repeatedMigrationUpdateCount)
	}

	rows, errorValue := database.QueryContext(ctx, `SELECT operation, target_calendar_url, remote_href, if_match_etag, status FROM calendar_outbox ORDER BY id`)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer rows.Close()
	var putOperation string
	var putTarget string
	var putRemoteHref string
	var putRemoteETag string
	var putStatus string
	if !rows.Next() {
		t.Fatal("missing migrated put row")
	}
	if errorValue := rows.Scan(&putOperation, &putTarget, &putRemoteHref, &putRemoteETag, &putStatus); errorValue != nil {
		t.Fatal(errorValue)
	}
	if putOperation != calendarOutboxOperationPut || putTarget != "/calendars/company/" || putRemoteHref != "" || putRemoteETag != "" || putStatus != calendarOutboxStatusPending {
		t.Fatalf("migrated put operation=%q target=%q href=%q etag=%q status=%q", putOperation, putTarget, putRemoteHref, putRemoteETag, putStatus)
	}
	var deleteOperation string
	var deleteTarget string
	var deleteRemoteHref string
	var deleteRemoteETag string
	var deleteStatus string
	if !rows.Next() {
		t.Fatal("missing migrated delete row")
	}
	if errorValue := rows.Scan(&deleteOperation, &deleteTarget, &deleteRemoteHref, &deleteRemoteETag, &deleteStatus); errorValue != nil {
		t.Fatal(errorValue)
	}
	if deleteOperation != calendarOutboxOperationDelete || deleteTarget != "" || deleteRemoteHref == "" || deleteRemoteETag == "" || deleteStatus != calendarOutboxStatusBlocked {
		t.Fatalf("migrated delete operation=%q target=%q href=%q etag=%q status=%q", deleteOperation, deleteTarget, deleteRemoteHref, deleteRemoteETag, deleteStatus)
	}
	var activeDeleteOperation string
	var activeDeleteTarget string
	var activeDeleteRemoteHref string
	var activeDeleteRemoteETag string
	var activeDeleteStatus string
	if !rows.Next() {
		t.Fatal("missing migrated active-target delete row")
	}
	if errorValue := rows.Scan(&activeDeleteOperation, &activeDeleteTarget, &activeDeleteRemoteHref, &activeDeleteRemoteETag, &activeDeleteStatus); errorValue != nil {
		t.Fatal(errorValue)
	}
	if activeDeleteOperation != calendarOutboxOperationDelete || activeDeleteTarget != "" || activeDeleteRemoteHref == "" || activeDeleteRemoteETag == "" || activeDeleteStatus != calendarOutboxStatusBlocked {
		t.Fatalf("migrated active-target delete operation=%q target=%q href=%q etag=%q status=%q", activeDeleteOperation, activeDeleteTarget, activeDeleteRemoteHref, activeDeleteRemoteETag, activeDeleteStatus)
	}
	if errorValue := rows.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	const expectedUnavailableError = "legacy calendar outbox PUT target unavailable; reconnect account or select a complete writable calendar"
	for _, eventID := range []string{"legacy-orphan-put", "legacy-partial-put"} {
		var targetCalendarURL string
		var status string
		var lastError string
		if errorValue := database.QueryRowContext(ctx, `SELECT target_calendar_url, status, last_error FROM calendar_outbox WHERE event_id = ?`, eventID).Scan(&targetCalendarURL, &status, &lastError); errorValue != nil {
			t.Fatal(errorValue)
		}
		if targetCalendarURL != "" || status != calendarOutboxStatusBlocked || lastError != expectedUnavailableError {
			t.Fatalf("migrated unavailable put event=%q target=%q status=%q error=%q", eventID, targetCalendarURL, status, lastError)
		}
	}
}
