package admind

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func readCalendarEventRows(ctx context.Context, database *sql.DB, startTime time.Time, endTime time.Time) ([]calendarEvent, error) {
	query, arguments := calendarEventRangeQuery(startTime, endTime)
	rows, errorValue := database.QueryContext(ctx, query, arguments...)
	if errorValue != nil {
		return nil, errorValue
	}
	events, errorValue := scanCalendarEventRows(rows)
	if errorValue != nil || startTime.IsZero() || endTime.IsZero() {
		return events, errorValue
	}
	return filterCalendarEventRowsByRange(events, startTime, endTime)
}

func calendarEventRangeQuery(startTime time.Time, endTime time.Time) (string, []any) {
	query := `
SELECT id, uid, title, description, location, start_at, end_at, time_zone, is_all_day, color, raw_ics, reminder_lead_hours, created_by_email, created_by_name, updated_by_email, updated_by_name, updated_by_at, mattermost_post_id, updated_at, remote_source, remote_etag, remote_href
FROM calendar_events
WHERE deleted_at = ''`
	arguments := []any{}
	if !startTime.IsZero() && !endTime.IsZero() {
		query += " AND julianday(end_at) >= julianday(?) AND julianday(start_at) <= julianday(?)"
		arguments = append(arguments, startTime.UTC().Format(time.RFC3339Nano), endTime.UTC().Format(time.RFC3339Nano))
	}
	return query + " ORDER BY start_at, title", arguments
}

func filterCalendarEventRowsByRange(events []calendarEvent, startTime time.Time, endTime time.Time) ([]calendarEvent, error) {
	filteredEvents := make([]calendarEvent, 0, len(events))
	for _, event := range events {
		eventStartTime, errorValue := time.Parse(time.RFC3339Nano, strings.TrimSpace(event.StartISO))
		if errorValue != nil {
			return nil, fmt.Errorf("parse calendar event %s start time: %w", event.ID, errorValue)
		}
		eventEndTime, errorValue := time.Parse(time.RFC3339Nano, strings.TrimSpace(event.EndISO))
		if errorValue != nil {
			return nil, fmt.Errorf("parse calendar event %s end time: %w", event.ID, errorValue)
		}
		if eventEndTime.After(startTime) && eventStartTime.Before(endTime) {
			filteredEvents = append(filteredEvents, event)
		}
	}
	return filteredEvents, nil
}
