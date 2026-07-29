package admind

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (service *Service) createAttendanceLeaveRequest(
	ctx context.Context,
	employee attendanceLeaveEmployee,
	input attendanceLeaveRequestInput,
	preview attendanceLeaveRequestPreview,
	leaveType attendanceLeaveType,
	attachments []attendanceLeaveRequestAttachment,
	now time.Time,
) (attendanceLeaveRequestRecord, error) {
	requestToken, errorValue := generateRandomURLToken(18)
	if errorValue != nil {
		return attendanceLeaveRequestRecord{}, errorValue
	}
	requestID := "leave-" + requestToken
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
	if errorValue := attendanceLeaveRequestEnsureNoConflict(
		ctx,
		transaction,
		employee.Email,
		preview.Occurrences,
		"",
		now,
		service.workspaceTimeZone().location,
	); errorValue != nil {
		return attendanceLeaveRequestRecord{}, errorValue
	}
	if leaveType.BalanceMode != "none" {
		if _, errorValue := reserveAttendanceLeaveRequestOccurrencesInTransaction(
			ctx,
			transaction,
			attendanceLeaveOperation{
				OperationKey: "leave-request:" + requestID + ":reserve:1",
				Employee:     employee,
				LeaveTypeID:  leaveType.ID,
				ReferenceID:  attendanceLeaveRequestReservationReference(requestID, 1),
			},
			preview.Occurrences,
		); errorValue != nil {
			return attendanceLeaveRequestRecord{}, errorValue
		}
	}
	createdAt := now.UTC().Format(time.RFC3339Nano)
	endDate := input.EndDate
	if endDate == "" {
		endDate = input.StartDate
	}
	record := attendanceLeaveRequestRecord{
		ID:                      requestID,
		EmployeeEmail:           normalizeAttendanceLeaveEmail(employee.Email),
		UserID:                  strings.TrimSpace(employee.UserID),
		LeaveTypeID:             leaveType.ID,
		LeaveTypeName:           leaveType.Name,
		BalanceMode:             leaveType.BalanceMode,
		Status:                  attendanceLeaveRequestStatusPending,
		Unit:                    input.Unit,
		StartDate:               input.StartDate,
		EndDate:                 endDate,
		PartialPeriod:           input.PartialPeriod,
		StartTime:               input.StartTime,
		Reason:                  strings.TrimSpace(input.Reason),
		TotalDeductionMilliDays: preview.TotalDeductionMilliDays,
		Revision:                1,
		CreatedAt:               createdAt,
		UpdatedAt:               createdAt,
		Occurrences:             preview.Occurrences,
		Attachments:             attachments,
	}
	if errorValue := insertAttendanceLeaveRequestRecord(ctx, transaction, record, service.workspaceTimeZone().name); errorValue != nil {
		return attendanceLeaveRequestRecord{}, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return attendanceLeaveRequestRecord{}, errorValue
	}
	for index := range record.Attachments {
		record.Attachments[index].DownloadURL = fmt.Sprintf(
			"/attendance/api/leave-requests/%s/attachments/%s",
			record.ID,
			record.Attachments[index].ID,
		)
	}
	return record, nil
}

func attendanceLeaveRequestReservationReference(requestID string, revision int) string {
	return fmt.Sprintf("%s:revision:%d", requestID, revision)
}

func insertAttendanceLeaveRequestRecord(
	ctx context.Context,
	transaction *sql.Tx,
	record attendanceLeaveRequestRecord,
	timeZone string,
) error {
	_, errorValue := transaction.ExecContext(ctx, `
INSERT INTO attendance_leave_requests (
	id, employee_email, user_id, leave_type_id, leave_type_name, balance_mode,
	unit, partial_period, start_date, end_date, start_time, reason, admin_response,
	status, total_deduction_milli_days, revision, created_at, updated_at, cancelled_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '')`,
		record.ID,
		record.EmployeeEmail,
		record.UserID,
		record.LeaveTypeID,
		record.LeaveTypeName,
		record.BalanceMode,
		record.Unit,
		record.PartialPeriod,
		record.StartDate,
		record.EndDate,
		record.StartTime,
		record.Reason,
		record.AdminResponse,
		record.Status,
		record.TotalDeductionMilliDays,
		record.Revision,
		record.CreatedAt,
		record.UpdatedAt,
	)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := insertAttendanceLeaveRequestOccurrences(ctx, transaction, record, timeZone); errorValue != nil {
		return errorValue
	}
	if errorValue := insertAttendanceLeaveRequestAttachments(
		ctx,
		transaction,
		record.ID,
		record.Attachments,
		record.CreatedAt,
	); errorValue != nil {
		return errorValue
	}
	_, errorValue = transaction.ExecContext(ctx, `
INSERT INTO attendance_leave_request_events (
	id, request_id, kind, actor_email, response, created_at
) VALUES (?, ?, ?, ?, '', ?)`,
		attendanceLeaveDeterministicID("leave-event", record.ID, 1),
		record.ID,
		"submitted",
		record.EmployeeEmail,
		record.CreatedAt,
	)
	return errorValue
}

func insertAttendanceLeaveRequestOccurrences(
	ctx context.Context,
	transaction *sql.Tx,
	record attendanceLeaveRequestRecord,
	timeZone string,
) error {
	for index, occurrence := range record.Occurrences {
		if _, errorValue := transaction.ExecContext(ctx, `
INSERT INTO attendance_leave_request_occurrences (
	id, request_id, date, start_time, end_time, time_zone, deduction_milli_days, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			attendanceLeaveDeterministicID("leave-occurrence", record.ID, index),
			record.ID,
			occurrence.Date,
			occurrence.StartTime,
			occurrence.EndTime,
			timeZone,
			occurrence.DeductionMilliDays,
			record.CreatedAt,
		); errorValue != nil {
			return errorValue
		}
	}
	return nil
}
