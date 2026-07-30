package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

type attendanceLeaveRequestLedgerAllocationForTest struct {
	occurrenceDate string
	grantKey       string
	amount         int
}

func TestAttendanceLeaveRequestReservesEachOccurrenceFromDateEligibleLots(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	employee := attendanceLeaveEmployee{Email: "staff@example.com"}
	grants := []attendanceLeaveGrant{
		{
			Operation: attendanceLeaveOperation{
				OperationKey: "grant-request-early-expiry",
				Employee:     employee,
				LeaveTypeID:  "annual",
				Amount:       1000,
				EffectiveOn:  "2027-01-01",
			},
			ExpiresOn: "2027-05-03",
		},
		{
			Operation: attendanceLeaveOperation{
				OperationKey: "grant-request-later-date",
				Employee:     employee,
				LeaveTypeID:  "annual",
				Amount:       1000,
				EffectiveOn:  "2027-05-04",
			},
		},
	}
	for _, grant := range grants {
		if _, errorValue := service.grantAttendanceLeave(t.Context(), grant); errorValue != nil {
			t.Fatal(errorValue)
		}
	}

	recorder := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests",
		employee.Email,
		`{"leaveTypeID":"annual","unit":"fullDay","startDate":"2027-05-03","endDate":"2027-05-04","reason":"Family appointment"}`,
	)
	requestID := decodeAttendanceLeaveRequestIDForTest(t, recorder)
	reservationReference := attendanceLeaveRequestReservationReference(requestID, 1)
	reserveAllocations := readAttendanceLeaveRequestLedgerAllocationsForTest(
		t,
		service,
		"leave-request:"+requestID+":reserve:1",
	)
	expected := []attendanceLeaveRequestLedgerAllocationForTest{
		{occurrenceDate: "2027-05-03", grantKey: "grant-request-early-expiry", amount: 1000},
		{occurrenceDate: "2027-05-04", grantKey: "grant-request-later-date", amount: 1000},
	}
	assertAttendanceLeaveRequestLedgerAllocationsForTest(t, reserveAllocations, expected)
	balance, errorValue := service.readAttendanceLeaveBalance(t.Context(), employee, "annual")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.AvailableMilliDays != 0 || balance.ReservedMilliDays != 2000 {
		t.Fatalf("balance = %+v", balance)
	}
	if _, errorValue := service.useAttendanceLeaveReservation(t.Context(), attendanceLeaveOperation{
		OperationKey: "leave-request:" + requestID + ":use:1",
		Employee:     employee,
		LeaveTypeID:  "annual",
		ReferenceID:  reservationReference,
		EffectiveOn:  "2027-05-03",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	useAllocations := readAttendanceLeaveRequestLedgerAllocationsForTest(
		t,
		service,
		"leave-request:"+requestID+":use:1",
	)
	assertAttendanceLeaveRequestLedgerAllocationsForTest(t, useAllocations, expected)
	setAttendanceLeaveRequestStatusForTest(
		t,
		service,
		requestID,
		attendanceLeaveRequestStatusApproved,
	)
	if errorValue := service.cancelAttendanceLeaveRequest(
		t.Context(),
		requestID,
		employee,
		time.Date(2027, 5, 1, 12, 0, 0, 0, service.workspaceTimeZone().location),
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	restoreAllocations := readAttendanceLeaveRequestLedgerAllocationsForTest(
		t,
		service,
		"leave-request:"+requestID+":restore:1",
	)
	assertAttendanceLeaveRequestLedgerAllocationsForTest(t, restoreAllocations, expected)
	balance, errorValue = service.readAttendanceLeaveBalance(t.Context(), employee, "annual")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.AvailableMilliDays != 2000 ||
		balance.ReservedMilliDays != 0 ||
		balance.UsedMilliDays != 0 {
		t.Fatalf("restored balance = %+v", balance)
	}
}

func TestAttendanceLeaveRequestDatedReservationShortageRollsBackAtomically(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	employee := attendanceLeaveEmployee{Email: "staff@example.com"}
	grants := []attendanceLeaveGrant{
		{
			Operation: attendanceLeaveOperation{
				OperationKey: "grant-request-shortage-early",
				Employee:     employee,
				LeaveTypeID:  "annual",
				Amount:       1000,
				EffectiveOn:  "2027-01-01",
			},
			ExpiresOn: "2027-05-04",
		},
		{
			Operation: attendanceLeaveOperation{
				OperationKey: "grant-request-shortage-future",
				Employee:     employee,
				LeaveTypeID:  "annual",
				Amount:       1000,
				EffectiveOn:  "2027-05-05",
			},
		},
	}
	for _, grant := range grants {
		if _, errorValue := service.grantAttendanceLeave(t.Context(), grant); errorValue != nil {
			t.Fatal(errorValue)
		}
	}

	recorder := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests",
		employee.Email,
		`{"leaveTypeID":"annual","unit":"fullDay","startDate":"2027-05-03","endDate":"2027-05-04","reason":"Family appointment"}`,
	)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var response attendanceLeaveErrorResponse
	if errorValue := json.NewDecoder(recorder.Body).Decode(&response); errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Code != attendanceLeaveErrorInsufficientBalance {
		t.Fatalf("response = %+v", response)
	}
	if recorder.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("cache control = %q", recorder.Header().Get("Cache-Control"))
	}
	database, errorValue := service.openAttendanceDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var requestCount int
	if errorValue := database.QueryRowContext(
		t.Context(),
		`SELECT COUNT(*) FROM attendance_leave_requests`,
	).Scan(&requestCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if requestCount != 0 {
		t.Fatalf("request count = %d", requestCount)
	}
	balance, errorValue := service.readAttendanceLeaveBalance(t.Context(), employee, "annual")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.AvailableMilliDays != 2000 || balance.ReservedMilliDays != 0 {
		t.Fatalf("balance = %+v", balance)
	}
}

func readAttendanceLeaveRequestLedgerAllocationsForTest(
	t *testing.T,
	service *Service,
	operationKey string,
) []attendanceLeaveRequestLedgerAllocationForTest {
	t.Helper()
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(context.Background(), `
SELECT entry.effective_at, lot.source_operation_key, entry.amount_milli_days
FROM attendance_leave_ledger_entries entry
JOIN attendance_leave_grant_lots lot ON lot.id = entry.grant_lot_id
WHERE entry.operation_key = ?
ORDER BY entry.sequence`,
		operationKey,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer rows.Close()
	allocations := []attendanceLeaveRequestLedgerAllocationForTest{}
	for rows.Next() {
		var value attendanceLeaveRequestLedgerAllocationForTest
		if errorValue := rows.Scan(
			&value.occurrenceDate,
			&value.grantKey,
			&value.amount,
		); errorValue != nil {
			t.Fatal(errorValue)
		}
		allocations = append(allocations, value)
	}
	if errorValue := rows.Err(); errorValue != nil {
		t.Fatal(errorValue)
	}
	return allocations
}

func assertAttendanceLeaveRequestLedgerAllocationsForTest(
	t *testing.T,
	actual []attendanceLeaveRequestLedgerAllocationForTest,
	expected []attendanceLeaveRequestLedgerAllocationForTest,
) {
	t.Helper()
	if len(actual) != len(expected) {
		t.Fatalf("allocations = %+v", actual)
	}
	for index := range expected {
		if actual[index] != expected[index] {
			t.Fatalf("allocation %d = %+v", index, actual[index])
		}
	}
}
