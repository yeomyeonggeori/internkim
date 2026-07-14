package admind

import (
	"context"
	"database/sql"
	"time"
)

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
