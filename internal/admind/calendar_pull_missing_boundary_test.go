package admind

import (
	"context"
	"testing"
	"time"
)

func TestPullConflictRepeatedMissingKeepsFirstBoundaryAndLaterLocalWrite(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	account := seedAccountWithDiscovery(t, service)
	target := activeRemoteCalendarTarget(account)
	event := newLocalTestCalendarEvent("pull-repeated-missing-boundary", "Original Title")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-base"`
	event.RemoteHref = "/calendars/me/pull-repeated-missing-boundary.ics"
	if errorValue := service.writeCalendarEventWithSource(contextValue, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	firstMissingBoundary, errorValue := service.reserveCalendarConflictCandidateTime(contextValue, time.Date(2036, 7, 16, 17, 0, 0, 0, time.UTC))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.upsertCalendarRemoteEventState(contextValue, calendarRemoteEventState{
		AccountID:   account.ID,
		CalendarURL: target.CalendarURL,
		EventUID:    event.UID,
		LastSeenAt:  firstMissingBoundary.Add(-time.Hour).Format(time.RFC3339Nano),
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	localUpdated := event
	localUpdated.Title = "Local After First Missing Snapshot"
	if errorValue := service.writeCalendarEvent(contextValue, localUpdated); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, isCurrentTarget, errorValue := service.applyCalendarPullCycleIfCurrent(contextValue, account, target, nil, "", firstMissingBoundary); errorValue != nil || !isCurrentTarget {
		t.Fatalf("first pull current=%v error=%v", isCurrentTarget, errorValue)
	}
	firstState, found, errorValue := service.readCalendarRemoteEventState(contextValue, account.ID, target.CalendarURL, event.UID)
	if errorValue != nil || !found {
		t.Fatalf("first state found=%v error=%v", found, errorValue)
	}
	if parseCalendarConflictTime(firstState.MissingDetectedAt) != firstMissingBoundary {
		t.Errorf("first missing detected at=%q boundary=%s", firstState.MissingDetectedAt, firstMissingBoundary)
	}
	secondMissingBoundary, errorValue := service.reserveCalendarConflictCandidateTime(contextValue, firstMissingBoundary.Add(time.Hour))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, isCurrentTarget, errorValue := service.applyCalendarPullCycleIfCurrent(contextValue, account, target, nil, "", secondMissingBoundary); errorValue != nil || !isCurrentTarget {
		t.Fatalf("second pull current=%v error=%v", isCurrentTarget, errorValue)
	}
	finalEvent, found, errorValue := service.readCalendarEventByID(contextValue, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("final event found=%v error=%v", found, errorValue)
	}
	if finalEvent.Title != "Local After First Missing Snapshot" {
		t.Fatalf("title=%q", finalEvent.Title)
	}
	pendingRows, errorValue := service.listPendingCalendarOutbox(contextValue, account.ID)
	if errorValue != nil || len(pendingRows) != 1 {
		t.Fatalf("pending rows=%d error=%v", len(pendingRows), errorValue)
	}
	finalState, found, errorValue := service.readCalendarRemoteEventState(contextValue, account.ID, target.CalendarURL, event.UID)
	if errorValue != nil || !found {
		t.Fatalf("final state found=%v error=%v", found, errorValue)
	}
	if parseCalendarConflictTime(finalState.MissingDetectedAt) != firstMissingBoundary {
		t.Fatalf("final missing detected at=%q first boundary=%s", finalState.MissingDetectedAt, firstMissingBoundary)
	}
}
