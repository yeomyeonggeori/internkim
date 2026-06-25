package admind

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"
)

const attendanceEventSelectColumns = `
id, mattermost_user_id, mattermost_username, email, display_name, kind, occurred_at, local_date, local_time,
	time_zone_at_event, source, team_id, channel_id, action_post_id, result_post_id, location_id, location_name,
	canceled_at, cancel_reason, repeated_click_at`

func (service *Service) openAttendanceDatabase(ctx context.Context) (*sql.DB, error) {
	return service.openSQLiteDatabase(ctx, service.Configuration.AttendanceDatabasePath, ensureAttendanceSchema)
}

func (service *Service) insertAttendanceEvent(ctx context.Context, database *sql.DB, event attendanceEvent) error {
	_, errorValue := database.ExecContext(ctx, `
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
	return errorValue
}

func (service *Service) latestActiveAttendanceEvent(ctx context.Context, database *sql.DB, mattermostUserID string) (attendanceEvent, bool, error) {
	row := database.QueryRowContext(ctx, "SELECT "+attendanceEventSelectColumns+`
FROM attendance_events
WHERE mattermost_user_id = ? AND canceled_at = ''
ORDER BY occurred_at DESC
LIMIT 1`, mattermostUserID)
	event, errorValue := scanAttendanceEvent(row)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return attendanceEvent{}, false, nil
	}
	if errorValue != nil {
		return attendanceEvent{}, false, errorValue
	}
	return event, true, nil
}

func (service *Service) latestActiveAttendanceEventForLocalDate(ctx context.Context, database *sql.DB, mattermostUserID string, localDate string) (attendanceEvent, bool, error) {
	row := database.QueryRowContext(ctx, "SELECT "+attendanceEventSelectColumns+`
FROM attendance_events
WHERE mattermost_user_id = ? AND local_date = ? AND canceled_at = ''
ORDER BY occurred_at DESC
LIMIT 1`, mattermostUserID, localDate)
	event, errorValue := scanAttendanceEvent(row)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return attendanceEvent{}, false, nil
	}
	if errorValue != nil {
		return attendanceEvent{}, false, errorValue
	}
	return event, true, nil
}

func (service *Service) latestActiveAttendanceEventForLocalDateAtOrBefore(ctx context.Context, database *sql.DB, mattermostUserID string, localDate string, now time.Time) (attendanceEvent, bool, error) {
	row := database.QueryRowContext(ctx, "SELECT "+attendanceEventSelectColumns+`
FROM attendance_events
WHERE mattermost_user_id = ? AND local_date = ? AND occurred_at <= ? AND canceled_at = ''
ORDER BY occurred_at DESC
LIMIT 1`, mattermostUserID, localDate, now.UTC().Format(time.RFC3339Nano))
	event, errorValue := scanAttendanceEvent(row)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return attendanceEvent{}, false, nil
	}
	if errorValue != nil {
		return attendanceEvent{}, false, errorValue
	}
	return event, true, nil
}

func (service *Service) latestActiveAttendanceEventForUserAndKind(ctx context.Context, database *sql.DB, mattermostUserID string, kind string) (attendanceEvent, bool, error) {
	row := database.QueryRowContext(ctx, "SELECT "+attendanceEventSelectColumns+`
FROM attendance_events
WHERE mattermost_user_id = ? AND kind = ? AND canceled_at = ''
ORDER BY occurred_at DESC
LIMIT 1`, mattermostUserID, kind)
	event, errorValue := scanAttendanceEvent(row)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return attendanceEvent{}, false, nil
	}
	if errorValue != nil {
		return attendanceEvent{}, false, errorValue
	}
	return event, true, nil
}

func (service *Service) latestActiveAttendanceEventBefore(ctx context.Context, database *sql.DB, mattermostUserID string, kind string, occurredAt time.Time) (attendanceEvent, bool, error) {
	row := database.QueryRowContext(ctx, "SELECT "+attendanceEventSelectColumns+`
FROM attendance_events
WHERE mattermost_user_id = ? AND kind = ? AND occurred_at < ? AND canceled_at = ''
ORDER BY occurred_at DESC
LIMIT 1`, mattermostUserID, kind, occurredAt.UTC().Format(time.RFC3339Nano))
	event, errorValue := scanAttendanceEvent(row)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return attendanceEvent{}, false, nil
	}
	if errorValue != nil {
		return attendanceEvent{}, false, errorValue
	}
	return event, true, nil
}

func (service *Service) markAttendanceRepeatedClick(ctx context.Context, database *sql.DB, eventID string, occurredAt time.Time) error {
	_, errorValue := database.ExecContext(ctx, "UPDATE attendance_events SET repeated_click_at = ? WHERE id = ?", occurredAt.Format(time.RFC3339), eventID)
	if errorValue != nil {
		return errorValue
	}
	return errAttendanceDuplicateIgnored
}

func (service *Service) readAttendanceEvents(ctx context.Context, month string, email string) ([]attendanceEvent, error) {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	startDate := month + "-01"
	endDate := attendanceNextMonth(month) + "-01"
	query := `SELECT ` + attendanceEventSelectColumns + `
FROM attendance_events
WHERE (local_date >= ? AND local_date < ? OR id IN (
	SELECT event_id
	FROM attendance_event_overrides
	WHERE override_local_date >= ? AND override_local_date < ?
))`
	arguments := []any{startDate, endDate, startDate, endDate}
	if strings.TrimSpace(email) != "" {
		query += " AND email = ?"
		arguments = append(arguments, strings.ToLower(strings.TrimSpace(email)))
	}
	query += " ORDER BY occurred_at DESC"
	rows, errorValue := database.QueryContext(ctx, query, arguments...)
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
	if errorValue := rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	events, errorValue = service.applyAttendanceEventOverrides(ctx, database, events)
	if errorValue != nil {
		return nil, errorValue
	}
	filteredEvents := make([]attendanceEvent, 0, len(events))
	for _, event := range events {
		if event.LocalDate >= startDate && event.LocalDate < endDate {
			filteredEvents = append(filteredEvents, event)
		}
	}
	sort.SliceStable(filteredEvents, func(firstIndex int, secondIndex int) bool {
		return filteredEvents[firstIndex].OccurredAt > filteredEvents[secondIndex].OccurredAt
	})
	return filteredEvents, nil
}

func (service *Service) repairFutureAttendanceEvents(ctx context.Context, now time.Time) error {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, "SELECT "+attendanceEventSelectColumns+`
FROM attendance_events
WHERE occurred_at > ? AND canceled_at = ''
ORDER BY occurred_at ASC`, now.UTC().Format(time.RFC3339Nano))
	if errorValue != nil {
		return errorValue
	}
	defer rows.Close()
	events := []attendanceEvent{}
	for rows.Next() {
		event, errorValue := scanAttendanceEvent(rows)
		if errorValue != nil {
			return errorValue
		}
		events = append(events, event)
	}
	if errorValue := rows.Err(); errorValue != nil {
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

func (service *Service) updateAttendanceEventTime(ctx context.Context, database *sql.DB, event attendanceEvent, localTime time.Time) error {
	_, errorValue := database.ExecContext(ctx, `
UPDATE attendance_events
SET occurred_at = ?, local_date = ?, local_time = ?, time_zone_at_event = ?
WHERE id = ?`,
		localTime.UTC().Format(time.RFC3339Nano),
		localTime.Format("2006-01-02"),
		localTime.Format("15:04:05"),
		localTime.Location().String(),
		event.ID,
	)
	return errorValue
}

func (service *Service) deleteAttendanceEventByResultPostID(ctx context.Context, resultPostID string) error {
	if strings.TrimSpace(resultPostID) == "" {
		return nil
	}
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, "DELETE FROM attendance_events WHERE result_post_id = ?", strings.TrimSpace(resultPostID))
	return errorValue
}

type attendanceEventScanner interface {
	Scan(dest ...any) error
}

func scanAttendanceEvent(scanner attendanceEventScanner) (attendanceEvent, error) {
	var event attendanceEvent
	errorValue := scanner.Scan(
		&event.ID,
		&event.MattermostUserID,
		&event.MattermostUsername,
		&event.Email,
		&event.DisplayName,
		&event.Kind,
		&event.OccurredAt,
		&event.LocalDate,
		&event.LocalTime,
		&event.TimeZoneAtEvent,
		&event.Source,
		&event.TeamID,
		&event.ChannelID,
		&event.ActionPostID,
		&event.ResultPostID,
		&event.LocationID,
		&event.LocationName,
		&event.CanceledAt,
		&event.CancelReason,
		&event.RepeatedClickAt,
	)
	return event, errorValue
}
