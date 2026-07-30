package admind

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"strings"
)

const (
	attendanceLegacyLeaveTypeID      = "legacy-leave"
	attendanceLegacyLeaveTypeName    = "Legacy leave"
	attendanceLegacyMigrationApplied = "applied"
	attendanceLegacyMigrationRolled  = "rolledBack"
)

type attendanceLegacyAbsenceMigrationPreview struct {
	CandidateLeaveCount      int                                           `json:"candidateLeaveCount"`
	CandidateOccurrenceCount int                                           `json:"candidateOccurrenceCount"`
	AlreadyMigratedCount     int                                           `json:"alreadyMigratedCount"`
	ConflictCount            int                                           `json:"conflictCount"`
	PreservedOtherCount      int                                           `json:"preservedOtherCount"`
	Items                    []attendanceLegacyAbsenceMigrationPreviewItem `json:"items"`
	Fingerprint              string                                        `json:"fingerprint"`
}

type attendanceLegacyAbsenceMigrationPreviewItem struct {
	RangeID         string `json:"rangeID"`
	EmployeeEmail   string `json:"employeeEmail"`
	UserID          string `json:"userID,omitempty"`
	OriginalKind    string `json:"originalKind"`
	StartDate       string `json:"startDate"`
	EndDate         string `json:"endDate"`
	OccurrenceCount int    `json:"occurrenceCount"`
	Status          string `json:"status"`
	Issue           string `json:"issue,omitempty"`
}

type attendanceLegacyAbsenceMigrationBatch struct {
	ID                  string `json:"id"`
	Status              string `json:"status"`
	LeaveCount          int    `json:"leaveCount"`
	PreservedOtherCount int    `json:"preservedOtherCount"`
	CreatedAt           string `json:"createdAt"`
	RolledBackAt        string `json:"rolledBackAt,omitempty"`
}

type attendanceLegacyAbsenceRange struct {
	ID        string
	Email     string
	UserID    string
	Kind      string
	StartDate string
	EndDate   string
	Reason    string
	CreatedBy string
	CreatedAt string
	UpdatedAt string
	Dates     []string
	Status    string
	Issue     string
}

type attendanceLegacyAbsenceQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (service *Service) previewAttendanceLegacyAbsenceMigration(
	ctx context.Context,
	employeeUserIDs map[string]string,
) (attendanceLegacyAbsenceMigrationPreview, error) {
	database, errorValue := service.openAttendanceLegacyAbsenceMigrationDatabase(ctx, false)
	if errorValue != nil {
		return attendanceLegacyAbsenceMigrationPreview{}, errorValue
	}
	defer database.Close()
	return readAttendanceLegacyAbsenceMigrationPreview(ctx, database, employeeUserIDs)
}

func readAttendanceLegacyAbsenceMigrationPreview(
	ctx context.Context,
	queryer attendanceLegacyAbsenceQueryer,
	employeeUserIDs map[string]string,
) (attendanceLegacyAbsenceMigrationPreview, error) {
	ranges, errorValue := readAttendanceLegacyAbsenceRanges(ctx, queryer, employeeUserIDs)
	if errorValue != nil {
		return attendanceLegacyAbsenceMigrationPreview{}, errorValue
	}
	otherItems, errorValue := readAttendanceLegacyOtherAbsencePreviewItems(ctx, queryer)
	if errorValue != nil {
		return attendanceLegacyAbsenceMigrationPreview{}, errorValue
	}
	preview := attendanceLegacyAbsenceMigrationPreview{
		PreservedOtherCount: len(otherItems),
		Items: make(
			[]attendanceLegacyAbsenceMigrationPreviewItem,
			0,
			len(ranges)+len(otherItems),
		),
	}
	for _, absenceRange := range ranges {
		item := attendanceLegacyAbsenceMigrationPreviewItem{
			RangeID:         absenceRange.ID,
			EmployeeEmail:   absenceRange.Email,
			UserID:          absenceRange.UserID,
			OriginalKind:    absenceRange.Kind,
			StartDate:       absenceRange.StartDate,
			EndDate:         absenceRange.EndDate,
			OccurrenceCount: len(absenceRange.Dates),
			Status:          absenceRange.Status,
			Issue:           absenceRange.Issue,
		}
		switch absenceRange.Status {
		case "candidate":
			preview.CandidateLeaveCount++
			preview.CandidateOccurrenceCount += len(absenceRange.Dates)
		case "alreadyMigrated":
			preview.AlreadyMigratedCount++
		default:
			preview.ConflictCount++
		}
		preview.Items = append(preview.Items, item)
	}
	preview.Items = append(preview.Items, otherItems...)
	preview.Fingerprint, errorValue = attendanceLegacyAbsenceMigrationFingerprint(preview.Items)
	if errorValue != nil {
		return attendanceLegacyAbsenceMigrationPreview{}, errorValue
	}
	return preview, nil
}

func attendanceLegacyAbsenceMigrationFingerprint(
	items []attendanceLegacyAbsenceMigrationPreviewItem,
) (string, error) {
	encodedItems, errorValue := json.Marshal(items)
	if errorValue != nil {
		return "", errorValue
	}
	value := sha256.Sum256(encodedItems)
	return "sha256:" + hex.EncodeToString(value[:]), nil
}

func readAttendanceLegacyOtherAbsencePreviewItems(
	ctx context.Context,
	queryer attendanceLegacyAbsenceQueryer,
) ([]attendanceLegacyAbsenceMigrationPreviewItem, error) {
	rows, errorValue := queryer.QueryContext(ctx, `
SELECT range_value.id, range_value.email, range_value.kind, range_value.start_date,
	range_value.end_date, COUNT(occurrence.id)
FROM attendance_absence_ranges range_value
LEFT JOIN attendance_absence_occurrences occurrence
	ON occurrence.range_id = range_value.id AND occurrence.canceled_at = ''
WHERE range_value.canceled_at = '' AND range_value.kind = ?
GROUP BY range_value.id, range_value.email, range_value.kind, range_value.start_date,
	range_value.end_date
ORDER BY range_value.created_at, range_value.id`,
		attendanceAbsenceOther,
	)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	items := []attendanceLegacyAbsenceMigrationPreviewItem{}
	for rows.Next() {
		var item attendanceLegacyAbsenceMigrationPreviewItem
		if errorValue := rows.Scan(
			&item.RangeID,
			&item.EmployeeEmail,
			&item.OriginalKind,
			&item.StartDate,
			&item.EndDate,
			&item.OccurrenceCount,
		); errorValue != nil {
			return nil, errorValue
		}
		item.EmployeeEmail = normalizeAttendanceLeaveEmail(item.EmployeeEmail)
		item.Status = "preserved"
		items = append(items, item)
	}
	return items, rows.Err()
}

func readAttendanceLegacyAbsenceRanges(
	ctx context.Context,
	queryer attendanceLegacyAbsenceQueryer,
	employeeUserIDs map[string]string,
) ([]attendanceLegacyAbsenceRange, error) {
	rows, errorValue := queryer.QueryContext(ctx, `
SELECT id, email, kind, start_date, end_date, reason, created_by, created_at, updated_at
FROM attendance_absence_ranges
WHERE canceled_at = '' AND kind IN (?, ?)
ORDER BY created_at, id`,
		attendanceAbsenceLeave,
		attendanceAbsenceDayOff,
	)
	if errorValue != nil {
		return nil, errorValue
	}
	ranges := []attendanceLegacyAbsenceRange{}
	for rows.Next() {
		var absenceRange attendanceLegacyAbsenceRange
		if errorValue := rows.Scan(
			&absenceRange.ID,
			&absenceRange.Email,
			&absenceRange.Kind,
			&absenceRange.StartDate,
			&absenceRange.EndDate,
			&absenceRange.Reason,
			&absenceRange.CreatedBy,
			&absenceRange.CreatedAt,
			&absenceRange.UpdatedAt,
		); errorValue != nil {
			rows.Close()
			return nil, errorValue
		}
		absenceRange.Email = normalizeAttendanceLeaveEmail(absenceRange.Email)
		ranges = append(ranges, absenceRange)
	}
	if errorValue := rows.Err(); errorValue != nil {
		rows.Close()
		return nil, errorValue
	}
	if errorValue := rows.Close(); errorValue != nil {
		return nil, errorValue
	}
	for index := range ranges {
		dates, datesError := readAttendanceLegacyAbsenceOccurrenceDates(ctx, queryer, ranges[index].ID)
		if datesError != nil {
			return nil, datesError
		}
		ranges[index].Dates = dates
		ranges[index].UserID = strings.TrimSpace(employeeUserIDs[ranges[index].Email])
		ranges[index].Status, ranges[index].Issue, errorValue = attendanceLegacyAbsenceMigrationStatus(
			ctx,
			queryer,
			ranges[index],
		)
		if errorValue != nil {
			return nil, errorValue
		}
	}
	return ranges, nil
}

func readAttendanceLegacyAbsenceOccurrenceDates(
	ctx context.Context,
	queryer attendanceLegacyAbsenceQueryer,
	rangeID string,
) ([]string, error) {
	rows, errorValue := queryer.QueryContext(ctx, `
SELECT date
FROM attendance_absence_occurrences
WHERE range_id = ? AND canceled_at = ''
ORDER BY date, id`,
		rangeID,
	)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	dates := []string{}
	for rows.Next() {
		var date string
		if errorValue := rows.Scan(&date); errorValue != nil {
			return nil, errorValue
		}
		dates = append(dates, date)
	}
	return dates, rows.Err()
}

func attendanceLegacyAbsenceMigrationStatus(
	ctx context.Context,
	queryer attendanceLegacyAbsenceQueryer,
	absenceRange attendanceLegacyAbsenceRange,
) (string, string, error) {
	var linkedCount int
	if errorValue := queryer.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM attendance_leave_request_absence_ranges
WHERE range_id = ?`,
		absenceRange.ID,
	).Scan(&linkedCount); errorValue != nil {
		return "", "", errorValue
	}
	if linkedCount > 0 {
		return "alreadyMigrated", "", nil
	}
	var activeMigrationCount int
	if errorValue := queryer.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM attendance_legacy_absence_migration_items
WHERE range_id = ? AND rolled_back_at = ''`,
		absenceRange.ID,
	).Scan(&activeMigrationCount); errorValue != nil {
		return "", "", errorValue
	}
	if activeMigrationCount > 0 {
		return "alreadyMigrated", "", nil
	}
	if absenceRange.Email == "" {
		return "conflict", "employee email is empty", nil
	}
	if absenceRange.UserID == "" {
		return "conflict", "employee is not in the account directory", nil
	}
	if len(absenceRange.Dates) == 0 {
		return "conflict", "absence has no active occurrences", nil
	}
	requestID := attendanceLegacyAbsenceRequestID(absenceRange.ID)
	var requestCount int
	if errorValue := queryer.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM attendance_leave_requests
WHERE id = ?`,
		requestID,
	).Scan(&requestCount); errorValue != nil {
		return "", "", errorValue
	}
	if requestCount > 0 {
		return "conflict", "deterministic request ID is already in use", nil
	}
	return "candidate", "", nil
}

func attendanceLegacyAbsenceRequestID(rangeID string) string {
	return attendanceLeaveDeterministicID("legacy-leave", strings.TrimSpace(rangeID), 0)
}
