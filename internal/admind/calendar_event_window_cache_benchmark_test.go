package admind

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func BenchmarkCalendarEventWindowCachedRead(benchmark *testing.B) {
	service := newCalendarEventWindowBenchmarkService(benchmark)
	startTime, endTime := calendarEventWindowBenchmarkRange()
	if _, errorValue := service.readCalendarEventWindow(context.Background(), startTime, endTime); errorValue != nil {
		benchmark.Fatal(errorValue)
	}
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for iteration := 0; iteration < benchmark.N; iteration++ {
		events, errorValue := service.readCalendarEventWindow(context.Background(), startTime, endTime)
		if errorValue != nil {
			benchmark.Fatal(errorValue)
		}
		if len(events) != calendarEventWindowBenchmarkEventCount {
			benchmark.Fatalf("calendar event count = %d", len(events))
		}
	}
}

func BenchmarkCalendarEventWindowConcurrentColdMisses(benchmark *testing.B) {
	service := newCalendarEventWindowBenchmarkService(benchmark)
	startTime, endTime := calendarEventWindowBenchmarkRange()
	const readerCount = 8
	type readResult struct {
		events []calendarEvent
		error  error
	}
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for iteration := 0; iteration < benchmark.N; iteration++ {
		benchmark.StopTimer()
		resetCalendarEventWindowBenchmarkCache(benchmark, service)
		cacheRange, cacheable := calendarEventWindowCacheRangeFor(startTime, endTime)
		if !cacheable {
			benchmark.Fatal("benchmark range is not cacheable")
		}
		var sourceReadCount atomic.Int32
		sourceReadStarted := make(chan struct{}, readerCount)
		resumeSourceRead := make(chan struct{})
		readResults := make(chan readResult, readerCount)
		sourceReader := func(ctx context.Context, rangeStart time.Time, rangeEnd time.Time) ([]calendarEvent, error) {
			sourceReadCount.Add(1)
			select {
			case sourceReadStarted <- struct{}{}:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			select {
			case <-resumeSourceRead:
				return service.readCalendarEvents(ctx, rangeStart, rangeEnd)
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		readContext, cancelRead := context.WithTimeout(context.Background(), 5*time.Second)
		benchmark.StartTimer()
		for range readerCount {
			go func() {
				events, errorValue := service.readCalendarEventWindowWithSourceReader(readContext, startTime, endTime, sourceReader)
				readResults <- readResult{events: events, error: errorValue}
			}()
		}
		select {
		case <-sourceReadStarted:
		case <-readContext.Done():
			cancelRead()
			benchmark.Fatal(readContext.Err())
		}
		waitForCalendarEventWindowCacheFollowers(benchmark, readContext, service, cacheRange.Key, readerCount-1)
		close(resumeSourceRead)
		for range readerCount {
			select {
			case result := <-readResults:
				if result.error != nil {
					cancelRead()
					benchmark.Fatal(result.error)
				}
				if len(result.events) != calendarEventWindowBenchmarkEventCount {
					cancelRead()
					benchmark.Fatalf("calendar event count = %d", len(result.events))
				}
			case <-readContext.Done():
				cancelRead()
				benchmark.Fatal(readContext.Err())
			}
		}
		cancelRead()
		benchmark.StopTimer()
		if sourceReadCount.Load() != 1 {
			benchmark.Fatalf("source read count = %d", sourceReadCount.Load())
		}
	}
}
