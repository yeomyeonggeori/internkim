package admind

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

var errAttendanceAbsenceDateConflict = errors.New("attendance absence already exists for one or more dates")

func insertAttendanceAbsenceOccurrencesInTransaction(ctx context.Context, transaction *sql.Tx, absenceRange attendanceAbsenceRange) error {
	dates, errorValue := attendanceAbsenceDates(absenceRange.StartDate, absenceRange.EndDate)
	if errorValue != nil {
		return errorValue
	}
	for _, date := range dates {
		if errorValue := insertAttendanceAbsenceOccurrenceInTransaction(ctx, transaction, absenceRange, date); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func insertAttendanceAbsenceOccurrenceInTransaction(ctx context.Context, transaction *sql.Tx, absenceRange attendanceAbsenceRange, date string) error {
	_, errorValue := transaction.ExecContext(ctx, `
INSERT INTO attendance_absence_occurrences (id, range_id, email, date, created_at, canceled_at)
VALUES (?, ?, ?, ?, ?, ?)`,
		joinAttendanceAbsenceOccurrenceID(absenceRange.ID, date),
		absenceRange.ID,
		absenceRange.Email,
		date,
		absenceRange.CreatedAt,
		"",
	)
	if errorValue != nil && isAttendanceAbsenceOccurrenceConflict(errorValue) {
		return errAttendanceAbsenceDateConflict
	}
	return errorValue
}

func isAttendanceAbsenceOccurrenceConflict(errorValue error) bool {
	message := strings.ToLower(errorValue.Error())
	if strings.Contains(message, "attendance_absence_occurrences_active_user_date") {
		return true
	}
	return strings.Contains(message, "constraint") && strings.Contains(message, "attendance_absence_occurrences")
}
