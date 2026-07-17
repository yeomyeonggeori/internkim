package admind

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCalendarDeleteIntentWorkerRecoversPendingIntentAfterRestart(t *testing.T) {
	service := newCalendarTestService(t)
	event := seedCalendarDeleteIntentEvent(t, service, "delete-intent-worker-restart")
	createCalendarDeleteIntentForTest(t, service, event, "worker-restart-operation", "page-a", 2, time.Now().UTC().Add(-calendarDeleteIntentDelay-time.Second))
	restartedService := NewService(service.Configuration)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		restartedService.runCalendarDeleteIntentLoop(ctx)
		close(done)
	}()
	waitForCalendarDeleteIntentCondition(t, 2*time.Second, func() bool {
		projection, found, errorValue := restartedService.readCalendarEventProjectionByID(context.Background(), event.ID)
		return errorValue == nil && found && projection.IsDeleted
	})
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("delete intent worker did not stop after restart test cancellation")
	}
}

func TestCalendarDeleteIntentWorkerUsesDynamicExpiryTimer(t *testing.T) {
	service := newCalendarTestService(t)
	event := seedCalendarDeleteIntentEvent(t, service, "delete-intent-worker-expiry")
	intent := createCalendarDeleteIntentForTest(t, service, event, "worker-expiry-operation", "page-a", 2, time.Now().UTC())
	executeAt := time.Now().UTC().Add(400 * time.Millisecond)
	setCalendarDeleteIntentSchedule(t, service, intent.OperationID, executeAt)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		service.runCalendarDeleteIntentLoop(ctx)
		close(done)
	}()
	time.Sleep(80 * time.Millisecond)
	if _, found, errorValue := service.readCalendarEventByID(context.Background(), event.ID); errorValue != nil || !found {
		cancel()
		t.Fatalf("event before expiry found = %v error = %v", found, errorValue)
	}
	waitForCalendarDeleteIntentCondition(t, 2*time.Second, func() bool {
		projection, found, errorValue := service.readCalendarEventProjectionByID(context.Background(), event.ID)
		return errorValue == nil && found && projection.IsDeleted
	})
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("delete intent worker did not stop after expiry test cancellation")
	}
}

func TestCalendarDeleteIntentCreationWakesIdleWorker(t *testing.T) {
	service := newCalendarTestService(t)
	event := seedCalendarDeleteIntentEvent(t, service, "delete-intent-worker-wake")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		service.runCalendarDeleteIntentLoop(ctx)
		close(done)
	}()
	time.Sleep(100 * time.Millisecond)
	createCalendarDeleteIntentForTest(t, service, event, "worker-wake-operation", "page-a", 2, time.Now().UTC().Add(-calendarDeleteIntentDelay-time.Second))

	waitForCalendarDeleteIntentCondition(t, time.Second, func() bool {
		projection, found, errorValue := service.readCalendarEventProjectionByID(context.Background(), event.ID)
		return errorValue == nil && found && projection.IsDeleted
	})
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("delete intent worker did not stop after wake test cancellation")
	}
}

func TestCalendarDeleteIntentCancellationSignalsWorker(t *testing.T) {
	service := newCalendarTestService(t)
	event := seedCalendarDeleteIntentEvent(t, service, "delete-intent-worker-cancel-wake")
	intent := createCalendarDeleteIntentForTest(t, service, event, "worker-cancel-wake-operation", "page-a", 2, time.Now().UTC())
	select {
	case <-service.calendarDeleteIntentWakeUp:
	case <-time.After(time.Second):
		t.Fatal("create did not signal delete intent worker")
	}

	if errorValue := service.cancelCalendarDeleteIntent(context.Background(), event.ID, intent.OperationID, "page-a", 3, time.Now().UTC()); errorValue != nil {
		t.Fatal(errorValue)
	}
	select {
	case <-service.calendarDeleteIntentWakeUp:
	case <-time.After(time.Second):
		t.Fatal("cancellation did not signal delete intent worker")
	}
}

func TestCalendarDeleteIntentWorkerRetriesTransientPersistenceFailure(t *testing.T) {
	service := newCalendarTestService(t)
	event := seedCalendarDeleteIntentEvent(t, service, "delete-intent-worker-retry")
	intent := createCalendarDeleteIntentForTest(t, service, event, "worker-retry-operation", "page-a", 2, time.Now().UTC().Add(-calendarDeleteIntentDelay-time.Second))
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(context.Background(), `CREATE TRIGGER fail_delete_intent_worker_update BEFORE UPDATE OF deleted_at ON calendar_events BEGIN SELECT RAISE(ABORT, 'worker persistence blocked'); END`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	database.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		service.runCalendarDeleteIntentLoop(ctx)
		close(done)
	}()
	waitForCalendarDeleteIntentCondition(t, 2*time.Second, func() bool {
		storedIntent := readCalendarDeleteIntentForTest(t, service, intent.OperationID)
		return storedIntent.AttemptCount == 1 && strings.Contains(storedIntent.LastError, "worker persistence blocked")
	})
	database, errorValue = service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		cancel()
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(context.Background(), `DROP TRIGGER fail_delete_intent_worker_update`); errorValue != nil {
		database.Close()
		cancel()
		t.Fatal(errorValue)
	}
	database.Close()
	waitForCalendarDeleteIntentCondition(t, 3*time.Second, func() bool {
		projection, found, errorValue := service.readCalendarEventProjectionByID(context.Background(), event.ID)
		return errorValue == nil && found && projection.IsDeleted
	})
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("delete intent worker did not stop after retry test cancellation")
	}
}

func TestCalendarDeleteIntentWorkerStopsWithContext(t *testing.T) {
	service := newCalendarTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		service.runCalendarDeleteIntentLoop(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("delete intent worker did not stop after context cancellation")
	}
}

func setCalendarDeleteIntentSchedule(t *testing.T, service *Service, operationID string, executeAt time.Time) {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	formattedExecuteAt := executeAt.UTC().Format(time.RFC3339Nano)
	if _, errorValue := database.ExecContext(context.Background(), `UPDATE calendar_delete_intents SET execute_at = ?, next_attempt_at = ? WHERE operation_id = ?`, formattedExecuteAt, formattedExecuteAt, operationID); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func waitForCalendarDeleteIntentCondition(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !condition() {
		t.Fatalf("condition was not met within %s", timeout)
	}
}
