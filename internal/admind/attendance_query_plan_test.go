package admind

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestAttendanceEventLookupQueryPlansUseDedicatedIndexes(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	monthPlan := attendanceQueryPlanForTest(
		t,
		database,
		attendanceMonthlyEventsQuery(false),
		"2026-07-01",
		"2026-08-01",
		"2026-07-01",
		"2026-08-01",
	)
	resultPostPlan := attendanceQueryPlanForTest(
		t,
		database,
		`SELECT events.local_date, overrides.override_local_date
		FROM attendance_events AS events
		LEFT JOIN attendance_event_overrides AS overrides ON overrides.event_id = events.id
		WHERE events.result_post_id = ?`,
		"result-post",
	)
	t.Logf("month query plan = %s", monthPlan)
	t.Logf("result-post query plan = %s", resultPostPlan)
	if !strings.Contains(monthPlan, "attendance_events_local_date") {
		t.Errorf("month query plan = %s", monthPlan)
	}
	if !strings.Contains(resultPostPlan, "attendance_events_result_post") {
		t.Errorf("result-post query plan = %s", resultPostPlan)
	}
}

func attendanceQueryPlanForTest(t *testing.T, database *sql.DB, query string, arguments ...any) string {
	t.Helper()
	rows, errorValue := database.Query("EXPLAIN QUERY PLAN "+query, arguments...)
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
	}
	if errorValue := rows.Err(); errorValue != nil {
		t.Fatal(errorValue)
	}
	return strings.Join(details, " | ")
}
