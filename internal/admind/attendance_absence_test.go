package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type attendanceAbsenceTestResponse struct {
	Absences []attendanceAbsence `json:"absences"`
}

func TestAttendanceAbsenceCreateSummaryAndCancel(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	createResponse := createAttendanceAbsenceForTest(t, service, "staff@example.com", `{
		"kind": "leave",
		"startDate": "2026-05-13",
		"endDate": "2026-05-15",
		"reason": "family"
	}`)

	if len(createResponse.Absences) != 3 {
		t.Fatalf("expected 3 created absences, got %d", len(createResponse.Absences))
	}
	if createResponse.Absences[0].Email != "staff@example.com" {
		t.Fatalf("expected created absence to use actor email, got %q", createResponse.Absences[0].Email)
	}
	if createResponse.Absences[0].Kind != "leave" {
		t.Fatalf("expected leave kind, got %q", createResponse.Absences[0].Kind)
	}
	if createResponse.Absences[0].LabelKey != "leave" {
		t.Fatalf("expected locale-neutral label key, got %q", createResponse.Absences[0].LabelKey)
	}

	summaryResponse := readAttendanceSummaryForTest(t, service, "staff@example.com", "2026-05")
	if len(summaryResponse.Absences) != 3 {
		t.Fatalf("expected 3 summary absences, got %d", len(summaryResponse.Absences))
	}

	cancelRequest := httptest.NewRequest(http.MethodDelete, "/attendance/api/absences/"+createResponse.Absences[0].ID, nil)
	cancelRequest.RemoteAddr = "203.0.113.10:1234"
	cancelRequest.Header.Set("X-Forwarded-Email", "staff@example.com")
	cancelRecorder := httptest.NewRecorder()
	service.handleAttendance(cancelRecorder, cancelRequest)
	if cancelRecorder.Code != http.StatusOK {
		t.Fatalf("expected cancel status 200, got %d: %s", cancelRecorder.Code, cancelRecorder.Body.String())
	}

	storedAbsences, errorValue := service.readAttendanceAbsences(context.Background(), "2026-05", "staff@example.com")
	if errorValue != nil {
		t.Fatalf("read absences: %v", errorValue)
	}
	if len(storedAbsences) != 2 {
		t.Fatalf("expected 2 active absences after cancel, got %d", len(storedAbsences))
	}
}

func TestAttendanceAbsenceCreateIsIdempotentForActiveDate(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	firstResponse := createAttendanceAbsenceForTest(t, service, "staff@example.com", `{
		"kind": "leave",
		"startDate": "2026-05-13"
	}`)
	secondResponse := createAttendanceAbsenceForTest(t, service, "staff@example.com", `{
		"kind": "leave",
		"startDate": "2026-05-13"
	}`)

	if len(firstResponse.Absences) != 1 {
		t.Fatalf("expected first create to return 1 absence, got %d", len(firstResponse.Absences))
	}
	if len(secondResponse.Absences) != 0 {
		t.Fatalf("expected duplicate create to return no new absences, got %d", len(secondResponse.Absences))
	}

	summaryResponse := readAttendanceSummaryForTest(t, service, "staff@example.com", "2026-05")
	if len(summaryResponse.Absences) != 1 {
		t.Fatalf("expected 1 active absence after duplicate create, got %d", len(summaryResponse.Absences))
	}
}

func TestAttendanceAbsenceCreateSkipsWeekends(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	response := createAttendanceAbsenceForTest(t, service, "staff@example.com", `{
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

	summaryResponse := readAttendanceSummaryForTest(t, service, "staff@example.com", "2026-05")
	if len(summaryResponse.Absences) != 2 {
		t.Fatalf("expected 2 summary absences, got %d", len(summaryResponse.Absences))
	}
}

func TestAttendanceAbsenceResponseKeepsLabelsLocaleNeutral(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	response := createAttendanceAbsenceForTest(t, service, "staff@example.com", `{
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

func TestAttendanceAbsenceBlocksNonAdminOtherEmail(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	request := httptest.NewRequest(http.MethodPost, "/attendance/api/absences", strings.NewReader(`{
		"email": "other@example.com",
		"kind": "leave",
		"startDate": "2026-05-13"
	}`))
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-Email", "staff@example.com")
	recorder := httptest.NewRecorder()

	service.handleAttendance(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestAttendanceAbsenceAllowsAdminOtherEmail(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	response := createAttendanceAbsenceForTest(t, service, "admin@example.com", `{
		"email": "other@example.com",
		"kind": "leave",
		"startDate": "2026-05-13"
	}`)

	if len(response.Absences) != 1 {
		t.Fatalf("expected 1 created absence, got %d", len(response.Absences))
	}
	if response.Absences[0].Email != "other@example.com" {
		t.Fatalf("expected admin-created absence for other email, got %q", response.Absences[0].Email)
	}
}

func TestAttendanceAbsenceLocalAdminUsesSystemCreatedBy(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	request := httptest.NewRequest(http.MethodPost, "/attendance/api/absences", strings.NewReader(`{
		"email": "other@example.com",
		"kind": "leave",
		"startDate": "2026-05-13"
	}`))
	request.RemoteAddr = "127.0.0.1:1234"
	recorder := httptest.NewRecorder()
	service.handleAttendance(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected create status 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response attendanceAbsenceTestResponse
	if errorValue := json.NewDecoder(recorder.Body).Decode(&response); errorValue != nil {
		t.Fatalf("decode create response: %v", errorValue)
	}
	if response.Absences[0].CreatedBy != attendanceAbsenceLocalAdminActor {
		t.Fatalf("expected local admin createdBy, got %q", response.Absences[0].CreatedBy)
	}
}

func TestAttendanceAbsenceCancelRequiresOwnerOrAdmin(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	response := createAttendanceAbsenceForTest(t, service, "admin@example.com", `{
		"email": "other@example.com",
		"kind": "leave",
		"startDate": "2026-05-13"
	}`)
	absenceID := response.Absences[0].ID

	staffRequest := httptest.NewRequest(http.MethodDelete, "/attendance/api/absences/"+absenceID, nil)
	staffRequest.RemoteAddr = "203.0.113.10:1234"
	staffRequest.Header.Set("X-Forwarded-Email", "staff@example.com")
	staffRecorder := httptest.NewRecorder()
	service.handleAttendance(staffRecorder, staffRequest)
	if staffRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected staff cancel status 403, got %d: %s", staffRecorder.Code, staffRecorder.Body.String())
	}

	adminRequest := httptest.NewRequest(http.MethodDelete, "/attendance/api/absences/"+absenceID, nil)
	adminRequest.RemoteAddr = "203.0.113.10:1234"
	adminRequest.Header.Set("X-Forwarded-Email", "admin@example.com")
	adminRecorder := httptest.NewRecorder()
	service.handleAttendance(adminRecorder, adminRequest)
	if adminRecorder.Code != http.StatusOK {
		t.Fatalf("expected admin cancel status 200, got %d: %s", adminRecorder.Code, adminRecorder.Body.String())
	}
}

func TestAttendanceAbsenceRejectsUnknownKind(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	request := httptest.NewRequest(http.MethodPost, "/attendance/api/absences", strings.NewReader(`{
		"kind": "unknown",
		"startDate": "2026-05-13"
	}`))
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-Email", "staff@example.com")
	recorder := httptest.NewRecorder()

	service.handleAttendance(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", recorder.Code, recorder.Body.String())
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
		request.Header.Set("X-Forwarded-Email", "staff@example.com")
		recorder := httptest.NewRecorder()

		service.handleAttendance(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400 for %q, got %d: %s", kind, recorder.Code, recorder.Body.String())
		}
	}
}

func TestAttendanceAbsenceSummaryHidesPrivateFieldsForOtherStaff(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	createAttendanceAbsenceForTest(t, service, "admin@example.com", `{
		"email": "other@example.com",
		"kind": "other",
		"startDate": "2026-05-13",
		"reason": "hospital"
	}`)

	staffSummary := readAttendanceSummaryForTest(t, service, "staff@example.com", "2026-05")
	if len(staffSummary.Absences) != 1 {
		t.Fatalf("expected 1 staff-visible absence, got %d", len(staffSummary.Absences))
	}
	if staffSummary.Absences[0].Kind != "other" {
		t.Fatalf("expected staff-visible other kind, got %q", staffSummary.Absences[0].Kind)
	}
	if staffSummary.Absences[0].LabelKey != "other" {
		t.Fatalf("expected staff-visible other label key, got %q", staffSummary.Absences[0].LabelKey)
	}
	if staffSummary.Absences[0].Reason != "" {
		t.Fatalf("expected sanitized reason, got %q", staffSummary.Absences[0].Reason)
	}
	if staffSummary.Absences[0].CreatedBy != "" {
		t.Fatalf("expected sanitized createdBy, got %q", staffSummary.Absences[0].CreatedBy)
	}

	ownerSummary := readAttendanceSummaryForTest(t, service, "other@example.com", "2026-05")
	if len(ownerSummary.Absences) != 1 {
		t.Fatalf("expected 1 owner-visible absence, got %d", len(ownerSummary.Absences))
	}
	if ownerSummary.Absences[0].Kind != "other" {
		t.Fatalf("expected owner-visible other kind, got %q", ownerSummary.Absences[0].Kind)
	}
	if ownerSummary.Absences[0].Reason != "hospital" {
		t.Fatalf("expected owner-visible reason, got %q", ownerSummary.Absences[0].Reason)
	}
	if ownerSummary.Absences[0].CreatedBy != "admin@example.com" {
		t.Fatalf("expected owner-visible createdBy, got %q", ownerSummary.Absences[0].CreatedBy)
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
	request.Header.Set("X-Forwarded-Email", "staff@example.com")
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
