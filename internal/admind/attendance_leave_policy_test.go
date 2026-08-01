package admind

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLeavePolicyDefaults(t *testing.T) {
	policy := defaultAttendanceLeavePolicy()
	if policy.Version != attendanceLeavePolicyVersion || len(policy.LeaveTypes) != 16 {
		t.Fatalf("defaults = %+v", policy)
	}
	annual := policy.LeaveTypes[0]
	if annual.ID != "annual" ||
		!annual.Paid ||
		annual.GrantCadence != "annual" ||
		annual.GrantAmountMilliDays != attendanceDefaultAnnualGrantMilliDays ||
		!annual.IncludeInSummary {
		t.Fatalf("annual default = %+v", annual)
	}
	if policy.LeaveTypes[9].Name != "포상휴가" ||
		policy.LeaveTypes[9].BalanceMode != "separate" ||
		!policy.LeaveTypes[9].IncludeInSummary ||
		policy.LeaveTypes[13].Name != "육아휴직" {
		t.Fatalf("expanded defaults = %+v", policy.LeaveTypes)
	}
	if errorValue := validateAttendanceLeavePolicy(&policy, nil); errorValue != nil {
		t.Fatalf("validate defaults: %v", errorValue)
	}
}

func TestLeavePolicyNormalizesLegacyGrantCadences(t *testing.T) {
	policy := defaultAttendanceLeavePolicy()
	policy.LeaveTypes[0].GrantCadence = "statutory"
	policy.LeaveTypes[2].GrantCadence = "manual"

	normalizeLegacyAttendanceLeavePolicy(&policy)

	if policy.LeaveTypes[0].GrantCadence != "annual" ||
		policy.LeaveTypes[2].GrantCadence != "none" {
		t.Fatalf("normalized policy = %+v", policy.LeaveTypes)
	}
}

func TestLeavePolicyMigratesV1DefaultsOnce(t *testing.T) {
	current := defaultAttendanceLeavePolicy()
	policy := attendanceLeavePolicy{
		Version:              1,
		FiscalYearStartMonth: 1,
		FiscalYearStartDay:   1,
		LeaveTypes: []attendanceLeaveType{
			current.LeaveTypes[0],
			current.LeaveTypes[1],
			{
				ID: "bereavement", SystemKind: "bereavement", Name: "경조휴가",
				BalanceMode: "separate", GrantCadence: "none", ExpiryMode: "none",
				AllowedUnits: []string{"fullDay"}, IsActive: true, IsSystem: true, SortOrder: 2,
			},
			current.LeaveTypes[14],
			current.LeaveTypes[15],
		},
	}

	normalizeLegacyAttendanceLeavePolicy(&policy)
	if policy.Version != attendanceLeavePolicyVersion || len(policy.LeaveTypes) != 16 {
		t.Fatalf("migrated policy = %+v", policy)
	}
	if !policy.LeaveTypes[2].IncludeInSummary || policy.LeaveTypes[2].BalanceMode != "separate" {
		t.Fatalf("existing balance behavior was not preserved = %+v", policy.LeaveTypes[2])
	}
	policy.LeaveTypes = policy.LeaveTypes[:15]
	normalizeLegacyAttendanceLeavePolicy(&policy)
	if len(policy.LeaveTypes) != 15 {
		t.Fatalf("removed type was restored = %+v", policy.LeaveTypes)
	}
}

func TestLeavePolicyValidationRejectsInvalidPolicies(t *testing.T) {
	negativeCarryoverLimit := -1
	tests := []struct {
		name   string
		mutate func(*attendanceLeavePolicy)
	}{
		{name: "annual inactive", mutate: func(policy *attendanceLeavePolicy) { policy.LeaveTypes[0].IsActive = false }},
		{name: "untracked summary", mutate: func(policy *attendanceLeavePolicy) { policy.LeaveTypes[1].IncludeInSummary = true }},
		{name: "balance mode", mutate: func(policy *attendanceLeavePolicy) { policy.LeaveTypes[0].BalanceMode = "invalid" }},
		{name: "grant cadence", mutate: func(policy *attendanceLeavePolicy) { policy.LeaveTypes[1].GrantCadence = "invalid" }},
		{name: "expiry mode", mutate: func(policy *attendanceLeavePolicy) { policy.LeaveTypes[1].ExpiryMode = "invalid" }},
		{name: "expiry months", mutate: func(policy *attendanceLeavePolicy) { policy.LeaveTypes[1].ExpiryMode = "monthsAfterGrant" }},
		{name: "carryover limit", mutate: func(policy *attendanceLeavePolicy) {
			policy.LeaveTypes[1].CarryoverEnabled = true
			policy.LeaveTypes[1].CarryoverLimitMilliDays = &negativeCarryoverLimit
		}},
		{name: "unknown unit", mutate: func(policy *attendanceLeavePolicy) { policy.LeaveTypes[1].AllowedUnits = []string{"invalid"} }},
		{name: "duplicate unit", mutate: func(policy *attendanceLeavePolicy) {
			policy.LeaveTypes[1].AllowedUnits = []string{"fullDay", "fullDay"}
		}},
		{name: "duplicate id", mutate: func(policy *attendanceLeavePolicy) { policy.LeaveTypes[1].ID = "annual" }},
		{name: "duplicate name", mutate: func(policy *attendanceLeavePolicy) { policy.LeaveTypes[1].Name = " 연차 " }},
		{name: "negative grant", mutate: func(policy *attendanceLeavePolicy) { policy.LeaveTypes[1].GrantAmountMilliDays = -1 }},
		{name: "negative order", mutate: func(policy *attendanceLeavePolicy) { policy.LeaveTypes[1].SortOrder = -1 }},
		{name: "annual shared policy", mutate: func(policy *attendanceLeavePolicy) {
			policy.LeaveTypes = append(policy.LeaveTypes, attendanceLeaveType{
				ID: "custom-shared", Name: "연차 차감 휴가", BalanceMode: "annual",
				GrantCadence: "annual", GrantAmountMilliDays: 1000, ExpiryMode: "none", AllowedUnits: []string{"fullDay"},
				IsActive: true, SortOrder: 5,
			})
		}},
		{name: "invalid fiscal date", mutate: func(policy *attendanceLeavePolicy) {
			policy.FiscalYearStartMonth = 2
			policy.FiscalYearStartDay = 30
		}},
		{name: "system identity", mutate: func(policy *attendanceLeavePolicy) { policy.LeaveTypes[1].SystemKind = "other" }},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			policy := defaultAttendanceLeavePolicy()
			testCase.mutate(&policy)
			if validateAttendanceLeavePolicy(&policy, nil) == nil {
				t.Fatal("expected validation failure")
			}
		})
	}
}

func TestLeavePolicyValidationAllowsDeletedTypes(t *testing.T) {
	policy := defaultAttendanceLeavePolicy()
	policy.LeaveTypes = policy.LeaveTypes[1:]
	if errorValue := validateAttendanceLeavePolicy(&policy, nil); errorValue != nil {
		t.Fatalf("validate deleted system type: %v", errorValue)
	}

	existing := defaultAttendanceLeavePolicy()
	customType := attendanceLeaveType{
		ID: "custom-existing", Name: "기존 휴가", BalanceMode: "none", GrantCadence: "none",
		ExpiryMode: "none", AllowedUnits: []string{"fullDay"}, IsActive: true, SortOrder: 5,
	}
	existing.LeaveTypes = append(existing.LeaveTypes, customType)
	policy = defaultAttendanceLeavePolicy()
	if errorValue := validateAttendanceLeavePolicy(&policy, &existing); errorValue != nil {
		t.Fatalf("validate deleted custom type: %v", errorValue)
	}
}

func TestLeavePolicyAdminAPIRoundtrip(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	policy := requestAttendanceLeavePolicy(t, service, http.MethodGet, "", "admin@example.com", http.StatusOK)
	if len(policy.LeaveTypes) != 16 {
		t.Fatalf("default leave types = %d", len(policy.LeaveTypes))
	}

	policy.LeaveTypes = append(policy.LeaveTypes, attendanceLeaveType{
		Name: "회사 특별 휴가", BalanceMode: "separate", GrantCadence: "none",
		ExpiryMode: "none", AllowedUnits: []string{"fullDay"}, IncludeInSummary: true,
		IsActive: true, SortOrder: 16,
	})
	encodedPolicy, errorValue := json.Marshal(policy)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	updated := requestAttendanceLeavePolicy(t, service, http.MethodPut, string(encodedPolicy), "admin@example.com", http.StatusOK)
	customType := updated.LeaveTypes[len(updated.LeaveTypes)-1]
	if !strings.HasPrefix(customType.ID, "custom-") || customType.Name != "회사 특별 휴가" || updated.UpdatedAt == "" {
		t.Fatalf("saved custom type = %+v policy = %+v", customType, updated)
	}

	stored := requestAttendanceLeavePolicy(t, service, http.MethodGet, "", "admin@example.com", http.StatusOK)
	if stored.LeaveTypes[len(stored.LeaveTypes)-1].ID != customType.ID {
		t.Fatalf("stored custom type = %+v", stored.LeaveTypes[len(stored.LeaveTypes)-1])
	}
}

func TestLeavePolicyAdminAPIImmediatelyAdjustsCurrentGrant(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	if errorValue := service.writeOrganizationProfiles(t.Context(), []organizationProfile{{
		UserID:   "user-1",
		Email:    "staff@example.com",
		HireDate: "2026-01-01",
	}}); errorValue != nil {
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
	policy.LeaveTypes[0].GrantAmountMilliDays = 16000
	encodedPolicy, errorValue := json.Marshal(policy)
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
	balance, errorValue := service.readAttendanceLeaveBalance(
		t.Context(),
		attendanceLeaveEmployee{Email: "staff@example.com", UserID: "user-1"},
		attendanceAnnualLeaveTypeID,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.GrantedMilliDays != 16000 || balance.AvailableMilliDays != 16000 {
		t.Fatalf("adjusted annual balance = %+v", balance)
	}
	employee := attendanceLeaveEmployee{Email: "staff@example.com", UserID: "user-1"}
	if _, errorValue := service.reserveAttendanceLeave(t.Context(), attendanceLeaveOperation{
		OperationKey: "reserve-before-policy-reduction",
		Employee:     employee,
		LeaveTypeID:  attendanceAnnualLeaveTypeID,
		ReferenceID:  "request-before-policy-reduction",
		Amount:       2000,
		EffectiveOn:  "2026-07-30",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	policy = requestAttendanceLeavePolicy(
		t,
		service,
		http.MethodGet,
		"",
		"admin@example.com",
		http.StatusOK,
	)
	policy.LeaveTypes[0].GrantAmountMilliDays = 1000
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
		http.StatusBadRequest,
	)
	stored := requestAttendanceLeavePolicy(
		t,
		service,
		http.MethodGet,
		"",
		"admin@example.com",
		http.StatusOK,
	)
	if stored.LeaveTypes[0].GrantAmountMilliDays != 16000 {
		t.Fatalf("stored policy after rejected reduction = %+v", stored.LeaveTypes[0])
	}
}

func TestLeavePolicyAdjustmentRetryUsesStableOperationKey(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	employee := attendanceLeaveEmployee{
		Email:    "staff@example.com",
		UserID:   "user-1",
		HireDate: "2026-01-01",
	}
	existing := defaultAttendanceLeavePolicy()
	existing.UpdatedAt = "2026-07-30T00:00:00Z"
	updated := existing
	updated.LeaveTypes = append([]attendanceLeaveType{}, existing.LeaveTypes...)
	updated.LeaveTypes[0] = existing.LeaveTypes[0]
	updated.LeaveTypes[0].GrantAmountMilliDays = 16000
	updated.UpdatedAt = "2026-07-30T01:00:00Z"
	request := httptest.NewRequest(http.MethodPut, "/admin/api/attendance-leave-policy", nil)

	first, required, errorValue := service.attendanceLeavePolicyAdjustment(
		request,
		employee,
		existing,
		existing.LeaveTypes[0],
		updated,
		updated.LeaveTypes[0],
		time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC),
	)
	if errorValue != nil || !required {
		t.Fatalf("first adjustment required = %t, error = %v", required, errorValue)
	}
	if _, errorValue = service.adjustAttendanceLeave(
		t.Context(),
		first.Operation,
		first.AmountMilliDays,
		first.ExpiresOn,
	); errorValue != nil {
		t.Fatal(errorValue)
	}

	updated.UpdatedAt = "2026-07-30T02:00:00Z"
	retry, required, errorValue := service.attendanceLeavePolicyAdjustment(
		request,
		employee,
		existing,
		existing.LeaveTypes[0],
		updated,
		updated.LeaveTypes[0],
		time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC),
	)
	if errorValue != nil || !required {
		t.Fatalf("retry adjustment required = %t, error = %v", required, errorValue)
	}
	if retry.Operation.OperationKey != first.Operation.OperationKey {
		t.Fatalf(
			"retry operation key = %q, first = %q",
			retry.Operation.OperationKey,
			first.Operation.OperationKey,
		)
	}
	if _, errorValue = service.adjustAttendanceLeave(
		t.Context(),
		retry.Operation,
		retry.AmountMilliDays,
		retry.ExpiresOn,
	); errorValue != nil {
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
	if balance.GrantedMilliDays != 1000 || balance.AvailableMilliDays != 1000 {
		t.Fatalf("retry balance = %+v", balance)
	}
}

func TestLeavePolicyExpiryChangesApplyToAvailableGrantLots(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	employee := attendanceLeaveEmployee{Email: "staff@example.com", UserID: "user-1"}
	if _, errorValue := service.grantAttendanceLeave(t.Context(), attendanceLeaveGrant{
		Operation: attendanceLeaveOperation{
			OperationKey: "grant-before-expiry-policy-update",
			Employee:     employee,
			LeaveTypeID:  attendanceAnnualLeaveTypeID,
			Kind:         attendanceLeaveOperationGrant,
			ReferenceID:  "test",
			Amount:       1000,
			EffectiveOn:  "2026-07-30",
		},
		ExpiresOn: "2027-01-01",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	existing := defaultAttendanceLeavePolicy()
	updated := defaultAttendanceLeavePolicy()
	updated.LeaveTypes[0].ExpiryMode = "none"
	if errorValue := service.updateAttendanceLeavePolicyExpiries(
		t.Context(),
		[]attendanceLeaveEmployee{employee},
		existing,
		updated,
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openAttendanceDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var expiresOn sql.NullString
	if errorValue := database.QueryRowContext(
		t.Context(),
		`SELECT expires_on
FROM attendance_leave_grant_lots
WHERE source_operation_key = ?`,
		"grant-before-expiry-policy-update",
	).Scan(&expiresOn); errorValue != nil {
		t.Fatal(errorValue)
	}
	if expiresOn.Valid {
		t.Fatalf("updated expiry = %q", expiresOn.String)
	}
}

func TestLeavePolicyAdminAPIRejectsUnknownFieldsAndDeletesUnusedType(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	requestAttendanceLeavePolicy(t, service, http.MethodPut, `{"version":1,"unknown":true}`, "admin@example.com", http.StatusBadRequest)

	policy := requestAttendanceLeavePolicy(t, service, http.MethodGet, "", "admin@example.com", http.StatusOK)
	policy.LeaveTypes = policy.LeaveTypes[1:]
	encodedPolicy, errorValue := json.Marshal(policy)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	updated := requestAttendanceLeavePolicy(t, service, http.MethodPut, string(encodedPolicy), "admin@example.com", http.StatusOK)
	if len(updated.LeaveTypes) != 15 {
		t.Fatalf("unused leave type was not deleted = %+v", updated.LeaveTypes)
	}
}

func TestLeavePolicyAdminAPIArchivesUsedRemovedType(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	employee := attendanceLeaveEmployee{Email: "staff@example.com", UserID: "user-1"}
	if _, errorValue := service.recordUntrackedAttendanceLeaveUse(t.Context(), attendanceLeaveOperation{
		OperationKey: "used-reward-leave",
		Employee:     employee,
		LeaveTypeID:  "reward",
		ReferenceID:  "reward-request",
		Amount:       500,
		EffectiveOn:  "2026-07-30",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	policy := requestAttendanceLeavePolicy(t, service, http.MethodGet, "", "admin@example.com", http.StatusOK)
	policy.LeaveTypes = append(policy.LeaveTypes[:9], policy.LeaveTypes[10:]...)
	encodedPolicy, errorValue := json.Marshal(policy)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	updated := requestAttendanceLeavePolicy(t, service, http.MethodPut, string(encodedPolicy), "admin@example.com", http.StatusOK)
	dashboard, errorValue := service.readAttendanceLeaveDashboard(
		t.Context(),
		employee,
		time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if dashboard.Summary.UsedMilliDays != 500 {
		t.Fatalf("used leave after policy update = %d, want 500", dashboard.Summary.UsedMilliDays)
	}
	for _, leaveType := range updated.LeaveTypes {
		if leaveType.ID == "reward" {
			if leaveType.IsActive || leaveType.IncludeInSummary {
				t.Fatalf("used removed type was not archived = %+v", leaveType)
			}
			return
		}
	}
	t.Fatal("used removed type was deleted")
}

func TestLeavePolicyAdminAPIAccessByRole(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	requestAttendanceLeavePolicyFromAddress(
		t,
		service,
		http.MethodGet,
		"",
		"staff@example.com",
		"127.0.0.1:1234",
		http.StatusForbidden,
	)

	operationsAdminService := newOperationsAdminAuthorizationTestService(t)
	requestAttendanceLeavePolicy(t, operationsAdminService, http.MethodGet, "", "operator@example.com", http.StatusOK)
}

func requestAttendanceLeavePolicy(
	t *testing.T,
	service *Service,
	method string,
	body string,
	email string,
	expectedStatus int,
) attendanceLeavePolicy {
	t.Helper()
	return requestAttendanceLeavePolicyFromAddress(
		t,
		service,
		method,
		body,
		email,
		"198.51.100.10:443",
		expectedStatus,
	)
}

func requestAttendanceLeavePolicyFromAddress(
	t *testing.T,
	service *Service,
	method string,
	body string,
	email string,
	remoteAddress string,
	expectedStatus int,
) attendanceLeavePolicy {
	t.Helper()
	request := httptest.NewRequest(method, "/admin/api/attendance-leave-policy", strings.NewReader(body))
	request.RemoteAddr = remoteAddress
	request.Header.Set("Cf-Access-Authenticated-User-Email", email)
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != expectedStatus {
		t.Fatalf("%s status = %d body = %s", method, response.Code, response.Body.String())
	}
	if response.Code != http.StatusOK {
		return attendanceLeavePolicy{}
	}
	var policy attendanceLeavePolicy
	if errorValue := json.NewDecoder(response.Body).Decode(&policy); errorValue != nil {
		t.Fatal(errorValue)
	}
	return policy
}
