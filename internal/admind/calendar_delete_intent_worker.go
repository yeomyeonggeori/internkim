package admind

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

func (service *Service) startCalendarDeleteIntentWorker(ctx context.Context) {
	go service.runCalendarDeleteIntentLoop(ctx)
}

func (service *Service) runCalendarDeleteIntentLoop(ctx context.Context) {
	service.runCalendarDeleteIntentLoopWithWaiter(ctx, waitForCalendarDeleteIntentWorker)
}

func (service *Service) runCalendarDeleteIntentLoopWithWaiter(ctx context.Context, waiter func(context.Context, time.Duration, <-chan struct{}) bool) {
	for {
		now := time.Now().UTC()
		if errorValue := service.processDueCalendarDeleteIntents(ctx, now); errorValue != nil && ctx.Err() == nil {
			slog.WarnContext(ctx, "calendar delete intent processing failed", "error", errorValue)
		}
		nextAttemptAt, found, errorValue := service.nextCalendarDeleteIntentAttemptAt(ctx)
		if ctx.Err() != nil {
			return
		}
		if errorValue != nil {
			slog.WarnContext(ctx, "calendar delete intent schedule read failed", "error", errorValue)
		}
		waitDuration := calendarDeleteIntentWorkerWaitDuration(time.Now().UTC(), nextAttemptAt, found, errorValue)
		if waiter(ctx, waitDuration, service.calendarDeleteIntentWakeUp) {
			return
		}
	}
}

func waitForCalendarDeleteIntentWorker(ctx context.Context, waitDuration time.Duration, wakeUp <-chan struct{}) bool {
	timer := time.NewTimer(waitDuration)
	select {
	case <-ctx.Done():
		stopCalendarDeleteIntentTimer(timer)
		return true
	case <-wakeUp:
		stopCalendarDeleteIntentTimer(timer)
		return false
	case <-timer.C:
		return false
	}
}

func calendarDeleteIntentWorkerWaitDuration(now time.Time, nextAttemptAt time.Time, found bool, scheduleError error) time.Duration {
	if scheduleError != nil {
		return calendarDeleteIntentRetryBaseDelay
	}
	if !found {
		return calendarDeleteIntentCleanupInterval
	}
	waitDuration := nextAttemptAt.Sub(now)
	if waitDuration < 0 {
		return 0
	}
	return waitDuration
}

func (service *Service) nextCalendarDeleteIntentAttemptAt(ctx context.Context) (time.Time, bool, error) {
	intents, errorValue := service.listPendingCalendarDeleteIntents(ctx)
	if errorValue != nil {
		return time.Time{}, false, errorValue
	}
	var nextAttemptAt time.Time
	for _, intent := range intents {
		executeAt := parseCalendarConflictTime(intent.ExecuteAt)
		retryAt := parseCalendarConflictTime(intent.NextAttemptAt)
		if executeAt.IsZero() || retryAt.IsZero() {
			return time.Time{}, false, fmt.Errorf("calendar delete intent %q schedule is invalid", intent.OperationID)
		}
		candidate := executeAt
		if retryAt.After(candidate) {
			candidate = retryAt
		}
		if nextAttemptAt.IsZero() || candidate.Before(nextAttemptAt) {
			nextAttemptAt = candidate
		}
	}
	return nextAttemptAt, !nextAttemptAt.IsZero(), nil
}

func (service *Service) signalCalendarDeleteIntentWakeUp() {
	if service.calendarDeleteIntentWakeUp == nil {
		return
	}
	select {
	case service.calendarDeleteIntentWakeUp <- struct{}{}:
	default:
	}
}

func stopCalendarDeleteIntentTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}
