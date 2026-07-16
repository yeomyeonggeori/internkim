package admind

import (
	"context"
	"testing"
	"time"
)

func TestCalendarEventWindowCacheInvalidatesOldAndNewRangesOnPullMove(t *testing.T) {
	service := newCalendarTestService(t)
	firstStart := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	firstEnd := firstStart.Add(24 * time.Hour)
	secondStart := firstEnd.Add(24 * time.Hour)
	secondEnd := secondStart.Add(24 * time.Hour)
	event := calendarEvent{
		ID:                "pulled-moved-event",
		UID:               "pulled-moved-event@example.test",
		Title:             "Pulled moved event",
		StartISO:          firstStart.Add(time.Hour).Format(time.RFC3339),
		EndISO:            firstStart.Add(2 * time.Hour).Format(time.RFC3339),
		TimeZone:          "UTC",
		Color:             "#2563eb",
		ReminderLeadHours: calendarDefaultReminderLeadHours,
		CreatedByEmail:    "staff@example.com",
	}
	if errorValue := service.writeCalendarEventWithSource(context.Background(), event, calendarSourcePull); errorValue != nil {
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
	if errorValue := service.writeCalendarEventWithSource(context.Background(), event, calendarSourcePull); errorValue != nil {
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
		t.Fatalf("cache entries after pull move = %d, want 0", cacheEntryCount)
	}
}

func TestGoogleCalendarPullInvalidatesCachedEventWindow(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	client := &fakeCalDAVPullClient{
		objects: []calDAVCalendarObject{
			fakeRemoteObject(t, "cache-pull@google", `"etag-v1"`, "Before pull update"),
		},
	}
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, client); errorValue != nil {
		t.Fatal(errorValue)
	}
	startTime := time.Now().UTC().Add(-time.Hour)
	endTime := startTime.Add(24 * time.Hour)
	if _, errorValue := service.readCalendarEventWindow(ctx, startTime, endTime); errorValue != nil {
		t.Fatal(errorValue)
	}
	client.objects = []calDAVCalendarObject{
		fakeRemoteObject(t, "cache-pull@google", `"etag-v2"`, "After pull update"),
	}
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, client); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var cacheEntryCount int
	if errorValue := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM calendar_event_window_cache_entries").Scan(&cacheEntryCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if cacheEntryCount != 0 {
		t.Fatalf("cache entries after Google pull update = %d, want 0", cacheEntryCount)
	}
}
