package admind

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAttendanceLeaveRequestCreateReservesBalanceAndReturnsPrivateDashboard(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	employee := attendanceLeaveEmployee{Email: "staff@example.com"}
	if _, errorValue := service.grantAttendanceLeave(t.Context(), attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-leave-request-create",
			Employee:     employee,
			LeaveTypeID:  "annual",
			Amount:       2000,
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
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create status = %d body = %s", createRecorder.Code, createRecorder.Body.String())
	}

	request := httptest.NewRequest(http.MethodGet, "/attendance/api/leave", nil)
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-Email", "staff@example.com")
	recorder := httptest.NewRecorder()
	service.handleAttendance(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("dashboard status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Summary struct {
			UsedMilliDays      int `json:"usedMilliDays"`
			ReservedMilliDays  int `json:"reservedMilliDays"`
			AvailableMilliDays int `json:"availableMilliDays"`
		} `json:"summary"`
		Requests []struct {
			ID                 string `json:"id"`
			LeaveTypeID        string `json:"leaveTypeID"`
			Status             string `json:"status"`
			DeductionMilliDays int    `json:"deductionMilliDays"`
			Reason             string `json:"reason"`
			CanCancel          bool   `json:"canCancel"`
			CanResubmit        bool   `json:"canResubmit"`
		} `json:"requests"`
	}
	if errorValue := json.NewDecoder(recorder.Body).Decode(&response); errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Summary.AvailableMilliDays != 1000 ||
		response.Summary.ReservedMilliDays != 1000 ||
		response.Summary.UsedMilliDays != 0 {
		t.Fatalf("summary = %+v", response.Summary)
	}
	if len(response.Requests) != 1 ||
		response.Requests[0].ID == "" ||
		response.Requests[0].LeaveTypeID != "annual" ||
		response.Requests[0].Status != attendanceLeaveRequestStatusPending ||
		response.Requests[0].DeductionMilliDays != 1000 ||
		response.Requests[0].Reason != "Family appointment" ||
		!response.Requests[0].CanCancel ||
		response.Requests[0].CanResubmit {
		t.Fatalf("requests = %+v", response.Requests)
	}
	if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "private, no-store" {
		t.Fatalf("cache control = %q", cacheControl)
	}
}

func TestAttendanceLeaveRequestAnnualBalanceModeUsesAnnualAccount(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	policy := defaultAttendanceLeavePolicy()
	policy.LeaveTypes = append(policy.LeaveTypes, attendanceLeaveType{
		ID: "custom-shared", Name: "연차 차감 휴가", BalanceMode: "annual",
		GrantCadence: "none", ExpiryMode: "none", AllowedUnits: []string{"fullDay"},
		IsActive: true, SortOrder: 5,
	})
	if errorValue := service.writeAttendanceLeavePolicy(t.Context(), policy); errorValue != nil {
		t.Fatal(errorValue)
	}
	employee := attendanceLeaveEmployee{Email: "staff@example.com"}
	if _, errorValue := service.grantAttendanceLeave(t.Context(), attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-shared-annual-balance",
			Employee:     employee,
			LeaveTypeID:  attendanceAnnualLeaveTypeID,
			Amount:       2000,
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
		`{"leaveTypeID":"custom-shared","unit":"fullDay","startDate":"2027-05-03"}`,
	)
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create status = %d body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	var created struct {
		Request struct {
			ID string `json:"id"`
		} `json:"request"`
	}
	if errorValue := json.NewDecoder(createRecorder.Body).Decode(&created); errorValue != nil {
		t.Fatal(errorValue)
	}
	annualBalance, errorValue := service.readAttendanceLeaveBalance(
		t.Context(),
		employee,
		attendanceAnnualLeaveTypeID,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	customBalance, errorValue := service.readAttendanceLeaveBalance(
		t.Context(),
		employee,
		"custom-shared",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if annualBalance.AvailableMilliDays != 1000 ||
		annualBalance.ReservedMilliDays != 1000 ||
		customBalance.AvailableMilliDays != 0 ||
		customBalance.ReservedMilliDays != 0 {
		t.Fatalf("annual = %+v custom = %+v", annualBalance, customBalance)
	}

	approvalRecorder := decideAttendanceLeaveForTest(
		t,
		service,
		created.Request.ID,
		`{"action":"approve"}`,
	)
	if approvalRecorder.Code != http.StatusOK {
		t.Fatalf("approve status = %d body = %s", approvalRecorder.Code, approvalRecorder.Body.String())
	}
	annualBalance, errorValue = service.readAttendanceLeaveBalance(
		t.Context(),
		employee,
		attendanceAnnualLeaveTypeID,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if annualBalance.AvailableMilliDays != 1000 ||
		annualBalance.ReservedMilliDays != 0 ||
		annualBalance.UsedMilliDays != 1000 {
		t.Fatalf("approved annual balance = %+v", annualBalance)
	}

	cancelRequest := httptest.NewRequest(
		http.MethodPost,
		"/attendance/api/leave-requests/"+created.Request.ID+"/cancel",
		nil,
	)
	cancelRequest.RemoteAddr = "203.0.113.10:1234"
	cancelRequest.Header.Set("X-Forwarded-Email", "staff@example.com")
	cancelRecorder := httptest.NewRecorder()
	service.handleAttendance(cancelRecorder, cancelRequest)
	if cancelRecorder.Code != http.StatusOK {
		t.Fatalf("cancel status = %d body = %s", cancelRecorder.Code, cancelRecorder.Body.String())
	}
	annualBalance, errorValue = service.readAttendanceLeaveBalance(
		t.Context(),
		employee,
		attendanceAnnualLeaveTypeID,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if annualBalance.AvailableMilliDays != 2000 ||
		annualBalance.ReservedMilliDays != 0 ||
		annualBalance.UsedMilliDays != 0 {
		t.Fatalf("cancelled annual balance = %+v", annualBalance)
	}
}

func TestAttendanceLeaveDashboardSummarizesOnlyIncludedBalanceAccounts(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	policy := defaultAttendanceLeavePolicy()
	for index := range policy.LeaveTypes {
		if policy.LeaveTypes[index].ID == "bereavement" {
			policy.LeaveTypes[index].BalanceMode = "separate"
			policy.LeaveTypes[index].IncludeInSummary = false
		}
	}
	if errorValue := service.writeAttendanceLeavePolicy(t.Context(), policy); errorValue != nil {
		t.Fatal(errorValue)
	}
	employee := attendanceLeaveEmployee{Email: "staff@example.com"}
	for _, grant := range []attendanceLeaveGrant{
		{
			Operation: attendanceLeaveOperation{
				OperationKey: "grant-dashboard-annual",
				Employee:     employee,
				LeaveTypeID:  "annual",
				Amount:       1000,
				EffectiveOn:  "2027-01-01",
			},
			ExpiresOn: "2028-01-01",
		},
		{
			Operation: attendanceLeaveOperation{
				OperationKey: "grant-dashboard-reward",
				Employee:     employee,
				LeaveTypeID:  "reward",
				Amount:       2000,
				EffectiveOn:  "2027-01-01",
			},
		},
		{
			Operation: attendanceLeaveOperation{
				OperationKey: "grant-dashboard-bereavement",
				Employee:     employee,
				LeaveTypeID:  "bereavement",
				Amount:       3000,
				EffectiveOn:  "2027-01-01",
			},
		},
	} {
		if _, errorValue := service.grantAttendanceLeave(t.Context(), grant); errorValue != nil {
			t.Fatal(errorValue)
		}
	}

	dashboard := readAttendanceLeaveDashboardForTest(t, service, employee.Email)
	if dashboard.Summary.AvailableMilliDays != 3000 {
		t.Fatalf("summary = %+v", dashboard.Summary)
	}
	expectedBalances := map[string]int{
		"annual":      1000,
		"reward":      2000,
		"bereavement": 3000,
	}
	for _, leaveType := range dashboard.LeaveTypes {
		expectedBalance, found := expectedBalances[leaveType.ID]
		if !found {
			continue
		}
		if leaveType.Balance == nil ||
			leaveType.Balance.AvailableMilliDays != expectedBalance {
			t.Fatalf("leave type %s balance = %+v", leaveType.ID, leaveType.Balance)
		}
		delete(expectedBalances, leaveType.ID)
	}
	if len(expectedBalances) != 0 {
		t.Fatalf("missing leave type balances = %+v", expectedBalances)
	}
}

func TestAttendanceLeaveRequestCreateInsufficientBalanceIsAtomic(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	employee := attendanceLeaveEmployee{Email: "staff@example.com"}
	if _, errorValue := service.grantAttendanceLeave(t.Context(), attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-leave-request-insufficient",
			Employee:     employee,
			LeaveTypeID:  "annual",
			Amount:       500,
			EffectiveOn:  "2027-01-01",
		},
		ExpiresOn: "2028-01-01",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}

	recorder := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests",
		"staff@example.com",
		`{"leaveTypeID":"annual","unit":"fullDay","startDate":"2027-05-03","reason":"Family appointment"}`,
	)
	assertAttendanceLeaveErrorResponse(
		t,
		recorder,
		http.StatusConflict,
		attendanceLeaveErrorInsufficientBalance,
	)
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
	if balance.AvailableMilliDays != 500 || balance.ReservedMilliDays != 0 {
		t.Fatalf("balance = %+v", balance)
	}
}

func TestAttendanceLeaveRequestCancelPendingHardDeletesAndReleasesReservation(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	employee := attendanceLeaveEmployee{Email: "staff@example.com"}
	if _, errorValue := service.grantAttendanceLeave(t.Context(), attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-leave-request-cancel-pending",
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
		`{"leaveTypeID":"annual","unit":"halfDay","startDate":"2027-05-03","partialPeriod":"morning","reason":"Family appointment"}`,
	)
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create status = %d body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	var created struct {
		Request struct {
			ID string `json:"id"`
		} `json:"request"`
	}
	if errorValue := json.NewDecoder(createRecorder.Body).Decode(&created); errorValue != nil {
		t.Fatal(errorValue)
	}

	cancelRequest := httptest.NewRequest(
		http.MethodPost,
		"/attendance/api/leave-requests/"+created.Request.ID+"/cancel",
		nil,
	)
	cancelRequest.RemoteAddr = "203.0.113.10:1234"
	cancelRequest.Header.Set("X-Forwarded-Email", "staff@example.com")
	cancelRecorder := httptest.NewRecorder()
	service.handleAttendance(cancelRecorder, cancelRequest)
	if cancelRecorder.Code != http.StatusOK {
		t.Fatalf("cancel status = %d body = %s", cancelRecorder.Code, cancelRecorder.Body.String())
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
	if balance.AvailableMilliDays != 1000 || balance.ReservedMilliDays != 0 {
		t.Fatalf("balance = %+v", balance)
	}
}

func TestAttendanceLeaveRequestResubmitReplacesReservationInOneTransaction(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	employee := attendanceLeaveEmployee{Email: "staff@example.com"}
	if _, errorValue := service.grantAttendanceLeave(t.Context(), attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-leave-request-resubmit",
			Employee:     employee,
			LeaveTypeID:  "annual",
			Amount:       1500,
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
		`{"leaveTypeID":"annual","unit":"fullDay","startDate":"2027-05-03","reason":"Original reason"}`,
	)
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create status = %d body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	var created struct {
		Request struct {
			ID string `json:"id"`
		} `json:"request"`
	}
	if errorValue := json.NewDecoder(createRecorder.Body).Decode(&created); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openAttendanceDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(t.Context(), `
UPDATE attendance_leave_requests
SET status = ?, admin_response = ?
WHERE id = ?`,
		attendanceLeaveRequestStatusNeedsChanges,
		"Please use a half day",
		created.Request.ID,
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	database.Close()

	resubmitRecorder := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests/"+created.Request.ID+"/resubmit",
		"staff@example.com",
		`{"leaveTypeID":"annual","unit":"halfDay","startDate":"2027-05-03","partialPeriod":"afternoon","reason":"Updated reason","response":"Changed to a half day"}`,
	)
	if resubmitRecorder.Code != http.StatusOK {
		t.Fatalf("resubmit status = %d body = %s", resubmitRecorder.Code, resubmitRecorder.Body.String())
	}
	dashboard := readAttendanceLeaveDashboardForTest(t, service, "staff@example.com")
	if dashboard.Summary.AvailableMilliDays != 1000 || dashboard.Summary.ReservedMilliDays != 500 {
		t.Fatalf("summary = %+v", dashboard.Summary)
	}
	if len(dashboard.Requests) != 1 ||
		dashboard.Requests[0].Status != attendanceLeaveRequestStatusPending ||
		dashboard.Requests[0].Unit != attendanceWorkScheduleHalfDay ||
		dashboard.Requests[0].Reason != "Updated reason" ||
		dashboard.Requests[0].PartialPeriod != attendanceLeavePartialPeriodAfternoon ||
		dashboard.Requests[0].StartTime != "14:00" ||
		dashboard.Requests[0].EndTime != "18:00" {
		t.Fatalf("request = %+v", dashboard.Requests)
	}
}
