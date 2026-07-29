package admind

import (
	"fmt"
	"time"
)

func attendanceLeaveFiscalYearBounds(
	date time.Time,
	startMonth int,
	startDay int,
) (time.Time, time.Time, error) {
	if !attendanceLeavePolicyFiscalDateIsValid(startMonth, startDay) {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid fiscal year start")
	}
	location := date.Location()
	start := attendanceLeaveCalendarDate(date.Year(), time.Month(startMonth), startDay, location)
	if date.Before(start) {
		start = attendanceLeaveCalendarDate(date.Year()-1, time.Month(startMonth), startDay, location)
	}
	end := attendanceLeaveCalendarDate(start.Year()+1, time.Month(startMonth), startDay, location)
	return start, end, nil
}

func attendanceLeaveProportionalGrantMilliDays(
	hireDate time.Time,
	fiscalYearStart time.Time,
	fiscalYearEnd time.Time,
	annualGrantMilliDays int,
) int {
	if annualGrantMilliDays <= 0 || !hireDate.Before(fiscalYearEnd) {
		return 0
	}
	effectiveStart := hireDate
	if effectiveStart.Before(fiscalYearStart) {
		effectiveStart = fiscalYearStart
	}
	remainingDays := attendanceLeaveCalendarDaysBetween(effectiveStart, fiscalYearEnd)
	fiscalYearDays := attendanceLeaveCalendarDaysBetween(fiscalYearStart, fiscalYearEnd)
	if remainingDays <= 0 || fiscalYearDays <= 0 {
		return 0
	}
	return (annualGrantMilliDays*remainingDays + fiscalYearDays/2) / fiscalYearDays
}

func attendanceLeaveCalendarDaysBetween(start time.Time, end time.Time) int {
	startDate := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	endDate := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	return int(endDate.Sub(startDate).Hours() / 24)
}

func attendanceLeaveExpiryDate(
	grantedOn time.Time,
	fiscalYearEnd time.Time,
	expiryMode string,
	expiryMonths int,
) (string, error) {
	switch expiryMode {
	case "fiscalYearEnd":
		if !grantedOn.Before(fiscalYearEnd) {
			return "", fmt.Errorf("fiscal year end must follow the grant date")
		}
		return fiscalYearEnd.Format(time.DateOnly), nil
	case "monthsAfterGrant":
		if expiryMonths <= 0 {
			return "", fmt.Errorf("expiry months must be positive")
		}
		return attendanceLeaveCalendarAnniversary(grantedOn, 0, expiryMonths).Format(time.DateOnly), nil
	case "none":
		return "", nil
	default:
		return "", fmt.Errorf("invalid expiry mode %q", expiryMode)
	}
}

func attendanceLeaveNextFiscalYearBoundary(
	date time.Time,
	startMonth int,
	startDay int,
) time.Time {
	return attendanceLeaveCalendarDate(
		date.Year()+1,
		time.Month(startMonth),
		startDay,
		date.Location(),
	)
}

func attendanceLeaveCalendarAnniversary(date time.Time, years int, months int) time.Time {
	totalMonths := int(date.Month()) - 1 + months
	targetYear := date.Year() + years + totalMonths/12
	targetMonth := time.Month(totalMonths%12 + 1)
	return attendanceLeaveCalendarDate(targetYear, targetMonth, date.Day(), date.Location())
}

func attendanceLeaveCalendarDate(year int, month time.Month, day int, location *time.Location) time.Time {
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, location).Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(year, month, day, 0, 0, 0, 0, location)
}
