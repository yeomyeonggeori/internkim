package admind

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestCalendarNotificationReconciliationCoalescesSameEvent(t *testing.T) {
	service := newCalendarTestService(t)
	started := make(chan string, 2)
	release := make(chan struct{})
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	var firstCall sync.Once
	reconciler := func(ctx context.Context, eventID string) error {
		started <- eventID
		firstCall.Do(func() { <-release })
		return nil
	}
	enqueueCalendarNotificationReconciliationForTest(service, "same-event", reconciler)
	select {
	case eventID := <-started:
		if eventID != "same-event" {
			t.Fatalf("first event ID = %q", eventID)
		}
	case <-time.After(time.Second):
		t.Fatal("first event reconciliation did not start")
	}
	enqueueCalendarNotificationReconciliationForTest(service, "same-event", reconciler)
	service.calendarNotificationMutex.Lock()
	state := service.calendarNotificationStates["same-event"]
	isCoalesced := state != nil && state.running && state.pending && !state.queued
	service.calendarNotificationMutex.Unlock()
	if !isCoalesced {
		t.Fatal("same event reconciliation was not coalesced while the first run was active")
	}
	close(release)
	select {
	case eventID := <-started:
		if eventID != "same-event" {
			t.Fatalf("second event ID = %q", eventID)
		}
	case <-time.After(time.Second):
		t.Fatal("coalesced reconciliation did not process the latest state")
	}
	waitForCalendarNotificationReconciliation(t, service, "same-event")
}

func TestCalendarNotificationReconciliationRunsDifferentEventsConcurrently(t *testing.T) {
	service := newCalendarTestService(t)
	firstStarted := make(chan struct{})
	secondStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	defer func() {
		select {
		case <-releaseFirst:
		default:
			close(releaseFirst)
		}
	}()
	reconciler := func(ctx context.Context, eventID string) error {
		if eventID == "first-event" {
			close(firstStarted)
			<-releaseFirst
			return nil
		}
		close(secondStarted)
		return nil
	}
	enqueueCalendarNotificationReconciliationForTest(service, "first-event", reconciler)
	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("first event reconciliation did not start")
	}
	enqueueCalendarNotificationReconciliationForTest(service, "second-event", reconciler)
	select {
	case <-secondStarted:
	case <-time.After(time.Second):
		close(releaseFirst)
		t.Fatal("different event reconciliation did not run concurrently")
	}
	close(releaseFirst)
	waitForCalendarNotificationReconciliation(t, service, "first-event")
	waitForCalendarNotificationReconciliation(t, service, "second-event")
}

func TestCalendarNotificationStartupReconcilesPersistedEvents(t *testing.T) {
	service := newCalendarTestService(t)
	event := calendarTestEvent("notification-startup-repair", "Startup repair", "")
	event.StartISO = time.Now().UTC().Add(4 * time.Hour).Truncate(time.Second).Format(time.RFC3339)
	event.EndISO = time.Now().UTC().Add(5 * time.Hour).Truncate(time.Second).Format(time.RFC3339)
	event.ReminderLeadHours = 1
	if errorValue := service.writeCalendarEvent(context.Background(), event); errorValue != nil {
		t.Fatal(errorValue)
	}
	waitForCalendarNotificationReconciliation(t, service, event.ID)
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(context.Background(), "DELETE FROM calendar_event_notifications WHERE event_id = ?", event.ID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	database.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service.startCalendarNotificationReconciliation(ctx)
	waitForCalendarDeleteIntentCondition(t, 2*time.Second, func() bool {
		database, errorValue := service.openCalendarDatabase(context.Background())
		if errorValue != nil {
			return false
		}
		defer database.Close()
		var count int
		if errorValue := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM calendar_event_notifications WHERE event_id = ?", event.ID).Scan(&count); errorValue != nil {
			return false
		}
		return count == 1
	})

	if status := readCalendarNotificationStatus(t, service, event.ID); status != "pending" {
		t.Fatalf("status = %q, expected pending", status)
	}
}

func TestCalendarNotificationReconciliationRetriesTransientFailure(t *testing.T) {
	service := newCalendarTestService(t)
	attempts := make(chan int, 2)
	attemptCount := 0
	reconciler := func(ctx context.Context, eventID string) error {
		attemptCount++
		attempts <- attemptCount
		if attemptCount == 1 {
			return errors.New("transient notification failure")
		}
		return nil
	}
	enqueueCalendarNotificationReconciliationForTest(service, "retry-event", reconciler)
	select {
	case attempt := <-attempts:
		if attempt != 1 {
			t.Fatalf("first attempt = %d", attempt)
		}
	case <-time.After(time.Second):
		t.Fatal("first notification reconciliation did not start")
	}
	waitForCalendarDeleteIntentCondition(t, time.Second, func() bool {
		service.calendarNotificationMutex.Lock()
		defer service.calendarNotificationMutex.Unlock()
		state := service.calendarNotificationStates["retry-event"]
		return state != nil && state.retryScheduled
	})
	service.enqueueReadyCalendarNotificationReconciliations(context.Background(), time.Now().Add(calendarNotificationRetryMaximumDelay))
	select {
	case attempt := <-attempts:
		if attempt != 2 {
			t.Fatalf("retry attempt = %d", attempt)
		}
	case <-time.After(time.Second):
		t.Fatal("notification reconciliation did not retry")
	}
	waitForCalendarNotificationReconciliation(t, service, "retry-event")
}

func TestCalendarNotificationDispatcherMakesProgressUnderContinuousLiveInput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	liveInput := make(chan calendarNotificationReconciliationJob, 128)
	repairInput := make(chan calendarNotificationReconciliationJob, 1)
	jobs := make(chan calendarNotificationReconciliationJob)
	for generation := uint64(1); generation <= 64; generation++ {
		liveInput <- calendarNotificationReconciliationJob{eventID: "live-event", generation: generation, live: true}
	}
	repairInput <- calendarNotificationReconciliationJob{eventID: "repair-event"}
	done := make(chan struct{})
	go func() {
		dispatchCalendarNotificationReconciliations(ctx, liveInput, repairInput, jobs)
		close(done)
	}()
	repairProcessed := false
	for range calendarNotificationLiveBurstLimit + 1 {
		select {
		case job := <-jobs:
			if !job.live {
				repairProcessed = true
			}
		case <-time.After(time.Second):
			t.Fatal("notification dispatcher starved worker output")
		}
		if repairProcessed {
			break
		}
	}
	if !repairProcessed {
		t.Fatal("notification dispatcher starved repair input")
	}
	cancel()
	<-done
}

func TestCalendarNotificationQueueSaturationDefersWithoutBlocking(t *testing.T) {
	service := newCalendarTestService(t)
	liveInput := make(chan calendarNotificationReconciliationJob, 1)
	repairInput := make(chan calendarNotificationReconciliationJob, 1)
	liveInput <- calendarNotificationReconciliationJob{eventID: "occupied"}
	state := &calendarNotificationReconciliationState{pending: true, retryDelay: calendarNotificationRetryBaseDelay}
	service.calendarNotificationMutex.Lock()
	service.calendarNotificationStates["deferred-event"] = state
	job, shouldQueue := service.queueCalendarNotificationReconciliationLocked("deferred-event", state, true)
	service.calendarNotificationMutex.Unlock()
	if !shouldQueue {
		t.Fatal("deferred notification reconciliation was not queued")
	}
	completed := make(chan struct{})
	go func() {
		service.queueCalendarNotificationReconciliation(context.Background(), liveInput, repairInput, job)
		close(completed)
	}()
	select {
	case <-completed:
	case <-time.After(time.Second):
		t.Fatal("saturated notification queue blocked the caller")
	}
	service.calendarNotificationMutex.Lock()
	isDeferred := state.pending && !state.queued
	service.calendarNotificationMutex.Unlock()
	if !isDeferred {
		t.Fatal("saturated notification job was not deferred")
	}
}

func TestCalendarNotificationRetryDelayIsBoundedAndJittered(t *testing.T) {
	if delay := nextCalendarNotificationRetryDelay(calendarNotificationRetryMaximumDelay); delay != calendarNotificationRetryMaximumDelay {
		t.Fatalf("maximum retry delay = %s", delay)
	}
	delay := calendarNotificationRetryDelay("jitter-event", calendarNotificationRetryBaseDelay)
	if delay < calendarNotificationRetryBaseDelay || delay > calendarNotificationRetryBaseDelay+calendarNotificationRetryBaseDelay/4 {
		t.Fatalf("jittered retry delay = %s", delay)
	}
}

func TestCalendarNotificationLiveMutationPromotesQueuedRepair(t *testing.T) {
	service := newCalendarTestService(t)
	state := &calendarNotificationReconciliationState{pending: true, retryDelay: calendarNotificationRetryBaseDelay}
	service.calendarNotificationMutex.Lock()
	repairJob, repairQueued := service.queueCalendarNotificationReconciliationLocked("priority-event", state, false)
	liveJob, liveQueued := service.queueCalendarNotificationReconciliationLocked("priority-event", state, true)
	service.calendarNotificationMutex.Unlock()

	if !repairQueued || !liveQueued {
		t.Fatalf("repair queued=%v live queued=%v", repairQueued, liveQueued)
	}
	if repairJob.live || !liveJob.live || liveJob.generation <= repairJob.generation {
		t.Fatalf("repair job=%+v live job=%+v", repairJob, liveJob)
	}
	if !state.queuedLive || state.generation != liveJob.generation {
		t.Fatalf("promoted state=%+v", state)
	}
}
