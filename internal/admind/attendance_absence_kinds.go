package admind

import (
	"errors"
	"strings"
	"time"
)

const (
	attendanceAbsenceAnnualLeave     = "annual_leave"
	attendanceAbsenceBusinessTrip    = "business_trip"
	attendanceAbsenceDayOff          = "day_off"
	attendanceAbsenceSickLeave       = "sick_leave"
	attendanceAbsencePrivateLeave    = "private_leave"
	attendanceAbsenceLocalAdminActor = "local_admin"
	attendanceAbsenceDateLimit       = 31
)

type attendanceAbsenceKindDefinition struct {
	IsSensitive bool
}

var attendanceAbsenceKindDefinitions = map[string]attendanceAbsenceKindDefinition{
	attendanceAbsenceAnnualLeave:  {},
	attendanceAbsenceBusinessTrip: {},
	attendanceAbsenceDayOff:       {},
	attendanceAbsenceSickLeave: {
		IsSensitive: true,
	},
	attendanceAbsencePrivateLeave: {},
}

func normalizeAttendanceAbsenceKind(value string) (string, error) {
	kind := strings.ToLower(strings.TrimSpace(value))
	_, exists := attendanceAbsenceKindDefinitions[kind]
	if !exists || kind == attendanceAbsencePrivateLeave {
		return "", errors.New("unsupported attendance absence kind")
	}
	return kind, nil
}

func isSensitiveAttendanceAbsenceKind(kind string) bool {
	definition, exists := attendanceAbsenceKindDefinitions[kind]
	return exists && definition.IsSensitive
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
	for date := start; !date.After(end); date = date.AddDate(0, 0, 1) {
		if len(dates) >= attendanceAbsenceDateLimit {
			return nil, errors.New("attendance absence range must be 31 days or fewer")
		}
		dates = append(dates, date.Format("2006-01-02"))
	}
	return dates, nil
}

func parseAttendanceAbsenceDate(value string) (time.Time, error) {
	date, errorValue := time.Parse("2006-01-02", strings.TrimSpace(value))
	if errorValue != nil {
		return time.Time{}, errors.New("attendance absence date must use YYYY-MM-DD")
	}
	return date, nil
}
