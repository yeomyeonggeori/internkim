package admind

import (
	"context"
	"fmt"
	"time"
)

type attendanceWorkCalendarDay struct {
	Date        string
	WorkMode    string
	WorkingDate bool
	Holiday     bool
}

func (service *Service) attendanceWorkCalendarProjection(
	ctx context.Context,
	from time.Time,
	to time.Time,
) ([]attendanceWorkCalendarDay, error) {
	if !to.After(from) {
		return nil, fmt.Errorf("work calendar window must end after it begins")
	}
	location, _ := service.workspaceTimeLocation()
	startDate, errorValue := attendanceWorkScheduleDate(from.UTC().Format(time.DateOnly), location)
	if errorValue != nil {
		return nil, errorValue
	}
	endDate, errorValue := attendanceWorkScheduleDate(to.UTC().Format(time.DateOnly), location)
	if errorValue != nil {
		return nil, errorValue
	}
	holidayDates, errorValue := service.readCalendarHolidayDatesForRange(ctx, startDate, endDate)
	if errorValue != nil {
		return nil, errorValue
	}
	policy, errorValue := service.readAttendanceWorkPolicy(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	projection := make([]attendanceWorkCalendarDay, 0, int(endDate.Sub(startDate).Hours()/24))
	for date := startDate; date.Before(endDate); date = date.AddDate(0, 0, 1) {
		dateValue := date.Format(time.DateOnly)
		revision, revisionError := attendanceWorkPolicyRevisionForDate(policy, dateValue)
		if revisionError != nil {
			return nil, revisionError
		}
		workingDate, workingDateError := attendanceWorkScheduleIsWorkingDate(
			attendanceWorkScheduleFromPolicyRevision(revision),
			dateValue,
			holidayDates,
		)
		if workingDateError != nil {
			return nil, workingDateError
		}
		projection = append(projection, attendanceWorkCalendarDay{
			Date:        dateValue,
			WorkMode:    revision.WorkMode,
			WorkingDate: workingDate,
			Holiday:     containsAttendanceHolidayDate(holidayDates, dateValue),
		})
	}
	return projection, nil
}

func containsAttendanceHolidayDate(holidayDates map[string]struct{}, date string) bool {
	_, found := holidayDates[date]
	return found
}
