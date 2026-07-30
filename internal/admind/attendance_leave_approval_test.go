package admind

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAttendanceLeaveApprovalInboxRequiresAdministrator(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	grantAttendanceLeaveForApprovalTest(t, service, 1000)
	createAttendanceLeaveForApprovalTest(t, service, "2027-05-03")

	staffRecorder := requestAttendanceLeaveApprovalInboxForTest(
		t,
		service,
		"staff@example.com",
	)
	assertAttendanceLeaveErrorResponse(
		t,
		staffRecorder,
		http.StatusForbidden,
		attendanceLeaveErrorAccessDenied,
	)

	adminRecorder := requestAttendanceLeaveApprovalInboxForTest(
		t,
		service,
		"admin@example.com",
	)
	if adminRecorder.Code != http.StatusOK {
		t.Fatalf("admin inbox status = %d body = %s", adminRecorder.Code, adminRecorder.Body.String())
	}
	var inbox attendanceLeaveApprovalInbox
	if errorValue := json.NewDecoder(adminRecorder.Body).Decode(&inbox); errorValue != nil {
		t.Fatal(errorValue)
	}
	if inbox.PendingCount != 1 || len(inbox.Pending) != 1 {
		t.Fatalf("inbox = %+v", inbox)
	}
	if inbox.Pending[0].EmployeeEmail != "staff@example.com" {
		t.Fatalf("pending request = %+v", inbox.Pending[0])
	}
}

func TestAttendanceLeaveApprovalUsesReservationAndCreatesAbsence(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	grantAttendanceLeaveForApprovalTest(t, service, 1000)
	requestID := createAttendanceLeaveForApprovalTest(t, service, "2027-05-03")

	recorder := decideAttendanceLeaveForTest(
		t,
		service,
		requestID,
		`{"action":"approve","response":""}`,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("approve status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	balance, errorValue := service.readAttendanceLeaveBalance(
		t.Context(),
		attendanceLeaveEmployee{Email: "staff@example.com"},
		"annual",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.AvailableMilliDays != 500 ||
		balance.ReservedMilliDays != 0 ||
		balance.UsedMilliDays != 500 {
		t.Fatalf("balance = %+v", balance)
	}
	absences, errorValue := service.readAttendanceAbsences(
		t.Context(),
		"2027-05",
		"staff@example.com",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(absences) != 1 || absences[0].Kind != "leave" {
		t.Fatalf("absences = %+v", absences)
	}
	if absences[0].CreatedBy != "staff@example.com" {
		t.Fatalf("absence createdBy = %q", absences[0].CreatedBy)
	}
	if strings.Contains(recorder.Body.String(), "admin@example.com") {
		t.Fatalf("approval response exposed processor: %s", recorder.Body.String())
	}
}

func TestAttendanceLeaveApprovalRecordsUntrackedUseForNonDeductingLeave(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	createRecorder := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests",
		"staff@example.com",
		`{"leaveTypeID":"sick","unit":"fullDay","startDate":"2027-05-03","reason":""}`,
	)
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create status = %d body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	requestID := decodeAttendanceLeaveRequestIDForTest(t, createRecorder)
	approvalRecorder := decideAttendanceLeaveForTest(
		t,
		service,
		requestID,
		`{"action":"approve","response":""}`,
	)
	if approvalRecorder.Code != http.StatusOK {
		t.Fatalf("approve status = %d body = %s", approvalRecorder.Code, approvalRecorder.Body.String())
	}
	entries, errorValue := service.readAttendanceLeaveLedger(
		t.Context(),
		attendanceLeaveEmployee{Email: "staff@example.com"},
		"sick",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(entries) != 1 ||
		entries[0].Kind != attendanceLeaveOperationUntrackedUse ||
		entries[0].UsedDeltaMilliDays != 1000 {
		t.Fatalf("untracked ledger entries = %+v", entries)
	}
}

func TestAttendanceLeaveApprovalIncludesPartialLeaveTimesInAbsenceSummary(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	grantAttendanceLeaveForApprovalTest(t, service, 1000)
	createRecorder := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests",
		"staff@example.com",
		`{"leaveTypeID":"annual","unit":"quarterDay","startDate":"2027-05-03","partialPeriod":"custom","startTime":"13:00","reason":"은행 방문"}`,
	)
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create status = %d body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	requestID := decodeAttendanceLeaveRequestIDForTest(t, createRecorder)
	approvalRecorder := decideAttendanceLeaveForTest(
		t,
		service,
		requestID,
		`{"action":"approve","response":""}`,
	)
	if approvalRecorder.Code != http.StatusOK {
		t.Fatalf("approve status = %d body = %s", approvalRecorder.Code, approvalRecorder.Body.String())
	}
	absences, errorValue := service.readAttendanceAbsences(
		t.Context(),
		"2027-05",
		"staff@example.com",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(absences) != 1 {
		t.Fatalf("absences = %+v", absences)
	}
	if absences[0].StartTime != "13:00" || absences[0].EndTime != "15:00" {
		t.Fatalf("partial leave time = %s-%s", absences[0].StartTime, absences[0].EndTime)
	}
}

func TestAttendanceLeaveApprovalNeedsChangesRequiresResponse(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	grantAttendanceLeaveForApprovalTest(t, service, 1000)
	requestID := createAttendanceLeaveForApprovalTest(t, service, "2027-05-03")

	missingResponseRecorder := decideAttendanceLeaveForTest(
		t,
		service,
		requestID,
		`{"action":"needsChanges","response":""}`,
	)
	assertAttendanceLeaveErrorResponse(
		t,
		missingResponseRecorder,
		http.StatusBadRequest,
		attendanceLeaveErrorInvalidInput,
	)
	changeRecorder := decideAttendanceLeaveForTest(
		t,
		service,
		requestID,
		`{"action":"needsChanges","response":"날짜를 확인해 주세요."}`,
	)
	if changeRecorder.Code != http.StatusOK {
		t.Fatalf("needs changes status = %d body = %s", changeRecorder.Code, changeRecorder.Body.String())
	}
	balance, errorValue := service.readAttendanceLeaveBalance(
		t.Context(),
		attendanceLeaveEmployee{Email: "staff@example.com"},
		"annual",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.AvailableMilliDays != 500 ||
		balance.ReservedMilliDays != 500 ||
		balance.UsedMilliDays != 0 {
		t.Fatalf("balance = %+v", balance)
	}
	absences, errorValue := service.readAttendanceAbsences(
		t.Context(),
		"2027-05",
		"staff@example.com",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(absences) != 0 {
		t.Fatalf("absences = %+v", absences)
	}
}

func TestAttendanceLeaveApprovalRejectsWithoutResponseAndReleasesReservation(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	grantAttendanceLeaveForApprovalTest(t, service, 1000)
	requestID := createAttendanceLeaveForApprovalTest(t, service, "2027-05-03")

	recorder := decideAttendanceLeaveForTest(
		t,
		service,
		requestID,
		`{"action":"reject","response":""}`,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("reject status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	balance, errorValue := service.readAttendanceLeaveBalance(
		t.Context(),
		attendanceLeaveEmployee{Email: "staff@example.com"},
		"annual",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.AvailableMilliDays != 1000 ||
		balance.ReservedMilliDays != 0 ||
		balance.UsedMilliDays != 0 {
		t.Fatalf("balance = %+v", balance)
	}
}

func TestAttendanceLeaveApprovalOnlyFirstConcurrentDecisionApplies(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	grantAttendanceLeaveForApprovalTest(t, service, 1000)
	requestID := createAttendanceLeaveForApprovalTest(t, service, "2027-05-03")
	now := time.Date(2026, 7, 28, 4, 0, 0, 0, time.UTC)

	start := make(chan struct{})
	results := make(chan error, 2)
	var waitGroup sync.WaitGroup
	for _, action := range []string{
		attendanceLeaveApprovalActionApprove,
		attendanceLeaveApprovalActionReject,
	} {
		waitGroup.Add(1)
		go func(actionValue string) {
			defer waitGroup.Done()
			<-start
			_, errorValue := service.decideAttendanceLeaveRequest(
				t.Context(),
				requestID,
				"admin@example.com",
				attendanceLeaveApprovalInput{Action: actionValue},
				now,
			)
			results <- errorValue
		}(action)
	}
	close(start)
	waitGroup.Wait()
	close(results)

	successCount := 0
	conflictCount := 0
	for errorValue := range results {
		switch {
		case errorValue == nil:
			successCount++
		case errors.Is(errorValue, errAttendanceLeaveApprovalConflict):
			conflictCount++
		default:
			t.Fatalf("unexpected decision error = %v", errorValue)
		}
	}
	if successCount != 1 || conflictCount != 1 {
		t.Fatalf("success = %d conflict = %d", successCount, conflictCount)
	}
	balance, errorValue := service.readAttendanceLeaveBalance(
		t.Context(),
		attendanceLeaveEmployee{Email: "staff@example.com"},
		"annual",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.ReservedMilliDays != 0 {
		t.Fatalf("balance = %+v", balance)
	}
}

func TestAttendanceLeaveApprovalRecentChangesIncludesApprovedCancellation(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	grantAttendanceLeaveForApprovalTest(t, service, 1000)
	requestID := createAttendanceLeaveForApprovalTest(t, service, "2027-05-03")
	approveRecorder := decideAttendanceLeaveForTest(
		t,
		service,
		requestID,
		`{"action":"approve"}`,
	)
	if approveRecorder.Code != http.StatusOK {
		t.Fatalf("approve status = %d body = %s", approveRecorder.Code, approveRecorder.Body.String())
	}

	cancelRequest := httptest.NewRequest(
		http.MethodPost,
		"/attendance/api/leave-requests/"+requestID+"/cancel",
		nil,
	)
	cancelRequest.RemoteAddr = "203.0.113.10:1234"
	cancelRequest.Header.Set("X-Forwarded-Email", "staff@example.com")
	cancelRecorder := httptest.NewRecorder()
	service.handleAttendance(cancelRecorder, cancelRequest)
	if cancelRecorder.Code != http.StatusOK {
		t.Fatalf("cancel status = %d body = %s", cancelRecorder.Code, cancelRecorder.Body.String())
	}

	inboxRecorder := requestAttendanceLeaveApprovalInboxForTest(
		t,
		service,
		"admin@example.com",
	)
	if inboxRecorder.Code != http.StatusOK {
		t.Fatalf("inbox status = %d body = %s", inboxRecorder.Code, inboxRecorder.Body.String())
	}
	var inbox attendanceLeaveApprovalInbox
	if errorValue := json.NewDecoder(inboxRecorder.Body).Decode(&inbox); errorValue != nil {
		t.Fatal(errorValue)
	}
	foundCancellation := false
	for _, change := range inbox.RecentChanges {
		if change.Request.ID == requestID &&
			change.Change == attendanceLeaveRequestStatusCancelled {
			foundCancellation = true
		}
	}
	if !foundCancellation {
		t.Fatalf("recent changes = %+v", inbox.RecentChanges)
	}
}

func TestAttendanceLeaveApprovalRecentChangesIncludesEarlyReturn(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	grantAttendanceLeaveForApprovalTest(t, service, 1000)
	requestID := createAttendanceLeaveForApprovalTest(t, service, "2027-05-03")
	approveRecorder := decideAttendanceLeaveForTest(
		t,
		service,
		requestID,
		`{"action":"approve"}`,
	)
	if approveRecorder.Code != http.StatusOK {
		t.Fatalf("approve status = %d body = %s", approveRecorder.Code, approveRecorder.Body.String())
	}

	returnedAt := "2027-05-03T05:30:00Z"
	database, errorValue := service.openAttendanceDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.ExecContext(t.Context(), `
INSERT INTO attendance_leave_request_events (
	id, request_id, kind, actor_email, response, created_at
) VALUES (?, ?, ?, ?, ?, ?)`,
		"leave-event-early-return",
		requestID,
		attendanceLeaveApprovalChangeEarlyReturn,
		"staff@example.com",
		returnedAt,
		returnedAt,
	); errorValue != nil {
		t.Fatal(errorValue)
	}

	inboxRecorder := requestAttendanceLeaveApprovalInboxForTest(
		t,
		service,
		"admin@example.com",
	)
	if inboxRecorder.Code != http.StatusOK {
		t.Fatalf("inbox status = %d body = %s", inboxRecorder.Code, inboxRecorder.Body.String())
	}
	var inbox attendanceLeaveApprovalInbox
	if errorValue := json.NewDecoder(inboxRecorder.Body).Decode(&inbox); errorValue != nil {
		t.Fatal(errorValue)
	}
	foundEarlyReturn := false
	for _, change := range inbox.RecentChanges {
		if change.Request.ID == requestID &&
			change.Change == attendanceLeaveApprovalChangeEarlyReturn &&
			change.ReturnedAt == returnedAt &&
			change.Response == "" {
			foundEarlyReturn = true
		}
	}
	if !foundEarlyReturn {
		t.Fatalf("recent changes = %+v", inbox.RecentChanges)
	}
}

func grantAttendanceLeaveForApprovalTest(
	t *testing.T,
	service *Service,
	amountMilliDays int,
) {
	t.Helper()
	if _, errorValue := service.grantAttendanceLeave(t.Context(), attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-leave-approval",
			Employee:     attendanceLeaveEmployee{Email: "staff@example.com"},
			LeaveTypeID:  "annual",
			Amount:       amountMilliDays,
			EffectiveOn:  "2027-01-01",
		},
		ExpiresOn: "2028-01-01",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func createAttendanceLeaveForApprovalTest(
	t *testing.T,
	service *Service,
	startDate string,
) string {
	t.Helper()
	recorder := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests",
		"staff@example.com",
		`{"leaveTypeID":"annual","unit":"halfDay","startDate":"`+startDate+`","partialPeriod":"morning","reason":"병원 방문"}`,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("create status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	return decodeAttendanceLeaveRequestIDForTest(t, recorder)
}

func requestAttendanceLeaveApprovalInboxForTest(
	t *testing.T,
	service *Service,
	actorEmail string,
) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/attendance/api/leave-approvals", nil)
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-Email", actorEmail)
	recorder := httptest.NewRecorder()
	service.handleAttendance(recorder, request)
	return recorder
}

func decideAttendanceLeaveForTest(
	t *testing.T,
	service *Service,
	requestID string,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(
		http.MethodPost,
		"/attendance/api/leave-approvals/"+requestID,
		strings.NewReader(body),
	)
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-Email", "admin@example.com")
	recorder := httptest.NewRecorder()
	service.handleAttendance(recorder, request)
	return recorder
}
