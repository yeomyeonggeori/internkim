package admind

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAttendanceAbsenceBlocksNonAdminOtherEmail(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	request := httptest.NewRequest(http.MethodPost, "/attendance/api/absences", strings.NewReader(`{
		"email": "other@example.com",
		"kind": "leave",
		"startDate": "2026-05-13"
	}`))
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-Email", "member@example.com")
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

func TestAttendanceAbsenceLocalRequestDoesNotGrantAdministratorAccess(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	request := httptest.NewRequest(http.MethodPost, "/attendance/api/absences", strings.NewReader(`{
		"email": "other@example.com",
		"kind": "leave",
		"startDate": "2026-05-13"
	}`))
	request.RemoteAddr = "127.0.0.1:1234"
	recorder := httptest.NewRecorder()
	service.handleAttendance(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected create status 403, got %d: %s", recorder.Code, recorder.Body.String())
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

	memberRequest := httptest.NewRequest(http.MethodDelete, "/attendance/api/absences/"+absenceID, nil)
	memberRequest.RemoteAddr = "203.0.113.10:1234"
	memberRequest.Header.Set("X-Forwarded-Email", "member@example.com")
	memberRecorder := httptest.NewRecorder()
	service.handleAttendance(memberRecorder, memberRequest)
	if memberRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected member cancel status 403, got %d: %s", memberRecorder.Code, memberRecorder.Body.String())
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

func TestAttendanceAbsenceSummaryHidesPrivateFieldsForOtherMember(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	createAttendanceAbsenceForTest(t, service, "admin@example.com", `{
		"email": "other@example.com",
		"kind": "other",
		"startDate": "2026-05-13",
		"reason": "hospital"
	}`)

	memberSummary := readAttendanceSummaryForTest(t, service, "member@example.com", "2026-05")
	if len(memberSummary.Absences) != 1 {
		t.Fatalf("expected 1 member-visible absence, got %d", len(memberSummary.Absences))
	}
	if memberSummary.Absences[0].Kind != "other" {
		t.Fatalf("expected member-visible other kind, got %q", memberSummary.Absences[0].Kind)
	}
	if memberSummary.Absences[0].LabelKey != "other" {
		t.Fatalf("expected member-visible other label key, got %q", memberSummary.Absences[0].LabelKey)
	}
	if memberSummary.Absences[0].Reason != "" {
		t.Fatalf("expected sanitized reason, got %q", memberSummary.Absences[0].Reason)
	}
	if memberSummary.Absences[0].CreatedBy != "" {
		t.Fatalf("expected sanitized createdBy, got %q", memberSummary.Absences[0].CreatedBy)
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
