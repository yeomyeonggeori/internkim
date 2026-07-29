package admind

import (
	"context"
	"database/sql"
	"strings"
)

func attendanceLeaveAvailableAllocations(ctx context.Context, transaction *sql.Tx, operation attendanceLeaveOperation) ([]attendanceLeaveReservationAllocation, error) {
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT id, available_milli_days
FROM attendance_leave_grant_lots
WHERE employee_email = ?
	AND leave_type_id = ?
	AND available_milli_days > 0
	AND (expires_on IS NULL OR expires_on > ?)
ORDER BY expires_on IS NULL, expires_on, granted_on, id`,
		normalizeAttendanceLeaveEmail(operation.Employee.Email),
		strings.TrimSpace(operation.LeaveTypeID),
		operation.EffectiveOn,
	)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	remaining := operation.Amount
	allocations := []attendanceLeaveReservationAllocation{}
	for rows.Next() && remaining > 0 {
		var lotID string
		var available int
		if errorValue := rows.Scan(&lotID, &available); errorValue != nil {
			return nil, errorValue
		}
		amount := available
		if amount > remaining {
			amount = remaining
		}
		allocations = append(allocations, attendanceLeaveReservationAllocation{GrantLotID: lotID, AmountMilliDays: amount})
		remaining -= amount
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	if remaining != 0 {
		return nil, errAttendanceLeaveInsufficientBalance
	}
	return allocations, nil
}

func attendanceLeaveAllAvailableAllocations(ctx context.Context, transaction *sql.Tx, employee attendanceLeaveEmployee, leaveTypeID string) ([]attendanceLeaveReservationAllocation, error) {
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT id, available_milli_days
FROM attendance_leave_grant_lots
WHERE employee_email = ?
	AND leave_type_id = ?
	AND available_milli_days > 0
ORDER BY expires_on IS NULL, expires_on, granted_on, id`,
		normalizeAttendanceLeaveEmail(employee.Email),
		strings.TrimSpace(leaveTypeID),
	)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	allocations := []attendanceLeaveReservationAllocation{}
	for rows.Next() {
		var allocation attendanceLeaveReservationAllocation
		if errorValue := rows.Scan(&allocation.GrantLotID, &allocation.AmountMilliDays); errorValue != nil {
			return nil, errorValue
		}
		allocations = append(allocations, allocation)
	}
	return allocations, rows.Err()
}

func attendanceLeaveExpiringAllocations(ctx context.Context, transaction *sql.Tx, operation attendanceLeaveOperation) ([]attendanceLeaveReservationAllocation, int, error) {
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT id, available_milli_days
FROM attendance_leave_grant_lots
WHERE employee_email = ?
	AND leave_type_id = ?
	AND available_milli_days > 0
	AND expires_on IS NOT NULL
	AND expires_on <= ?
ORDER BY expires_on, granted_on, id`,
		normalizeAttendanceLeaveEmail(operation.Employee.Email),
		strings.TrimSpace(operation.LeaveTypeID),
		operation.EffectiveOn,
	)
	if errorValue != nil {
		return nil, 0, errorValue
	}
	defer rows.Close()
	allocations := []attendanceLeaveReservationAllocation{}
	total := 0
	for rows.Next() {
		var allocation attendanceLeaveReservationAllocation
		if errorValue := rows.Scan(&allocation.GrantLotID, &allocation.AmountMilliDays); errorValue != nil {
			return nil, 0, errorValue
		}
		allocations = append(allocations, allocation)
		total += allocation.AmountMilliDays
	}
	return allocations, total, rows.Err()
}
