package admind

import (
	"context"
	"database/sql"
	"fmt"
)

func readAttendanceLeaveRequestRecords(
	ctx context.Context,
	database *sql.DB,
	employeeEmail string,
) ([]attendanceLeaveRequestRecord, error) {
	rows, errorValue := database.QueryContext(ctx, `
SELECT
	id, employee_email, user_id, leave_type_id, leave_type_name, balance_mode,
	status, unit, start_date, end_date, partial_period, start_time, reason,
	admin_response, total_deduction_milli_days, revision, created_at, updated_at, cancelled_at
FROM attendance_leave_requests
WHERE employee_email = ?
ORDER BY created_at DESC, id DESC`,
		normalizeAttendanceLeaveEmail(employeeEmail),
	)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	records := []attendanceLeaveRequestRecord{}
	for rows.Next() {
		var record attendanceLeaveRequestRecord
		if errorValue := rows.Scan(
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
		); errorValue != nil {
			return nil, errorValue
		}
		records = append(records, record)
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	for index := range records {
		occurrences, occurrenceError := readAttendanceLeaveRequestOccurrences(ctx, database, records[index].ID)
		if occurrenceError != nil {
			return nil, occurrenceError
		}
		records[index].Occurrences = occurrences
		attachments, attachmentError := readAttendanceLeaveRequestAttachments(ctx, database, records[index].ID)
		if attachmentError != nil {
			return nil, attachmentError
		}
		records[index].Attachments = attachments
	}
	return records, nil
}

func readAttendanceLeaveRequestOccurrences(
	ctx context.Context,
	database *sql.DB,
	requestID string,
) ([]attendanceLeaveRequestOccurrence, error) {
	rows, errorValue := database.QueryContext(ctx, `
SELECT date, start_time, end_time, deduction_milli_days
FROM attendance_leave_request_occurrences
WHERE request_id = ?
ORDER BY date, start_time, id`,
		requestID,
	)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	occurrences := []attendanceLeaveRequestOccurrence{}
	for rows.Next() {
		var occurrence attendanceLeaveRequestOccurrence
		if errorValue := rows.Scan(
			&occurrence.Date,
			&occurrence.StartTime,
			&occurrence.EndTime,
			&occurrence.DeductionMilliDays,
		); errorValue != nil {
			return nil, errorValue
		}
		occurrences = append(occurrences, occurrence)
	}
	return occurrences, rows.Err()
}

func readAttendanceLeaveRequestAttachments(
	ctx context.Context,
	database *sql.DB,
	requestID string,
) ([]attendanceLeaveRequestAttachment, error) {
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, file_name, content_type, size_bytes, storage_key
FROM attendance_leave_request_attachments
WHERE request_id = ?
ORDER BY created_at, id`,
		requestID,
	)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	attachments := []attendanceLeaveRequestAttachment{}
	for rows.Next() {
		var attachment attendanceLeaveRequestAttachment
		if errorValue := rows.Scan(
			&attachment.ID,
			&attachment.FileName,
			&attachment.ContentType,
			&attachment.SizeBytes,
			&attachment.StorageKey,
		); errorValue != nil {
			return nil, errorValue
		}
		attachment.DownloadURL = fmt.Sprintf(
			"/attendance/api/leave-requests/%s/attachments/%s",
			requestID,
			attachment.ID,
		)
		attachments = append(attachments, attachment)
	}
	return attachments, rows.Err()
}
