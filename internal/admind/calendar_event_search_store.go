package admind

import (
	"context"
	"fmt"
	"strings"
)

const calendarEventSearchResultLimit = 2

func (service *Service) searchCalendarEvents(ctx context.Context, query string) ([]calendarEvent, error) {
	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	if normalizedQuery == "" {
		return nil, fmt.Errorf("calendar event search query is required")
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, uid, title, description, location, start_at, end_at, time_zone, is_all_day, color, raw_ics, reminder_lead_hours, created_by_email, created_by_name, updated_by_email, updated_by_name, updated_by_at, mattermost_post_id, updated_at, remote_source, remote_etag, remote_href
FROM calendar_events
WHERE deleted_at = '' AND (
	instr(lower(title), ?) > 0 OR
	instr(lower(description), ?) > 0 OR
	instr(lower(location), ?) > 0
)
ORDER BY start_at, title
LIMIT ?`, normalizedQuery, normalizedQuery, normalizedQuery, calendarEventSearchResultLimit)
	if errorValue != nil {
		return nil, errorValue
	}
	events, errorValue := scanCalendarEventRows(rows)
	if errorValue != nil {
		return nil, errorValue
	}
	return loadCalendarEventListParticipants(ctx, database, events)
}
