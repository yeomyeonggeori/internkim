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
	SUM(CASE WHEN status = ? THEN total_deduction_milli_days ELSE 0 END),
	SUM(CASE WHEN status IN (?, ?) THEN total_deduction_milli_days ELSE 0 END)
FROM attendance_leave_requests
WHERE employee_email = ?
	AND status IN (?, ?, ?)
GROUP BY leave_type_id
ORDER BY leave_type_id`,
		attendanceLeaveRequestStatusApproved,
		attendanceLeaveRequestStatusPending,
		attendanceLeaveRequestStatusNeedsChanges,
		normalizeAttendanceLeaveEmail(employeeEmail),
		attendanceLeaveRequestStatusApproved,
		attendanceLeaveRequestStatusPending,
		attendanceLeaveRequestStatusNeedsChanges,
	)
	if errorValue != nil {
		return nil, attendanceLeaveDashboardSummary{}, errorValue
	}
	defer rows.Close()
	byLeaveType := map[string]attendanceLeaveDashboardSummary{}
	total := attendanceLeaveDashboardSummary{}
	for rows.Next() {
		var leaveTypeID string
		var usedMilliDays int
		var reservedMilliDays int
		if errorValue := rows.Scan(
			&leaveTypeID,
			&usedMilliDays,
			&reservedMilliDays,
		); errorValue != nil {
			return nil, attendanceLeaveDashboardSummary{}, errorValue
		}
		summary := attendanceLeaveDashboardSummary{
			UsedMilliDays:     usedMilliDays,
			ReservedMilliDays: reservedMilliDays,
		}
		byLeaveType[leaveTypeID] = summary
		total.UsedMilliDays += usedMilliDays
		total.ReservedMilliDays += reservedMilliDays
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, attendanceLeaveDashboardSummary{}, errorValue
	}
	return byLeaveType, total, nil
}
