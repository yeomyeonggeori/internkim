package admind

import (
	"context"
	"database/sql"
	"encoding/xml"
	"errors"
	"strings"
	"time"
)

func (service *Service) openCalendarDatabase(ctx context.Context) (*sql.DB, error) {
	options := sqliteDatabaseOptions{transactionLock: "immediate"}
	database, errorValue := service.openSQLiteDatabaseWithOptions(ctx, service.Configuration.CalendarDatabasePath, ensureCalendarSchema, options)
	if errorValue != nil {
		return nil, errorValue
	}
	service.initializeCalendarEventWindowCache(ctx, database)
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
	created_by_name TEXT NOT NULL DEFAULT '',
	updated_by_email TEXT NOT NULL DEFAULT '',
	updated_by_name TEXT NOT NULL DEFAULT '',
	updated_by_at TEXT NOT NULL DEFAULT '',
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
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_events", "created_by_name", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_events", "updated_by_email", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_events", "updated_by_name", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_events", "updated_by_at", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_events", "mattermost_post_id", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_events", "mattermost_post_created_at", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarParticipantSchema(ctx, database); errorValue != nil {
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
	if errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_events", "remote_source", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_events", "remote_etag", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_events", "remote_href", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarChannelOutboxTable(ctx, database); errorValue != nil {
		return errorValue
	}
	return ensureCalendarSyncSchema(ctx, database)
}

func ensureCalendarChannelOutboxTable(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
	CREATE TABLE IF NOT EXISTS calendar_channel_outbox (
		event_id TEXT PRIMARY KEY,
		generation INTEGER NOT NULL DEFAULT 1,
		lease_owner TEXT NOT NULL DEFAULT '',
		lease_generation INTEGER NOT NULL DEFAULT 0,
		attempt_count INTEGER NOT NULL DEFAULT 0,
		last_error TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		last_attempted_at TEXT NOT NULL DEFAULT ''
	)`)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_channel_outbox", "generation", "INTEGER NOT NULL DEFAULT 1"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_channel_outbox", "lease_owner", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureCalendarColumn(ctx, database, "calendar_channel_outbox", "lease_generation", "INTEGER NOT NULL DEFAULT 0"); errorValue != nil {
		return errorValue
	}
	_, errorValue = database.ExecContext(ctx, `
UPDATE calendar_channel_outbox
SET lease_owner = '', lease_generation = 0
WHERE lease_owner != ''`)
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
	if errorValue != nil && strings.Contains(strings.ToLower(errorValue.Error()), "duplicate column") {
		return nil
	}
	return errorValue
}

func (service *Service) readCalendarEvents(ctx context.Context, startTime time.Time, endTime time.Time) ([]calendarEvent, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	events, errorValue := readCalendarEventRows(ctx, database, startTime, endTime)
	if errorValue != nil {
		return nil, errorValue
	}
	return loadCalendarEventListParticipants(ctx, database, events)
}

func (service *Service) readCalendarEventsWithMattermostPosts(ctx context.Context) ([]calendarEvent, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, uid, title, description, location, start_at, end_at, time_zone, is_all_day, color, raw_ics, reminder_lead_hours, created_by_email, created_by_name, updated_by_email, updated_by_name, updated_by_at, mattermost_post_id, updated_at, remote_source, remote_etag, remote_href
FROM calendar_events
WHERE deleted_at = '' AND mattermost_post_id != ''
ORDER BY start_at, title`)
	if errorValue != nil {
		return nil, errorValue
	}
	events, errorValue := scanCalendarEventRows(rows)
	if errorValue != nil {
		return nil, errorValue
	}
	return loadCalendarEventListParticipants(ctx, database, events)
}

func (service *Service) readCalendarEventIDsRequiringMattermostProjection(ctx context.Context) ([]string, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
	SELECT id
	FROM calendar_events
	WHERE deleted_at = '' OR mattermost_post_id != '' OR id IN (SELECT event_id FROM calendar_channel_outbox)
	ORDER BY updated_at DESC`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	var eventIDs []string
	for rows.Next() {
		var eventID string
		if errorValue := rows.Scan(&eventID); errorValue != nil {
			return nil, errorValue
		}
		eventIDs = append(eventIDs, strings.TrimSpace(eventID))
	}
	return uniqueNonEmpty(eventIDs), rows.Err()
}

func (service *Service) readRemoteCalendarEventsByProvider(ctx context.Context, source string) ([]calendarEvent, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, uid, title, description, location, start_at, end_at, time_zone, is_all_day, color, raw_ics, reminder_lead_hours, created_by_email, created_by_name, updated_by_email, updated_by_name, updated_by_at, mattermost_post_id, updated_at, remote_source, remote_etag, remote_href
FROM calendar_events
WHERE deleted_at = '' AND remote_source = ?
ORDER BY uid`, strings.TrimSpace(source))
	if errorValue != nil {
		return nil, errorValue
	}
	events, errorValue := scanCalendarEventRows(rows)
	if errorValue != nil {
		return nil, errorValue
	}
	return loadCalendarEventListParticipants(ctx, database, events)
}

func (service *Service) readSoftDeletedCalendarEvents(ctx context.Context) ([]calendarEvent, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, uid, title, description, location, start_at, end_at, time_zone, is_all_day, color, raw_ics, reminder_lead_hours, created_by_email, created_by_name, updated_by_email, updated_by_name, updated_by_at, mattermost_post_id, updated_at, remote_source, remote_etag, remote_href
FROM calendar_events
WHERE deleted_at != ''
ORDER BY uid`)
	if errorValue != nil {
		return nil, errorValue
	}
	events, errorValue := scanCalendarEventRows(rows)
	if errorValue != nil {
		return nil, errorValue
	}
	return loadCalendarEventListParticipants(ctx, database, events)
}

func (service *Service) readCalendarEventByID(ctx context.Context, eventID string) (calendarEvent, bool, error) {
	return service.readCalendarEvent(ctx, "id", eventID)
}

func (service *Service) readCalendarEventByUID(ctx context.Context, uid string) (calendarEvent, bool, error) {
	return service.readCalendarEvent(ctx, "uid", uid)
}

type calendarEventProjection struct {
	Event     calendarEvent
	IsDeleted bool
	DeletedAt string
}

func (service *Service) readCalendarEventProjectionByID(ctx context.Context, eventID string) (calendarEventProjection, bool, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return calendarEventProjection{}, false, errorValue
	}
	defer database.Close()
	row := database.QueryRowContext(ctx, `
	SELECT id, uid, title, description, location, start_at, end_at, time_zone, is_all_day, color, raw_ics, reminder_lead_hours, created_by_email, created_by_name, updated_by_email, updated_by_name, updated_by_at, mattermost_post_id, updated_at, remote_source, remote_etag, remote_href, deleted_at
	FROM calendar_events
	WHERE id = ?`, strings.TrimSpace(eventID))
	projection, errorValue := scanCalendarEventProjection(row)
	if errorValue == nil {
		if errorValue := loadCalendarEventParticipants(ctx, database, &projection.Event); errorValue != nil {
			return calendarEventProjection{}, false, errorValue
		}
		return projection, true, nil
	}
	if errors.Is(errorValue, sql.ErrNoRows) {
		return calendarEventProjection{}, false, nil
	}
	return calendarEventProjection{}, false, errorValue
}

func (service *Service) readCalendarEvent(ctx context.Context, columnName string, value string) (calendarEvent, bool, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return calendarEvent{}, false, errorValue
	}
	defer database.Close()
	row := database.QueryRowContext(ctx, `
SELECT id, uid, title, description, location, start_at, end_at, time_zone, is_all_day, color, raw_ics, reminder_lead_hours, created_by_email, created_by_name, updated_by_email, updated_by_name, updated_by_at, mattermost_post_id, updated_at, remote_source, remote_etag, remote_href
FROM calendar_events
WHERE deleted_at = '' AND `+columnName+` = ?`, strings.TrimSpace(value))
	event, errorValue := scanCalendarEvent(row)
	if errorValue == nil {
		if errorValue := loadCalendarEventParticipants(ctx, database, &event); errorValue != nil {
			return calendarEvent{}, false, errorValue
		}
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

func scanCalendarEventRows(rows *sql.Rows) (events []calendarEvent, errorValue error) {
	defer func() {
		closeError := rows.Close()
		if errorValue == nil && closeError != nil {
			errorValue = closeError
		}
	}()
	events = []calendarEvent{}
	for rows.Next() {
		event, errorValue := scanCalendarEvent(rows)
		if errorValue != nil {
			return nil, errorValue
		}
		events = append(events, event)
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	return events, nil
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
		&event.CreatedByName,
		&event.UpdatedByEmail,
		&event.UpdatedByName,
		&event.UpdatedByAt,
		&event.MattermostPostID,
		&event.UpdatedAt,
		&event.RemoteSource,
		&event.RemoteETag,
		&event.RemoteHref,
	)
	event.IsAllDay = isAllDay == 1
	event.ReminderLeadHours = normalizeCalendarReminderLeadHours(event.ReminderLeadHours)
	return event, errorValue
}

func scanCalendarEventProjection(scanner calendarEventScanner) (calendarEventProjection, error) {
	var projection calendarEventProjection
	var isAllDay int
	errorValue := scanner.Scan(
		&projection.Event.ID,
		&projection.Event.UID,
		&projection.Event.Title,
		&projection.Event.Description,
		&projection.Event.Location,
		&projection.Event.StartISO,
		&projection.Event.EndISO,
		&projection.Event.TimeZone,
		&isAllDay,
		&projection.Event.Color,
		&projection.Event.RawICS,
		&projection.Event.ReminderLeadHours,
		&projection.Event.CreatedByEmail,
		&projection.Event.CreatedByName,
		&projection.Event.UpdatedByEmail,
		&projection.Event.UpdatedByName,
		&projection.Event.UpdatedByAt,
		&projection.Event.MattermostPostID,
		&projection.Event.UpdatedAt,
		&projection.Event.RemoteSource,
		&projection.Event.RemoteETag,
		&projection.Event.RemoteHref,
		&projection.DeletedAt,
	)
	projection.Event.IsAllDay = isAllDay == 1
	projection.Event.ReminderLeadHours = normalizeCalendarReminderLeadHours(projection.Event.ReminderLeadHours)
	projection.IsDeleted = strings.TrimSpace(projection.DeletedAt) != ""
	return projection, errorValue
}

func boolToSQLiteInteger(value bool) int {
	if value {
		return 1
	}
	return 0
}

type calendarStoredTextProperty struct {
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

func (service *Service) readCalendarProperties(ctx context.Context, calendarPath string) ([]calendarStoredTextProperty, error) {
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
	properties := []calendarStoredTextProperty{}
	for rows.Next() {
		var xmlns string
		var localName string
		var value string
		if errorValue := rows.Scan(&xmlns, &localName, &value); errorValue != nil {
			return nil, errorValue
		}
		properties = append(properties, calendarStoredTextProperty{
			XMLName: xml.Name{Space: xmlns, Local: localName},
			Value:   value,
		})
	}
	return properties, rows.Err()
}
