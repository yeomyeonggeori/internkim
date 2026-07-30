package admind

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAttendanceLeaveManagementRequiresAdministrator(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	request := httptest.NewRequest(http.MethodGet, "/attendance/api/leave-management", nil)
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-Email", "staff@example.com")
	recorder := httptest.NewRecorder()

	service.handleAttendance(recorder, request)

	assertAttendanceLeaveErrorResponse(
		t,
		recorder,
		http.StatusForbidden,
		attendanceLeaveErrorAccessDenied,
	)
}

func TestAttendanceLeaveManagementAdjustsBalanceAndKeepsReason(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	recorder := performAttendanceLeaveManagementRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-management/adjustments",
		`{
			"employeeEmail":"staff@example.com",
			"leaveTypeID":"annual",
			"amountMilliDays":1500,
			"kind":"adjustment",
			"reason":"관리자 추가 부여",
			"effectiveOn":"2026-07-28",
			"expiresOn":"2026-12-31"
		}`,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("adjust status = %d body = %s", recorder.Code, recorder.Body.String())
	}

	detailRecorder := performAttendanceLeaveManagementRequest(
		t,
		service,
		http.MethodGet,
		"/attendance/api/leave-management?email=staff%40example.com",
		"",
	)
	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("detail status = %d body = %s", detailRecorder.Code, detailRecorder.Body.String())
	}
	var response attendanceLeaveManagementResponse
	if errorValue := json.NewDecoder(detailRecorder.Body).Decode(&response); errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Detail == nil {
		t.Fatal("missing employee detail")
	}
	if response.Detail.Employee.AvailableMilliDays != 1500 ||
		response.Detail.Employee.ExpiringMilliDays != 1500 {
		t.Fatalf("employee = %+v", response.Detail.Employee)
	}
	if len(response.Detail.LedgerEntries) != 1 {
		t.Fatalf("ledger entries = %+v", response.Detail.LedgerEntries)
	}
	entry := response.Detail.LedgerEntries[0]
	if entry.OperationType != attendanceLeaveOperationAdjustment ||
		entry.Reason != "관리자 추가 부여" {
		t.Fatalf("ledger entry = %+v", entry)
	}
}

func TestAttendanceLeaveManagementRejectsLegalCorrection(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	recorder := performAttendanceLeaveManagementRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-management/adjustments",
		`{
			"employeeEmail":"staff@example.com",
			"leaveTypeID":"annual",
			"amountMilliDays":1000,
			"kind":"legalCorrection",
			"reason":"",
			"effectiveOn":"2026-07-28",
			"expiresOn":""
		}`,
	)
	assertAttendanceLeaveErrorResponse(
		t,
		recorder,
		http.StatusBadRequest,
		attendanceLeaveErrorInvalidInput,
	)
}

func TestAttendanceLeaveManagementAdjustmentAllowsEmptyReason(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	recorder := performAttendanceLeaveManagementRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-management/adjustments",
		`{
			"employeeEmail":"staff@example.com",
			"leaveTypeID":"annual",
			"amountMilliDays":1000,
			"kind":"adjustment",
			"reason":"",
			"effectiveOn":"2026-07-28",
			"expiresOn":""
		}`,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("adjust status = %d body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestAttendanceLeaveManagementCreatesPastApprovedLeave(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	_, errorValue := service.adjustManagedAttendanceLeave(
		t.Context(),
		attendanceLeaveManagementAdjustmentInput{
			EmployeeEmail:   "staff@example.com",
			LeaveTypeID:     "annual",
			AmountMilliDays: 2000,
			Kind:            attendanceLeaveOperationAdjustment,
			Reason:          "테스트 부여",
			EffectiveOn:     "2026-01-01",
		},
		time.Date(2026, 7, 28, 9, 0, 0, 0, time.UTC),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	view, errorValue := service.createManagedPastAttendanceLeave(
		t.Context(),
		attendanceLeaveManagementPastLeaveInput{
			EmployeeEmail: "staff@example.com",
			LeaveTypeID:   "annual",
			Unit:          "fullDay",
			StartDate:     "2026-07-27",
			EndDate:       "2026-07-27",
			Reason:        "",
		},
		"admin@example.com",
		time.Date(2026, 7, 28, 9, 0, 0, 0, time.UTC),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if view.Status != attendanceLeaveRequestStatusApproved ||
		view.EmployeeEmail != "staff@example.com" {
		t.Fatalf("view = %+v", view)
	}
	absences, errorValue := service.readAttendanceAbsences(
		t.Context(),
		"2026-07",
		"staff@example.com",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(absences) != 1 || absences[0].Date != "2026-07-27" {
		t.Fatalf("absences = %+v", absences)
	}
}

func TestAttendanceLeaveManagementRejectsPastRangeEndingInFuture(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	_, errorValue := service.createManagedPastAttendanceLeave(
		t.Context(),
		attendanceLeaveManagementPastLeaveInput{
			EmployeeEmail: "staff@example.com",
			LeaveTypeID:   "sick",
			Unit:          "fullDay",
			StartDate:     "2026-07-27",
			EndDate:       "2026-07-29",
		},
		"admin@example.com",
		time.Date(2026, 7, 28, 9, 0, 0, 0, time.UTC),
	)
	if !errors.Is(errorValue, errAttendanceLeaveInvalidInput) {
		t.Fatalf("future end date error = %v", errorValue)
	}
}

func TestAttendanceLeaveManagementCancelsPastApprovedLeave(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	_, errorValue := service.adjustManagedAttendanceLeave(
		t.Context(),
		attendanceLeaveManagementAdjustmentInput{
			EmployeeEmail:   "staff@example.com",
			LeaveTypeID:     "annual",
			AmountMilliDays: 2000,
			Kind:            attendanceLeaveOperationAdjustment,
			Reason:          "테스트 부여",
			EffectiveOn:     "2026-01-01",
		},
		time.Date(2026, 7, 28, 9, 0, 0, 0, time.UTC),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	view, errorValue := service.createManagedPastAttendanceLeave(
		t.Context(),
		attendanceLeaveManagementPastLeaveInput{
			EmployeeEmail: "staff@example.com",
			LeaveTypeID:   "annual",
			Unit:          "fullDay",
			StartDate:     "2026-07-27",
			EndDate:       "2026-07-27",
			Reason:        "취소할 과거 휴가",
		},
		"admin@example.com",
		time.Date(2026, 7, 28, 9, 0, 0, 0, time.UTC),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	recorder := performAttendanceLeaveManagementRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-management/requests/"+view.ID+"/cancel",
		`{"employeeEmail":"staff@example.com"}`,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("cancel status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	absences, errorValue := service.readAttendanceAbsences(
		t.Context(),
		"2026-07",
		"staff@example.com",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(absences) != 0 {
		t.Fatalf("absences = %+v", absences)
	}
	detailRecorder := performAttendanceLeaveManagementRequest(
		t,
		service,
		http.MethodGet,
		"/attendance/api/leave-management?email=staff%40example.com",
		"",
	)
	var response attendanceLeaveManagementResponse
	if errorValue := json.NewDecoder(detailRecorder.Body).Decode(&response); errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Detail == nil ||
		response.Detail.Employee.UsedMilliDays != 0 ||
		response.Detail.Employee.AvailableMilliDays != 2000 {
		t.Fatalf("detail = %+v", response.Detail)
	}
	if len(response.Detail.Requests) != 1 ||
		response.Detail.Requests[0].Status != attendanceLeaveRequestStatusCancelled {
		t.Fatalf("requests = %+v", response.Detail.Requests)
	}
}

func TestAttendanceLeaveManagementCorrectsPastPartialLeaveTime(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	now := time.Date(2026, 7, 28, 9, 0, 0, 0, time.UTC)
	_, errorValue := service.adjustManagedAttendanceLeave(
		t.Context(),
		attendanceLeaveManagementAdjustmentInput{
			EmployeeEmail:   "staff@example.com",
			LeaveTypeID:     "annual",
			AmountMilliDays: 2000,
			Kind:            attendanceLeaveOperationAdjustment,
			Reason:          "테스트 부여",
			EffectiveOn:     "2026-01-01",
		},
		now,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	view, errorValue := service.createManagedPastAttendanceLeave(
		t.Context(),
		attendanceLeaveManagementPastLeaveInput{
			EmployeeEmail: "staff@example.com",
			LeaveTypeID:   "annual",
			Unit:          "halfDay",
			StartDate:     "2026-07-27",
			EndDate:       "2026-07-27",
			PartialPeriod: attendanceLeavePartialPeriodCustom,
			StartTime:     "14:00",
			Reason:        "시간을 정정할 과거 휴가",
		},
		"admin@example.com",
		now,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	errorValue = service.correctManagedAttendanceLeaveTime(
		t.Context(),
		view.ID,
		attendanceLeaveManagementTimeInput{
			EmployeeEmail: "staff@example.com",
			StartTime:     "12:00",
			EndTime:       "16:00",
			Reason:        "실제 사용 시간 반영",
		},
		"admin@example.com",
		now,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	absences, errorValue := service.readAttendanceAbsences(
		t.Context(),
		"2026-07",
		"staff@example.com",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(absences) != 1 ||
		absences[0].StartTime != "12:00" ||
		absences[0].EndTime != "16:00" {
		t.Fatalf("absences = %+v", absences)
	}
	detailRecorder := performAttendanceLeaveManagementRequest(
		t,
		service,
		http.MethodGet,
		"/attendance/api/leave-management?email=staff%40example.com",
		"",
	)
	var response attendanceLeaveManagementResponse
	if errorValue := json.NewDecoder(detailRecorder.Body).Decode(&response); errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Detail == nil ||
		response.Detail.Employee.AvailableMilliDays != 1500 ||
		len(response.Detail.Requests) != 1 ||
		response.Detail.Requests[0].StartTime != "12:00" ||
		response.Detail.Requests[0].EndTime != "16:00" {
		t.Fatalf("detail = %+v", response.Detail)
	}

	database, errorValue := service.openAttendanceDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	location := service.workspaceTimeZone().location
	userRecord := mattermostUserRecord{
		ID:       "user-1",
		Username: "staff",
		Email:    "staff@example.com",
		Nickname: "Staff",
	}
	for _, event := range []attendanceEvent{
		service.createAttendanceEvent(
			userRecord,
			attendanceKindClockIn,
			time.Date(2026, 7, 27, 10, 0, 0, 0, location),
			"team-1",
			"attendance-channel",
			"entry-post",
			"clock-in-post",
			service.attendanceLocationByID("office"),
		),
		service.createAttendanceEvent(
			userRecord,
			attendanceKindClockOut,
			time.Date(2026, 7, 27, 13, 0, 0, 0, location),
			"team-1",
			"attendance-channel",
			"entry-post",
			"clock-out-post",
			service.attendanceLocationByID("office"),
		),
	} {
		if errorValue := service.insertAttendanceEvent(t.Context(), database, event); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	database.Close()
	errorValue = service.correctManagedAttendanceLeaveTime(
		t.Context(),
		view.ID,
		attendanceLeaveManagementTimeInput{
			EmployeeEmail: "staff@example.com",
			StartTime:     "11:00",
			EndTime:       "15:00",
			Reason:        "근무 기록과 겹치는 정정",
		},
		"admin@example.com",
		now,
	)
	if !errors.Is(errorValue, errAttendanceLeaveWorkConflict) {
		t.Fatalf("correction error = %v, want %v", errorValue, errAttendanceLeaveWorkConflict)
	}
}

func performAttendanceLeaveManagementRequest(
	t *testing.T,
	service *Service,
	method string,
	target string,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-Email", "admin@example.com")
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	service.handleAttendance(recorder, request)
	return recorder
}
