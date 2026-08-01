package admind

import (
	"sort"
	"time"
)

func attendanceWorkCompliance(
	date time.Time,
	revision attendanceWorkPolicyRevision,
	workingDate bool,
	workSegments []attendanceWorkSegment,
	leaveIntervals []attendanceWorkSegment,
	location *time.Location,
	now time.Time,
) (bool, bool, bool) {
	if !workingDate || revision.WorkMode == attendanceWorkModeAutonomous {
		return false, false, false
	}
	coverage := append(append([]attendanceWorkSegment{}, workSegments...), leaveIntervals...)
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, location)
	if revision.WorkMode == attendanceWorkModeFlexible {
		if !revision.CoreTimeEnabled {
			return false, false, false
		}
		coreStartMinute, startError := attendanceWorkScheduleTimeMinutes(revision.CoreStartTime)
		coreEndMinute, endError := attendanceWorkScheduleTimeMinutes(revision.CoreEndTime)
		if startError != nil || endError != nil {
			return false, false, false
		}
		coreStart := dayStart.Add(time.Duration(coreStartMinute) * time.Minute)
		coreEnd := dayStart.Add(time.Duration(coreEndMinute) * time.Minute)
		coreTimeMissed := false
		if now.After(coreEnd) {
			for _, requiredInterval := range attendanceIntervalsExcludingBreaks(
				coreStart,
				coreEnd,
				dayStart,
				revision.BreakPeriods,
			) {
				if !attendanceIntervalCovered(requiredInterval.Start, requiredInterval.End, coverage) {
					coreTimeMissed = true
					break
				}
			}
		}
		return coreTimeMissed, false, false
	}
	fixedStartMinute, startError := attendanceWorkScheduleTimeMinutes(revision.FixedStartTime)
	fixedEndMinute, endError := attendanceWorkScheduleTimeMinutes(revision.FixedEndTime)
	if startError != nil || endError != nil {
		return false, false, false
	}
	fixedStart := dayStart.Add(time.Duration(fixedStartMinute) * time.Minute)
	fixedEnd := dayStart.Add(time.Duration(fixedEndMinute) * time.Minute)
	late := now.After(fixedStart) && !attendanceInstantCovered(fixedStart, coverage)
	earlyLeave := now.After(fixedEnd) && !attendanceInstantCovered(fixedEnd.Add(-time.Nanosecond), coverage)
	return false, late, earlyLeave
}

func attendanceIntervalsExcludingBreaks(
	start time.Time,
	end time.Time,
	dayStart time.Time,
	breakPeriods []attendanceWorkScheduleBreakPeriod,
) []attendanceWorkSegment {
	intervals := []attendanceWorkSegment{{Start: start, End: end}}
	for _, breakPeriod := range breakPeriods {
		breakStartMinute, startError := attendanceWorkScheduleTimeMinutes(breakPeriod.StartTime)
		breakEndMinute, endError := attendanceWorkScheduleTimeMinutes(breakPeriod.EndTime)
		if startError != nil || endError != nil {
			continue
		}
		breakStart := dayStart.Add(time.Duration(breakStartMinute) * time.Minute)
		breakEnd := dayStart.Add(time.Duration(breakEndMinute) * time.Minute)
		next := make([]attendanceWorkSegment, 0, len(intervals)+1)
		for _, interval := range intervals {
			if !breakEnd.After(interval.Start) || !interval.End.After(breakStart) {
				next = append(next, interval)
				continue
			}
			if breakStart.After(interval.Start) {
				next = append(next, attendanceWorkSegment{
					Start: interval.Start,
					End:   earliestTime(interval.End, breakStart),
				})
			}
			if interval.End.After(breakEnd) {
				next = append(next, attendanceWorkSegment{
					Start: latestTime(interval.Start, breakEnd),
					End:   interval.End,
				})
			}
		}
		intervals = next
	}
	return intervals
}

func attendanceIntervalCovered(
	requiredStart time.Time,
	requiredEnd time.Time,
	intervals []attendanceWorkSegment,
) bool {
	if !requiredEnd.After(requiredStart) {
		return true
	}
	sortedIntervals := append([]attendanceWorkSegment{}, intervals...)
	sort.Slice(sortedIntervals, func(left int, right int) bool {
		return sortedIntervals[left].Start.Before(sortedIntervals[right].Start)
	})
	coveredUntil := requiredStart
	for _, interval := range sortedIntervals {
		if !interval.End.After(coveredUntil) || interval.Start.After(coveredUntil) {
			continue
		}
		if interval.End.After(coveredUntil) {
			coveredUntil = interval.End
		}
		if !coveredUntil.Before(requiredEnd) {
			return true
		}
	}
	return false
}

func attendanceInstantCovered(instant time.Time, intervals []attendanceWorkSegment) bool {
	for _, interval := range intervals {
		if !instant.Before(interval.Start) && instant.Before(interval.End) {
			return true
		}
	}
	return false
}
