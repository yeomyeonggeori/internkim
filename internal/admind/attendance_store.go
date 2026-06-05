package admind

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

const attendanceEventSelectColumns = `
id, mattermost_user_id, mattermost_username, email, display_name, kind, occurred_at, local_date, local_time,
	time_zone_at_event, source, team_id, channel_id, action_post_id, result_post_id, location_id, location_name,
	canceled_at, cancel_reason, repeated_click_at, source_message, original_location_id, original_location_name,
	overridden_by, overridden_at`

var errAttendanceEventLocationConflict = errors.New("attendance event can no longer change location")

func (service *Service) openAttendanceDatabase(ctx context.Context) (*sql.DB, error) {
	return service.openSQLiteDatabase(ctx, service.Configuration.AttendanceDatabasePath, ensureAttendanceSchema)
}

func (service *Service) insertAttendanceEvent(ctx context.Context, database *sql.DB, event attendanceEvent) error {
	_, errorValue := database.ExecContext(ctx, `
INSERT INTO attendance_events (
	id, mattermost_user_id, mattermost_username, email, display_name, kind, occurred_at, local_date, local_time,
	time_zone_at_event, source, team_id, channel_id, action_post_id, result_post_id, location_id, location_name,
	canceled_at, cancel_reason, repeated_click_at, source_message, original_location_id, original_location_name,
	overridden_by, overridden_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
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
		event.SourceMessage,
		event.OriginalLocationID,
		event.OriginalLocationName,
		event.OverriddenBy,
		event.OverriddenAt,
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

func (service *Service) attendanceEventByID(ctx context.Context, database *sql.DB, eventID string) (attendanceEvent, bool, error) {
	row := database.QueryRowContext(ctx, "SELECT "+attendanceEventSelectColumns+`
FROM attendance_events
WHERE id = ?`, strings.TrimSpace(eventID))
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
WHERE local_date >= ? AND local_date < ?`
	arguments := []any{startDate, endDate}
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
	return events, rows.Err()
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

func (service *Service) updateAttendanceEventLocation(ctx context.Context, database *sql.DB, event attendanceEvent, location attendanceLocation, actorEmail string, overriddenAt time.Time) (attendanceEvent, error) {
	originalLocationID := strings.TrimSpace(event.OriginalLocationID)
	originalLocationName := strings.TrimSpace(event.OriginalLocationName)
	if originalLocationID == "" {
		originalLocationID = strings.TrimSpace(event.LocationID)
		originalLocationName = strings.TrimSpace(event.LocationName)
	}
	overriddenAtText := overriddenAt.UTC().Format(time.RFC3339)
	result, errorValue := database.ExecContext(ctx, `
UPDATE attendance_events
SET location_id = ?, location_name = ?, original_location_id = ?, original_location_name = ?, overridden_by = ?, overridden_at = ?
WHERE id = ? AND kind = ? AND canceled_at = ''`,
		strings.TrimSpace(location.ID),
		strings.TrimSpace(location.Name),
		originalLocationID,
		originalLocationName,
		strings.ToLower(strings.TrimSpace(actorEmail)),
		overriddenAtText,
		event.ID,
		attendanceKindClockIn,
	)
	if errorValue != nil {
		return attendanceEvent{}, errorValue
	}
	affectedRows, errorValue := result.RowsAffected()
	if errorValue != nil {
		return attendanceEvent{}, errorValue
	}
	if affectedRows == 0 {
		return attendanceEvent{}, errAttendanceEventLocationConflict
	}
	event.LocationID = strings.TrimSpace(location.ID)
	event.LocationName = strings.TrimSpace(location.Name)
	event.OriginalLocationID = originalLocationID
	event.OriginalLocationName = originalLocationName
	event.OverriddenBy = strings.ToLower(strings.TrimSpace(actorEmail))
	event.OverriddenAt = overriddenAtText
	return event.withDerivedAttendanceFields(), nil
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
		&event.SourceMessage,
		&event.OriginalLocationID,
		&event.OriginalLocationName,
		&event.OverriddenBy,
		&event.OverriddenAt,
	)
	return event.withDerivedAttendanceFields(), errorValue
}

func (event attendanceEvent) withDerivedAttendanceFields() attendanceEvent {
	if strings.TrimSpace(event.OriginalLocationID) == "" {
		return event
	}
	event.ParsedAs = &attendanceParsedClassification{
		Kind:       event.Kind,
		LocationID: event.OriginalLocationID,
	}
	return event
}
