package admind

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

func TestAttendanceAbsenceCreateSummaryAndCancel(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	createResponse := createAttendanceAbsenceForTest(t, service, "member@example.com", `{
		"kind": "leave",
		"startDate": "2026-05-13",
		"endDate": "2026-05-15",
		"reason": "family"
	}`)

	if len(createResponse.Absences) != 3 {
		t.Fatalf("expected 3 created absences, got %d", len(createResponse.Absences))
	}
	if createResponse.Absences[0].Email != "member@example.com" {
		t.Fatalf("expected created absence to use actor email, got %q", createResponse.Absences[0].Email)
	}
	if createResponse.Absences[0].Kind != "leave" {
		t.Fatalf("expected leave kind, got %q", createResponse.Absences[0].Kind)
	}
	if createResponse.Absences[0].LabelKey != "leave" {
		t.Fatalf("expected locale-neutral label key, got %q", createResponse.Absences[0].LabelKey)
	}
	if createResponse.Absences[0].RangeID == "" {
		t.Fatal("expected range id on absence occurrence")
	}
	for _, absence := range createResponse.Absences {
		if absence.RangeID != createResponse.Absences[0].RangeID {
			t.Fatalf("expected one range for created period, got %+v", createResponse.Absences)
		}
	}

	summaryResponse := readAttendanceSummaryForTest(t, service, "member@example.com", "2026-05")
	if len(summaryResponse.Absences) != 3 {
		t.Fatalf("expected 3 summary absences, got %d", len(summaryResponse.Absences))
	}
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatalf("open attendance database: %v", errorValue)
	}
	defer database.Close()
	if attendanceActiveAbsenceOccurrenceCount(t, database, "member@example.com") != 3 {
		t.Fatal("expected 3 active absence occurrences after create")
	}

	cancelRequest := httptest.NewRequest(http.MethodDelete, "/attendance/api/absences/"+createResponse.Absences[0].ID, nil)
	cancelRequest.RemoteAddr = "203.0.113.10:1234"
	cancelRequest.Header.Set("X-Forwarded-Email", "member@example.com")
	cancelRecorder := httptest.NewRecorder()
	service.handleAttendance(cancelRecorder, cancelRequest)
	if cancelRecorder.Code != http.StatusOK {
		t.Fatalf("expected cancel status 200, got %d: %s", cancelRecorder.Code, cancelRecorder.Body.String())
	}

	storedAbsences, errorValue := service.readAttendanceAbsences(context.Background(), "2026-05", "member@example.com")
	if errorValue != nil {
		t.Fatalf("read absences: %v", errorValue)
	}
	if len(storedAbsences) != 2 {
		t.Fatalf("expected 2 active absences after cancel, got %d", len(storedAbsences))
	}
	if attendanceActiveAbsenceOccurrenceCount(t, database, "member@example.com") != 2 {
		t.Fatal("expected 2 active absence occurrences after cancel")
	}
	if attendanceCanceledAbsenceOccurrenceCount(t, database, "member@example.com") != 3 {
		t.Fatal("expected original 3 absence occurrences to be canceled after split")
	}
}

func TestAttendanceAbsenceCreateIsIdempotentForActiveDate(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	firstResponse := createAttendanceAbsenceForTest(t, service, "member@example.com", `{
		"kind": "leave",
		"startDate": "2026-05-13"
	}`)
	secondResponse := createAttendanceAbsenceForTest(t, service, "member@example.com", `{
		"kind": "leave",
		"startDate": "2026-05-13"
	}`)

	if len(firstResponse.Absences) != 1 {
		t.Fatalf("expected first create to return 1 absence, got %d", len(firstResponse.Absences))
	}
	if len(secondResponse.Absences) != 0 {
		t.Fatalf("expected duplicate create to return no new absences, got %d", len(secondResponse.Absences))
	}

	summaryResponse := readAttendanceSummaryForTest(t, service, "member@example.com", "2026-05")
	if len(summaryResponse.Absences) != 1 {
		t.Fatalf("expected 1 active absence after duplicate create, got %d", len(summaryResponse.Absences))
	}
}

func TestAttendanceAbsenceCreateSkipsWeekends(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	response := createAttendanceAbsenceForTest(t, service, "member@example.com", `{
		"kind": "leave",
		"startDate": "2026-05-15",
		"endDate": "2026-05-18"
	}`)

	if len(response.Absences) != 2 {
		t.Fatalf("expected 2 weekday absences, got %d", len(response.Absences))
	}
	if response.Absences[0].Date != "2026-05-15" || response.Absences[1].Date != "2026-05-18" {
		t.Fatalf("expected Friday and Monday absences, got %+v", response.Absences)
	}
	if response.Absences[0].RangeID != response.Absences[1].RangeID {
		t.Fatalf("expected weekend-spanning absence to remain one range, got %+v", response.Absences)
	}

	summaryResponse := readAttendanceSummaryForTest(t, service, "member@example.com", "2026-05")
	if len(summaryResponse.Absences) != 2 {
		t.Fatalf("expected 2 summary absences, got %d", len(summaryResponse.Absences))
	}
}

func TestAttendanceAbsenceCreateMergesAdjacentSameKindRange(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	createAttendanceAbsenceForTest(t, service, "member@example.com", `{
		"kind": "other",
		"startDate": "2026-05-11",
		"endDate": "2026-05-13"
	}`)
	createAttendanceAbsenceForTest(t, service, "member@example.com", `{
		"kind": "other",
		"startDate": "2026-05-14",
		"endDate": "2026-05-15"
	}`)

	ranges, errorValue := service.readAttendanceAbsenceRanges(context.Background(), "2026-05-01", "2026-06-01", "member@example.com")
	if errorValue != nil {
		t.Fatalf("read active ranges: %v", errorValue)
	}
	if len(ranges) != 1 {
		t.Fatalf("expected adjacent same-kind ranges to merge, got %+v", ranges)
	}
	if ranges[0].StartDate != "2026-05-11" || ranges[0].EndDate != "2026-05-15" {
		t.Fatalf("expected merged range bounds, got %+v", ranges[0])
	}
}

func TestAttendanceAbsenceCreateRejectsActiveOccurrenceConflict(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatalf("open attendance database: %v", errorValue)
	}
	defer database.Close()
	_, errorValue = database.ExecContext(context.Background(), `
INSERT INTO attendance_absence_occurrences (id, range_id, email, date, created_at, canceled_at)
VALUES (?, ?, ?, ?, ?, ?)`,
		"existing-occurrence",
		"external-range",
		"member@example.com",
		"2026-05-13",
		"2026-05-01T00:00:00Z",
		"",
	)
	if errorValue != nil {
		t.Fatalf("insert occurrence guard row: %v", errorValue)
	}

	request := httptest.NewRequest(http.MethodPost, "/attendance/api/absences", strings.NewReader(`{
		"kind": "leave",
		"startDate": "2026-05-13"
	}`))
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-Email", "member@example.com")
	recorder := httptest.NewRecorder()

	service.handleAttendance(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if attendanceAbsenceRangeCount(t, database, "member@example.com") != 0 {
		t.Fatal("expected conflicting range insert to roll back")
	}
}

func TestAttendanceAbsenceCreateAllowsSameDateForDifferentPeople(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	createAttendanceAbsenceForTest(t, service, "admin@example.com", `{
		"email": "member@example.com",
		"kind": "leave",
		"startDate": "2026-05-13"
	}`)
	createAttendanceAbsenceForTest(t, service, "admin@example.com", `{
		"email": "other@example.com",
		"kind": "other",
		"startDate": "2026-05-13"
	}`)

	summaryResponse := readAttendanceSummaryForTest(t, service, "admin@example.com", "2026-05")
	// Two people are off on one date. Which of them the summary lists first
	// follows from their email addresses and is not what this is about.
	held := strings.Split(attendanceAbsenceDateKinds(summaryResponse.Absences), ",")
	sort.Strings(held)
	if strings.Join(held, ",") != "2026-05-13:leave,2026-05-13:other" {
		t.Fatalf("expected same-date absences for different people, got %+v", summaryResponse.Absences)
	}
}

func TestAttendanceAbsenceCreateReplacesOverlappingDateWithSplitRanges(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	createAttendanceAbsenceForTest(t, service, "member@example.com", `{
		"kind": "leave",
		"startDate": "2026-05-11",
		"endDate": "2026-05-15"
	}`)
	createAttendanceAbsenceForTest(t, service, "member@example.com", `{
		"kind": "other",
		"startDate": "2026-05-13",
		"endDate": "2026-05-13"
	}`)

	summaryResponse := readAttendanceSummaryForTest(t, service, "member@example.com", "2026-05")
	expected := []string{
		"2026-05-11:leave",
		"2026-05-12:leave",
		"2026-05-13:other",
		"2026-05-14:leave",
		"2026-05-15:leave",
	}
	if attendanceAbsenceDateKinds(summaryResponse.Absences) != strings.Join(expected, ",") {
		t.Fatalf("unexpected split replacement: %+v", summaryResponse.Absences)
	}
	ranges, errorValue := service.readAttendanceAbsenceRanges(context.Background(), "2026-05-01", "2026-06-01", "member@example.com")
	if errorValue != nil {
		t.Fatalf("read active ranges: %v", errorValue)
	}
	if len(ranges) != 3 {
		t.Fatalf("expected 3 active split ranges, got %+v", ranges)
	}
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatalf("open attendance database: %v", errorValue)
	}
	defer database.Close()
	if attendanceActiveAbsenceOccurrenceCount(t, database, "member@example.com") != 5 {
		t.Fatal("expected 5 active occurrence rows after replacement")
	}
	if attendanceCanceledAbsenceOccurrenceCount(t, database, "member@example.com") != 5 {
		t.Fatal("expected 5 canceled occurrence rows from replaced range")
	}
}

func TestAttendanceAbsenceCancelSplitsRange(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	response := createAttendanceAbsenceForTest(t, service, "member@example.com", `{
		"kind": "leave",
		"startDate": "2026-05-11",
		"endDate": "2026-05-15"
	}`)
	var wednesdayID string
	for _, absence := range response.Absences {
		if absence.Date == "2026-05-13" {
			wednesdayID = absence.ID
		}
	}
	if wednesdayID == "" {
		t.Fatalf("expected Wednesday occurrence in response: %+v", response.Absences)
	}

	cancelRequest := httptest.NewRequest(http.MethodDelete, "/attendance/api/absences/"+wednesdayID, nil)
	cancelRequest.RemoteAddr = "203.0.113.10:1234"
	cancelRequest.Header.Set("X-Forwarded-Email", "member@example.com")
	cancelRecorder := httptest.NewRecorder()
	service.handleAttendance(cancelRecorder, cancelRequest)
	if cancelRecorder.Code != http.StatusOK {
		t.Fatalf("expected cancel status 200, got %d: %s", cancelRecorder.Code, cancelRecorder.Body.String())
	}

	summaryResponse := readAttendanceSummaryForTest(t, service, "member@example.com", "2026-05")
	expected := []string{
		"2026-05-11:leave",
		"2026-05-12:leave",
		"2026-05-14:leave",
		"2026-05-15:leave",
	}
	if attendanceAbsenceDateKinds(summaryResponse.Absences) != strings.Join(expected, ",") {
		t.Fatalf("unexpected split cancel: %+v", summaryResponse.Absences)
	}
}

func TestAttendanceAbsenceSummaryIncludesVisibleMonthGridOverlap(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	createAttendanceAbsenceForTest(t, service, "member@example.com", `{
		"kind": "leave",
		"startDate": "2026-05-29",
		"endDate": "2026-06-02"
	}`)

	summaryResponse := readAttendanceSummaryForTest(t, service, "member@example.com", "2026-05")
	expected := []string{
		"2026-05-29:leave",
		"2026-06-01:leave",
		"2026-06-02:leave",
	}
	if attendanceAbsenceDateKinds(summaryResponse.Absences) != strings.Join(expected, ",") {
		t.Fatalf("expected visible grid overlap absences, got %+v", summaryResponse.Absences)
	}
}
