package admind

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"
)

func TestCalendarConflictClockAdvancesPastPersistedValueDuringWallClockRollback(t *testing.T) {
	service := newCalendarTestService(t)
	candidate := time.Date(2036, 7, 16, 3, 0, 0, 0, time.UTC)
	first := allocateCommittedCalendarConflictTime(t, service, "logical-clock-rollback@internkim", candidate)
	second := allocateCommittedCalendarConflictTime(t, service, "logical-clock-rollback@internkim", candidate.Add(-time.Hour))
	if first != candidate {
		t.Fatalf("first logical time=%s candidate=%s", first, candidate)
	}
	if second != first.Add(time.Nanosecond) {
		t.Fatalf("second logical time=%s first=%s", second, first)
	}
}

func TestCalendarConflictClockUsesEveryLegacyLocalEvidenceFloor(t *testing.T) {
	candidate := time.Date(2036, 7, 16, 4, 0, 0, 0, time.UTC)
	legacyTime := candidate.Add(time.Hour)
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
			eventUID := "legacy-" + testCase.name + "@internkim"
			testCase.seed(t, service, eventUID, legacyTime)
			allocated := allocateCommittedCalendarConflictTime(t, service, eventUID, candidate)
			if allocated != legacyTime.Add(time.Nanosecond) {
				t.Fatalf("logical time=%s legacy time=%s", allocated, legacyTime)
			}
		})
	}
}

func TestCalendarConflictClockIgnoresRemoteModifiedAtAsFloor(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	candidate := time.Date(2036, 7, 16, 5, 0, 0, 0, time.UTC)
	remoteModifiedAt := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	if errorValue := service.upsertCalendarRemoteEventState(contextValue, calendarRemoteEventState{
		AccountID:        "account-remote-skew",
		CalendarURL:      "/calendars/remote-skew/",
		EventUID:         "remote-skew@internkim",
		RemoteModifiedAt: remoteModifiedAt.Format(time.RFC3339Nano),
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	allocated := allocateCommittedCalendarConflictTime(t, service, "remote-skew@internkim", candidate)
	if allocated != candidate {
		t.Fatalf("logical time=%s candidate=%s remote modified=%s", allocated, candidate, remoteModifiedAt)
	}
}

func TestCalendarConflictClockKeepsUIDsIndependent(t *testing.T) {
	service := newCalendarTestService(t)
	firstCandidate := time.Date(2036, 7, 16, 6, 0, 0, 0, time.UTC)
	secondCandidate := firstCandidate.Add(-time.Hour)
	allocateCommittedCalendarConflictTime(t, service, "logical-clock-first@internkim", firstCandidate)
	second := allocateCommittedCalendarConflictTime(t, service, "logical-clock-second@internkim", secondCandidate)
	if second != secondCandidate {
		t.Fatalf("second UID logical time=%s candidate=%s", second, secondCandidate)
	}
}

func TestCalendarConflictClockSerializesConcurrentSameUIDAllocation(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	database, errorValue := service.openCalendarDatabase(contextValue)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	const allocationCount = 8
	candidate := time.Date(2036, 7, 16, 7, 0, 0, 0, time.UTC)
	start := make(chan struct{})
	results := make(chan time.Time, allocationCount)
	errors := make(chan error, allocationCount)
	waitGroup := sync.WaitGroup{}
	for range allocationCount {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			allocated, allocationError := allocateCalendarConflictTimeInNewTransaction(contextValue, service, "logical-clock-concurrent@internkim", candidate)
			if allocationError != nil {
				errors <- allocationError
				return
			}
			results <- allocated
		}()
	}
	close(start)
	waitGroup.Wait()
	close(results)
	close(errors)
	for allocationError := range errors {
		t.Fatal(allocationError)
	}
	logicalTimes := make([]time.Time, 0, allocationCount)
	for logicalTime := range results {
		logicalTimes = append(logicalTimes, logicalTime)
	}
	if len(logicalTimes) != allocationCount {
		t.Fatalf("logical time count=%d want=%d", len(logicalTimes), allocationCount)
	}
	sort.Slice(logicalTimes, func(leftIndex int, rightIndex int) bool {
		return logicalTimes[leftIndex].Before(logicalTimes[rightIndex])
	})
	for index, logicalTime := range logicalTimes {
		expected := candidate.Add(time.Duration(index) * time.Nanosecond)
		if logicalTime != expected {
			t.Fatalf("logical time[%d]=%s want=%s", index, logicalTime, expected)
		}
	}
}

func TestCalendarConflictClockRollsBackWithCallerTransaction(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	candidate := time.Date(2036, 7, 16, 8, 0, 0, 0, time.UTC)
	database, errorValue := service.openCalendarDatabase(contextValue)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	transaction, errorValue := database.BeginTx(contextValue, nil)
	if errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if _, errorValue := allocateCalendarConflictTime(contextValue, transaction, "logical-clock-rollback-transaction@internkim", candidate); errorValue != nil {
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
	allocated := allocateCommittedCalendarConflictTime(t, service, "logical-clock-rollback-transaction@internkim", candidate)
	if allocated != candidate {
		t.Fatalf("logical time after rollback=%s candidate=%s", allocated, candidate)
	}
}

func allocateCommittedCalendarConflictTime(t *testing.T, service *Service, eventUID string, candidate time.Time) time.Time {
	t.Helper()
	logicalTime, errorValue := allocateCalendarConflictTimeInNewTransaction(context.Background(), service, eventUID, candidate)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return logicalTime
}

func allocateCalendarConflictTimeInNewTransaction(contextValue context.Context, service *Service, eventUID string, candidate time.Time) (time.Time, error) {
	database, errorValue := service.openCalendarDatabase(contextValue)
	if errorValue != nil {
		return time.Time{}, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(contextValue, nil)
	if errorValue != nil {
		return time.Time{}, errorValue
	}
	logicalTime, errorValue := allocateCalendarConflictTime(contextValue, transaction, eventUID, candidate)
	if errorValue != nil {
		transaction.Rollback()
		return time.Time{}, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return time.Time{}, errorValue
	}
	return logicalTime, nil
}

func readCalendarConflictLogicalTime(t *testing.T, service *Service, eventUID string) time.Time {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var logicalTimeUnixNano int64
	if errorValue := database.QueryRowContext(context.Background(), `SELECT logical_time_unix_nano FROM calendar_event_logical_clocks WHERE event_uid = ?`, eventUID).Scan(&logicalTimeUnixNano); errorValue != nil {
		t.Fatal(errorValue)
	}
	return time.Unix(0, logicalTimeUnixNano).UTC()
}

func seedLegacyCalendarEventUpdatedTime(t *testing.T, service *Service, eventUID string, legacyTime time.Time) {
	t.Helper()
	event := newLocalTestCalendarEvent("legacy-event-updated", "Legacy")
	event.UID = eventUID
	if errorValue := service.writeCalendarEventWithSource(context.Background(), event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	updateCalendarEventConflictTimes(t, service, event.ID, legacyTime, time.Time{})
}

func seedLegacyCalendarEventDeletedTime(t *testing.T, service *Service, eventUID string, legacyTime time.Time) {
	t.Helper()
	event := newLocalTestCalendarEvent("legacy-event-deleted", "Legacy")
	event.UID = eventUID
	if errorValue := service.writeCalendarEventWithSource(context.Background(), event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	updateCalendarEventConflictTimes(t, service, event.ID, legacyTime.Add(-time.Hour), legacyTime)
}

func seedLegacyCalendarOutboxCreatedTime(t *testing.T, service *Service, eventUID string, legacyTime time.Time) {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	_, errorValue = database.ExecContext(context.Background(), `
INSERT INTO calendar_outbox(account_id, event_id, event_uid, operation, created_at)
VALUES(?, ?, ?, ?, ?)`, "legacy-account", "legacy-event", eventUID, calendarOutboxOperationPut, legacyTime.Format(time.RFC3339Nano))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
}

func seedLegacyCalendarRemoteLastSeenTime(t *testing.T, service *Service, eventUID string, legacyTime time.Time) {
	t.Helper()
	if errorValue := service.upsertCalendarRemoteEventState(context.Background(), calendarRemoteEventState{
		AccountID:   "legacy-account",
		CalendarURL: "/calendars/legacy/",
		EventUID:    eventUID,
		LastSeenAt:  legacyTime.Format(time.RFC3339Nano),
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func seedLegacyCalendarRemoteMissingTime(t *testing.T, service *Service, eventUID string, legacyTime time.Time) {
	t.Helper()
	if errorValue := service.upsertCalendarRemoteEventState(context.Background(), calendarRemoteEventState{
		AccountID:         "legacy-account",
		CalendarURL:       "/calendars/legacy/",
		EventUID:          eventUID,
		MissingDetectedAt: legacyTime.Format(time.RFC3339Nano),
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func updateCalendarEventConflictTimes(t *testing.T, service *Service, eventID string, updatedAt time.Time, deletedAt time.Time) {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	deletedAtValue := ""
	if !deletedAt.IsZero() {
		deletedAtValue = deletedAt.Format(time.RFC3339Nano)
	}
	result, errorValue := database.ExecContext(context.Background(), `UPDATE calendar_events SET updated_at = ?, deleted_at = ? WHERE id = ?`, updatedAt.Format(time.RFC3339Nano), deletedAtValue, eventID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	affectedRows, errorValue := result.RowsAffected()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if affectedRows != 1 {
		t.Fatal(fmt.Sprintf("updated calendar event rows=%d", affectedRows))
	}
}
