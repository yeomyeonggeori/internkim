package admind

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type attendanceLeaveRequestRevisionOperation struct {
	ExpectedStatus       string
	NextStatus           string
	ExpectedRevision     int
	EventKind            string
	EventResponse        string
	RemovedAttachmentIDs []string
	ConflictError        error
}

func (service *Service) reviseAttendanceLeaveRequest(
	ctx context.Context,
	requestID string,
	employee attendanceLeaveEmployee,
	input attendanceLeaveRequestInput,
	preview attendanceLeaveRequestPreview,
	leaveType attendanceLeaveType,
	policy attendanceLeavePolicy,
	attachments []attendanceLeaveRequestAttachment,
	operation attendanceLeaveRequestRevisionOperation,
	now time.Time,
) (record attendanceLeaveRequestRecord, returnError error) {
	database, errorValue := service.openAttendanceLeaveMutationDatabase(ctx)
	if errorValue != nil {
		return attendanceLeaveRequestRecord{}, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return attendanceLeaveRequestRecord{}, errorValue
	}
	defer transaction.Rollback()
	record, errorValue = readAttendanceLeaveRequestRecordInTransaction(
		ctx,
		transaction,
		requestID,
		employee.Email,
	)
	if errorValue != nil {
		return attendanceLeaveRequestRecord{}, errorValue
	}
	if record.Status != operation.ExpectedStatus ||
		(operation.ExpectedRevision > 0 && record.Revision != operation.ExpectedRevision) {
		return attendanceLeaveRequestRecord{}, operation.ConflictError
	}
	existingAttachments, errorValue := readAttendanceLeaveRequestAttachmentInTransaction(ctx, transaction, record.ID)
	if errorValue != nil {
		return attendanceLeaveRequestRecord{}, errorValue
	}
	keptAttachments, removedAttachments, errorValue := splitAttendanceLeaveRequestAttachments(
		existingAttachments,
		operation.RemovedAttachmentIDs,
	)
	if errorValue != nil {
		return attendanceLeaveRequestRecord{}, errorValue
	}
	if len(keptAttachments)+len(attachments) > attendanceLeaveRequestMaximumAttachmentCount {
		return attendanceLeaveRequestRecord{}, fmt.Errorf(
			"%w: leave request supports at most %d attachments",
			errAttendanceLeaveRequestAttachmentLimit,
			attendanceLeaveRequestMaximumAttachmentCount,
		)
	}
	quarantinedAttachments, errorValue := quarantineAttendanceLeaveRequestAttachmentFiles(
		service.attendanceLeaveAttachmentDirectory(),
		removedAttachments,
	)
	restoreQuarantinedAttachments := len(quarantinedAttachments) > 0
	defer func() {
		if restoreQuarantinedAttachments {
			returnError = errors.Join(
				returnError,
				restoreAttendanceLeaveRequestAttachmentFiles(quarantinedAttachments),
			)
		}
	}()
	if errorValue != nil {
		return attendanceLeaveRequestRecord{}, errorValue
	}
	employee.UserID = record.UserID
	if record.BalanceMode != "none" {
		if _, errorValue := closeAttendanceLeaveReservationInTransaction(ctx, transaction, attendanceLeaveOperation{
			OperationKey: fmt.Sprintf("leave-request:%s:release:%d", record.ID, record.Revision),
			Employee:     employee,
			LeaveTypeID:  attendanceLeaveBalanceAccountID(record.LeaveTypeID, record.BalanceMode),
			Kind:         attendanceLeaveOperationRelease,
			ReferenceID:  attendanceLeaveRequestReservationReference(record.ID, record.Revision),
			EffectiveOn:  record.StartDate,
		}); errorValue != nil {
			return attendanceLeaveRequestRecord{}, errorValue
		}
	}
	if errorValue := attendanceLeaveRequestEnsureNoConflict(
		ctx,
		transaction,
		employee.Email,
		preview.Occurrences,
		record.ID,
		now,
		service.workspaceTimeZone().location,
	); errorValue != nil {
		return attendanceLeaveRequestRecord{}, errorValue
	}
	nextRevision := record.Revision + 1
	if leaveType.BalanceMode != "none" {
		if _, errorValue := service.reserveAttendanceLeaveRequestOccurrencesInTransaction(
			ctx,
			transaction,
			attendanceLeaveOperation{
				OperationKey: fmt.Sprintf("leave-request:%s:reserve:%d", record.ID, nextRevision),
				Employee:     employee,
				LeaveTypeID:  attendanceLeaveBalanceAccountID(leaveType.ID, leaveType.BalanceMode),
				ReferenceID:  attendanceLeaveRequestReservationReference(record.ID, nextRevision),
			},
			preview.Occurrences,
			policy,
			"",
		); errorValue != nil {
			return attendanceLeaveRequestRecord{}, errorValue
		}
	}
	endDate := input.EndDate
	if endDate == "" {
		endDate = input.StartDate
	}
	updatedAt := now.UTC().Format(time.RFC3339Nano)
	result, errorValue := transaction.ExecContext(ctx, `
UPDATE attendance_leave_requests
SET leave_type_id = ?,
	leave_type_name = ?,
	balance_mode = ?,
	unit = ?,
	partial_period = ?,
	start_date = ?,
	end_date = ?,
	start_time = ?,
	reason = ?,
	status = ?,
	total_deduction_milli_days = ?,
	revision = ?,
	updated_at = ?,
	cancelled_at = ''
WHERE id = ? AND employee_email = ? AND status = ? AND revision = ?`,
		leaveType.ID,
		leaveType.Name,
		leaveType.BalanceMode,
		input.Unit,
		input.PartialPeriod,
		input.StartDate,
		endDate,
		input.StartTime,
		input.Reason,
		operation.NextStatus,
		preview.TotalDeductionMilliDays,
		nextRevision,
		updatedAt,
		record.ID,
		normalizeAttendanceLeaveEmail(employee.Email),
		operation.ExpectedStatus,
		record.Revision,
	)
	if errorValue != nil {
		return attendanceLeaveRequestRecord{}, errorValue
	}
	updatedRows, errorValue := result.RowsAffected()
	if errorValue != nil {
		return attendanceLeaveRequestRecord{}, errorValue
	}
	if updatedRows != 1 {
		return attendanceLeaveRequestRecord{}, operation.ConflictError
	}
	if errorValue := replaceAttendanceLeaveRequestOccurrences(
		ctx,
		transaction,
		&record,
		input,
		preview,
		leaveType,
		operation.NextStatus,
		nextRevision,
		updatedAt,
		service.workspaceTimeZone().name,
	); errorValue != nil {
		return attendanceLeaveRequestRecord{}, errorValue
	}
	if errorValue := deleteAttendanceLeaveRequestAttachments(
		ctx,
		transaction,
		record.ID,
		removedAttachments,
	); errorValue != nil {
		return attendanceLeaveRequestRecord{}, errorValue
	}
	if errorValue := insertAttendanceLeaveRequestRevisionEvent(
		ctx,
		transaction,
		record,
		operation.EventKind,
		operation.EventResponse,
	); errorValue != nil {
		return attendanceLeaveRequestRecord{}, errorValue
	}
	if errorValue := insertAttendanceLeaveRequestAttachments(
		ctx,
		transaction,
		record.ID,
		attachments,
		record.UpdatedAt,
	); errorValue != nil {
		return attendanceLeaveRequestRecord{}, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return attendanceLeaveRequestRecord{}, errorValue
	}
	restoreQuarantinedAttachments = false
	cleanupCancelledAttendanceLeaveRequestAttachments(
		ctx,
		record.ID,
		quarantinedAttachments,
	)
	record.Attachments = append(keptAttachments, attachments...)
	for index := range record.Attachments {
		record.Attachments[index].DownloadURL = fmt.Sprintf(
			"/attendance/api/leave-requests/%s/attachments/%s",
			record.ID,
			record.Attachments[index].ID,
		)
	}
	return record, nil
}

func splitAttendanceLeaveRequestAttachments(
	existing []attendanceLeaveRequestAttachment,
	removedIDs []string,
) ([]attendanceLeaveRequestAttachment, []attendanceLeaveRequestAttachment, error) {
	removedSet := make(map[string]struct{}, len(removedIDs))
	for _, attachmentID := range removedIDs {
		removedSet[attachmentID] = struct{}{}
	}
	kept := make([]attendanceLeaveRequestAttachment, 0, len(existing))
	removed := make([]attendanceLeaveRequestAttachment, 0, len(removedIDs))
	for _, attachment := range existing {
		if _, found := removedSet[attachment.ID]; found {
			removed = append(removed, attachment)
			delete(removedSet, attachment.ID)
			continue
		}
		kept = append(kept, attachment)
	}
	if len(removedSet) > 0 {
		return nil, nil, attendanceLeaveInvalidInputErrorf("attachment does not belong to leave request")
	}
	return kept, removed, nil
}

func replaceAttendanceLeaveRequestOccurrences(
	ctx context.Context,
	transaction *sql.Tx,
	record *attendanceLeaveRequestRecord,
	input attendanceLeaveRequestInput,
	preview attendanceLeaveRequestPreview,
	leaveType attendanceLeaveType,
	status string,
	revision int,
	updatedAt string,
	timeZoneName string,
) error {
	if _, errorValue := transaction.ExecContext(
		ctx,
		`DELETE FROM attendance_leave_request_occurrences WHERE request_id = ?`,
		record.ID,
	); errorValue != nil {
		return errorValue
	}
	endDate := input.EndDate
	if endDate == "" {
		endDate = input.StartDate
	}
	record.LeaveTypeID = leaveType.ID
	record.LeaveTypeName = leaveType.Name
	record.BalanceMode = leaveType.BalanceMode
	record.Status = status
	record.Unit = input.Unit
	record.PartialPeriod = input.PartialPeriod
	record.StartDate = input.StartDate
	record.EndDate = endDate
	record.StartTime = input.StartTime
	record.Reason = input.Reason
	record.TotalDeductionMilliDays = preview.TotalDeductionMilliDays
	record.Revision = revision
	record.UpdatedAt = updatedAt
	record.Occurrences = preview.Occurrences
	return insertAttendanceLeaveRequestOccurrences(ctx, transaction, *record, timeZoneName)
}

func deleteAttendanceLeaveRequestAttachments(
	ctx context.Context,
	transaction *sql.Tx,
	requestID string,
	attachments []attendanceLeaveRequestAttachment,
) error {
	for _, attachment := range attachments {
		result, errorValue := transaction.ExecContext(
			ctx,
			`DELETE FROM attendance_leave_request_attachments WHERE request_id = ? AND id = ?`,
			requestID,
			attachment.ID,
		)
		if errorValue != nil {
			return errorValue
		}
		deletedRows, errorValue := result.RowsAffected()
		if errorValue != nil {
			return errorValue
		}
		if deletedRows != 1 {
			return attendanceLeaveInvalidInputErrorf("attachment does not belong to leave request")
		}
	}
	return nil
}

func insertAttendanceLeaveRequestRevisionEvent(
	ctx context.Context,
	transaction *sql.Tx,
	record attendanceLeaveRequestRecord,
	kind string,
	response string,
) error {
	deterministicKind := "leave-event-" + kind
	if kind == "resubmitted" {
		deterministicKind = "leave-event"
	}
	_, errorValue := transaction.ExecContext(ctx, `
INSERT INTO attendance_leave_request_events (
	id, request_id, kind, actor_email, response, created_at
) VALUES (?, ?, ?, ?, ?, ?)`,
		attendanceLeaveDeterministicID(deterministicKind, record.ID, record.Revision),
		record.ID,
		kind,
		record.EmployeeEmail,
		response,
		record.UpdatedAt,
	)
	return errorValue
}
