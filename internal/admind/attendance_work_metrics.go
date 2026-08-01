package admind

import (
	"fmt"
	"strings"
	"time"
)

func calculateAttendanceWorkStatus(
	email string,
	displayName string,
	startDate string,
	endDate string,
	events []attendanceEvent,
	leaveOccurrences []attendanceApprovedLeaveOccurrence,
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
		status.LeaveMinutes += day.LeaveMinutes
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
		status.FulfilledMinutes = min(status.TargetMinutes, status.ActualMinutes)
		status.RemainingMinutes = max(0, status.TargetMinutes-status.FulfilledMinutes)
		status.DifferenceMinutes = status.ActualMinutes - status.TargetMinutes
		status.OvertimeMinutes = max(0, status.ActualMinutes-status.TargetMinutes)
	}
	status.Status = attendanceWorkStatusLabel(status)
	return status, nil
}

func calculateAttendanceWorkDayStatus(
	date time.Time,
	revision attendanceWorkPolicyRevision,
	records attendanceWorkRecords,
	leaveOccurrences []attendanceApprovedLeaveOccurrence,
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
	leaveMinutes := 0
	if targetMinutes > 0 {
		for _, occurrence := range leaveOccurrences {
			leaveMinutes += (revision.DailyTargetMinutes*occurrence.DeductionMilliDays + 999) / 1000
		}
		leaveMinutes = min(targetMinutes, leaveMinutes)
		targetMinutes -= leaveMinutes
	}
	fulfilledMinutes := actualMinutes
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
		differenceMinutes = actualMinutes - targetMinutes
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
		LeaveMinutes:            leaveMinutes,
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
