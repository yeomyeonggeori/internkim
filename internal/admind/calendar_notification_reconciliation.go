package admind

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const (
	calendarNotificationReconciliationTimeout = 30 * time.Second
	calendarNotificationRetryBaseDelay        = time.Second
	calendarNotificationRetryMaximumDelay     = time.Minute
	calendarNotificationWorkerCount           = 4
	calendarNotificationInputBufferSize       = 64
	calendarNotificationLiveBurstLimit        = 8
	calendarNotificationSchedulerInterval     = time.Second
)

type calendarNotificationReconciliationState struct {
	pending        bool
	queued         bool
	queuedLive     bool
	running        bool
	retryScheduled bool
	livePending    bool
	generation     uint64
	retryDelay     time.Duration
	retryAt        time.Time
	reconciler     calendarNotificationReconciler
}

type calendarNotificationReconciliationJob struct {
	eventID    string
	state      *calendarNotificationReconciliationState
	generation uint64
	live       bool
}

type calendarNotificationReconciler func(context.Context, string) error

func (service *Service) startCalendarNotificationReconciliation(ctx context.Context) {
	service.startCalendarNotificationWorkers(ctx)
	go func() {
		if errorValue := service.enqueuePersistedCalendarNotificationReconciliations(ctx); errorValue != nil && ctx.Err() == nil {
			slog.WarnContext(ctx, "calendar notification startup reconciliation failed", "error", errorValue)
		}
	}()
}

func (service *Service) startCalendarNotificationWorkers(ctx context.Context) {
	service.calendarNotificationMutex.Lock()
	defer service.calendarNotificationMutex.Unlock()
	if service.calendarNotificationLive != nil {
		return
	}
	liveInput := make(chan calendarNotificationReconciliationJob, calendarNotificationInputBufferSize)
	repairInput := make(chan calendarNotificationReconciliationJob, calendarNotificationInputBufferSize)
	jobs := make(chan calendarNotificationReconciliationJob)
	service.calendarNotificationLive = liveInput
	service.calendarNotificationRepair = repairInput
	service.calendarNotificationCtx = ctx
	service.calendarNotificationGroup.Add(calendarNotificationWorkerCount + 2)
	go func() {
		defer service.calendarNotificationGroup.Done()
		dispatchCalendarNotificationReconciliations(ctx, liveInput, repairInput, jobs)
	}()
	go func() {
		defer service.calendarNotificationGroup.Done()
		service.scheduleCalendarNotificationReconciliations(ctx)
	}()
	for range calendarNotificationWorkerCount {
		go func() {
			defer service.calendarNotificationGroup.Done()
			service.runCalendarNotificationReconciliationWorker(ctx, jobs)
		}()
	}
}

func (service *Service) reconcileCalendarEventNotifications(ctx context.Context, eventID string) {
	service.enqueueCalendarNotificationReconciliationWithContext(ctx, eventID, service.reconcileCalendarEventNotificationsOnce, true)
}

func (service *Service) enqueueCalendarNotificationReconciliationWithContext(ctx context.Context, eventID string, reconciler calendarNotificationReconciler, isLive bool) {
	normalizedEventID := strings.TrimSpace(eventID)
	service.calendarNotificationMutex.Lock()
	if service.calendarNotificationLive == nil {
		service.calendarNotificationMutex.Unlock()
		attemptContext, cancel := context.WithTimeout(ctx, calendarNotificationReconciliationTimeout)
		errorValue := reconciler(attemptContext, normalizedEventID)
		cancel()
		if errorValue != nil {
			slog.WarnContext(ctx, "calendar notification reconciliation failed before worker startup", "event_id", normalizedEventID, "error", errorValue)
		}
		return
	}
	state := service.calendarNotificationStates[normalizedEventID]
	if state == nil {
		state = &calendarNotificationReconciliationState{retryDelay: calendarNotificationRetryBaseDelay}
		service.calendarNotificationStates[normalizedEventID] = state
	}
	state.pending = true
	state.reconciler = reconciler
	state.livePending = state.livePending || isLive
	job, shouldQueue := service.queueCalendarNotificationReconciliationLocked(normalizedEventID, state, isLive)
	liveInput := service.calendarNotificationLive
	repairInput := service.calendarNotificationRepair
	workerContext := service.calendarNotificationCtx
	service.calendarNotificationMutex.Unlock()
	if shouldQueue {
		service.queueCalendarNotificationReconciliation(workerContext, liveInput, repairInput, job)
	}
}

func (service *Service) queueCalendarNotificationReconciliationLocked(eventID string, state *calendarNotificationReconciliationState, isLive bool) (calendarNotificationReconciliationJob, bool) {
	if state.running || state.retryScheduled {
		return calendarNotificationReconciliationJob{}, false
	}
	if state.queued && (!isLive || state.queuedLive) {
		return calendarNotificationReconciliationJob{}, false
	}
	state.queued = true
	state.queuedLive = isLive
	state.generation++
	return calendarNotificationReconciliationJob{eventID: eventID, state: state, generation: state.generation, live: isLive}, true
}

func (service *Service) reconcileCalendarEventNotificationsOnce(ctx context.Context, eventID string) error {
	event, found, errorValue := service.readCalendarNotificationEventSnapshot(ctx, eventID)
	if errorValue != nil {
		return fmt.Errorf("read calendar notification event: %w", errorValue)
	}
	if found {
		return service.upsertCalendarNotifications(ctx, event)
	}
	if errorValue := service.cancelCalendarNotifications(ctx, eventID); errorValue != nil {
		return fmt.Errorf("cancel calendar notifications: %w", errorValue)
	}
	return nil
}

func (service *Service) readCalendarNotificationEventSnapshot(ctx context.Context, eventID string) (calendarEvent, bool, error) {
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
	return service.readCalendarEventByID(ctx, eventID)
}

func (service *Service) enqueuePersistedCalendarNotificationReconciliations(ctx context.Context) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	if errorValue := service.enqueueStoredCalendarNotificationRows(ctx, database, "SELECT id FROM calendar_events WHERE deleted_at = '' AND end_at >= ? ORDER BY end_at", time.Now().UTC().Format(time.RFC3339)); errorValue != nil {
		return errorValue
	}
	return service.enqueueStoredCalendarNotificationRows(ctx, database, "SELECT event_id FROM calendar_event_notifications WHERE status = 'pending' ORDER BY notify_at, event_id")
}

func (service *Service) enqueueStoredCalendarNotificationRows(ctx context.Context, database calendarSQLRunner, query string, arguments ...any) error {
	rows, errorValue := database.QueryContext(ctx, query, arguments...)
	if errorValue != nil {
		return errorValue
	}
	defer rows.Close()
	for rows.Next() {
		var eventID string
		if errorValue := rows.Scan(&eventID); errorValue != nil {
			return errorValue
		}
		service.enqueueCalendarNotificationReconciliationWithContext(ctx, eventID, service.reconcileCalendarEventNotificationsOnce, false)
	}
	return rows.Err()
}

func nextCalendarNotificationRetryDelay(delay time.Duration) time.Duration {
	if delay >= calendarNotificationRetryMaximumDelay/2 {
		return calendarNotificationRetryMaximumDelay
	}
	return delay * 2
}

func calendarNotificationRetryDelay(eventID string, delay time.Duration) time.Duration {
	jitterWindow := delay / 4
	if jitterWindow <= 0 {
		return delay
	}
	var hash uint64
	for _, character := range eventID {
		hash = hash*33 + uint64(character)
	}
	return delay + time.Duration(hash%uint64(jitterWindow+1))
}
