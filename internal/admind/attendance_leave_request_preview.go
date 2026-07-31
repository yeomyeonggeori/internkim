package admind

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
)

const attendanceLeaveRequestMaximumDateRange = 366

func (service *Service) previewAttendanceLeaveRequest(
	ctx context.Context,
	input attendanceLeaveRequestInput,
	now time.Time,
) (attendanceLeaveRequestPreview, error) {
	return service.previewAttendanceLeaveRequestWithPastOption(ctx, input, now, false)
}

func (service *Service) previewAttendanceLeaveRequestAllowPast(
	ctx context.Context,
	input attendanceLeaveRequestInput,
	now time.Time,
) (attendanceLeaveRequestPreview, error) {
	return service.previewAttendanceLeaveRequestWithPastOption(ctx, input, now, true)
}

func (service *Service) previewAttendanceLeaveRequestWithPastOption(
	ctx context.Context,
	input attendanceLeaveRequestInput,
	now time.Time,
	allowPast bool,
) (attendanceLeaveRequestPreview, error) {
	input = normalizeAttendanceLeaveRequestInput(input)
	policy, errorValue := service.readAttendanceLeavePolicy(ctx)
	if errorValue != nil {
		return attendanceLeaveRequestPreview{}, errorValue
	}
	leaveType, found := attendanceLeaveTypeByID(policy, input.LeaveTypeID)
	if !found || !leaveType.IsActive {
		return attendanceLeaveRequestPreview{}, attendanceLeaveInvalidInputErrorf("leave type is not active")
	}
	if !slices.Contains(leaveType.AllowedUnits, input.Unit) {
		return attendanceLeaveRequestPreview{}, attendanceLeaveInvalidInputErrorf("leave unit is not allowed")
	}
	timeZone := service.workspaceTimeZone()
	schedule := defaultAttendanceWorkSchedule()
	startDate, endDate, errorValue := attendanceLeaveRequestDateRangeWithPastOption(
		input,
		now.In(timeZone.location),
		allowPast,
	)
	if errorValue != nil {
		return attendanceLeaveRequestPreview{}, errorValue
	}
	holidayDates, errorValue := service.readCalendarHolidayDatesForRange(
		ctx,
		startDate,
		endDate.AddDate(0, 0, 1),
	)
	if errorValue != nil {
		return attendanceLeaveRequestPreview{}, errorValue
	}
	preview := attendanceLeaveRequestPreview{
		Occurrences:   []attendanceLeaveRequestOccurrence{},
		ExcludedDates: []attendanceLeaveRequestExcludedDate{},
	}
	for date := startDate; !date.After(endDate); date = date.AddDate(0, 0, 1) {
		dateValue := date.Format(time.DateOnly)
		working, workingError := attendanceWorkScheduleIsWorkingDate(schedule, dateValue, holidayDates)
		if workingError != nil {
			return attendanceLeaveRequestPreview{}, workingError
		}
		if !working {
			preview.ExcludedDates = append(preview.ExcludedDates, attendanceLeaveRequestExcludedDate{
				Date:   dateValue,
				Reason: attendanceLeaveRequestExclusionReason(holidayDates, dateValue),
			})
			continue
		}
		occurrence, occurrenceError := attendanceLeaveRequestOccurrenceForDate(schedule, input, dateValue, timeZone.location)
		if occurrenceError != nil {
			return attendanceLeaveRequestPreview{}, occurrenceError
		}
		preview.Occurrences = append(preview.Occurrences, occurrence)
		preview.TotalDeductionMilliDays += occurrence.DeductionMilliDays
	}
	if len(preview.Occurrences) == 0 {
		return attendanceLeaveRequestPreview{}, attendanceLeaveInvalidInputErrorf("leave request has no working dates")
	}
	return preview, nil
}

func normalizeAttendanceLeaveRequestInput(input attendanceLeaveRequestInput) attendanceLeaveRequestInput {
	return attendanceLeaveRequestInput{
		LeaveTypeID:          strings.TrimSpace(input.LeaveTypeID),
		Unit:                 strings.TrimSpace(input.Unit),
		StartDate:            strings.TrimSpace(input.StartDate),
		EndDate:              strings.TrimSpace(input.EndDate),
		PartialPeriod:        strings.TrimSpace(input.PartialPeriod),
		StartTime:            strings.TrimSpace(input.StartTime),
		Reason:               strings.TrimSpace(input.Reason),
		Response:             strings.TrimSpace(input.Response),
		Revision:             input.Revision,
		RemovedAttachmentIDs: normalizedAttendanceLeaveAttachmentIDs(input.RemovedAttachmentIDs),
	}
}

func normalizedAttendanceLeaveAttachmentIDs(attachmentIDs []string) []string {
	normalized := make([]string, 0, len(attachmentIDs))
	seen := map[string]struct{}{}
	for _, attachmentID := range attachmentIDs {
		attachmentID = strings.TrimSpace(attachmentID)
		if attachmentID == "" {
			continue
		}
		if _, found := seen[attachmentID]; found {
			continue
		}
		seen[attachmentID] = struct{}{}
		normalized = append(normalized, attachmentID)
	}
	return normalized
}

func attendanceLeaveTypeByID(policy attendanceLeavePolicy, leaveTypeID string) (attendanceLeaveType, bool) {
	normalizedID := strings.TrimSpace(leaveTypeID)
	for _, leaveType := range policy.LeaveTypes {
		if leaveType.ID == normalizedID {
			return leaveType, true
		}
	}
	return attendanceLeaveType{}, false
}

func attendanceLeaveRequestDateRange(
	input attendanceLeaveRequestInput,
	now time.Time,
) (time.Time, time.Time, error) {
	return attendanceLeaveRequestDateRangeWithPastOption(input, now, false)
}

func attendanceLeaveRequestDateRangeWithPastOption(
	input attendanceLeaveRequestInput,
	now time.Time,
	allowPast bool,
) (time.Time, time.Time, error) {
	startDate, errorValue := attendanceWorkScheduleDate(input.StartDate, now.Location())
	if errorValue != nil {
		return time.Time{}, time.Time{}, attendanceLeaveInvalidInputError(errorValue)
	}
	endDateValue := input.EndDate
	if endDateValue == "" {
		endDateValue = input.StartDate
	}
	endDate, errorValue := attendanceWorkScheduleDate(endDateValue, now.Location())
	if errorValue != nil {
		return time.Time{}, time.Time{}, attendanceLeaveInvalidInputError(errorValue)
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if !allowPast && startDate.Before(today) {
		return time.Time{}, time.Time{}, attendanceLeaveInvalidInputErrorf(
			"leave request date must not be in the past",
		)
	}
	if endDate.Before(startDate) {
		return time.Time{}, time.Time{}, attendanceLeaveInvalidInputErrorf("leave request end date must not precede start date")
	}
	maximumEndDate := startDate.AddDate(0, 0, attendanceLeaveRequestMaximumDateRange-1)
	if endDate.After(maximumEndDate) {
		return time.Time{}, time.Time{}, attendanceLeaveInvalidInputErrorf(
			"leave request range must be %d days or fewer",
			attendanceLeaveRequestMaximumDateRange,
		)
	}
	if input.Unit != attendanceWorkScheduleFullDay && !endDate.Equal(startDate) {
		return time.Time{}, time.Time{}, attendanceLeaveInvalidInputErrorf("partial-day leave must use a single date")
	}
	return startDate, endDate, nil
}

func attendanceLeaveRequestOccurrenceForDate(
	schedule attendanceWorkSchedule,
	input attendanceLeaveRequestInput,
	date string,
	location *time.Location,
) (attendanceLeaveRequestOccurrence, error) {
	deductionMilliDays, errorValue := attendanceLeaveRequestDeductionMilliDays(input.Unit)
	if errorValue != nil {
		return attendanceLeaveRequestOccurrence{}, errorValue
	}
	startTime, errorValue := attendanceLeaveRequestStartTime(schedule, input, date, location)
	if errorValue != nil {
		return attendanceLeaveRequestOccurrence{}, errorValue
	}
	endTime, errorValue := calculateAttendanceWorkScheduleEndTime(schedule, date, startTime, input.Unit, location)
	if errorValue != nil {
		return attendanceLeaveRequestOccurrence{}, attendanceLeaveInvalidInputError(errorValue)
	}
	return attendanceLeaveRequestOccurrence{
		Date:               date,
		StartTime:          startTime,
		EndTime:            endTime.In(location).Format("15:04"),
		DeductionMilliDays: deductionMilliDays,
	}, nil
}

func attendanceLeaveRequestStartTime(
	schedule attendanceWorkSchedule,
	input attendanceLeaveRequestInput,
	date string,
	location *time.Location,
) (string, error) {
	if input.Unit == attendanceWorkScheduleFullDay {
		if input.PartialPeriod != "" || input.StartTime != "" {
			return "", attendanceLeaveInvalidInputErrorf("full-day leave must not include a partial period or start time")
		}
		return attendanceWorkSchedulePartialLeaveStartTime(schedule), nil
	}
	switch input.PartialPeriod {
	case attendanceLeavePartialPeriodMorning:
		if input.StartTime != "" {
			return "", attendanceLeaveInvalidInputErrorf("morning leave must not include a custom start time")
		}
		return attendanceWorkSchedulePartialLeaveStartTime(schedule), nil
	case attendanceLeavePartialPeriodAfternoon:
		if input.StartTime != "" {
			return "", attendanceLeaveInvalidInputErrorf("afternoon leave must not include a custom start time")
		}
		fullDayStart := attendanceWorkSchedulePartialLeaveStartTime(schedule)
		fullDayEnd, errorValue := calculateAttendanceWorkScheduleEndTime(schedule, date, fullDayStart, attendanceWorkScheduleFullDay, location)
		if errorValue != nil {
			return "", errorValue
		}
		requiredMinutes, errorValue := attendanceWorkScheduleMinutesForUnit(schedule, input.Unit)
		if errorValue != nil {
			return "", errorValue
		}
		startMinute, errorValue := calculateAttendanceLeaveRequestStartMinute(
			fullDayEnd.Hour()*60+fullDayEnd.Minute(),
			requiredMinutes,
			schedule.BreakPeriods,
		)
		if errorValue != nil {
			return "", errorValue
		}
		return fmt.Sprintf("%02d:%02d", startMinute/60, startMinute%60), nil
	case attendanceLeavePartialPeriodCustom:
		if input.StartTime == "" {
			return "", attendanceLeaveInvalidInputErrorf("custom partial leave requires a start time")
		}
		if _, errorValue := attendanceWorkScheduleTimeMinutes(input.StartTime); errorValue != nil {
			return "", attendanceLeaveInvalidInputError(errorValue)
		}
		return input.StartTime, nil
	default:
		return "", attendanceLeaveInvalidInputErrorf("partial-day leave requires a valid partial period")
	}
}

func calculateAttendanceLeaveRequestStartMinute(
	endMinute int,
	requiredMinutes int,
	breakPeriods []attendanceWorkScheduleBreakPeriod,
) (int, error) {
	currentMinute := endMinute
	remainingMinutes := requiredMinutes
	for remainingMinutes > 0 {
		previousBreakStart, previousBreakEnd := attendanceLeaveRequestPreviousBreak(currentMinute, breakPeriods)
		availableMinutes := currentMinute
		if previousBreakEnd >= 0 {
			availableMinutes = currentMinute - previousBreakEnd
		}
		if remainingMinutes <= availableMinutes {
			return currentMinute - remainingMinutes, nil
		}
		remainingMinutes -= availableMinutes
		if previousBreakEnd < 0 {
			return 0, fmt.Errorf("calculated start time crosses the day boundary")
		}
		currentMinute = previousBreakStart
	}
	return currentMinute, nil
}

func attendanceLeaveRequestPreviousBreak(
	currentMinute int,
	breakPeriods []attendanceWorkScheduleBreakPeriod,
) (int, int) {
	for index := len(breakPeriods) - 1; index >= 0; index-- {
		breakStartMinute, _ := attendanceWorkScheduleTimeMinutes(breakPeriods[index].StartTime)
		breakEndMinute, _ := attendanceWorkScheduleTimeMinutes(breakPeriods[index].EndTime)
		if breakEndMinute <= currentMinute {
			return breakStartMinute, breakEndMinute
		}
	}
	return -1, -1
}

func attendanceLeaveRequestDeductionMilliDays(unit string) (int, error) {
	switch unit {
	case attendanceWorkScheduleFullDay:
		return 1000, nil
	case attendanceWorkScheduleHalfDay:
		return 500, nil
	case attendanceWorkScheduleQuarterDay:
		return 250, nil
	default:
		return 0, attendanceLeaveInvalidInputErrorf("unsupported leave unit %q", unit)
	}
}

func attendanceLeaveRequestExclusionReason(holidayDates map[string]struct{}, date string) string {
	if _, isHoliday := holidayDates[date]; isHoliday {
		return "holiday"
	}
	return "nonWorkingDay"
}
