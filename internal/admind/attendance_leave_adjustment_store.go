package admind

import (
	"context"
	"fmt"
	"time"
)

func (service *Service) removeAvailableAttendanceLeave(ctx context.Context, operation attendanceLeaveOperation, entryKind string) (attendanceLeaveBalance, error) {
	if operation.Amount <= 0 {
		return attendanceLeaveBalance{}, fmt.Errorf("removal amount must be positive")
	}
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
	isNew, errorValue := insertAttendanceLeaveOperation(ctx, transaction, operation)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	if !isNew {
		return commitAttendanceLeaveBalance(ctx, transaction, operation.Employee, operation.LeaveTypeID)
	}
	allocations, errorValue := attendanceLeaveAvailableAllocations(ctx, transaction, operation)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for sequence, allocation := range allocations {
		if _, errorValue := transaction.ExecContext(ctx, `
UPDATE attendance_leave_grant_lots
SET available_milli_days = available_milli_days - ?,
	expired_milli_days = expired_milli_days + ?,
	updated_at = ?
WHERE id = ? AND available_milli_days >= ?`,
			allocation.AmountMilliDays,
			allocation.AmountMilliDays,
			now,
			allocation.GrantLotID,
			allocation.AmountMilliDays,
		); errorValue != nil {
			return attendanceLeaveBalance{}, errorValue
		}
		availableAfter, availableError := attendanceLeaveAvailableMilliDays(ctx, transaction, operation.Employee.Email, operation.LeaveTypeID)
		if availableError != nil {
			return attendanceLeaveBalance{}, availableError
		}
		if errorValue := insertAttendanceLeaveLedgerEntry(ctx, transaction, operation, sequence, allocation.GrantLotID, entryKind, allocation.AmountMilliDays, -allocation.AmountMilliDays, 0, 0, allocation.AmountMilliDays, availableAfter, now); errorValue != nil {
			return attendanceLeaveBalance{}, errorValue
		}
	}
	return commitAttendanceLeaveBalance(ctx, transaction, operation.Employee, operation.LeaveTypeID)
}
