package admind

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type attendanceWorkSegment struct {
	Start time.Time
	End   time.Time
}

type attendanceWorkRecords struct {
	CompletedSegments []attendanceWorkSegment
	Provisional       *attendanceWorkSegment
	IncompleteDates   map[string]struct{}
}

func calculateAttendanceWorkStatus(
	email string,
	displayName string,
	startDate string,
	endDate string,
	events []attendanceEvent,
	leaveOccurrences []attendancePaidLeaveOccurrence,
	policy attendanceWorkPolicy,
	holidayDates map[string]struct{},
	location *time.Location,
	now time.Time,
) (attendanceWorkStatus, error) {
	if location == nil {
		return attendanceWorkStatus{}, fmt.Errorf("time zone is required")
	}
	start, errorValue := attendanceWorkScheduleDate(startDate, location)
	if errorValue != nil {
		return attendanceWorkStatus{}, errorValue
	}
	end, errorValue := attendanceWorkScheduleDate(endDate, location)
	if errorValue != nil {
		return attendanceWorkStatus{}, errorValue
	}
	if end.Before(start) {
		return attendanceWorkStatus{}, fmt.Errorf("period end must not be before period start")
	}
	if errorValue = validateAndNormalizeAttendanceWorkPolicy(&policy); errorValue != nil {
		return attendanceWorkStatus{}, errorValue
	}
	records := attendanceWorkRecordsForEmail(events, email, now, location)
	leaveByDate := attendanceLeaveByDate(leaveOccurrences, email)
	status := attendanceWorkStatus{
		Email:       strings.ToLower(strings.TrimSpace(email)),
		DisplayName: displayName,
		PeriodStart: startDate,
		PeriodEnd:   endDate,
		Days:        []attendanceWorkDayStatus{},
	}
	for date := start; !date.After(end); date = date.AddDate(0, 0, 1) {
		dateValue := date.Format(time.DateOnly)
		revision, revisionError := attendanceWorkPolicyRevisionForDate(policy, dateValue)
		if revisionError != nil {
			return attendanceWorkStatus{}, revisionError
		}
		day := calculateAttendanceWorkDayStatus(
			date,
			revision,
			records,
			leaveByDate[dateValue],
			holidayDates,
			location,
			now,
		)
		status.Days = append(status.Days, day)
		status.HasBaseline = status.HasBaseline || day.HasBaseline
		status.TargetMinutes += day.TargetMinutes
		status.ActualMinutes += day.ActualMinutes
		status.ProvisionalMinutes += day.ProvisionalMinutes
		status.PaidLeaveMinutes += day.PaidLeaveMinutes
		status.CreditedLeaveMinutes += day.CreditedLeaveMinutes
		status.FulfilledMinutes += day.FulfilledMinutes
		status.NightMinutes += day.NightMinutes
		status.IsWorking = status.IsWorking || day.IsWorking
		status.NeedsReview = status.NeedsReview || day.NeedsReview
		status.CoreTimeMissed = status.CoreTimeMissed || day.CoreTimeMissed
		status.Late = status.Late || day.Late
		status.EarlyLeave = status.EarlyLeave || day.EarlyLeave
		status.HasLeaveWorkOverlap = status.HasLeaveWorkOverlap || day.HasLeaveWorkOverlap
		status.HasIncompleteRecords = status.HasIncompleteRecords || day.HasIncompleteWorkRecord
	}
	currentRevision, errorValue := attendanceWorkPolicyRevisionForDate(policy, endDate)
	if errorValue != nil {
		return attendanceWorkStatus{}, errorValue
	}
	status.WorkMode = currentRevision.WorkMode
	if status.HasBaseline {
		status.FulfilledMinutes = min(status.TargetMinutes, status.FulfilledMinutes)
		status.RemainingMinutes = max(0, status.TargetMinutes-status.FulfilledMinutes)
		status.DifferenceMinutes = status.FulfilledMinutes - status.TargetMinutes
		status.OvertimeMinutes = max(0, status.ActualMinutes-status.TargetMinutes)
		status.Status = attendanceWorkStatusLabel(status)
	} else {
		status.Status = attendanceWorkStatusLabel(status)
	}
	return status, nil
}

func calculateAttendanceWorkDayStatus(
	date time.Time,
	revision attendanceWorkPolicyRevision,
	records attendanceWorkRecords,
	leaveOccurrences []attendancePaidLeaveOccurrence,
	holidayDates map[string]struct{},
	location *time.Location,
	now time.Time,
) attendanceWorkDayStatus {
	dateValue := date.Format(time.DateOnly)
	workingDate, _ := attendanceWorkScheduleIsWorkingDate(
		attendanceWorkScheduleFromPolicyRevision(revision),
		dateValue,
		holidayDates,
	)
	hasBaseline := revision.WorkMode != attendanceWorkModeAutonomous
	targetMinutes := 0
	if hasBaseline && workingDate {
		targetMinutes = revision.DailyTargetMinutes
	}
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, location)
	dayEnd := dayStart.AddDate(0, 0, 1)
	actualMinutes := 0
	nightMinutes := 0
	daySegments := make([]attendanceWorkSegment, 0)
	workSegmentDetails := make([]attendanceWorkIntervalDetail, 0)
	for _, segment := range records.CompletedSegments {
		start := latestTime(segment.Start, dayStart)
		end := earliestTime(segment.End, dayEnd)
		if !end.After(start) {
			continue
		}
		daySegments = append(daySegments, attendanceWorkSegment{Start: start, End: end})
		workSegmentDetails = append(workSegmentDetails, attendanceWorkIntervalDetail{
			StartTime: start.In(location).Format("15:04"),
			EndTime:   end.In(location).Format("15:04"),
		})
		actualMinutes += attendanceWorkMinutesExcludingBreaks(
			start,
			end,
			dayStart,
			revision.BreakPeriods,
		)
		nightMinutes += attendanceNightMinutesExcludingBreaks(
			start,
			end,
			dayStart,
			revision,
		)
	}
	provisionalMinutes := 0
	isWorking := false
	if records.Provisional != nil {
		start := latestTime(records.Provisional.Start, dayStart)
		end := earliestTime(records.Provisional.End, dayEnd)
		if end.After(start) {
			isWorking = true
			provisionalMinutes = attendanceWorkMinutesExcludingBreaks(
				start,
				end,
				dayStart,
				revision.BreakPeriods,
			)
			daySegments = append(daySegments, attendanceWorkSegment{Start: start, End: end})
			workSegmentDetails = append(workSegmentDetails, attendanceWorkIntervalDetail{
				StartTime:   start.In(location).Format("15:04"),
				EndTime:     end.In(location).Format("15:04"),
				Provisional: true,
			})
		}
	}
	leaveIntervals := attendanceLeaveIntervals(date, leaveOccurrences, location)
	leaveSegmentDetails := make([]attendanceLeaveIntervalDetail, 0, len(leaveOccurrences))
	for _, occurrence := range leaveOccurrences {
		leaveSegmentDetails = append(leaveSegmentDetails, attendanceLeaveIntervalDetail{
			StartTime: occurrence.StartTime,
			EndTime:   occurrence.EndTime,
			Paid:      occurrence.Paid,
		})
	}
	paidLeaveMinutes := 0
	if targetMinutes > 0 {
		for _, occurrence := range leaveOccurrences {
			if occurrence.Paid {
				paidLeaveMinutes += (revision.DailyTargetMinutes*occurrence.DeductionMilliDays + 999) / 1000
			}
		}
	}
	creditedLeaveMinutes := min(paidLeaveMinutes, max(0, targetMinutes-min(actualMinutes, targetMinutes)))
	fulfilledMinutes := actualMinutes + creditedLeaveMinutes
	if hasBaseline {
		fulfilledMinutes = min(targetMinutes, fulfilledMinutes)
	}
	remainingMinutes := 0
	overtimeMinutes := 0
	differenceMinutes := 0
	status := "actualOnly"
	if hasBaseline {
		remainingMinutes = max(0, targetMinutes-fulfilledMinutes)
		overtimeMinutes = max(0, actualMinutes-targetMinutes)
		differenceMinutes = fulfilledMinutes - targetMinutes
	}
	_, hasIncompleteWorkRecord := records.IncompleteDates[dateValue]
	hasLeaveWorkOverlap := attendanceSegmentsOverlap(daySegments, leaveIntervals)
	coreTimeMissed, late, earlyLeave := attendanceWorkCompliance(
		date,
		revision,
		workingDate,
		daySegments,
		leaveIntervals,
		location,
		now,
	)
	dayStatus := attendanceWorkDayStatus{
		Date:                    dateValue,
		WorkMode:                revision.WorkMode,
		HasBaseline:             hasBaseline,
		TargetMinutes:           targetMinutes,
		ActualMinutes:           actualMinutes,
		ProvisionalMinutes:      provisionalMinutes,
		PaidLeaveMinutes:        paidLeaveMinutes,
		CreditedLeaveMinutes:    creditedLeaveMinutes,
		FulfilledMinutes:        fulfilledMinutes,
		DifferenceMinutes:       differenceMinutes,
		RemainingMinutes:        remainingMinutes,
		OvertimeMinutes:         overtimeMinutes,
		NightMinutes:            nightMinutes,
		IsWorking:               isWorking,
		NeedsReview:             hasIncompleteWorkRecord || hasLeaveWorkOverlap,
		CoreTimeMissed:          coreTimeMissed,
		Late:                    late,
		EarlyLeave:              earlyLeave,
		HasLeaveWorkOverlap:     hasLeaveWorkOverlap,
		HasIncompleteWorkRecord: hasIncompleteWorkRecord,
		Status:                  status,
		WorkSegments:            workSegmentDetails,
		LeaveSegments:           leaveSegmentDetails,
	}
	dayStatus.Status = attendanceWorkDayStatusLabel(dayStatus)
	return dayStatus
}

func attendanceWorkRecordsForEmail(
	events []attendanceEvent,
	email string,
	now time.Time,
	location *time.Location,
) attendanceWorkRecords {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	filtered := make([]attendanceEvent, 0, len(events))
	for _, event := range events {
		if event.CanceledAt != "" || !strings.EqualFold(event.Email, normalizedEmail) {
			continue
		}
		filtered = append(filtered, event)
	}
	sort.Slice(filtered, func(left int, right int) bool {
		return filtered[left].OccurredAt < filtered[right].OccurredAt
	})
	result := attendanceWorkRecords{
		CompletedSegments: make([]attendanceWorkSegment, 0, len(filtered)/2),
		IncompleteDates:   make(map[string]struct{}),
	}
	var startedAt time.Time
	for _, event := range filtered {
		occurredAt, errorValue := parseAttendanceEventTime(event.OccurredAt)
		if errorValue != nil {
			continue
		}
		if event.Kind == attendanceKindClockIn {
			if !startedAt.IsZero() {
				result.IncompleteDates[startedAt.In(location).Format(time.DateOnly)] = struct{}{}
			}
			startedAt = occurredAt
			continue
		}
		if event.Kind != attendanceKindClockOut {
			continue
		}
		if startedAt.IsZero() || !occurredAt.After(startedAt) {
			result.IncompleteDates[occurredAt.In(location).Format(time.DateOnly)] = struct{}{}
			continue
		}
		result.CompletedSegments = append(
			result.CompletedSegments,
			attendanceWorkSegment{Start: startedAt, End: occurredAt},
		)
		startedAt = time.Time{}
	}
	if !startedAt.IsZero() && now.After(startedAt) {
		if startedAt.In(location).Format(time.DateOnly) == now.In(location).Format(time.DateOnly) {
			result.Provisional = &attendanceWorkSegment{Start: startedAt, End: now}
		} else {
			result.IncompleteDates[startedAt.In(location).Format(time.DateOnly)] = struct{}{}
		}
	}
	return result
}

func attendanceLeaveByDate(
	occurrences []attendancePaidLeaveOccurrence,
	email string,
) map[string][]attendancePaidLeaveOccurrence {
	result := make(map[string][]attendancePaidLeaveOccurrence)
	for _, occurrence := range occurrences {
		if strings.EqualFold(occurrence.Email, email) {
			result[occurrence.Date] = append(result[occurrence.Date], occurrence)
		}
	}
	return result
}

func attendanceLeaveIntervals(
	date time.Time,
	occurrences []attendancePaidLeaveOccurrence,
	location *time.Location,
) []attendanceWorkSegment {
	intervals := make([]attendanceWorkSegment, 0, len(occurrences))
	for _, occurrence := range occurrences {
		startMinute, startError := attendanceWorkScheduleTimeMinutes(occurrence.StartTime)
		endMinute, endError := attendanceWorkScheduleTimeMinutes(occurrence.EndTime)
		if startError != nil || endError != nil || endMinute <= startMinute {
			continue
		}
		dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, location)
		intervals = append(intervals, attendanceWorkSegment{
			Start: dayStart.Add(time.Duration(startMinute) * time.Minute),
			End:   dayStart.Add(time.Duration(endMinute) * time.Minute),
		})
	}
	return intervals
}

func attendanceSegmentsOverlap(
	workSegments []attendanceWorkSegment,
	leaveIntervals []attendanceWorkSegment,
) bool {
	for _, workSegment := range workSegments {
		for _, leaveInterval := range leaveIntervals {
			if earliestTime(workSegment.End, leaveInterval.End).After(
				latestTime(workSegment.Start, leaveInterval.Start),
			) {
				return true
			}
		}
	}
	return false
}

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

func attendanceWorkDayStatusLabel(status attendanceWorkDayStatus) string {
	if status.NeedsReview {
		return "needsReview"
	}
	if status.IsWorking {
		return "working"
	}
	if status.CoreTimeMissed {
		return "coreTimeMissed"
	}
	if status.Late && status.EarlyLeave {
		return "lateAndEarlyLeave"
	}
	if status.Late {
		return "late"
	}
	if status.EarlyLeave {
		return "earlyLeave"
	}
	if !status.HasBaseline {
		return "actualOnly"
	}
	if status.RemainingMinutes > 0 {
		return "remaining"
	}
	if status.OvertimeMinutes > 0 {
		return "overtime"
	}
	return "fulfilled"
}

func attendanceWorkStatusLabel(status attendanceWorkStatus) string {
	if status.NeedsReview {
		return "needsReview"
	}
	if status.IsWorking {
		return "working"
	}
	if status.CoreTimeMissed {
		return "coreTimeMissed"
	}
	if status.Late && status.EarlyLeave {
		return "lateAndEarlyLeave"
	}
	if status.Late {
		return "late"
	}
	if status.EarlyLeave {
		return "earlyLeave"
	}
	if !status.HasBaseline {
		return "actualOnly"
	}
	if status.RemainingMinutes > 0 {
		return "remaining"
	}
	if status.OvertimeMinutes > 0 {
		return "overtime"
	}
	return "fulfilled"
}
