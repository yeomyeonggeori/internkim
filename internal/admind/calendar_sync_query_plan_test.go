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
		"calendar_events_active_remote_source_uid_idx",
		remoteCalendarProviderGoogle,
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
