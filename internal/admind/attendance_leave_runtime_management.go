package admind

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const attendanceLeaveClockOutCancelReason = "approved_leave_cancelled"

func (service *Service) updateApprovedLeaveClockOutInTransaction(
	ctx context.Context,
	transaction *sql.Tx,
	requestID string,
	occurrence attendanceLeaveRequestOccurrence,
) error {
	occurrenceIDs, errorValue := readAttendanceLeaveOccurrenceIDsInTransaction(
		ctx,
		transaction,
		requestID,
	)
	if errorValue != nil || len(occurrenceIDs) != 1 {
		if errorValue == nil {
			return fmt.Errorf(
				"leave request %q has %d occurrences for time correction",
				requestID,
				len(occurrenceIDs),
			)
		}
		return errorValue
	}
	event, found, errorValue := readApprovedLeaveClockOutInTransaction(
		ctx,
		transaction,
		occurrenceIDs[0],
	)
	if errorValue != nil || !found {
		return errorValue
	}
	localTime, errorValue := time.ParseInLocation(
		"2006-01-02 15:04",
		occurrence.Date+" "+occurrence.StartTime,
		service.workspaceTimeZone().location,
	)
	if errorValue != nil {
		return errorValue
	}
	cacheDates, errorValue := attendanceEventCacheDatesInTransaction(ctx, transaction, event)
	if errorValue != nil {
		return errorValue
	}
	if _, errorValue := transaction.ExecContext(ctx, `
UPDATE attendance_events
SET occurred_at = ?, local_date = ?, local_time = ?, time_zone_at_event = ?
WHERE id = ? AND source = ? AND canceled_at = ''`,
		localTime.UTC().Format(time.RFC3339Nano),
		localTime.Format(time.DateOnly),
		localTime.Format("15:04:05"),
		service.workspaceTimeZone().name,
		event.ID,
		attendanceSourceApprovedLeave,
	); errorValue != nil {
		return errorValue
	}
	cacheDates = append(cacheDates, localTime.Format(time.DateOnly))
	return invalidateAttendanceEventCacheDates(ctx, transaction, cacheDates)
}

func cancelApprovedLeaveClockOutsInTransaction(
	ctx context.Context,
	transaction *sql.Tx,
	requestID string,
	now time.Time,
) error {
	occurrenceIDs, errorValue := readAttendanceLeaveOccurrenceIDsInTransaction(
		ctx,
		transaction,
		requestID,
	)
	if errorValue != nil {
		return errorValue
	}
	cacheDates := []string{}
	for _, occurrenceID := range occurrenceIDs {
		event, found, errorValue := readApprovedLeaveClockOutInTransaction(
			ctx,
			transaction,
			occurrenceID,
		)
		if errorValue != nil {
			return errorValue
		}
		if !found || event.CanceledAt != "" {
			continue
		}
		eventDates, errorValue := attendanceEventCacheDatesInTransaction(ctx, transaction, event)
		if errorValue != nil {
			return errorValue
		}
		cacheDates = append(cacheDates, eventDates...)
		if _, errorValue := transaction.ExecContext(ctx, `
UPDATE attendance_events
SET canceled_at = ?, cancel_reason = ?
WHERE id = ? AND source = ? AND canceled_at = ''`,
			now.UTC().Format(time.RFC3339Nano),
			attendanceLeaveClockOutCancelReason,
			event.ID,
			attendanceSourceApprovedLeave,
		); errorValue != nil {
			return errorValue
		}
	}
	return invalidateAttendanceEventCacheDates(ctx, transaction, cacheDates)
}

func readAttendanceLeaveOccurrenceIDsInTransaction(
	ctx context.Context,
	transaction *sql.Tx,
	requestID string,
) ([]string, error) {
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT id
FROM attendance_leave_request_occurrences
WHERE request_id = ?
ORDER BY date, start_time, id`,
		requestID,
	)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	occurrenceIDs := []string{}
	for rows.Next() {
		var occurrenceID string
		if errorValue := rows.Scan(&occurrenceID); errorValue != nil {
			return nil, errorValue
		}
		occurrenceIDs = append(occurrenceIDs, occurrenceID)
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	return occurrenceIDs, nil
}

func readApprovedLeaveClockOutInTransaction(
	ctx context.Context,
	transaction *sql.Tx,
	occurrenceID string,
) (attendanceEvent, bool, error) {
	eventID := attendanceLeaveDeterministicID("leave-clock-out", occurrenceID, 0)
	event, found, errorValue := readAttendanceEventByIDInTransaction(ctx, transaction, eventID)
	if errorValue != nil || !found {
		return event, found, errorValue
	}
	if event.Source != attendanceSourceApprovedLeave ||
		event.Kind != attendanceKindClockOut {
		return attendanceEvent{}, false, fmt.Errorf(
			"attendance event %q is not an approved leave clock out",
			event.ID,
		)
	}
	return event, true, nil
}

func invalidateAttendanceEventCacheDates(
	ctx context.Context,
	transaction *sql.Tx,
	dates []string,
) error {
	if len(dates) == 0 {
		return nil
	}
	months, errorValue := attendanceEventCacheMonthsForDates(dates)
	if errorValue != nil {
		return errorValue
	}
	return invalidateAttendanceSummaryCacheMonths(
		ctx,
		transaction,
		attendanceSummaryCacheKindEvents,
		months,
	)
}
