package admind

import (
	"context"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func TestCalendarEventWindowCacheCoalescesConcurrentColdMisses(t *testing.T) {
	service := newCalendarTestService(t)
	database, errorValue := service.openCalendarDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	startTime := time.Date(2026, time.July, 16, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(24 * time.Hour)
	cacheRange, cacheable := calendarEventWindowCacheRangeFor(startTime, endTime)
	if !cacheable {
		t.Fatal("test range is not cacheable")
	}
	expectedEvents := []calendarEvent{{ID: "coalesced-result", Title: "Coalesced result"}}
	var sourceReadCount atomic.Int32
	firstSourceReadStarted := make(chan struct{})
	releaseSourceRead := make(chan struct{})
	sourceReader := func(ctx context.Context, rangeStart time.Time, rangeEnd time.Time) ([]calendarEvent, error) {
		if sourceReadCount.Add(1) == 1 {
			close(firstSourceReadStarted)
		}
		select {
		case <-releaseSourceRead:
			return expectedEvents, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	testContext, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	type readResult struct {
		events []calendarEvent
		error  error
	}
	const readerCount = 8
	readResults := make(chan readResult, readerCount)
	go func() {
		events, errorValue := service.readCalendarEventWindowWithSourceReader(testContext, startTime, endTime, sourceReader)
		readResults <- readResult{events: events, error: errorValue}
	}()
	select {
	case <-firstSourceReadStarted:
	case <-testContext.Done():
		t.Fatal(testContext.Err())
	}
	for range readerCount - 1 {
		go func() {
			events, errorValue := service.readCalendarEventWindowWithSourceReader(testContext, startTime, endTime, sourceReader)
			readResults <- readResult{events: events, error: errorValue}
		}()
	}
	waitForCalendarEventWindowCacheFollowers(t, testContext, service, cacheRange.Key, readerCount-1)
	close(releaseSourceRead)
	for range readerCount {
		select {
		case result := <-readResults:
			if result.error != nil {
				t.Fatal(result.error)
			}
			if !reflect.DeepEqual(result.events, expectedEvents) {
				t.Fatalf("events = %+v, want %+v", result.events, expectedEvents)
			}
		case <-testContext.Done():
			t.Fatal(testContext.Err())
		}
	}
	if sourceReadCount.Load() != 1 {
		t.Fatalf("source read count = %d, want 1", sourceReadCount.Load())
	}
	database, errorValue = service.openCalendarDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var cacheEntryCount int
	if errorValue := database.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM calendar_event_window_cache_entries WHERE cache_key = ?", cacheRange.Key).Scan(&cacheEntryCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if cacheEntryCount != 1 {
		t.Fatalf("cache entry count = %d, want 1", cacheEntryCount)
	}
}

func TestCalendarEventWindowCacheBuildsDifferentRangesConcurrently(t *testing.T) {
	service := newCalendarTestService(t)
	database, errorValue := service.openCalendarDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	firstStartTime := time.Date(2026, time.July, 17, 0, 0, 0, 0, time.UTC)
	secondStartTime := firstStartTime.Add(24 * time.Hour)
	sourceReadStarted := make(chan time.Time, 2)
	resumeSourceReads := make(chan struct{})
	sourceReader := func(ctx context.Context, rangeStart time.Time, rangeEnd time.Time) ([]calendarEvent, error) {
		select {
		case sourceReadStarted <- rangeStart:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		select {
		case <-resumeSourceReads:
			return []calendarEvent{{ID: rangeStart.Format(time.DateOnly)}}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	type readResult struct {
		events []calendarEvent
		error  error
	}
	readResults := make(chan readResult, 2)
	for _, startTime := range []time.Time{firstStartTime, secondStartTime} {
		go func() {
			events, errorValue := service.readCalendarEventWindowWithSourceReader(ctx, startTime, startTime.Add(24*time.Hour), sourceReader)
			readResults <- readResult{events: events, error: errorValue}
		}()
	}
	startedRanges := map[time.Time]bool{}
	for range 2 {
		select {
		case startTime := <-sourceReadStarted:
			startedRanges[startTime] = true
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	if !startedRanges[firstStartTime] || !startedRanges[secondStartTime] {
		t.Fatalf("started ranges = %+v", startedRanges)
	}
	close(resumeSourceReads)
	for range 2 {
		select {
		case result := <-readResults:
			if result.error != nil {
				t.Fatal(result.error)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
}
