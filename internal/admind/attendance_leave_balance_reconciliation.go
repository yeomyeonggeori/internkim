package admind

import (
	"context"
	"time"
)

func (service *Service) attendanceLeaveUntrackedUsageForAccount(
	ctx context.Context,
	queryer attendanceLeaveQueryer,
	employeeEmail string,
	accountID string,
	policy attendanceLeavePolicy,
	onDate time.Time,
	excludedRequestID string,
) (int, error) {
	fiscalYearStart, fiscalYearEnd, errorValue := attendanceLeaveFiscalYearBounds(
		onDate.In(service.workspaceTimeZone().location),
		policy.FiscalYearStartMonth,
		policy.FiscalYearStartDay,
	)
	if errorValue != nil {
		return 0, errorValue
	}
	rows, errorValue := queryer.QueryContext(ctx, `
SELECT request.leave_type_id, COALESCE(SUM(occurrence.deduction_milli_days), 0)
FROM attendance_leave_requests request
JOIN attendance_leave_request_occurrences occurrence ON occurrence.request_id = request.id
WHERE request.employee_email = ?
	AND request.status = ?
	AND request.balance_mode = 'none'
	AND request.id <> ?
	AND occurrence.date >= ?
	AND occurrence.date < ?
GROUP BY request.leave_type_id`,
		normalizeAttendanceLeaveEmail(employeeEmail),
		attendanceLeaveRequestStatusApproved,
		excludedRequestID,
		fiscalYearStart.Format(time.DateOnly),
		fiscalYearEnd.Format(time.DateOnly),
	)
	if errorValue != nil {
		return 0, errorValue
	}
	defer rows.Close()
	leaveTypesByID := make(map[string]attendanceLeaveType, len(policy.LeaveTypes))
	for _, leaveType := range policy.LeaveTypes {
		leaveTypesByID[leaveType.ID] = leaveType
	}
	total := 0
	for rows.Next() {
		var leaveTypeID string
		var usedMilliDays int
		if errorValue := rows.Scan(&leaveTypeID, &usedMilliDays); errorValue != nil {
			return 0, errorValue
		}
		leaveType, found := leaveTypesByID[leaveTypeID]
		if !found || leaveType.BalanceMode == "none" {
			continue
		}
		if attendanceLeaveBalanceAccountID(leaveType.ID, leaveType.BalanceMode) == accountID {
			total += usedMilliDays
		}
	}
	if errorValue := rows.Err(); errorValue != nil {
		return 0, errorValue
	}
	return total, nil
}

func (service *Service) attendanceLeaveBalanceWithUntrackedUsage(
	ctx context.Context,
	queryer attendanceLeaveQueryer,
	balance attendanceLeaveBalance,
	policy attendanceLeavePolicy,
	onDate time.Time,
) (attendanceLeaveBalance, error) {
	usedMilliDays, errorValue := service.attendanceLeaveUntrackedUsageForAccount(
		ctx,
		queryer,
		balance.EmployeeEmail,
		balance.LeaveTypeID,
		policy,
		onDate,
		"",
	)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	balance.AvailableMilliDays = max(balance.AvailableMilliDays-usedMilliDays, 0)
	return balance, nil
}
