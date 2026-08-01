package admind

import (
	"context"
	"database/sql"
)

func readAttendanceLeaveUsageSummaries(
	ctx context.Context,
	database *sql.DB,
	employeeEmail string,
) (map[string]attendanceLeaveDashboardSummary, attendanceLeaveDashboardSummary, error) {
	rows, errorValue := database.QueryContext(ctx, `
SELECT
	leave_type_id,
	SUM(CASE WHEN status IN (?, ?) THEN total_deduction_milli_days ELSE 0 END)
FROM attendance_leave_requests
WHERE employee_email = ?
	AND status IN (?, ?)
GROUP BY leave_type_id
ORDER BY leave_type_id`,
		attendanceLeaveRequestStatusPending,
		attendanceLeaveRequestStatusNeedsChanges,
		normalizeAttendanceLeaveEmail(employeeEmail),
		attendanceLeaveRequestStatusPending,
		attendanceLeaveRequestStatusNeedsChanges,
	)
	if errorValue != nil {
		return nil, attendanceLeaveDashboardSummary{}, errorValue
	}
	byLeaveType := map[string]attendanceLeaveDashboardSummary{}
	total := attendanceLeaveDashboardSummary{}
	for rows.Next() {
		var leaveTypeID string
		var reservedMilliDays int
		if errorValue := rows.Scan(
			&leaveTypeID,
			&reservedMilliDays,
		); errorValue != nil {
			rows.Close()
			return nil, attendanceLeaveDashboardSummary{}, errorValue
		}
		summary := attendanceLeaveDashboardSummary{
			ReservedMilliDays: reservedMilliDays,
		}
		byLeaveType[leaveTypeID] = summary
		total.ReservedMilliDays += reservedMilliDays
	}
	if errorValue := rows.Err(); errorValue != nil {
		rows.Close()
		return nil, attendanceLeaveDashboardSummary{}, errorValue
	}
	if errorValue := rows.Close(); errorValue != nil {
		return nil, attendanceLeaveDashboardSummary{}, errorValue
	}
	rows, errorValue = database.QueryContext(ctx, `
SELECT
	COALESCE(request.leave_type_id, operation.leave_type_id),
	COALESCE(SUM(entry.used_delta_milli_days), 0)
FROM attendance_leave_operations operation
JOIN attendance_leave_ledger_entries entry
	ON entry.operation_key = operation.operation_key
LEFT JOIN attendance_leave_requests request
	ON (
		operation.reference_id = request.id
		OR instr(operation.reference_id, request.id || ':revision:') = 1
	)
	AND request.employee_email = operation.employee_email
WHERE operation.employee_email = ?
	AND (request.id IS NULL OR request.status = ?)
GROUP BY COALESCE(request.leave_type_id, operation.leave_type_id)
ORDER BY COALESCE(request.leave_type_id, operation.leave_type_id)`,
		normalizeAttendanceLeaveEmail(employeeEmail),
		attendanceLeaveRequestStatusApproved,
	)
	if errorValue != nil {
		return nil, attendanceLeaveDashboardSummary{}, errorValue
	}
	defer rows.Close()
	for rows.Next() {
		var leaveTypeID string
		var usedMilliDays int
		if errorValue := rows.Scan(&leaveTypeID, &usedMilliDays); errorValue != nil {
			return nil, attendanceLeaveDashboardSummary{}, errorValue
		}
		summary := byLeaveType[leaveTypeID]
		summary.UsedMilliDays = usedMilliDays
		byLeaveType[leaveTypeID] = summary
		total.UsedMilliDays += usedMilliDays
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, attendanceLeaveDashboardSummary{}, errorValue
	}
	return byLeaveType, total, nil
}
