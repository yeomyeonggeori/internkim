package admind

import (
	"context"
	"strings"
)

func (service *Service) readAttendanceLeaveBalance(ctx context.Context, employee attendanceLeaveEmployee, leaveTypeID string) (attendanceLeaveBalance, error) {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	defer database.Close()
	return queryAttendanceLeaveBalance(ctx, database, employee, leaveTypeID)
}

func (service *Service) readAttendanceLeaveLedger(ctx context.Context, employee attendanceLeaveEmployee, leaveTypeID string) ([]attendanceLeaveLedgerEntry, error) {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT
	entry.id, entry.operation_key, entry.sequence, COALESCE(entry.grant_lot_id, ''), entry.kind,
	entry.amount_milli_days, entry.available_delta_milli_days, entry.reserved_delta_milli_days,
	entry.used_delta_milli_days, entry.expired_delta_milli_days, entry.available_after_milli_days,
	entry.effective_at, entry.created_at
FROM attendance_leave_ledger_entries entry
JOIN attendance_leave_operations operation ON operation.operation_key = entry.operation_key
WHERE operation.employee_email = ? AND operation.leave_type_id = ?
ORDER BY entry.created_at, entry.operation_key, entry.sequence`,
		normalizeAttendanceLeaveEmail(employee.Email),
		strings.TrimSpace(leaveTypeID),
	)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	entries := []attendanceLeaveLedgerEntry{}
	for rows.Next() {
		var entry attendanceLeaveLedgerEntry
		if errorValue := rows.Scan(
			&entry.ID,
			&entry.OperationKey,
			&entry.Sequence,
			&entry.GrantLotID,
			&entry.Kind,
			&entry.AmountMilliDays,
			&entry.AvailableDeltaMilliDays,
			&entry.ReservedDeltaMilliDays,
			&entry.UsedDeltaMilliDays,
			&entry.ExpiredDeltaMilliDays,
			&entry.AvailableAfterMilliDays,
			&entry.EffectiveAt,
			&entry.CreatedAt,
		); errorValue != nil {
			return nil, errorValue
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}
