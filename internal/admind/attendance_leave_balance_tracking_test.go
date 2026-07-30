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

func TestAttendanceLeaveManagedModeDoesNotRetroactivelyDeductUnlimitedRequests(t *testing.T) {
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
	balance, errorValue := service.readAttendanceLeaveBalance(
		t.Context(),
		employee,
		attendanceAnnualLeaveTypeID,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.AvailableMilliDays != 1000 || balance.ReservedMilliDays != 0 {
		t.Fatalf("retroactively deducted balance = %+v", balance)
	}
	managedRequest := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests",
		employee.Email,
		`{"leaveTypeID":"annual","unit":"fullDay","startDate":"2027-05-04"}`,
	)
	if managedRequest.Code != http.StatusOK {
		t.Fatalf("managed create status = %d body = %s", managedRequest.Code, managedRequest.Body.String())
	}
	balance, errorValue = service.readAttendanceLeaveBalance(
		t.Context(),
		employee,
		attendanceAnnualLeaveTypeID,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.AvailableMilliDays != 0 || balance.ReservedMilliDays != 1000 {
		t.Fatalf("managed balance = %+v", balance)
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
