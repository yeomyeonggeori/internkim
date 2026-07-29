package admind

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"
)

func (service *Service) cancelAttendanceLeaveRequest(
	ctx context.Context,
	requestID string,
	employee attendanceLeaveEmployee,
	now time.Time,
) (returnError error) {
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
		employee.Email,
	)
	if errorValue != nil {
		return errorValue
	}
	employee.UserID = record.UserID
	attachments := []attendanceLeaveRequestAttachment{}
	removeAttachments := false
	quarantinedAttachments := []attendanceLeaveRequestAttachmentQuarantine{}
	restoreQuarantinedAttachments := false
	defer func() {
		if restoreQuarantinedAttachments {
			returnError = errors.Join(
				returnError,
				restoreAttendanceLeaveRequestAttachmentFiles(quarantinedAttachments),
			)
		}
	}()
	switch record.Status {
	case attendanceLeaveRequestStatusPending, attendanceLeaveRequestStatusNeedsChanges:
		attachments, errorValue = readAttendanceLeaveRequestAttachmentInTransaction(ctx, transaction, record.ID)
		if errorValue != nil {
			return errorValue
		}
		quarantinedAttachments, errorValue = quarantineAttendanceLeaveRequestAttachmentFiles(
			service.attendanceLeaveAttachmentDirectory(),
			attachments,
		)
		restoreQuarantinedAttachments = len(quarantinedAttachments) > 0
		if errorValue != nil {
			return errorValue
		}
		if record.BalanceMode != "none" {
			if _, errorValue := closeAttendanceLeaveReservationInTransaction(ctx, transaction, attendanceLeaveOperation{
				OperationKey: "leave-request:" + record.ID + ":release:" + attendanceLeaveRequestRevisionValue(record.Revision),
				Employee:     employee,
				LeaveTypeID:  record.LeaveTypeID,
				Kind:         attendanceLeaveOperationRelease,
				ReferenceID:  attendanceLeaveRequestReservationReference(record.ID, record.Revision),
				EffectiveOn:  record.StartDate,
			}); errorValue != nil {
				return errorValue
			}
		}
		if _, errorValue := transaction.ExecContext(
			ctx,
			`DELETE FROM attendance_leave_requests WHERE id = ? AND employee_email = ?`,
			record.ID,
			normalizeAttendanceLeaveEmail(employee.Email),
		); errorValue != nil {
			return errorValue
		}
		removeAttachments = true
	case attendanceLeaveRequestStatusApproved:
		if errorValue := service.cancelApprovedAttendanceLeaveRequestInTransaction(
			ctx,
			transaction,
			record,
			employee,
			record.EmployeeEmail,
			now,
			false,
		); errorValue != nil {
			return errorValue
		}
	default:
		return errAttendanceLeaveRequestCannotCancel
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return errorValue
	}
	if removeAttachments {
		restoreQuarantinedAttachments = false
		cleanupCancelledAttendanceLeaveRequestAttachments(
			ctx,
			record.ID,
			quarantinedAttachments,
		)
	}
	return nil
}

func (service *Service) cancelApprovedAttendanceLeaveRequestInTransaction(
	ctx context.Context,
	transaction *sql.Tx,
	record attendanceLeaveRequestRecord,
	employee attendanceLeaveEmployee,
	actorEmail string,
	now time.Time,
	allowStarted bool,
) error {
	if !allowStarted &&
		!attendanceLeaveRequestStartsAfter(record, now, service.workspaceTimeZone().location) {
		return errAttendanceLeaveRequestAlreadyStarted
	}
	if record.BalanceMode != "none" {
		if _, errorValue := restoreAttendanceLeaveUseInTransaction(ctx, transaction, attendanceLeaveOperation{
			OperationKey: "leave-request:" + record.ID + ":restore:" + attendanceLeaveRequestRevisionValue(record.Revision),
			Employee:     employee,
			LeaveTypeID:  record.LeaveTypeID,
			ReferenceID:  attendanceLeaveRequestReservationReference(record.ID, record.Revision),
			EffectiveOn:  record.StartDate,
		}); errorValue != nil {
			return errorValue
		}
	}
	if errorValue := cancelAttendanceLeaveRequestAbsences(ctx, transaction, record.ID, now); errorValue != nil {
		return errorValue
	}
	if errorValue := cancelApprovedLeaveClockOutsInTransaction(
		ctx,
		transaction,
		record.ID,
		now,
	); errorValue != nil {
		return errorValue
	}
	occurrenceDates := make([]string, 0, len(record.Occurrences))
	for _, occurrence := range record.Occurrences {
		occurrenceDates = append(occurrenceDates, occurrence.Date)
	}
	if errorValue := invalidateAttendanceAbsenceCacheDates(ctx, transaction, occurrenceDates); errorValue != nil {
		return errorValue
	}
	cancelledAt := now.UTC().Format(time.RFC3339Nano)
	result, errorValue := transaction.ExecContext(ctx, `
UPDATE attendance_leave_requests
SET status = ?, cancelled_at = ?, updated_at = ?
WHERE id = ? AND employee_email = ? AND status = ?`,
		attendanceLeaveRequestStatusCancelled,
		cancelledAt,
		cancelledAt,
		record.ID,
		normalizeAttendanceLeaveEmail(employee.Email),
		attendanceLeaveRequestStatusApproved,
	)
	if errorValue != nil {
		return errorValue
	}
	updatedRows, errorValue := result.RowsAffected()
	if errorValue != nil {
		return errorValue
	}
	if updatedRows != 1 {
		return errAttendanceLeaveRequestCannotCancel
	}
	_, errorValue = transaction.ExecContext(ctx, `
INSERT INTO attendance_leave_request_events (
	id, request_id, kind, actor_email, response, created_at
) VALUES (?, ?, ?, ?, '', ?)`,
		attendanceLeaveDeterministicID("leave-event-cancelled", record.ID, record.Revision),
		record.ID,
		attendanceLeaveRequestStatusCancelled,
		normalizeAttendanceLeaveEmail(actorEmail),
		cancelledAt,
	)
	return errorValue
}

func readAttendanceLeaveRequestRecordInTransaction(
	ctx context.Context,
	transaction *sql.Tx,
	requestID string,
	employeeEmail string,
) (attendanceLeaveRequestRecord, error) {
	var record attendanceLeaveRequestRecord
	errorValue := transaction.QueryRowContext(ctx, `
SELECT
	id, employee_email, user_id, leave_type_id, leave_type_name, balance_mode,
	status, unit, start_date, end_date, partial_period, start_time, reason,
	admin_response, total_deduction_milli_days, revision, created_at, updated_at, cancelled_at
FROM attendance_leave_requests
WHERE id = ? AND employee_email = ?`,
		strings.TrimSpace(requestID),
		normalizeAttendanceLeaveEmail(employeeEmail),
	).Scan(
		&record.ID,
		&record.EmployeeEmail,
		&record.UserID,
		&record.LeaveTypeID,
		&record.LeaveTypeName,
		&record.BalanceMode,
		&record.Status,
		&record.Unit,
		&record.StartDate,
		&record.EndDate,
		&record.PartialPeriod,
		&record.StartTime,
		&record.Reason,
		&record.AdminResponse,
		&record.TotalDeductionMilliDays,
		&record.Revision,
		&record.CreatedAt,
		&record.UpdatedAt,
		&record.CancelledAt,
	)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return attendanceLeaveRequestRecord{}, errAttendanceLeaveRequestNotFound
	}
	if errorValue != nil {
		return attendanceLeaveRequestRecord{}, errorValue
	}
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT date, start_time, end_time, deduction_milli_days
FROM attendance_leave_request_occurrences
WHERE request_id = ?
ORDER BY date, start_time, id`,
		record.ID,
	)
	if errorValue != nil {
		return attendanceLeaveRequestRecord{}, errorValue
	}
	defer rows.Close()
	record.Occurrences = []attendanceLeaveRequestOccurrence{}
	for rows.Next() {
		var occurrence attendanceLeaveRequestOccurrence
		if errorValue := rows.Scan(
			&occurrence.Date,
			&occurrence.StartTime,
			&occurrence.EndTime,
			&occurrence.DeductionMilliDays,
		); errorValue != nil {
			return attendanceLeaveRequestRecord{}, errorValue
		}
		record.Occurrences = append(record.Occurrences, occurrence)
	}
	if errorValue := rows.Err(); errorValue != nil {
		return attendanceLeaveRequestRecord{}, errorValue
	}
	return record, nil
}

func cancelAttendanceLeaveRequestAbsences(
	ctx context.Context,
	transaction *sql.Tx,
	requestID string,
	now time.Time,
) error {
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT range_id
FROM attendance_leave_request_absence_ranges
WHERE request_id = ?`,
		requestID,
	)
	if errorValue != nil {
		return errorValue
	}
	defer rows.Close()
	rangeIDs := []string{}
	for rows.Next() {
		var rangeID string
		if errorValue := rows.Scan(&rangeID); errorValue != nil {
			return errorValue
		}
		rangeIDs = append(rangeIDs, rangeID)
	}
	if errorValue := rows.Err(); errorValue != nil {
		return errorValue
	}
	if errorValue := rows.Close(); errorValue != nil {
		return errorValue
	}
	for _, rangeID := range rangeIDs {
		if errorValue := cancelAttendanceAbsenceRangeInTransaction(
			ctx,
			transaction,
			rangeID,
			now.UTC().Format(time.RFC3339Nano),
			"",
		); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func attendanceLeaveRequestRevisionValue(revision int) string {
	return strconv.Itoa(revision)
}
