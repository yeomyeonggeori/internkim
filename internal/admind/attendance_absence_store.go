package admind

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

func (service *Service) readAttendanceAbsences(ctx context.Context, month string, email string) ([]attendanceAbsence, error) {
	startDate, endDate := attendanceMonthGridDateRange(month)
	return service.readAttendanceAbsenceOccurrences(ctx, startDate, endDate, email)
}

func (service *Service) readAttendanceAbsenceByID(ctx context.Context, absenceID string) (attendanceAbsence, bool, error) {
	rangeID, date := splitAttendanceAbsenceOccurrenceID(absenceID)
	absenceRange, exists, errorValue := service.readAttendanceAbsenceRangeByID(ctx, rangeID)
	if errorValue != nil {
		return attendanceAbsence{}, false, errorValue
	}
	if !exists {
		return attendanceAbsence{}, false, nil
	}
	if strings.TrimSpace(date) == "" {
		occurrences := expandAttendanceAbsenceRanges([]attendanceAbsenceRange{absenceRange}, absenceRange.StartDate, attendanceDateAfter(absenceRange.EndDate))
		if len(occurrences) == 0 {
			return attendanceAbsence{}, false, nil
		}
		return occurrences[0], true, nil
	}
	occurrences := expandAttendanceAbsenceRanges([]attendanceAbsenceRange{absenceRange}, date, attendanceDateAfter(date))
	if len(occurrences) == 0 {
		return attendanceAbsence{}, false, nil
	}
	return occurrences[0], true, nil
}

func (service *Service) cancelAttendanceAbsence(ctx context.Context, absenceID string) error {
	rangeID, date := splitAttendanceAbsenceOccurrenceID(absenceID)
	if strings.TrimSpace(date) == "" {
		return service.cancelAttendanceAbsenceRange(ctx, rangeID)
	}
	return service.cancelAttendanceAbsenceOccurrence(ctx, rangeID, date)
}

func scanAttendanceAbsenceRange(scanner attendanceAbsenceScanner) (attendanceAbsenceRange, error) {
	var absenceRange attendanceAbsenceRange
	errorValue := scanner.Scan(
		&absenceRange.ID,
		&absenceRange.Email,
		&absenceRange.Kind,
		&absenceRange.StartDate,
		&absenceRange.EndDate,
		&absenceRange.Reason,
		&absenceRange.CreatedBy,
		&absenceRange.CreatedAt,
		&absenceRange.UpdatedAt,
		&absenceRange.CanceledAt,
		&absenceRange.ReplacedBy,
	)
	if errorValue != nil {
		return attendanceAbsenceRange{}, errorValue
	}
	absenceRange.Kind = normalizeStoredAttendanceAbsenceKind(absenceRange.Kind)
	return absenceRange, nil
}

func attendanceAbsenceRangeNotFound(errorValue error) bool {
	return errors.Is(errorValue, sql.ErrNoRows)
}

type attendanceAbsenceScanner interface {
	Scan(dest ...any) error
}
