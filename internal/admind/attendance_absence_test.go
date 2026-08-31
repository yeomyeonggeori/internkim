package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type attendanceAbsenceTestResponse struct {
	Absences []attendanceAbsence `json:"absences"`
}

func TestAttendanceAbsenceResponseKeepsLabelsLocaleNeutral(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	response := createAttendanceAbsenceForTest(t, service, "member@example.com", `{
		"kind": "leave",
		"startDate": "2026-05-13"
	}`)

	if response.Absences[0].LabelKey != "leave" {
		t.Fatalf("expected label key to match kind, got %q", response.Absences[0].LabelKey)
	}

	responseJSON, errorValue := json.Marshal(response)
	if errorValue != nil {
		t.Fatalf("marshal response: %v", errorValue)
	}
	if strings.Contains(string(responseJSON), "연차") || strings.Contains(string(responseJSON), "휴가") {
		t.Fatalf("expected locale-neutral absence response, got %s", string(responseJSON))
	}
}

func TestAttendanceAbsenceRejectsUnknownKind(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	request := httptest.NewRequest(http.MethodPost, "/attendance/api/absences", strings.NewReader(`{
		"kind": "unknown",
		"startDate": "2026-05-13"
	}`))
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-Email", "member@example.com")
	recorder := httptest.NewRecorder()

	service.handleAttendance(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestAttendanceAbsenceNormalizesLegacyDayOffToLeave(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	response := createAttendanceAbsenceForTest(t, service, "member@example.com", `{
		"kind": "day_off",
		"startDate": "2026-05-13"
	}`)

	if len(response.Absences) != 1 {
		t.Fatalf("expected 1 created absence, got %d", len(response.Absences))
	}
	if response.Absences[0].Kind != "leave" || response.Absences[0].LabelKey != "leave" {
		t.Fatalf("expected legacy day_off to normalize to leave, got %+v", response.Absences[0])
	}
}

func TestAttendanceAbsenceSummaryNormalizesStoredLegacyDayOffToLeave(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatalf("open attendance database: %v", errorValue)
	}
	defer database.Close()
	_, errorValue = database.ExecContext(context.Background(), `
INSERT INTO attendance_absence_ranges (id, email, kind, start_date, end_date, reason, created_by, created_at, updated_at, canceled_at, replaced_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"legacy-day-off",
		"member@example.com",
		attendanceAbsenceDayOff,
		"2026-05-13",
		"2026-05-13",
		"",
		"member@example.com",
		"2026-05-01T00:00:00Z",
		"2026-05-01T00:00:00Z",
		"",
		"",
	)
	if errorValue != nil {
		t.Fatalf("insert legacy day_off range: %v", errorValue)
	}

	summaryResponse := readAttendanceSummaryForTest(t, service, "member@example.com", "2026-05")
	if len(summaryResponse.Absences) != 1 {
		t.Fatalf("expected 1 summary absence, got %d", len(summaryResponse.Absences))
	}
	if summaryResponse.Absences[0].Kind != "leave" || summaryResponse.Absences[0].LabelKey != "leave" {
		t.Fatalf("expected stored legacy day_off to normalize to leave, got %+v", summaryResponse.Absences[0])
	}
}

func TestAttendanceAbsenceSchemaUsesRangeTable(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatalf("open attendance database: %v", errorValue)
	}
	defer database.Close()

	if !attendanceTableExists(t, database, "attendance_absence_ranges") {
		t.Fatal("expected range absence table")
	}
	if !attendanceTableExists(t, database, "attendance_absence_occurrences") {
		t.Fatal("expected occurrence absence table")
	}
}

func TestAttendanceAbsenceRejectsPrivateKindInputs(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	for _, kind := range []string{"sick_leave", "private_leave"} {
		request := httptest.NewRequest(http.MethodPost, "/attendance/api/absences", strings.NewReader(`{
			"kind": "`+kind+`",
			"startDate": "2026-05-13"
		}`))
		request.RemoteAddr = "203.0.113.10:1234"
		request.Header.Set("X-Forwarded-Email", "member@example.com")
		recorder := httptest.NewRecorder()

		service.handleAttendance(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400 for %q, got %d: %s", kind, recorder.Code, recorder.Body.String())
		}
	}
}

func TestAttendanceAbsenceRejectsRangeOverPerRequestLimit(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	request := httptest.NewRequest(http.MethodPost, "/attendance/api/absences", strings.NewReader(`{
		"kind": "leave",
		"startDate": "2026-01-01",
		"endDate": "2027-01-02"
	}`))
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-Email", "member@example.com")
	recorder := httptest.NewRecorder()

	service.handleAttendance(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "366 days or fewer per request") {
		t.Fatalf("expected per-request limit message, got %q", recorder.Body.String())
	}
}

func createAttendanceAbsenceForTest(t *testing.T, service *Service, actorEmail string, body string) attendanceAbsenceTestResponse {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, "/attendance/api/absences", strings.NewReader(body))
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-Email", actorEmail)
	recorder := httptest.NewRecorder()
	service.handleAttendance(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected create status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response attendanceAbsenceTestResponse
	if errorValue := json.NewDecoder(recorder.Body).Decode(&response); errorValue != nil {
		t.Fatalf("decode create response: %v", errorValue)
	}
	return response
}

func readAttendanceSummaryForTest(t *testing.T, service *Service, actorEmail string, month string) attendanceSummaryResponse {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, "/attendance/api/summary?month="+month, nil)
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-Email", actorEmail)
	recorder := httptest.NewRecorder()
	service.handleAttendance(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected summary status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response attendanceSummaryResponse
	if errorValue := json.NewDecoder(recorder.Body).Decode(&response); errorValue != nil {
		t.Fatalf("decode summary response: %v", errorValue)
	}
	return response
}

func attendanceAbsenceDateKinds(absences []attendanceAbsence) string {
	values := make([]string, 0, len(absences))
	for _, absence := range absences {
		values = append(values, absence.Date+":"+absence.Kind)
	}
	return strings.Join(values, ",")
}

func attendanceTableExists(t *testing.T, database *sql.DB, tableName string) bool {
	t.Helper()

	var count int
	errorValue := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", tableName).Scan(&count)
	if errorValue != nil {
		t.Fatalf("query table %s: %v", tableName, errorValue)
	}
	return count > 0
}

func attendanceAbsenceRangeCount(t *testing.T, database *sql.DB, email string) int {
	t.Helper()

	var count int
	errorValue := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM attendance_absence_ranges WHERE email = ?", email).Scan(&count)
	if errorValue != nil {
		t.Fatalf("query range count: %v", errorValue)
	}
	return count
}

func attendanceActiveAbsenceOccurrenceCount(t *testing.T, database *sql.DB, email string) int {
	t.Helper()

	var count int
	errorValue := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM attendance_absence_occurrences WHERE email = ? AND canceled_at = ''", email).Scan(&count)
	if errorValue != nil {
		t.Fatalf("query active occurrence count: %v", errorValue)
	}
	return count
}

func attendanceCanceledAbsenceOccurrenceCount(t *testing.T, database *sql.DB, email string) int {
	t.Helper()

	var count int
	errorValue := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM attendance_absence_occurrences WHERE email = ? AND canceled_at <> ''", email).Scan(&count)
	if errorValue != nil {
		t.Fatalf("query canceled occurrence count: %v", errorValue)
	}
	return count
}
