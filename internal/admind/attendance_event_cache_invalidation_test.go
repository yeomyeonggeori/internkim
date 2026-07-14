package admind

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"
)

func TestAttendanceEventCacheMonthsForDates(t *testing.T) {
	months, errorValue := attendanceEventCacheMonthsForDates([]string{"2026-08-01", "2026-07-31", "2026-08-01"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	want := []string{"2026-07", "2026-08"}
	if !reflect.DeepEqual(months, want) {
		t.Fatalf("months = %#v, want %#v", months, want)
	}
}

func TestInsertAttendanceEventInvalidatesTeamCache(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	month := "2026-07"
	email := "staff@example.com"
	revision := primeAttendanceSummaryCacheForTest(t, service, attendanceSummaryCacheKindEvents, month)
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	event := service.createAttendanceEvent(
		mattermostUserRecord{ID: "user-1", Username: "staff", Email: email, Nickname: "Staff"},
		attendanceKindClockIn,
		time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC),
		"team-1",
		"attendance-channel",
		"action-post",
		"result-post",
		attendanceLocation{},
	)
	if errorValue := service.insertAttendanceEvent(ctx, database, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertAttendanceSummaryCacheInvalidatedForTest(t, service, attendanceSummaryCacheKindEvents, month, revision)
}

func TestInsertAttendanceEventRollsBackWhenCacheInvalidationFails(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	month := "2026-07"
	email := "staff@example.com"
	revision := primeAttendanceSummaryCacheForTest(t, service, attendanceSummaryCacheKindEvents, month)
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.Exec(`
CREATE TRIGGER reject_attendance_summary_cache_delete_for_event_rollback
BEFORE DELETE ON attendance_summary_cache_entries
BEGIN
	SELECT RAISE(FAIL, 'forced cache invalidation failure');
END`); errorValue != nil {
		t.Fatal(errorValue)
	}
	event := service.createAttendanceEvent(
		mattermostUserRecord{ID: "user-1", Username: "staff", Email: email, Nickname: "Staff"},
		attendanceKindClockIn,
		time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC),
		"team-1",
		"attendance-channel",
		"action-post",
		"result-post",
		attendanceLocation{},
	)
	if errorValue := service.insertAttendanceEvent(ctx, database, event); errorValue == nil {
		t.Fatal("insert attendance event succeeded when cache invalidation failed")
	}
	var eventCount int
	if errorValue := database.QueryRow("SELECT COUNT(*) FROM attendance_events WHERE id = ?", event.ID).Scan(&eventCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if eventCount != 0 {
		t.Fatalf("event count = %d, want 0", eventCount)
	}
	assertAttendanceSummaryCachePreservedForTest(t, service, attendanceSummaryCacheKindEvents, month, revision)
}

func TestMarkAttendanceRepeatedClickInvalidatesEventCaches(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	database, event := insertAttendanceCacheTestEvent(t, service, "2026-07-13T09:00:00Z")
	defer database.Close()
	revision := primeAttendanceSummaryCacheForTest(t, service, attendanceSummaryCacheKindEvents, "2026-07")

	errorValue := service.markAttendanceRepeatedClick(ctx, database, event.ID, time.Date(2026, 7, 13, 9, 1, 0, 0, time.UTC))
	if !errors.Is(errorValue, errAttendanceDuplicateIgnored) {
		t.Fatalf("mark repeated click error = %v", errorValue)
	}
	assertAttendanceSummaryCacheInvalidatedForTest(t, service, attendanceSummaryCacheKindEvents, "2026-07", revision)
}

func TestCancelAttendanceEventInvalidatesEventCaches(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	database, event := insertAttendanceCacheTestEvent(t, service, "2026-07-13T09:00:00Z")
	defer database.Close()
	revision := primeAttendanceSummaryCacheForTest(t, service, attendanceSummaryCacheKindEvents, "2026-07")

	if errorValue := service.cancelAttendanceEventWithReason(
		ctx,
		database,
		"user-token",
		event,
		time.Date(2026, 7, 13, 9, 1, 0, 0, time.UTC),
		attendanceCancelReason,
		"",
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertAttendanceSummaryCacheInvalidatedForTest(t, service, attendanceSummaryCacheKindEvents, "2026-07", revision)
}

func TestCancelAttendanceEventCommitsMutationWhenMattermostNotificationFails(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	notificationError := errors.New("forced Mattermost cancel notification failure")
	notificationAttempts := 0
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.String() != "http://mattermost.local/api/v4/posts" {
			t.Fatalf("Mattermost request = %s %s, want POST /api/v4/posts", request.Method, request.URL.String())
		}
		if authorization := request.Header.Get("Authorization"); authorization != "Bearer user-token" {
			t.Fatalf("Mattermost authorization = %q, want %q", authorization, "Bearer user-token")
		}
		notificationAttempts++
		return nil, notificationError
	})}
	ctx := context.Background()
	database, event := insertAttendanceCacheTestEvent(t, service, "2026-07-13T09:00:00Z")
	defer database.Close()
	month := "2026-07"
	revision := primeAttendanceSummaryCacheForTest(t, service, attendanceSummaryCacheKindEvents, month)
	canceledAt := time.Date(2026, 7, 13, 9, 1, 0, 0, time.UTC)

	errorValue := service.cancelAttendanceEventWithReason(
		ctx,
		database,
		"user-token",
		event,
		canceledAt,
		attendanceCancelReason,
		"",
	)
	if !errors.Is(errorValue, notificationError) {
		t.Fatalf("cancel attendance event error = %v, want %v", errorValue, notificationError)
	}
	if notificationAttempts != 1 {
		t.Fatalf("Mattermost notification attempts = %d, want 1", notificationAttempts)
	}
	var storedCanceledAt string
	if errorValue := database.QueryRow("SELECT canceled_at FROM attendance_events WHERE id = ?", event.ID).Scan(&storedCanceledAt); errorValue != nil {
		t.Fatal(errorValue)
	}
	if storedCanceledAt != canceledAt.Format(time.RFC3339) {
		t.Fatalf("canceled at = %q, want %q", storedCanceledAt, canceledAt.Format(time.RFC3339))
	}
	assertAttendanceSummaryCacheInvalidatedForTest(t, service, attendanceSummaryCacheKindEvents, month, revision)

	errorValue = service.cancelAttendanceEventWithReason(
		ctx,
		database,
		"user-token",
		event,
		canceledAt,
		attendanceCancelReason,
		"",
	)
	if !errors.Is(errorValue, notificationError) {
		t.Fatalf("repeated cancel attendance event error = %v, want %v", errorValue, notificationError)
	}
	if notificationAttempts != 2 {
		t.Fatalf("Mattermost notification attempts = %d, want 2", notificationAttempts)
	}
	if errorValue := database.QueryRow("SELECT canceled_at FROM attendance_events WHERE id = ?", event.ID).Scan(&storedCanceledAt); errorValue != nil {
		t.Fatal(errorValue)
	}
	if storedCanceledAt != canceledAt.Format(time.RFC3339) {
		t.Fatalf("canceled at after repeated call = %q, want %q", storedCanceledAt, canceledAt.Format(time.RFC3339))
	}
	assertAttendanceSummaryCacheInvalidatedForTest(t, service, attendanceSummaryCacheKindEvents, month, revision+1)
}

func TestCancelAttendanceEventDoesNotPostWhenCacheInvalidationFails(t *testing.T) {
	service, posts := newAttendanceActionTestService(t)
	ctx := context.Background()
	database, event := insertAttendanceCacheTestEvent(t, service, "2026-07-13T09:00:00Z")
	defer database.Close()
	revision := primeAttendanceSummaryCacheForTest(t, service, attendanceSummaryCacheKindEvents, "2026-07")
	if _, errorValue := database.Exec(`
CREATE TRIGGER reject_attendance_summary_cache_delete_for_cancel_rollback
BEFORE DELETE ON attendance_summary_cache_entries
BEGIN
	SELECT RAISE(FAIL, 'forced cache invalidation failure');
END`); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := service.cancelAttendanceEventWithReason(
		ctx,
		database,
		"user-token",
		event,
		time.Date(2026, 7, 13, 9, 1, 0, 0, time.UTC),
		attendanceCancelReason,
		"",
	); errorValue == nil {
		t.Fatal("cancel attendance event succeeded when cache invalidation failed")
	}
	var canceledAt string
	if errorValue := database.QueryRow("SELECT canceled_at FROM attendance_events WHERE id = ?", event.ID).Scan(&canceledAt); errorValue != nil {
		t.Fatal(errorValue)
	}
	if canceledAt != "" {
		t.Fatalf("canceled at = %q, want empty", canceledAt)
	}
	assertAttendanceSummaryCachePreservedForTest(t, service, attendanceSummaryCacheKindEvents, "2026-07", revision)
	if len(*posts) != 0 {
		t.Fatalf("Mattermost post count = %d, want 0", len(*posts))
	}
}

func TestCancelAttendanceEventDoesNotPostWhenEventDoesNotExist(t *testing.T) {
	service, posts := newAttendanceActionTestService(t)
	ctx := context.Background()
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	event := service.createAttendanceEvent(
		mattermostUserRecord{ID: "user-1", Username: "staff", Email: "staff@example.com", Nickname: "Staff"},
		attendanceKindClockIn,
		time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC),
		"team-1",
		"attendance-channel",
		"action-post",
		"result-post",
		attendanceLocation{},
	)

	errorValue = service.cancelAttendanceEventWithReason(
		ctx,
		database,
		"user-token",
		event,
		time.Date(2026, 7, 13, 9, 1, 0, 0, time.UTC),
		attendanceCancelReason,
		"",
	)
	if len(*posts) != 0 {
		t.Fatalf("Mattermost post count = %d, want 0", len(*posts))
	}
	if errorValue == nil {
		t.Fatal("cancel missing attendance event returned nil")
	}
}

func TestCancelAttendanceEventInvalidatesCurrentMonthWhenCallerEventIsStale(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	database, staleEvent := insertAttendanceCacheTestEvent(t, service, "2026-06-30T09:00:00Z")
	defer database.Close()
	if errorValue := service.updateAttendanceEventTime(ctx, database, staleEvent, time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)); errorValue != nil {
		t.Fatal(errorValue)
	}
	revision := primeAttendanceSummaryCacheForTest(t, service, attendanceSummaryCacheKindEvents, "2026-07")

	if errorValue := service.cancelAttendanceEventRecord(
		ctx,
		database,
		staleEvent,
		time.Date(2026, 7, 1, 9, 1, 0, 0, time.UTC),
		attendanceCancelReason,
		"",
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertAttendanceSummaryCacheInvalidatedForTest(t, service, attendanceSummaryCacheKindEvents, "2026-07", revision)
}

func TestInsertAttendanceEventOverrideInvalidatesOldAndNewMonthCaches(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	database, event := insertAttendanceCacheTestEvent(t, service, "2026-06-30T09:00:00Z")
	defer database.Close()
	monthRevisions := map[string]int64{}
	for _, month := range []string{"2026-06", "2026-07"} {
		monthRevisions[month] = primeAttendanceSummaryCacheForTest(t, service, attendanceSummaryCacheKindEvents, month)
	}
	override := attendanceEventOverride{
		ID:                 "override-1",
		EventID:            event.ID,
		EditedBy:           event.Email,
		EditedAt:           "2026-07-01T00:00:00Z",
		Reason:             "Correction",
		OriginalOccurredAt: event.OccurredAt,
		OriginalLocalDate:  event.LocalDate,
		OriginalLocalTime:  event.LocalTime,
		OverrideOccurredAt: "2026-07-01T09:00:00Z",
		OverrideLocalDate:  "2026-07-01",
		OverrideLocalTime:  "09:00:00",
	}
	if errorValue := service.insertAttendanceEventOverride(ctx, database, override); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, month := range []string{"2026-06", "2026-07"} {
		assertAttendanceSummaryCacheInvalidatedForTest(t, service, attendanceSummaryCacheKindEvents, month, monthRevisions[month])
	}
}

func TestUpdateAttendanceEventTimeInvalidatesOldAndNewMonthCaches(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	database, event := insertAttendanceCacheTestEvent(t, service, "2026-06-30T09:00:00Z")
	defer database.Close()
	monthRevisions := map[string]int64{}
	for _, month := range []string{"2026-06", "2026-07"} {
		monthRevisions[month] = primeAttendanceSummaryCacheForTest(t, service, attendanceSummaryCacheKindEvents, month)
	}
	newTime := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	if errorValue := service.updateAttendanceEventTime(ctx, database, event, newTime); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, month := range []string{"2026-06", "2026-07"} {
		assertAttendanceSummaryCacheInvalidatedForTest(t, service, attendanceSummaryCacheKindEvents, month, monthRevisions[month])
	}
}

func TestUpdateAttendanceEventTimeInvalidatesCurrentMonthWhenCallerEventIsStale(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	database, staleEvent := insertAttendanceCacheTestEvent(t, service, "2026-06-30T09:00:00Z")
	defer database.Close()
	if errorValue := service.updateAttendanceEventTime(ctx, database, staleEvent, time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)); errorValue != nil {
		t.Fatal(errorValue)
	}
	revision := primeAttendanceSummaryCacheForTest(t, service, attendanceSummaryCacheKindEvents, "2026-07")

	if errorValue := service.updateAttendanceEventTime(ctx, database, staleEvent, time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertAttendanceSummaryCacheInvalidatedForTest(t, service, attendanceSummaryCacheKindEvents, "2026-07", revision)
}

func TestRepairFutureAttendanceEventsInvalidatesOldAndNewMonthCaches(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	if errorValue := service.writeWorkspaceSettingsFile(workspaceSettings{Language: workspaceLanguageKorean, TimeZone: "Asia/Seoul"}); errorValue != nil {
		t.Fatal(errorValue)
	}
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	userRecord := mattermostUserRecord{ID: "user-1", Username: "staff", Email: "staff@example.com", Nickname: "Staff"}
	futureTime := time.Date(2026, 7, 1, 0, 30, 0, 0, location)
	event := service.createAttendanceEvent(userRecord, attendanceKindClockIn, futureTime.UTC(), "team-1", "attendance-channel", "action-post", "result-post", attendanceLocation{})
	if errorValue := service.insertAttendanceEvent(ctx, database, event); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	monthRevisions := map[string]int64{}
	for _, month := range []string{"2026-06", "2026-07"} {
		monthRevisions[month] = primeAttendanceSummaryCacheForTest(t, service, attendanceSummaryCacheKindEvents, month)
	}
	now := time.Date(2026, 6, 30, 8, 30, 0, 0, location).UTC()
	if errorValue := service.repairFutureAttendanceEvents(ctx, now); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, month := range []string{"2026-06", "2026-07"} {
		assertAttendanceSummaryCacheInvalidatedForTest(t, service, attendanceSummaryCacheKindEvents, month, monthRevisions[month])
	}
}

func TestDeleteAttendanceEventByResultPostIDInvalidatesEventCaches(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	database, event := insertAttendanceCacheTestEvent(t, service, "2026-07-13T09:00:00Z")
	database.Close()
	revision := primeAttendanceSummaryCacheForTest(t, service, attendanceSummaryCacheKindEvents, "2026-07")

	if errorValue := service.deleteAttendanceEventByResultPostID(ctx, event.ResultPostID); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertAttendanceSummaryCacheInvalidatedForTest(t, service, attendanceSummaryCacheKindEvents, "2026-07", revision)
}

func TestDeleteAttendanceEventByResultPostIDInvalidatesEveryAffectedMonth(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	email := "staff@example.com"
	resultPostID := "shared-result-post"
	userRecord := mattermostUserRecord{ID: "user-1", Username: "staff", Email: email, Nickname: "Staff"}
	overrideEvent := attendanceEvent{}
	for _, occurredAt := range []time.Time{
		time.Date(2026, 6, 30, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC),
	} {
		event := service.createAttendanceEvent(
			userRecord,
			attendanceKindClockIn,
			occurredAt,
			"team-1",
			"attendance-channel",
			"action-post-"+occurredAt.Format("20060102"),
			resultPostID,
			attendanceLocation{},
		)
		if errorValue := service.insertAttendanceEvent(ctx, database, event); errorValue != nil {
			database.Close()
			t.Fatal(errorValue)
		}
		if overrideEvent.ID == "" {
			overrideEvent = event
		}
	}
	override := attendanceEventOverride{
		ID:                 "override-third-month",
		EventID:            overrideEvent.ID,
		EditedBy:           overrideEvent.Email,
		EditedAt:           "2026-08-01T00:00:00Z",
		Reason:             "Correction",
		OriginalOccurredAt: overrideEvent.OccurredAt,
		OriginalLocalDate:  overrideEvent.LocalDate,
		OriginalLocalTime:  overrideEvent.LocalTime,
		OverrideOccurredAt: "2026-08-01T09:00:00Z",
		OverrideLocalDate:  "2026-08-01",
		OverrideLocalTime:  "09:00:00",
	}
	if errorValue := service.insertAttendanceEventOverride(ctx, database, override); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	monthRevisions := map[string]int64{}
	for _, month := range []string{"2026-06", "2026-07", "2026-08"} {
		monthRevisions[month] = primeAttendanceSummaryCacheForTest(t, service, attendanceSummaryCacheKindEvents, month)
	}

	if errorValue := service.deleteAttendanceEventByResultPostID(ctx, resultPostID); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue = service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var eventCount int
	if errorValue := database.QueryRow("SELECT COUNT(*) FROM attendance_events WHERE result_post_id = ?", resultPostID).Scan(&eventCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if eventCount != 0 {
		t.Fatalf("event count = %d, want 0", eventCount)
	}
	for _, month := range []string{"2026-06", "2026-07", "2026-08"} {
		assertAttendanceSummaryCacheInvalidatedForTest(t, service, attendanceSummaryCacheKindEvents, month, monthRevisions[month])
	}
}
