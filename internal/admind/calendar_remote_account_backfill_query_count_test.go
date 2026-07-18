package admind

import (
	"context"
	"database/sql"
	"testing"
)

type countingCalendarSQLRunner struct {
	runner     calendarSQLRunner
	queryCount int
}

func (runner *countingCalendarSQLRunner) ExecContext(ctx context.Context, query string, arguments ...any) (sql.Result, error) {
	return runner.runner.ExecContext(ctx, query, arguments...)
}

func (runner *countingCalendarSQLRunner) QueryContext(ctx context.Context, query string, arguments ...any) (*sql.Rows, error) {
	runner.queryCount++
	return runner.runner.QueryContext(ctx, query, arguments...)
}

func TestReadCalendarBackfillStateUsesConstantQueryCount(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	for _, eventID := range []string{"backfill-query-one", "backfill-query-two", "backfill-query-three"} {
		if errorValue := service.writeCalendarEventWithSource(ctx, newLocalTestCalendarEvent(eventID, eventID), calendarSourcePull); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	runner := &countingCalendarSQLRunner{runner: database}

	events, errorValue := readCalendarBackfillState(ctx, runner, "account", "/calendars/company/")

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != 3 {
		t.Fatalf("events = %d", len(events))
	}
	if runner.queryCount > 4 {
		t.Fatalf("query count = %d, expected at most 4", runner.queryCount)
	}
}
