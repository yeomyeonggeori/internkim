package admind

import (
	"fmt"
	"time"
)

func attendanceWorkScheduleDate(value string, location *time.Location) (time.Time, error) {
	if location == nil {
		return time.Time{}, fmt.Errorf("time zone is required")
	}
	date, errorValue := time.ParseInLocation("2006-01-02", value, location)
	if errorValue != nil || date.Format("2006-01-02") != value {
		return time.Time{}, fmt.Errorf("date must use YYYY-MM-DD")
	}
	return date, nil
}

func attendanceWorkScheduleIsWorkingDate(schedule attendanceWorkSchedule, date string) (bool, error) {
	parsedDate, errorValue := attendanceWorkScheduleDate(date, time.UTC)
	if errorValue != nil {
		return false, errorValue
	}
	for _, holiday := range schedule.Holidays {
		if holiday.Date == date {
			return false, nil
		}
	}
	weekday := int(parsedDate.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	for _, workingWeekday := range schedule.WorkingWeekdays {
		if workingWeekday == weekday {
			return true, nil
		}
	}
	return false, nil
}

func attendanceWorkScheduleIsWorkingInstant(
	schedule attendanceWorkSchedule,
	instant time.Time,
	location *time.Location,
) (bool, error) {
	if location == nil {
		return false, fmt.Errorf("time zone is required")
	}
	return attendanceWorkScheduleIsWorkingDate(schedule, instant.In(location).Format("2006-01-02"))
}
