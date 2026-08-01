package admind

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (service *Service) openAttendanceLeaveMutationDatabase(ctx context.Context) (*sql.DB, error) {
	options := sqliteDatabaseOptions{transactionLock: "immediate"}
	return service.openStateDatabase(ctx, "attendance", ensureAttendanceSchema, options)
}

func (service *Service) grantAttendanceLeave(ctx context.Context, grant attendanceLeaveGrant) (attendanceLeaveBalance, error) {
	grant.Operation.Kind = normalizedAttendanceLeaveGrantKind(grant.Operation.Kind)
	if grant.Operation.Amount <= 0 {
		return attendanceLeaveBalance{}, fmt.Errorf("grant amount must be positive")
	}
	if errorValue := validateAttendanceLeaveOperation(grant.Operation); errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	if errorValue := validateOptionalAttendanceLeaveDate(grant.ExpiresOn); errorValue != nil {
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
	isNew, errorValue := insertAttendanceLeaveOperation(ctx, transaction, grant.Operation)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	if !isNew {
		return commitAttendanceLeaveBalance(ctx, transaction, grant.Operation.Employee, grant.Operation.LeaveTypeID)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	lotID := attendanceLeaveDeterministicID("lot", grant.Operation.OperationKey, 0)
	var expiresOn any
	if grant.ExpiresOn != "" {
		expiresOn = grant.ExpiresOn
	}
	_, errorValue = transaction.ExecContext(ctx, `
INSERT INTO attendance_leave_grant_lots (
	id, employee_email, user_id, leave_type_id, source_operation_key, granted_on, expires_on,
	original_milli_days, available_milli_days, reserved_milli_days, used_milli_days,
	expired_milli_days, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, 0, ?, ?)`,
		lotID,
		normalizeAttendanceLeaveEmail(grant.Operation.Employee.Email),
		strings.TrimSpace(grant.Operation.Employee.UserID),
		strings.TrimSpace(grant.Operation.LeaveTypeID),
		grant.Operation.OperationKey,
		grant.Operation.EffectiveOn,
		expiresOn,
		grant.Operation.Amount,
		grant.Operation.Amount,
		now,
		now,
	)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	availableAfter, errorValue := attendanceLeaveAvailableMilliDays(ctx, transaction, grant.Operation.Employee.Email, grant.Operation.LeaveTypeID)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	entryKind := grant.Operation.Kind
	if entryKind == attendanceLeaveOperationCarryover {
		entryKind = "carryoverIn"
	}
	if errorValue := insertAttendanceLeaveLedgerEntry(ctx, transaction, grant.Operation, 0, lotID, entryKind, grant.Operation.Amount, grant.Operation.Amount, 0, 0, 0, availableAfter, now); errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	return commitAttendanceLeaveBalance(ctx, transaction, grant.Operation.Employee, grant.Operation.LeaveTypeID)
}

func (service *Service) reserveAttendanceLeave(ctx context.Context, operation attendanceLeaveOperation) (attendanceLeaveBalance, error) {
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
	balance, errorValue := reserveAttendanceLeaveInTransaction(ctx, transaction, operation)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	return balance, nil
}

func reserveAttendanceLeaveInTransaction(
	ctx context.Context,
	transaction *sql.Tx,
	operation attendanceLeaveOperation,
) (attendanceLeaveBalance, error) {
	operation.Kind = attendanceLeaveOperationReserve
	if operation.Amount <= 0 || strings.TrimSpace(operation.ReferenceID) == "" {
		return attendanceLeaveBalance{}, fmt.Errorf("reservation amount and reference are required")
	}
	if errorValue := validateAttendanceLeaveOperation(operation); errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	isNew, errorValue := insertAttendanceLeaveOperation(ctx, transaction, operation)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	if !isNew {
		return queryAttendanceLeaveBalance(ctx, transaction, operation.Employee, operation.LeaveTypeID)
	}
	available, errorValue := attendanceLeaveAvailableMilliDays(ctx, transaction, operation.Employee.Email, operation.LeaveTypeID)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	if available < operation.Amount {
		return attendanceLeaveBalance{}, errAttendanceLeaveInsufficientBalance
	}
	allocations, errorValue := attendanceLeaveAvailableAllocations(ctx, transaction, operation)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for sequence, allocation := range allocations {
		result, updateError := transaction.ExecContext(ctx, `
UPDATE attendance_leave_grant_lots
SET available_milli_days = available_milli_days - ?,
	reserved_milli_days = reserved_milli_days + ?,
	updated_at = ?
WHERE id = ? AND available_milli_days >= ?`,
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
			return attendanceLeaveBalance{}, errAttendanceLeaveInsufficientBalance
		}
		availableAfter, availableError := attendanceLeaveAvailableMilliDays(ctx, transaction, operation.Employee.Email, operation.LeaveTypeID)
		if availableError != nil {
			return attendanceLeaveBalance{}, availableError
		}
		if errorValue := insertAttendanceLeaveLedgerEntry(ctx, transaction, operation, sequence, allocation.GrantLotID, operation.Kind, allocation.AmountMilliDays, -allocation.AmountMilliDays, allocation.AmountMilliDays, 0, 0, availableAfter, now); errorValue != nil {
			return attendanceLeaveBalance{}, errorValue
		}
	}
	return queryAttendanceLeaveBalance(ctx, transaction, operation.Employee, operation.LeaveTypeID)
}

func (service *Service) useAttendanceLeaveReservation(ctx context.Context, operation attendanceLeaveOperation) (attendanceLeaveBalance, error) {
	operation.Kind = attendanceLeaveOperationUse
	return service.closeAttendanceLeaveReservation(ctx, operation)
}

func (service *Service) releaseAttendanceLeaveReservation(ctx context.Context, operation attendanceLeaveOperation) (attendanceLeaveBalance, error) {
	operation.Kind = attendanceLeaveOperationRelease
	return service.closeAttendanceLeaveReservation(ctx, operation)
}

func (service *Service) expireAttendanceLeave(ctx context.Context, operation attendanceLeaveOperation) (attendanceLeaveBalance, error) {
	operation.Kind = attendanceLeaveOperationExpire
	operation.Amount = 0
	if errorValue := validateAttendanceLeaveOperation(operation); errorValue != nil {
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
	lots, total, errorValue := attendanceLeaveExpiringAllocations(ctx, transaction, operation)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	operation.Amount = total
	isNew, errorValue := insertAttendanceLeaveOperation(ctx, transaction, operation)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	if !isNew {
		return commitAttendanceLeaveBalance(ctx, transaction, operation.Employee, operation.LeaveTypeID)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for sequence, lot := range lots {
		if _, errorValue := transaction.ExecContext(ctx, `
UPDATE attendance_leave_grant_lots
SET available_milli_days = 0,
	expired_milli_days = expired_milli_days + ?,
	updated_at = ?
WHERE id = ? AND available_milli_days = ?`,
			lot.AmountMilliDays,
			now,
			lot.GrantLotID,
			lot.AmountMilliDays,
		); errorValue != nil {
			return attendanceLeaveBalance{}, errorValue
		}
		availableAfter, availableError := attendanceLeaveAvailableMilliDays(ctx, transaction, operation.Employee.Email, operation.LeaveTypeID)
		if availableError != nil {
			return attendanceLeaveBalance{}, availableError
		}
		if errorValue := insertAttendanceLeaveLedgerEntry(ctx, transaction, operation, sequence, lot.GrantLotID, operation.Kind, lot.AmountMilliDays, -lot.AmountMilliDays, 0, 0, lot.AmountMilliDays, availableAfter, now); errorValue != nil {
			return attendanceLeaveBalance{}, errorValue
		}
	}
	return commitAttendanceLeaveBalance(ctx, transaction, operation.Employee, operation.LeaveTypeID)
}

func (service *Service) adjustAttendanceLeave(ctx context.Context, operation attendanceLeaveOperation, signedAmountMilliDays int, expiresOn string) (attendanceLeaveBalance, error) {
	operation.Kind = attendanceLeaveOperationAdjustment
	if signedAmountMilliDays == 0 {
		return attendanceLeaveBalance{}, fmt.Errorf("adjustment amount must not be zero")
	}
	if signedAmountMilliDays > 0 {
		operation.Amount = signedAmountMilliDays
		return service.grantAttendanceLeave(ctx, attendanceLeaveGrant{Operation: operation, ExpiresOn: expiresOn})
	}
	operation.Amount = -signedAmountMilliDays
	return service.removeAvailableAttendanceLeave(ctx, operation, operation.Kind)
}

func (service *Service) recordUntrackedAttendanceLeaveUse(ctx context.Context, operation attendanceLeaveOperation) (attendanceLeaveBalance, error) {
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
	balance, errorValue := recordUntrackedAttendanceLeaveUseInTransaction(ctx, transaction, operation)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	return balance, nil
}

func recordUntrackedAttendanceLeaveUseInTransaction(
	ctx context.Context,
	transaction *sql.Tx,
	operation attendanceLeaveOperation,
) (attendanceLeaveBalance, error) {
	operation.Kind = attendanceLeaveOperationUntrackedUse
	if operation.Amount <= 0 {
		return attendanceLeaveBalance{}, fmt.Errorf("untracked use amount must be positive")
	}
	if errorValue := validateAttendanceLeaveOperation(operation); errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	isNew, errorValue := insertAttendanceLeaveOperation(ctx, transaction, operation)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	if isNew {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		availableAfter, availableError := attendanceLeaveAvailableMilliDays(ctx, transaction, operation.Employee.Email, operation.LeaveTypeID)
		if availableError != nil {
			return attendanceLeaveBalance{}, availableError
		}
		if errorValue := insertAttendanceLeaveLedgerEntry(ctx, transaction, operation, 0, "", operation.Kind, operation.Amount, 0, 0, operation.Amount, 0, availableAfter, now); errorValue != nil {
			return attendanceLeaveBalance{}, errorValue
		}
	}
	return queryAttendanceLeaveBalance(ctx, transaction, operation.Employee, operation.LeaveTypeID)
}

func normalizedAttendanceLeaveGrantKind(kind string) string {
	switch kind {
	case attendanceLeaveOperationLegalCorrection, attendanceLeaveOperationAdjustment, attendanceLeaveOperationCarryover:
		return kind
	default:
		return attendanceLeaveOperationGrant
	}
}
