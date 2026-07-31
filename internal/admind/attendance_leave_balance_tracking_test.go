package admind

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAttendanceLeavePolicyDefaultsLegacyBalanceTrackingToManaged(t *testing.T) {
	policy := defaultAttendanceLeavePolicy()
	if policy.BalanceTrackingMode != attendanceLeaveBalanceTrackingManaged {
		t.Fatalf("default balance tracking mode = %q", policy.BalanceTrackingMode)
	}
	policy.BalanceTrackingMode = ""
	normalizeLegacyAttendanceLeavePolicy(&policy)
	if policy.BalanceTrackingMode != attendanceLeaveBalanceTrackingManaged {
		t.Fatalf("normalized balance tracking mode = %q", policy.BalanceTrackingMode)
	}
}

func TestAttendanceLeaveUnlimitedModeReleasesPendingAndTracksUsageWithoutDeduction(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	employee := attendanceLeaveEmployee{Email: "staff@example.com"}
	if _, errorValue := service.grantAttendanceLeave(t.Context(), attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-before-unlimited-mode",
			Employee:     employee,
			LeaveTypeID:  attendanceAnnualLeaveTypeID,
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
		employee.Email,
		`{"leaveTypeID":"annual","unit":"fullDay","startDate":"2027-05-03"}`,
	)
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create status = %d body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	var created struct {
		Request attendanceLeaveRequestView `json:"request"`
	}
	if errorValue := json.NewDecoder(createRecorder.Body).Decode(&created); errorValue != nil {
		t.Fatal(errorValue)
	}
	balance, errorValue := service.readAttendanceLeaveBalance(
		t.Context(),
		employee,
		attendanceAnnualLeaveTypeID,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.AvailableMilliDays != 0 || balance.ReservedMilliDays != 1000 {
		t.Fatalf("reserved balance = %+v", balance)
	}

	policy := requestAttendanceLeavePolicy(
		t,
		service,
		http.MethodGet,
		"",
		"admin@example.com",
		http.StatusOK,
	)
	policy.BalanceTrackingMode = attendanceLeaveBalanceTrackingUnlimited
	encodedPolicy, errorValue := json.Marshal(policy)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	updated := requestAttendanceLeavePolicy(
		t,
		service,
		http.MethodPut,
		string(encodedPolicy),
		"admin@example.com",
		http.StatusOK,
	)
	if updated.BalanceTrackingMode != attendanceLeaveBalanceTrackingUnlimited {
		t.Fatalf("updated mode = %q", updated.BalanceTrackingMode)
	}
	balance, errorValue = service.readAttendanceLeaveBalance(
		t.Context(),
		employee,
		attendanceAnnualLeaveTypeID,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.AvailableMilliDays != 1000 ||
		balance.ReservedMilliDays != 0 ||
		balance.UsedMilliDays != 0 {
		t.Fatalf("released balance = %+v", balance)
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
	balance, errorValue = service.readAttendanceLeaveBalance(
		t.Context(),
		employee,
		attendanceAnnualLeaveTypeID,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.AvailableMilliDays != 1000 ||
		balance.ReservedMilliDays != 0 ||
		balance.UsedMilliDays != 1000 {
		t.Fatalf("approved balance = %+v", balance)
	}
	dashboard := readAttendanceLeaveDashboardForTest(t, service, employee.Email)
	if dashboard.BalanceTrackingMode != attendanceLeaveBalanceTrackingUnlimited ||
		dashboard.Summary.UsedMilliDays != 1000 ||
		dashboard.Summary.ReservedMilliDays != 0 ||
		dashboard.Summary.AvailableMilliDays != 0 ||
		dashboard.HireDateRequired {
		t.Fatalf("unlimited dashboard = %+v", dashboard)
	}
	annualType, found := attendanceLeaveTypeViewByID(dashboard.LeaveTypes, "annual")
	if !found ||
		annualType.Balance == nil ||
		annualType.Balance.UsedMilliDays != 1000 ||
		annualType.Balance.AvailableMilliDays != 0 {
		t.Fatalf("annual usage = %+v", annualType)
	}

	reloaded := NewService(service.Configuration)
	stored, errorValue := reloaded.readAttendanceLeavePolicy(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if stored.BalanceTrackingMode != attendanceLeaveBalanceTrackingUnlimited {
		t.Fatalf("reloaded mode = %q", stored.BalanceTrackingMode)
	}
}

func TestAttendanceLeaveUnlimitedModeAllowsRequestWithoutBalance(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	policy := defaultAttendanceLeavePolicy()
	policy.BalanceTrackingMode = attendanceLeaveBalanceTrackingUnlimited
	if errorValue := service.writeAttendanceLeavePolicy(t.Context(), policy); errorValue != nil {
		t.Fatal(errorValue)
	}
	recorder := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests",
		"staff@example.com",
		`{"leaveTypeID":"annual","unit":"fullDay","startDate":"2027-05-03"}`,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("create status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	dashboard := readAttendanceLeaveDashboardForTest(t, service, "staff@example.com")
	if dashboard.Summary.ReservedMilliDays != 1000 ||
		dashboard.Summary.AvailableMilliDays != 0 ||
		dashboard.HireDateRequired {
		t.Fatalf("dashboard = %+v", dashboard)
	}
}

func TestAttendanceLeaveManagedModeBindsPendingUnlimitedRequestToBalance(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	employee := attendanceLeaveEmployee{Email: "staff@example.com"}
	if _, errorValue := service.grantAttendanceLeave(t.Context(), attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-before-return-to-managed-mode",
			Employee:     employee,
			LeaveTypeID:  attendanceAnnualLeaveTypeID,
			Amount:       1000,
			EffectiveOn:  "2027-01-01",
		},
		ExpiresOn: "2028-01-01",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	policy := defaultAttendanceLeavePolicy()
	policy.BalanceTrackingMode = attendanceLeaveBalanceTrackingUnlimited
	if errorValue := service.writeAttendanceLeavePolicy(t.Context(), policy); errorValue != nil {
		t.Fatal(errorValue)
	}
	unlimitedRequest := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests",
		employee.Email,
		`{"leaveTypeID":"annual","unit":"fullDay","startDate":"2027-05-03"}`,
	)
	if unlimitedRequest.Code != http.StatusOK {
		t.Fatalf("unlimited create status = %d body = %s", unlimitedRequest.Code, unlimitedRequest.Body.String())
	}
	var created struct {
		Request attendanceLeaveRequestView `json:"request"`
	}
	if errorValue := json.NewDecoder(unlimitedRequest.Body).Decode(&created); errorValue != nil {
		t.Fatal(errorValue)
	}
	policy.BalanceTrackingMode = attendanceLeaveBalanceTrackingManaged
	if errorValue := service.writeAttendanceLeavePolicy(t.Context(), policy); errorValue != nil {
		t.Fatal(errorValue)
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
	var approved struct {
		Request attendanceLeaveApprovalRequestView `json:"request"`
	}
	if errorValue := json.NewDecoder(approvalRecorder.Body).Decode(&approved); errorValue != nil {
		t.Fatal(errorValue)
	}
	if approved.Request.BalanceMode != "annual" {
		t.Fatalf("approved balance mode = %q", approved.Request.BalanceMode)
	}
	balance, errorValue := service.readAttendanceLeaveBalance(
		t.Context(),
		employee,
		attendanceAnnualLeaveTypeID,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.AvailableMilliDays != 0 ||
		balance.ReservedMilliDays != 0 ||
		balance.UsedMilliDays != 1000 {
		t.Fatalf("managed approval balance = %+v", balance)
	}
	managedRequest := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests",
		employee.Email,
		`{"leaveTypeID":"annual","unit":"fullDay","startDate":"2027-05-04"}`,
	)
	if managedRequest.Code != http.StatusConflict {
		t.Fatalf("managed create status = %d body = %s", managedRequest.Code, managedRequest.Body.String())
	}
}

func TestAttendanceLeaveManagedModeCountsUnlimitedUsageWithoutDoubleDeduction(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	employee := attendanceLeaveEmployee{Email: "staff@example.com"}
	if _, errorValue := service.grantAttendanceLeave(t.Context(), attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-before-repeated-policy-switches",
			Employee:     employee,
			LeaveTypeID:  attendanceAnnualLeaveTypeID,
			Amount:       1000,
			EffectiveOn:  "2027-01-01",
		},
		ExpiresOn: "2028-01-01",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	policy := defaultAttendanceLeavePolicy()
	policy.BalanceTrackingMode = attendanceLeaveBalanceTrackingUnlimited
	if errorValue := service.writeAttendanceLeavePolicy(t.Context(), policy); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, date := range []string{"2027-05-03", "2027-05-04"} {
		createRecorder := performAttendanceLeaveMultipartRequest(
			t,
			service,
			http.MethodPost,
			"/attendance/api/leave-requests",
			employee.Email,
			`{"leaveTypeID":"annual","unit":"fullDay","startDate":"`+date+`"}`,
		)
		if createRecorder.Code != http.StatusOK {
			t.Fatalf("unlimited create status = %d body = %s", createRecorder.Code, createRecorder.Body.String())
		}
		var created struct {
			Request attendanceLeaveRequestView `json:"request"`
		}
		if errorValue := json.NewDecoder(createRecorder.Body).Decode(&created); errorValue != nil {
			t.Fatal(errorValue)
		}
		approvalRecorder := decideAttendanceLeaveForTest(
			t,
			service,
			created.Request.ID,
			`{"action":"approve"}`,
		)
		if approvalRecorder.Code != http.StatusOK {
			t.Fatalf("unlimited approve status = %d body = %s", approvalRecorder.Code, approvalRecorder.Body.String())
		}
	}
	asOf := time.Date(2027, 5, 5, 0, 0, 0, 0, time.UTC)
	for _, mode := range []string{
		attendanceLeaveBalanceTrackingManaged,
		attendanceLeaveBalanceTrackingUnlimited,
		attendanceLeaveBalanceTrackingManaged,
	} {
		policy.BalanceTrackingMode = mode
		if errorValue := service.writeAttendanceLeavePolicy(t.Context(), policy); errorValue != nil {
			t.Fatal(errorValue)
		}
		if mode != attendanceLeaveBalanceTrackingManaged {
			continue
		}
		dashboard, errorValue := service.readAttendanceLeaveDashboard(t.Context(), employee, asOf)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if dashboard.Summary.AvailableMilliDays != 0 ||
			dashboard.Summary.UsedMilliDays != 2000 {
			t.Fatalf("managed dashboard after repeated switch = %+v", dashboard.Summary)
		}
	}
	managedRequest := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests",
		employee.Email,
		`{"leaveTypeID":"annual","unit":"fullDay","startDate":"2027-05-05"}`,
	)
	if managedRequest.Code != http.StatusConflict {
		t.Fatalf("managed create status = %d body = %s", managedRequest.Code, managedRequest.Body.String())
	}
}

func TestAttendanceLeaveManagedModeExcludesCancelledUnlimitedUsage(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	employee := attendanceLeaveEmployee{Email: "staff@example.com"}
	if _, errorValue := service.grantAttendanceLeave(t.Context(), attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-before-cancelled-unlimited-use",
			Employee:     employee,
			LeaveTypeID:  attendanceAnnualLeaveTypeID,
			Amount:       1000,
			EffectiveOn:  "2027-01-01",
		},
		ExpiresOn: "2028-01-01",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	policy := requestAttendanceLeavePolicy(
		t,
		service,
		http.MethodGet,
		"",
		"admin@example.com",
		http.StatusOK,
	)
	policy.BalanceTrackingMode = attendanceLeaveBalanceTrackingUnlimited
	encodedPolicy, errorValue := json.Marshal(policy)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	policy = requestAttendanceLeavePolicy(
		t,
		service,
		http.MethodPut,
		string(encodedPolicy),
		"admin@example.com",
		http.StatusOK,
	)
	createRecorder := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests",
		employee.Email,
		`{"leaveTypeID":"annual","unit":"fullDay","startDate":"2027-05-03"}`,
	)
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create status = %d body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	var created struct {
		Request attendanceLeaveRequestView `json:"request"`
	}
	if errorValue := json.NewDecoder(createRecorder.Body).Decode(&created); errorValue != nil {
		t.Fatal(errorValue)
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
	policy.BalanceTrackingMode = attendanceLeaveBalanceTrackingManaged
	encodedPolicy, errorValue = json.Marshal(policy)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	requestAttendanceLeavePolicy(
		t,
		service,
		http.MethodPut,
		string(encodedPolicy),
		"admin@example.com",
		http.StatusOK,
	)
	cancelRequest := httptest.NewRequest(
		http.MethodPost,
		"/attendance/api/leave-requests/"+created.Request.ID+"/cancel",
		nil,
	)
	cancelRequest.RemoteAddr = "203.0.113.10:1234"
	cancelRequest.Header.Set("X-Forwarded-Email", employee.Email)
	cancelRecorder := httptest.NewRecorder()
	service.handleAttendance(cancelRecorder, cancelRequest)
	if cancelRecorder.Code != http.StatusOK {
		t.Fatalf("cancel status = %d body = %s", cancelRecorder.Code, cancelRecorder.Body.String())
	}
	dashboard := readAttendanceLeaveDashboardForTest(t, service, employee.Email)
	annualType, found := attendanceLeaveTypeViewByID(
		dashboard.LeaveTypes,
		attendanceAnnualLeaveTypeID,
	)
	if !found ||
		annualType.Balance == nil ||
		annualType.Balance.AvailableMilliDays != 1000 ||
		annualType.Balance.UsedMilliDays != 0 ||
		dashboard.Summary.UsedMilliDays != 0 {
		t.Fatalf("cancelled unlimited usage = %+v summary = %+v", annualType, dashboard.Summary)
	}
}

func TestAttendanceLeaveManagedModeReconcilesOnlyCurrentFiscalYearUsage(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	employee := attendanceLeaveEmployee{Email: "staff@example.com"}
	if _, errorValue := service.grantAttendanceLeave(t.Context(), attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-for-fiscal-boundary-reconciliation",
			Employee:     employee,
			LeaveTypeID:  attendanceAnnualLeaveTypeID,
			Amount:       2000,
			EffectiveOn:  "2027-01-01",
		},
		ExpiresOn: "2028-01-01",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	policy := defaultAttendanceLeavePolicy()
	policy.BalanceTrackingMode = attendanceLeaveBalanceTrackingUnlimited
	policy.FiscalYearStartMonth = 4
	policy.FiscalYearStartDay = 1
	if errorValue := service.writeAttendanceLeavePolicy(t.Context(), policy); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, date := range []string{"2027-03-31", "2027-04-01"} {
		createRecorder := performAttendanceLeaveMultipartRequest(
			t,
			service,
			http.MethodPost,
			"/attendance/api/leave-requests",
			employee.Email,
			`{"leaveTypeID":"annual","unit":"fullDay","startDate":"`+date+`"}`,
		)
		if createRecorder.Code != http.StatusOK {
			t.Fatalf("create %s status = %d body = %s", date, createRecorder.Code, createRecorder.Body.String())
		}
		var created struct {
			Request attendanceLeaveRequestView `json:"request"`
		}
		if errorValue := json.NewDecoder(createRecorder.Body).Decode(&created); errorValue != nil {
			t.Fatal(errorValue)
		}
		approvalRecorder := decideAttendanceLeaveForTest(
			t,
			service,
			created.Request.ID,
			`{"action":"approve"}`,
		)
		if approvalRecorder.Code != http.StatusOK {
			t.Fatalf("approve %s status = %d body = %s", date, approvalRecorder.Code, approvalRecorder.Body.String())
		}
	}
	policy.BalanceTrackingMode = attendanceLeaveBalanceTrackingManaged
	if errorValue := service.writeAttendanceLeavePolicy(t.Context(), policy); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openAttendanceDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	rawBalance, errorValue := queryAttendanceLeaveBalance(
		t.Context(),
		database,
		employee,
		attendanceAnnualLeaveTypeID,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, asOf := range []time.Time{
		time.Date(2027, 3, 31, 0, 0, 0, 0, time.UTC),
		time.Date(2027, 4, 2, 0, 0, 0, 0, time.UTC),
	} {
		balance, balanceError := service.attendanceLeaveBalanceWithUntrackedUsage(
			t.Context(),
			database,
			rawBalance,
			policy,
			asOf,
		)
		if balanceError != nil {
			t.Fatal(balanceError)
		}
		if balance.AvailableMilliDays != 1000 {
			t.Fatalf("effective balance at %s = %+v", asOf.Format(time.DateOnly), balance)
		}
	}
}

func TestAttendanceLeaveBalanceTrackingSerializesModeChangeWithRequest(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	employee := attendanceLeaveEmployee{Email: "staff@example.com"}
	if _, errorValue := service.grantAttendanceLeave(t.Context(), attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-before-concurrent-mode-request",
			Employee:     employee,
			LeaveTypeID:  "reward",
			Amount:       1000,
			EffectiveOn:  "2027-01-01",
		},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	policy := requestAttendanceLeavePolicy(
		t,
		service,
		http.MethodGet,
		"",
		"admin@example.com",
		http.StatusOK,
	)
	policy.BalanceTrackingMode = attendanceLeaveBalanceTrackingUnlimited
	encodedPolicy, errorValue := json.Marshal(policy)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	policyRequest := httptest.NewRequest(
		http.MethodPut,
		"/admin/api/attendance-leave-policy",
		strings.NewReader(string(encodedPolicy)),
	)
	policyRequest.RemoteAddr = "198.51.100.10:443"
	policyRequest.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	leaveRequest, errorValue := newConcurrentAttendanceLeaveRequest(
		`{"leaveTypeID":"reward","unit":"fullDay","startDate":"2027-05-03"}`,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	policyRecorder := httptest.NewRecorder()
	leaveRecorder := httptest.NewRecorder()
	router := service.router()
	start := make(chan struct{})
	var requests sync.WaitGroup
	requests.Add(2)
	go func() {
		defer requests.Done()
		<-start
		router.ServeHTTP(policyRecorder, policyRequest)
	}()
	go func() {
		defer requests.Done()
		<-start
		router.ServeHTTP(leaveRecorder, leaveRequest)
	}()
	close(start)
	requests.Wait()
	if policyRecorder.Code != http.StatusOK {
		t.Fatalf(
			"policy status = %d body = %s",
			policyRecorder.Code,
			policyRecorder.Body.String(),
		)
	}
	if leaveRecorder.Code != http.StatusOK {
		t.Fatalf(
			"leave status = %d body = %s",
			leaveRecorder.Code,
			leaveRecorder.Body.String(),
		)
	}
	balance, errorValue := service.readAttendanceLeaveBalance(
		t.Context(),
		employee,
		"reward",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.AvailableMilliDays != 1000 || balance.ReservedMilliDays != 0 {
		t.Fatalf("concurrent balance = %+v", balance)
	}
	dashboard := readAttendanceLeaveDashboardForTest(t, service, employee.Email)
	if dashboard.BalanceTrackingMode != attendanceLeaveBalanceTrackingUnlimited ||
		dashboard.Summary.ReservedMilliDays != 1000 ||
		len(dashboard.Requests) != 1 {
		t.Fatalf("concurrent dashboard = %+v", dashboard)
	}
}

func TestAttendanceLeaveBalanceTrackingSerializesModeChangeWithApproval(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	employee := attendanceLeaveEmployee{Email: "staff@example.com"}
	if _, errorValue := service.grantAttendanceLeave(t.Context(), attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-before-concurrent-mode-approval",
			Employee:     employee,
			LeaveTypeID:  "reward",
			Amount:       1000,
			EffectiveOn:  "2027-01-01",
		},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	createRecorder := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests",
		employee.Email,
		`{"leaveTypeID":"reward","unit":"fullDay","startDate":"2027-05-03"}`,
	)
	if createRecorder.Code != http.StatusOK {
		t.Fatalf(
			"create status = %d body = %s",
			createRecorder.Code,
			createRecorder.Body.String(),
		)
	}
	var created struct {
		Request attendanceLeaveRequestView `json:"request"`
	}
	if errorValue := json.NewDecoder(createRecorder.Body).Decode(&created); errorValue != nil {
		t.Fatal(errorValue)
	}
	policy := requestAttendanceLeavePolicy(
		t,
		service,
		http.MethodGet,
		"",
		"admin@example.com",
		http.StatusOK,
	)
	policy.BalanceTrackingMode = attendanceLeaveBalanceTrackingUnlimited
	encodedPolicy, errorValue := json.Marshal(policy)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	policyRequest := httptest.NewRequest(
		http.MethodPut,
		"/admin/api/attendance-leave-policy",
		strings.NewReader(string(encodedPolicy)),
	)
	policyRequest.RemoteAddr = "198.51.100.10:443"
	policyRequest.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	approvalRequest := httptest.NewRequest(
		http.MethodPost,
		"/attendance/api/leave-approvals/"+created.Request.ID,
		strings.NewReader(`{"action":"approve"}`),
	)
	approvalRequest.RemoteAddr = "198.51.100.10:443"
	approvalRequest.Header.Set("X-Forwarded-Email", "admin@example.com")
	approvalRecorder := httptest.NewRecorder()
	policyRecorder := httptest.NewRecorder()
	router := service.router()
	start := make(chan struct{})
	var requests sync.WaitGroup
	requests.Add(2)
	go func() {
		defer requests.Done()
		<-start
		router.ServeHTTP(policyRecorder, policyRequest)
	}()
	go func() {
		defer requests.Done()
		<-start
		router.ServeHTTP(approvalRecorder, approvalRequest)
	}()
	close(start)
	requests.Wait()
	if policyRecorder.Code != http.StatusOK {
		t.Fatalf(
			"policy status = %d body = %s",
			policyRecorder.Code,
			policyRecorder.Body.String(),
		)
	}
	if approvalRecorder.Code != http.StatusOK {
		t.Fatalf(
			"approval status = %d body = %s",
			approvalRecorder.Code,
			approvalRecorder.Body.String(),
		)
	}
	balance, errorValue := service.readAttendanceLeaveBalance(
		t.Context(),
		employee,
		"reward",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.ReservedMilliDays != 0 ||
		(balance.AvailableMilliDays != 0 && balance.AvailableMilliDays != 1000) {
		t.Fatalf("concurrent approval balance = %+v", balance)
	}
	dashboard := readAttendanceLeaveDashboardForTest(t, service, employee.Email)
	if dashboard.BalanceTrackingMode != attendanceLeaveBalanceTrackingUnlimited ||
		dashboard.Summary.UsedMilliDays != 1000 ||
		dashboard.Summary.ReservedMilliDays != 0 ||
		len(dashboard.Requests) != 1 ||
		dashboard.Requests[0].Status != attendanceLeaveRequestStatusApproved {
		t.Fatalf("concurrent approval dashboard = %+v", dashboard)
	}
}

func newConcurrentAttendanceLeaveRequest(requestJSON string) (*http.Request, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	field, errorValue := writer.CreateFormField("request")
	if errorValue != nil {
		return nil, errorValue
	}
	if _, errorValue = field.Write([]byte(requestJSON)); errorValue != nil {
		return nil, errorValue
	}
	if errorValue = writer.Close(); errorValue != nil {
		return nil, errorValue
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/attendance/api/leave-requests",
		&body,
	)
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-Email", "staff@example.com")
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request, nil
}

func attendanceLeaveTypeViewByID(
	leaveTypes []attendanceLeaveTypeView,
	leaveTypeID string,
) (attendanceLeaveTypeView, bool) {
	for _, leaveType := range leaveTypes {
		if leaveType.ID == leaveTypeID {
			return leaveType, true
		}
	}
	return attendanceLeaveTypeView{}, false
}
