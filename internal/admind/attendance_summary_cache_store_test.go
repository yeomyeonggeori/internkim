package admind

import (
	"context"
	"testing"
	"time"
)

func TestAttendanceSummaryCacheStoresTeamScope(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC)

	stored, errorValue := service.writeAttendanceSummaryCachePayloadIfCurrent(
		ctx,
		attendanceSummaryCacheKindEvents,
		"2026-07",
		0,
		[]byte(`{"events":[{"email":"team@example.com"}]}`),
		now,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !stored {
		t.Fatal("team cache was not stored")
	}

	teamPayload, _, teamFound, errorValue := service.readAttendanceSummaryCachePayload(ctx, attendanceSummaryCacheKindEvents, "2026-07", now)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !teamFound {
		t.Fatal("team cache was not found")
	}
	if string(teamPayload) != `{"events":[{"email":"team@example.com"}]}` {
		t.Fatalf("team payload = %s", teamPayload)
	}
	if count := attendanceSummaryCacheEntryCountForTest(t, service); count != 1 {
		t.Fatalf("cache entry count = %d, want 1", count)
	}
}

func TestAttendanceSummaryCacheDoesNotReuseEntriesFromPreviousService(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	userRecord := mattermostUserRecord{ID: "user-1", Username: "staff", Email: "staff@example.com", Nickname: "Staff"}
	insertAttendanceSummaryTestEvent(t, service, database, userRecord, attendanceKindClockIn, time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC))
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	firstEvents, errorValue := service.readCachedAttendanceEvents(ctx, "2026-07", "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(firstEvents) != 1 || firstEvents[0].DisplayName != "Staff" {
		t.Fatalf("first events = %+v", firstEvents)
	}
	database, errorValue = service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, "UPDATE attendance_events SET display_name = 'Changed' WHERE id = ?", firstEvents[0].ID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	restartedService := NewService(service.Configuration)
	secondEvents, errorValue := restartedService.readCachedAttendanceEvents(ctx, "2026-07", "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(secondEvents) != 1 || secondEvents[0].DisplayName != "Changed" {
		t.Fatalf("restarted events = %+v", secondEvents)
	}
}

func TestReadAttendanceSummaryCacheSnapshotReturnsMatchingRevisionAndEntry(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `
	INSERT INTO attendance_summary_cache_revisions (cache_kind, month, revision)
	VALUES (?, ?, ?)`, attendanceSummaryCacheKindEvents, "2026-07", 3)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	insertAttendanceSummaryCacheEntryForTest(
		t,
		service,
		attendanceSummaryCacheKindEvents,
		"2026-07",
		3,
		attendanceSummaryCacheSchemaVersion,
		`{"events":[]}`,
		time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC),
	)

	revision, entry, found, errorValue := readAttendanceSummaryCacheSnapshot(
		ctx,
		database,
		attendanceSummaryCacheKindEvents,
		"2026-07",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found {
		t.Fatal("cache snapshot entry was not found")
	}
	if revision != 3 || entry.SourceRevision != 3 {
		t.Fatalf("snapshot revisions = (%d, %d), want (3, 3)", revision, entry.SourceRevision)
	}
}

func TestAttendanceSummaryCacheTreatsInvalidEntriesAsMisses(t *testing.T) {
	testCases := []struct {
		name          string
		payload       string
		schemaVersion int
		cachedAt      time.Time
	}{
		{name: "old schema", payload: `{}`, schemaVersion: attendanceSummaryCacheSchemaVersion + 1, cachedAt: time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC)},
		{name: "expired", payload: `{}`, schemaVersion: attendanceSummaryCacheSchemaVersion, cachedAt: time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service, _ := newAttendanceActionTestService(t)
			insertAttendanceSummaryCacheEntryForTest(
				t,
				service,
				attendanceSummaryCacheKindEvents,
				"2026-07",
				0,
				testCase.schemaVersion,
				testCase.payload,
				testCase.cachedAt,
			)

			_, revision, found, errorValue := service.readAttendanceSummaryCachePayload(
				context.Background(),
				attendanceSummaryCacheKindEvents,
				"2026-07",
				time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC),
			)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if found {
				t.Fatal("invalid cache entry was returned as a hit")
			}
			if revision != 0 {
				t.Fatalf("revision = %d, want 0", revision)
			}
			if count := attendanceSummaryCacheEntryCountForTest(t, service); count != 0 {
				t.Fatalf("cache entry count = %d, want 0", count)
			}
		})
	}
}

func TestReadAttendanceSummaryCachePayloadLeavesJSONValidationToTypedReader(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	now := time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC)
	insertAttendanceSummaryCacheEntryForTest(
		t,
		service,
		attendanceSummaryCacheKindEvents,
		"2026-07",
		0,
		attendanceSummaryCacheSchemaVersion,
		"{",
		now,
	)

	payload, _, found, errorValue := service.readAttendanceSummaryCachePayload(
		context.Background(),
		attendanceSummaryCacheKindEvents,
		"2026-07",
		now,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found || string(payload) != "{" {
		t.Fatalf("found = %v, payload = %q", found, payload)
	}
}

func TestAttendanceSummaryCacheRejectsStaleConditionalWrite(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC)
	type cacheReadResult struct {
		revision int64
		found    bool
		error    error
	}
	type cacheWriteResult struct {
		stored bool
		error  error
	}
	readResults := make(chan cacheReadResult, 1)
	writeResults := make(chan cacheWriteResult, 1)
	allowWrite := make(chan struct{})
	go func() {
		_, revision, found, errorValue := service.readAttendanceSummaryCachePayload(ctx, attendanceSummaryCacheKindEvents, "2026-07", now)
		readResults <- cacheReadResult{revision: revision, found: found, error: errorValue}
		<-allowWrite
		stored, errorValue := service.writeAttendanceSummaryCachePayloadIfCurrent(
			ctx,
			attendanceSummaryCacheKindEvents,
			"2026-07",
			revision,
			[]byte(`{"events":[]}`),
			now,
		)
		writeResults <- cacheWriteResult{stored: stored, error: errorValue}
	}()
	readResult := <-readResults
	if readResult.error != nil {
		t.Fatal(readResult.error)
	}
	if readResult.found {
		t.Fatal("unexpected cache hit")
	}

	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := invalidateAttendanceSummaryCacheMonths(ctx, transaction, attendanceSummaryCacheKindEvents, []string{"2026-07"}); errorValue != nil {
		transaction.Rollback()
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	database.Close()
	close(allowWrite)
	writeResult := <-writeResults
	if writeResult.error != nil {
		t.Fatal(writeResult.error)
	}
	if writeResult.stored {
		t.Fatal("stale cache write stored after revision changed")
	}
	if count := attendanceSummaryCacheEntryCountForTest(t, service); count != 0 {
		t.Fatalf("cache entry count = %d, want 0", count)
	}
}

func TestAttendanceSummaryCacheInvalidationTargetsTeam(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC)
	stored, errorValue := service.writeAttendanceSummaryCachePayloadIfCurrent(
		ctx,
		attendanceSummaryCacheKindEvents,
		"2026-07",
		0,
		[]byte(`{"events":[]}`),
		now,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !stored {
		t.Fatal("team cache was not stored")
	}

	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := invalidateAttendanceSummaryCacheMonths(ctx, transaction, attendanceSummaryCacheKindEvents, []string{"2026-07", "2026-07"}); errorValue != nil {
		transaction.Rollback()
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	database.Close()

	if count := attendanceSummaryCacheEntryCountForTest(t, service); count != 0 {
		t.Fatalf("cache entry count = %d, want 0", count)
	}
	if revision := attendanceSummaryCacheRevisionForTest(t, service, attendanceSummaryCacheKindEvents, "2026-07"); revision != 1 {
		t.Fatalf("team revision = %d, want 1", revision)
	}
}

func TestAttendanceSummaryCacheWriteDoesNotDeleteUnrelatedExpiredRows(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	now := time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC)
	insertAttendanceSummaryCacheEntryForTest(t, service, attendanceSummaryCacheKindEvents, "2026-03", 0, attendanceSummaryCacheSchemaVersion, `{}`, now.Add(-attendanceSummaryCacheLifetime-time.Hour))
	stored, errorValue := service.writeAttendanceSummaryCachePayloadIfCurrent(
		context.Background(),
		attendanceSummaryCacheKindEvents,
		"2026-07",
		0,
		[]byte(`{"events":[{"email":"staff@example.com"}]}`),
		now,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !stored {
		t.Fatal("cache was not stored")
	}
	if count := attendanceSummaryCacheEntryCountForTest(t, service); count != 2 {
		t.Fatalf("cache entry count = %d, want 2", count)
	}
}
