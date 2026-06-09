package admind

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

const attendanceAbsenceSelectColumns = `id, email, kind, date, reason, created_by, created_at, canceled_at`

func (service *Service) insertAttendanceAbsences(ctx context.Context, email string, kind string, dates []string, reason string, actorEmail string) ([]attendanceAbsence, error) {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return nil, errorValue
	}
	defer transaction.Rollback()

	createdAt := time.Now().UTC().Format(time.RFC3339)
	absences := make([]attendanceAbsence, 0, len(dates))
	for _, date := range dates {
		absence := attendanceAbsence{
			ID:        "absence-" + randomHex(12),
			Email:     strings.ToLower(strings.TrimSpace(email)),
			Kind:      kind,
			LabelKey:  kind,
			Date:      date,
			Reason:    strings.TrimSpace(reason),
			CreatedBy: strings.ToLower(strings.TrimSpace(actorEmail)),
			CreatedAt: createdAt,
		}
		result, errorValue := transaction.ExecContext(ctx, `
INSERT OR IGNORE INTO attendance_absences (id, email, kind, date, reason, created_by, created_at, canceled_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			absence.ID,
			absence.Email,
			absence.Kind,
			absence.Date,
			absence.Reason,
			absence.CreatedBy,
			absence.CreatedAt,
			"",
		)
		if errorValue != nil {
			return nil, errorValue
		}
		rowsAffected, errorValue := result.RowsAffected()
		if errorValue != nil {
			return nil, errorValue
		}
		if rowsAffected == 0 {
			continue
		}
		absences = append(absences, absence)
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return nil, errorValue
	}
	return absences, nil
}

func (service *Service) readAttendanceAbsences(ctx context.Context, month string, email string) ([]attendanceAbsence, error) {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()

	startDate := month + "-01"
	endDate := attendanceNextMonth(month) + "-01"
	query := `SELECT ` + attendanceAbsenceSelectColumns + `
FROM attendance_absences
WHERE date >= ? AND date < ? AND canceled_at = ''`
	arguments := []any{startDate, endDate}
	if strings.TrimSpace(email) != "" {
		query += " AND email = ?"
		arguments = append(arguments, strings.ToLower(strings.TrimSpace(email)))
	}
	query += " ORDER BY date ASC, created_at ASC"
	rows, errorValue := database.QueryContext(ctx, query, arguments...)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()

	absences := []attendanceAbsence{}
	for rows.Next() {
		absence, errorValue := scanAttendanceAbsence(rows)
		if errorValue != nil {
			return nil, errorValue
		}
		absences = append(absences, absence)
	}
	return absences, rows.Err()
}

func (service *Service) readAttendanceAbsenceByID(ctx context.Context, absenceID string) (attendanceAbsence, bool, error) {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return attendanceAbsence{}, false, errorValue
	}
	defer database.Close()

	row := database.QueryRowContext(ctx, "SELECT "+attendanceAbsenceSelectColumns+`
FROM attendance_absences
WHERE id = ? AND canceled_at = ''`, strings.TrimSpace(absenceID))
	absence, errorValue := scanAttendanceAbsence(row)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return attendanceAbsence{}, false, nil
	}
	if errorValue != nil {
		return attendanceAbsence{}, false, errorValue
	}
	return absence, true, nil
}

func (service *Service) cancelAttendanceAbsence(ctx context.Context, absenceID string) error {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()

	_, errorValue = database.ExecContext(ctx, `
UPDATE attendance_absences
SET canceled_at = ?
WHERE id = ? AND canceled_at = ''`, time.Now().UTC().Format(time.RFC3339), strings.TrimSpace(absenceID))
	return errorValue
}

type attendanceAbsenceScanner interface {
	Scan(dest ...any) error
}

func scanAttendanceAbsence(scanner attendanceAbsenceScanner) (attendanceAbsence, error) {
	var absence attendanceAbsence
	errorValue := scanner.Scan(
		&absence.ID,
		&absence.Email,
		&absence.Kind,
		&absence.Date,
		&absence.Reason,
		&absence.CreatedBy,
		&absence.CreatedAt,
		&absence.CanceledAt,
	)
	absence.LabelKey = absence.Kind
	return absence, errorValue
}
