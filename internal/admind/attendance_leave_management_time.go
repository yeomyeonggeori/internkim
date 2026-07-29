package admind

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const attendanceLeaveManagementChangeTimeCorrected = "timeCorrected"

func (service *Service) correctManagedAttendanceLeaveTime(
	ctx context.Context,
	requestID string,
	input attendanceLeaveManagementTimeInput,
	administratorEmail string,
	now time.Time,
) error {
	input.EmployeeEmail = normalizeAttendanceLeaveEmail(input.EmployeeEmail)
	input.StartTime = strings.TrimSpace(input.StartTime)
	input.EndTime = strings.TrimSpace(input.EndTime)
	input.Reason = strings.TrimSpace(input.Reason)
	if input.EmployeeEmail == "" ||
		input.StartTime == "" ||
		input.EndTime == "" ||
		input.Reason == "" {
		return attendanceLeaveInvalidInputErrorf(
			"employee, start time, end time, and reason are required",
		)
	}
	startTime, startError := time.Parse("15:04", input.StartTime)
	endTime, endError := time.Parse("15:04", input.EndTime)
	if startError != nil || endError != nil || !endTime.After(startTime) {
		return attendanceLeaveInvalidInputErrorf("leave time range is invalid")
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
	record, errorValue := readAttendanceLeaveRequestRecordInTransaction(
		ctx,
		transaction,
		requestID,
		input.EmployeeEmail,
	)
	if errorValue != nil {
		return errorValue
	}
	if record.Status != attendanceLeaveRequestStatusApproved ||
		record.Unit == "fullDay" ||
		len(record.Occurrences) != 1 {
		return errAttendanceLeaveRequestCannotUpdate
	}
	occurrence := record.Occurrences[0]
	occurrence.StartTime = input.StartTime
	occurrence.EndTime = input.EndTime
	if errorValue := service.updateApprovedLeaveClockOutInTransaction(
		ctx,
		transaction,
		record.ID,
		occurrence,
	); errorValue != nil {
		return errorValue
	}
	if errorValue := service.ensureManagedAttendanceLeaveTimeHasNoConflict(
		ctx,
		transaction,
		record,
		occurrence,
		now,
	); errorValue != nil {
		return errorValue
	}
	nextRevision := record.Revision + 1
	if errorValue := rebindManagedAttendanceLeaveUseInTransaction(
		ctx,
		transaction,
		record,
		occurrence,
		nextRevision,
	); errorValue != nil {
		return errorValue
	}
	updatedAt := now.UTC().Format(time.RFC3339Nano)
	result, errorValue := transaction.ExecContext(ctx, `
UPDATE attendance_leave_requests
SET partial_period = ?, start_time = ?, revision = ?, updated_at = ?
WHERE id = ? AND employee_email = ? AND status = ? AND revision = ?`,
		attendanceLeavePartialPeriodCustom,
		input.StartTime,
		nextRevision,
		updatedAt,
		record.ID,
		record.EmployeeEmail,
		attendanceLeaveRequestStatusApproved,
		record.Revision,
	)
	if errorValue != nil {
		return errorValue
	}
	rowsAffected, errorValue := result.RowsAffected()
	if errorValue != nil {
		return errorValue
	}
	if rowsAffected != 1 {
		return errAttendanceLeaveRequestCannotUpdate
	}
	if _, errorValue := transaction.ExecContext(ctx, `
UPDATE attendance_leave_request_occurrences
SET start_time = ?, end_time = ?
WHERE request_id = ?`,
		input.StartTime,
		input.EndTime,
		record.ID,
	); errorValue != nil {
		return errorValue
	}
	if _, errorValue := transaction.ExecContext(ctx, `
INSERT INTO attendance_leave_request_events (
	id, request_id, kind, actor_email, response, created_at
) VALUES (?, ?, ?, ?, ?, ?)`,
		attendanceLeaveDeterministicID("leave-event-time-corrected", record.ID, nextRevision),
		record.ID,
		attendanceLeaveManagementChangeTimeCorrected,
		normalizeAttendanceLeaveEmail(administratorEmail),
		input.Reason,
		updatedAt,
	); errorValue != nil {
		return errorValue
	}
	if errorValue := invalidateAttendanceAbsenceCacheDates(
		ctx,
		transaction,
		[]string{occurrence.Date},
	); errorValue != nil {
		return errorValue
	}
	return transaction.Commit()
}

func rebindManagedAttendanceLeaveUseInTransaction(
	ctx context.Context,
	transaction *sql.Tx,
	record attendanceLeaveRequestRecord,
	occurrence attendanceLeaveRequestOccurrence,
	nextRevision int,
) error {
	if record.BalanceMode == "none" {
		return nil
	}
	employee := attendanceLeaveEmployee{
		Email:  record.EmployeeEmail,
		UserID: record.UserID,
	}
	if _, errorValue := restoreAttendanceLeaveUseInTransaction(
		ctx,
		transaction,
		attendanceLeaveOperation{
			OperationKey: fmt.Sprintf(
				"leave-request:%s:restore:%d",
				record.ID,
				record.Revision,
			),
			Employee:    employee,
			LeaveTypeID: record.LeaveTypeID,
			ReferenceID: attendanceLeaveRequestReservationReference(
				record.ID,
				record.Revision,
			),
			EffectiveOn: record.StartDate,
		},
	); errorValue != nil {
		return errorValue
	}
	nextReferenceID := attendanceLeaveRequestReservationReference(record.ID, nextRevision)
	if _, errorValue := reserveAttendanceLeaveRequestOccurrencesInTransaction(
		ctx,
		transaction,
		attendanceLeaveOperation{
			OperationKey: fmt.Sprintf(
				"leave-request:%s:reserve:%d",
				record.ID,
				nextRevision,
			),
			Employee:    employee,
			LeaveTypeID: record.LeaveTypeID,
			ReferenceID: nextReferenceID,
		},
		[]attendanceLeaveRequestOccurrence{occurrence},
	); errorValue != nil {
		return errorValue
	}
	_, errorValue := closeAttendanceLeaveReservationInTransaction(
		ctx,
		transaction,
		attendanceLeaveOperation{
			OperationKey: fmt.Sprintf(
				"leave-request:%s:use:%d",
				record.ID,
				nextRevision,
			),
			Employee:    employee,
			LeaveTypeID: record.LeaveTypeID,
			Kind:        attendanceLeaveOperationUse,
			ReferenceID: nextReferenceID,
			EffectiveOn: record.StartDate,
		},
	)
	return errorValue
}

func (service *Service) ensureManagedAttendanceLeaveTimeHasNoConflict(
	ctx context.Context,
	transaction *sql.Tx,
	record attendanceLeaveRequestRecord,
	occurrence attendanceLeaveRequestOccurrence,
	now time.Time,
) error {
	var leaveConflictCount int
	if errorValue := transaction.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM attendance_leave_request_occurrences occurrence
JOIN attendance_leave_requests request ON request.id = occurrence.request_id
WHERE request.employee_email = ?
	AND request.status IN (?, ?, ?)
	AND request.id <> ?
	AND occurrence.date = ?
	AND occurrence.start_time < ?
	AND occurrence.end_time > ?`,
		record.EmployeeEmail,
		attendanceLeaveRequestStatusPending,
		attendanceLeaveRequestStatusNeedsChanges,
		attendanceLeaveRequestStatusApproved,
		record.ID,
		occurrence.Date,
		occurrence.EndTime,
		occurrence.StartTime,
	).Scan(&leaveConflictCount); errorValue != nil {
		return errorValue
	}
	if leaveConflictCount > 0 {
		return errAttendanceLeaveRequestConflict
	}
	var absenceConflictCount int
	if errorValue := transaction.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM attendance_absence_occurrences occurrence
LEFT JOIN attendance_leave_request_absence_ranges link
	ON link.range_id = occurrence.range_id AND link.request_id = ?
WHERE occurrence.email = ?
	AND occurrence.date = ?
	AND occurrence.canceled_at = ''
	AND link.range_id IS NULL`,
		record.ID,
		record.EmployeeEmail,
		occurrence.Date,
	).Scan(&absenceConflictCount); errorValue != nil {
		return errorValue
	}
	if absenceConflictCount > 0 {
		return errAttendanceLeaveRequestConflict
	}
	workConflict, errorValue := attendanceLeaveRequestHasConfirmedWorkConflict(
		ctx,
		transaction,
		record.EmployeeEmail,
		occurrence,
		now,
		service.workspaceTimeZone().location,
	)
	if errorValue != nil {
		return errorValue
	}
	if workConflict {
		return errAttendanceLeaveWorkConflict
	}
	return nil
}
