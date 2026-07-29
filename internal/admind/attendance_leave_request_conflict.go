package admind

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"time"
)

func attendanceLeaveRequestEnsureNoConflict(
	ctx context.Context,
	transaction *sql.Tx,
	employeeEmail string,
	occurrences []attendanceLeaveRequestOccurrence,
	excludedRequestID string,
	now time.Time,
	location *time.Location,
) error {
	for _, occurrence := range occurrences {
		var requestConflictCount int
		errorValue := transaction.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM attendance_leave_request_occurrences occurrence
JOIN attendance_leave_requests request ON request.id = occurrence.request_id
WHERE request.employee_email = ?
	AND request.status IN (?, ?, ?)
	AND request.id <> ?
	AND occurrence.date = ?
	AND occurrence.start_time < ?
	AND occurrence.end_time > ?`,
			normalizeAttendanceLeaveEmail(employeeEmail),
			attendanceLeaveRequestStatusPending,
			attendanceLeaveRequestStatusNeedsChanges,
			attendanceLeaveRequestStatusApproved,
			excludedRequestID,
			occurrence.Date,
			occurrence.EndTime,
			occurrence.StartTime,
		).Scan(&requestConflictCount)
		if errorValue != nil {
			return errorValue
		}
		if requestConflictCount > 0 {
			return errAttendanceLeaveRequestConflict
		}
		var absenceConflictCount int
		errorValue = transaction.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM attendance_absence_occurrences
WHERE email = ? AND date = ? AND canceled_at = ''`,
			normalizeAttendanceLeaveEmail(employeeEmail),
			occurrence.Date,
		).Scan(&absenceConflictCount)
		if errorValue != nil {
			return errorValue
		}
		if absenceConflictCount > 0 {
			return errAttendanceLeaveRequestConflict
		}
		workConflict, errorValue := attendanceLeaveRequestHasConfirmedWorkConflict(
			ctx,
			transaction,
			employeeEmail,
			occurrence,
			now,
			location,
		)
		if errorValue != nil {
			return errorValue
		}
		if workConflict {
			return errAttendanceLeaveWorkConflict
		}
	}
	return nil
}

func attendanceLeaveRequestHasConfirmedWorkConflict(
	ctx context.Context,
	transaction *sql.Tx,
	employeeEmail string,
	occurrence attendanceLeaveRequestOccurrence,
	now time.Time,
	location *time.Location,
) (bool, error) {
	if location == nil {
		return false, fmt.Errorf("attendance leave conflict time zone is required")
	}
	occurrenceStart, errorValue := time.ParseInLocation(
		"2006-01-02 15:04",
		occurrence.Date+" "+occurrence.StartTime,
		location,
	)
	if errorValue != nil {
		return false, errorValue
	}
	occurrenceEnd, errorValue := time.ParseInLocation(
		"2006-01-02 15:04",
		occurrence.Date+" "+occurrence.EndTime,
		location,
	)
	if errorValue != nil {
		return false, errorValue
	}
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT
	event.id,
	event.kind,
	COALESCE((
		SELECT override.override_occurred_at
		FROM attendance_event_overrides override
		WHERE override.event_id = event.id
		ORDER BY override.edited_at DESC, override.id DESC
		LIMIT 1
	), event.occurred_at)
FROM attendance_events event
WHERE event.email = ?
	AND event.canceled_at = ''`,
		normalizeAttendanceLeaveEmail(employeeEmail),
	)
	if errorValue != nil {
		return false, errorValue
	}
	defer rows.Close()
	type confirmedWorkEvent struct {
		id         string
		kind       string
		occurredAt time.Time
	}
	events := []confirmedWorkEvent{}
	for rows.Next() {
		var event confirmedWorkEvent
		var occurredAtValue string
		if errorValue := rows.Scan(&event.id, &event.kind, &occurredAtValue); errorValue != nil {
			return false, errorValue
		}
		occurredAt, errorValue := time.Parse(time.RFC3339Nano, occurredAtValue)
		if errorValue != nil {
			return false, errorValue
		}
		event.occurredAt = occurredAt
		events = append(events, event)
	}
	if errorValue := rows.Err(); errorValue != nil {
		return false, errorValue
	}
	slices.SortFunc(events, func(first confirmedWorkEvent, second confirmedWorkEvent) int {
		if instantOrder := first.occurredAt.Compare(second.occurredAt); instantOrder != 0 {
			return instantOrder
		}
		return strings.Compare(first.id, second.id)
	})
	var openStart time.Time
	for _, event := range events {
		switch event.kind {
		case attendanceKindClockIn:
			if !openStart.IsZero() &&
				attendanceLeaveRequestInstantRangesOverlap(
					openStart,
					event.occurredAt,
					occurrenceStart,
					occurrenceEnd,
				) {
				return true, nil
			}
			openStart = event.occurredAt
		case attendanceKindClockOut:
			if !openStart.IsZero() &&
				attendanceLeaveRequestInstantRangesOverlap(
					openStart,
					event.occurredAt,
					occurrenceStart,
					occurrenceEnd,
				) {
				return true, nil
			}
			openStart = time.Time{}
		}
	}
	if !openStart.IsZero() &&
		now.After(openStart) &&
		attendanceLeaveRequestInstantRangesOverlap(
			openStart,
			now,
			occurrenceStart,
			occurrenceEnd,
		) {
		return true, nil
	}
	return false, nil
}

func attendanceLeaveRequestInstantRangesOverlap(
	firstStart time.Time,
	firstEnd time.Time,
	secondStart time.Time,
	secondEnd time.Time,
) bool {
	return firstStart.Before(firstEnd) &&
		secondStart.Before(secondEnd) &&
		firstStart.Before(secondEnd) &&
		firstEnd.After(secondStart)
}
