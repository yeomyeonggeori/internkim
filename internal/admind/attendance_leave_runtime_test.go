package admind

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestApprovedPartialLeaveClosesOpenWorkAtLeaveStart(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	requestID := createApprovedPartialLeaveForRuntimeTest(t, service)
	insertRuntimeClockInForTest(t, service)
	now := time.Date(2027, 5, 3, 5, 0, 0, 0, time.UTC)

	activeLeave, errorValue := service.reconcileApprovedLeaveClockOut(
		t.Context(),
		"staff@example.com",
		now,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if activeLeave == nil || activeLeave.RequestID != requestID || activeLeave.LeaveTypeID != "annual" {
		t.Fatalf("active leave = %+v", activeLeave)
	}
	events, errorValue := service.readAttendanceEvents(
		t.Context(),
		"2027-05",
		"staff@example.com",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != 2 {
		t.Fatalf("events = %+v", events)
	}
	if events[0].Kind != attendanceKindClockOut ||
		events[0].LocalTime != "13:00:00" ||
		events[0].Source != attendanceSourceApprovedLeave {
		t.Fatalf("automatic clock out = %+v", events[0])
	}

	if _, errorValue := service.reconcileApprovedLeaveClockOut(
		t.Context(),
		"staff@example.com",
		now,
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	events, errorValue = service.readAttendanceEvents(
		t.Context(),
		"2027-05",
		"staff@example.com",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != 2 {
		t.Fatalf("duplicate reconciliation events = %+v", events)
	}
}

func TestApprovedPartialLeaveEarlyReturnShortensDisplayedIntervalWithoutRestoringBalance(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	requestID := createApprovedPartialLeaveForRuntimeTest(t, service)
	insertRuntimeClockInForTest(t, service)
	returnedAt := time.Date(2027, 5, 3, 5, 30, 0, 0, time.UTC)
	activeLeave, errorValue := service.reconcileApprovedLeaveClockOut(
		t.Context(),
		"staff@example.com",
		returnedAt,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if activeLeave == nil {
		t.Fatal("missing active leave")
	}

	if errorValue := service.recordAttendanceLeaveEarlyReturn(
		t.Context(),
		*activeLeave,
		"staff@example.com",
		returnedAt,
	); errorValue != nil {
		t.Fatal(errorValue)
	}

	if currentLeave, errorValue := service.readActiveAttendanceLeave(
		t.Context(),
		"staff@example.com",
		returnedAt,
	); errorValue != nil || currentLeave != nil {
		t.Fatalf("current leave = %+v error = %v", currentLeave, errorValue)
	}
	dashboard, errorValue := service.readAttendanceLeaveDashboard(
		t.Context(),
		attendanceLeaveEmployee{Email: "staff@example.com"},
		returnedAt,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if dashboard.Summary.UsedMilliDays != 250 ||
		dashboard.Summary.AvailableMilliDays != 750 {
		t.Fatalf("dashboard summary = %+v", dashboard.Summary)
	}
	var approved attendanceLeaveRequestView
	for _, request := range dashboard.Requests {
		if request.ID == requestID {
			approved = request
			break
		}
	}
	if approved.EndTime != "14:30" || approved.DeductionMilliDays != 250 {
		t.Fatalf("approved request = %+v", approved)
	}
	inbox, errorValue := service.readAttendanceLeaveApprovalInbox(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	foundEarlyReturn := false
	for _, change := range inbox.RecentChanges {
		if change.Request.ID == requestID &&
			change.Change == attendanceLeaveApprovalChangeEarlyReturn &&
			change.ReturnedAt == returnedAt.Format(time.RFC3339Nano) {
			foundEarlyReturn = true
		}
	}
	if !foundEarlyReturn {
		t.Fatalf("recent changes = %+v", inbox.RecentChanges)
	}
}

func TestManagedTimeCorrectionMovesAutomaticClockOutAndKeepsCancellationRestorable(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	requestID := createApprovedPartialLeaveForRuntimeTest(t, service)
	insertRuntimeClockInForTest(t, service)
	if _, errorValue := service.reconcileApprovedLeaveClockOut(
		t.Context(),
		"staff@example.com",
		time.Date(2027, 5, 3, 5, 0, 0, 0, time.UTC),
	); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := service.correctManagedAttendanceLeaveTime(
		t.Context(),
		requestID,
		attendanceLeaveManagementTimeInput{
			EmployeeEmail: "staff@example.com",
			StartTime:     "14:00",
			EndTime:       "16:00",
			Reason:        "실제 휴가 시작 시간 반영",
		},
		"admin@example.com",
		time.Date(2027, 5, 4, 0, 0, 0, 0, time.UTC),
	); errorValue != nil {
		t.Fatal(errorValue)
	}

	event := readRuntimeAutomaticClockOutForTest(t, service, requestID)
	if event.LocalTime != "14:00:00" {
		t.Fatalf("automatic clock out = %+v", event)
	}
	correctedDashboard, errorValue := service.readAttendanceLeaveDashboard(
		t.Context(),
		attendanceLeaveEmployee{Email: "staff@example.com"},
		time.Date(2027, 5, 4, 0, 0, 0, 0, time.UTC),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if correctedDashboard.Summary.UsedMilliDays != 250 ||
		correctedDashboard.Summary.AvailableMilliDays != 750 {
		t.Fatalf("corrected dashboard summary = %+v", correctedDashboard.Summary)
	}
	if errorValue := service.cancelManagedAttendanceLeave(
		t.Context(),
		requestID,
		"staff@example.com",
		"admin@example.com",
		time.Date(2027, 5, 4, 1, 0, 0, 0, time.UTC),
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	dashboard, errorValue := service.readAttendanceLeaveDashboard(
		t.Context(),
		attendanceLeaveEmployee{Email: "staff@example.com"},
		time.Date(2027, 5, 4, 1, 0, 0, 0, time.UTC),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if dashboard.Summary.UsedMilliDays != 0 ||
		dashboard.Summary.AvailableMilliDays != 1000 {
		t.Fatalf("dashboard summary = %+v", dashboard.Summary)
	}
}

func TestManagedTimeCorrectionMovesAutomaticClockOutEarlier(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	requestID := createApprovedPartialLeaveForRuntimeTest(t, service)
	insertRuntimeClockInForTest(t, service)
	if _, errorValue := service.reconcileApprovedLeaveClockOut(
		t.Context(),
		"staff@example.com",
		time.Date(2027, 5, 3, 5, 0, 0, 0, time.UTC),
	); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := service.correctManagedAttendanceLeaveTime(
		t.Context(),
		requestID,
		attendanceLeaveManagementTimeInput{
			EmployeeEmail: "staff@example.com",
			StartTime:     "12:00",
			EndTime:       "14:00",
			Reason:        "예정보다 일찍 시작한 휴가 반영",
		},
		"admin@example.com",
		time.Date(2027, 5, 4, 0, 0, 0, 0, time.UTC),
	); errorValue != nil {
		t.Fatal(errorValue)
	}

	event := readRuntimeAutomaticClockOutForTest(t, service, requestID)
	if event.LocalTime != "12:00:00" {
		t.Fatalf("automatic clock out = %+v", event)
	}
}

func TestManagedCancellationCancelsAutomaticClockOut(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	requestID := createApprovedPartialLeaveForRuntimeTest(t, service)
	insertRuntimeClockInForTest(t, service)
	if _, errorValue := service.reconcileApprovedLeaveClockOut(
		t.Context(),
		"staff@example.com",
		time.Date(2027, 5, 3, 5, 0, 0, 0, time.UTC),
	); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := service.cancelManagedAttendanceLeave(
		t.Context(),
		requestID,
		"staff@example.com",
		"admin@example.com",
		time.Date(2027, 5, 3, 6, 0, 0, 0, time.UTC),
	); errorValue != nil {
		t.Fatal(errorValue)
	}

	event := readRuntimeAutomaticClockOutForTest(t, service, requestID)
	if event.CanceledAt == "" {
		t.Fatalf("automatic clock out = %+v", event)
	}
	database, errorValue := service.openAttendanceDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	latestEvent, foundLatest, errorValue := service.latestActiveAttendanceEvent(
		t.Context(),
		database,
		"staff-user",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !foundLatest || latestEvent.Kind != attendanceKindClockIn {
		t.Fatalf("latest active event = %+v found = %t", latestEvent, foundLatest)
	}
}

func TestAttendanceClockRequestDuringLeaveRequiresEarlyReturnConfirmation(t *testing.T) {
	if errorValue := attendanceClockRequestDuringLeaveError(attendanceClockRequest{
		Kind: attendanceKindClockIn,
	}); errorValue != errAttendanceLeaveEarlyReturnConfirmationRequired {
		t.Fatalf("clock in error = %v", errorValue)
	}
	if errorValue := attendanceClockRequestDuringLeaveError(attendanceClockRequest{
		Kind: attendanceKindClockOut,
	}); errorValue != errAttendanceLeaveClockOutAlreadyApplied {
		t.Fatalf("clock out error = %v", errorValue)
	}
	if errorValue := attendanceClockRequestDuringLeaveError(attendanceClockRequest{
		Kind:               attendanceKindClockIn,
		ConfirmEarlyReturn: true,
	}); errorValue != nil {
		t.Fatalf("confirmed early return error = %v", errorValue)
	}
}

func createApprovedPartialLeaveForRuntimeTest(
	t *testing.T,
	service *Service,
) string {
	t.Helper()
	grantAttendanceLeaveForApprovalTest(t, service, 1000)
	createRecorder := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests",
		"staff@example.com",
		`{"leaveTypeID":"annual","unit":"quarterDay","startDate":"2027-05-03","partialPeriod":"custom","startTime":"13:00","reason":"은행 방문"}`,
	)
	requestID := decodeAttendanceLeaveRequestIDForTest(t, createRecorder)
	approvalRecorder := decideAttendanceLeaveForTest(
		t,
		service,
		requestID,
		`{"action":"approve"}`,
	)
	if approvalRecorder.Code != http.StatusOK {
		t.Fatalf("approve status = %d body = %s", approvalRecorder.Code, approvalRecorder.Body.String())
	}
	var response struct {
		Request attendanceLeaveApprovalRequestView `json:"request"`
	}
	if errorValue := json.NewDecoder(approvalRecorder.Body).Decode(&response); errorValue != nil {
		t.Fatal(errorValue)
	}
	return requestID
}

func insertRuntimeClockInForTest(t *testing.T, service *Service) {
	t.Helper()
	occurredAt := time.Date(2027, 5, 3, 0, 0, 0, 0, time.UTC)
	database, errorValue := service.openAttendanceDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	event := service.createAttendanceEvent(
		mattermostUserRecord{
			ID:       "staff-user",
			Username: "staff",
			Email:    "staff@example.com",
		},
		attendanceKindClockIn,
		occurredAt,
		"team-1",
		"channel-1",
		"action-post-1",
		"result-post-1",
		attendanceLocation{ID: "office", Name: "사무실"},
	)
	if errorValue := service.insertAttendanceEvent(t.Context(), database, event); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func readRuntimeAutomaticClockOutForTest(
	t *testing.T,
	service *Service,
	requestID string,
) attendanceEvent {
	t.Helper()
	database, errorValue := service.openAttendanceDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var occurrenceID string
	if errorValue := database.QueryRowContext(
		t.Context(),
		`SELECT id FROM attendance_leave_request_occurrences WHERE request_id = ?`,
		requestID,
	).Scan(&occurrenceID); errorValue != nil {
		t.Fatal(errorValue)
	}
	event, found, errorValue := scanOptionalAttendanceEvent(
		database.QueryRowContext(
			t.Context(),
			"SELECT "+attendanceEventSelectColumns+" FROM attendance_events WHERE id = ?",
			attendanceLeaveDeterministicID("leave-clock-out", occurrenceID, 0),
		),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found {
		t.Fatal("automatic clock out was not found")
	}
	return event
}
