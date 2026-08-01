package admind

import "time"

func attendanceWorkScheduleFromPolicyRevision(
	revision attendanceWorkPolicyRevision,
) attendanceWorkSchedule {
	return attendanceWorkSchedule{
		Version:              attendanceWorkScheduleVersion,
		WorkMode:             maxWorkScheduleMode(revision.WorkMode),
		WorkingWeekdays:      revision.WorkingWeekdays,
		ScheduledWorkMinutes: revision.DailyTargetMinutes,
		ReferenceStartTime:   revision.ReferenceStartTime,
		FixedStartTime:       revision.FixedStartTime,
		FixedEndTime:         revision.FixedEndTime,
		BreakPeriods:         revision.BreakPeriods,
	}
}

func maxWorkScheduleMode(workMode string) string {
	if workMode == attendanceWorkModeFixed {
		return attendanceWorkModeFixed
	}
	return attendanceWorkModeFlexible
}

func attendanceWorkMinutesExcludingBreaks(
	start time.Time,
	end time.Time,
	dayStart time.Time,
	breakPeriods []attendanceWorkScheduleBreakPeriod,
) int {
	minutes := durationMinutes(end.Sub(start))
	for _, breakPeriod := range breakPeriods {
		breakStartMinute, startError := attendanceWorkScheduleTimeMinutes(breakPeriod.StartTime)
		breakEndMinute, endError := attendanceWorkScheduleTimeMinutes(breakPeriod.EndTime)
		if startError != nil || endError != nil {
			continue
		}
		breakStart := dayStart.Add(time.Duration(breakStartMinute) * time.Minute)
		breakEnd := dayStart.Add(time.Duration(breakEndMinute) * time.Minute)
		minutes -= overlapMinutes(start, end, breakStart, breakEnd)
	}
	return max(0, minutes)
}

func attendanceNightMinutesExcludingBreaks(
	start time.Time,
	end time.Time,
	dayStart time.Time,
	revision attendanceWorkPolicyRevision,
) int {
	nightStartMinute, _ := attendanceWorkScheduleTimeMinutes(revision.NightStartTime)
	nightEndMinute, _ := attendanceWorkScheduleTimeMinutes(revision.NightEndTime)
	intervals := [][2]time.Time{}
	if nightStartMinute < nightEndMinute {
		intervals = append(intervals, [2]time.Time{
			dayStart.Add(time.Duration(nightStartMinute) * time.Minute),
			dayStart.Add(time.Duration(nightEndMinute) * time.Minute),
		})
	} else {
		intervals = append(
			intervals,
			[2]time.Time{dayStart, dayStart.Add(time.Duration(nightEndMinute) * time.Minute)},
			[2]time.Time{
				dayStart.Add(time.Duration(nightStartMinute) * time.Minute),
				dayStart.AddDate(0, 0, 1),
			},
		)
	}
	total := 0
	for _, interval := range intervals {
		overlapStart := latestTime(start, interval[0])
		overlapEnd := earliestTime(end, interval[1])
		if !overlapEnd.After(overlapStart) {
			continue
		}
		total += attendanceWorkMinutesExcludingBreaks(
			overlapStart,
			overlapEnd,
			dayStart,
			revision.BreakPeriods,
		)
	}
	return total
}

func overlapMinutes(
	leftStart time.Time,
	leftEnd time.Time,
	rightStart time.Time,
	rightEnd time.Time,
) int {
	start := latestTime(leftStart, rightStart)
	end := earliestTime(leftEnd, rightEnd)
	if !end.After(start) {
		return 0
	}
	return durationMinutes(end.Sub(start))
}

func durationMinutes(duration time.Duration) int {
	return int(duration / time.Minute)
}

func latestTime(left time.Time, right time.Time) time.Time {
	if left.After(right) {
		return left
	}
	return right
}

func earliestTime(left time.Time, right time.Time) time.Time {
	if left.Before(right) {
		return left
	}
	return right
}
