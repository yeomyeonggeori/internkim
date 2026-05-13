package admind

// 캘린더 SQLite 스키마, 이벤트 read/write/soft-delete, 저장된 DAV property I/O.

import (
	"context"
	"database/sql"
	"encoding/xml"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

func (service *Service) openCalendarDatabase(ctx context.Context) (*sql.DB, error) {
	if errorValue := os.MkdirAll(filepath.Dir(service.Configuration.CalendarDatabasePath), 0o700); errorValue != nil {
		return nil, errorValue
	}
	database, errorValue := sql.Open("sqlite", service.Configuration.CalendarDatabasePath)
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := ensureCalendarSchema(ctx, database); errorValue != nil {
		_ = database.Close()
		return nil, errorValue
	}
	return database, nil
}

func ensureCalendarSchema(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS calendar_events (
	id TEXT PRIMARY KEY,
	uid TEXT NOT NULL UNIQUE,
	title TEXT NOT NULL,
	description TEXT NOT NULL,
	location TEXT NOT NULL,
	start_at TEXT NOT NULL,
	end_at TEXT NOT NULL,
	time_zone TEXT NOT NULL,
	is_all_day INTEGER NOT NULL,
	color TEXT NOT NULL,
	raw_ics TEXT NOT NULL,
	reminder_lead_hours INTEGER NOT NULL DEFAULT 24,
	created_by_email TEXT NOT NULL,
	mattermost_post_id TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL,
	deleted_at TEXT NOT NULL
)`)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_events", "reminder_lead_hours", "INTEGER NOT NULL DEFAULT 24"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_events", "mattermost_post_id", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	_, errorValue = database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS calendar_settings (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
)`)
	if errorValue != nil {
		return errorValue
	}
	_, errorValue = database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS calendar_event_notifications (
	event_id TEXT NOT NULL,
	recipient_key TEXT NOT NULL,
	target_type TEXT NOT NULL,
	target_value TEXT NOT NULL,
	target_label TEXT NOT NULL,
	notify_at TEXT NOT NULL,
	status TEXT NOT NULL,
	sent_at TEXT NOT NULL,
	error TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY(event_id, recipient_key)
)`)
	if errorValue != nil {
		return errorValue
	}
	_, errorValue = database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS calendar_properties (
	calendar_path TEXT NOT NULL,
	property_xmlns TEXT NOT NULL,
	property_local_name TEXT NOT NULL,
	value TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY(calendar_path, property_xmlns, property_local_name)
)`)
	return errorValue
}

func ensureCalendarColumn(ctx context.Context, database *sql.DB, tableName string, columnName string, definition string) error {
	rows, errorValue := database.QueryContext(ctx, "PRAGMA table_info("+tableName+")")
	if errorValue != nil {
		return errorValue
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if errorValue := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); errorValue != nil {
			return errorValue
		}
		if name == columnName {
			return nil
		}
	}
	if errorValue := rows.Err(); errorValue != nil {
		return errorValue
	}
	_, errorValue = database.ExecContext(ctx, "ALTER TABLE "+tableName+" ADD COLUMN "+columnName+" "+definition)
	return errorValue
}

func (service *Service) readCalendarEvents(ctx context.Context, startTime time.Time, endTime time.Time) ([]calendarEvent, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	query := `
SELECT id, uid, title, description, location, start_at, end_at, time_zone, is_all_day, color, raw_ics, reminder_lead_hours, created_by_email, mattermost_post_id, updated_at
FROM calendar_events
WHERE deleted_at = ''`
	arguments := []any{}
	if !startTime.IsZero() && !endTime.IsZero() {
		query += " AND end_at > ? AND start_at < ?"
		arguments = append(arguments, startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))
	}
	query += " ORDER BY start_at, title"
	rows, errorValue := database.QueryContext(ctx, query, arguments...)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	events := []calendarEvent{}
	for rows.Next() {
		event, errorValue := scanCalendarEvent(rows)
		if errorValue != nil {
			return nil, errorValue
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (service *Service) readCalendarEventByID(ctx context.Context, eventID string) (calendarEvent, bool, error) {
	return service.readCalendarEvent(ctx, "id", eventID)
}

func (service *Service) readCalendarEventByUID(ctx context.Context, uid string) (calendarEvent, bool, error) {
	return service.readCalendarEvent(ctx, "uid", uid)
}

func (service *Service) readCalendarEvent(ctx context.Context, columnName string, value string) (calendarEvent, bool, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return calendarEvent{}, false, errorValue
	}
	defer database.Close()
	row := database.QueryRowContext(ctx, `
SELECT id, uid, title, description, location, start_at, end_at, time_zone, is_all_day, color, raw_ics, reminder_lead_hours, created_by_email, mattermost_post_id, updated_at
FROM calendar_events
WHERE deleted_at = '' AND `+columnName+` = ?`, strings.TrimSpace(value))
	event, errorValue := scanCalendarEvent(row)
	if errorValue == nil {
		return event, true, nil
	}
	if errors.Is(errorValue, sql.ErrNoRows) {
		return calendarEvent{}, false, nil
	}
	return calendarEvent{}, false, errorValue
}

type calendarEventScanner interface {
	Scan(destinations ...any) error
}

func scanCalendarEvent(scanner calendarEventScanner) (calendarEvent, error) {
	var event calendarEvent
	var isAllDay int
	errorValue := scanner.Scan(
		&event.ID,
		&event.UID,
		&event.Title,
		&event.Description,
		&event.Location,
		&event.StartISO,
		&event.EndISO,
		&event.TimeZone,
		&isAllDay,
		&event.Color,
		&event.RawICS,
		&event.ReminderLeadHours,
		&event.CreatedByEmail,
		&event.MattermostPostID,
		&event.UpdatedAt,
	)
	event.IsAllDay = isAllDay == 1
	event.ReminderLeadHours = normalizeCalendarReminderLeadHours(event.ReminderLeadHours)
	return event, errorValue
}

func (service *Service) writeCalendarEvent(ctx context.Context, event calendarEvent) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	updatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	_, errorValue = database.ExecContext(ctx, `
INSERT INTO calendar_events (
	id, uid, title, description, location, start_at, end_at, time_zone, is_all_day, color, raw_ics, reminder_lead_hours, created_by_email, mattermost_post_id, updated_at, deleted_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '')
ON CONFLICT(id) DO UPDATE SET
	uid = excluded.uid,
	title = excluded.title,
	description = excluded.description,
	location = excluded.location,
	start_at = excluded.start_at,
	end_at = excluded.end_at,
	time_zone = excluded.time_zone,
	is_all_day = excluded.is_all_day,
	color = excluded.color,
	raw_ics = excluded.raw_ics,
	reminder_lead_hours = excluded.reminder_lead_hours,
	mattermost_post_id = excluded.mattermost_post_id,
	updated_at = excluded.updated_at,
	deleted_at = ''`,
		event.ID,
		event.UID,
		event.Title,
		event.Description,
		event.Location,
		event.StartISO,
		event.EndISO,
		event.TimeZone,
		boolToSQLiteInteger(event.IsAllDay),
		event.Color,
		event.RawICS,
		normalizeCalendarReminderLeadHours(event.ReminderLeadHours),
		event.CreatedByEmail,
		event.MattermostPostID,
		updatedAt,
	)
	event.UpdatedAt = updatedAt
	if errorValue == nil {
		service.upsertCalendarNotifications(ctx, event)
		service.syncCalendarMattermostLog(ctx, event)
	}
	return errorValue
}

func (service *Service) softDeleteCalendarEvent(ctx context.Context, eventID string) error {
	event, found, errorValue := service.readCalendarEventByID(ctx, eventID)
	if errorValue != nil {
		return errorValue
	}
	if !found {
		return sql.ErrNoRows
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	result, errorValue := database.ExecContext(ctx, "UPDATE calendar_events SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at = ''", time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano), strings.TrimSpace(eventID))
	if errorValue != nil {
		return errorValue
	}
	affectedRows, errorValue := result.RowsAffected()
	if errorValue != nil {
		return errorValue
	}
	if affectedRows == 0 {
		return sql.ErrNoRows
	}
	if errorValue := service.cancelCalendarNotifications(ctx, eventID); errorValue != nil {
		log.Printf("calendar notification cancel failed: %v", errorValue)
	}
	service.deleteCalendarMattermostLog(ctx, event)
	return nil
}

func (service *Service) updateCalendarEventMattermostPostID(ctx context.Context, eventID string, postID string) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, "UPDATE calendar_events SET mattermost_post_id = ?, updated_at = ? WHERE id = ?", strings.TrimSpace(postID), time.Now().UTC().Format(time.RFC3339Nano), strings.TrimSpace(eventID))
	return errorValue
}

func boolToSQLiteInteger(value bool) int {
	if value {
		return 1
	}
	return 0
}

type calendarStoredProperty struct {
	XMLName xml.Name
	Value   string
}

func (service *Service) writeCalendarProperty(ctx context.Context, calendarPath string, xmlns string, localName string, value string) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	updatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	_, errorValue = database.ExecContext(ctx, `
INSERT INTO calendar_properties (calendar_path, property_xmlns, property_local_name, value, updated_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(calendar_path, property_xmlns, property_local_name) DO UPDATE SET
	value = excluded.value,
	updated_at = excluded.updated_at`,
		calendarPath, xmlns, localName, value, updatedAt)
	return errorValue
}

func (service *Service) deleteCalendarProperty(ctx context.Context, calendarPath string, xmlns string, localName string) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx,
		"DELETE FROM calendar_properties WHERE calendar_path = ? AND property_xmlns = ? AND property_local_name = ?",
		calendarPath, xmlns, localName)
	return errorValue
}

func (service *Service) readCalendarProperties(ctx context.Context, calendarPath string) ([]calendarStoredProperty, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx,
		"SELECT property_xmlns, property_local_name, value FROM calendar_properties WHERE calendar_path = ? ORDER BY property_xmlns, property_local_name",
		calendarPath)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	properties := []calendarStoredProperty{}
	for rows.Next() {
		var xmlns string
		var localName string
		var value string
		if errorValue := rows.Scan(&xmlns, &localName, &value); errorValue != nil {
			return nil, errorValue
		}
		properties = append(properties, calendarStoredProperty{
			XMLName: xml.Name{Space: xmlns, Local: localName},
			Value:   value,
		})
	}
	return properties, rows.Err()
}
