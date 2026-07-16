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

	pullStartedAt := time.Now().UTC()
	if errorValue := service.upsertCalendarRemoteEventState(ctx, calendarRemoteEventState{
		AccountID:   account.ID,
		CalendarURL: activeRemoteCalendarTarget(account).CalendarURL,
		EventUID:    baselineEvent.UID,
		LastSeenAt:  pullStartedAt.Add(-time.Minute).Format(time.RFC3339Nano),
	}); errorValue != nil {
		t.Fatalf("seed remote state: %v", errorValue)
	}
	localUpdated := baselineEvent
	localUpdated.Title = "Late Local"
	if errorValue := service.writeCalendarEvent(ctx, localUpdated); errorValue != nil {
		t.Fatalf("late local update: %v", errorValue)
	}

	_, isCurrentTarget, errorValue := service.applyCalendarPullCycleIfCurrent(ctx, account, activeRemoteCalendarTarget(account), nil, "", pullStartedAt)
	if errorValue != nil || !isCurrentTarget {
		t.Fatalf("apply pull cycle current=%v error=%v", isCurrentTarget, errorValue)
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

func TestPullConflictWallClockRollbackPreservesLocalWriteAfterSnapshotCompletion(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	account := seedAccountWithDiscovery(t, service)
	baselineEvent := newLocalTestCalendarEvent("pull-wall-clock-rollback", "Original Title")
	baselineEvent.RemoteSource = remoteCalendarProviderGoogle
	baselineEvent.RemoteETag = `"etag-base"`
	baselineEvent.RemoteHref = "/calendars/me/pull-wall-clock-rollback.ics"
	if errorValue := service.writeCalendarEventWithSource(contextValue, baselineEvent, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	snapshotCompletedAt := time.Date(2036, 7, 16, 12, 0, 0, 0, time.UTC)
	reservedSnapshotBoundary, errorValue := service.reserveCalendarConflictCandidateTime(contextValue, snapshotCompletedAt)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if reservedSnapshotBoundary != snapshotCompletedAt {
		t.Fatalf("snapshot boundary=%s want=%s", reservedSnapshotBoundary, snapshotCompletedAt)
	}
	if errorValue := service.upsertCalendarRemoteEventState(contextValue, calendarRemoteEventState{
		AccountID:   account.ID,
		CalendarURL: activeRemoteCalendarTarget(account).CalendarURL,
		EventUID:    baselineEvent.UID,
		LastSeenAt:  snapshotCompletedAt.Add(-time.Minute).Format(time.RFC3339Nano),
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	localUpdated := baselineEvent
	localUpdated.Title = "Local After Snapshot"
	if errorValue := service.writeCalendarEvent(contextValue, localUpdated); errorValue != nil {
		t.Fatal(errorValue)
	}
	pendingRows, errorValue := service.listPendingCalendarOutbox(contextValue, account.ID)
	if errorValue != nil || len(pendingRows) != 1 {
		t.Fatalf("pending rows=%d error=%v", len(pendingRows), errorValue)
	}
	localChangedAt := parseCalendarConflictTime(pendingRows[0].CreatedAt)
	if !localChangedAt.After(reservedSnapshotBoundary) {
		t.Fatalf("local changed at=%s snapshot boundary=%s", localChangedAt, reservedSnapshotBoundary)
	}
	_, isCurrentTarget, errorValue := service.applyCalendarPullCycleIfCurrent(contextValue, account, activeRemoteCalendarTarget(account), nil, "", reservedSnapshotBoundary)
	if errorValue != nil || !isCurrentTarget {
		t.Fatalf("apply pull cycle current=%v error=%v", isCurrentTarget, errorValue)
	}
	finalEvent, found, errorValue := service.readCalendarEventByID(contextValue, baselineEvent.ID)
	if errorValue != nil || !found {
		t.Fatalf("final event found=%v error=%v", found, errorValue)
	}
	if finalEvent.Title != "Local After Snapshot" {
		t.Fatalf("title=%q want Local After Snapshot", finalEvent.Title)
	}
	remainingRows, errorValue := service.listPendingCalendarOutbox(contextValue, account.ID)
	if errorValue != nil || len(remainingRows) != 1 {
		t.Fatalf("remaining rows=%d error=%v", len(remainingRows), errorValue)
	}
}

func TestPullPresentSnapshotKeepsLastSeenBeforeLaterLocalWrite(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	account := seedAccountWithDiscovery(t, service)
	baselineEvent := newLocalTestCalendarEvent("pull-present-before-local", "Original Title")
	baselineEvent.RemoteSource = remoteCalendarProviderGoogle
	baselineEvent.RemoteETag = `"etag-base"`
	baselineEvent.RemoteHref = "/calendars/me/pull-present-before-local.ics"
	baselineICS, errorValue := encodeEventToICS(baselineEvent)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	baselineEvent.RawICS = string(baselineICS)
	if errorValue := service.writeCalendarEventWithSource(contextValue, baselineEvent, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	snapshotCompletedAt := time.Date(2036, 7, 16, 12, 0, 0, 0, time.UTC)
	reservedSnapshotBoundary, errorValue := service.reserveCalendarConflictCandidateTime(contextValue, snapshotCompletedAt)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	localUpdated := baselineEvent
	localUpdated.Title = "Local After Present Snapshot"
	if errorValue := service.writeCalendarEvent(contextValue, localUpdated); errorValue != nil {
		t.Fatal(errorValue)
	}
	pendingRows, errorValue := service.listPendingCalendarOutbox(contextValue, account.ID)
	if errorValue != nil || len(pendingRows) != 1 {
		t.Fatalf("pending rows=%d error=%v", len(pendingRows), errorValue)
	}
	localChangedAt := parseCalendarConflictTime(pendingRows[0].CreatedAt)
	if !localChangedAt.After(reservedSnapshotBoundary) {
		t.Fatalf("local changed at=%s snapshot boundary=%s", localChangedAt, reservedSnapshotBoundary)
	}
	_, isCurrentTarget, errorValue := service.applyCalendarPullCycleIfCurrent(contextValue, account, activeRemoteCalendarTarget(account), []calDAVCalendarObject{{
		Path: baselineEvent.RemoteHref,
		ETag: baselineEvent.RemoteETag,
		Data: baselineICS,
	}}, "", reservedSnapshotBoundary)
	if errorValue != nil || !isCurrentTarget {
		t.Fatalf("apply pull cycle current=%v error=%v", isCurrentTarget, errorValue)
	}
	remoteState, found, errorValue := service.readCalendarRemoteEventState(contextValue, account.ID, activeRemoteCalendarTarget(account).CalendarURL, baselineEvent.UID)
	if errorValue != nil || !found {
		t.Fatalf("remote state found=%v error=%v", found, errorValue)
	}
	lastSeenAt := parseCalendarConflictTime(remoteState.LastSeenAt)
	if lastSeenAt != reservedSnapshotBoundary {
		t.Fatalf("last seen at=%s snapshot boundary=%s", lastSeenAt, reservedSnapshotBoundary)
	}
	if !lastSeenAt.Before(localChangedAt) {
		t.Fatalf("last seen at=%s local changed at=%s", lastSeenAt, localChangedAt)
	}
}

func TestPullConflictMissingSnapshotBoundaryPrefersDeletionAtOrBeforeCompletion(t *testing.T) {
	testCases := []struct {
		name                string
		snapshotCompletedAt func(time.Time) time.Time
	}{
		{name: "at completion", snapshotCompletedAt: func(localChangedAt time.Time) time.Time { return localChangedAt }},
		{name: "during query", snapshotCompletedAt: func(localChangedAt time.Time) time.Time { return localChangedAt.Add(time.Nanosecond) }},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := newCalendarTestService(t)
			contextValue := context.Background()
			account := seedAccountWithDiscovery(t, service)
			baselineEvent := newLocalTestCalendarEvent("pull-missing-boundary-"+testCase.name, "Original Title")
			baselineEvent.RemoteSource = remoteCalendarProviderGoogle
			baselineEvent.RemoteETag = `"etag-base"`
			baselineEvent.RemoteHref = "/calendars/me/" + baselineEvent.ID + ".ics"
			if errorValue := service.writeCalendarEventWithSource(contextValue, baselineEvent, calendarSourcePull); errorValue != nil {
				t.Fatal(errorValue)
			}
			localUpdated := baselineEvent
			localUpdated.Title = "Local During Query"
			if errorValue := service.writeCalendarEvent(contextValue, localUpdated); errorValue != nil {
				t.Fatal(errorValue)
			}
			rows, errorValue := service.listPendingCalendarOutbox(contextValue, account.ID)
			if errorValue != nil || len(rows) != 1 {
				t.Fatalf("outbox rows=%d error=%v", len(rows), errorValue)
			}
			localChangedAt := parseCalendarConflictTime(rows[0].CreatedAt)
			if errorValue := service.upsertCalendarRemoteEventState(contextValue, calendarRemoteEventState{
				AccountID:   account.ID,
				CalendarURL: activeRemoteCalendarTarget(account).CalendarURL,
				EventUID:    baselineEvent.UID,
				LastSeenAt:  localChangedAt.Add(-time.Minute).Format(time.RFC3339Nano),
			}); errorValue != nil {
				t.Fatal(errorValue)
			}
			snapshotCompletedAt := testCase.snapshotCompletedAt(localChangedAt)
			_, isCurrentTarget, errorValue := service.applyCalendarPullCycleIfCurrent(contextValue, account, activeRemoteCalendarTarget(account), nil, "", snapshotCompletedAt)
			if errorValue != nil || !isCurrentTarget {
				t.Fatalf("apply pull cycle current=%v error=%v", isCurrentTarget, errorValue)
			}
			if _, found, errorValue := service.readCalendarEventByID(contextValue, baselineEvent.ID); errorValue != nil || found {
				t.Fatalf("event found=%v error=%v", found, errorValue)
			}
		})
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
