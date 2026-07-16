package admind

import (
	"context"
	"sync"
)

type calendarEventWindowCacheBuildResult struct {
	events            []calendarEvent
	errorValue        error
	shouldRetry       bool
	sourceRevision    int64
	hasSourceRevision bool
}

type calendarEventWindowCacheBuildFlight struct {
	completion    chan struct{}
	result        calendarEventWindowCacheBuildResult
	followerCount int
}

type calendarEventWindowCacheBuildCoordinator struct {
	mutex       sync.Mutex
	flightByKey map[string]*calendarEventWindowCacheBuildFlight
}

func (coordinator *calendarEventWindowCacheBuildCoordinator) begin(cacheKey string) (bool, *calendarEventWindowCacheBuildFlight) {
	coordinator.mutex.Lock()
	defer coordinator.mutex.Unlock()
	if flight, found := coordinator.flightByKey[cacheKey]; found {
		flight.followerCount++
		return false, flight
	}
	if coordinator.flightByKey == nil {
		coordinator.flightByKey = map[string]*calendarEventWindowCacheBuildFlight{}
	}
	flight := &calendarEventWindowCacheBuildFlight{completion: make(chan struct{})}
	coordinator.flightByKey[cacheKey] = flight
	return true, flight
}

func (coordinator *calendarEventWindowCacheBuildCoordinator) complete(cacheKey string, flight *calendarEventWindowCacheBuildFlight, result calendarEventWindowCacheBuildResult) {
	clonedResult := cloneCalendarEventWindowCacheBuildResult(result)
	coordinator.mutex.Lock()
	defer coordinator.mutex.Unlock()
	currentFlight, found := coordinator.flightByKey[cacheKey]
	if !found || currentFlight != flight {
		return
	}
	delete(coordinator.flightByKey, cacheKey)
	flight.result = clonedResult
	close(flight.completion)
}

func (flight *calendarEventWindowCacheBuildFlight) wait(ctx context.Context) (calendarEventWindowCacheBuildResult, error) {
	select {
	case <-flight.completion:
		return cloneCalendarEventWindowCacheBuildResult(flight.result), nil
	case <-ctx.Done():
		return calendarEventWindowCacheBuildResult{}, ctx.Err()
	}
}

func cloneCalendarEventWindowCacheBuildResult(result calendarEventWindowCacheBuildResult) calendarEventWindowCacheBuildResult {
	result.events = cloneCalendarEventWindowCacheEvents(result.events)
	return result
}

func cloneCalendarEventWindowCacheEvents(events []calendarEvent) []calendarEvent {
	if events == nil {
		return nil
	}
	clonedEvents := make([]calendarEvent, len(events))
	copy(clonedEvents, events)
	for index := range clonedEvents {
		if events[index].People != nil {
			clonedEvents[index].People = append([]string{}, events[index].People...)
		}
		if events[index].Participants != nil {
			clonedEvents[index].Participants = append([]calendarParticipant{}, events[index].Participants...)
		}
	}
	return clonedEvents
}
