package admind

import (
	"context"
	"testing"
	"time"
)

func TestCalendarEventWindowCacheConcurrentRevisionChangeRejectsStalePayload(t *testing.T) {
	service := newCalendarTestService(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	startTime := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(24 * time.Hour)
	event := calendarEvent{
		ID:                "concurrent-revision-event",
		UID:               "concurrent-revision-event@intern.kim",
		Title:             "Stale",
		StartISO:          startTime.Add(time.Hour).Format(time.RFC3339),
		EndISO:            startTime.Add(2 * time.Hour).Format(time.RFC3339),
		TimeZone:          "UTC",
		Color:             "#2563eb",
		ReminderLeadHours: calendarDefaultReminderLeadHours,
		CreatedByEmail:    "staff@example.com",
	}
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	readCount := 0
	firstSourceReadCompleted := make(chan struct{})
	resumeFirstSourceRead := make(chan struct{})
	type readResult struct {
		events []calendarEvent
		error  error
	}
	readResults := make(chan readResult, 1)
	sourceReader := func(ctx context.Context, rangeStart time.Time, rangeEnd time.Time) ([]calendarEvent, error) {
		events, errorValue := service.readCalendarEvents(ctx, rangeStart, rangeEnd)
		if errorValue != nil {
			return nil, errorValue
		}
		readCount++
		if readCount == 1 {
			close(firstSourceReadCompleted)
			select {
			case <-resumeFirstSourceRead:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return events, nil
	}
	go func() {
		events, errorValue := service.readCalendarEventWindowWithSourceReader(ctx, startTime, endTime, sourceReader)
		readResults <- readResult{events: events, error: errorValue}
	}()
	select {
	case <-firstSourceReadCompleted:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	event.Title = "Fresh"
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		close(resumeFirstSourceRead)
		t.Fatal(errorValue)
	}
	close(resumeFirstSourceRead)
	var events []calendarEvent
	select {
	case readResult := <-readResults:
		if readResult.error != nil {
			t.Fatal(readResult.error)
		}
		events = readResult.events
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if readCount != 2 || len(events) != 1 || events[0].ID != event.ID || events[0].Title != event.Title {
		t.Fatalf("read count = %d events = %+v", readCount, events)
	}
	cacheRange, _ := calendarEventWindowCacheRangeFor(startTime, endTime)
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	cachedEvents, sourceRevision, found, errorValue := readCalendarEventWindowCacheEntry(ctx, database, cacheRange, time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found || len(cachedEvents) != 1 || cachedEvents[0].ID != event.ID || cachedEvents[0].Title != event.Title {
		t.Fatalf("cached events = %+v found = %v", cachedEvents, found)
	}
	if sourceRevision != 2 {
		t.Fatalf("cached source revision = %d, want 2", sourceRevision)
	}
}

func TestCalendarEventWindowCacheBuildResultRejectsChangedRevision(t *testing.T) {
	service := newCalendarTestService(t)
	database, errorValue := service.openCalendarDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	revision, errorValue := readCalendarEventWindowSourceRevision(t.Context(), database)
	if errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(t.Context(), "UPDATE calendar_event_window_source_state SET revision = revision + 1 WHERE id = 1"); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	result := calendarEventWindowCacheBuildResult{
		events:            []calendarEvent{{ID: "stale-result"}},
		sourceRevision:    revision,
		hasSourceRevision: true,
	}
	isCurrent, errorValue := isCalendarEventWindowCacheBuildResultCurrent(t.Context(), database, result)
	if errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	if isCurrent {
		t.Fatal("changed source revision accepted stale flight result")
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
