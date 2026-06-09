package admind

import (
	"errors"
	"strings"
	"time"
)

const (
	attendanceAbsenceLeave           = "leave"
	attendanceAbsenceBusinessTrip    = "business_trip"
	attendanceAbsenceDayOff          = "day_off"
	attendanceAbsenceOther           = "other"
	attendanceAbsenceLocalAdminActor = "local_admin"
	attendanceAbsenceDateLimit       = 366
)

var attendanceAbsenceKinds = map[string]struct{}{
	attendanceAbsenceLeave:        {},
	attendanceAbsenceBusinessTrip: {},
	attendanceAbsenceDayOff:       {},
	attendanceAbsenceOther:        {},
}

func normalizeAttendanceAbsenceKind(value string) (string, error) {
	kind := strings.ToLower(strings.TrimSpace(value))
	_, exists := attendanceAbsenceKinds[kind]
	if !exists {
		return "", errors.New("unsupported attendance absence kind")
	}
	return kind, nil
}

func attendanceAbsenceDates(startDate string, endDate string) ([]string, error) {
	start, errorValue := parseAttendanceAbsenceDate(startDate)
	if errorValue != nil {
		return nil, errorValue
	}
	end, errorValue := parseAttendanceAbsenceDate(firstNonEmpty(strings.TrimSpace(endDate), strings.TrimSpace(startDate)))
	if errorValue != nil {
		return nil, errorValue
	}
	if end.Before(start) {
		return nil, errors.New("attendance absence endDate must be on or after startDate")
	}
	dates := []string{}
	rangeDays := 0
	for date := start; !date.After(end); date = date.AddDate(0, 0, 1) {
		if rangeDays >= attendanceAbsenceDateLimit {
			return nil, errors.New("attendance absence range must be 366 days or fewer per request")
		}
		rangeDays++
		if isAttendanceAbsenceWeekend(date) {
			continue
		}
		dates = append(dates, date.Format("2006-01-02"))
	}
	return dates, nil
}

func isAttendanceAbsenceWeekend(date time.Time) bool {
	weekday := date.Weekday()
	return weekday == time.Saturday || weekday == time.Sunday
}

func parseAttendanceAbsenceDate(value string) (time.Time, error) {
	date, errorValue := time.Parse("2006-01-02", strings.TrimSpace(value))
	if errorValue != nil {
		return time.Time{}, errors.New("attendance absence date must use YYYY-MM-DD")
	}
	return date, nil
}
