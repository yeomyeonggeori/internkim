package admind

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"
)

func TestCalendarConflictCandidateClockSeedsFromPersistedClockAfterRestart(t *testing.T) {
	service := newCalendarTestService(t)
	persistedTime := time.Date(2036, 7, 16, 12, 0, 0, 0, time.UTC)
	allocateCommittedCalendarConflictTime(t, service, "candidate-restart@internkim", persistedTime)
	restartedService := NewService(service.Configuration)
	reserved, errorValue := restartedService.reserveCalendarConflictCandidateTime(context.Background(), persistedTime.Add(-time.Hour))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if reserved != persistedTime.Add(time.Nanosecond) {
		t.Fatalf("reserved=%s persisted=%s", reserved, persistedTime)
	}
}

func TestCalendarConflictCandidateClockSeedsFromLegacyEvidenceAfterRestart(t *testing.T) {
	legacyTime := time.Date(2036, 7, 16, 13, 0, 0, 0, time.UTC)
	testCases := []struct {
		name string
		seed func(*testing.T, *Service, string, time.Time)
	}{
		{name: "event updated", seed: seedLegacyCalendarEventUpdatedTime},
		{name: "event deleted", seed: seedLegacyCalendarEventDeletedTime},
		{name: "outbox created", seed: seedLegacyCalendarOutboxCreatedTime},
		{name: "remote last seen", seed: seedLegacyCalendarRemoteLastSeenTime},
		{name: "remote missing detected", seed: seedLegacyCalendarRemoteMissingTime},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := newCalendarTestService(t)
			testCase.seed(t, service, "candidate-legacy-"+testCase.name+"@internkim", legacyTime)
			restartedService := NewService(service.Configuration)
			reserved, errorValue := restartedService.reserveCalendarConflictCandidateTime(context.Background(), legacyTime.Add(-time.Hour))
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if reserved != legacyTime.Add(time.Nanosecond) {
				t.Fatalf("reserved=%s legacy=%s", reserved, legacyTime)
			}
		})
	}
}

func TestCalendarConflictCandidateClockIgnoresRemoteModifiedAtWhenSeeding(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	candidate := time.Date(2036, 7, 16, 13, 30, 0, 0, time.UTC)
	if errorValue := service.upsertCalendarRemoteEventState(contextValue, calendarRemoteEventState{
		AccountID:        "candidate-remote-skew-account",
		CalendarURL:      "/calendars/candidate-remote-skew/",
		EventUID:         "candidate-remote-skew@internkim",
		RemoteModifiedAt: time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	restartedService := NewService(service.Configuration)
	reserved, errorValue := restartedService.reserveCalendarConflictCandidateTime(contextValue, candidate)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if reserved != candidate {
		t.Fatalf("reserved=%s candidate=%s", reserved, candidate)
	}
}

func TestCalendarConflictCandidateClockSerializesConcurrentReservations(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	const reservationCount = 8
	candidate := time.Date(2036, 7, 16, 14, 0, 0, 0, time.UTC)
	start := make(chan struct{})
	results := make(chan time.Time, reservationCount)
	errors := make(chan error, reservationCount)
	waitGroup := sync.WaitGroup{}
	for range reservationCount {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			reserved, errorValue := service.reserveCalendarConflictCandidateTime(contextValue, candidate)
			if errorValue != nil {
				errors <- errorValue
				return
			}
			results <- reserved
		}()
	}
	close(start)
	waitGroup.Wait()
	close(results)
	close(errors)
	for errorValue := range errors {
		t.Fatal(errorValue)
	}
	reservedTimes := make([]time.Time, 0, reservationCount)
	for reserved := range results {
		reservedTimes = append(reservedTimes, reserved)
	}
	sort.Slice(reservedTimes, func(leftIndex int, rightIndex int) bool {
		return reservedTimes[leftIndex].Before(reservedTimes[rightIndex])
	})
	for index, reserved := range reservedTimes {
		expected := candidate.Add(time.Duration(index) * time.Nanosecond)
		if reserved != expected {
			t.Fatalf("reserved[%d]=%s want=%s", index, reserved, expected)
		}
	}
}

func TestCalendarConflictCandidateClockOrdersLocalActionsAcrossUIDs(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	account := seedAccountWithDiscovery(t, service)
	snapshotBoundary, errorValue := service.reserveCalendarConflictCandidateTime(contextValue, time.Date(2036, 7, 16, 15, 0, 0, 0, time.UTC))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	firstEvent := newLocalTestCalendarEvent("candidate-first-uid", "First")
	if errorValue := service.writeCalendarEvent(contextValue, firstEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	secondEvent := newLocalTestCalendarEvent("candidate-second-uid", "Second")
	if errorValue := service.writeCalendarEvent(contextValue, secondEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	rows, errorValue := service.listCalendarOutbox(contextValue, account.ID, true)
	if errorValue != nil || len(rows) != 2 {
		t.Fatalf("outbox rows=%d error=%v", len(rows), errorValue)
	}
	firstCreatedAt := parseCalendarConflictTime(rows[0].CreatedAt)
	secondCreatedAt := parseCalendarConflictTime(rows[1].CreatedAt)
	if !firstCreatedAt.After(snapshotBoundary) {
		t.Fatalf("first created at=%s snapshot boundary=%s", firstCreatedAt, snapshotBoundary)
	}
	if !secondCreatedAt.After(firstCreatedAt) {
		t.Fatalf("second created at=%s first created at=%s", secondCreatedAt, firstCreatedAt)
	}
}

func TestCalendarConflictCandidateClockOrdersLocalDeleteAfterSnapshotBoundary(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("candidate-local-delete", "Delete")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteHref = "/calendars/me/candidate-local-delete.ics"
	if errorValue := service.writeCalendarEventWithSource(contextValue, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	snapshotBoundary, errorValue := service.reserveCalendarConflictCandidateTime(contextValue, time.Date(2036, 7, 16, 15, 30, 0, 0, time.UTC))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.softDeleteCalendarEvent(contextValue, event.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	rows, errorValue := service.listCalendarOutbox(contextValue, account.ID, true)
	if errorValue != nil || len(rows) != 1 {
		t.Fatalf("outbox rows=%d error=%v", len(rows), errorValue)
	}
	deletedAt := parseCalendarConflictTime(rows[0].CreatedAt)
	if !deletedAt.After(snapshotBoundary) {
		t.Fatalf("deleted at=%s snapshot boundary=%s", deletedAt, snapshotBoundary)
	}
}

func TestCalendarConflictCandidateClockKeepsAllocatorResultAfterTransactionRollback(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	database, errorValue := service.openCalendarDatabase(contextValue)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	transaction, errorValue := database.BeginTx(contextValue, nil)
	if errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	candidate := time.Date(2036, 7, 16, 16, 0, 0, 0, time.UTC)
	allocated, errorValue := service.allocateCalendarConflictTime(contextValue, transaction, "candidate-rollback@internkim", candidate)
	if errorValue != nil {
		transaction.Rollback()
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := transaction.Rollback(); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	reserved, errorValue := service.reserveCalendarConflictCandidateTime(contextValue, candidate.Add(-time.Hour))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !reserved.After(allocated) {
		t.Fatalf("reserved=%s allocated before rollback=%s", reserved, allocated)
	}
}
