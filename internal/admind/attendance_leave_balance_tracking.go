package admind

import (
	"context"
	"database/sql"
	"fmt"
)

func attendanceLeaveTypeForRequest(
	policy attendanceLeavePolicy,
	leaveType attendanceLeaveType,
) attendanceLeaveType {
	if policy.BalanceTrackingMode == attendanceLeaveBalanceTrackingUnlimited {
		leaveType.BalanceMode = "none"
	}
	return leaveType
}

func (service *Service) synchronizeAttendanceLeaveBalanceTrackingBeforeUpdate(
	ctx context.Context,
	existing attendanceLeavePolicy,
	updated attendanceLeavePolicy,
) error {
	if existing.BalanceTrackingMode != attendanceLeaveBalanceTrackingManaged ||
		updated.BalanceTrackingMode != attendanceLeaveBalanceTrackingUnlimited {
		return nil
	}
	database, errorValue := service.openAttendanceLeaveMutationDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	defer transaction.Rollback()
	records, errorValue := readTrackedPendingAttendanceLeaveRequests(ctx, transaction)
	if errorValue != nil {
		return errorValue
	}
	for _, record := range records {
		if _, errorValue := closeAttendanceLeaveReservationInTransaction(
			ctx,
			transaction,
			attendanceLeaveOperation{
				OperationKey: fmt.Sprintf(
					"leave-policy:unlimited:release:%s:%d",
					record.ID,
					record.Revision,
				),
				Employee: attendanceLeaveEmployee{
					Email:  record.EmployeeEmail,
					UserID: record.UserID,
				},
				LeaveTypeID: attendanceLeaveBalanceAccountID(
					record.LeaveTypeID,
					record.BalanceMode,
				),
				Kind: attendanceLeaveOperationRelease,
				ReferenceID: attendanceLeaveRequestReservationReference(
					record.ID,
					record.Revision,
				),
				EffectiveOn: record.StartDate,
			},
		); errorValue != nil {
			return errorValue
		}
		if _, errorValue := transaction.ExecContext(
			ctx,
			`UPDATE attendance_leave_requests
SET balance_mode = 'none'
WHERE id = ? AND revision = ? AND status IN (?, ?)`,
			record.ID,
			record.Revision,
			attendanceLeaveRequestStatusPending,
			attendanceLeaveRequestStatusNeedsChanges,
		); errorValue != nil {
			return errorValue
		}
	}
	return transaction.Commit()
}

func readTrackedPendingAttendanceLeaveRequests(
	ctx context.Context,
	transaction *sql.Tx,
) ([]attendanceLeaveRequestRecord, error) {
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT id, employee_email, user_id, leave_type_id, balance_mode, start_date, revision
FROM attendance_leave_requests
WHERE status IN (?, ?) AND balance_mode <> 'none'
ORDER BY id`,
		attendanceLeaveRequestStatusPending,
		attendanceLeaveRequestStatusNeedsChanges,
	)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	records := []attendanceLeaveRequestRecord{}
	for rows.Next() {
		var record attendanceLeaveRequestRecord
		if errorValue := rows.Scan(
			&record.ID,
			&record.EmployeeEmail,
			&record.UserID,
			&record.LeaveTypeID,
			&record.BalanceMode,
			&record.StartDate,
			&record.Revision,
		); errorValue != nil {
			return nil, errorValue
		}
		records = append(records, record)
	}
	return records, rows.Err()
}
