package admind

import (
	"context"
	"strings"
)

func (service *Service) readPaidAttendanceLeaveOccurrences(
	ctx context.Context,
	startDate string,
	endDate string,
) ([]attendancePaidLeaveOccurrence, error) {
	policy, errorValue := service.readAttendanceLeavePolicy(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	paidLeaveTypeIDs := make(map[string]struct{})
	for _, leaveType := range policy.LeaveTypes {
		if leaveType.Paid {
			paidLeaveTypeIDs[leaveType.ID] = struct{}{}
		}
	}
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT request.employee_email, request.leave_type_id, occurrence.date,
	occurrence.start_time, occurrence.end_time, occurrence.deduction_milli_days
FROM attendance_leave_request_occurrences occurrence
JOIN attendance_leave_requests request ON request.id = occurrence.request_id
WHERE request.status = ? AND occurrence.date >= ? AND occurrence.date <= ?
ORDER BY occurrence.date, request.employee_email`,
		attendanceLeaveRequestStatusApproved,
		startDate,
		endDate,
	)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	result := []attendancePaidLeaveOccurrence{}
	for rows.Next() {
		var occurrence attendancePaidLeaveOccurrence
		var leaveTypeID string
		if errorValue = rows.Scan(
			&occurrence.Email,
			&leaveTypeID,
			&occurrence.Date,
			&occurrence.StartTime,
			&occurrence.EndTime,
			&occurrence.DeductionMilliDays,
		); errorValue != nil {
			return nil, errorValue
		}
		if _, paid := paidLeaveTypeIDs[strings.TrimSpace(leaveTypeID)]; paid {
			occurrence.Paid = true
		}
		result = append(result, occurrence)
	}
	if errorValue = rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	return result, nil
}
