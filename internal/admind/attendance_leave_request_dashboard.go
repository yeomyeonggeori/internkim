package admind

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

func (service *Service) readAttendanceLeaveDashboard(
	ctx context.Context,
	employee attendanceLeaveEmployee,
	now time.Time,
) (attendanceLeaveDashboard, error) {
	policy, errorValue := service.readAttendanceLeavePolicy(ctx)
	if errorValue != nil {
		return attendanceLeaveDashboard{}, errorValue
	}
	if errorValue := service.synchronizeAttendanceLeaveAccruals(ctx, employee, policy, now); errorValue != nil {
		return attendanceLeaveDashboard{}, errorValue
	}
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return attendanceLeaveDashboard{}, errorValue
	}
	defer database.Close()
	dashboard := attendanceLeaveDashboard{
		BalanceTrackingMode: policy.BalanceTrackingMode,
		Requests:            []attendanceLeaveRequestView{},
		LedgerEntries:       []attendanceLeaveLedgerView{},
		HireDateRequired: strings.TrimSpace(employee.HireDate) == "" &&
			attendanceLeavePolicyRequiresHireDate(policy),
	}
	balancesByAccount := make(map[string]attendanceLeaveDashboardSummary)
	usageByLeaveType := map[string]attendanceLeaveDashboardSummary{}
	usageByLeaveType, usageSummary, errorValue := readAttendanceLeaveUsageSummaries(
		ctx,
		database,
		employee.Email,
	)
	if errorValue != nil {
		return attendanceLeaveDashboard{}, errorValue
	}
	if policy.BalanceTrackingMode == attendanceLeaveBalanceTrackingUnlimited {
		dashboard.Summary = usageSummary
	} else {
		dashboard.Summary.UsedMilliDays = usageSummary.UsedMilliDays
		for _, leaveType := range policy.LeaveTypes {
			if !leaveType.IsActive || !attendanceLeaveTypeOwnsBalance(leaveType) {
				continue
			}
			balance, balanceError := queryAttendanceLeaveBalance(ctx, database, employee, leaveType.ID)
			if balanceError != nil {
				return attendanceLeaveDashboard{}, balanceError
			}
			if employee.HireDate != "" {
				hireDate, parseError := time.ParseInLocation(
					time.DateOnly,
					employee.HireDate,
					service.workspaceTimeZone().location,
				)
				if parseError != nil {
					return attendanceLeaveDashboard{}, parseError
				}
				balance = attendanceLeaveBalanceWithSchedule(
					balance,
					hireDate,
					now,
					policy,
					leaveType,
				)
			}
			balanceView := attendanceLeaveDashboardSummary{
				UsedMilliDays:      balance.UsedMilliDays,
				ReservedMilliDays:  balance.ReservedMilliDays,
				AvailableMilliDays: balance.AvailableMilliDays,
			}
			balancesByAccount[attendanceLeaveBalanceAccountID(leaveType.ID, leaveType.BalanceMode)] = balanceView
			if leaveType.IncludeInSummary {
				dashboard.Summary.AvailableMilliDays += balanceView.AvailableMilliDays
				dashboard.Summary.ReservedMilliDays += balanceView.ReservedMilliDays
			}
		}
	}
	dashboard.LeaveTypes = attendanceLeaveTypeViews(
		policy,
		balancesByAccount,
		usageByLeaveType,
	)
	records, errorValue := readAttendanceLeaveRequestRecords(ctx, database, employee.Email)
	if errorValue != nil {
		return attendanceLeaveDashboard{}, errorValue
	}
	for _, record := range records {
		dashboard.Requests = append(
			dashboard.Requests,
			projectAttendanceLeaveRequest(record, now, service.workspaceTimeZone().location),
		)
	}
	dashboard.LedgerEntries, errorValue = readAttendanceLeaveLedgerViews(ctx, database, employee.Email, policy)
	if errorValue != nil {
		return attendanceLeaveDashboard{}, errorValue
	}
	return dashboard, nil
}

func attendanceLeaveTypeViews(
	policy attendanceLeavePolicy,
	balancesByAccount map[string]attendanceLeaveDashboardSummary,
	usageByLeaveType map[string]attendanceLeaveDashboardSummary,
) []attendanceLeaveTypeView {
	views := make([]attendanceLeaveTypeView, 0, len(policy.LeaveTypes))
	for _, leaveType := range policy.LeaveTypes {
		view := attendanceLeaveTypeView{
			ID:               leaveType.ID,
			Name:             leaveType.Name,
			BalanceMode:      leaveType.BalanceMode,
			AllowedUnits:     append([]string{}, leaveType.AllowedUnits...),
			IncludeInSummary: leaveType.IncludeInSummary,
			IsActive:         leaveType.IsActive,
			RequiresHireDate: policy.BalanceTrackingMode == attendanceLeaveBalanceTrackingManaged &&
				attendanceLeaveTypeRequiresHireDate(leaveType),
		}
		if policy.BalanceTrackingMode == attendanceLeaveBalanceTrackingUnlimited {
			balance := usageByLeaveType[leaveType.ID]
			view.Balance = &balance
		} else if leaveType.BalanceMode != "none" {
			accountID := attendanceLeaveBalanceAccountID(leaveType.ID, leaveType.BalanceMode)
			if balance, found := balancesByAccount[accountID]; found {
				balanceCopy := balance
				view.Balance = &balanceCopy
			}
		}
		views = append(views, view)
	}
	return views
}

func projectAttendanceLeaveRequest(
	record attendanceLeaveRequestRecord,
	now time.Time,
	location *time.Location,
) attendanceLeaveRequestView {
	view := attendanceLeaveRequestView{
		ID:                 record.ID,
		LeaveTypeID:        record.LeaveTypeID,
		LeaveTypeName:      record.LeaveTypeName,
		Status:             record.Status,
		Unit:               record.Unit,
		StartDate:          record.StartDate,
		PartialPeriod:      record.PartialPeriod,
		DeductionMilliDays: record.TotalDeductionMilliDays,
		Reason:             record.Reason,
		AdminResponse:      record.AdminResponse,
		Attachments:        record.Attachments,
		CanEdit:            record.Status == attendanceLeaveRequestStatusPending,
		CanResubmit:        record.Status == attendanceLeaveRequestStatusNeedsChanges,
		Revision:           record.Revision,
		CreatedAt:          record.CreatedAt,
		UpdatedAt:          record.UpdatedAt,
	}
	if record.EndDate != record.StartDate {
		view.EndDate = record.EndDate
	}
	if len(record.Occurrences) > 0 {
		view.StartTime = record.Occurrences[0].StartTime
		view.EndTime = record.Occurrences[len(record.Occurrences)-1].EndTime
	}
	switch record.Status {
	case attendanceLeaveRequestStatusPending, attendanceLeaveRequestStatusNeedsChanges:
		view.CanCancel = true
	case attendanceLeaveRequestStatusApproved:
		view.CanCancel = attendanceLeaveRequestStartsAfter(record, now, location)
	}
	return view
}

func attendanceLeaveRequestStartsAfter(
	record attendanceLeaveRequestRecord,
	now time.Time,
	location *time.Location,
) bool {
	if len(record.Occurrences) == 0 || location == nil {
		return false
	}
	start, errorValue := time.ParseInLocation(
		"2006-01-02 15:04",
		record.Occurrences[0].Date+" "+record.Occurrences[0].StartTime,
		location,
	)
	return errorValue == nil && start.After(now)
}

func readAttendanceLeaveLedgerViews(
	ctx context.Context,
	database *sql.DB,
	employeeEmail string,
	policy attendanceLeavePolicy,
) ([]attendanceLeaveLedgerView, error) {
	rows, errorValue := database.QueryContext(ctx, `
SELECT
	MIN(entry.id),
	operation.operation_key,
	operation.kind,
	operation.created_at,
	operation.leave_type_id,
	CASE operation.kind
		WHEN 'use' THEN -SUM(entry.used_delta_milli_days)
		WHEN 'untrackedUse' THEN -SUM(entry.used_delta_milli_days)
		ELSE SUM(entry.available_delta_milli_days)
	END,
	(
		SELECT final_entry.available_after_milli_days
		FROM attendance_leave_ledger_entries final_entry
		WHERE final_entry.operation_key = operation.operation_key
		ORDER BY final_entry.sequence DESC
		LIMIT 1
	),
	CASE WHEN operation.kind = 'untrackedUse' THEN 1 ELSE 0 END,
	COALESCE(request.id, '')
FROM attendance_leave_operations operation
JOIN attendance_leave_ledger_entries entry ON entry.operation_key = operation.operation_key
	LEFT JOIN attendance_leave_requests request
	ON (
		operation.reference_id = request.id
		OR instr(operation.reference_id, request.id || ':revision:') = 1
	)
	AND request.employee_email = operation.employee_email
WHERE operation.employee_email = ?
GROUP BY
	operation.operation_key,
	operation.kind,
	operation.created_at,
	operation.leave_type_id,
	request.id
ORDER BY operation.created_at DESC, operation.operation_key DESC`,
		normalizeAttendanceLeaveEmail(employeeEmail),
	)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	leaveTypeNames := map[string]string{}
	for _, leaveType := range policy.LeaveTypes {
		leaveTypeNames[leaveType.ID] = leaveType.Name
	}
	entries := []attendanceLeaveLedgerView{}
	for rows.Next() {
		var entry attendanceLeaveLedgerView
		var isUntracked int
		if errorValue := rows.Scan(
			&entry.ID,
			&entry.OperationKey,
			&entry.OperationType,
			&entry.OccurredAt,
			&entry.LeaveTypeID,
			&entry.DeltaMilliDays,
			&entry.BalanceAfterMilliDays,
			&isUntracked,
			&entry.RequestID,
		); errorValue != nil {
			return nil, errorValue
		}
		entry.IsUntracked = isUntracked != 0
		entry.LeaveTypeName = strings.TrimSpace(leaveTypeNames[entry.LeaveTypeID])
		if entry.LeaveTypeName == "" {
			entry.LeaveTypeName = entry.LeaveTypeID
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}
