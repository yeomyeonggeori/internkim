package admind

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

func validateAndNormalizeAttendanceWorkSchedule(schedule *attendanceWorkSchedule) error {
	schedule.WorkMode = strings.TrimSpace(schedule.WorkMode)
	schedule.ReferenceStartTime = strings.TrimSpace(schedule.ReferenceStartTime)
	schedule.FixedStartTime = strings.TrimSpace(schedule.FixedStartTime)
	schedule.FixedEndTime = strings.TrimSpace(schedule.FixedEndTime)
	if schedule.Version != attendanceWorkScheduleVersion {
		return fmt.Errorf("version must be %d", attendanceWorkScheduleVersion)
	}
	if schedule.WorkMode != attendanceWorkModeFixed && schedule.WorkMode != attendanceWorkModeFlexible {
		return fmt.Errorf("workMode must be fixed or flexible")
	}
	if schedule.ScheduledWorkMinutes <= 0 || schedule.ScheduledWorkMinutes > attendanceWorkScheduleMaximumMinutes {
		return fmt.Errorf("scheduledWorkMinutes must be between 1 and %d", attendanceWorkScheduleMaximumMinutes)
	}
	if errorValue := normalizeAttendanceWorkScheduleWeekdays(schedule); errorValue != nil {
		return errorValue
	}
	if errorValue := normalizeAttendanceWorkScheduleBreakPeriods(schedule); errorValue != nil {
		return errorValue
	}
	if errorValue := normalizeAttendanceWorkScheduleHolidays(schedule); errorValue != nil {
		return errorValue
	}
	return validateAttendanceWorkScheduleMode(schedule)
}

func normalizeAttendanceWorkScheduleWeekdays(schedule *attendanceWorkSchedule) error {
	if len(schedule.WorkingWeekdays) == 0 {
		return fmt.Errorf("workingWeekdays must not be empty")
	}
	weekdays := append([]int(nil), schedule.WorkingWeekdays...)
	sort.Ints(weekdays)
	for index, weekday := range weekdays {
		if weekday < 1 || weekday > 7 {
			return fmt.Errorf("workingWeekdays must contain values from 1 to 7")
		}
		if index > 0 && weekdays[index-1] == weekday {
			return fmt.Errorf("workingWeekdays must be unique")
		}
	}
	schedule.WorkingWeekdays = weekdays
	return nil
}

func normalizeAttendanceWorkScheduleBreakPeriods(schedule *attendanceWorkSchedule) error {
	breakPeriods := make([]attendanceWorkScheduleBreakPeriod, len(schedule.BreakPeriods))
	copy(breakPeriods, schedule.BreakPeriods)
	for index := range breakPeriods {
		breakPeriods[index].StartTime = strings.TrimSpace(breakPeriods[index].StartTime)
		breakPeriods[index].EndTime = strings.TrimSpace(breakPeriods[index].EndTime)
		startMinute, startError := attendanceWorkScheduleTimeMinutes(breakPeriods[index].StartTime)
		endMinute, endError := attendanceWorkScheduleTimeMinutes(breakPeriods[index].EndTime)
		if startError != nil || endError != nil || startMinute >= endMinute {
			return fmt.Errorf("invalid break period")
		}
	}
	sort.Slice(breakPeriods, func(left int, right int) bool {
		leftStart, _ := attendanceWorkScheduleTimeMinutes(breakPeriods[left].StartTime)
		rightStart, _ := attendanceWorkScheduleTimeMinutes(breakPeriods[right].StartTime)
		return leftStart < rightStart
	})
	for index := 1; index < len(breakPeriods); index++ {
		previousEnd, _ := attendanceWorkScheduleTimeMinutes(breakPeriods[index-1].EndTime)
		currentStart, _ := attendanceWorkScheduleTimeMinutes(breakPeriods[index].StartTime)
		if previousEnd > currentStart {
			return fmt.Errorf("breakPeriods must not overlap")
		}
	}
	schedule.BreakPeriods = breakPeriods
	return nil
}

func normalizeAttendanceWorkScheduleHolidays(schedule *attendanceWorkSchedule) error {
	holidays := make([]attendanceWorkScheduleHoliday, len(schedule.Holidays))
	copy(holidays, schedule.Holidays)
	seenDates := make(map[string]struct{}, len(holidays))
	for index := range holidays {
		holidays[index].Date = strings.TrimSpace(holidays[index].Date)
		holidays[index].Name = strings.TrimSpace(holidays[index].Name)
		if _, errorValue := attendanceWorkScheduleDate(holidays[index].Date, time.UTC); errorValue != nil {
			return fmt.Errorf("invalid holiday date: %w", errorValue)
		}
		if holidays[index].Name == "" {
			return fmt.Errorf("holiday name must not be empty")
		}
		if _, exists := seenDates[holidays[index].Date]; exists {
			return fmt.Errorf("holiday dates must be unique")
		}
		seenDates[holidays[index].Date] = struct{}{}
	}
	sort.Slice(holidays, func(left int, right int) bool {
		return holidays[left].Date < holidays[right].Date
	})
	schedule.Holidays = holidays
	return nil
}

func validateAttendanceWorkScheduleMode(schedule *attendanceWorkSchedule) error {
	if schedule.WorkMode == attendanceWorkModeFlexible {
		return validateFlexibleAttendanceWorkSchedule(schedule)
	}
	fixedStartMinute, startError := attendanceWorkScheduleTimeMinutes(schedule.FixedStartTime)
	fixedEndMinute, endError := attendanceWorkScheduleTimeMinutes(schedule.FixedEndTime)
	if startError != nil || endError != nil || fixedStartMinute >= fixedEndMinute {
		return fmt.Errorf("fixed work time must be a valid same-day range")
	}
	workingMinutes := fixedEndMinute - fixedStartMinute
	for _, breakPeriod := range schedule.BreakPeriods {
		breakStartMinute, _ := attendanceWorkScheduleTimeMinutes(breakPeriod.StartTime)
		breakEndMinute, _ := attendanceWorkScheduleTimeMinutes(breakPeriod.EndTime)
		workingMinutes -= attendanceWorkScheduleRangeOverlap(
			fixedStartMinute,
			fixedEndMinute,
			breakStartMinute,
			breakEndMinute,
		)
	}
	if workingMinutes != schedule.ScheduledWorkMinutes {
		return fmt.Errorf("fixed work time excluding breaks must equal scheduledWorkMinutes")
	}
	schedule.ReferenceStartTime = schedule.FixedStartTime
	return nil
}

func validateFlexibleAttendanceWorkSchedule(schedule *attendanceWorkSchedule) error {
	schedule.FixedStartTime = ""
	schedule.FixedEndTime = ""
	referenceStartMinute, errorValue := attendanceWorkScheduleTimeMinutes(schedule.ReferenceStartTime)
	if errorValue != nil {
		return fmt.Errorf("invalid referenceStartTime: %w", errorValue)
	}
	if _, errorValue = calculateAttendanceWorkScheduleEndMinute(
		referenceStartMinute,
		schedule.ScheduledWorkMinutes,
		schedule.BreakPeriods,
	); errorValue != nil {
		return fmt.Errorf("flexible full-day schedule is invalid: %w", errorValue)
	}
	return nil
}

func attendanceWorkScheduleRangeOverlap(leftStart int, leftEnd int, rightStart int, rightEnd int) int {
	overlapStart := max(leftStart, rightStart)
	overlapEnd := min(leftEnd, rightEnd)
	return max(0, overlapEnd-overlapStart)
}

func attendanceWorkScheduleTimeMinutes(value string) (int, error) {
	if len(value) != len("00:00") || value[2] != ':' {
		return 0, fmt.Errorf("time must use HH:mm")
	}
	for _, index := range []int{0, 1, 3, 4} {
		if value[index] < '0' || value[index] > '9' {
			return 0, fmt.Errorf("time must use HH:mm")
		}
	}
	hour, hourError := strconv.Atoi(value[:2])
	minute, minuteError := strconv.Atoi(value[3:])
	if hourError != nil || minuteError != nil || hour > 23 || minute > 59 {
		return 0, fmt.Errorf("time must use HH:mm")
	}
	return hour*60 + minute, nil
}
