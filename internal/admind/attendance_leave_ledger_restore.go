package admind

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

func restoreAttendanceLeaveUseInTransaction(
	ctx context.Context,
	transaction *sql.Tx,
	operation attendanceLeaveOperation,
) (attendanceLeaveBalance, error) {
	operation.Kind = attendanceLeaveOperationRelease
	operation.Amount = 0
	if errorValue := validateAttendanceLeaveOperation(operation); errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	allocations, total, errorValue := attendanceLeaveUseAllocations(ctx, transaction, operation)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	operation.Amount = total
	isNew, errorValue := insertAttendanceLeaveOperation(ctx, transaction, operation)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	if !isNew {
		return queryAttendanceLeaveBalance(ctx, transaction, operation.Employee, operation.LeaveTypeID)
	}
	restored, errorValue := attendanceLeaveUseAlreadyRestored(ctx, transaction, operation)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	if restored {
		return attendanceLeaveBalance{}, errAttendanceLeaveReservationClosed
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for sequence, allocation := range allocations {
		result, updateError := transaction.ExecContext(ctx, `
UPDATE attendance_leave_grant_lots
SET used_milli_days = used_milli_days - ?,
	available_milli_days = available_milli_days + ?,
	updated_at = ?
WHERE id = ? AND used_milli_days >= ?`,
			allocation.AmountMilliDays,
			allocation.AmountMilliDays,
			now,
			allocation.GrantLotID,
			allocation.AmountMilliDays,
		)
		if updateError != nil {
			return attendanceLeaveBalance{}, updateError
		}
		updatedRows, rowsError := result.RowsAffected()
		if rowsError != nil {
			return attendanceLeaveBalance{}, rowsError
		}
		if updatedRows != 1 {
			return attendanceLeaveBalance{}, errAttendanceLeaveReservationClosed
		}
		availableAfter, availableError := attendanceLeaveAvailableMilliDays(ctx, transaction, operation.Employee.Email, operation.LeaveTypeID)
		if availableError != nil {
			return attendanceLeaveBalance{}, availableError
		}
		entryOperation := operation
		entryOperation.EffectiveOn = allocation.EffectiveOn
		if errorValue := insertAttendanceLeaveLedgerEntry(
			ctx,
			transaction,
			entryOperation,
			sequence,
			allocation.GrantLotID,
			attendanceLeaveOperationRelease,
			allocation.AmountMilliDays,
			allocation.AmountMilliDays,
			0,
			-allocation.AmountMilliDays,
			0,
			availableAfter,
			now,
		); errorValue != nil {
			return attendanceLeaveBalance{}, errorValue
		}
	}
	return queryAttendanceLeaveBalance(ctx, transaction, operation.Employee, operation.LeaveTypeID)
}

func attendanceLeaveUseAllocations(
	ctx context.Context,
	transaction *sql.Tx,
	operation attendanceLeaveOperation,
) ([]attendanceLeaveReservationAllocation, int, error) {
	var useOperationKey string
	var employeeEmail string
	var userID string
	var leaveTypeID string
	errorValue := transaction.QueryRowContext(ctx, `
SELECT operation_key, employee_email, user_id, leave_type_id
FROM attendance_leave_operations
WHERE kind = ? AND reference_id = ?`,
		attendanceLeaveOperationUse,
		strings.TrimSpace(operation.ReferenceID),
	).Scan(&useOperationKey, &employeeEmail, &userID, &leaveTypeID)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return nil, 0, errAttendanceLeaveReservationNotFound
	}
	if errorValue != nil {
		return nil, 0, errorValue
	}
	if employeeEmail != normalizeAttendanceLeaveEmail(operation.Employee.Email) ||
		userID != strings.TrimSpace(operation.Employee.UserID) ||
		leaveTypeID != strings.TrimSpace(operation.LeaveTypeID) {
		return nil, 0, errAttendanceLeaveOperationConflict
	}
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT grant_lot_id, amount_milli_days, effective_at
FROM attendance_leave_ledger_entries
WHERE operation_key = ?
ORDER BY sequence`,
		useOperationKey,
	)
	if errorValue != nil {
		return nil, 0, errorValue
	}
	defer rows.Close()
	allocations := []attendanceLeaveReservationAllocation{}
	total := 0
	for rows.Next() {
		var allocation attendanceLeaveReservationAllocation
		if errorValue := rows.Scan(
			&allocation.GrantLotID,
			&allocation.AmountMilliDays,
			&allocation.EffectiveOn,
		); errorValue != nil {
			return nil, 0, errorValue
		}
		allocations = append(allocations, allocation)
		total += allocation.AmountMilliDays
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, 0, errorValue
	}
	if len(allocations) == 0 {
		return nil, 0, errAttendanceLeaveReservationNotFound
	}
	return allocations, total, nil
}

func attendanceLeaveUseAlreadyRestored(
	ctx context.Context,
	transaction *sql.Tx,
	operation attendanceLeaveOperation,
) (bool, error) {
	var count int
	errorValue := transaction.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM attendance_leave_operations
WHERE reference_id = ?
	AND kind = ?
	AND operation_key <> ?`,
		strings.TrimSpace(operation.ReferenceID),
		attendanceLeaveOperationRelease,
		strings.TrimSpace(operation.OperationKey),
	).Scan(&count)
	return count > 0, errorValue
}
