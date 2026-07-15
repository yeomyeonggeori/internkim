package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

func (service *Service) writeAttendanceEventOverride(responseWriter http.ResponseWriter, request *http.Request, eventID string) {
	var payload attendanceEventOverrideRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	database, errorValue := service.openAttendanceDatabase(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	defer database.Close()
	event, found, errorValue := service.readAttendanceEventByID(request.Context(), database, eventID)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(responseWriter, request)
		return
	}
	actorEmail := strings.ToLower(strings.TrimSpace(service.webStaffActorEmail(request)))
	if actorEmail == "" {
		http.Error(responseWriter, "attendance staff identity is required", http.StatusForbidden)
		return
	}
	if !service.isAuthorized(request) && !strings.EqualFold(event.Email, actorEmail) {
		http.Error(responseWriter, "attendance event owner or admin required", http.StatusForbidden)
		return
	}
	if event.CanceledAt != "" {
		http.Error(responseWriter, "canceled attendance event cannot be edited", http.StatusBadRequest)
		return
	}
	override, errorValue := service.createAttendanceEventOverride(request.Context(), database, event, payload, actorEmail, time.Now().UTC())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := service.insertAttendanceEventOverride(request.Context(), database, override); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	events, errorValue := service.applyAttendanceEventOverrides(request.Context(), database, []attendanceEvent{event})
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, map[string]attendanceEvent{"event": events[0]})
}

func (service *Service) createAttendanceEventOverride(ctx context.Context, database *sql.DB, event attendanceEvent, payload attendanceEventOverrideRequest, actorEmail string, editedAt time.Time) (attendanceEventOverride, error) {
	reason := strings.TrimSpace(payload.Reason)
	if reason == "" {
		return attendanceEventOverride{}, fmt.Errorf("override reason is required")
	}
	location, errorValue := service.attendanceOverrideLocationByID(payload.LocationID)
	if errorValue != nil {
		return attendanceEventOverride{}, errorValue
	}
	localTime, errorValue := service.parseAttendanceOverrideLocalTime(payload.LocalDate, payload.LocalTime)
	if errorValue != nil {
		return attendanceEventOverride{}, errorValue
	}
	if localTime.After(editedAt) {
		return attendanceEventOverride{}, fmt.Errorf("attendance event time cannot be in the future")
	}
	events, errorValue := service.applyAttendanceEventOverrides(ctx, database, []attendanceEvent{event})
	if errorValue != nil {
		return attendanceEventOverride{}, errorValue
	}
	effectiveEvent := events[0]
	return attendanceEventOverride{
		ID:                   "attendance-event-override-" + randomHex(16),
		EventID:              event.ID,
		EditedBy:             actorEmail,
		EditedAt:             editedAt.Format(time.RFC3339Nano),
		Reason:               reason,
		OriginalOccurredAt:   effectiveEvent.OccurredAt,
		OriginalLocalDate:    effectiveEvent.LocalDate,
		OriginalLocalTime:    effectiveEvent.LocalTime,
		OriginalLocationID:   effectiveEvent.LocationID,
		OriginalLocationName: effectiveEvent.LocationName,
		OverrideOccurredAt:   localTime.UTC().Format(time.RFC3339Nano),
		OverrideLocalDate:    localTime.Format("2006-01-02"),
		OverrideLocalTime:    localTime.Format("15:04:05"),
		OverrideLocationID:   location.ID,
		OverrideLocationName: location.Name,
	}, nil
}

func (service *Service) parseAttendanceOverrideLocalTime(localDate string, localTime string) (time.Time, error) {
	normalizedDate := strings.TrimSpace(localDate)
	normalizedTime := strings.TrimSpace(localTime)
	if _, errorValue := time.Parse("2006-01-02", normalizedDate); errorValue != nil {
		return time.Time{}, fmt.Errorf("invalid localDate")
	}
	if _, errorValue := time.Parse("15:04", normalizedTime); errorValue == nil {
		normalizedTime += ":00"
	}
	location, _ := service.workspaceTimeLocation()
	parsedTime, errorValue := time.ParseInLocation("2006-01-02 15:04:05", normalizedDate+" "+normalizedTime, location)
	if errorValue != nil {
		return time.Time{}, fmt.Errorf("invalid localTime")
	}
	return parsedTime, nil
}

func (service *Service) attendanceOverrideLocationByID(locationID string) (attendanceLocation, error) {
	normalizedLocationID := strings.TrimSpace(locationID)
	if normalizedLocationID == "" {
		return attendanceLocation{}, fmt.Errorf("locationID is required")
	}
	locations, errorValue := service.readAttendanceLocations()
	if errorValue != nil {
		return attendanceLocation{}, errorValue
	}
	for _, location := range locations {
		if location.ID == normalizedLocationID {
			return location, nil
		}
	}
	return attendanceLocation{}, fmt.Errorf("attendance location was not found")
}

func (service *Service) readAttendanceEventByID(ctx context.Context, database *sql.DB, eventID string) (attendanceEvent, bool, error) {
	row := database.QueryRowContext(ctx, "SELECT "+attendanceEventSelectColumns+" FROM attendance_events WHERE id = ?", strings.TrimSpace(eventID))
	event, errorValue := scanAttendanceEvent(row)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return attendanceEvent{}, false, nil
	}
	if errorValue != nil {
		return attendanceEvent{}, false, errorValue
	}
	return event, true, nil
}

func (service *Service) applyAttendanceEventOverrides(ctx context.Context, database *sql.DB, events []attendanceEvent) ([]attendanceEvent, error) {
	if len(events) == 0 {
		return events, nil
	}
	overridesByEventID, errorValue := service.readAttendanceEventOverridesForEvents(ctx, database, events)
	if errorValue != nil {
		return nil, errorValue
	}
	updatedEvents := make([]attendanceEvent, 0, len(events))
	for _, event := range events {
		overrides := overridesByEventID[event.ID]
		if len(overrides) > 0 {
			latestOverride := overrides[len(overrides)-1]
			event.OriginalOccurredAt = latestOverride.OriginalOccurredAt
			event.OriginalLocalDate = latestOverride.OriginalLocalDate
			event.OriginalLocalTime = latestOverride.OriginalLocalTime
			event.OriginalLocationID = latestOverride.OriginalLocationID
			event.OriginalLocationName = latestOverride.OriginalLocationName
			event.OccurredAt = latestOverride.OverrideOccurredAt
			event.LocalDate = latestOverride.OverrideLocalDate
			event.LocalTime = latestOverride.OverrideLocalTime
			event.LocationID = latestOverride.OverrideLocationID
			event.LocationName = latestOverride.OverrideLocationName
			event.OverrideReason = latestOverride.Reason
			event.OverriddenBy = latestOverride.EditedBy
			event.OverriddenAt = latestOverride.EditedAt
			event.OverrideHistory = overrides
		}
		updatedEvents = append(updatedEvents, event)
	}
	sort.SliceStable(updatedEvents, func(firstIndex int, secondIndex int) bool {
		return updatedEvents[firstIndex].OccurredAt > updatedEvents[secondIndex].OccurredAt
	})
	return updatedEvents, nil
}

func (service *Service) readAttendanceEventOverridesForEvents(ctx context.Context, database *sql.DB, events []attendanceEvent) (map[string][]attendanceEventOverride, error) {
	eventIDs := make([]string, 0, len(events))
	for _, event := range events {
		eventIDs = append(eventIDs, event.ID)
	}
	placeholders := make([]string, 0, len(eventIDs))
	arguments := make([]any, 0, len(eventIDs))
	for _, eventID := range eventIDs {
		placeholders = append(placeholders, "?")
		arguments = append(arguments, eventID)
	}
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, event_id, edited_by, edited_at, reason,
	original_occurred_at, original_local_date, original_local_time, original_location_id, original_location_name,
	override_occurred_at, override_local_date, override_local_time, override_location_id, override_location_name
FROM attendance_event_overrides
WHERE event_id IN (`+strings.Join(placeholders, ",")+`)
ORDER BY event_id ASC, edited_at ASC`, arguments...)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	overridesByEventID := map[string][]attendanceEventOverride{}
	for rows.Next() {
		override, errorValue := scanAttendanceEventOverride(rows)
		if errorValue != nil {
			return nil, errorValue
		}
		overridesByEventID[override.EventID] = append(overridesByEventID[override.EventID], override)
	}
	return overridesByEventID, rows.Err()
}

func scanAttendanceEventOverride(rows *sql.Rows) (attendanceEventOverride, error) {
	var override attendanceEventOverride
	errorValue := rows.Scan(
		&override.ID,
		&override.EventID,
		&override.EditedBy,
		&override.EditedAt,
		&override.Reason,
		&override.OriginalOccurredAt,
		&override.OriginalLocalDate,
		&override.OriginalLocalTime,
		&override.OriginalLocationID,
		&override.OriginalLocationName,
		&override.OverrideOccurredAt,
		&override.OverrideLocalDate,
		&override.OverrideLocalTime,
		&override.OverrideLocationID,
		&override.OverrideLocationName,
	)
	return override, errorValue
}
