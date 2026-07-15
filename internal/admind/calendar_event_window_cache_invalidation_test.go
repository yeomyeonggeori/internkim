package admind

import (
	"context"
	"testing"
	"time"
)

func TestCalendarEventWindowCacheInvalidatesOverlappingCreate(t *testing.T) {
	service := newCalendarTestService(t)
	firstStart := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	firstEnd := firstStart.Add(24 * time.Hour)
	secondStart := firstEnd.Add(24 * time.Hour)
	secondEnd := secondStart.Add(24 * time.Hour)
	if _, errorValue := service.readCalendarEventWindow(context.Background(), firstStart, firstEnd); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.readCalendarEventWindow(context.Background(), secondStart, secondEnd); errorValue != nil {
		t.Fatal(errorValue)
	}
	event := calendarEvent{
		ID:                "created-event",
		UID:               "created-event@example.test",
		Title:             "Created event",
		StartISO:          firstStart.Add(time.Hour).Format(time.RFC3339),
		EndISO:            firstStart.Add(2 * time.Hour).Format(time.RFC3339),
		TimeZone:          "UTC",
		Color:             "#2563eb",
		ReminderLeadHours: calendarDefaultReminderLeadHours,
		CreatedByEmail:    "staff@example.com",
	}
	if errorValue := service.writeCalendarEventWithSource(context.Background(), event, calendarSourceLocal); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	firstRange, _ := calendarEventWindowCacheRangeFor(firstStart, firstEnd)
	secondRange, _ := calendarEventWindowCacheRangeFor(secondStart, secondEnd)
	var firstCount int
	var secondCount int
	var revision int64
	if errorValue := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM calendar_event_window_cache_entries WHERE cache_key = ?", firstRange.Key).Scan(&firstCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM calendar_event_window_cache_entries WHERE cache_key = ?", secondRange.Key).Scan(&secondCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.QueryRowContext(context.Background(), "SELECT revision FROM calendar_event_window_source_state WHERE id = 1").Scan(&revision); errorValue != nil {
		t.Fatal(errorValue)
	}
	if firstCount != 0 || secondCount != 1 || revision != 1 {
		t.Fatalf("cache counts = %d, %d revision = %d", firstCount, secondCount, revision)
	}
}

func TestCalendarEventWindowCacheInvalidatesOldAndNewRangesOnMove(t *testing.T) {
	service := newCalendarTestService(t)
	firstStart := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	firstEnd := firstStart.Add(24 * time.Hour)
	secondStart := firstEnd.Add(24 * time.Hour)
	secondEnd := secondStart.Add(24 * time.Hour)
	event := calendarEvent{
		ID:                "moved-event",
		UID:               "moved-event@example.test",
		Title:             "Moved event",
		StartISO:          firstStart.Add(time.Hour).Format(time.RFC3339),
		EndISO:            firstStart.Add(2 * time.Hour).Format(time.RFC3339),
		TimeZone:          "UTC",
		Color:             "#2563eb",
		ReminderLeadHours: calendarDefaultReminderLeadHours,
		CreatedByEmail:    "staff@example.com",
	}
	if errorValue := service.writeCalendarEventWithSource(context.Background(), event, calendarSourceLocal); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.readCalendarEventWindow(context.Background(), firstStart, firstEnd); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.readCalendarEventWindow(context.Background(), secondStart, secondEnd); errorValue != nil {
		t.Fatal(errorValue)
	}
	event.StartISO = secondStart.Add(time.Hour).Format(time.RFC3339)
	event.EndISO = secondStart.Add(2 * time.Hour).Format(time.RFC3339)
	if errorValue := service.writeCalendarEventWithSource(context.Background(), event, calendarSourceLocal); errorValue != nil {
		t.Fatal(errorValue)
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
		t.Fatalf("cache entries after move = %d, want 0", cacheEntryCount)
	}
}

func TestCalendarEventWindowCacheInvalidatesDeletedEventRange(t *testing.T) {
	service := newCalendarTestService(t)
	startTime := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(24 * time.Hour)
	event := calendarEvent{
		ID:                "deleted-event",
		UID:               "deleted-event@example.test",
		Title:             "Deleted event",
		StartISO:          startTime.Add(time.Hour).Format(time.RFC3339),
		EndISO:            startTime.Add(2 * time.Hour).Format(time.RFC3339),
		TimeZone:          "UTC",
		Color:             "#2563eb",
		ReminderLeadHours: calendarDefaultReminderLeadHours,
		CreatedByEmail:    "staff@example.com",
	}
	if errorValue := service.writeCalendarEventWithSource(context.Background(), event, calendarSourceLocal); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.readCalendarEventWindow(context.Background(), startTime, endTime); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.softDeleteCalendarEventWithSource(context.Background(), event.ID, calendarSourceLocal); errorValue != nil {
		t.Fatal(errorValue)
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
		t.Fatalf("cache entries after delete = %d, want 0", cacheEntryCount)
	}
}
