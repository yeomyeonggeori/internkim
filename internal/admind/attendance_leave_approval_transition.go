package admind

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (service *Service) decideAttendanceLeaveRequest(
	ctx context.Context,
	requestID string,
	administratorEmail string,
	input attendanceLeaveApprovalInput,
	now time.Time,
) (attendanceLeaveApprovalRequestView, error) {
	action, response, errorValue := normalizeAttendanceLeaveApprovalInput(input)
	if errorValue != nil {
		return attendanceLeaveApprovalRequestView{}, errorValue
	}
	status := attendanceLeaveApprovalStatus(action)
	policy, errorValue := service.readAttendanceLeavePolicy(ctx)
	if errorValue != nil {
		return attendanceLeaveApprovalRequestView{}, errorValue
	}
	database, errorValue := service.openAttendanceLeaveMutationDatabase(ctx)
	if errorValue != nil {
		return attendanceLeaveApprovalRequestView{}, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return attendanceLeaveApprovalRequestView{}, errorValue
	}
	defer transaction.Rollback()
	updatedAt := now.UTC().Format(time.RFC3339Nano)
	employeeEmail, errorValue := claimAttendanceLeaveApproval(
		ctx,
		transaction,
		requestID,
		status,
		response,
		updatedAt,
	)
	if errorValue != nil {
		return attendanceLeaveApprovalRequestView{}, errorValue
	}
	record, errorValue := readAttendanceLeaveRequestRecordInTransaction(
		ctx,
		transaction,
		requestID,
		employeeEmail,
	)
	if errorValue != nil {
		return attendanceLeaveApprovalRequestView{}, errorValue
	}
	record.Attachments, errorValue = readAttendanceLeaveRequestAttachmentInTransaction(
		ctx,
		transaction,
		record.ID,
	)
	if errorValue != nil {
		return attendanceLeaveApprovalRequestView{}, errorValue
	}
	record.Status = status
	record.AdminResponse = response
	record.UpdatedAt = updatedAt
	employee := attendanceLeaveEmployee{
		Email:  record.EmployeeEmail,
		UserID: record.UserID,
	}
	switch action {
	case attendanceLeaveApprovalActionApprove:
		record, errorValue = service.approveAttendanceLeaveRequest(
			ctx,
			transaction,
			record,
			employee,
			policy,
			now,
		)
		if errorValue != nil {
			return attendanceLeaveApprovalRequestView{}, errorValue
		}
	case attendanceLeaveApprovalActionReject:
		if record.BalanceMode != "none" {
			if _, errorValue := closeAttendanceLeaveReservationInTransaction(
				ctx,
				transaction,
				attendanceLeaveOperation{
					OperationKey: "leave-request:" + record.ID + ":release:" + attendanceLeaveRequestRevisionValue(record.Revision),
					Employee:     employee,
					LeaveTypeID:  attendanceLeaveBalanceAccountID(record.LeaveTypeID, record.BalanceMode),
					Kind:         attendanceLeaveOperationRelease,
					ReferenceID:  attendanceLeaveRequestReservationReference(record.ID, record.Revision),
					EffectiveOn:  record.StartDate,
				},
			); errorValue != nil {
				return attendanceLeaveApprovalRequestView{}, errorValue
			}
		}
	}
	if errorValue := insertAttendanceLeaveApprovalEvent(
		ctx,
		transaction,
		record,
		normalizeAttendanceLeaveEmail(administratorEmail),
		status,
		response,
		updatedAt,
	); errorValue != nil {
		return attendanceLeaveApprovalRequestView{}, errorValue
	}
	balance, errorValue := service.attendanceLeaveApprovalBalance(
		ctx,
		transaction,
		record,
		policy,
	)
	if errorValue != nil {
		return attendanceLeaveApprovalRequestView{}, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return attendanceLeaveApprovalRequestView{}, errorValue
	}
	for index := range record.Attachments {
		record.Attachments[index].DownloadURL = fmt.Sprintf(
			"/attendance/api/leave-requests/%s/attachments/%s",
			record.ID,
			record.Attachments[index].ID,
		)
	}
	return projectAttendanceLeaveApprovalRequestRecord(record, balance), nil
}

func normalizeAttendanceLeaveApprovalInput(
	input attendanceLeaveApprovalInput,
) (string, string, error) {
	action := strings.TrimSpace(input.Action)
	response := strings.TrimSpace(input.Response)
	switch action {
	case attendanceLeaveApprovalActionApprove,
		attendanceLeaveApprovalActionReject:
	case attendanceLeaveApprovalActionNeedsChanges:
		if response == "" {
			return "", "", attendanceLeaveInvalidInputErrorf("response is required when requesting changes")
		}
	default:
		return "", "", attendanceLeaveInvalidInputErrorf("unsupported leave approval action")
	}
	return action, response, nil
}

func attendanceLeaveApprovalStatus(action string) string {
	switch action {
	case attendanceLeaveApprovalActionApprove:
		return attendanceLeaveRequestStatusApproved
	case attendanceLeaveApprovalActionReject:
		return attendanceLeaveRequestStatusRejected
	default:
		return attendanceLeaveRequestStatusNeedsChanges
	}
}

func claimAttendanceLeaveApproval(
	ctx context.Context,
	transaction *sql.Tx,
	requestID string,
	status string,
	response string,
	updatedAt string,
) (string, error) {
	var employeeEmail string
	errorValue := transaction.QueryRowContext(ctx, `
UPDATE attendance_leave_requests
SET status = ?, admin_response = ?, updated_at = ?
WHERE id = ? AND status = ?
RETURNING employee_email`,
		status,
		response,
		updatedAt,
		strings.TrimSpace(requestID),
		attendanceLeaveRequestStatusPending,
	).Scan(&employeeEmail)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return "", errAttendanceLeaveApprovalConflict
	}
	if errorValue != nil {
		return "", errorValue
	}
	return normalizeAttendanceLeaveEmail(employeeEmail), nil
}

func (service *Service) approveAttendanceLeaveRequest(
	ctx context.Context,
	transaction *sql.Tx,
	record attendanceLeaveRequestRecord,
	employee attendanceLeaveEmployee,
	policy attendanceLeavePolicy,
	now time.Time,
) (attendanceLeaveRequestRecord, error) {
	if errorValue := attendanceLeaveRequestEnsureNoConflict(
		ctx,
		transaction,
		record.EmployeeEmail,
		record.Occurrences,
		record.ID,
		now,
		service.workspaceTimeZone().location,
	); errorValue != nil {
		return attendanceLeaveRequestRecord{}, errorValue
	}
	if policy.BalanceTrackingMode == attendanceLeaveBalanceTrackingManaged &&
		record.BalanceMode == "none" {
		leaveType, found := attendanceLeaveTypeByID(policy, record.LeaveTypeID)
		if found && leaveType.IsActive && leaveType.BalanceMode != "none" {
			nextRevision := record.Revision + 1
			if _, errorValue := service.reserveAttendanceLeaveRequestOccurrencesInTransaction(
				ctx,
				transaction,
				attendanceLeaveOperation{
					OperationKey: fmt.Sprintf(
						"leave-request:%s:reserve:%d",
						record.ID,
						nextRevision,
					),
					Employee: employee,
					LeaveTypeID: attendanceLeaveBalanceAccountID(
						leaveType.ID,
						leaveType.BalanceMode,
					),
					ReferenceID: attendanceLeaveRequestReservationReference(
						record.ID,
						nextRevision,
					),
				},
				record.Occurrences,
				policy,
				record.ID,
			); errorValue != nil {
				return attendanceLeaveRequestRecord{}, errorValue
			}
			result, errorValue := transaction.ExecContext(ctx, `
UPDATE attendance_leave_requests
SET leave_type_name = ?, balance_mode = ?, revision = ?, updated_at = ?
WHERE id = ? AND status = ? AND balance_mode = 'none' AND revision = ?`,
				leaveType.Name,
				leaveType.BalanceMode,
				nextRevision,
				now.UTC().Format(time.RFC3339Nano),
				record.ID,
				attendanceLeaveRequestStatusApproved,
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
				return attendanceLeaveRequestRecord{}, errAttendanceLeaveApprovalConflict
			}
			record.LeaveTypeName = leaveType.Name
			record.BalanceMode = leaveType.BalanceMode
			record.Revision = nextRevision
			record.UpdatedAt = now.UTC().Format(time.RFC3339Nano)
		}
	}
	if record.BalanceMode != "none" {
		if _, errorValue := closeAttendanceLeaveReservationInTransaction(
			ctx,
			transaction,
			attendanceLeaveOperation{
				OperationKey: "leave-request:" + record.ID + ":use:" + attendanceLeaveRequestRevisionValue(record.Revision),
				Employee:     employee,
				LeaveTypeID:  attendanceLeaveBalanceAccountID(record.LeaveTypeID, record.BalanceMode),
				Kind:         attendanceLeaveOperationUse,
				ReferenceID:  attendanceLeaveRequestReservationReference(record.ID, record.Revision),
				EffectiveOn:  record.StartDate,
			},
		); errorValue != nil {
			return attendanceLeaveRequestRecord{}, errorValue
		}
	} else {
		if _, errorValue := recordUntrackedAttendanceLeaveUseInTransaction(
			ctx,
			transaction,
			attendanceLeaveOperation{
				OperationKey: "leave-request:" + record.ID + ":untracked-use:" + attendanceLeaveRequestRevisionValue(record.Revision),
				Employee:     employee,
				LeaveTypeID:  record.LeaveTypeID,
				ReferenceID:  attendanceLeaveRequestReservationReference(record.ID, record.Revision),
				Amount:       record.TotalDeductionMilliDays,
				EffectiveOn:  record.StartDate,
			},
		); errorValue != nil {
			return attendanceLeaveRequestRecord{}, errorValue
		}
	}
	if errorValue := createAttendanceLeaveApprovalAbsences(ctx, transaction, record, now); errorValue != nil {
		return attendanceLeaveRequestRecord{}, errorValue
	}
	return record, nil
}

func createAttendanceLeaveApprovalAbsences(
	ctx context.Context,
	transaction *sql.Tx,
	record attendanceLeaveRequestRecord,
	now time.Time,
) error {
	dateSet := map[string]struct{}{}
	for _, occurrence := range record.Occurrences {
		dateSet[occurrence.Date] = struct{}{}
	}
	dates := sortedAttendanceDatesFromSet(dateSet)
	timestamp := now.UTC().Format(time.RFC3339Nano)
	ranges, errorValue := insertAttendanceAbsenceRangesForDates(
		ctx,
		transaction,
		record.EmployeeEmail,
		"leave",
		record.Reason,
		record.EmployeeEmail,
		timestamp,
		timestamp,
		dates,
	)
	if errorValue != nil {
		return errorValue
	}
	for _, absenceRange := range ranges {
		if errorValue := linkAttendanceLeaveRequestAbsenceRange(
			ctx,
			transaction,
			record.ID,
			absenceRange.ID,
		); errorValue != nil {
			return errorValue
		}
	}
	return invalidateAttendanceAbsenceCacheDates(ctx, transaction, dates)
}

func insertAttendanceLeaveApprovalEvent(
	ctx context.Context,
	transaction *sql.Tx,
	record attendanceLeaveRequestRecord,
	administratorEmail string,
	action string,
	response string,
	createdAt string,
) error {
	_, errorValue := transaction.ExecContext(ctx, `
INSERT INTO attendance_leave_request_events (
	id, request_id, kind, actor_email, response, created_at
) VALUES (?, ?, ?, ?, ?, ?)`,
		attendanceLeaveDeterministicID("leave-event-"+action, record.ID, record.Revision),
		record.ID,
		action,
		administratorEmail,
		response,
		createdAt,
	)
	return errorValue
}
