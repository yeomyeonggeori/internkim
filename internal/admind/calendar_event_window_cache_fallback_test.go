package admind

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCalendarEventWindowCacheBuilderRetriesOversizedResultAfterRevisionChange(t *testing.T) {
	service := newCalendarTestService(t)
	startTime := time.Date(2026, time.July, 16, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(24 * time.Hour)
	if _, cacheable := calendarEventWindowCacheRangeFor(startTime, endTime); !cacheable {
		t.Fatal("test range is not cacheable")
	}
	oversizedEvent := calendarEvent{
		ID:          "oversized-before-revision-change",
		Description: strings.Repeat("x", calendarEventWindowCacheMaximumPayloadBytes),
	}
	freshEvent := calendarEvent{ID: "fresh-after-revision-change"}
	sourceReadCount := 0
	sourceReader := func(ctx context.Context, rangeStart time.Time, rangeEnd time.Time) ([]calendarEvent, error) {
		sourceReadCount++
		if sourceReadCount != 1 {
			return []calendarEvent{freshEvent}, nil
		}
		database, errorValue := service.openCalendarDatabase(ctx)
		if errorValue != nil {
			return nil, errorValue
		}
		_, updateError := database.ExecContext(ctx, "UPDATE calendar_event_window_source_state SET revision = revision + 1 WHERE id = 1")
		closeError := database.Close()
		if updateError != nil {
			return nil, updateError
		}
		if closeError != nil {
			return nil, closeError
		}
		return []calendarEvent{oversizedEvent}, nil
	}

	events, errorValue := service.readCalendarEventWindowWithSourceReader(t.Context(), startTime, endTime, sourceReader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if sourceReadCount != 2 {
		t.Fatalf("source read count = %d, want 2", sourceReadCount)
	}
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}
	if events[0].ID != freshEvent.ID {
		t.Fatalf("event ID = %q, want %q", events[0].ID, freshEvent.ID)
	}
}

func TestCalendarEventWindowCacheBuilderSharesOversizedUncachedResult(t *testing.T) {
	service := newCalendarTestService(t)
	database, errorValue := service.openCalendarDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	startTime := time.Date(2026, time.July, 16, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(24 * time.Hour)
	cacheRange, cacheable := calendarEventWindowCacheRangeFor(startTime, endTime)
	if !cacheable {
		database.Close()
		t.Fatal("test range is not cacheable")
	}
	expectedEvent := calendarEvent{
		ID:          "oversized-result",
		Description: strings.Repeat("x", calendarEventWindowCacheMaximumPayloadBytes),
	}
	var sourceReadCount atomic.Int32
	sourceReader := func(ctx context.Context, rangeStart time.Time, rangeEnd time.Time) ([]calendarEvent, error) {
		sourceReadCount.Add(1)
		return []calendarEvent{expectedEvent}, nil
	}
	isBuilder, flight := service.calendarWindowBuilds.begin(cacheRange.Key)
	if !isBuilder {
		database.Close()
		t.Fatal("first cache miss did not become builder")
	}
	actualEvents, errorValue := service.readCalendarEventWindowAsBuilder(t.Context(), database, cacheRange, startTime, endTime, sourceReader, flight)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if sourceReadCount.Load() != 1 {
		t.Fatalf("source read count = %d, want 1", sourceReadCount.Load())
	}
	if len(actualEvents) != 1 {
		t.Fatalf("builder event count = %d, want 1", len(actualEvents))
	}
	if actualEvents[0].ID != expectedEvent.ID {
		t.Fatalf("builder event ID = %q, want %q", actualEvents[0].ID, expectedEvent.ID)
	}
	if len(actualEvents[0].Description) != calendarEventWindowCacheMaximumPayloadBytes {
		t.Fatalf("builder description byte length = %d, want %d", len(actualEvents[0].Description), calendarEventWindowCacheMaximumPayloadBytes)
	}
	result, errorValue := flight.wait(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if result.errorValue != nil {
		t.Fatalf("follower error = %v, want nil", result.errorValue)
	}
	if result.shouldRetry {
		t.Fatal("follower result requested retry")
	}
	if len(result.events) != 1 {
		t.Fatalf("follower event count = %d, want 1", len(result.events))
	}
	if result.events[0].ID != expectedEvent.ID {
		t.Fatalf("follower event ID = %q, want %q", result.events[0].ID, expectedEvent.ID)
	}
	if len(result.events[0].Description) != calendarEventWindowCacheMaximumPayloadBytes {
		t.Fatalf("follower description byte length = %d, want %d", len(result.events[0].Description), calendarEventWindowCacheMaximumPayloadBytes)
	}
	cacheDatabase, errorValue := service.openCalendarDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer cacheDatabase.Close()
	var cacheEntryCount int
	if errorValue := cacheDatabase.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM calendar_event_window_cache_entries").Scan(&cacheEntryCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if cacheEntryCount != 0 {
		t.Fatalf("cache entry count = %d, want 0", cacheEntryCount)
	}
}

func TestCalendarEventWindowCacheBuilderSharesResultAfterCacheWriteFailure(t *testing.T) {
	service := newCalendarTestService(t)
	database, errorValue := service.openCalendarDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(t.Context(), `
		CREATE TRIGGER fail_calendar_event_window_cache_flight_insert
		BEFORE INSERT ON calendar_event_window_cache_entries
		BEGIN
			SELECT RAISE(FAIL, 'forced cache insert failure');
		END`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	startTime := time.Date(2026, time.July, 16, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(24 * time.Hour)
	cacheRange, cacheable := calendarEventWindowCacheRangeFor(startTime, endTime)
	if !cacheable {
		database.Close()
		t.Fatal("test range is not cacheable")
	}
	expectedEvent := calendarEvent{ID: "cache-write-failure-result"}
	var sourceReadCount atomic.Int32
	sourceReader := func(ctx context.Context, rangeStart time.Time, rangeEnd time.Time) ([]calendarEvent, error) {
		sourceReadCount.Add(1)
		return []calendarEvent{expectedEvent}, nil
	}
	isBuilder, flight := service.calendarWindowBuilds.begin(cacheRange.Key)
	if !isBuilder {
		database.Close()
		t.Fatal("first cache miss did not become builder")
	}
	actualEvents, errorValue := service.readCalendarEventWindowAsBuilder(t.Context(), database, cacheRange, startTime, endTime, sourceReader, flight)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if sourceReadCount.Load() != 1 {
		t.Fatalf("source read count = %d, want 1", sourceReadCount.Load())
	}
	if len(actualEvents) != 1 {
		t.Fatalf("builder event count = %d, want 1", len(actualEvents))
	}
	if actualEvents[0].ID != expectedEvent.ID {
		t.Fatalf("builder event ID = %q, want %q", actualEvents[0].ID, expectedEvent.ID)
	}
	result, errorValue := flight.wait(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if result.errorValue != nil {
		t.Fatalf("follower error = %v, want nil", result.errorValue)
	}
	if result.shouldRetry {
		t.Fatal("follower result requested retry")
	}
	if len(result.events) != 1 {
		t.Fatalf("follower event count = %d, want 1", len(result.events))
	}
	if result.events[0].ID != expectedEvent.ID {
		t.Fatalf("follower event ID = %q, want %q", result.events[0].ID, expectedEvent.ID)
	}
}
