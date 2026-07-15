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
		UID:               "created-event@intern.kim",
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
		UID:               "moved-event@intern.kim",
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
		UID:               "deleted-event@intern.kim",
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

func TestCalendarEventWindowCacheInvalidatesFractionalOverlap(t *testing.T) {
	service := newCalendarTestService(t)
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	startTime := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	cacheRange, cacheable := calendarEventWindowCacheRangeFor(startTime, startTime.Add(500*time.Millisecond))
	if !cacheable {
		t.Fatal("fractional cache range is not cacheable")
	}
	revision, errorValue := readCalendarEventWindowSourceRevision(context.Background(), database)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	writeResult, errorValue := writeCalendarEventWindowCacheEntryIfCurrent(context.Background(), database, cacheRange, revision, nil, time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if writeResult != calendarEventWindowCacheWriteStored {
		t.Fatal("fractional cache range was not stored")
	}
	transaction, errorValue := database.BeginTx(context.Background(), nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event := calendarEvent{
		ID:       "fractional-overlap",
		StartISO: startTime.Format(time.RFC3339Nano),
		EndISO:   startTime.Add(time.Second).Format(time.RFC3339Nano),
	}
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
	if cacheEntryCount != 0 {
		t.Fatalf("fractional overlap cache entries = %d, want 0", cacheEntryCount)
	}
}

func TestCalendarEventWindowCacheInvalidatesMattermostProjectionChanges(t *testing.T) {
	service := newCalendarTestService(t)
	startTime := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	endTime := startTime.Add(24 * time.Hour)
	event := calendarEvent{
		ID:                "mattermost-projection-event",
		UID:               "mattermost-projection-event@intern.kim",
		Title:             "Mattermost projection event",
		StartISO:          startTime.Add(time.Hour).Format(time.RFC3339),
		EndISO:            startTime.Add(2 * time.Hour).Format(time.RFC3339),
		TimeZone:          "UTC",
		Color:             "#2563eb",
		ReminderLeadHours: calendarDefaultReminderLeadHours,
		CreatedByEmail:    "staff@example.com",
	}
	if errorValue := service.writeCalendarEventWithSource(context.Background(), event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.readCalendarEventWindow(context.Background(), startTime, endTime); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.updateCalendarEventMattermostPostID(context.Background(), event.ID, "post-1"); errorValue != nil {
		t.Fatal(errorValue)
	}
	events, errorValue := service.readCalendarEventWindow(context.Background(), startTime, endTime)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != 1 || events[0].MattermostPostID != "post-1" {
		t.Fatalf("events after Mattermost projection update = %+v", events)
	}
	if errorValue := service.updateCalendarEventMattermostPostID(context.Background(), event.ID, ""); errorValue != nil {
		t.Fatal(errorValue)
	}
	events, errorValue = service.readCalendarEventWindow(context.Background(), startTime, endTime)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != 1 || events[0].MattermostPostID != "" {
		t.Fatalf("events after Mattermost projection clear = %+v", events)
	}
}
