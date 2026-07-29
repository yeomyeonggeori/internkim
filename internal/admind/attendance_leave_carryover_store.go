package admind

import (
	"context"
	"database/sql"
	"time"
)

func (service *Service) carryoverAttendanceLeave(
	ctx context.Context,
	operation attendanceLeaveOperation,
	limitMilliDays *int,
	expiresOn string,
) (attendanceLeaveBalance, error) {
	operation.Kind = attendanceLeaveOperationCarryover
	operation.Amount = 0
	if errorValue := validateAttendanceLeaveOperation(operation); errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	if errorValue := validateOptionalAttendanceLeaveDate(expiresOn); errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
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
	exists, errorValue := attendanceLeaveComputedOperationExists(ctx, transaction, operation)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	if exists {
		return commitAttendanceLeaveBalance(ctx, transaction, operation.Employee, operation.LeaveTypeID)
	}
	allocations, available, errorValue := attendanceLeaveExpiringAllocations(ctx, transaction, operation)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	carryoverAmount := attendanceLeaveCarryoverMilliDays(available, true, limitMilliDays)
	operation.Amount = carryoverAmount
	isNew, errorValue := insertAttendanceLeaveOperation(ctx, transaction, operation)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	if !isNew {
		return commitAttendanceLeaveBalance(ctx, transaction, operation.Employee, operation.LeaveTypeID)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	remainingCarryover := carryoverAmount
	sequence := 0
	for _, allocation := range allocations {
		carryoverFromLot := allocation.AmountMilliDays
		if carryoverFromLot > remainingCarryover {
			carryoverFromLot = remainingCarryover
		}
		if carryoverFromLot > 0 {
			if errorValue := moveAttendanceLeaveLotToExpired(ctx, transaction, allocation.GrantLotID, carryoverFromLot, now); errorValue != nil {
				return attendanceLeaveBalance{}, errorValue
			}
			remainingCarryover -= carryoverFromLot
			availableAfter, availableError := attendanceLeaveAvailableMilliDays(ctx, transaction, operation.Employee.Email, operation.LeaveTypeID)
			if availableError != nil {
				return attendanceLeaveBalance{}, availableError
			}
			if errorValue := insertAttendanceLeaveLedgerEntry(ctx, transaction, operation, sequence, allocation.GrantLotID, "carryoverOut", carryoverFromLot, -carryoverFromLot, 0, 0, carryoverFromLot, availableAfter, now); errorValue != nil {
				return attendanceLeaveBalance{}, errorValue
			}
			sequence++
		}
		expiringAmount := allocation.AmountMilliDays - carryoverFromLot
		if expiringAmount > 0 {
			if errorValue := moveAttendanceLeaveLotToExpired(ctx, transaction, allocation.GrantLotID, expiringAmount, now); errorValue != nil {
				return attendanceLeaveBalance{}, errorValue
			}
			availableAfter, availableError := attendanceLeaveAvailableMilliDays(ctx, transaction, operation.Employee.Email, operation.LeaveTypeID)
			if availableError != nil {
				return attendanceLeaveBalance{}, availableError
			}
			if errorValue := insertAttendanceLeaveLedgerEntry(ctx, transaction, operation, sequence, allocation.GrantLotID, attendanceLeaveOperationExpire, expiringAmount, -expiringAmount, 0, 0, expiringAmount, availableAfter, now); errorValue != nil {
				return attendanceLeaveBalance{}, errorValue
			}
			sequence++
		}
	}
	if carryoverAmount > 0 {
		if errorValue := insertAttendanceLeaveCarryoverLot(ctx, transaction, operation, carryoverAmount, expiresOn, sequence, now); errorValue != nil {
			return attendanceLeaveBalance{}, errorValue
		}
	}
	return commitAttendanceLeaveBalance(ctx, transaction, operation.Employee, operation.LeaveTypeID)
}

func moveAttendanceLeaveLotToExpired(ctx context.Context, transaction *sql.Tx, lotID string, amount int, now string) error {
	result, errorValue := transaction.ExecContext(ctx, `
UPDATE attendance_leave_grant_lots
SET available_milli_days = available_milli_days - ?,
	expired_milli_days = expired_milli_days + ?,
	updated_at = ?
WHERE id = ? AND available_milli_days >= ?`,
		amount,
		amount,
		now,
		lotID,
		amount,
	)
	if errorValue != nil {
		return errorValue
	}
	rowsAffected, errorValue := result.RowsAffected()
	if errorValue != nil {
		return errorValue
	}
	if rowsAffected != 1 {
		return errAttendanceLeaveInsufficientBalance
	}
	return nil
}

func insertAttendanceLeaveCarryoverLot(
	ctx context.Context,
	transaction *sql.Tx,
	operation attendanceLeaveOperation,
	amount int,
	expiresOn string,
	sequence int,
	now string,
) error {
	lotID := attendanceLeaveDeterministicID("lot", operation.OperationKey, 0)
	var expiresOnValue any
	if expiresOn != "" {
		expiresOnValue = expiresOn
	}
	_, errorValue := transaction.ExecContext(ctx, `
INSERT INTO attendance_leave_grant_lots (
	id, employee_email, user_id, leave_type_id, source_operation_key, granted_on, expires_on,
	original_milli_days, available_milli_days, reserved_milli_days, used_milli_days,
	expired_milli_days, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, 0, ?, ?)`,
		lotID,
		normalizeAttendanceLeaveEmail(operation.Employee.Email),
		operation.Employee.UserID,
		operation.LeaveTypeID,
		operation.OperationKey,
		operation.EffectiveOn,
		expiresOnValue,
		amount,
		amount,
		now,
		now,
	)
	if errorValue != nil {
		return errorValue
	}
	availableAfter, errorValue := attendanceLeaveAvailableMilliDays(ctx, transaction, operation.Employee.Email, operation.LeaveTypeID)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := insertAttendanceLeaveLedgerEntry(ctx, transaction, operation, sequence, lotID, "carryoverIn", amount, amount, 0, 0, 0, availableAfter, now); errorValue != nil {
		return errorValue
	}
	return nil
}
