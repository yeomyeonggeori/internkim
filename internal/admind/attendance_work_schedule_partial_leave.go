package admind

import (
	"fmt"
	"time"
)

func attendanceWorkScheduleMinutesForUnit(schedule attendanceWorkSchedule, unit string) (int, error) {
	switch unit {
	case attendanceWorkScheduleFullDay:
		return schedule.ScheduledWorkMinutes, nil
	case attendanceWorkScheduleHalfDay:
		return (schedule.ScheduledWorkMinutes + 1) / 2, nil
	case attendanceWorkScheduleQuarterDay:
		return (schedule.ScheduledWorkMinutes + 3) / 4, nil
	default:
		return 0, fmt.Errorf("unsupported work schedule unit %q", unit)
	}
}

func attendanceWorkSchedulePartialLeaveStartTime(schedule attendanceWorkSchedule) string {
	if schedule.WorkMode == attendanceWorkModeFixed {
		return schedule.FixedStartTime
	}
	return schedule.ReferenceStartTime
}

func calculateAttendanceWorkScheduleEndTime(
	schedule attendanceWorkSchedule,
	date string,
	startTime string,
	unit string,
	location *time.Location,
) (time.Time, error) {
	normalizedSchedule := schedule
	if errorValue := validateAndNormalizeAttendanceWorkSchedule(&normalizedSchedule); errorValue != nil {
		return time.Time{}, errorValue
	}
	localDate, errorValue := attendanceWorkScheduleDate(date, location)
	if errorValue != nil {
		return time.Time{}, errorValue
	}
	startMinute, errorValue := attendanceWorkScheduleTimeMinutes(startTime)
	if errorValue != nil {
		return time.Time{}, errorValue
	}
	if _, errorValue = attendanceWorkScheduleWallTime(localDate, startMinute, location); errorValue != nil {
		return time.Time{}, errorValue
	}
	requiredMinutes, errorValue := attendanceWorkScheduleMinutesForUnit(normalizedSchedule, unit)
	if errorValue != nil {
		return time.Time{}, errorValue
	}
	endMinute, errorValue := calculateAttendanceWorkScheduleEndMinute(
		startMinute,
		requiredMinutes,
		normalizedSchedule.BreakPeriods,
	)
	if errorValue != nil {
		return time.Time{}, errorValue
	}
	return attendanceWorkScheduleWallTime(localDate, endMinute, location)
}

func calculateAttendanceWorkScheduleEndMinute(
	startMinute int,
	requiredMinutes int,
	breakPeriods []attendanceWorkScheduleBreakPeriod,
) (int, error) {
	currentMinute := startMinute
	remainingMinutes := requiredMinutes
	for remainingMinutes > 0 {
		currentMinute = attendanceWorkScheduleMinuteAfterBreak(currentMinute, breakPeriods)
		nextBreakStart, nextBreakEnd := attendanceWorkScheduleNextBreak(currentMinute, breakPeriods)
		availableMinutes := attendanceWorkScheduleMinutesPerDay - currentMinute
		if nextBreakStart >= 0 {
			availableMinutes = nextBreakStart - currentMinute
		}
		if remainingMinutes <= availableMinutes {
			endMinute := currentMinute + remainingMinutes
			if endMinute >= attendanceWorkScheduleMinutesPerDay {
				return 0, fmt.Errorf("calculated end time crosses the day boundary")
			}
			return endMinute, nil
		}
		remainingMinutes -= availableMinutes
		if nextBreakStart < 0 {
			return 0, fmt.Errorf("calculated end time crosses the day boundary")
		}
		currentMinute = nextBreakEnd
	}
	return currentMinute, nil
}

func attendanceWorkScheduleMinuteAfterBreak(
	currentMinute int,
	breakPeriods []attendanceWorkScheduleBreakPeriod,
) int {
	for _, breakPeriod := range breakPeriods {
		breakStartMinute, _ := attendanceWorkScheduleTimeMinutes(breakPeriod.StartTime)
		breakEndMinute, _ := attendanceWorkScheduleTimeMinutes(breakPeriod.EndTime)
		if currentMinute >= breakStartMinute && currentMinute < breakEndMinute {
			currentMinute = breakEndMinute
		}
	}
	return currentMinute
}

func attendanceWorkScheduleNextBreak(
	currentMinute int,
	breakPeriods []attendanceWorkScheduleBreakPeriod,
) (int, int) {
	for _, breakPeriod := range breakPeriods {
		breakStartMinute, _ := attendanceWorkScheduleTimeMinutes(breakPeriod.StartTime)
		if breakStartMinute > currentMinute {
			breakEndMinute, _ := attendanceWorkScheduleTimeMinutes(breakPeriod.EndTime)
			return breakStartMinute, breakEndMinute
		}
	}
	return -1, -1
}
