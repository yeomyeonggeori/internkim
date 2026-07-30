package admind

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

const attendanceSourceApprovedLeave = "approved_leave"

var errAttendanceLeaveEarlyReturnConfirmationRequired = errors.New(
	"early return confirmation is required",
)
var errAttendanceLeaveClockOutAlreadyApplied = errors.New(
	"work already ended when the approved leave started",
)

type attendanceActiveLeaveView struct {
	RequestID          string `json:"requestID"`
	OccurrenceID       string `json:"occurrenceID"`
	LeaveTypeID        string `json:"leaveTypeID"`
	LeaveTypeName      string `json:"leaveTypeName"`
	StartTime          string `json:"startTime"`
	EndTime            string `json:"endTime"`
	DeductionMilliDays int    `json:"deductionMilliDays"`
	StartAt            string `json:"startAt"`
	EndAt              string `json:"endAt"`
}

func (service *Service) readActiveAttendanceLeave(
	ctx context.Context,
	employeeEmail string,
	now time.Time,
) (*attendanceActiveLeaveView, error) {
	normalizedEmail := normalizeAttendanceLeaveEmail(employeeEmail)
	if normalizedEmail == "" {
		return nil, nil
	}
	location := service.workspaceTimeZone().location
	localNow := now.In(location)
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	var activeLeave attendanceActiveLeaveView
	errorValue = database.QueryRowContext(ctx, `
SELECT
	request.id,
	occurrence.id,
	request.leave_type_id,
	request.leave_type_name,
	occurrence.start_time,
	occurrence.end_time,
	occurrence.deduction_milli_days
FROM attendance_leave_request_occurrences occurrence
JOIN attendance_leave_requests request ON request.id = occurrence.request_id
WHERE request.employee_email = ?
	AND request.status = ?
	AND occurrence.date = ?
	AND occurrence.start_time <= ?
	AND occurrence.end_time > ?
ORDER BY occurrence.start_time
LIMIT 1`,
		normalizedEmail,
		attendanceLeaveRequestStatusApproved,
		localNow.Format(time.DateOnly),
		localNow.Format("15:04"),
		localNow.Format("15:04"),
	).Scan(
		&activeLeave.RequestID,
		&activeLeave.OccurrenceID,
		&activeLeave.LeaveTypeID,
		&activeLeave.LeaveTypeName,
		&activeLeave.StartTime,
		&activeLeave.EndTime,
		&activeLeave.DeductionMilliDays,
	)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return nil, nil
	}
	if errorValue != nil {
		return nil, errorValue
	}
	startAt, errorValue := time.ParseInLocation(
		"2006-01-02 15:04",
		localNow.Format(time.DateOnly)+" "+activeLeave.StartTime,
		location,
	)
	if errorValue != nil {
		return nil, errorValue
	}
	endAt, errorValue := time.ParseInLocation(
		"2006-01-02 15:04",
		localNow.Format(time.DateOnly)+" "+activeLeave.EndTime,
		location,
	)
	if errorValue != nil {
		return nil, errorValue
	}
	activeLeave.StartAt = startAt.Format(time.RFC3339)
	activeLeave.EndAt = endAt.Format(time.RFC3339)
	return &activeLeave, nil
}

func (service *Service) reconcileApprovedLeaveClockOut(
	ctx context.Context,
	employeeEmail string,
	now time.Time,
) (*attendanceActiveLeaveView, error) {
	activeLeave, errorValue := service.readActiveAttendanceLeave(ctx, employeeEmail, now)
	if errorValue != nil || activeLeave == nil {
		return activeLeave, errorValue
	}
	events, errorValue := service.readAttendanceEvents(
		ctx,
		now.In(service.workspaceTimeZone().location).Format("2006-01"),
		employeeEmail,
	)
	if errorValue != nil {
		return nil, errorValue
	}
	var latestEvent attendanceEvent
	foundLatest := false
	for _, event := range events {
		if event.CanceledAt != "" || attendanceEventIsAfter(event, now) {
			continue
		}
		latestEvent = event
		foundLatest = true
		break
	}
	if !foundLatest || latestEvent.Kind != attendanceKindClockIn {
		return activeLeave, nil
	}
	leaveStart, errorValue := time.Parse(time.RFC3339, activeLeave.StartAt)
	if errorValue != nil {
		return nil, errorValue
	}
	clockInAt, errorValue := parseAttendanceEventTime(latestEvent.OccurredAt)
	if errorValue != nil {
		return nil, errorValue
	}
	if !clockInAt.Before(leaveStart) {
		return activeLeave, nil
	}
	clockOut := attendanceEvent{
		ID:                 attendanceLeaveDeterministicID("leave-clock-out", activeLeave.OccurrenceID, 0),
		MattermostUserID:   latestEvent.MattermostUserID,
		MattermostUsername: latestEvent.MattermostUsername,
		Email:              latestEvent.Email,
		DisplayName:        latestEvent.DisplayName,
		Kind:               attendanceKindClockOut,
		OccurredAt:         leaveStart.UTC().Format(time.RFC3339Nano),
		LocalDate:          leaveStart.In(service.workspaceTimeZone().location).Format(time.DateOnly),
		LocalTime:          activeLeave.StartTime + ":00",
		TimeZoneAtEvent:    service.workspaceTimeZone().name,
		Source:             attendanceSourceApprovedLeave,
		TeamID:             latestEvent.TeamID,
		ChannelID:          latestEvent.ChannelID,
		ActionPostID:       latestEvent.ActionPostID,
		ResultPostID:       "",
		LocationID:         latestEvent.LocationID,
		LocationName:       latestEvent.LocationName,
	}
	if errorValue := service.insertApprovedLeaveClockOut(ctx, clockOut); errorValue != nil {
		return nil, errorValue
	}
	return activeLeave, nil
}

func (service *Service) insertApprovedLeaveClockOut(
	ctx context.Context,
	event attendanceEvent,
) error {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	return service.withAttendanceEventMutation(ctx, database, func(transaction *sql.Tx) ([]string, error) {
		result, errorValue := transaction.ExecContext(ctx, `
INSERT OR IGNORE INTO attendance_events (
	id, mattermost_user_id, mattermost_username, email, display_name, kind, occurred_at, local_date, local_time,
	time_zone_at_event, source, team_id, channel_id, action_post_id, result_post_id, location_id, location_name,
	canceled_at, cancel_reason, repeated_click_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', '', '')`,
			event.ID,
			event.MattermostUserID,
			event.MattermostUsername,
			event.Email,
			event.DisplayName,
			event.Kind,
			event.OccurredAt,
			event.LocalDate,
			event.LocalTime,
			event.TimeZoneAtEvent,
			event.Source,
			event.TeamID,
			event.ChannelID,
			event.ActionPostID,
			event.ResultPostID,
			event.LocationID,
			event.LocationName,
		)
		if errorValue != nil {
			return nil, errorValue
		}
		rowsAffected, errorValue := result.RowsAffected()
		if errorValue != nil || rowsAffected == 0 {
			return nil, errorValue
		}
		return []string{event.LocalDate}, nil
	})
}

func (service *Service) recordAttendanceLeaveEarlyReturn(
	ctx context.Context,
	activeLeave attendanceActiveLeaveView,
	employeeEmail string,
	returnedAt time.Time,
) error {
	database, errorValue := service.openAttendanceLeaveMutationDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	defer transaction.Rollback()
	localReturn := returnedAt.In(service.workspaceTimeZone().location)
	returnTime := localReturn.Format("15:04")
	result, errorValue := transaction.ExecContext(ctx, `
UPDATE attendance_leave_request_occurrences
SET end_time = ?
WHERE id = ? AND request_id = ? AND end_time > ?`,
		returnTime,
		activeLeave.OccurrenceID,
		activeLeave.RequestID,
		returnTime,
	)
	if errorValue != nil {
		return errorValue
	}
	rowsAffected, errorValue := result.RowsAffected()
	if errorValue != nil {
		return errorValue
	}
	if rowsAffected == 0 {
		return nil
	}
	returnedAtValue := returnedAt.UTC().Format(time.RFC3339Nano)
	_, errorValue = transaction.ExecContext(ctx, `
INSERT OR IGNORE INTO attendance_leave_request_events (
	id, request_id, kind, actor_email, response, created_at
) VALUES (?, ?, ?, ?, ?, ?)`,
		attendanceLeaveDeterministicID("leave-event-early-return", activeLeave.OccurrenceID, 0),
		activeLeave.RequestID,
		attendanceLeaveApprovalChangeEarlyReturn,
		normalizeAttendanceLeaveEmail(employeeEmail),
		returnedAtValue,
		returnedAtValue,
	)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := invalidateAttendanceAbsenceCacheDates(
		ctx,
		transaction,
		[]string{localReturn.Format(time.DateOnly)},
	); errorValue != nil {
		return errorValue
	}
	return transaction.Commit()
}

func attendanceClockRequestWantsEarlyReturn(body attendanceClockRequest) bool {
	return body.Kind == attendanceKindClockIn && body.ConfirmEarlyReturn
}

func attendanceClockRequestDuringLeaveError(
	body attendanceClockRequest,
) error {
	if body.Kind == attendanceKindClockOut {
		return errAttendanceLeaveClockOutAlreadyApplied
	}
	if body.Kind == attendanceKindClockIn && !attendanceClockRequestWantsEarlyReturn(body) {
		return errAttendanceLeaveEarlyReturnConfirmationRequired
	}
	return nil
}

func (service *Service) prepareAttendanceLeaveClock(
	ctx context.Context,
	employeeEmail string,
	body attendanceClockRequest,
	now time.Time,
) (*attendanceActiveLeaveView, error) {
	activeLeave, errorValue := service.reconcileApprovedLeaveClockOut(ctx, employeeEmail, now)
	if errorValue != nil || activeLeave == nil {
		return activeLeave, errorValue
	}
	if errorValue := attendanceClockRequestDuringLeaveError(body); errorValue != nil {
		return nil, errorValue
	}
	return activeLeave, nil
}

func (service *Service) completeAttendanceLeaveClock(
	ctx context.Context,
	activeLeave *attendanceActiveLeaveView,
	employeeEmail string,
	body attendanceClockRequest,
	now time.Time,
) error {
	if activeLeave == nil || !attendanceClockRequestWantsEarlyReturn(body) {
		return nil
	}
	return service.recordAttendanceLeaveEarlyReturn(ctx, *activeLeave, employeeEmail, now)
}
