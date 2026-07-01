package admind

import (
	"context"
	"database/sql"
	"strings"
)

func ensureCalendarParticipantSchema(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS calendar_event_participants (
	event_id TEXT NOT NULL,
	person_id TEXT NOT NULL,
	name TEXT NOT NULL,
	email TEXT NOT NULL DEFAULT '',
	sort_order INTEGER NOT NULL,
	PRIMARY KEY(event_id, person_id, email, name)
)`)
	return errorValue
}

func replaceCalendarEventParticipants(ctx context.Context, transaction *sql.Tx, eventID string, participants []calendarParticipantIdentity) error {
	normalizedParticipants := normalizeCalendarParticipantIdentities(participants)
	if _, errorValue := transaction.ExecContext(ctx, "DELETE FROM calendar_event_participants WHERE event_id = ?", strings.TrimSpace(eventID)); errorValue != nil {
		return errorValue
	}
	for index, participant := range normalizedParticipants {
		if _, errorValue := transaction.ExecContext(ctx, `
INSERT INTO calendar_event_participants (event_id, person_id, name, email, sort_order)
VALUES (?, ?, ?, ?, ?)`,
			strings.TrimSpace(eventID),
			participant.PersonID,
			participant.Name,
			participant.Email,
			index,
		); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func readCalendarEventParticipants(ctx context.Context, database *sql.DB, eventID string) ([]calendarParticipantIdentity, error) {
	rows, errorValue := database.QueryContext(ctx, `
SELECT person_id, name, email
FROM calendar_event_participants
WHERE event_id = ?
ORDER BY sort_order, name`, strings.TrimSpace(eventID))
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	participants := []calendarParticipantIdentity{}
	for rows.Next() {
		var participant calendarParticipantIdentity
		if errorValue := rows.Scan(&participant.PersonID, &participant.Name, &participant.Email); errorValue != nil {
			return nil, errorValue
		}
		participants = append(participants, participant)
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	return normalizeCalendarParticipantIdentities(participants), nil
}

func loadCalendarEventListParticipants(ctx context.Context, database *sql.DB, events []calendarEvent) ([]calendarEvent, error) {
	if len(events) == 0 {
		return events, nil
	}
	eventIDs := make([]string, 0, len(events))
	eventIndexByID := map[string]int{}
	for index, event := range events {
		eventID := strings.TrimSpace(event.ID)
		if eventID == "" {
			continue
		}
		eventIDs = append(eventIDs, eventID)
		eventIndexByID[eventID] = index
	}
	if len(eventIDs) == 0 {
		return events, nil
	}
	query := `
SELECT event_id, person_id, name, email
FROM calendar_event_participants
WHERE event_id IN (` + queryPlaceholders(len(eventIDs)) + `)
ORDER BY event_id, sort_order, name`
	arguments := make([]any, 0, len(eventIDs))
	for _, eventID := range eventIDs {
		arguments = append(arguments, eventID)
	}
	rows, errorValue := database.QueryContext(ctx, query, arguments...)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	for rows.Next() {
		var eventID string
		var participant calendarParticipantIdentity
		if errorValue := rows.Scan(&eventID, &participant.PersonID, &participant.Name, &participant.Email); errorValue != nil {
			return nil, errorValue
		}
		index, found := eventIndexByID[strings.TrimSpace(eventID)]
		if found {
			events[index].Participants = append(events[index].Participants, calendarParticipant{
				PersonID: participant.PersonID,
				Name:     participant.Name,
				Email:    participant.Email,
			})
		}
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	for index := range events {
		events[index].Participants = calendarEventDisplayParticipants(events[index])
		events[index].People = calendarParticipantNames(events[index].Participants)
	}
	return events, nil
}

func loadCalendarEventParticipants(ctx context.Context, database *sql.DB, event *calendarEvent) error {
	participants, errorValue := readCalendarEventParticipants(ctx, database, event.ID)
	if errorValue != nil {
		return errorValue
	}
	event.Participants = calendarEventDisplayParticipants(calendarEvent{
		Description:  event.Description,
		Participants: calendarParticipantsFromIdentities(participants),
	})
	event.People = calendarParticipantNames(event.Participants)
	return nil
}

func calendarEventDisplayParticipants(event calendarEvent) []calendarParticipant {
	participants := normalizeCalendarParticipants(event.Participants)
	if len(participants) > 0 {
		return participants
	}
	if people, hasPeopleLine := calendarPeopleFromDescription(event.Description); hasPeopleLine {
		return calendarParticipantsFromPeople(people)
	}
	return participants
}

func queryPlaceholders(count int) string {
	if count <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", count), ",")
}
