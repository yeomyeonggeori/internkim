package admind

import (
	"context"
	"database/sql"
	"time"
)

func readCalendarEventRows(ctx context.Context, database *sql.DB, startTime time.Time, endTime time.Time) ([]calendarEvent, error) {
	query, arguments := calendarEventRangeQuery(startTime, endTime)
	rows, errorValue := database.QueryContext(ctx, query, arguments...)
	if errorValue != nil {
		return nil, errorValue
	}
	return scanCalendarEventRows(rows)
}

func calendarEventRangeQuery(startTime time.Time, endTime time.Time) (string, []any) {
	query := `
SELECT id, uid, title, description, location, start_at, end_at, time_zone, is_all_day, color, raw_ics, reminder_lead_hours, created_by_email, created_by_name, updated_by_email, updated_by_name, updated_by_at, mattermost_post_id, updated_at, remote_source, remote_etag, remote_href
FROM calendar_events
WHERE deleted_at = ''`
	arguments := []any{}
	if !startTime.IsZero() && !endTime.IsZero() {
		query += " AND end_at > ? AND start_at < ?"
		arguments = append(arguments, startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))
	}
	return query + " ORDER BY start_at, title", arguments
}
