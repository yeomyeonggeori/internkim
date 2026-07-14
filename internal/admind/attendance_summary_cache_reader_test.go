package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReadCachedAttendanceEventsUsesTeamCache(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	userRecord := mattermostUserRecord{ID: "user-1", Username: "staff", Email: "staff@example.com", Nickname: "Staff"}
	insertAttendanceSummaryTestEvent(t, service, database, userRecord, attendanceKindClockIn, time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC))
	database.Close()

	firstEvents, errorValue := service.readCachedAttendanceEvents(ctx, "2026-07", "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(firstEvents) != 1 || firstEvents[0].DisplayName != "Staff" {
		t.Fatalf("first events = %+v", firstEvents)
	}
	if count := attendanceSummaryCacheEntryCountForTest(t, service); count != 1 {
		t.Fatalf("cache entry count = %d, want 1", count)
	}

	database, errorValue = service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, "UPDATE attendance_events SET display_name = 'Changed' WHERE id = ?", firstEvents[0].ID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	database.Close()

	secondEvents, errorValue := service.readCachedAttendanceEvents(ctx, "2026-07", "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(secondEvents) != 1 || secondEvents[0].DisplayName != "Staff" {
		t.Fatalf("cached events = %+v", secondEvents)
	}
}

func TestReadCachedAttendanceEventsBypassesCacheForUser(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	userRecord := mattermostUserRecord{ID: "user-1", Username: "staff", Email: "staff@example.com", Nickname: "Staff"}
	insertAttendanceSummaryTestEvent(t, service, database, userRecord, attendanceKindClockIn, time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC))
	database.Close()

	firstEvents, errorValue := service.readCachedAttendanceEvents(ctx, "2026-07", "staff@example.com")
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
	database.Close()

	secondEvents, errorValue := service.readCachedAttendanceEvents(ctx, "2026-07", "staff@example.com")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(secondEvents) != 1 || secondEvents[0].DisplayName != "Changed" {
		t.Fatalf("events = %+v", secondEvents)
	}
}

func TestReadCachedAttendanceAbsencesBypassesCacheForUser(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	absences, errorValue := service.insertAttendanceAbsenceRange(ctx, "staff@example.com", "annual", "2026-07-13", "2026-07-13", "Original", "staff@example.com")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(absences) != 1 {
		t.Fatalf("created absences = %+v", absences)
	}

	firstAbsences, errorValue := service.readCachedAttendanceAbsences(ctx, "2026-07", "staff@example.com")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(firstAbsences) != 1 || firstAbsences[0].Reason != "Original" {
		t.Fatalf("first absences = %+v", firstAbsences)
	}

	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, "UPDATE attendance_absence_ranges SET reason = 'Changed' WHERE id = ?", firstAbsences[0].RangeID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	database.Close()

	secondAbsences, errorValue := service.readCachedAttendanceAbsences(ctx, "2026-07", "staff@example.com")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(secondAbsences) != 1 || secondAbsences[0].Reason != "Changed" {
		t.Fatalf("absences = %+v", secondAbsences)
	}
	if count := attendanceSummaryCacheEntryCountForTest(t, service); count != 0 {
		t.Fatalf("cache entry count = %d, want 0", count)
	}
}

func TestReadCachedAttendanceSummaryStoresEmptyTeamResults(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	firstEvents, errorValue := service.readCachedAttendanceEvents(ctx, "2026-07", "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	firstAbsences, errorValue := service.readCachedAttendanceAbsences(ctx, "2026-07", "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(firstEvents) != 0 || len(firstAbsences) != 0 {
		t.Fatalf("events = %d, absences = %d, want empty", len(firstEvents), len(firstAbsences))
	}
	if count := attendanceSummaryCacheEntryCountForTest(t, service); count != 2 {
		t.Fatalf("cache entry count = %d, want 2", count)
	}

	secondEvents, errorValue := service.readCachedAttendanceEvents(ctx, "2026-07", "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	secondAbsences, errorValue := service.readCachedAttendanceAbsences(ctx, "2026-07", "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if secondEvents == nil || secondAbsences == nil || len(secondEvents) != 0 || len(secondAbsences) != 0 {
		t.Fatalf("cached events = %+v, cached absences = %+v, want empty arrays", secondEvents, secondAbsences)
	}
}

func TestReadCachedAttendanceEventsRebuildsTypedCorruptPayload(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	userRecord := mattermostUserRecord{ID: "user-1", Username: "staff", Email: "staff@example.com", Nickname: "Staff"}
	insertAttendanceSummaryTestEvent(t, service, database, userRecord, attendanceKindClockIn, time.Date(2026, 7, 13, 9, 0, 0, 0, time.UTC))
	database.Close()
	currentRevision := attendanceSummaryCacheRevisionForTest(t, service, attendanceSummaryCacheKindEvents, "2026-07")
	insertAttendanceSummaryCacheEntryForTest(
		t,
		service,
		attendanceSummaryCacheKindEvents,
		"2026-07",
		currentRevision,
		attendanceSummaryCacheSchemaVersion,
		`{"events":"invalid"}`,
		time.Now().UTC(),
	)
	database, errorValue = service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, cacheEntry, found, errorValue := readAttendanceSummaryCacheSnapshot(ctx, database, attendanceSummaryCacheKindEvents, "2026-07")
	database.Close()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found {
		t.Fatal("corrupt cache entry was not found")
	}
	if cacheEntry.SourceRevision != currentRevision {
		t.Fatalf("corrupt cache revision = %d, source revision = %d", cacheEntry.SourceRevision, currentRevision)
	}

	events, errorValue := service.readCachedAttendanceEvents(ctx, "2026-07", "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != 1 || events[0].Email != "staff@example.com" {
		t.Fatalf("rebuilt events = %+v", events)
	}
}

func TestReadCachedAttendanceAbsencesRebuildsTypedCorruptPayload(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	email := "staff@example.com"
	absences, errorValue := service.insertAttendanceAbsenceRange(ctx, email, "annual", "2026-07-13", "2026-07-13", "Original", email)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(absences) != 1 {
		t.Fatalf("created absences = %+v", absences)
	}
	revision := attendanceSummaryCacheRevisionForTest(t, service, attendanceSummaryCacheKindAbsences, "2026-07")
	insertAttendanceSummaryCacheEntryForTest(
		t,
		service,
		attendanceSummaryCacheKindAbsences,
		"2026-07",
		revision,
		attendanceSummaryCacheSchemaVersion,
		`{"absences":"invalid"}`,
		time.Now().UTC(),
	)

	firstAbsences, errorValue := service.readCachedAttendanceAbsences(ctx, "2026-07", "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(firstAbsences) != 1 || firstAbsences[0].Reason != "Original" {
		t.Fatalf("rebuilt absences = %+v", firstAbsences)
	}
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, "UPDATE attendance_absence_ranges SET reason = 'Changed' WHERE id = ?", firstAbsences[0].RangeID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	secondAbsences, errorValue := service.readCachedAttendanceAbsences(ctx, "2026-07", "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(secondAbsences) != 1 || secondAbsences[0].Reason != "Original" {
		t.Fatalf("cached rebuilt absences = %+v", secondAbsences)
	}
}

func TestAttendanceSummaryReturnsSourceWhenCacheLookupFails(t *testing.T) {
	service, event := prepareAttendanceSummaryCacheFailureTest(t)
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.Exec("DROP TABLE attendance_summary_cache_entries"); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	database.Close()

	response := requestAttendanceSummaryForCacheFailureTest(t, service)
	if len(response.Events) != 1 || response.Events[0].ID != event.ID {
		t.Fatalf("events = %+v", response.Events)
	}
}

func TestAttendanceSummaryReturnsSourceWhenCacheWriteFails(t *testing.T) {
	service, event := prepareAttendanceSummaryCacheFailureTest(t)
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.Exec(`
CREATE TRIGGER reject_attendance_summary_cache_insert
BEFORE INSERT ON attendance_summary_cache_entries
BEGIN
	SELECT RAISE(FAIL, 'forced cache insert failure');
END`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	database.Close()

	response := requestAttendanceSummaryForCacheFailureTest(t, service)
	if len(response.Events) != 1 || response.Events[0].ID != event.ID {
		t.Fatalf("events = %+v", response.Events)
	}
	if count := attendanceSummaryCacheEntryCountForTest(t, service); count != 0 {
		t.Fatalf("cache entry count = %d, want 0", count)
	}
}

func TestAttendanceSummaryReturnsSourceWhenCorruptCacheDeleteFails(t *testing.T) {
	service, event := prepareAttendanceSummaryCacheFailureTest(t)
	revision := attendanceSummaryCacheRevisionForTest(t, service, attendanceSummaryCacheKindEvents, "2026-07")
	insertAttendanceSummaryCacheEntryForTest(
		t,
		service,
		attendanceSummaryCacheKindEvents,
		"2026-07",
		revision,
		attendanceSummaryCacheSchemaVersion,
		`{"events":"invalid"}`,
		time.Now().UTC(),
	)
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.Exec(`
CREATE TRIGGER reject_attendance_summary_cache_delete
BEFORE DELETE ON attendance_summary_cache_entries
BEGIN
	SELECT RAISE(FAIL, 'forced cache delete failure');
END`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	database.Close()

	response := requestAttendanceSummaryForCacheFailureTest(t, service)
	if len(response.Events) != 1 || response.Events[0].ID != event.ID {
		t.Fatalf("events = %+v", response.Events)
	}
	if count := attendanceSummaryCacheEntryCountForTest(t, service); count != 2 {
		t.Fatalf("cache entry count = %d, want 2", count)
	}
}

func prepareAttendanceSummaryCacheFailureTest(t *testing.T) (*Service, attendanceEvent) {
	t.Helper()
	service, _ := newAttendanceActionTestService(t)
	database, event := insertAttendanceCacheTestEvent(t, service, "2026-07-13T09:00:00Z")
	database.Close()
	if errorValue := service.writeAttendanceTeamViewVisibleToAll(context.Background(), true); errorValue != nil {
		t.Fatal(errorValue)
	}
	return service, event
}

func requestAttendanceSummaryForCacheFailureTest(t *testing.T, service *Service) attendanceSummaryResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/attendance/api/summary?month=2026-07", nil)
	request.Header.Set("X-Forwarded-Email", "staff@example.com")
	recorder := httptest.NewRecorder()
	service.writeAttendanceSummary(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var response attendanceSummaryResponse
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &response); errorValue != nil {
		t.Fatal(errorValue)
	}
	return response
}
