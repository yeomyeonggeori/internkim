package admind

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (service *Service) closeAttendanceLeaveReservation(ctx context.Context, operation attendanceLeaveOperation) (attendanceLeaveBalance, error) {
	database, errorValue := service.openAttendanceLeaveMutationDatabase(ctx)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	defer transaction.Rollback()
	balance, errorValue := closeAttendanceLeaveReservationInTransaction(ctx, transaction, operation)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	return balance, nil
}

func closeAttendanceLeaveReservationInTransaction(
	ctx context.Context,
	transaction *sql.Tx,
	operation attendanceLeaveOperation,
) (attendanceLeaveBalance, error) {
	if strings.TrimSpace(operation.ReferenceID) == "" {
		return attendanceLeaveBalance{}, fmt.Errorf("reservation reference is required")
	}
	operation.Amount = 0
	if errorValue := validateAttendanceLeaveOperation(operation); errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	allocations, total, errorValue := attendanceLeaveReservationAllocations(ctx, transaction, operation)
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
	if closed, errorValue := attendanceLeaveReservationHasOtherTerminal(ctx, transaction, operation); errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	} else if closed {
		return attendanceLeaveBalance{}, errAttendanceLeaveReservationClosed
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for sequence, allocation := range allocations {
		availableDelta := 0
		reservedDelta := -allocation.AmountMilliDays
		usedDelta := allocation.AmountMilliDays
		update := `
UPDATE attendance_leave_grant_lots
SET reserved_milli_days = reserved_milli_days - ?,
	used_milli_days = used_milli_days + ?,
	updated_at = ?
WHERE id = ? AND reserved_milli_days >= ?`
		if operation.Kind == attendanceLeaveOperationRelease {
			availableDelta = allocation.AmountMilliDays
			usedDelta = 0
			update = `
UPDATE attendance_leave_grant_lots
SET reserved_milli_days = reserved_milli_days - ?,
	available_milli_days = available_milli_days + ?,
	updated_at = ?
WHERE id = ? AND reserved_milli_days >= ?`
		}
		result, updateError := transaction.ExecContext(ctx, update, allocation.AmountMilliDays, allocation.AmountMilliDays, now, allocation.GrantLotID, allocation.AmountMilliDays)
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
		if errorValue := insertAttendanceLeaveLedgerEntry(ctx, transaction, entryOperation, sequence, allocation.GrantLotID, operation.Kind, allocation.AmountMilliDays, availableDelta, reservedDelta, usedDelta, 0, availableAfter, now); errorValue != nil {
			return attendanceLeaveBalance{}, errorValue
		}
	}
	return queryAttendanceLeaveBalance(ctx, transaction, operation.Employee, operation.LeaveTypeID)
}

func attendanceLeaveReservationAllocations(ctx context.Context, transaction *sql.Tx, operation attendanceLeaveOperation) ([]attendanceLeaveReservationAllocation, int, error) {
	var reserveEmployeeEmail string
	var reserveUserID string
	var reserveLeaveTypeID string
	errorValue := transaction.QueryRowContext(ctx, `
SELECT employee_email, user_id, leave_type_id
FROM attendance_leave_operations
WHERE kind = ? AND reference_id = ?`,
		attendanceLeaveOperationReserve,
		strings.TrimSpace(operation.ReferenceID),
	).Scan(&reserveEmployeeEmail, &reserveUserID, &reserveLeaveTypeID)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return nil, 0, errAttendanceLeaveReservationNotFound
	}
	if errorValue != nil {
		return nil, 0, errorValue
	}
	if reserveEmployeeEmail != normalizeAttendanceLeaveEmail(operation.Employee.Email) ||
		reserveUserID != strings.TrimSpace(operation.Employee.UserID) ||
		reserveLeaveTypeID != strings.TrimSpace(operation.LeaveTypeID) {
		return nil, 0, errAttendanceLeaveOperationConflict
	}
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT entry.grant_lot_id, entry.amount_milli_days, entry.effective_at
FROM attendance_leave_ledger_entries entry
JOIN attendance_leave_operations operation ON operation.operation_key = entry.operation_key
WHERE operation.kind = ? AND operation.reference_id = ?
ORDER BY entry.sequence`,
		attendanceLeaveOperationReserve,
		strings.TrimSpace(operation.ReferenceID),
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

func attendanceLeaveReservationHasOtherTerminal(ctx context.Context, transaction *sql.Tx, operation attendanceLeaveOperation) (bool, error) {
	var count int
	errorValue := transaction.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM attendance_leave_operations
WHERE reference_id = ?
	AND kind IN (?, ?)
	AND operation_key <> ?`,
		strings.TrimSpace(operation.ReferenceID),
		attendanceLeaveOperationUse,
		attendanceLeaveOperationRelease,
		strings.TrimSpace(operation.OperationKey),
	).Scan(&count)
	return count > 0, errorValue
}
