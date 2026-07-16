package admind

import (
	"context"
	"testing"
	"time"
)

func TestCalendarEventWindowCacheKeepsAdjacentRange(t *testing.T) {
	service := newCalendarTestService(t)
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	startTime := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(time.Hour)
	cacheRange, _ := calendarEventWindowCacheRangeFor(startTime, endTime)
	revision, errorValue := readCalendarEventWindowSourceRevision(context.Background(), database)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if writeResult, errorValue := writeCalendarEventWindowCacheEntryIfCurrent(context.Background(), database, cacheRange, revision, nil, time.Now()); errorValue != nil {
		t.Fatal(errorValue)
	} else if writeResult != calendarEventWindowCacheWriteStored {
		t.Fatal("adjacent cache range was not stored")
	}
	transaction, errorValue := database.BeginTx(context.Background(), nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event := calendarEvent{ID: "adjacent-event", StartISO: endTime.Format(time.RFC3339Nano), EndISO: endTime.Add(time.Hour).Format(time.RFC3339Nano)}
	if errorValue := invalidateCalendarEventWindowCache(context.Background(), transaction, event); errorValue != nil {
		_ = transaction.Rollback()
		t.Fatal(errorValue)
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		t.Fatal(errorValue)
	}
	var cacheEntryCount int
	if errorValue := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM calendar_event_window_cache_entries WHERE cache_key = ?", cacheRange.Key).Scan(&cacheEntryCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if cacheEntryCount != 1 {
		t.Fatalf("adjacent cache entries = %d, want 1", cacheEntryCount)
	}
}

func TestCalendarEventWindowCacheRebuildsPayloadVersionMismatch(t *testing.T) {
	service := newCalendarTestService(t)
	startTime := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(24 * time.Hour)
	event := calendarEventWindowCacheEdgeEvent(startTime)
	if errorValue := service.writeCalendarEventWithSource(context.Background(), event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.readCalendarEventWindow(context.Background(), startTime, endTime); errorValue != nil {
		t.Fatal(errorValue)
	}
	cacheRange, _ := calendarEventWindowCacheRangeFor(startTime, endTime)
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(context.Background(), "UPDATE calendar_event_window_cache_entries SET payload_json = ? WHERE cache_key = ?", `{"version":1,"events":[]}`, cacheRange.Key); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	events, errorValue := service.readCalendarEventWindow(context.Background(), startTime, endTime)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != 1 || events[0].ID != event.ID {
		t.Fatalf("events after payload version mismatch = %+v", events)
	}
}

func TestCalendarEventWindowCacheRollsBackMutationWhenInvalidationFails(t *testing.T) {
	service := newCalendarTestService(t)
	startTime := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(24 * time.Hour)
	event := calendarEventWindowCacheEdgeEvent(startTime)
	if errorValue := service.writeCalendarEventWithSource(context.Background(), event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.readCalendarEventWindow(context.Background(), startTime, endTime); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(context.Background(), `
		CREATE TRIGGER fail_calendar_event_window_cache_invalidation
		BEFORE DELETE ON calendar_event_window_cache_entries
		BEGIN
			SELECT RAISE(FAIL, 'cache invalidation failed');
		END`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	event.Title = "Changed title"
	if errorValue := service.writeCalendarEventWithSource(context.Background(), event, calendarSourcePull); errorValue == nil {
		t.Fatal("event mutation succeeded")
	}
	database, errorValue = service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var storedTitle string
	if errorValue := database.QueryRowContext(context.Background(), "SELECT title FROM calendar_events WHERE id = ?", event.ID).Scan(&storedTitle); errorValue != nil {
		t.Fatal(errorValue)
	}
	if storedTitle != "Edge event" {
		t.Fatalf("stored title after rollback = %q", storedTitle)
	}
	var cacheEntryCount int
	if errorValue := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM calendar_event_window_cache_entries").Scan(&cacheEntryCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if cacheEntryCount != 1 {
		t.Fatalf("cache entries after rollback = %d, want 1", cacheEntryCount)
	}
	var revision int64
	if errorValue := database.QueryRowContext(context.Background(), "SELECT revision FROM calendar_event_window_source_state WHERE id = 1").Scan(&revision); errorValue != nil {
		t.Fatal(errorValue)
	}
	if revision != 1 {
		t.Fatalf("revision after rollback = %d, want 1", revision)
	}
}

func TestCalendarEventWindowCacheInitializationFailureRetriesAfterSourceFallback(t *testing.T) {
	service := newCalendarTestService(t)
	startTime := time.Date(2026, time.July, 16, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(24 * time.Hour)
	event := calendarEventWindowCacheEdgeEvent(startTime)
	if errorValue := service.writeCalendarEventWithSource(context.Background(), event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.readCalendarEventWindow(context.Background(), startTime, endTime); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(context.Background(), `
		CREATE TRIGGER fail_calendar_event_window_cache_reset
		BEFORE DELETE ON calendar_event_window_cache_entries
		BEGIN
			SELECT RAISE(FAIL, 'forced cache reset failure');
		END`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	restartedService := NewService(service.Configuration)
	event.Title = "Updated from source"
	if errorValue := restartedService.writeCalendarEventWithSource(context.Background(), event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	events, errorValue := restartedService.readCalendarEventWindow(context.Background(), startTime, endTime)
	if errorValue != nil || len(events) != 1 || events[0].Title != event.Title {
		t.Fatalf("source fallback events=%+v error=%v", events, errorValue)
	}
	database, errorValue = restartedService.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(context.Background(), "DROP TRIGGER fail_calendar_event_window_cache_reset"); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue = restartedService.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !restartedService.isCalendarEventWindowCacheEnabled() {
		t.Fatal("calendar event window cache did not recover")
	}
	events, errorValue = restartedService.readCalendarEventWindow(context.Background(), startTime, endTime)
	if errorValue != nil || len(events) != 1 || events[0].Title != event.Title {
		t.Fatalf("recovered cache events=%+v error=%v", events, errorValue)
	}
}

func calendarEventWindowCacheEdgeEvent(startTime time.Time) calendarEvent {
	return calendarEvent{
		ID:                "cache-edge-event",
		UID:               "cache-edge-event@example.test",
		Title:             "Edge event",
		StartISO:          startTime.Add(time.Hour).Format(time.RFC3339),
		EndISO:            startTime.Add(2 * time.Hour).Format(time.RFC3339),
		TimeZone:          "UTC",
		Color:             "#2563eb",
		ReminderLeadHours: calendarDefaultReminderLeadHours,
		CreatedByEmail:    "staff@example.com",
	}
}
