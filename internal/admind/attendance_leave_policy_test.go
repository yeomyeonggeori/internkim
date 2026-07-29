package admind

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLeavePolicyDefaults(t *testing.T) {
	policy := defaultAttendanceLeavePolicy()
	if policy.Version != attendanceLeavePolicyVersion || len(policy.LeaveTypes) != 5 {
		t.Fatalf("defaults = %+v", policy)
	}
	annual := policy.LeaveTypes[0]
	if annual.ID != "annual" || !annual.Paid || annual.GrantAmountMilliDays != attendanceAnnualStatutoryGrantMilliDays {
		t.Fatalf("annual default = %+v", annual)
	}
	if errorValue := validateAttendanceLeavePolicy(&policy, nil); errorValue != nil {
		t.Fatalf("validate defaults: %v", errorValue)
	}
}

func TestLeavePolicyValidationRejectsInvalidPolicies(t *testing.T) {
	negativeCarryoverLimit := -1
	tests := []struct {
		name   string
		mutate func(*attendanceLeavePolicy)
	}{
		{name: "annual floor", mutate: func(policy *attendanceLeavePolicy) {
			policy.LeaveTypes[0].GrantAmountMilliDays = attendanceAnnualStatutoryGrantMilliDays - 1
		}},
		{name: "annual inactive", mutate: func(policy *attendanceLeavePolicy) { policy.LeaveTypes[0].IsActive = false }},
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

func TestLeavePolicyValidationRejectsDeletedTypes(t *testing.T) {
	policy := defaultAttendanceLeavePolicy()
	policy.LeaveTypes = policy.LeaveTypes[1:]
	if validateAttendanceLeavePolicy(&policy, nil) == nil {
		t.Fatal("expected system deletion failure")
	}

	existing := defaultAttendanceLeavePolicy()
	customType := attendanceLeaveType{
		ID: "custom-existing", Name: "기존 휴가", BalanceMode: "none", GrantCadence: "none",
		ExpiryMode: "none", AllowedUnits: []string{"fullDay"}, IsActive: true, SortOrder: 5,
	}
	existing.LeaveTypes = append(existing.LeaveTypes, customType)
	policy = defaultAttendanceLeavePolicy()
	if validateAttendanceLeavePolicy(&policy, &existing) == nil {
		t.Fatal("expected custom deletion failure")
	}
}

func TestLeavePolicyAdminAPIRoundtrip(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	policy := requestAttendanceLeavePolicy(t, service, http.MethodGet, "", "admin@example.com", http.StatusOK)
	if len(policy.LeaveTypes) != 5 {
		t.Fatalf("default leave types = %d", len(policy.LeaveTypes))
	}

	policy.LeaveTypes = append(policy.LeaveTypes, attendanceLeaveType{
		Name: "가족돌봄 휴가", BalanceMode: "separate", GrantCadence: "manual",
		ExpiryMode: "none", AllowedUnits: []string{"fullDay"}, IsActive: true, SortOrder: 5,
	})
	encodedPolicy, errorValue := json.Marshal(policy)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	updated := requestAttendanceLeavePolicy(t, service, http.MethodPut, string(encodedPolicy), "admin@example.com", http.StatusOK)
	customType := updated.LeaveTypes[len(updated.LeaveTypes)-1]
	if !strings.HasPrefix(customType.ID, "custom-") || customType.Name != "가족돌봄 휴가" || updated.UpdatedAt == "" {
		t.Fatalf("saved custom type = %+v policy = %+v", customType, updated)
	}

	stored := requestAttendanceLeavePolicy(t, service, http.MethodGet, "", "admin@example.com", http.StatusOK)
	if stored.LeaveTypes[len(stored.LeaveTypes)-1].ID != customType.ID {
		t.Fatalf("stored custom type = %+v", stored.LeaveTypes[len(stored.LeaveTypes)-1])
	}
}

func TestLeavePolicyAdminAPIRejectsUnknownFieldsAndDeletion(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	requestAttendanceLeavePolicy(t, service, http.MethodPut, `{"version":1,"unknown":true}`, "admin@example.com", http.StatusBadRequest)

	policy := requestAttendanceLeavePolicy(t, service, http.MethodGet, "", "admin@example.com", http.StatusOK)
	policy.LeaveTypes = policy.LeaveTypes[1:]
	encodedPolicy, errorValue := json.Marshal(policy)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	requestAttendanceLeavePolicy(t, service, http.MethodPut, string(encodedPolicy), "admin@example.com", http.StatusBadRequest)
}

func TestLeavePolicyAdminAPIRejectsNonAdmins(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	requestAttendanceLeavePolicy(t, service, http.MethodGet, "", "staff@example.com", http.StatusForbidden)

	operationsAdminService := newOperationsAdminAuthorizationTestService(t)
	requestAttendanceLeavePolicy(t, operationsAdminService, http.MethodGet, "", "operator@example.com", http.StatusForbidden)
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
	request := httptest.NewRequest(method, "/admin/api/attendance-leave-policy", strings.NewReader(body))
	request.RemoteAddr = "198.51.100.10:443"
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
