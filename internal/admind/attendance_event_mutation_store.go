package admind

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (service *Service) withAttendanceEventMutation(
	ctx context.Context,
	database *sql.DB,
	mutate func(*sql.Tx) ([]string, error),
) error {
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return fmt.Errorf("begin attendance event mutation: %w", errorValue)
	}
	defer transaction.Rollback()
	dates, errorValue := mutate(transaction)
	if errorValue != nil {
		return errorValue
	}
	if len(dates) > 0 {
		months, errorValue := attendanceEventCacheMonthsForDates(dates)
		if errorValue != nil {
			return errorValue
		}
		if errorValue := invalidateAttendanceSummaryCacheMonths(ctx, transaction, attendanceSummaryCacheKindEvents, months); errorValue != nil {
			return errorValue
		}
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return fmt.Errorf("commit attendance event mutation: %w", errorValue)
	}
	return nil
}

func (service *Service) insertAttendanceEvent(ctx context.Context, database *sql.DB, event attendanceEvent) error {
	return service.withAttendanceEventMutation(ctx, database, func(transaction *sql.Tx) ([]string, error) {
		_, errorValue := transaction.ExecContext(ctx, `
INSERT INTO attendance_events (
	id, mattermost_user_id, mattermost_username, email, display_name, kind, occurred_at, local_date, local_time,
	time_zone_at_event, source, team_id, channel_id, action_post_id, result_post_id, location_id, location_name,
	canceled_at, cancel_reason, repeated_click_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
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
			"",
			"",
			"",
		)
		if errorValue != nil {
			return nil, fmt.Errorf("insert attendance event: %w", errorValue)
		}
		return []string{event.LocalDate}, nil
	})
}

func (service *Service) markAttendanceRepeatedClick(ctx context.Context, database *sql.DB, eventID string, occurredAt time.Time) error {
	errorValue := service.withAttendanceEventMutation(ctx, database, func(transaction *sql.Tx) ([]string, error) {
		event, found, errorValue := readAttendanceEventByIDInTransaction(ctx, transaction, eventID)
		if errorValue != nil || !found {
			return nil, errorValue
		}
		_, errorValue = transaction.ExecContext(ctx, "UPDATE attendance_events SET repeated_click_at = ? WHERE id = ?", occurredAt.Format(time.RFC3339), eventID)
		if errorValue != nil {
			return nil, fmt.Errorf("mark attendance repeated click: %w", errorValue)
		}
		dates, errorValue := attendanceEventCacheDatesInTransaction(ctx, transaction, event)
		return dates, errorValue
	})
	if errorValue != nil {
		return errorValue
	}
	return errAttendanceDuplicateIgnored
}

func (service *Service) cancelAttendanceEventRecord(
	ctx context.Context,
	database *sql.DB,
	event attendanceEvent,
	canceledAt time.Time,
	reason string,
	repeatedClickAt string,
) error {
	return service.withAttendanceEventMutation(ctx, database, func(transaction *sql.Tx) ([]string, error) {
		currentEvent, found, errorValue := readAttendanceEventByIDInTransaction(ctx, transaction, event.ID)
		if errorValue != nil {
			return nil, errorValue
		}
		if !found {
			return nil, fmt.Errorf("attendance event %q was not found", event.ID)
		}
		_, errorValue = transaction.ExecContext(ctx, `
UPDATE attendance_events
SET canceled_at = ?, cancel_reason = ?, repeated_click_at = ?
	WHERE id = ?`, canceledAt.Format(time.RFC3339), reason, repeatedClickAt, currentEvent.ID)
		if errorValue != nil {
			return nil, fmt.Errorf("cancel attendance event: %w", errorValue)
		}
		dates, errorValue := attendanceEventCacheDatesInTransaction(ctx, transaction, currentEvent)
		return dates, errorValue
	})
}

func (service *Service) updateAttendanceEventTime(ctx context.Context, database *sql.DB, event attendanceEvent, localTime time.Time) error {
	return service.withAttendanceEventMutation(ctx, database, func(transaction *sql.Tx) ([]string, error) {
		currentEvent, found, errorValue := readAttendanceEventByIDInTransaction(ctx, transaction, event.ID)
		if errorValue != nil || !found {
			return nil, errorValue
		}
		_, errorValue = transaction.ExecContext(ctx, `
UPDATE attendance_events
SET occurred_at = ?, local_date = ?, local_time = ?, time_zone_at_event = ?
WHERE id = ?`,
			localTime.UTC().Format(time.RFC3339Nano),
			localTime.Format("2006-01-02"),
			localTime.Format("15:04:05"),
			localTime.Location().String(),
			currentEvent.ID,
		)
		if errorValue != nil {
			return nil, fmt.Errorf("update attendance event time: %w", errorValue)
		}
		dates, errorValue := attendanceEventCacheDatesInTransaction(ctx, transaction, currentEvent)
		dates = append(dates, localTime.Format("2006-01-02"))
		return dates, errorValue
	})
}

func (service *Service) deleteAttendanceEventByResultPostID(ctx context.Context, resultPostID string) error {
	normalizedResultPostID := strings.TrimSpace(resultPostID)
	if normalizedResultPostID == "" {
		return nil
	}
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	return service.withAttendanceEventMutation(ctx, database, func(transaction *sql.Tx) ([]string, error) {
		dates, errorValue := readAttendanceEventCacheDatesByResultPostIDInTransaction(ctx, transaction, normalizedResultPostID)
		if errorValue != nil || len(dates) == 0 {
			return nil, errorValue
		}
		if _, errorValue := transaction.ExecContext(ctx, "DELETE FROM attendance_events WHERE result_post_id = ?", normalizedResultPostID); errorValue != nil {
			return nil, fmt.Errorf("delete attendance event by result post ID: %w", errorValue)
		}
		return dates, nil
	})
}

func readAttendanceEventByIDInTransaction(ctx context.Context, transaction *sql.Tx, eventID string) (attendanceEvent, bool, error) {
	row := transaction.QueryRowContext(ctx, "SELECT "+attendanceEventSelectColumns+" FROM attendance_events WHERE id = ?", strings.TrimSpace(eventID))
	return scanOptionalAttendanceEvent(row)
}

func readAttendanceEventCacheDatesByResultPostIDInTransaction(ctx context.Context, transaction *sql.Tx, resultPostID string) ([]string, error) {
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT events.local_date, overrides.override_local_date
FROM attendance_events AS events
LEFT JOIN attendance_event_overrides AS overrides ON overrides.event_id = events.id
WHERE events.result_post_id = ?`, resultPostID)
	if errorValue != nil {
		return nil, fmt.Errorf("read attendance event cache dates by result post ID: %w", errorValue)
	}
	defer rows.Close()
	datesByValue := map[string]struct{}{}
	for rows.Next() {
		var localDate string
		var overrideLocalDate sql.NullString
		if errorValue := rows.Scan(&localDate, &overrideLocalDate); errorValue != nil {
			return nil, errorValue
		}
		datesByValue[localDate] = struct{}{}
		if overrideLocalDate.Valid {
			datesByValue[overrideLocalDate.String] = struct{}{}
		}
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	dates := make([]string, 0, len(datesByValue))
	for date := range datesByValue {
		dates = append(dates, date)
	}
	return dates, nil
}

func scanOptionalAttendanceEvent(scanner attendanceEventScanner) (attendanceEvent, bool, error) {
	event, errorValue := scanAttendanceEvent(scanner)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return attendanceEvent{}, false, nil
	}
	if errorValue != nil {
		return attendanceEvent{}, false, errorValue
	}
	return event, true, nil
}

func attendanceEventCacheDatesInTransaction(ctx context.Context, transaction *sql.Tx, event attendanceEvent) ([]string, error) {
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT override_local_date
FROM attendance_event_overrides
WHERE event_id = ?`, event.ID)
	if errorValue != nil {
		return nil, fmt.Errorf("read attendance event override cache dates: %w", errorValue)
	}
	defer rows.Close()
	dates := []string{event.LocalDate}
	for rows.Next() {
		var date string
		if errorValue := rows.Scan(&date); errorValue != nil {
			return nil, errorValue
		}
		dates = append(dates, date)
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	return dates, nil
}
