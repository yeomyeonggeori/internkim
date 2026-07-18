package admind

import (
	"context"
	"testing"
	"time"
)

func TestCalendarRemoteMissingBoundAdvancesPastLastSeenDuringWallClockRollback(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	account := seedAccountWithDiscovery(t, service)
	target := activeRemoteCalendarTarget(account)
	event := newLocalTestCalendarEvent("logical-clock-remote-bounds", "Remote Event")
	event.RemoteModifiedAt = time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	lastSeenAt := time.Date(2036, 7, 16, 2, 0, 0, 0, time.UTC)
	if errorValue := service.markCalendarRemoteEventObserved(contextValue, account.ID, target.CalendarURL, event, lastSeenAt); errorValue != nil {
		t.Fatal(errorValue)
	}
	state, errorValue := service.markCalendarRemoteEventMissing(contextValue, account.ID, target.CalendarURL, event.UID, lastSeenAt.Add(-time.Hour))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	observedBound := parseCalendarConflictTime(state.LastSeenAt)
	missingBound := parseCalendarConflictTime(state.MissingDetectedAt)
	if missingBound.Before(observedBound) {
		t.Fatalf("last_seen_at=%s missing_detected_at=%s", observedBound, missingBound)
	}
	firstMissingBound := state.MissingDetectedAt
	state, errorValue = service.markCalendarRemoteEventMissing(contextValue, account.ID, target.CalendarURL, event.UID, lastSeenAt.Add(time.Hour))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if state.MissingDetectedAt != firstMissingBound {
		t.Fatalf("repeated missing bound=%q first bound=%q", state.MissingDetectedAt, firstMissingBound)
	}
	logicalTime := readCalendarConflictLogicalTime(t, service, event.UID)
	if logicalTime != missingBound {
		t.Fatalf("logical time=%s missing_detected_at=%s", logicalTime, missingBound)
	}
}

func TestReservedCalendarRemoteObservationClampsToPriorRemoteBounds(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	account := seedAccountWithDiscovery(t, service)
	target := activeRemoteCalendarTarget(account)
	event := newLocalTestCalendarEvent("reserved-observation-clamp", "Remote Event")
	lastSeenAt := time.Date(2036, 7, 16, 13, 0, 0, 0, time.UTC)
	missingDetectedAt := lastSeenAt.Add(time.Hour)
	if errorValue := service.upsertCalendarRemoteEventState(contextValue, calendarRemoteEventState{
		AccountID:         account.ID,
		CalendarURL:       target.CalendarURL,
		EventUID:          event.UID,
		LastSeenAt:        lastSeenAt.Format(time.RFC3339Nano),
		MissingDetectedAt: missingDetectedAt.Format(time.RFC3339Nano),
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	reservedBoundary := lastSeenAt.Add(-time.Hour)
	if errorValue := service.markCalendarRemoteEventObservedAtReservedBoundary(contextValue, account.ID, target.CalendarURL, event, reservedBoundary); errorValue != nil {
		t.Fatal(errorValue)
	}
	state, found, errorValue := service.readCalendarRemoteEventState(contextValue, account.ID, target.CalendarURL, event.UID)
	if errorValue != nil || !found {
		t.Fatalf("state found=%v error=%v", found, errorValue)
	}
	if parseCalendarConflictTime(state.LastSeenAt) != missingDetectedAt {
		t.Fatalf("last seen at=%s missing bound=%s", state.LastSeenAt, missingDetectedAt)
	}
	if state.MissingDetectedAt != "" {
		t.Fatalf("missing detected at=%q", state.MissingDetectedAt)
	}
}

func TestReservedCalendarRemoteMissingClampsToPriorLastSeenBound(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	account := seedAccountWithDiscovery(t, service)
	target := activeRemoteCalendarTarget(account)
	event := newLocalTestCalendarEvent("reserved-missing-clamp", "Remote Event")
	lastSeenAt := time.Date(2036, 7, 16, 14, 0, 0, 0, time.UTC)
	if errorValue := service.upsertCalendarRemoteEventState(contextValue, calendarRemoteEventState{
		AccountID:   account.ID,
		CalendarURL: target.CalendarURL,
		EventUID:    event.UID,
		LastSeenAt:  lastSeenAt.Format(time.RFC3339Nano),
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	state, errorValue := service.markCalendarRemoteEventMissingAtReservedBoundary(contextValue, account.ID, target.CalendarURL, event.UID, lastSeenAt.Add(-time.Hour))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if parseCalendarConflictTime(state.MissingDetectedAt) != lastSeenAt {
		t.Fatalf("missing detected at=%s last seen at=%s", state.MissingDetectedAt, lastSeenAt)
	}
}
