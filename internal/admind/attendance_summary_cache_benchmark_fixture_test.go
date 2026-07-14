package admind

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func clearAttendanceSummaryBenchmarkCache(b *testing.B, service *Service) {
	b.Helper()
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		b.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.Exec("DELETE FROM attendance_summary_cache_entries"); errorValue != nil {
		b.Fatal(errorValue)
	}
}

func newAttendanceSummaryBenchmarkService(testContext testing.TB) *Service {
	testContext.Helper()
	stateDirectory := testContext.TempDir()
	service := NewService(Configuration{
		StateDirectory:         stateDirectory,
		AttendanceDatabasePath: filepath.Join(stateDirectory, "attendance.sqlite"),
		FlowDatabasePath:       filepath.Join(stateDirectory, "flow.sqlite"),
	})
	seedAttendanceSummaryBenchmark(testContext, service)
	return service
}

func newAuthenticatedAttendanceSummaryBenchmarkService(testContext testing.TB) *Service {
	testContext.Helper()
	service := newAttendanceSummaryBenchmarkService(testContext)
	usersSyncPath := filepath.Join(service.Configuration.StateDirectory, "users-sync.json")
	if errorValue := os.WriteFile(usersSyncPath, []byte(`{"users":["member-00@example.com"]}`), 0o600); errorValue != nil {
		testContext.Fatal(errorValue)
	}
	if errorValue := service.writeAttendanceTeamViewVisibleToAll(context.Background(), true); errorValue != nil {
		testContext.Fatal(errorValue)
	}
	return service
}

func seedAttendanceSummaryBenchmark(testContext testing.TB, service *Service) {
	testContext.Helper()
	ctx := context.Background()
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
	defer transaction.Rollback()
	eventStatement, errorValue := transaction.PrepareContext(ctx, `
INSERT INTO attendance_events (
	id, mattermost_user_id, mattermost_username, email, display_name, kind, occurred_at, local_date, local_time,
	time_zone_at_event, source, team_id, channel_id, action_post_id, result_post_id, location_id, location_name,
	canceled_at, cancel_reason, repeated_click_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', '', '')`)
	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
	defer eventStatement.Close()
	for memberIndex := 0; memberIndex < 50; memberIndex++ {
		email := fmt.Sprintf("member-%02d@example.com", memberIndex)
		for day := 1; day <= 31; day++ {
			date := time.Date(2026, 7, day, 0, 0, 0, 0, time.UTC)
			for eventIndex, kind := range []string{attendanceKindClockIn, attendanceKindClockOut} {
				occurredAt := date.Add(time.Duration(9+eventIndex*9) * time.Hour)
				eventID := fmt.Sprintf("event-%02d-%02d-%d", memberIndex, day, eventIndex)
				if _, errorValue := eventStatement.ExecContext(
					ctx,
					eventID,
					fmt.Sprintf("user-%02d", memberIndex),
					fmt.Sprintf("member-%02d", memberIndex),
					email,
					fmt.Sprintf("Member %02d", memberIndex),
					kind,
					occurredAt.Format(time.RFC3339Nano),
					date.Format("2006-01-02"),
					occurredAt.Format("15:04:05"),
					"UTC",
					attendanceSourceMattermostButton,
					"team-1",
					"attendance-channel",
					"action-post",
					"result-"+eventID,
					"office",
					"Office",
				); errorValue != nil {
					testContext.Fatal(errorValue)
				}
			}
		}
	}
	for memberIndex := 0; memberIndex < 50; memberIndex++ {
		email := fmt.Sprintf("member-%02d@example.com", memberIndex)
		if _, errorValue := transaction.ExecContext(ctx, `
INSERT INTO attendance_absence_ranges (
	id, email, kind, start_date, end_date, reason, created_by, created_at, updated_at, canceled_at, replaced_by
) VALUES (?, ?, 'annual', '2026-07-15', '2026-07-15', 'Vacation', ?, '2026-07-01T00:00:00Z', '2026-07-01T00:00:00Z', '', '')`,
			fmt.Sprintf("absence-%02d", memberIndex),
			email,
			email,
		); errorValue != nil {
			testContext.Fatal(errorValue)
		}
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		testContext.Fatal(errorValue)
	}
}

func seedAttendanceSummaryHistoricalBenchmark(testContext testing.TB, service *Service) {
	testContext.Helper()
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
	defer database.Close()
	_, errorValue = database.Exec(`
WITH RECURSIVE
months(value) AS (VALUES(1) UNION ALL SELECT value + 1 FROM months WHERE value < 12),
members(value) AS (VALUES(0) UNION ALL SELECT value + 1 FROM members WHERE value < 49),
days(value) AS (VALUES(1) UNION ALL SELECT value + 1 FROM days WHERE value < 28),
event_kinds(event_index, kind, hour) AS (VALUES(0, 'clock_in', 9), (1, 'clock_out', 18))
INSERT INTO attendance_events (
	id, mattermost_user_id, mattermost_username, email, display_name, kind, occurred_at, local_date, local_time,
	time_zone_at_event, source, team_id, channel_id, action_post_id, result_post_id, location_id, location_name,
	canceled_at, cancel_reason, repeated_click_at
) SELECT
	printf('historical-event-%02d-%02d-%02d-%d', months.value, members.value, days.value, event_kinds.event_index),
	printf('user-%02d', members.value), printf('member-%02d', members.value),
	printf('member-%02d@example.com', members.value), printf('Member %02d', members.value), event_kinds.kind,
	printf('2026-%02d-%02dT%02d:00:00Z', months.value, days.value, event_kinds.hour),
	printf('2026-%02d-%02d', months.value, days.value), printf('%02d:00:00', event_kinds.hour),
	'UTC', 'mattermost_button', 'team-1', 'attendance-channel', 'action-post',
	printf('historical-result-%02d-%02d-%02d-%d', months.value, members.value, days.value, event_kinds.event_index),
	'office', 'Office', '', '', ''
FROM months
CROSS JOIN members
CROSS JOIN days
CROSS JOIN event_kinds
WHERE months.value != 7`)
	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
}
