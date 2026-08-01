package admind

import (
	"context"
	"strings"
)

func (service *Service) readApprovedAttendanceLeaveOccurrences(
	ctx context.Context,
	startDate string,
	endDate string,
) ([]attendanceApprovedLeaveOccurrence, error) {
	policy, errorValue := service.readAttendanceLeavePolicy(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	leaveTypePaidByID := make(map[string]bool, len(policy.LeaveTypes))
	for _, leaveType := range policy.LeaveTypes {
		leaveTypePaidByID[leaveType.ID] = leaveType.Paid
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
	result := []attendanceApprovedLeaveOccurrence{}
	for rows.Next() {
		var occurrence attendanceApprovedLeaveOccurrence
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
		occurrence.Paid = leaveTypePaidByID[strings.TrimSpace(leaveTypeID)]
		result = append(result, occurrence)
	}
	if errorValue = rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	return result, nil
}
