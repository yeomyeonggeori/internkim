package admind

import (
	"context"
	"testing"
	"time"
)

func TestPullConflictStaleSnapshotPreservesLatePendingLocalEdit(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	baselineEvent := newLocalTestCalendarEvent("pull-stale", "Original Title")
	baselineEvent.RemoteSource = remoteCalendarProviderGoogle
	baselineEvent.RemoteETag = `"etag-base"`
	baselineEvent.RemoteHref = "/calendars/me/pull-stale.ics"
	baselineICS, errorValue := encodeEventToICS(baselineEvent)
	if errorValue != nil {
		t.Fatalf("encode baseline: %v", errorValue)
	}
	baselineEvent.RawICS = string(baselineICS)
	if errorValue := service.writeCalendarEventWithSource(ctx, baselineEvent, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed baseline: %v", errorValue)
	}

	activeEvents, errorValue := service.readCalendarEvents(ctx, time.Time{}, time.Time{})
	if errorValue != nil {
		t.Fatalf("read stale snapshot: %v", errorValue)
	}
	existingByUID := map[string]calendarEvent{}
	for _, event := range activeEvents {
		existingByUID[event.UID] = event
	}

	localUpdated := baselineEvent
	localUpdated.Title = "Late Local"
	if errorValue := service.writeCalendarEvent(ctx, localUpdated); errorValue != nil {
		t.Fatalf("late local update: %v", errorValue)
	}

	remoteUpdated := baselineEvent
	remoteUpdated.Location = "Remote Room"
	remoteICS, errorValue := encodeEventToICS(remoteUpdated)
	if errorValue != nil {
		t.Fatalf("encode remote update: %v", errorValue)
	}
	if _, errorValue := service.applyPulledRemoteEvents(ctx, account, []calDAVCalendarObject{{
		Path: baselineEvent.RemoteHref,
		ETag: `"etag-remote"`,
		Data: remoteICS,
	}}, existingByUID); errorValue != nil {
		t.Fatalf("apply stale pull: %v", errorValue)
	}

	final, _, errorValue := service.readCalendarEventByID(ctx, baselineEvent.ID)
	if errorValue != nil {
		t.Fatalf("read final: %v", errorValue)
	}
	if final.Title != "Late Local" {
		t.Errorf("title: got %q, want Late Local", final.Title)
	}
	if final.Location != "Remote Room" {
		t.Errorf("location: got %q, want Remote Room", final.Location)
	}
}

func TestPullConflictStaleMissingRemotePreservesLatePendingLocalEdit(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	baselineEvent := newLocalTestCalendarEvent("pull-stale-missing", "Original Title")
	baselineEvent.RemoteSource = remoteCalendarProviderGoogle
	baselineEvent.RemoteETag = `"etag-base"`
	baselineEvent.RemoteHref = "/calendars/me/pull-stale-missing.ics"
	baselineICS, errorValue := encodeEventToICS(baselineEvent)
	if errorValue != nil {
		t.Fatalf("encode baseline: %v", errorValue)
	}
	baselineEvent.RawICS = string(baselineICS)
	if errorValue := service.writeCalendarEventWithSource(ctx, baselineEvent, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed baseline: %v", errorValue)
	}

	activeEvents, errorValue := service.readCalendarEvents(ctx, time.Time{}, time.Time{})
	if errorValue != nil {
		t.Fatalf("read stale snapshot: %v", errorValue)
	}
	pullStartedAt := time.Now().UTC()
	if errorValue := service.upsertCalendarRemoteEventState(ctx, calendarRemoteEventState{
		AccountID:   account.ID,
		CalendarURL: activeRemoteCalendarTarget(account).CalendarURL,
		EventUID:    baselineEvent.UID,
		LastSeenAt:  pullStartedAt.Add(-time.Minute).Format(time.RFC3339Nano),
	}); errorValue != nil {
		t.Fatalf("seed remote state: %v", errorValue)
	}
	pendingLocalChanges, errorValue := service.listPendingCalendarLocalChanges(ctx, account.ID, activeRemoteCalendarTarget(account).CalendarURL)
	if errorValue != nil {
		t.Fatalf("read stale pending changes: %v", errorValue)
	}

	localUpdated := baselineEvent
	localUpdated.Title = "Late Local"
	if errorValue := service.writeCalendarEvent(ctx, localUpdated); errorValue != nil {
		t.Fatalf("late local update: %v", errorValue)
	}

	if errorValue := service.softDeleteMissingRemoteEventsWithConflictState(ctx, account.ID, activeRemoteCalendarTarget(account), activeEvents, map[string]struct{}{}, nil, pendingLocalChanges, pullStartedAt); errorValue != nil {
		t.Fatalf("soft delete missing remote: %v", errorValue)
	}

	final, found, errorValue := service.readCalendarEventByID(ctx, baselineEvent.ID)
	if errorValue != nil {
		t.Fatalf("read final: %v", errorValue)
	}
	if !found {
		t.Fatal("late pending local event should not be soft-deleted")
	}
	if final.Title != "Late Local" {
		t.Errorf("title: got %q, want Late Local", final.Title)
	}
}

func TestPullConflictStaleSnapshotKeepsLatePendingLocalDelete(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	baselineEvent := newLocalTestCalendarEvent("pull-stale-delete", "Original Title")
	baselineEvent.RemoteSource = remoteCalendarProviderGoogle
	baselineEvent.RemoteETag = `"etag-base"`
	baselineEvent.RemoteHref = "/calendars/me/pull-stale-delete.ics"
	baselineICS, errorValue := encodeEventToICS(baselineEvent)
	if errorValue != nil {
		t.Fatalf("encode baseline: %v", errorValue)
	}
	baselineEvent.RawICS = string(baselineICS)
	if errorValue := service.writeCalendarEventWithSource(ctx, baselineEvent, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed baseline: %v", errorValue)
	}

	activeEvents, errorValue := service.readCalendarEvents(ctx, time.Time{}, time.Time{})
	if errorValue != nil {
		t.Fatalf("read stale snapshot: %v", errorValue)
	}
	existingByUID := map[string]calendarEvent{}
	for _, event := range activeEvents {
		existingByUID[event.UID] = event
	}

	if errorValue := service.softDeleteCalendarEvent(ctx, baselineEvent.ID); errorValue != nil {
		t.Fatalf("late local delete: %v", errorValue)
	}

	remoteUpdated := baselineEvent
	remoteUpdated.Location = "Remote Room"
	remoteICS, errorValue := encodeEventToICS(remoteUpdated)
	if errorValue != nil {
		t.Fatalf("encode remote update: %v", errorValue)
	}
	if _, errorValue := service.applyPulledRemoteEvents(ctx, account, []calDAVCalendarObject{{
		Path: baselineEvent.RemoteHref,
		ETag: `"etag-remote"`,
		Data: remoteICS,
	}}, existingByUID); errorValue != nil {
		t.Fatalf("apply stale pull: %v", errorValue)
	}

	_, found, errorValue := service.readCalendarEventByID(ctx, baselineEvent.ID)
	if errorValue != nil {
		t.Fatalf("read final: %v", errorValue)
	}
	if found {
		t.Fatal("late pending local delete should not be restored by stale pull")
	}
}
