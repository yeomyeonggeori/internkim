package admind

import (
	"context"
	"testing"
	"time"
)

func TestPruneResolvedCalendarDeleteIntentsPreservesPendingAndRecentRows(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	now := time.Now().UTC()
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	rows := []struct {
		operationID string
		status      string
		resolvedAt  time.Time
	}{
		{operationID: "old-executed", status: calendarDeleteIntentStatusExecuted, resolvedAt: now.Add(-31 * 24 * time.Hour)},
		{operationID: "recent-conflicted", status: calendarDeleteIntentStatusConflicted, resolvedAt: now.Add(-29 * 24 * time.Hour)},
		{operationID: "old-pending", status: calendarDeleteIntentStatusPending, resolvedAt: now.Add(-31 * 24 * time.Hour)},
	}
	for _, row := range rows {
		if _, errorValue := database.ExecContext(ctx, `
INSERT INTO calendar_delete_intents(
	operation_id, event_id, client_id, sequence, expected_updated_at, requested_at, execute_at,
	status, resolved_at, next_attempt_at
) VALUES(?, ?, ?, 1, ?, ?, ?, ?, ?, ?)`,
			row.operationID,
			"event-"+row.operationID,
			"client",
			now.Format(time.RFC3339Nano),
			now.Format(time.RFC3339Nano),
			now.Format(time.RFC3339Nano),
			row.status,
			row.resolvedAt.Format(time.RFC3339Nano),
			now.Format(time.RFC3339Nano),
		); errorValue != nil {
			database.Close()
			t.Fatal(errorValue)
		}
	}
	database.Close()

	if errorValue := service.pruneResolvedCalendarDeleteIntents(ctx, now); errorValue != nil {
		t.Fatal(errorValue)
	}

	assertCalendarDeleteIntentExists(t, service, "old-executed", false)
	assertCalendarDeleteIntentExists(t, service, "recent-conflicted", true)
	assertCalendarDeleteIntentExists(t, service, "old-pending", true)
}

func assertCalendarDeleteIntentExists(t *testing.T, service *Service, operationID string, expected bool) {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	_, found, errorValue := readCalendarDeleteIntentWithRunner(context.Background(), database, operationID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if found != expected {
		t.Fatalf("operation %q found=%v expected=%v", operationID, found, expected)
	}
}
