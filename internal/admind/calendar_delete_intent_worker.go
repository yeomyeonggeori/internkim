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
	for {
		now := time.Now().UTC()
		if errorValue := service.processDueCalendarDeleteIntents(ctx, now); errorValue != nil && ctx.Err() == nil {
			slog.WarnContext(ctx, "calendar delete intent processing failed", "error", errorValue)
		}
		nextAttemptAt, found, errorValue := service.nextCalendarDeleteIntentAttemptAt(ctx)
		if ctx.Err() != nil {
			return
		}
		waitDuration := calendarDeleteIntentRetryBaseDelay
		if errorValue == nil && found {
			waitDuration = time.Until(nextAttemptAt)
			if waitDuration < 0 {
				waitDuration = 0
			}
		}
		if errorValue == nil && !found {
			select {
			case <-ctx.Done():
				return
			case <-service.calendarDeleteIntentWakeUp:
				continue
			}
		}
		if errorValue != nil {
			slog.WarnContext(ctx, "calendar delete intent schedule read failed", "error", errorValue)
		}
		timer := time.NewTimer(waitDuration)
		select {
		case <-ctx.Done():
			stopCalendarDeleteIntentTimer(timer)
			return
		case <-service.calendarDeleteIntentWakeUp:
			stopCalendarDeleteIntentTimer(timer)
		case <-timer.C:
		}
	}
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
