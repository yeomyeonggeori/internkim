package admind

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (service *Service) repairAttendanceClockOutDates(ctx context.Context) (int, error) {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return 0, errorValue
	}
	defer database.Close()
	events, errorValue := readLegacyAttendanceClockOutEvents(ctx, database)
	if errorValue != nil {
		return 0, errorValue
	}
	repairedCount := 0
	for _, event := range events {
		actualDate, errorValue := service.attendanceEventActualDate(event)
		if errorValue != nil {
			return repairedCount, errorValue
		}
		if actualDate == event.LocalDate {
			continue
		}
		if errorValue := updateAttendanceEventLocalDate(ctx, database, event, actualDate); errorValue != nil {
			return repairedCount, errorValue
		}
		repairedCount++
	}
	return repairedCount, nil
}

func readLegacyAttendanceClockOutEvents(ctx context.Context, database *sql.DB) ([]attendanceEvent, error) {
	rows, errorValue := database.QueryContext(ctx, "SELECT "+attendanceEventSelectColumns+`
FROM attendance_events AS events
WHERE kind = ? AND canceled_at = '' AND NOT EXISTS (
	SELECT 1 FROM attendance_event_overrides AS overrides WHERE overrides.event_id = events.id
)
ORDER BY occurred_at ASC`, attendanceKindClockOut)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	events := []attendanceEvent{}
	for rows.Next() {
		event, errorValue := scanAttendanceEvent(rows)
		if errorValue != nil {
			return nil, errorValue
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (service *Service) attendanceEventActualDate(event attendanceEvent) (string, error) {
	occurredAt, errorValue := parseAttendanceEventTime(event.OccurredAt)
	if errorValue != nil {
		return "", fmt.Errorf("parse attendance event %q occurred_at: %w", event.ID, errorValue)
	}
	timeZoneName := strings.TrimSpace(event.TimeZoneAtEvent)
	location, errorValue := time.LoadLocation(timeZoneName)
	if timeZoneName == "" || timeZoneName == "Local" || errorValue != nil {
		location, _ = service.workspaceTimeLocation()
	}
	return occurredAt.In(location).Format("2006-01-02"), nil
}

func updateAttendanceEventLocalDate(ctx context.Context, database *sql.DB, event attendanceEvent, localDate string) error {
	return withAttendanceEventMutationOnDatabase(ctx, database, func(transaction *sql.Tx) ([]string, error) {
		if _, errorValue := transaction.ExecContext(ctx, "UPDATE attendance_events SET local_date = ? WHERE id = ?", localDate, event.ID); errorValue != nil {
			return nil, fmt.Errorf("update attendance event %q local date: %w", event.ID, errorValue)
		}
		return []string{event.LocalDate, localDate}, nil
	})
}

func (service *Service) repairFutureAttendanceEvents(ctx context.Context, now time.Time) error {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	events, errorValue := readFutureAttendanceEvents(ctx, database, now)
	if errorValue != nil {
		return errorValue
	}
	for _, event := range events {
		localTime, errorValue := service.repairedFutureAttendanceLocalTime(event, now)
		if errorValue != nil {
			return errorValue
		}
		if errorValue := service.updateAttendanceEventTime(ctx, database, event, localTime); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func readFutureAttendanceEvents(ctx context.Context, database *sql.DB, now time.Time) ([]attendanceEvent, error) {
	rows, errorValue := database.QueryContext(ctx, "SELECT "+attendanceEventSelectColumns+`
FROM attendance_events
WHERE occurred_at > ? AND canceled_at = ''
ORDER BY occurred_at ASC`, now.UTC().Format(time.RFC3339Nano))
	if errorValue != nil {
		return nil, errorValue
	}
	events := []attendanceEvent{}
	for rows.Next() {
		event, errorValue := scanAttendanceEvent(rows)
		if errorValue != nil {
			rows.Close()
			return nil, errorValue
		}
		events = append(events, event)
	}
	if errorValue := rows.Err(); errorValue != nil {
		rows.Close()
		return nil, errorValue
	}
	if errorValue := rows.Close(); errorValue != nil {
		return nil, errorValue
	}
	return events, nil
}

func (service *Service) repairedFutureAttendanceLocalTime(event attendanceEvent, now time.Time) (time.Time, error) {
	occurredAt, errorValue := parseAttendanceEventTime(event.OccurredAt)
	if errorValue != nil {
		return time.Time{}, errorValue
	}
	location, _ := service.workspaceTimeLocation()
	localTime := occurredAt.In(location)
	for localTime.UTC().After(now) {
		localTime = localTime.AddDate(0, 0, -1)
	}
	return localTime, nil
}
