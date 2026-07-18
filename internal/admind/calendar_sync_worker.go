package admind

import (
	"context"
	"log/slog"
	"time"
)

const (
	calendarSyncSafetyInterval = 60 * time.Minute
	calendarSyncCycleTimeout   = 2 * time.Minute
	calendarPullCacheTTL       = 1 * time.Minute
)

type calendarSyncCycleResult struct {
	Changed            bool
	PullAttempted      bool
	PullSkippedByCache bool
	PushFailed         bool
	PullFailed         bool
	SkippedByRunning   bool
}

func (service *Service) startCalendarSyncWorker(ctx context.Context) {
	if service.Configuration.CalendarSyncDisabled {
		return
	}
	go service.runCalendarSyncLoop(ctx, service.runCalendarSyncCycle, calendarSyncSafetyInterval)
}

func (service *Service) runCalendarSyncLoop(ctx context.Context, runCycle func(context.Context) bool, interval time.Duration) {
	runCycle(ctx)
	timer := time.NewTimer(interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-service.calendarSyncWakeUp:
			stopCalendarSyncTimer(timer)
			runCycle(ctx)
			timer.Reset(interval)
		case <-timer.C:
			runCycle(ctx)
			timer.Reset(interval)
		}
	}
}

func (service *Service) runCalendarSyncCycle(ctx context.Context) bool {
	result := service.runCalendarSyncCycleWithHooks(ctx, time.Now, service.pullGoogleCalendarChanges, service.pushPendingCalendarOutbox, false)
	return result.Changed
}

func (service *Service) runCalendarUserSyncCycle(ctx context.Context) calendarSyncCycleResult {
	return service.runCalendarSyncCycleWithHooks(ctx, time.Now, service.pullGoogleCalendarChanges, service.pushPendingCalendarOutbox, true)
}

func (service *Service) runCalendarSyncCycleWithHooks(
	ctx context.Context,
	now func() time.Time,
	pull func(context.Context) (bool, error),
	push func(context.Context) (map[string]struct{}, error),
	allowPull bool,
) calendarSyncCycleResult {
	if !service.calendarSyncCycleMutex.TryLock() {
		slog.WarnContext(ctx, "calendar sync skipped", "reason", "previous cycle still running")
		return calendarSyncCycleResult{SkippedByRunning: true}
	}
	defer service.calendarSyncCycleMutex.Unlock()
	cycleCtx, cancel := context.WithTimeout(ctx, calendarSyncCycleTimeout)
	defer cancel()
	nowValue := now()
	result := calendarSyncCycleResult{}
	_, errorValue := push(cycleCtx)
	if errorValue != nil {
		result.PushFailed = true
		slog.WarnContext(ctx, "calendar push failed", "error", errorValue)
	}
	if allowPull && service.shouldRunCalendarPull(nowValue) {
		result.PullAttempted = true
		pulled, errorValue := pull(cycleCtx)
		if errorValue != nil {
			result.PullFailed = true
			slog.WarnContext(ctx, "calendar pull failed", "error", errorValue)
		} else {
			result.Changed = pulled
			service.markCalendarPullCompleted(now())
			if pulled && !result.PushFailed {
				_, errorValue = push(cycleCtx)
				if errorValue != nil {
					result.PushFailed = true
					slog.WarnContext(ctx, "calendar push after pull failed", "error", errorValue)
				}
			}
		}
	} else if allowPull {
		result.PullSkippedByCache = true
	}
	return result
}

func (result calendarSyncCycleResult) Succeeded() bool {
	return !result.PushFailed && !result.PullFailed && !result.SkippedByRunning
}

func (service *Service) shouldRunCalendarPull(now time.Time) bool {
	service.calendarPullCacheMutex.Lock()
	defer service.calendarPullCacheMutex.Unlock()
	if service.lastCalendarPullAt.IsZero() {
		return true
	}
	return now.Sub(service.lastCalendarPullAt) >= calendarPullCacheTTL
}

func (service *Service) markCalendarPullCompleted(now time.Time) {
	service.calendarPullCacheMutex.Lock()
	defer service.calendarPullCacheMutex.Unlock()
	service.lastCalendarPullAt = now
}

func stopCalendarSyncTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func (service *Service) signalCalendarSyncWakeUp() {
	if service.calendarSyncWakeUp == nil {
		return
	}
	select {
	case service.calendarSyncWakeUp <- struct{}{}:
	default:
	}
}
