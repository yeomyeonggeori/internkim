package admind

import (
	"context"
	"reflect"
	"testing"
)

func TestAttendanceAbsenceCacheMonthsForDatesIncludesAdjacentGrids(t *testing.T) {
	months, errorValue := attendanceAbsenceCacheMonthsForDates([]string{"2026-08-01"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	want := []string{"2026-07", "2026-08"}
	if !reflect.DeepEqual(months, want) {
		t.Fatalf("months = %#v, want %#v", months, want)
	}
}

func TestInsertAttendanceAbsenceRangeInvalidatesVisibleMonthGridCaches(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	email := "staff@example.com"
	monthRevisions := map[string]int64{}
	for _, month := range []string{"2026-07", "2026-08"} {
		monthRevisions[month] = primeAttendanceSummaryCacheForTest(t, service, attendanceSummaryCacheKindAbsences, month)
	}
	if _, errorValue := service.insertAttendanceAbsenceRange(ctx, email, "annual", "2026-07-31", "2026-07-31", "Vacation", email); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, month := range []string{"2026-07", "2026-08"} {
		assertAttendanceSummaryCacheInvalidatedForTest(t, service, attendanceSummaryCacheKindAbsences, month, monthRevisions[month])
	}
}

func TestInsertAttendanceAbsenceRangeRollsBackWhenCacheInvalidationFails(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	month := "2026-07"
	email := "staff@example.com"
	revision := primeAttendanceSummaryCacheForTest(t, service, attendanceSummaryCacheKindAbsences, month)
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.Exec(`
CREATE TRIGGER reject_attendance_summary_cache_delete_for_absence_rollback
BEFORE DELETE ON attendance_summary_cache_entries
BEGIN
	SELECT RAISE(FAIL, 'forced cache invalidation failure');
END`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.insertAttendanceAbsenceRange(ctx, email, "annual", "2026-07-13", "2026-07-13", "Vacation", email); errorValue == nil {
		t.Fatal("insert attendance absence range succeeded when cache invalidation failed")
	}
	database, errorValue = service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var rangeCount int
	var occurrenceCount int
	if errorValue := database.QueryRow(`
SELECT
	(SELECT COUNT(*) FROM attendance_absence_ranges WHERE email = ?),
	(SELECT COUNT(*) FROM attendance_absence_occurrences WHERE email = ?)`, email, email).Scan(&rangeCount, &occurrenceCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if rangeCount != 0 {
		t.Fatalf("absence range count = %d, want 0", rangeCount)
	}
	if occurrenceCount != 0 {
		t.Fatalf("absence occurrence count = %d, want 0", occurrenceCount)
	}
	assertAttendanceSummaryCachePreservedForTest(t, service, attendanceSummaryCacheKindAbsences, month, revision)
}

func TestMergeAttendanceAbsenceRangesInvalidatesRewrittenGridCaches(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	email := "staff@example.com"
	if _, errorValue := service.insertAttendanceAbsenceRange(ctx, email, "annual", "2026-07-10", "2026-07-10", "Vacation", email); errorValue != nil {
		t.Fatal(errorValue)
	}
	monthRevisions := map[string]int64{}
	for _, month := range []string{"2026-06", "2026-07"} {
		monthRevisions[month] = primeAttendanceSummaryCacheForTest(t, service, attendanceSummaryCacheKindAbsences, month)
	}
	if _, errorValue := service.insertAttendanceAbsenceRange(ctx, email, "annual", "2026-07-13", "2026-07-13", "Vacation", email); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, month := range []string{"2026-06", "2026-07"} {
		assertAttendanceSummaryCacheInvalidatedForTest(t, service, attendanceSummaryCacheKindAbsences, month, monthRevisions[month])
	}
}

func TestMergeAttendanceAbsenceRangesInvalidatesConflictingRemainderGrids(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	email := "staff@example.com"
	if _, errorValue := service.insertAttendanceAbsenceRange(ctx, email, "annual", "2026-07-10", "2026-07-10", "Vacation", email); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.insertAttendanceAbsenceRange(ctx, email, "sick", "2026-07-31", "2026-09-01", "Recovery", email); errorValue != nil {
		t.Fatal(errorValue)
	}
	revision := primeAttendanceSummaryCacheForTest(t, service, attendanceSummaryCacheKindAbsences, "2026-09")

	if _, errorValue := service.insertAttendanceAbsenceRange(ctx, email, "annual", "2026-07-13", "2026-07-31", "Vacation", email); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertAttendanceSummaryCacheInvalidatedForTest(t, service, attendanceSummaryCacheKindAbsences, "2026-09", revision)
}

func TestCancelAttendanceAbsenceRangeInvalidatesGridCaches(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	email := "staff@example.com"
	absences, errorValue := service.insertAttendanceAbsenceRange(ctx, email, "annual", "2026-07-30", "2026-07-31", "Vacation", email)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(absences) != 2 {
		t.Fatalf("absences = %+v", absences)
	}
	monthRevisions := map[string]int64{}
	for _, month := range []string{"2026-07", "2026-08"} {
		monthRevisions[month] = primeAttendanceSummaryCacheForTest(t, service, attendanceSummaryCacheKindAbsences, month)
	}
	if errorValue := service.cancelAttendanceAbsenceRange(ctx, absences[0].RangeID); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, month := range []string{"2026-07", "2026-08"} {
		assertAttendanceSummaryCacheInvalidatedForTest(t, service, attendanceSummaryCacheKindAbsences, month, monthRevisions[month])
	}
}

func TestCancelAttendanceAbsenceRangeRollsBackWhenCacheInvalidationFails(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	email := "staff@example.com"
	absences, errorValue := service.insertAttendanceAbsenceRange(ctx, email, "annual", "2026-07-30", "2026-07-31", "Vacation", email)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	revision := primeAttendanceSummaryCacheForTest(t, service, attendanceSummaryCacheKindAbsences, "2026-07")
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.Exec(`
CREATE TRIGGER reject_attendance_summary_cache_delete_for_range_cancel
BEFORE DELETE ON attendance_summary_cache_entries
BEGIN
	SELECT RAISE(FAIL, 'forced cache invalidation failure');
END`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := service.cancelAttendanceAbsenceRange(ctx, absences[0].RangeID); errorValue == nil {
		t.Fatal("cancel attendance absence range succeeded when cache invalidation failed")
	}
	if _, exists, errorValue := service.readAttendanceAbsenceRangeByID(ctx, absences[0].RangeID); errorValue != nil || !exists {
		t.Fatalf("active absence range exists = %v, error = %v", exists, errorValue)
	}
	assertAttendanceSummaryCachePreservedForTest(t, service, attendanceSummaryCacheKindAbsences, "2026-07", revision)
}

func TestCancelAttendanceAbsenceOccurrenceInvalidatesSplitGridCaches(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	email := "staff@example.com"
	absences, errorValue := service.insertAttendanceAbsenceRange(ctx, email, "annual", "2026-07-30", "2026-07-31", "Vacation", email)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(absences) != 2 {
		t.Fatalf("absences = %+v", absences)
	}
	monthRevisions := map[string]int64{}
	for _, month := range []string{"2026-07", "2026-08"} {
		monthRevisions[month] = primeAttendanceSummaryCacheForTest(t, service, attendanceSummaryCacheKindAbsences, month)
	}
	if errorValue := service.cancelAttendanceAbsenceOccurrence(ctx, absences[0].RangeID, absences[0].Date); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, month := range []string{"2026-07", "2026-08"} {
		assertAttendanceSummaryCacheInvalidatedForTest(t, service, attendanceSummaryCacheKindAbsences, month, monthRevisions[month])
	}
}

func TestCancelAttendanceAbsenceOccurrenceRollsBackWhenCacheInvalidationFails(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()
	email := "staff@example.com"
	absences, errorValue := service.insertAttendanceAbsenceRange(ctx, email, "annual", "2026-07-30", "2026-07-31", "Vacation", email)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	revision := primeAttendanceSummaryCacheForTest(t, service, attendanceSummaryCacheKindAbsences, "2026-07")
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.Exec(`
CREATE TRIGGER reject_attendance_summary_cache_delete_for_occurrence_cancel
BEFORE DELETE ON attendance_summary_cache_entries
BEGIN
	SELECT RAISE(FAIL, 'forced cache invalidation failure');
END`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := service.cancelAttendanceAbsenceOccurrence(ctx, absences[0].RangeID, absences[0].Date); errorValue == nil {
		t.Fatal("cancel attendance absence occurrence succeeded when cache invalidation failed")
	}
	if _, exists, errorValue := service.readAttendanceAbsenceRangeByID(ctx, absences[0].RangeID); errorValue != nil || !exists {
		t.Fatalf("active absence range exists = %v, error = %v", exists, errorValue)
	}
	database, errorValue = service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var activeOccurrenceCount int
	if errorValue := database.QueryRow(
		"SELECT COUNT(*) FROM attendance_absence_occurrences WHERE range_id = ? AND canceled_at = ''",
		absences[0].RangeID,
	).Scan(&activeOccurrenceCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if activeOccurrenceCount != 2 {
		t.Fatalf("active occurrence count = %d, want 2", activeOccurrenceCount)
	}
	assertAttendanceSummaryCachePreservedForTest(t, service, attendanceSummaryCacheKindAbsences, "2026-07", revision)
}
