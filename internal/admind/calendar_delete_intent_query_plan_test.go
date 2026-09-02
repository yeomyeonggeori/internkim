package admind

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestCalendarDeleteIntentCleanupUsesResolvedAtIndex(t *testing.T) {
	service := newCalendarTestService(t)
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
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
