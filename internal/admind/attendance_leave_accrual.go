package admind

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	attendanceLeaveFullDayMilliDays    = 1000
	attendanceLeaveHalfDayMilliDays    = 500
	attendanceLeaveQuarterDayMilliDays = 250
)

func attendanceLeaveUnitMilliDays(unit string) (int, error) {
	switch unit {
	case "fullDay":
		return attendanceLeaveFullDayMilliDays, nil
	case "halfDay":
		return attendanceLeaveHalfDayMilliDays, nil
	case "quarterDay":
		return attendanceLeaveQuarterDayMilliDays, nil
	default:
		return 0, fmt.Errorf("invalid leave unit %q", unit)
	}
}

func attendanceLeaveUnitWorkMinutes(unit string, netScheduledWorkMinutes int) (int, error) {
	if netScheduledWorkMinutes < 0 {
		return 0, fmt.Errorf("net scheduled work minutes must not be negative")
	}
	milliDays, errorValue := attendanceLeaveUnitMilliDays(unit)
	if errorValue != nil {
		return 0, errorValue
	}
	return netScheduledWorkMinutes * milliDays / attendanceLeaveFullDayMilliDays, nil
}

func attendanceLeaveAccrualsThrough(
	hireDate time.Time,
	asOf time.Time,
	policy attendanceLeavePolicy,
	leaveType attendanceLeaveType,
) ([]attendanceLeaveAccrual, error) {
	accruals := []attendanceLeaveAccrual{}
	switch leaveType.GrantCadence {
	case "annual":
		annualAccruals, errorValue := attendanceLeaveAnnualAccruals(
			hireDate,
			asOf,
			policy,
			leaveType,
		)
		if errorValue != nil {
			return nil, errorValue
		}
		accruals = append(accruals, annualAccruals...)
	case "monthly":
		for completedMonths := 1; ; completedMonths++ {
			grantDate := attendanceLeaveCalendarAnniversary(hireDate, 0, completedMonths)
			if grantDate.After(asOf) {
				break
			}
			accruals = append(accruals, attendanceLeaveAccrual{
				GrantDate:       grantDate.Format(time.DateOnly),
				AmountMilliDays: leaveType.GrantAmountMilliDays,
				Kind:            attendanceLeaveOperationGrant,
				ReferenceID:     "automatic:monthly",
			})
		}
	default:
		return nil, fmt.Errorf("unsupported automatic leave cadence %q", leaveType.GrantCadence)
	}
	effectiveDate, hasEffectiveDate, errorValue := attendanceLeavePolicyEffectiveDate(
		policy,
		hireDate.Location(),
	)
	if errorValue != nil {
		return nil, errorValue
	}
	result := make([]attendanceLeaveAccrual, 0, len(accruals))
	for _, accrual := range accruals {
		if accrual.AmountMilliDays <= 0 {
			continue
		}
		grantDate, parseError := time.ParseInLocation(
			time.DateOnly,
			accrual.GrantDate,
			hireDate.Location(),
		)
		if parseError != nil {
			return nil, parseError
		}
		if hasEffectiveDate && !grantDate.After(effectiveDate) {
			continue
		}
		accrual, errorValue = attendanceLeaveAccrualWithExpiry(
			accrual,
			grantDate,
			policy,
			leaveType,
		)
		if errorValue != nil {
			return nil, errorValue
		}
		result = append(result, accrual)
	}
	sort.SliceStable(result, func(firstIndex int, secondIndex int) bool {
		if result[firstIndex].GrantDate != result[secondIndex].GrantDate {
			return result[firstIndex].GrantDate < result[secondIndex].GrantDate
		}
		return result[firstIndex].Kind < result[secondIndex].Kind
	})
	return result, nil
}

func attendanceLeaveAnnualAccruals(
	hireDate time.Time,
	asOf time.Time,
	policy attendanceLeavePolicy,
	leaveType attendanceLeaveType,
) ([]attendanceLeaveAccrual, error) {
	firstFiscalYearStart, firstFiscalYearEnd, errorValue := attendanceLeaveFiscalYearBounds(
		hireDate,
		policy.FiscalYearStartMonth,
		policy.FiscalYearStartDay,
	)
	if errorValue != nil {
		return nil, errorValue
	}
	accruals := []attendanceLeaveAccrual{{
		GrantDate: hireDate.Format(time.DateOnly),
		AmountMilliDays: attendanceLeaveProportionalGrantMilliDays(
			hireDate,
			firstFiscalYearStart,
			firstFiscalYearEnd,
			leaveType.GrantAmountMilliDays,
		),
		Kind:        attendanceLeaveOperationGrant,
		ReferenceID: "automatic:annual",
	}}
	for fiscalGrantDate := firstFiscalYearEnd; !fiscalGrantDate.After(asOf); fiscalGrantDate = attendanceLeaveNextFiscalYearBoundary(
		fiscalGrantDate,
		policy.FiscalYearStartMonth,
		policy.FiscalYearStartDay,
	) {
		accruals = append(accruals, attendanceLeaveAccrual{
			GrantDate:       fiscalGrantDate.Format(time.DateOnly),
			AmountMilliDays: leaveType.GrantAmountMilliDays,
			Kind:            attendanceLeaveOperationGrant,
			ReferenceID:     "automatic:annual",
		})
	}
	return accruals, nil
}

func attendanceLeavePolicyEffectiveDate(
	policy attendanceLeavePolicy,
	location *time.Location,
) (time.Time, bool, error) {
	updatedAt := strings.TrimSpace(policy.UpdatedAt)
	if updatedAt == "" {
		return time.Time{}, false, nil
	}
	instant, errorValue := time.Parse(time.RFC3339, updatedAt)
	if errorValue != nil {
		return time.Time{}, false, fmt.Errorf("invalid leave policy update time: %w", errorValue)
	}
	local := instant.In(location)
	return attendanceLeaveCalendarDate(
		local.Year(),
		local.Month(),
		local.Day(),
		location,
	), true, nil
}

func attendanceLeaveAccrualWithExpiry(
	accrual attendanceLeaveAccrual,
	grantDate time.Time,
	policy attendanceLeavePolicy,
	leaveType attendanceLeaveType,
) (attendanceLeaveAccrual, error) {
	_, fiscalYearEnd, errorValue := attendanceLeaveFiscalYearBounds(
		grantDate,
		policy.FiscalYearStartMonth,
		policy.FiscalYearStartDay,
	)
	if errorValue != nil {
		return attendanceLeaveAccrual{}, errorValue
	}
	expiryMonths := 0
	if leaveType.ExpiryMonths != nil {
		expiryMonths = *leaveType.ExpiryMonths
	}
	accrual.ExpiresOn, errorValue = attendanceLeaveExpiryDate(
		grantDate,
		fiscalYearEnd,
		leaveType.ExpiryMode,
		expiryMonths,
	)
	if errorValue != nil {
		return attendanceLeaveAccrual{}, errorValue
	}
	return accrual, nil
}

func attendanceLeaveNextAccrual(
	hireDate time.Time,
	asOf time.Time,
	policy attendanceLeavePolicy,
	leaveType attendanceLeaveType,
) (attendanceLeaveAccrual, bool) {
	accruals, errorValue := attendanceLeaveAccrualsThrough(
		hireDate,
		asOf.AddDate(2, 0, 1),
		policy,
		leaveType,
	)
	if errorValue != nil {
		return attendanceLeaveAccrual{}, false
	}
	asOfDate := attendanceLeaveCalendarDate(
		asOf.In(hireDate.Location()).Year(),
		asOf.In(hireDate.Location()).Month(),
		asOf.In(hireDate.Location()).Day(),
		hireDate.Location(),
	)
	for _, accrual := range accruals {
		grantDate, parseError := time.ParseInLocation(
			time.DateOnly,
			accrual.GrantDate,
			hireDate.Location(),
		)
		if parseError == nil && grantDate.After(asOfDate) {
			return accrual, true
		}
	}
	return attendanceLeaveAccrual{}, false
}

func attendanceLeaveBalanceWithSchedule(
	balance attendanceLeaveBalance,
	hireDate time.Time,
	asOf time.Time,
	policy attendanceLeavePolicy,
	leaveType attendanceLeaveType,
) attendanceLeaveBalance {
	next, exists := attendanceLeaveNextAccrual(
		hireDate,
		asOf,
		policy,
		leaveType,
	)
	if !exists {
		return balance
	}
	balance.NextGrantDate = next.GrantDate
	balance.NextGrantMilliDays = next.AmountMilliDays
	return balance
}

func attendanceLeaveCarryoverMilliDays(availableMilliDays int, enabled bool, limitMilliDays *int) int {
	if !enabled || availableMilliDays <= 0 {
		return 0
	}
	if limitMilliDays == nil || *limitMilliDays >= availableMilliDays {
		return availableMilliDays
	}
	if *limitMilliDays <= 0 {
		return 0
	}
	return *limitMilliDays
}
