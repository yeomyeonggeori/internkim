package admind

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

const attendanceAbsenceRangeSelectColumns = `id, email, kind, start_date, end_date, reason, created_by, created_at, updated_at, canceled_at, replaced_by`

func (service *Service) insertAttendanceAbsenceRange(ctx context.Context, email string, kind string, startDate string, endDate string, reason string, actorEmail string) ([]attendanceAbsence, error) {
	normalizedStartDate, normalizedEndDate := normalizeAttendanceAbsenceDateRange(startDate, endDate)
	dates, errorValue := attendanceAbsenceDates(startDate, endDate)
	if errorValue != nil {
		return nil, errorValue
	}
	if len(dates) == 0 {
		return []attendanceAbsence{}, nil
	}
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
	rewriteResult, errorValue := rewriteAttendanceAbsenceRanges(ctx, transaction, attendanceAbsenceRangeRewriteRequest{
		Email:      strings.ToLower(strings.TrimSpace(email)),
		Kind:       kind,
		Reason:     strings.TrimSpace(reason),
		ActorEmail: strings.ToLower(strings.TrimSpace(actorEmail)),
		StartDate:  normalizedStartDate,
		EndDate:    normalizedEndDate,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Dates:      dates,
	})
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := invalidateAttendanceAbsenceCacheDates(ctx, transaction, rewriteResult.AffectedDates); errorValue != nil {
		return nil, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return nil, errorValue
	}
	expandedRanges := expandAttendanceAbsenceRanges(rewriteResult.CreatedRanges, normalizedStartDate, attendanceDateAfter(normalizedEndDate))
	return filterAttendanceAbsencesByDates(expandedRanges, attendanceDateSet(rewriteResult.CreatedDates)), nil
}

func (service *Service) readAttendanceAbsenceOccurrences(ctx context.Context, startDate string, endDate string, email string) ([]attendanceAbsence, error) {
	ranges, errorValue := service.readAttendanceAbsenceRanges(ctx, startDate, endDate, email)
	if errorValue != nil {
		return nil, errorValue
	}
	return expandAttendanceAbsenceRanges(ranges, startDate, endDate), nil
}

func (service *Service) readAttendanceAbsenceRanges(ctx context.Context, startDate string, endDate string, email string) ([]attendanceAbsenceRange, error) {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	query := `SELECT ` + attendanceAbsenceRangeSelectColumns + `
FROM attendance_absence_ranges
WHERE start_date < ? AND end_date >= ? AND canceled_at = ''`
	arguments := []any{endDate, startDate}
	if strings.TrimSpace(email) != "" {
		query += " AND email = ?"
		arguments = append(arguments, strings.ToLower(strings.TrimSpace(email)))
	}
	query += " ORDER BY start_date ASC, created_at ASC"
	rows, errorValue := database.QueryContext(ctx, query, arguments...)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	ranges := []attendanceAbsenceRange{}
	for rows.Next() {
		absenceRange, errorValue := scanAttendanceAbsenceRange(rows)
		if errorValue != nil {
			return nil, errorValue
		}
		ranges = append(ranges, absenceRange)
	}
	return ranges, rows.Err()
}

func (service *Service) readAttendanceAbsenceRangeByID(ctx context.Context, rangeID string) (attendanceAbsenceRange, bool, error) {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return attendanceAbsenceRange{}, false, errorValue
	}
	defer database.Close()
	row := database.QueryRowContext(ctx, `SELECT `+attendanceAbsenceRangeSelectColumns+`
FROM attendance_absence_ranges
WHERE id = ? AND canceled_at = ''`, strings.TrimSpace(rangeID))
	absenceRange, errorValue := scanAttendanceAbsenceRange(row)
	if attendanceAbsenceRangeNotFound(errorValue) {
		return attendanceAbsenceRange{}, false, nil
	}
	if errorValue != nil {
		return attendanceAbsenceRange{}, false, errorValue
	}
	return absenceRange, true, nil
}

func (service *Service) cancelAttendanceAbsenceRange(ctx context.Context, rangeID string) error {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	defer transaction.Rollback()
	absenceRange, exists, errorValue := readAttendanceAbsenceRangeByIDInTransaction(ctx, transaction, rangeID)
	if errorValue != nil || !exists {
		return errorValue
	}
	dates, errorValue := attendanceAbsenceDates(absenceRange.StartDate, absenceRange.EndDate)
	if errorValue != nil {
		return errorValue
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if errorValue := cancelAttendanceAbsenceRangeInTransaction(ctx, transaction, rangeID, now, ""); errorValue != nil {
		return errorValue
	}
	if errorValue := invalidateAttendanceAbsenceCacheDates(ctx, transaction, dates); errorValue != nil {
		return errorValue
	}
	return transaction.Commit()
}

func (service *Service) cancelAttendanceAbsenceOccurrence(ctx context.Context, rangeID string, date string) error {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	defer transaction.Rollback()
	absenceRange, exists, errorValue := readAttendanceAbsenceRangeByIDInTransaction(ctx, transaction, rangeID)
	if errorValue != nil || !exists {
		return errorValue
	}
	existingDates, errorValue := attendanceAbsenceDates(absenceRange.StartDate, absenceRange.EndDate)
	if errorValue != nil {
		return errorValue
	}
	removedDates := map[string]struct{}{date: {}}
	if len(attendanceDateRangeOverlapDates(date, date, attendanceDateSet(existingDates))) == 0 {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if errorValue := cancelAttendanceAbsenceRangeInTransaction(ctx, transaction, absenceRange.ID, now, ""); errorValue != nil {
		return errorValue
	}
	remainingDates := subtractAttendanceDates(existingDates, removedDates)
	if _, errorValue := insertAttendanceAbsenceRangesForDates(ctx, transaction, absenceRange.Email, absenceRange.Kind, absenceRange.Reason, absenceRange.CreatedBy, absenceRange.CreatedAt, now, remainingDates); errorValue != nil {
		return errorValue
	}
	if errorValue := invalidateAttendanceAbsenceCacheDates(ctx, transaction, existingDates); errorValue != nil {
		return errorValue
	}
	return transaction.Commit()
}

func readAttendanceAbsenceRangesForUpdate(ctx context.Context, transaction *sql.Tx, email string, startDate string, endDate string) ([]attendanceAbsenceRange, error) {
	rows, errorValue := transaction.QueryContext(ctx, `SELECT `+attendanceAbsenceRangeSelectColumns+`
FROM attendance_absence_ranges
WHERE email = ? AND start_date <= ? AND end_date >= ? AND canceled_at = ''
ORDER BY start_date ASC, created_at ASC`, strings.ToLower(strings.TrimSpace(email)), endDate, startDate)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	ranges := []attendanceAbsenceRange{}
	for rows.Next() {
		absenceRange, errorValue := scanAttendanceAbsenceRange(rows)
		if errorValue != nil {
			return nil, errorValue
		}
		ranges = append(ranges, absenceRange)
	}
	return ranges, rows.Err()
}

func readAttendanceAbsenceRangeByIDInTransaction(ctx context.Context, transaction *sql.Tx, rangeID string) (attendanceAbsenceRange, bool, error) {
	row := transaction.QueryRowContext(ctx, `SELECT `+attendanceAbsenceRangeSelectColumns+`
FROM attendance_absence_ranges
WHERE id = ? AND canceled_at = ''`, strings.TrimSpace(rangeID))
	absenceRange, errorValue := scanAttendanceAbsenceRange(row)
	if attendanceAbsenceRangeNotFound(errorValue) {
		return attendanceAbsenceRange{}, false, nil
	}
	if errorValue != nil {
		return attendanceAbsenceRange{}, false, errorValue
	}
	return absenceRange, true, nil
}

func insertAttendanceAbsenceRangesForDates(ctx context.Context, transaction *sql.Tx, email string, kind string, reason string, createdBy string, createdAt string, updatedAt string, dates []string) ([]attendanceAbsenceRange, error) {
	ranges := collapseAttendanceDatesToRanges(dates)
	createdRanges := make([]attendanceAbsenceRange, 0, len(ranges))
	for _, dateRange := range ranges {
		absenceRange := attendanceAbsenceRange{
			ID:        "absence-range-" + randomHex(12),
			Email:     strings.ToLower(strings.TrimSpace(email)),
			Kind:      normalizeStoredAttendanceAbsenceKind(kind),
			StartDate: dateRange.StartDate,
			EndDate:   dateRange.EndDate,
			Reason:    strings.TrimSpace(reason),
			CreatedBy: strings.ToLower(strings.TrimSpace(createdBy)),
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		}
		if errorValue := insertAttendanceAbsenceRangeInTransaction(ctx, transaction, absenceRange); errorValue != nil {
			return nil, errorValue
		}
		if errorValue := insertAttendanceAbsenceOccurrencesInTransaction(ctx, transaction, absenceRange); errorValue != nil {
			return nil, errorValue
		}
		createdRanges = append(createdRanges, absenceRange)
	}
	return createdRanges, nil
}

func insertAttendanceAbsenceRangeInTransaction(ctx context.Context, transaction *sql.Tx, absenceRange attendanceAbsenceRange) error {
	_, errorValue := transaction.ExecContext(ctx, `
INSERT INTO attendance_absence_ranges (id, email, kind, start_date, end_date, reason, created_by, created_at, updated_at, canceled_at, replaced_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		absenceRange.ID,
		absenceRange.Email,
		absenceRange.Kind,
		absenceRange.StartDate,
		absenceRange.EndDate,
		absenceRange.Reason,
		absenceRange.CreatedBy,
		absenceRange.CreatedAt,
		absenceRange.UpdatedAt,
		"",
		"",
	)
	return errorValue
}

func cancelAttendanceAbsenceRangeInTransaction(ctx context.Context, transaction *sql.Tx, rangeID string, canceledAt string, replacedBy string) error {
	if _, errorValue := transaction.ExecContext(ctx, `
UPDATE attendance_absence_ranges
SET canceled_at = ?, updated_at = ?, replaced_by = ?
WHERE id = ? AND canceled_at = ''`, canceledAt, canceledAt, replacedBy, strings.TrimSpace(rangeID)); errorValue != nil {
		return errorValue
	}
	_, errorValue := transaction.ExecContext(ctx, `
UPDATE attendance_absence_occurrences
SET canceled_at = ?
WHERE range_id = ? AND canceled_at = ''`, canceledAt, strings.TrimSpace(rangeID))
	return errorValue
}

func filterAttendanceAbsencesByDates(absences []attendanceAbsence, dates map[string]struct{}) []attendanceAbsence {
	filteredAbsences := make([]attendanceAbsence, 0, len(absences))
	for _, absence := range absences {
		if _, exists := dates[absence.Date]; exists {
			filteredAbsences = append(filteredAbsences, absence)
		}
	}
	return filteredAbsences
}
