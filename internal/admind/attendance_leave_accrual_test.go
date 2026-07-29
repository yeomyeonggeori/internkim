package admind

import (
	"strings"
	"testing"
	"time"
)

func TestAttendanceLeaveUnitsUseNetScheduledWorkMinutes(t *testing.T) {
	tests := []struct {
		unit            string
		expectedDays    int
		expectedMinutes int
	}{
		{unit: "fullDay", expectedDays: 1000, expectedMinutes: 480},
		{unit: "halfDay", expectedDays: 500, expectedMinutes: 240},
		{unit: "quarterDay", expectedDays: 250, expectedMinutes: 120},
	}
	for _, testCase := range tests {
		t.Run(testCase.unit, func(t *testing.T) {
			days, errorValue := attendanceLeaveUnitMilliDays(testCase.unit)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			minutes, errorValue := attendanceLeaveUnitWorkMinutes(testCase.unit, 480)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if days != testCase.expectedDays || minutes != testCase.expectedMinutes {
				t.Fatalf("unit %s = %d milli-days, %d minutes", testCase.unit, days, minutes)
			}
		})
	}
}

func TestAttendanceLeaveFiscalYearAndProportionalGrant(t *testing.T) {
	location := time.FixedZone("Asia/Seoul", 9*60*60)
	start, end, errorValue := attendanceLeaveFiscalYearBounds(time.Date(2026, 7, 1, 0, 0, 0, 0, location), 1, 1)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if start.Format(time.DateOnly) != "2026-01-01" || end.Format(time.DateOnly) != "2027-01-01" {
		t.Fatalf("fiscal year = %s to %s", start.Format(time.DateOnly), end.Format(time.DateOnly))
	}
	grant := attendanceLeaveProportionalGrantMilliDays(
		time.Date(2026, 7, 1, 0, 0, 0, 0, location),
		start,
		end,
		attendanceAnnualStatutoryGrantMilliDays,
	)
	if grant != 7562 {
		t.Fatalf("proportional grant = %d", grant)
	}
}

func TestAttendanceLeaveCalendarMathClampsMonthEndsAndLeapFiscalYears(t *testing.T) {
	location := time.FixedZone("Asia/Seoul", 9*60*60)
	expiry, errorValue := attendanceLeaveExpiryDate(
		time.Date(2027, 1, 31, 0, 0, 0, 0, location),
		time.Time{},
		"monthsAfterGrant",
		1,
	)
	if errorValue != nil || expiry != "2027-02-28" {
		t.Fatalf("month-end expiry = %q, error = %v", expiry, errorValue)
	}
	start, end, errorValue := attendanceLeaveFiscalYearBounds(
		time.Date(2027, 3, 1, 0, 0, 0, 0, location),
		2,
		29,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if start.Format(time.DateOnly) != "2027-02-28" ||
		end.Format(time.DateOnly) != "2028-02-29" {
		t.Fatalf("leap fiscal year = %s to %s", start, end)
	}
	nextBoundary := attendanceLeaveNextFiscalYearBoundary(end, 2, 29)
	if nextBoundary.Format(time.DateOnly) != "2029-02-28" {
		t.Fatalf("next fiscal boundary = %s", nextBoundary)
	}
	grant := attendanceLeaveProportionalGrantMilliDays(
		time.Date(2027, 3, 1, 0, 0, 0, 0, location),
		start,
		end,
		attendanceAnnualStatutoryGrantMilliDays,
	)
	if grant != 14959 {
		t.Fatalf("leap fiscal proportional grant = %d", grant)
	}
}

func TestAttendanceLeaveStatutoryAccrualsUseFiscalYearAndLegalCorrection(t *testing.T) {
	location := time.FixedZone("Asia/Seoul", 9*60*60)
	hireDate := time.Date(2026, 7, 1, 0, 0, 0, 0, location)
	policy := defaultAttendanceLeavePolicy()
	accruals, errorValue := attendanceLeaveAccrualsThrough(
		hireDate,
		time.Date(2027, 7, 1, 0, 0, 0, 0, location),
		policy,
		policy.LeaveTypes[0],
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var fiscalGrant attendanceLeaveAccrual
	var legalCorrection attendanceLeaveAccrual
	for _, accrual := range accruals {
		switch accrual.ReferenceID {
		case "automatic:statutory:fiscal":
			fiscalGrant = accrual
		case "automatic:statutory:legal-correction":
			legalCorrection = accrual
		}
	}
	if fiscalGrant.GrantDate != "2027-01-01" ||
		fiscalGrant.AmountMilliDays != 7562 {
		t.Fatalf("fiscal grant = %+v", fiscalGrant)
	}
	if legalCorrection.GrantDate != "2027-07-01" ||
		legalCorrection.AmountMilliDays != 7438 ||
		fiscalGrant.AmountMilliDays+legalCorrection.AmountMilliDays != attendanceAnnualStatutoryGrantMilliDays {
		t.Fatalf("legal correction = %+v", legalCorrection)
	}
}

func TestAttendanceLeaveMonthlyAndAnnualAccrualsClampCalendarDates(t *testing.T) {
	hireDate := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	accruals := attendanceLeaveMonthlyAccruals(hireDate, time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC), 12)
	if len(accruals) != 3 {
		t.Fatalf("monthly accrual count = %d", len(accruals))
	}
	expectedDates := []string{"2026-02-28", "2026-03-31", "2026-04-30"}
	for index, expectedDate := range expectedDates {
		if accruals[index].GrantDate != expectedDate || accruals[index].AmountMilliDays != 1000 {
			t.Fatalf("accrual %d = %+v", index, accruals[index])
		}
	}
	policy := defaultAttendanceLeavePolicy()
	next, exists := attendanceLeaveNextAccrual(
		hireDate,
		time.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC),
		policy,
		policy.LeaveTypes[0],
	)
	if !exists || next.GrantDate != "2028-01-01" || next.AmountMilliDays != 15000 {
		t.Fatalf("next annual accrual = %+v, exists=%v", next, exists)
	}
	balance := attendanceLeaveBalance{}
	balance = attendanceLeaveBalanceWithSchedule(
		balance,
		hireDate,
		time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		policy,
		policy.LeaveTypes[0],
	)
	if balance.NextGrantDate != "2026-03-31" || balance.NextGrantMilliDays != 1000 {
		t.Fatalf("scheduled balance = %+v", balance)
	}
}

func TestAttendanceLeaveStatutoryCorrectionExpiryAndCarryover(t *testing.T) {
	if attendanceLeaveStatutoryAnnualGrantMilliDays(1) != 15000 {
		t.Fatal("expected 15 days after one completed year")
	}
	if attendanceLeaveStatutoryAnnualGrantMilliDays(3) != 16000 {
		t.Fatal("expected 16 days after three completed years")
	}
	if attendanceLeaveStatutoryAnnualGrantMilliDays(50) != 25000 {
		t.Fatal("expected statutory maximum of 25 days")
	}
	if attendanceLeaveLegalCorrectionMilliDays(15000, 12000) != 3000 {
		t.Fatal("expected 3 day legal correction")
	}
	expiry, errorValue := attendanceLeaveExpiryDate(
		time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		"fiscalYearEnd",
		0,
	)
	if errorValue != nil || expiry != "2027-01-01" {
		t.Fatalf("expiry = %q, error = %v", expiry, errorValue)
	}
	limit := 3000
	if attendanceLeaveCarryoverMilliDays(5000, true, &limit) != 3000 {
		t.Fatal("expected carryover limit")
	}
}

func TestAttendanceLeaveAccrualSynchronizationPersistsMonthlyGrantsOnce(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	policy := defaultAttendanceLeavePolicy()
	employee := attendanceLeaveEmployee{
		Email:    "staff@example.com",
		UserID:   "user-1",
		HireDate: "2026-01-31",
	}
	asOf := time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)
	if errorValue := service.synchronizeAttendanceLeaveAccruals(
		t.Context(),
		employee,
		policy,
		asOf,
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.synchronizeAttendanceLeaveAccruals(
		t.Context(),
		employee,
		policy,
		asOf,
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	balance, errorValue := service.readAttendanceLeaveBalance(
		t.Context(),
		employee,
		"annual",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.GrantedMilliDays != 3000 ||
		balance.AvailableMilliDays != 3000 ||
		balance.NextExpiryDate != "2027-01-01" {
		t.Fatalf("synchronized balance = %+v", balance)
	}
	entries, errorValue := service.readAttendanceLeaveLedger(
		t.Context(),
		employee,
		"annual",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(entries) != 3 {
		t.Fatalf("ledger entries = %+v", entries)
	}
}

func TestAttendanceLeaveAccrualSynchronizationRecordsFiscalGrantAndLegalCorrection(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	employee := attendanceLeaveEmployee{
		Email:    "staff@example.com",
		UserID:   "user-1",
		HireDate: "2026-07-01",
	}
	if errorValue := service.synchronizeAttendanceLeaveAccruals(
		t.Context(),
		employee,
		defaultAttendanceLeavePolicy(),
		time.Date(2027, 7, 1, 9, 0, 0, 0, time.UTC),
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	entries, errorValue := service.readAttendanceLeaveLedger(
		t.Context(),
		employee,
		"annual",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var fiscalGrantAmount int
	var legalCorrectionAmount int
	for _, entry := range entries {
		switch entry.Kind {
		case attendanceLeaveOperationGrant:
			if strings.HasPrefix(entry.OperationKey, "automatic-leave-fiscal:") {
				fiscalGrantAmount += entry.AmountMilliDays
			}
		case attendanceLeaveOperationLegalCorrection:
			legalCorrectionAmount += entry.AmountMilliDays
		}
	}
	if fiscalGrantAmount != 7562 ||
		legalCorrectionAmount != 7438 ||
		fiscalGrantAmount+legalCorrectionAmount != attendanceAnnualStatutoryGrantMilliDays {
		t.Fatalf(
			"fiscal grant = %d, legal correction = %d, entries = %+v",
			fiscalGrantAmount,
			legalCorrectionAmount,
			entries,
		)
	}
}

func TestAttendanceLeaveAccrualSynchronizationExpiresPriorFiscalYear(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	employee := attendanceLeaveEmployee{
		Email:    "staff@example.com",
		UserID:   "user-1",
		HireDate: "2026-01-01",
	}
	if errorValue := service.synchronizeAttendanceLeaveAccruals(
		t.Context(),
		employee,
		defaultAttendanceLeavePolicy(),
		time.Date(2027, 1, 2, 9, 0, 0, 0, time.UTC),
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	balance, errorValue := service.readAttendanceLeaveBalance(
		t.Context(),
		employee,
		"annual",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.GrantedMilliDays != 26000 ||
		balance.AvailableMilliDays != 15000 ||
		balance.ExpiredMilliDays != 11000 ||
		balance.NextExpiryDate != "2028-01-01" {
		t.Fatalf("fiscal year balance = %+v", balance)
	}
}

func TestAttendanceLeaveAccrualSynchronizationCarriesLimitedBalance(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	policy := defaultAttendanceLeavePolicy()
	carryoverLimit := 3000
	policy.LeaveTypes[0].CarryoverEnabled = true
	policy.LeaveTypes[0].CarryoverLimitMilliDays = &carryoverLimit
	employee := attendanceLeaveEmployee{
		Email:    "staff@example.com",
		UserID:   "user-1",
		HireDate: "2026-01-01",
	}
	asOf := time.Date(2027, 1, 2, 9, 0, 0, 0, time.UTC)
	if errorValue := service.synchronizeAttendanceLeaveAccruals(
		t.Context(),
		employee,
		policy,
		asOf,
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.synchronizeAttendanceLeaveAccruals(
		t.Context(),
		employee,
		policy,
		asOf,
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	balance, errorValue := service.readAttendanceLeaveBalance(
		t.Context(),
		employee,
		"annual",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.GrantedMilliDays != 26000 ||
		balance.AvailableMilliDays != 18000 ||
		balance.ExpiredMilliDays != 8000 ||
		balance.NextExpiryDate != "2028-01-01" {
		t.Fatalf("carryover balance = %+v", balance)
	}
}

func TestAttendanceLeaveAccrualSynchronizationKeepsExistingGrantsAfterPolicyChange(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	employee := attendanceLeaveEmployee{
		Email:    "staff@example.com",
		UserID:   "user-1",
		HireDate: "2026-01-01",
	}
	initialPolicy := defaultAttendanceLeavePolicy()
	if errorValue := service.synchronizeAttendanceLeaveAccruals(
		t.Context(),
		employee,
		initialPolicy,
		time.Date(2027, 1, 2, 9, 0, 0, 0, time.UTC),
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	updatedPolicy := defaultAttendanceLeavePolicy()
	updatedPolicy.LeaveTypes[0].GrantAmountMilliDays = 16000
	updatedPolicy.UpdatedAt = "2027-01-02T00:00:00Z"
	if errorValue := service.synchronizeAttendanceLeaveAccruals(
		t.Context(),
		employee,
		updatedPolicy,
		time.Date(2027, 1, 3, 9, 0, 0, 0, time.UTC),
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	balance, errorValue := service.readAttendanceLeaveBalance(
		t.Context(),
		employee,
		"annual",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.GrantedMilliDays != 26000 {
		t.Fatalf("policy change rewrote existing grants: %+v", balance)
	}
}

func TestAttendanceLeaveAccrualSynchronizationStopsInactiveTypeAndExpiresExistingGrants(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	expiryMonths := 2
	customType := attendanceLeaveType{
		ID:                   "custom-monthly",
		Name:                 "월 정기 휴가",
		Paid:                 true,
		BalanceMode:          "separate",
		GrantCadence:         "monthly",
		GrantAmountMilliDays: 1000,
		ExpiryMode:           "monthsAfterGrant",
		ExpiryMonths:         &expiryMonths,
		AllowedUnits:         []string{"fullDay"},
		IsActive:             true,
		SortOrder:            5,
	}
	initialPolicy := defaultAttendanceLeavePolicy()
	initialPolicy.LeaveTypes = append(initialPolicy.LeaveTypes, customType)
	employee := attendanceLeaveEmployee{
		Email:    "staff@example.com",
		UserID:   "user-1",
		HireDate: "2026-01-01",
	}
	if errorValue := service.synchronizeAttendanceLeaveAccruals(
		t.Context(),
		employee,
		initialPolicy,
		time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC),
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	inactivePolicy := initialPolicy
	inactivePolicy.LeaveTypes = append([]attendanceLeaveType{}, initialPolicy.LeaveTypes...)
	inactivePolicy.LeaveTypes[len(inactivePolicy.LeaveTypes)-1].IsActive = false
	inactivePolicy.UpdatedAt = "2026-03-03T00:00:00Z"
	asOf := time.Date(2026, 4, 2, 9, 0, 0, 0, time.UTC)
	if errorValue := service.synchronizeAttendanceLeaveAccruals(
		t.Context(),
		employee,
		inactivePolicy,
		asOf,
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	balance, errorValue := service.readAttendanceLeaveBalance(
		t.Context(),
		employee,
		customType.ID,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.GrantedMilliDays != 2000 ||
		balance.ExpiredMilliDays != 1000 ||
		balance.AvailableMilliDays != 1000 {
		t.Fatalf("inactive type balance = %+v", balance)
	}
	entries, errorValue := service.readAttendanceLeaveLedger(
		t.Context(),
		employee,
		customType.ID,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(entries) != 3 {
		t.Fatalf("inactive type ledger entries = %+v", entries)
	}
	if errorValue := service.writeAttendanceLeavePolicy(t.Context(), inactivePolicy); errorValue != nil {
		t.Fatal(errorValue)
	}
	dashboard, errorValue := service.readAttendanceLeaveDashboard(
		t.Context(),
		employee,
		asOf,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if dashboard.Summary.AvailableMilliDays != 3000 {
		t.Fatalf("dashboard summary = %+v", dashboard.Summary)
	}
}

func TestAttendanceLeaveAccrualSynchronizationBackfillsLateRegisteredEmployee(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	policy := defaultAttendanceLeavePolicy()
	policy.UpdatedAt = "2026-07-29T00:00:00Z"
	employee := attendanceLeaveEmployee{
		Email:    "late-registered@example.com",
		UserID:   "user-late",
		HireDate: "2026-01-01",
	}
	if errorValue := service.synchronizeAttendanceLeaveAccruals(
		t.Context(),
		employee,
		policy,
		time.Date(2026, 7, 30, 9, 0, 0, 0, time.UTC),
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	balance, errorValue := service.readAttendanceLeaveBalance(
		t.Context(),
		employee,
		"annual",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if balance.GrantedMilliDays != 6000 ||
		balance.AvailableMilliDays != 6000 {
		t.Fatalf("late registered employee balance = %+v", balance)
	}
}
