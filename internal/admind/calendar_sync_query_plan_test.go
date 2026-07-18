package admind

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestCalendarOutboxLatestEventActionQueryUsesTargetEventIndex(t *testing.T) {
	service := newCalendarTestService(t)
	database := openCalendarQueryPlanTestDatabase(t, service)
	defer database.Close()
	query := `SELECT operation FROM calendar_outbox WHERE account_id = ? AND target_calendar_url = ? AND event_uid = ? AND ` + calendarOutboxActiveStatusPredicate + ` ORDER BY id DESC LIMIT 1`
	assertCalendarQueryPlanUsesIndex(
		t,
		database,
		query,
		"calendar_outbox_account_target_event_idx",
		"account",
		"/calendars/company/",
		"event@internkim",
		calendarOutboxStatusPending,
		calendarOutboxStatusBlocked,
	)
}

func TestCalendarConflictDuplicateIdentityQueryUsesActiveIdentityIndex(t *testing.T) {
	service := newCalendarTestService(t)
	database := openCalendarQueryPlanTestDatabase(t, service)
	defer database.Close()
	assertCalendarQueryPlanUsesIndex(
		t,
		database,
		`SELECT 1 FROM calendar_conflicts WHERE event_uid = ? AND field = ? AND local_value = ? AND remote_value = ? AND dismissed_at = ''`,
		"calendar_conflicts_active_identity_idx",
		"event@internkim",
		calendarFieldTitle,
		"local",
		"remote",
	)
}

func TestCalendarActiveRemoteSourceQueryUsesRemoteSourceUIDIndex(t *testing.T) {
	service := newCalendarTestService(t)
	database := openCalendarQueryPlanTestDatabase(t, service)
	defer database.Close()
	assertCalendarQueryPlanUsesIndex(
		t,
		database,
		`SELECT id, uid FROM calendar_events WHERE deleted_at = '' AND remote_source = ? ORDER BY uid`,
		"calendar_events_remote_source_uid_deleted_idx",
		remoteCalendarProviderGoogle,
	)
}

func TestCalendarConflictClockOutboxQueryUsesEventUIDIndex(t *testing.T) {
	service := newCalendarTestService(t)
	database := openCalendarQueryPlanTestDatabase(t, service)
	defer database.Close()
	assertCalendarQueryPlanUsesIndex(
		t,
		database,
		`SELECT created_at FROM calendar_outbox WHERE event_uid = ?`,
		"calendar_outbox_event_uid_idx",
		"event@internkim",
	)
}

func TestCalendarConflictClockRemoteStateQueryUsesEventUIDIndex(t *testing.T) {
	service := newCalendarTestService(t)
	database := openCalendarQueryPlanTestDatabase(t, service)
	defer database.Close()
	assertCalendarQueryPlanUsesIndex(
		t,
		database,
		`SELECT last_seen_at, missing_detected_at FROM calendar_remote_event_sync_state WHERE event_uid = ?`,
		"calendar_remote_event_sync_state_event_uid_idx",
		"event@internkim",
	)
}

func TestCalendarDeleteIntentCleanupUsesResolvedAtIndex(t *testing.T) {
	service := newCalendarTestService(t)
	database := openCalendarQueryPlanTestDatabase(t, service)
	defer database.Close()
	assertCalendarQueryPlanUsesIndex(
		t,
		database,
		`DELETE FROM calendar_delete_intents WHERE status IN (?, ?, ?) AND resolved_at != '' AND resolved_at < ?`,
		"calendar_delete_intents_status_resolved_at_idx",
		calendarDeleteIntentStatusCanceled,
		calendarDeleteIntentStatusExecuted,
		calendarDeleteIntentStatusConflicted,
		"2026-01-01T00:00:00Z",
	)
}

func TestCalendarBackfillPendingActionQueryUsesStatusIndex(t *testing.T) {
	service := newCalendarTestService(t)
	database := openCalendarQueryPlanTestDatabase(t, service)
	defer database.Close()
	assertCalendarQueryPlanUsesIndex(
		t,
		database,
		`SELECT event_uid, operation FROM calendar_outbox WHERE account_id = ? AND target_calendar_url = ? AND status IN (?, ?)`,
		"calendar_outbox_account_target_status_event_idx",
		"account",
		"/calendars/company/",
		calendarOutboxStatusPending,
		calendarOutboxStatusBlocked,
	)
}

func TestCalendarPendingNotificationQueryUsesStatusNotifyAtIndex(t *testing.T) {
	service := newCalendarTestService(t)
	database := openCalendarQueryPlanTestDatabase(t, service)
	defer database.Close()
	assertCalendarQueryPlanUsesIndex(
		t,
		database,
		`SELECT event_id, recipient_key FROM calendar_event_notifications WHERE status = 'pending' AND notify_at <= ? ORDER BY notify_at`,
		"calendar_event_notifications_status_notify_at_idx",
		"2026-01-01T00:00:00Z",
	)
}

func TestCalendarNotificationStartupEventQueryUsesDeletedEndIndex(t *testing.T) {
	service := newCalendarTestService(t)
	database := openCalendarQueryPlanTestDatabase(t, service)
	defer database.Close()
	assertCalendarQueryPlanUsesIndex(
		t,
		database,
		`SELECT id FROM calendar_events WHERE deleted_at = '' AND end_at >= ? ORDER BY end_at`,
		"calendar_events_active_end_start_idx",
		"2026-01-01T00:00:00Z",
	)
}

func openCalendarQueryPlanTestDatabase(t *testing.T, service *Service) *sql.DB {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return database
}

func assertCalendarQueryPlanUsesIndex(t *testing.T, database *sql.DB, query string, expectedIndex string, arguments ...any) {
	t.Helper()
	rows, errorValue := database.QueryContext(context.Background(), "EXPLAIN QUERY PLAN "+query, arguments...)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer rows.Close()
	details := []string{}
	for rows.Next() {
		var identifier int
		var parentIdentifier int
		var unused int
		var detail string
		if errorValue := rows.Scan(&identifier, &parentIdentifier, &unused, &detail); errorValue != nil {
			t.Fatal(errorValue)
		}
		details = append(details, detail)
		normalizedDetail := strings.ToUpper(strings.TrimSpace(detail))
		if strings.Contains(normalizedDetail, "TEMP B-TREE") {
			t.Fatalf("query plan uses temporary B-tree: %v", details)
		}
		if strings.HasPrefix(normalizedDetail, "SCAN ") && !strings.Contains(normalizedDetail, " USING INDEX ") && !strings.Contains(normalizedDetail, " USING COVERING INDEX ") {
			t.Fatalf("query plan uses bare table scan: %v", details)
		}
	}
	if errorValue := rows.Err(); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(strings.Join(details, "\n"), expectedIndex) {
		t.Fatalf("query plan does not use %s: %v", expectedIndex, details)
	}
}
