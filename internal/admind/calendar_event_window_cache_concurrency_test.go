package admind

import (
	"context"
	"testing"
	"time"
)

func TestCalendarEventWindowCacheRetriesMissAfterRevisionChange(t *testing.T) {
	service := newCalendarTestService(t)
	startTime := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(24 * time.Hour)
	readCount := 0
	sourceReader := func(ctx context.Context, rangeStart time.Time, rangeEnd time.Time) ([]calendarEvent, error) {
		readCount++
		if readCount == 1 {
			database, errorValue := service.openCalendarDatabase(ctx)
			if errorValue != nil {
				return nil, errorValue
			}
			_, errorValue = database.ExecContext(ctx, "UPDATE calendar_event_window_source_state SET revision = revision + 1 WHERE id = 1")
			closeError := database.Close()
			if errorValue != nil {
				return nil, errorValue
			}
			if closeError != nil {
				return nil, closeError
			}
			return []calendarEvent{{ID: "stale", Title: "Stale"}}, nil
		}
		return []calendarEvent{{ID: "fresh", Title: "Fresh"}}, nil
	}
	events, errorValue := service.readCalendarEventWindowWithSourceReader(context.Background(), startTime, endTime, sourceReader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if readCount != 2 || len(events) != 1 || events[0].ID != "fresh" {
		t.Fatalf("read count = %d events = %+v", readCount, events)
	}
	cacheRange, _ := calendarEventWindowCacheRangeFor(startTime, endTime)
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	cachedEvents, found, errorValue := readCalendarEventWindowCacheEntry(context.Background(), database, cacheRange, time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found || len(cachedEvents) != 1 || cachedEvents[0].ID != "fresh" {
		t.Fatalf("cached events = %+v found = %v", cachedEvents, found)
	}
}

func TestCalendarEventWindowCacheReturnsUncachedResultAfterRetryExhaustion(t *testing.T) {
	service := newCalendarTestService(t)
	startTime := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(24 * time.Hour)
	readCount := 0
	sourceReader := func(ctx context.Context, rangeStart time.Time, rangeEnd time.Time) ([]calendarEvent, error) {
		readCount++
		database, errorValue := service.openCalendarDatabase(ctx)
		if errorValue != nil {
			return nil, errorValue
		}
		_, errorValue = database.ExecContext(ctx, "UPDATE calendar_event_window_source_state SET revision = revision + 1 WHERE id = 1")
		closeError := database.Close()
		if errorValue != nil {
			return nil, errorValue
		}
		if closeError != nil {
			return nil, closeError
		}
		return []calendarEvent{{ID: "uncached-result"}}, nil
	}
	events, errorValue := service.readCalendarEventWindowWithSourceReader(context.Background(), startTime, endTime, sourceReader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if readCount != calendarEventWindowCacheMaximumBuildAttempts+1 || len(events) != 1 || events[0].ID != "uncached-result" {
		t.Fatalf("read count = %d events = %+v", readCount, events)
	}
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var cacheEntryCount int
	if errorValue := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM calendar_event_window_cache_entries").Scan(&cacheEntryCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if cacheEntryCount != 0 {
		t.Fatalf("cache entries after retry exhaustion = %d, want 0", cacheEntryCount)
	}
}
