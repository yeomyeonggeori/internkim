package admind

import (
	"context"
	"database/sql"
	"time"
)

func (service *Service) readAttendanceLeaveApprovalInbox(
	ctx context.Context,
) (attendanceLeaveApprovalInbox, error) {
	policy, errorValue := service.readAttendanceLeavePolicy(ctx)
	if errorValue != nil {
		return attendanceLeaveApprovalInbox{}, errorValue
	}
	database, errorValue := service.openAttendanceLeaveMutationDatabase(ctx)
	if errorValue != nil {
		return attendanceLeaveApprovalInbox{}, errorValue
	}
	defer database.Close()
	pendingRecords, errorValue := readAttendanceLeaveApprovalPendingRecords(ctx, database)
	if errorValue != nil {
		return attendanceLeaveApprovalInbox{}, errorValue
	}
	pending := make([]attendanceLeaveApprovalRequestView, 0, len(pendingRecords))
	for _, record := range pendingRecords {
		view, viewError := service.projectAttendanceLeaveApprovalRequest(
			ctx,
			database,
			record,
			policy,
		)
		if viewError != nil {
			return attendanceLeaveApprovalInbox{}, viewError
		}
		pending = append(pending, view)
	}
	recentChanges, errorValue := service.readAttendanceLeaveApprovalRecentChanges(
		ctx,
		database,
		policy,
	)
	if errorValue != nil {
		return attendanceLeaveApprovalInbox{}, errorValue
	}
	return attendanceLeaveApprovalInbox{
		PendingCount:  len(pending),
		Pending:       pending,
		RecentChanges: recentChanges,
	}, nil
}

func readAttendanceLeaveApprovalPendingRecords(
	ctx context.Context,
	database *sql.DB,
) ([]attendanceLeaveRequestRecord, error) {
	rows, errorValue := database.QueryContext(ctx, `
SELECT
	id, employee_email, user_id, leave_type_id, leave_type_name, balance_mode,
	status, unit, start_date, end_date, partial_period, start_time, reason,
	admin_response, total_deduction_milli_days, revision, created_at, updated_at, cancelled_at
FROM attendance_leave_requests
WHERE status = ?
ORDER BY created_at, id`,
		attendanceLeaveRequestStatusPending,
	)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	records := []attendanceLeaveRequestRecord{}
	for rows.Next() {
		record, scanError := scanAttendanceLeaveApprovalRequest(rows)
		if scanError != nil {
			return nil, scanError
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (service *Service) readAttendanceLeaveApprovalRecentChanges(
	ctx context.Context,
	database *sql.DB,
	policy attendanceLeavePolicy,
) ([]attendanceLeaveApprovalChangeView, error) {
	rows, errorValue := database.QueryContext(ctx, `
SELECT
	request.id, request.employee_email, request.user_id, request.leave_type_id,
	request.leave_type_name, request.balance_mode, request.status, request.unit,
	request.start_date, request.end_date, request.partial_period, request.start_time,
	request.reason, request.admin_response, request.total_deduction_milli_days,
	request.revision, request.created_at, request.updated_at, request.cancelled_at,
	event.kind, event.response, event.created_at
FROM attendance_leave_request_events event
JOIN attendance_leave_requests request ON request.id = event.request_id
WHERE event.kind IN (?, ?, ?, ?, ?)
ORDER BY event.created_at DESC, event.id DESC
LIMIT 50`,
		attendanceLeaveRequestStatusApproved,
		attendanceLeaveRequestStatusNeedsChanges,
		attendanceLeaveRequestStatusRejected,
		attendanceLeaveRequestStatusCancelled,
		attendanceLeaveApprovalChangeEarlyReturn,
	)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	type changeRecord struct {
		request   attendanceLeaveRequestRecord
		change    string
		response  string
		changedAt string
	}
	records := []changeRecord{}
	for rows.Next() {
		var record changeRecord
		scanValues := attendanceLeaveApprovalRequestScanValues(&record.request)
		scanValues = append(scanValues, &record.change, &record.response, &record.changedAt)
		if errorValue := rows.Scan(scanValues...); errorValue != nil {
			return nil, errorValue
		}
		records = append(records, record)
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	changes := make([]attendanceLeaveApprovalChangeView, 0, len(records))
	for _, record := range records {
		view, viewError := service.projectAttendanceLeaveApprovalRequest(
			ctx,
			database,
			record.request,
			policy,
		)
		if viewError != nil {
			return nil, viewError
		}
		change := attendanceLeaveApprovalChangeView{
			Request:   view,
			Change:    record.change,
			ChangedAt: record.changedAt,
		}
		if record.change == attendanceLeaveApprovalChangeEarlyReturn {
			change.ReturnedAt = record.response
		} else {
			change.Response = record.response
		}
		changes = append(changes, change)
	}
	return changes, nil
}

func (service *Service) projectAttendanceLeaveApprovalRequest(
	ctx context.Context,
	database *sql.DB,
	record attendanceLeaveRequestRecord,
	policy attendanceLeavePolicy,
) (attendanceLeaveApprovalRequestView, error) {
	occurrences, errorValue := readAttendanceLeaveRequestOccurrences(ctx, database, record.ID)
	if errorValue != nil {
		return attendanceLeaveApprovalRequestView{}, errorValue
	}
	attachments, errorValue := readAttendanceLeaveRequestAttachments(ctx, database, record.ID)
	if errorValue != nil {
		return attendanceLeaveApprovalRequestView{}, errorValue
	}
	record.Occurrences = occurrences
	record.Attachments = attachments
	balance, errorValue := service.attendanceLeaveApprovalBalance(
		ctx,
		database,
		record,
		policy,
	)
	if errorValue != nil {
		return attendanceLeaveApprovalRequestView{}, errorValue
	}
	return projectAttendanceLeaveApprovalRequestRecord(record, balance), nil
}

func projectAttendanceLeaveApprovalRequestRecord(
	record attendanceLeaveRequestRecord,
	balance attendanceLeaveBalance,
) attendanceLeaveApprovalRequestView {
	view := attendanceLeaveApprovalRequestView{
		ID:                 record.ID,
		EmployeeEmail:      record.EmployeeEmail,
		LeaveTypeID:        record.LeaveTypeID,
		LeaveTypeName:      record.LeaveTypeName,
		BalanceMode:        record.BalanceMode,
		Status:             record.Status,
		Unit:               record.Unit,
		StartDate:          record.StartDate,
		PartialPeriod:      record.PartialPeriod,
		DeductionMilliDays: record.TotalDeductionMilliDays,
		Reason:             record.Reason,
		AdminResponse:      record.AdminResponse,
		Attachments:        record.Attachments,
		Balance: attendanceLeaveApprovalBalanceView{
			AvailableMilliDays: balance.AvailableMilliDays,
			ReservedMilliDays:  balance.ReservedMilliDays,
			UsedMilliDays:      balance.UsedMilliDays,
		},
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}
	if record.EndDate != record.StartDate {
		view.EndDate = record.EndDate
	}
	if len(record.Occurrences) > 0 {
		view.StartTime = record.Occurrences[0].StartTime
		view.EndTime = record.Occurrences[len(record.Occurrences)-1].EndTime
	}
	return view
}

func (service *Service) attendanceLeaveApprovalBalance(
	ctx context.Context,
	queryer attendanceLeaveQueryer,
	record attendanceLeaveRequestRecord,
	policy attendanceLeavePolicy,
) (attendanceLeaveBalance, error) {
	leaveType, found := attendanceLeaveTypeByID(policy, record.LeaveTypeID)
	if policy.BalanceTrackingMode != attendanceLeaveBalanceTrackingManaged ||
		!found ||
		leaveType.BalanceMode == "none" {
		return attendanceLeaveBalance{}, nil
	}
	balance, errorValue := queryAttendanceLeaveBalance(ctx, queryer, attendanceLeaveEmployee{
		Email:  record.EmployeeEmail,
		UserID: record.UserID,
	}, attendanceLeaveBalanceAccountID(leaveType.ID, leaveType.BalanceMode))
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	requestDate, errorValue := time.ParseInLocation(
		time.DateOnly,
		record.StartDate,
		service.workspaceTimeZone().location,
	)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	return service.attendanceLeaveBalanceWithUntrackedUsage(
		ctx,
		queryer,
		balance,
		policy,
		requestDate,
	)
}

func scanAttendanceLeaveApprovalRequest(
	scanner interface{ Scan(...any) error },
) (attendanceLeaveRequestRecord, error) {
	var record attendanceLeaveRequestRecord
	if errorValue := scanner.Scan(attendanceLeaveApprovalRequestScanValues(&record)...); errorValue != nil {
		return attendanceLeaveRequestRecord{}, errorValue
	}
	return record, nil
}

func attendanceLeaveApprovalRequestScanValues(
	record *attendanceLeaveRequestRecord,
) []any {
	return []any{
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
	}
}
