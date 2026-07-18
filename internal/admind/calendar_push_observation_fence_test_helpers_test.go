package admind

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func seedCalendarPushObservationFenceForTest(t *testing.T, service *Service, accountID string, calendarURL string, eventUID string) {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	_, errorValue = database.ExecContext(context.Background(), `
INSERT INTO calendar_push_observation_fences (account_id, calendar_url, event_uid, created_at)
VALUES (?, ?, ?, ?)`, accountID, calendarURL, eventUID, time.Now().UTC().Format(time.RFC3339Nano))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
}

func readCalendarPushObservationFenceUIDsForTest(t *testing.T, service *Service, accountID string, calendarURL string) map[string]struct{} {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(context.Background(), `
SELECT event_uid
FROM calendar_push_observation_fences
WHERE account_id = ? AND calendar_url = ?`, accountID, calendarURL)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer rows.Close()
	result := map[string]struct{}{}
	for rows.Next() {
		var eventUID string
		if errorValue := rows.Scan(&eventUID); errorValue != nil {
			t.Fatal(errorValue)
		}
		result[eventUID] = struct{}{}
	}
	if errorValue := rows.Err(); errorValue != nil {
		t.Fatal(errorValue)
	}
	return result
}

type calendarPushObservationFenceCountingRunner struct {
	executionCalls int
	query          string
	arguments      []any
}

func (runner *calendarPushObservationFenceCountingRunner) ExecContext(ctx context.Context, query string, arguments ...any) (sql.Result, error) {
	runner.executionCalls++
	runner.query = query
	runner.arguments = append([]any(nil), arguments...)
	return calendarPushObservationFenceTestResult{}, nil
}

func (runner *calendarPushObservationFenceCountingRunner) QueryContext(ctx context.Context, query string, arguments ...any) (*sql.Rows, error) {
	return nil, nil
}

type calendarPushObservationFenceTestResult struct{}

func (calendarPushObservationFenceTestResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (calendarPushObservationFenceTestResult) RowsAffected() (int64, error) {
	return 0, nil
}
