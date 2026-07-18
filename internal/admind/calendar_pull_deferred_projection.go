package admind

import (
	"context"
	"log/slog"
)

type calendarPullDeferredProjectionQueue struct {
	eventIDs []string
	seenIDs  map[string]struct{}
}

func (service *Service) runCalendarPullLocked(ctx context.Context, queue *calendarPullDeferredProjectionQueue, operation func()) {
	service.calendarStoreWriteMutex.Lock()
	defer func() {
		service.calendarStoreWriteMutex.Unlock()
		service.runCalendarPullDeferredProjections(ctx, queue)
	}()
	operation()
}

func (queue *calendarPullDeferredProjectionQueue) add(eventID string) {
	if queue.seenIDs == nil {
		queue.seenIDs = map[string]struct{}{}
	}
	if _, found := queue.seenIDs[eventID]; found {
		return
	}
	queue.seenIDs[eventID] = struct{}{}
	queue.eventIDs = append(queue.eventIDs, eventID)
}

func (service *Service) runCalendarPullDeferredProjections(ctx context.Context, queue *calendarPullDeferredProjectionQueue) {
	for _, eventID := range queue.eventIDs {
		service.reconcileCalendarEventNotifications(ctx, eventID)
		if errorValue := service.applyCalendarMattermostProjectionByID(ctx, eventID); errorValue != nil {
			slog.WarnContext(ctx, "apply deferred calendar pull projection failed", "event_id", eventID, "error", errorValue)
		}
	}
}
