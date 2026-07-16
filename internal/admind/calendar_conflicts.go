package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type calendarConflictRow struct {
	ID          int64  `json:"id"`
	EventID     string `json:"eventID"`
	EventUID    string `json:"eventUID"`
	Field       string `json:"field"`
	LocalValue  string `json:"localValue"`
	RemoteValue string `json:"remoteValue"`
	DetectedAt  string `json:"detectedAt"`
	DismissedAt string `json:"dismissedAt,omitempty"`
}

func (service *Service) recordCalendarConflict(ctx context.Context, eventID string, eventUID string, field string, localValue string, remoteValue string) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, errorValue = database.ExecContext(ctx, `
	INSERT INTO calendar_conflicts (event_id, event_uid, field, local_value, remote_value, detected_at, dismissed_at)
	SELECT ?, ?, ?, ?, ?, ?, ''
	WHERE NOT EXISTS (
		SELECT 1
		FROM calendar_conflicts
		WHERE event_uid = ? AND field = ? AND local_value = ? AND remote_value = ? AND dismissed_at = ''
	)`,
		strings.TrimSpace(eventID),
		strings.TrimSpace(eventUID),
		strings.TrimSpace(field),
		localValue,
		remoteValue,
		now,
		strings.TrimSpace(eventUID),
		strings.TrimSpace(field),
		localValue,
		remoteValue,
	)
	return errorValue
}

func (service *Service) listActiveCalendarConflicts(ctx context.Context) ([]calendarConflictRow, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx,
		`SELECT id, event_id, event_uid, field, local_value, remote_value, detected_at, dismissed_at
FROM calendar_conflicts
WHERE dismissed_at = ''
ORDER BY detected_at DESC, id DESC`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	result := []calendarConflictRow{}
	for rows.Next() {
		var row calendarConflictRow
		if errorValue := rows.Scan(
			&row.ID,
			&row.EventID,
			&row.EventUID,
			&row.Field,
			&row.LocalValue,
			&row.RemoteValue,
			&row.DetectedAt,
			&row.DismissedAt,
		); errorValue != nil {
			return nil, errorValue
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (service *Service) dismissCalendarConflict(ctx context.Context, conflictID int64) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, errorValue = database.ExecContext(ctx,
		`UPDATE calendar_conflicts SET dismissed_at = ? WHERE id = ? AND dismissed_at = ''`,
		now, conflictID)
	return errorValue
}

func (service *Service) recordCalendarFieldConflicts(ctx context.Context, localEvent calendarEvent, remoteEvent calendarEvent, localChangedFields []string) error {
	if len(localChangedFields) == 0 {
		return nil
	}
	previousRemote := decodeCalendarEventFromRawICS(localEvent.RawICS, localEvent.RemoteHref, localEvent.CreatedByEmail)
	remoteChangedFields := diffCalendarEventFields(previousRemote, remoteEvent)
	collisions := intersectCalendarFields(localChangedFields, remoteChangedFields)
	if len(collisions) == 0 {
		return nil
	}
	for _, field := range collisions {
		localValue := calendarEventFieldStringValue(localEvent, field)
		remoteValue := calendarEventFieldStringValue(remoteEvent, field)
		if errorValue := service.recordCalendarConflict(ctx, localEvent.ID, localEvent.UID, field, localValue, remoteValue); errorValue != nil {
			return fmt.Errorf("record calendar conflict %s/%s: %w", localEvent.UID, field, errorValue)
		}
	}
	return nil
}

func calendarEventFieldStringValue(event calendarEvent, field string) string {
	switch field {
	case calendarFieldTitle:
		return event.Title
	case calendarFieldDescription:
		return event.Description
	case calendarFieldLocation:
		return event.Location
	case calendarFieldStart:
		return event.StartISO
	case calendarFieldEnd:
		return event.EndISO
	case calendarFieldTimeZone:
		return event.TimeZone
	case calendarFieldIsAllDay:
		if event.IsAllDay {
			return "true"
		}
		return "false"
	case calendarFieldColor:
		return event.Color
	case calendarFieldReminderLeadHours:
		return strconv.Itoa(event.ReminderLeadHours)
	}
	return ""
}

func calendarConflictIDFromPath(path string) (int64, error) {
	trimmed := strings.TrimSuffix(strings.TrimPrefix(path, "/conflicts/"), "/dismiss")
	identifier, errorValue := strconv.ParseInt(strings.TrimSpace(trimmed), 10, 64)
	if errorValue != nil {
		return 0, fmt.Errorf("invalid conflict id %q: %w", trimmed, errorValue)
	}
	return identifier, nil
}

type calendarConflictsResponse struct {
	Conflicts []calendarConflictRow `json:"conflicts"`
}

func (service *Service) serveCalendarConflicts(writer http.ResponseWriter, request *http.Request) {
	conflicts, errorValue := service.listActiveCalendarConflicts(request.Context())
	if errorValue != nil {
		http.Error(writer, "failed to list calendar conflicts", http.StatusInternalServerError)
		log.Printf("calendar conflicts list: %v", errorValue)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if errorValue := json.NewEncoder(writer).Encode(calendarConflictsResponse{Conflicts: conflicts}); errorValue != nil {
		log.Printf("calendar conflicts encode: %v", errorValue)
	}
}

func (service *Service) dismissCalendarConflictRequest(writer http.ResponseWriter, request *http.Request, path string) {
	conflictID, errorValue := calendarConflictIDFromPath(path)
	if errorValue != nil {
		http.Error(writer, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := service.dismissCalendarConflict(request.Context(), conflictID); errorValue != nil {
		http.Error(writer, "failed to dismiss conflict", http.StatusInternalServerError)
		log.Printf("calendar conflict dismiss %d: %v", conflictID, errorValue)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}
