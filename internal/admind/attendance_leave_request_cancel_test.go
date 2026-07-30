package admind

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAttendanceLeaveRequestCancelApprovedFutureRestoresUsedBalance(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	employee := attendanceLeaveEmployee{Email: "staff@example.com"}
	if _, errorValue := service.grantAttendanceLeave(t.Context(), attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-leave-request-approved-cancel",
			Employee:     employee,
			LeaveTypeID:  "annual",
			Amount:       1000,
			EffectiveOn:  "2027-01-01",
		},
		ExpiresOn: "2028-01-01",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	createRecorder := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests",
		"staff@example.com",
		`{"leaveTypeID":"annual","unit":"fullDay","startDate":"2027-05-03","reason":"Family appointment"}`,
	)
	requestID := decodeAttendanceLeaveRequestIDForTest(t, createRecorder)
	if _, errorValue := service.useAttendanceLeaveReservation(t.Context(), attendanceLeaveOperation{
		OperationKey: "leave-request:" + requestID + ":use:1",
		Employee:     employee,
		LeaveTypeID:  "annual",
		ReferenceID:  attendanceLeaveRequestReservationReference(requestID, 1),
		EffectiveOn:  "2027-05-03",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	setAttendanceLeaveRequestStatusForTest(
		t,
		service,
		requestID,
		attendanceLeaveRequestStatusApproved,
	)

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
	dashboard := readAttendanceLeaveDashboardForTest(t, service, "staff@example.com")
	if len(dashboard.Requests) != 1 ||
		dashboard.Requests[0].Status != attendanceLeaveRequestStatusCancelled ||
		dashboard.Requests[0].CanCancel {
		t.Fatalf("requests = %+v", dashboard.Requests)
	}
	if dashboard.Summary.AvailableMilliDays != 1000 ||
		dashboard.Summary.ReservedMilliDays != 0 ||
		dashboard.Summary.UsedMilliDays != 0 {
		t.Fatalf("summary = %+v", dashboard.Summary)
	}
}

func TestAttendanceLeaveRequestApprovedCancellationUsesLinkedAbsenceBoundary(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	createRecorder := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests",
		"staff@example.com",
		`{"leaveTypeID":"sick","unit":"fullDay","startDate":"2027-05-03","reason":"Medical appointment"}`,
	)
	requestID := decodeAttendanceLeaveRequestIDForTest(t, createRecorder)
	database, errorValue := service.openAttendanceLeaveMutationDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	transaction, errorValue := database.BeginTx(t.Context(), nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	createdAt := "2027-04-01T00:00:00Z"
	ranges, errorValue := insertAttendanceAbsenceRangesForDates(
		t.Context(),
		transaction,
		"staff@example.com",
		attendanceAbsenceLeave,
		"Medical appointment",
		"admin@example.com",
		createdAt,
		createdAt,
		[]string{"2027-05-03"},
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(ranges) != 1 {
		t.Fatalf("ranges = %+v", ranges)
	}
	if errorValue := linkAttendanceLeaveRequestAbsenceRange(
		t.Context(),
		transaction,
		requestID,
		ranges[0].ID,
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := transaction.ExecContext(
		t.Context(),
		`UPDATE attendance_leave_requests SET status = ? WHERE id = ?`,
		attendanceLeaveRequestStatusApproved,
		requestID,
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		t.Fatal(errorValue)
	}
	database.Close()

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
	database, errorValue = service.openAttendanceDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var activeOccurrenceCount int
	if errorValue := database.QueryRowContext(t.Context(), `
SELECT COUNT(*)
FROM attendance_absence_occurrences
WHERE range_id = ? AND canceled_at = ''`,
		ranges[0].ID,
	).Scan(&activeOccurrenceCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if activeOccurrenceCount != 0 {
		t.Fatalf("active linked absence occurrences = %d", activeOccurrenceCount)
	}
}

func TestAttendanceLeaveRequestCancelApprovedStartedIsRejected(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	createRecorder := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests",
		"staff@example.com",
		`{"leaveTypeID":"sick","unit":"fullDay","startDate":"2027-05-03","reason":"Medical appointment"}`,
	)
	requestID := decodeAttendanceLeaveRequestIDForTest(t, createRecorder)
	database, errorValue := service.openAttendanceDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(t.Context(), `
UPDATE attendance_leave_request_occurrences
SET date = '2020-01-02'
WHERE request_id = ?`,
		requestID,
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(t.Context(), `
UPDATE attendance_leave_requests
SET status = ?, start_date = '2020-01-02', end_date = '2020-01-02'
WHERE id = ?`,
		attendanceLeaveRequestStatusApproved,
		requestID,
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	database.Close()

	cancelRequest := httptest.NewRequest(
		http.MethodPost,
		"/attendance/api/leave-requests/"+requestID+"/cancel",
		nil,
	)
	cancelRequest.RemoteAddr = "203.0.113.10:1234"
	cancelRequest.Header.Set("X-Forwarded-Email", "staff@example.com")
	cancelRecorder := httptest.NewRecorder()
	service.handleAttendance(cancelRecorder, cancelRequest)
	assertAttendanceLeaveErrorResponse(
		t,
		cancelRecorder,
		http.StatusConflict,
		attendanceLeaveErrorInvalidStatus,
	)
	dashboard := readAttendanceLeaveDashboardForTest(t, service, "staff@example.com")
	if len(dashboard.Requests) != 1 ||
		dashboard.Requests[0].Status != attendanceLeaveRequestStatusApproved {
		t.Fatalf("requests = %+v", dashboard.Requests)
	}
}
