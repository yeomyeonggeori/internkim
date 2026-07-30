package admind

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"time"
)

func reserveAttendanceLeaveRequestOccurrencesInTransaction(
	ctx context.Context,
	transaction *sql.Tx,
	operation attendanceLeaveOperation,
	occurrences []attendanceLeaveRequestOccurrence,
) (attendanceLeaveBalance, error) {
	orderedOccurrences := slices.Clone(occurrences)
	slices.SortFunc(
		orderedOccurrences,
		func(first attendanceLeaveRequestOccurrence, second attendanceLeaveRequestOccurrence) int {
			if first.Date != second.Date {
				return strings.Compare(first.Date, second.Date)
			}
			return strings.Compare(first.StartTime, second.StartTime)
		},
	)
	total := 0
	for _, occurrence := range orderedOccurrences {
		if _, errorValue := time.Parse(time.DateOnly, occurrence.Date); errorValue != nil {
			return attendanceLeaveBalance{}, errorValue
		}
		if occurrence.DeductionMilliDays <= 0 {
			return attendanceLeaveBalance{}, fmt.Errorf("leave occurrence deduction must be positive")
		}
		total += occurrence.DeductionMilliDays
	}
	if len(orderedOccurrences) == 0 || strings.TrimSpace(operation.ReferenceID) == "" {
		return attendanceLeaveBalance{}, fmt.Errorf("leave reservation occurrences and reference are required")
	}
	operation.Kind = attendanceLeaveOperationReserve
	operation.Amount = total
	operation.EffectiveOn = orderedOccurrences[0].Date
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
	createdAt := time.Now().UTC().Format(time.RFC3339Nano)
	sequence := 0
	for _, occurrence := range orderedOccurrences {
		allocations, errorValue := attendanceLeaveRequestOccurrenceAllocations(
			ctx,
			transaction,
			operation,
			occurrence,
		)
		if errorValue != nil {
			return attendanceLeaveBalance{}, errorValue
		}
		for _, allocation := range allocations {
			result, updateError := transaction.ExecContext(ctx, `
UPDATE attendance_leave_grant_lots
SET available_milli_days = available_milli_days - ?,
	reserved_milli_days = reserved_milli_days + ?,
	updated_at = ?
WHERE id = ? AND available_milli_days >= ?`,
				allocation.AmountMilliDays,
				allocation.AmountMilliDays,
				createdAt,
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
			availableAfter, availableError := attendanceLeaveAvailableMilliDays(
				ctx,
				transaction,
				operation.Employee.Email,
				operation.LeaveTypeID,
			)
			if availableError != nil {
				return attendanceLeaveBalance{}, availableError
			}
			entryOperation := operation
			entryOperation.EffectiveOn = occurrence.Date
			if errorValue := insertAttendanceLeaveLedgerEntry(
				ctx,
				transaction,
				entryOperation,
				sequence,
				allocation.GrantLotID,
				operation.Kind,
				allocation.AmountMilliDays,
				-allocation.AmountMilliDays,
				allocation.AmountMilliDays,
				0,
				0,
				availableAfter,
				createdAt,
			); errorValue != nil {
				return attendanceLeaveBalance{}, errorValue
			}
			sequence++
		}
	}
	return queryAttendanceLeaveBalance(ctx, transaction, operation.Employee, operation.LeaveTypeID)
}

func attendanceLeaveRequestOccurrenceAllocations(
	ctx context.Context,
	transaction *sql.Tx,
	operation attendanceLeaveOperation,
	occurrence attendanceLeaveRequestOccurrence,
) ([]attendanceLeaveReservationAllocation, error) {
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT id, available_milli_days
FROM attendance_leave_grant_lots
WHERE employee_email = ?
	AND leave_type_id = ?
	AND available_milli_days > 0
	AND granted_on <= ?
	AND (expires_on IS NULL OR expires_on >= ?)
ORDER BY expires_on IS NULL, expires_on, granted_on, id`,
		normalizeAttendanceLeaveEmail(operation.Employee.Email),
		strings.TrimSpace(operation.LeaveTypeID),
		occurrence.Date,
		occurrence.Date,
	)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	remaining := occurrence.DeductionMilliDays
	allocations := []attendanceLeaveReservationAllocation{}
	for rows.Next() && remaining > 0 {
		var allocation attendanceLeaveReservationAllocation
		var available int
		if errorValue := rows.Scan(&allocation.GrantLotID, &available); errorValue != nil {
			return nil, errorValue
		}
		allocation.AmountMilliDays = min(available, remaining)
		allocations = append(allocations, allocation)
		remaining -= allocation.AmountMilliDays
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	if remaining != 0 {
		return nil, errAttendanceLeaveInsufficientBalance
	}
	return allocations, nil
}
