package admind

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestPersistObservedCalendarRemoteEventStateBatchSkipsDatabaseForEmptyEvents(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue, cancel := context.WithCancel(context.Background())
	cancel()
	if errorValue := service.persistObservedCalendarRemoteEventStateBatch(contextValue, "account", "/calendar/", nil, time.Now()); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestPersistObservedCalendarRemoteEventStateBatchRollsBackStatesAndLogicalClocks(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	account := seedAccountWithDiscovery(t, service)
	target := activeRemoteCalendarTarget(account)
	firstEvent := newLocalTestCalendarEvent("observed-batch-first", "First")
	secondEvent := newLocalTestCalendarEvent("observed-batch-second", "Second")
	database, errorValue := service.openCalendarDatabase(contextValue)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = database.ExecContext(contextValue, `
CREATE TRIGGER fail_second_observed_batch_state
BEFORE INSERT ON calendar_remote_event_sync_state
WHEN NEW.event_uid = 'observed-batch-second@internkim'
BEGIN
	SELECT RAISE(ABORT, 'forced observed batch failure');
END`)
	database.Close()
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	reservedObservedAt := time.Date(2036, 7, 16, 15, 0, 0, 0, time.UTC)
	errorValue = service.persistObservedCalendarRemoteEventStateBatch(
		contextValue,
		account.ID,
		target.CalendarURL,
		[]calendarEvent{firstEvent, secondEvent},
		reservedObservedAt,
	)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "forced observed batch failure") {
		t.Fatalf("batch error=%v", errorValue)
	}
	for _, event := range []calendarEvent{firstEvent, secondEvent} {
		_, found, readError := service.readCalendarRemoteEventState(contextValue, account.ID, target.CalendarURL, event.UID)
		if readError != nil {
			t.Fatal(readError)
		}
		if found {
			t.Fatalf("remote event state persisted for %s", event.UID)
		}
	}
	database, errorValue = service.openCalendarDatabase(contextValue)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var logicalClockCount int
	if errorValue := database.QueryRowContext(contextValue, `SELECT COUNT(*) FROM calendar_event_logical_clocks WHERE event_uid IN (?, ?)`, firstEvent.UID, secondEvent.UID).Scan(&logicalClockCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if logicalClockCount != 0 {
		t.Fatalf("logical clock count=%d want 0", logicalClockCount)
	}
}

func TestPersistObservedCalendarRemoteEventStateBatchPreservesObservedStateSemantics(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	account := seedAccountWithDiscovery(t, service)
	target := activeRemoteCalendarTarget(account)
	event := newLocalTestCalendarEvent("observed-batch-semantics", "Observed")
	priorRemoteModifiedAt := "2036-07-16T13:00:00Z"
	if errorValue := service.upsertCalendarRemoteEventState(contextValue, calendarRemoteEventState{
		AccountID:         account.ID,
		CalendarURL:       target.CalendarURL,
		EventUID:          event.UID,
		RemoteModifiedAt:  priorRemoteModifiedAt,
		LastSeenAt:        "2036-07-16T13:30:00Z",
		MissingDetectedAt: "2036-07-16T14:00:00Z",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	reservedObservedAt := time.Date(2036, 7, 16, 15, 0, 0, 0, time.UTC)
	if errorValue := service.persistObservedCalendarRemoteEventStateBatch(
		contextValue,
		account.ID,
		target.CalendarURL,
		[]calendarEvent{event},
		reservedObservedAt,
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	state, found, errorValue := service.readCalendarRemoteEventState(contextValue, account.ID, target.CalendarURL, event.UID)
	if errorValue != nil || !found {
		t.Fatalf("state found=%v error=%v", found, errorValue)
	}
	if state.RemoteModifiedAt != priorRemoteModifiedAt {
		t.Fatalf("remote modified at=%q want %q", state.RemoteModifiedAt, priorRemoteModifiedAt)
	}
	if parseCalendarConflictTime(state.LastSeenAt) != reservedObservedAt {
		t.Fatalf("last seen at=%q want %s", state.LastSeenAt, reservedObservedAt)
	}
	if state.MissingDetectedAt != "" {
		t.Fatalf("missing detected at=%q", state.MissingDetectedAt)
	}
}
