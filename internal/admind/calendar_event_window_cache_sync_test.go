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
		UID:               "pulled-moved-event@intern.kim",
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

func TestMissingGoogleCalendarEventInvalidatesCachedEventWindow(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	target := activeRemoteCalendarTarget(account)
	event := newLocalTestCalendarEvent("cache-pull-delete", "Deleted remotely")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteHref = target.CalendarURL + event.UID + ".ics"
	event.RemoteETag = `"etag-before-delete"`
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	startTime, errorValue := time.Parse(time.RFC3339, event.StartISO)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.readCalendarEventWindow(ctx, startTime.Add(-time.Hour), startTime.Add(2*time.Hour)); errorValue != nil {
		t.Fatal(errorValue)
	}
	storedEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("event found=%v error=%v", found, errorValue)
	}
	if errorValue := service.softDeleteMissingRemoteEvent(ctx, account.ID, target, storedEvent, pendingCalendarLocalChange{}, time.Now().UTC()); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertCalendarEventWindowCacheIsEmpty(t, service)
}

func TestPushConflictRemoteDeletionInvalidatesCachedEventWindow(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	target := activeRemoteCalendarTarget(account)
	event := newLocalTestCalendarEvent("cache-push-delete", "Deleted during push")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteHref = target.CalendarURL + event.UID + ".ics"
	event.RemoteETag = `"etag-before-delete"`
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	event.Title = "Pending local update"
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil || len(rows) != 1 {
		t.Fatalf("pending rows=%d error=%v", len(rows), errorValue)
	}
	storedEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("event found=%v error=%v", found, errorValue)
	}
	startTime, errorValue := time.Parse(time.RFC3339, storedEvent.StartISO)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.readCalendarEventWindow(ctx, startTime.Add(-time.Hour), startTime.Add(2*time.Hour)); errorValue != nil {
		t.Fatal(errorValue)
	}
	accepted, errorValue := service.acceptCalendarRemoteDeletion(ctx, rows[0], storedEvent)
	if errorValue != nil || !accepted {
		t.Fatalf("accepted=%v error=%v", accepted, errorValue)
	}
	assertCalendarEventWindowCacheIsEmpty(t, service)
}

func assertCalendarEventWindowCacheIsEmpty(t *testing.T, service *Service) {
	t.Helper()
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
		t.Fatalf("calendar event window cache entries = %d, want 0", cacheEntryCount)
	}
}
