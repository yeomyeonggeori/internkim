package admind

import (
	"context"
	"database/sql"
	"strings"
)

// The device's own calendar, kept only so what it still holds can be counted and
// carried before it is let go.
func (service *Service) localCalendarEvents(ctx context.Context) ([]calendarEvent, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	if !calendarTableExists(ctx, database, "calendar_events") {
		return nil, nil
	}
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, title, start_at, end_at, is_all_day, created_by_email
FROM calendar_events
WHERE deleted_at = ''
ORDER BY start_at`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	events := []calendarEvent{}
	for rows.Next() {
		var event calendarEvent
		var isAllDay int
		if errorValue := rows.Scan(&event.ID, &event.Title, &event.StartISO, &event.EndISO, &isAllDay, &event.CreatedByEmail); errorValue != nil {
			return nil, errorValue
		}
		event.IsAllDay = isAllDay != 0
		events = append(events, event)
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	return service.localCalendarEventsWithParticipants(ctx, database, events)
}

func (service *Service) localCalendarEventsWithParticipants(ctx context.Context, database *sql.DB, events []calendarEvent) ([]calendarEvent, error) {
	if len(events) == 0 || !calendarTableExists(ctx, database, "calendar_event_participants") {
		return events, nil
	}
	rows, errorValue := database.QueryContext(ctx, `
SELECT event_id, person_id, name, email
FROM calendar_event_participants
ORDER BY event_id, sort_order, name`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	participantsByEvent := map[string][]calendarParticipant{}
	for rows.Next() {
		var eventID string
		var participant calendarParticipant
		if errorValue := rows.Scan(&eventID, &participant.PersonID, &participant.Name, &participant.Email); errorValue != nil {
			return nil, errorValue
		}
		participantsByEvent[eventID] = append(participantsByEvent[eventID], participant)
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	for index := range events {
		events[index].Participants = participantsByEvent[events[index].ID]
	}
	return events, nil
}

func calendarTableExists(ctx context.Context, database *sql.DB, tableName string) bool {
	var found int
	errorValue := database.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", strings.TrimSpace(tableName)).Scan(&found)
	return errorValue == nil && found > 0
}

const calendarCarryLinkTable = "calendar_carried_events"

func ensureCalendarCarryLinkTable(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS calendar_carried_events (
	local_event_id TEXT PRIMARY KEY,
	record_event_id TEXT NOT NULL
)`)
	return errorValue
}

func (service *Service) rememberEventCarriedIntoTheRecord(ctx context.Context, localEventID string, recordEventID string) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	if errorValue := ensureCalendarCarryLinkTable(ctx, database); errorValue != nil {
		return errorValue
	}
	_, errorValue = database.ExecContext(ctx,
		"INSERT OR REPLACE INTO calendar_carried_events(local_event_id, record_event_id) VALUES (?, ?)",
		strings.TrimSpace(localEventID), strings.TrimSpace(recordEventID))
	return errorValue
}

func (service *Service) eventsAlreadyCarriedIntoTheRecord(ctx context.Context) (map[string]string, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	carried := map[string]string{}
	if !calendarTableExists(ctx, database, calendarCarryLinkTable) {
		return carried, nil
	}
	rows, errorValue := database.QueryContext(ctx,
		"SELECT local_event_id, record_event_id FROM calendar_carried_events")
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	for rows.Next() {
		var localEventID, recordEventID string
		if errorValue := rows.Scan(&localEventID, &recordEventID); errorValue != nil {
			return nil, errorValue
		}
		carried[localEventID] = recordEventID
	}
	return carried, rows.Err()
}
