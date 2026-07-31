package admind

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

type attendanceLeaveQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func validateAttendanceLeaveOperation(operation attendanceLeaveOperation) error {
	if strings.TrimSpace(operation.OperationKey) == "" {
		return fmt.Errorf("operation key is required")
	}
	if normalizeAttendanceLeaveEmail(operation.Employee.Email) == "" {
		return fmt.Errorf("employee email is required")
	}
	if strings.TrimSpace(operation.LeaveTypeID) == "" {
		return fmt.Errorf("leave type id is required")
	}
	if operation.Amount < 0 {
		return fmt.Errorf("operation amount must not be negative")
	}
	if _, errorValue := time.Parse(time.DateOnly, operation.EffectiveOn); errorValue != nil {
		return fmt.Errorf("invalid effective date: %w", errorValue)
	}
	switch operation.Kind {
	case attendanceLeaveOperationGrant,
		attendanceLeaveOperationReserve,
		attendanceLeaveOperationUse,
		attendanceLeaveOperationRelease,
		attendanceLeaveOperationExpire,
		attendanceLeaveOperationCarryover,
		attendanceLeaveOperationLegalCorrection,
		attendanceLeaveOperationAdjustment,
		attendanceLeaveOperationUntrackedUse:
		return nil
	default:
		return fmt.Errorf("invalid leave operation kind %q", operation.Kind)
	}
}

func validateOptionalAttendanceLeaveDate(value string) error {
	if value == "" {
		return nil
	}
	if _, errorValue := time.Parse(time.DateOnly, value); errorValue != nil {
		return fmt.Errorf("invalid leave date: %w", errorValue)
	}
	return nil
}

func normalizeAttendanceLeaveEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func attendanceLeaveDeterministicID(prefix string, operationKey string, sequence int) string {
	value := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d", prefix, operationKey, sequence)))
	return prefix + "-" + hex.EncodeToString(value[:12])
}

func normalizeAttendanceLeaveOperation(operation attendanceLeaveOperation) attendanceLeaveOperation {
	return attendanceLeaveOperation{
		OperationKey: strings.TrimSpace(operation.OperationKey),
		Employee: attendanceLeaveEmployee{
			Email:  normalizeAttendanceLeaveEmail(operation.Employee.Email),
			UserID: strings.TrimSpace(operation.Employee.UserID),
		},
		LeaveTypeID: strings.TrimSpace(operation.LeaveTypeID),
		Kind:        operation.Kind,
		ReferenceID: strings.TrimSpace(operation.ReferenceID),
		Amount:      operation.Amount,
		EffectiveOn: operation.EffectiveOn,
	}
}

func insertAttendanceLeaveOperation(ctx context.Context, transaction *sql.Tx, operation attendanceLeaveOperation) (bool, error) {
	normalized := normalizeAttendanceLeaveOperation(operation)
	result, errorValue := transaction.ExecContext(ctx, `
INSERT OR IGNORE INTO attendance_leave_operations (
	operation_key, employee_email, user_id, leave_type_id, kind, reference_id,
	amount_milli_days, effective_date, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		normalized.OperationKey,
		normalized.Employee.Email,
		normalized.Employee.UserID,
		normalized.LeaveTypeID,
		normalized.Kind,
		normalized.ReferenceID,
		normalized.Amount,
		normalized.EffectiveOn,
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	if errorValue != nil {
		return false, errorValue
	}
	rowsAffected, errorValue := result.RowsAffected()
	if errorValue != nil {
		return false, errorValue
	}
	if rowsAffected == 1 {
		return true, nil
	}
	var stored attendanceLeaveOperation
	stored.OperationKey = normalized.OperationKey
	errorValue = transaction.QueryRowContext(ctx, `
SELECT employee_email, user_id, leave_type_id, kind, reference_id, amount_milli_days, effective_date
FROM attendance_leave_operations
WHERE operation_key = ?`,
		normalized.OperationKey,
	).Scan(
		&stored.Employee.Email,
		&stored.Employee.UserID,
		&stored.LeaveTypeID,
		&stored.Kind,
		&stored.ReferenceID,
		&stored.Amount,
		&stored.EffectiveOn,
	)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return false, errAttendanceLeaveOperationConflict
	}
	if errorValue != nil {
		return false, errorValue
	}
	if stored != normalized {
		return false, errAttendanceLeaveOperationConflict
	}
	return false, nil
}

func attendanceLeaveComputedOperationExists(ctx context.Context, transaction *sql.Tx, operation attendanceLeaveOperation) (bool, error) {
	normalized := normalizeAttendanceLeaveOperation(operation)
	var stored attendanceLeaveOperation
	stored.OperationKey = normalized.OperationKey
	errorValue := transaction.QueryRowContext(ctx, `
SELECT employee_email, user_id, leave_type_id, kind, reference_id, amount_milli_days, effective_date
FROM attendance_leave_operations
WHERE operation_key = ?`,
		normalized.OperationKey,
	).Scan(
		&stored.Employee.Email,
		&stored.Employee.UserID,
		&stored.LeaveTypeID,
		&stored.Kind,
		&stored.ReferenceID,
		&stored.Amount,
		&stored.EffectiveOn,
	)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return false, nil
	}
	if errorValue != nil {
		return false, errorValue
	}
	stored.Amount = normalized.Amount
	if stored != normalized {
		return false, errAttendanceLeaveOperationConflict
	}
	return true, nil
}

func insertAttendanceLeaveLedgerEntry(
	ctx context.Context,
	transaction *sql.Tx,
	operation attendanceLeaveOperation,
	sequence int,
	lotID string,
	kind string,
	amount int,
	availableDelta int,
	reservedDelta int,
	usedDelta int,
	expiredDelta int,
	availableAfter int,
	createdAt string,
) error {
	var grantLotID any
	if lotID != "" {
		grantLotID = lotID
	}
	_, errorValue := transaction.ExecContext(ctx, `
INSERT INTO attendance_leave_ledger_entries (
	id, operation_key, sequence, grant_lot_id, kind, amount_milli_days,
	available_delta_milli_days, reserved_delta_milli_days, used_delta_milli_days,
	expired_delta_milli_days, available_after_milli_days, effective_at, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		attendanceLeaveDeterministicID("ledger", operation.OperationKey, sequence),
		operation.OperationKey,
		sequence,
		grantLotID,
		kind,
		amount,
		availableDelta,
		reservedDelta,
		usedDelta,
		expiredDelta,
		availableAfter,
		operation.EffectiveOn,
		createdAt,
	)
	return errorValue
}

func attendanceLeaveAvailableMilliDays(ctx context.Context, queryer attendanceLeaveQueryer, email string, leaveTypeID string) (int, error) {
	var available int
	errorValue := queryer.QueryRowContext(ctx, `
SELECT COALESCE(SUM(available_milli_days), 0)
FROM attendance_leave_grant_lots
WHERE employee_email = ? AND leave_type_id = ?`,
		normalizeAttendanceLeaveEmail(email),
		strings.TrimSpace(leaveTypeID),
	).Scan(&available)
	return available, errorValue
}

func queryAttendanceLeaveBalance(ctx context.Context, queryer attendanceLeaveQueryer, employee attendanceLeaveEmployee, leaveTypeID string) (attendanceLeaveBalance, error) {
	balance := attendanceLeaveBalance{
		EmployeeEmail: normalizeAttendanceLeaveEmail(employee.Email),
		UserID:        strings.TrimSpace(employee.UserID),
		LeaveTypeID:   strings.TrimSpace(leaveTypeID),
	}
	errorValue := queryer.QueryRowContext(ctx, `
SELECT
	COALESCE(SUM(
		CASE WHEN operation.kind <> 'carryover' THEN lot.original_milli_days ELSE 0 END
	), 0),
	COALESCE(SUM(available_milli_days), 0),
	COALESCE(SUM(reserved_milli_days), 0),
	COALESCE(SUM(used_milli_days), 0)
FROM attendance_leave_grant_lots lot
JOIN attendance_leave_operations operation ON operation.operation_key = lot.source_operation_key
WHERE lot.employee_email = ? AND lot.leave_type_id = ?`,
		balance.EmployeeEmail,
		balance.LeaveTypeID,
	).Scan(
		&balance.GrantedMilliDays,
		&balance.AvailableMilliDays,
		&balance.ReservedMilliDays,
		&balance.UsedMilliDays,
	)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	var untrackedUsed int
	errorValue = queryer.QueryRowContext(ctx, `
SELECT COALESCE(SUM(entry.amount_milli_days), 0)
FROM attendance_leave_ledger_entries entry
JOIN attendance_leave_operations operation ON operation.operation_key = entry.operation_key
LEFT JOIN attendance_leave_requests request
	ON (
		operation.reference_id = request.id
		OR instr(operation.reference_id, request.id || ':revision:') = 1
	)
	AND request.employee_email = operation.employee_email
WHERE operation.employee_email = ?
	AND operation.leave_type_id = ?
	AND entry.kind = ?
	AND (request.id IS NULL OR request.status = ?)`,
		balance.EmployeeEmail,
		balance.LeaveTypeID,
		attendanceLeaveOperationUntrackedUse,
		attendanceLeaveRequestStatusApproved,
	).Scan(&untrackedUsed)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	balance.UsedMilliDays += untrackedUsed
	errorValue = queryer.QueryRowContext(ctx, `
SELECT COALESCE(SUM(entry.amount_milli_days), 0)
FROM attendance_leave_ledger_entries entry
JOIN attendance_leave_operations operation ON operation.operation_key = entry.operation_key
WHERE operation.employee_email = ?
	AND operation.leave_type_id = ?
	AND entry.kind = ?`,
		balance.EmployeeEmail,
		balance.LeaveTypeID,
		attendanceLeaveOperationExpire,
	).Scan(&balance.ExpiredMilliDays)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	var nextExpiryDate sql.NullString
	var nextExpiryAmount sql.NullInt64
	errorValue = queryer.QueryRowContext(ctx, `
SELECT expires_on, SUM(available_milli_days)
FROM attendance_leave_grant_lots
WHERE employee_email = ? AND leave_type_id = ? AND expires_on IS NOT NULL AND available_milli_days > 0
GROUP BY expires_on
ORDER BY expires_on
LIMIT 1`,
		balance.EmployeeEmail,
		balance.LeaveTypeID,
	).Scan(&nextExpiryDate, &nextExpiryAmount)
	if errorValue != nil && !errors.Is(errorValue, sql.ErrNoRows) {
		return attendanceLeaveBalance{}, errorValue
	}
	if nextExpiryDate.Valid {
		balance.NextExpiryDate = nextExpiryDate.String
		balance.NextExpiryMilliDays = int(nextExpiryAmount.Int64)
	}
	return balance, nil
}

func commitAttendanceLeaveBalance(ctx context.Context, transaction *sql.Tx, employee attendanceLeaveEmployee, leaveTypeID string) (attendanceLeaveBalance, error) {
	balance, errorValue := queryAttendanceLeaveBalance(ctx, transaction, employee, leaveTypeID)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	return balance, nil
}
