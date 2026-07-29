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
	attendanceLeaveMaximumAnnualDays   = 25
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

func attendanceLeaveStatutoryAnnualGrantMilliDays(completedYears int) int {
	if completedYears < 1 {
		return 0
	}
	additionalDays := (completedYears - 1) / 2
	days := 15 + additionalDays
	if days > attendanceLeaveMaximumAnnualDays {
		days = attendanceLeaveMaximumAnnualDays
	}
	return days * attendanceLeaveFullDayMilliDays
}

func attendanceLeaveMonthlyAccruals(hireDate time.Time, through time.Time, expiryMonths int) []attendanceLeaveAccrual {
	accruals := []attendanceLeaveAccrual{}
	for completedMonths := 1; completedMonths <= 11; completedMonths++ {
		grantDate := attendanceLeaveCalendarAnniversary(hireDate, 0, completedMonths)
		if grantDate.After(through) {
			break
		}
		expiresOn := ""
		if expiryMonths > 0 {
			expiresOn = attendanceLeaveCalendarAnniversary(grantDate, 0, expiryMonths).Format(time.DateOnly)
		}
		accruals = append(accruals, attendanceLeaveAccrual{
			GrantDate:       grantDate.Format(time.DateOnly),
			AmountMilliDays: attendanceLeaveFullDayMilliDays,
			ExpiresOn:       expiresOn,
			Kind:            attendanceLeaveOperationGrant,
			ReferenceID:     "automatic:statutory:monthly",
		})
	}
	return accruals
}

func attendanceLeaveAccrualsThrough(
	hireDate time.Time,
	asOf time.Time,
	policy attendanceLeavePolicy,
	leaveType attendanceLeaveType,
) ([]attendanceLeaveAccrual, error) {
	accruals := []attendanceLeaveAccrual{}
	switch leaveType.GrantCadence {
	case "statutory":
		accruals = append(
			accruals,
			attendanceLeaveMonthlyAccruals(hireDate, asOf, 0)...,
		)
		statutoryAccruals, errorValue := attendanceLeaveStatutoryFiscalAccruals(
			hireDate,
			asOf,
			policy,
			leaveType,
		)
		if errorValue != nil {
			return nil, errorValue
		}
		accruals = append(accruals, statutoryAccruals...)
	case "annual":
		for completedYears := 1; ; completedYears++ {
			grantDate := attendanceLeaveCalendarAnniversary(hireDate, completedYears, 0)
			if grantDate.After(asOf) {
				break
			}
			accruals = append(accruals, attendanceLeaveAccrual{
				GrantDate:       grantDate.Format(time.DateOnly),
				AmountMilliDays: leaveType.GrantAmountMilliDays,
				Kind:            attendanceLeaveOperationGrant,
				ReferenceID:     "automatic:annual",
			})
		}
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

func attendanceLeaveStatutoryFiscalAccruals(
	hireDate time.Time,
	asOf time.Time,
	policy attendanceLeavePolicy,
	leaveType attendanceLeaveType,
) ([]attendanceLeaveAccrual, error) {
	firstFiscalYearStart, firstFiscalGrantDate, errorValue := attendanceLeaveFiscalYearBounds(
		hireDate,
		policy.FiscalYearStartMonth,
		policy.FiscalYearStartDay,
	)
	if errorValue != nil {
		return nil, errorValue
	}
	accruals := []attendanceLeaveAccrual{}
	for yearOffset := 0; ; yearOffset++ {
		fiscalGrantDate := attendanceLeaveCalendarDate(
			firstFiscalGrantDate.Year()+yearOffset,
			time.Month(policy.FiscalYearStartMonth),
			policy.FiscalYearStartDay,
			hireDate.Location(),
		)
		if fiscalGrantDate.After(asOf) {
			break
		}
		amount := attendanceLeaveStatutoryFiscalGrantMilliDays(
			hireDate,
			firstFiscalYearStart,
			firstFiscalGrantDate,
			fiscalGrantDate,
			leaveType.GrantAmountMilliDays,
		)
		accruals = append(accruals, attendanceLeaveAccrual{
			GrantDate:       fiscalGrantDate.Format(time.DateOnly),
			AmountMilliDays: amount,
			Kind:            attendanceLeaveOperationGrant,
			ReferenceID:     "automatic:statutory:fiscal",
		})
	}
	for completedYears := 1; ; completedYears++ {
		anniversary := attendanceLeaveCalendarAnniversary(hireDate, completedYears, 0)
		if anniversary.After(asOf) {
			break
		}
		fiscalGrantDate, _, boundsError := attendanceLeaveFiscalYearBounds(
			anniversary,
			policy.FiscalYearStartMonth,
			policy.FiscalYearStartDay,
		)
		if boundsError != nil {
			return nil, boundsError
		}
		fiscalGrantAmount := attendanceLeaveStatutoryFiscalGrantMilliDays(
			hireDate,
			firstFiscalYearStart,
			firstFiscalGrantDate,
			fiscalGrantDate,
			leaveType.GrantAmountMilliDays,
		)
		requiredAmount := attendanceLeaveStatutoryAnnualGrantMilliDays(completedYears)
		if requiredAmount < leaveType.GrantAmountMilliDays {
			requiredAmount = leaveType.GrantAmountMilliDays
		}
		correctionAmount := attendanceLeaveLegalCorrectionMilliDays(
			requiredAmount,
			fiscalGrantAmount,
		)
		if correctionAmount == 0 {
			continue
		}
		accruals = append(accruals, attendanceLeaveAccrual{
			GrantDate:       anniversary.Format(time.DateOnly),
			AmountMilliDays: correctionAmount,
			Kind:            attendanceLeaveOperationLegalCorrection,
			ReferenceID:     "automatic:statutory:legal-correction",
		})
	}
	return accruals, nil
}

func attendanceLeaveStatutoryFiscalGrantMilliDays(
	hireDate time.Time,
	firstFiscalYearStart time.Time,
	firstFiscalGrantDate time.Time,
	fiscalGrantDate time.Time,
	configuredAnnualMilliDays int,
) int {
	if fiscalGrantDate.Equal(firstFiscalGrantDate) {
		return attendanceLeaveProportionalGrantMilliDays(
			hireDate,
			firstFiscalYearStart,
			firstFiscalGrantDate,
			configuredAnnualMilliDays,
		)
	}
	completedYears := attendanceLeaveCompletedYears(hireDate, fiscalGrantDate)
	amount := attendanceLeaveStatutoryAnnualGrantMilliDays(completedYears)
	if amount < configuredAnnualMilliDays {
		return configuredAnnualMilliDays
	}
	return amount
}

func attendanceLeaveCompletedYears(hireDate time.Time, date time.Time) int {
	years := date.Year() - hireDate.Year()
	if attendanceLeaveCalendarAnniversary(hireDate, years, 0).After(date) {
		years--
	}
	if years < 0 {
		return 0
	}
	return years
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

func attendanceLeaveLegalCorrectionMilliDays(statutoryRequiredMilliDays int, alreadyGrantedMilliDays int) int {
	if statutoryRequiredMilliDays <= alreadyGrantedMilliDays {
		return 0
	}
	return statutoryRequiredMilliDays - alreadyGrantedMilliDays
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
