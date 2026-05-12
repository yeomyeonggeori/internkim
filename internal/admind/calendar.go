package admind

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"

	_ "modernc.org/sqlite"
)

const (
	calendarProductID                       = "-//InternKim//Shared Calendar//EN"
	calendarName                            = "Work"
	calendarDAVUsername                     = "internkim"
	calendarPrincipalPath                   = "/calendar/dav/team/"
	calendarHomeSetPath                     = "/calendar/dav/team/calendars/"
	calendarCollectionPath                  = "/calendar/dav/team/calendars/internkim/"
	calendarSettingsICSKey                  = "ics_token"
	calendarDefaultReminderLeadHours        = 24
	calendarAnnouncementsChannelName        = "announcements"
	calendarAnnouncementsChannelDisplayName = "Announcements"
)

type calendarEvent struct {
	ID                string   `json:"id"`
	UID               string   `json:"uid"`
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	Location          string   `json:"location"`
	StartISO          string   `json:"startISO"`
	EndISO            string   `json:"endISO"`
	TimeZone          string   `json:"timeZone"`
	IsAllDay          bool     `json:"isAllDay"`
	Color             string   `json:"color"`
	People            []string `json:"people"`
	ReminderLeadHours int      `json:"reminderLeadHours"`
	CreatedByEmail    string   `json:"createdByEmail"`
	UpdatedAt         string   `json:"updatedAt"`
	MattermostPostID  string   `json:"mattermostPostID,omitempty"`
	RawICS            string   `json:"-"`
}

type calendarEventWriteRequest struct {
	EventID           string              `json:"eventID"`
	Title             string              `json:"title"`
	Description       string              `json:"description"`
	Location          string              `json:"location"`
	StartISO          string              `json:"startISO"`
	EndISO            string              `json:"endISO"`
	TimeZone          string              `json:"timeZone"`
	IsAllDay          bool                `json:"isAllDay"`
	Color             string              `json:"color"`
	People            calendarPeopleInput `json:"people"`
	ReminderLeadHours int                 `json:"reminderLeadHours"`
}

type calendarEventsResponse struct {
	Events []calendarEvent `json:"events"`
}

type calendarSyncResponse struct {
	CalDAVURL      string `json:"caldavURL"`
	CalDAVUsername string `json:"caldavUsername"`
	CalDAVPassword string `json:"caldavPassword"`
	ICSURL         string `json:"icsURL"`
}

type calendarDAVBackend struct {
	service *Service
}

type calendarPeopleInput []string

type calendarNotificationTarget struct {
	TargetType string
	Key        string
	Label      string
	UserID     string
}

type calendarNotification struct {
	EventID      string
	RecipientKey string
	TargetType   string
	TargetValue  string
	TargetLabel  string
	NotifyAt     string
	Status       string
}

func (service *Service) serveCalendarPage(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/calendar" {
		http.Redirect(responseWriter, request, "/calendar/", http.StatusFound)
		return
	}
	if service.serveCalendarStaticFile(responseWriter, request) {
		return
	}
	service.serveCalendarIndex(responseWriter, request)
}

func (service *Service) serveCalendarStaticFile(responseWriter http.ResponseWriter, request *http.Request) bool {
	relativePath := strings.TrimPrefix(request.URL.Path, "/calendar/")
	if relativePath == "" {
		return false
	}
	filePath := filepath.Join(service.Configuration.AdminUIPath, "calendar", relativePath)
	fileInformation, errorValue := os.Stat(filePath)
	if errorValue != nil || fileInformation.IsDir() {
		return false
	}
	http.ServeFile(responseWriter, request, filePath)
	return true
}

func (service *Service) serveCalendarIndex(responseWriter http.ResponseWriter, request *http.Request) {
	calendarIndexPath := filepath.Join(service.Configuration.AdminUIPath, "calendar", "index.html")
	if fileInformation, errorValue := os.Stat(calendarIndexPath); errorValue == nil && !fileInformation.IsDir() {
		http.ServeFile(responseWriter, request, calendarIndexPath)
		return
	}
	http.ServeFile(responseWriter, request, filepath.Join(service.Configuration.AdminUIPath, "index.html"))
}

func (service *Service) handleCalendar(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.authorizeCalendarRequest(request) {
		http.Error(responseWriter, "calendar access required", http.StatusForbidden)
		return
	}
	path := strings.TrimPrefix(request.URL.Path, "/calendar/api")
	switch {
	case request.Method == http.MethodGet && path == "/events":
		service.listCalendarEvents(responseWriter, request)
	case request.Method == http.MethodPost && path == "/events":
		service.createCalendarEvent(responseWriter, request)
	case request.Method == http.MethodPut && strings.HasPrefix(path, "/events/"):
		service.updateCalendarEvent(responseWriter, request, strings.TrimPrefix(path, "/events/"))
	case request.Method == http.MethodDelete && strings.HasPrefix(path, "/events/"):
		service.deleteCalendarEvent(responseWriter, request, strings.TrimPrefix(path, "/events/"))
	case request.Method == http.MethodGet && path == "/sync":
		service.writeCalendarSync(responseWriter, request)
	case request.Method == http.MethodPost && path == "/ics-token":
		service.rotateCalendarICSToken(responseWriter, request)
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) authorizeCalendarRequest(request *http.Request) bool {
	if isLocalRequest(request) {
		return true
	}
	if service.authorizeCalendarTokenRequest(request) {
		return true
	}
	actorEmail := authenticatedCallerEmail(request)
	if actorEmail == "" {
		return false
	}
	return service.isFlowStaffActor(request.Context(), actorEmail)
}

func (service *Service) authorizeCalendarTokenRequest(request *http.Request) bool {
	username, password, ok := request.BasicAuth()
	if !ok {
		return false
	}
	token := firstNonEmpty(password, username)
	return service.isValidCalendarICSToken(request.Context(), token)
}

func (service *Service) listCalendarEvents(responseWriter http.ResponseWriter, request *http.Request) {
	startTime, endTime, errorValue := parseCalendarRange(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	events, errorValue := service.readCalendarEvents(request.Context(), startTime, endTime)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, calendarEventsResponse{Events: events})
}

func (service *Service) createCalendarEvent(responseWriter http.ResponseWriter, request *http.Request) {
	event, errorValue := service.decodeCalendarEventWriteRequest(request, "")
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := service.writeCalendarEvent(request.Context(), event); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	responseWriter.WriteHeader(http.StatusCreated)
	service.writeJSON(responseWriter, event)
}

func (service *Service) updateCalendarEvent(responseWriter http.ResponseWriter, request *http.Request, eventID string) {
	existingEvent, found, errorValue := service.readCalendarEventByID(request.Context(), eventID)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(responseWriter, request)
		return
	}
	event, errorValue := service.decodeCalendarEventWriteRequest(request, existingEvent.ID)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	event.UID = existingEvent.UID
	event.CreatedByEmail = existingEvent.CreatedByEmail
	event.MattermostPostID = existingEvent.MattermostPostID
	if errorValue := service.writeCalendarEvent(request.Context(), event); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, event)
}

func (service *Service) deleteCalendarEvent(responseWriter http.ResponseWriter, request *http.Request, eventID string) {
	if errorValue := service.softDeleteCalendarEvent(request.Context(), eventID); errorValue != nil {
		if errors.Is(errorValue, sql.ErrNoRows) {
			http.NotFound(responseWriter, request)
			return
		}
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (service *Service) writeCalendarSync(responseWriter http.ResponseWriter, request *http.Request) {
	token, errorValue := service.ensureCalendarICSToken(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	baseURL := service.calendarExternalBaseURL(request)
	service.writeJSON(responseWriter, calendarSyncResponse{
		CalDAVURL:      calendarDAVSubscriptionURL(baseURL, token),
		CalDAVUsername: calendarDAVUsername,
		CalDAVPassword: token,
		ICSURL:         baseURL + "/calendar/ics/" + url.PathEscape(token) + ".ics",
	})
}

func (service *Service) rotateCalendarICSToken(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.isAuthorized(request) {
		http.Error(responseWriter, "admin access required", http.StatusForbidden)
		return
	}
	token, errorValue := service.writeCalendarICSToken(request.Context(), randomHex(32))
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	baseURL := service.calendarExternalBaseURL(request)
	service.writeJSON(responseWriter, calendarSyncResponse{
		CalDAVURL:      calendarDAVSubscriptionURL(baseURL, token),
		CalDAVUsername: calendarDAVUsername,
		CalDAVPassword: token,
		ICSURL:         baseURL + "/calendar/ics/" + url.PathEscape(token) + ".ics",
	})
}

func (service *Service) serveCalendarICS(responseWriter http.ResponseWriter, request *http.Request) {
	token := strings.TrimSuffix(strings.TrimPrefix(request.URL.Path, "/calendar/ics/"), ".ics")
	if token == "" || !service.isValidCalendarICSToken(request.Context(), token) {
		http.NotFound(responseWriter, request)
		return
	}
	events, errorValue := service.readCalendarEvents(request.Context(), time.Time{}, time.Time{})
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	var calendar *ical.Calendar
	if len(events) > 0 {
		calendar, errorValue = buildCalendarFeed(events)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
			return
		}
	}
	responseWriter.Header().Set("Content-Type", ical.MIMEType+"; charset=utf-8")
	responseWriter.Header().Set("Content-Disposition", `inline; filename="internkim.ics"`)
	responseWriter.Header().Set("Cache-Control", "no-store")
	if len(events) == 0 {
		calendar = newCalendarFeedDocument()
	}
	_ = ical.NewEncoder(responseWriter).Encode(calendar)
}

func (service *Service) serveCalendarDAV(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.authorizeCalendarRequest(request) {
		http.Error(responseWriter, "calendar access required", http.StatusForbidden)
		return
	}
	if request.Method == "REPORT" {
		body, err := io.ReadAll(request.Body)
		if err == nil {
			request.Body = io.NopCloser(bytes.NewReader(body))
			if bytes.Contains(body, []byte("sync-collection")) {
				writeCalendarSyncCollectionFallback(responseWriter)
				return
			}
		}
	}
	if request.Method == "PROPPATCH" {
		service.handleCalendarPropPatch(responseWriter, request)
		return
	}
	if request.Method == "PROPFIND" {
		service.handleCalendarPropFind(responseWriter, request)
		return
	}
	service.invokeCalendarDAVHandler(responseWriter, request)
}

func (service *Service) invokeCalendarDAVHandler(responseWriter http.ResponseWriter, request *http.Request) {
	handler := caldav.Handler{
		Backend: calendarDAVBackend{service: service},
		Prefix:  "/calendar/dav",
	}
	handler.ServeHTTP(responseWriter, request)
}

func writeCalendarSyncCollectionFallback(responseWriter http.ResponseWriter) {
	responseWriter.Header().Set("Content-Type", "application/xml; charset=utf-8")
	responseWriter.WriteHeader(http.StatusForbidden)
	_, _ = responseWriter.Write([]byte(
		`<?xml version="1.0" encoding="utf-8"?>` +
			`<D:error xmlns:D="DAV:"><D:valid-sync-token/></D:error>`,
	))
}

func (service *Service) decodeCalendarEventWriteRequest(request *http.Request, eventID string) (calendarEvent, error) {
	var payload calendarEventWriteRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		return calendarEvent{}, errorValue
	}
	return service.normalizeCalendarEventWriteRequest(request, payload, eventID)
}

func (service *Service) normalizeCalendarEventWriteRequest(request *http.Request, payload calendarEventWriteRequest, eventID string) (calendarEvent, error) {
	title := strings.TrimSpace(payload.Title)
	if title == "" {
		return calendarEvent{}, errors.New("title is required")
	}
	startTime, errorValue := time.Parse(time.RFC3339, strings.TrimSpace(payload.StartISO))
	if errorValue != nil {
		return calendarEvent{}, errors.New("startISO must be RFC3339")
	}
	endTime, errorValue := time.Parse(time.RFC3339, strings.TrimSpace(payload.EndISO))
	if errorValue != nil {
		return calendarEvent{}, errors.New("endISO must be RFC3339")
	}
	if !endTime.After(startTime) {
		return calendarEvent{}, errors.New("endISO must be after startISO")
	}
	id := strings.TrimSpace(eventID)
	if id == "" {
		id = strings.TrimSpace(payload.EventID)
	}
	if id == "" {
		id = randomHex(16)
	}
	people := normalizeCalendarPeople([]string(payload.People))
	description := calendarDescriptionWithPeople(people, payload.Description)
	event := calendarEvent{
		ID:                id,
		UID:               id + "@internkim",
		Title:             title,
		Description:       description,
		Location:          strings.TrimSpace(payload.Location),
		StartISO:          startTime.UTC().Format(time.RFC3339),
		EndISO:            endTime.UTC().Format(time.RFC3339),
		TimeZone:          firstNonEmpty(strings.TrimSpace(payload.TimeZone), "UTC"),
		IsAllDay:          payload.IsAllDay,
		Color:             firstNonEmpty(strings.TrimSpace(payload.Color), "#2563eb"),
		People:            people,
		ReminderLeadHours: normalizeCalendarReminderLeadHours(payload.ReminderLeadHours),
		CreatedByEmail:    authenticatedCallerEmail(request),
	}
	rawICS, errorValue := encodeCalendarObject(event)
	if errorValue != nil {
		return calendarEvent{}, errorValue
	}
	event.RawICS = rawICS
	return event, nil
}

func (people *calendarPeopleInput) UnmarshalJSON(document []byte) error {
	trimmedDocument := bytes.TrimSpace(document)
	if len(trimmedDocument) == 0 || bytes.Equal(trimmedDocument, []byte("null")) {
		*people = nil
		return nil
	}
	var values []string
	if errorValue := json.Unmarshal(trimmedDocument, &values); errorValue == nil {
		*people = normalizeCalendarPeople(values)
		return nil
	}
	var value string
	if errorValue := json.Unmarshal(trimmedDocument, &value); errorValue != nil {
		return errorValue
	}
	*people = normalizeCalendarPeople(strings.Split(value, ","))
	return nil
}

func normalizeCalendarPeople(values []string) []string {
	people := []string{}
	seenPeople := map[string]bool{}
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue == "" {
			continue
		}
		normalizedValue := strings.ToLower(trimmedValue)
		if seenPeople[normalizedValue] {
			continue
		}
		seenPeople[normalizedValue] = true
		people = append(people, trimmedValue)
	}
	return people
}

func calendarDescriptionWithPeople(people []string, description string) string {
	trimmedDescription := strings.TrimSpace(description)
	if len(people) == 0 {
		return trimmedDescription
	}
	peopleLine := strings.Join(people, ", ")
	if trimmedDescription == "" {
		return peopleLine
	}
	return peopleLine + "\n" + trimmedDescription
}

func normalizeCalendarReminderLeadHours(value int) int {
	switch value {
	case 1, 2, 3, 6, 12, 24, 48:
		return value
	default:
		return calendarDefaultReminderLeadHours
	}
}

func parseCalendarRange(request *http.Request) (time.Time, time.Time, error) {
	startValue := strings.TrimSpace(request.URL.Query().Get("startISO"))
	endValue := strings.TrimSpace(request.URL.Query().Get("endISO"))
	if startValue == "" && endValue == "" {
		return time.Time{}, time.Time{}, nil
	}
	startTime, errorValue := time.Parse(time.RFC3339, startValue)
	if errorValue != nil {
		return time.Time{}, time.Time{}, errors.New("startISO must be RFC3339")
	}
	endTime, errorValue := time.Parse(time.RFC3339, endValue)
	if errorValue != nil {
		return time.Time{}, time.Time{}, errors.New("endISO must be RFC3339")
	}
	if !endTime.After(startTime) {
		return time.Time{}, time.Time{}, errors.New("endISO must be after startISO")
	}
	return startTime.UTC(), endTime.UTC(), nil
}

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

func (service *Service) syncCalendarMattermostLog(ctx context.Context, event calendarEvent) calendarEvent {
	nextEvent, errorValue := service.trySyncCalendarMattermostLog(ctx, event)
	if errorValue != nil {
		log.Printf("calendar Mattermost log sync failed: %v", errorValue)
		return event
	}
	return nextEvent
}

func (service *Service) trySyncCalendarMattermostLog(ctx context.Context, event calendarEvent) (calendarEvent, error) {
	if strings.TrimSpace(service.Configuration.MattermostAdminPasswordPath) == "" {
		return event, nil
	}
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return event, errorValue
	}
	teamRecord, errorValue := service.ensureMattermostTeam(ctx, adminToken)
	if errorValue != nil {
		return event, errorValue
	}
	channelID, errorValue := service.ensureMattermostCalendarChannel(ctx, adminToken, teamRecord.ID)
	if errorValue != nil {
		return event, errorValue
	}
	if strings.TrimSpace(event.MattermostPostID) == "" {
		return service.createCalendarMattermostLog(ctx, adminToken, channelID, event)
	}
	body := map[string]any{
		"message": service.calendarMattermostLogMessage(event),
		"props":   calendarMattermostLogProps(event),
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodPut, "/api/v4/posts/"+url.PathEscape(event.MattermostPostID)+"/patch", adminToken, body, nil); errorValue != nil {
		return event, errorValue
	}
	return event, nil
}

func (service *Service) createCalendarMattermostLog(ctx context.Context, token string, channelID string, event calendarEvent) (calendarEvent, error) {
	body := map[string]any{
		"channel_id": channelID,
		"message":    service.calendarMattermostLogMessage(event),
		"props":      calendarMattermostLogProps(event),
	}
	var response struct {
		ID string `json:"id"`
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/posts", token, body, &response); errorValue != nil {
		return event, errorValue
	}
	event.MattermostPostID = strings.TrimSpace(response.ID)
	if event.MattermostPostID == "" {
		return event, nil
	}
	return event, service.updateCalendarEventMattermostPostID(ctx, event.ID, event.MattermostPostID)
}

func (service *Service) deleteCalendarMattermostLog(ctx context.Context, event calendarEvent) {
	if errorValue := service.tryDeleteCalendarMattermostLog(ctx, event); errorValue != nil {
		log.Printf("calendar Mattermost log delete failed: %v", errorValue)
	}
}

func (service *Service) tryDeleteCalendarMattermostLog(ctx context.Context, event calendarEvent) error {
	if strings.TrimSpace(event.MattermostPostID) == "" || strings.TrimSpace(service.Configuration.MattermostAdminPasswordPath) == "" {
		return nil
	}
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return errorValue
	}
	return service.mattermostRequest(ctx, http.MethodDelete, "/api/v4/posts/"+url.PathEscape(event.MattermostPostID), adminToken, nil, nil)
}

func (service *Service) calendarMattermostLogMessage(event calendarEvent) string {
	lines := []string{fmt.Sprintf("**%s · %s**", calendarMattermostEventDateText(event), event.Title)}
	if strings.TrimSpace(event.Location) != "" {
		lines = append(lines, "장소: "+strings.TrimSpace(event.Location))
	}
	if peopleText := calendarMattermostPeopleText(event); peopleText != "" {
		lines = append(lines, "대상: "+peopleText)
	}
	if note := calendarMattermostNoteText(event.Description); note != "" {
		lines = append(lines, "메모: "+note)
	}
	lines = append(lines, service.mattermostCalendarLink(event.StartISO))
	return strings.Join(lines, "\n")
}

func calendarMattermostLogProps(event calendarEvent) map[string]any {
	return map[string]any{
		"internkim_calendar_event":    true,
		"internkim_calendar_event_id": event.ID,
	}
}

func calendarMattermostEventDateText(event calendarEvent) string {
	startTime, startError := time.Parse(time.RFC3339, strings.TrimSpace(event.StartISO))
	endTime, endError := time.Parse(time.RFC3339, strings.TrimSpace(event.EndISO))
	if startError != nil || endError != nil {
		return strings.TrimSpace(event.StartISO + " - " + event.EndISO)
	}
	location := time.Local
	if parsedLocation, errorValue := time.LoadLocation(firstNonEmpty(strings.TrimSpace(event.TimeZone), "UTC")); errorValue == nil {
		location = parsedLocation
	}
	startTime = startTime.In(location)
	endTime = endTime.In(location)
	if event.IsAllDay {
		return startTime.Format("2006-01-02")
	}
	return startTime.Format("2006-01-02 15:04") + " - " + endTime.Format("15:04")
}

func calendarMattermostPeopleText(event calendarEvent) string {
	people, hasPeopleLine := calendarPeopleFromDescription(event.Description)
	if hasPeopleLine && len(people) > 0 {
		return strings.Join(people, ", ")
	}
	return "전체"
}

func calendarMattermostNoteText(description string) string {
	lines := strings.Split(strings.TrimSpace(description), "\n")
	if len(lines) == 0 {
		return ""
	}
	_, hasPeopleLine := calendarPeopleFromDescription(description)
	if hasPeopleLine && len(lines) > 1 {
		return strings.TrimSpace(strings.Join(lines[1:], "\n"))
	}
	if hasPeopleLine {
		return ""
	}
	return strings.TrimSpace(description)
}

func (service *Service) mattermostCalendarLink(startISO string) string {
	return "[Calendar 열기](" + service.mattermostCalendarURL(startISO) + ")"
}

func (service *Service) mattermostCalendarURL(startISO string) string {
	path := "/calendar/"
	if startTime, errorValue := time.Parse(time.RFC3339, strings.TrimSpace(startISO)); errorValue == nil {
		path += "?date=" + url.QueryEscape(startTime.Format("2006-01-02"))
	}
	baseURL := strings.TrimRight(strings.TrimSpace(service.mattermostFlowBaseURL()), "/")
	if baseURL == "" {
		return path
	}
	return baseURL + path
}

func (service *Service) startCalendarNotificationWorker(ctx context.Context) {
	go func() {
		service.processDueCalendarNotifications(ctx, time.Now().UTC())
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				service.processDueCalendarNotifications(ctx, now.UTC())
			}
		}
	}()
}

func (service *Service) upsertCalendarNotifications(ctx context.Context, event calendarEvent) {
	notifyAt, shouldNotify := calendarNotificationTime(event, time.Now().UTC())
	if !shouldNotify {
		if errorValue := service.cancelCalendarNotifications(ctx, event.ID); errorValue != nil {
			log.Printf("calendar notification cancel failed: %v", errorValue)
		}
		return
	}
	targets, errorValue := service.calendarNotificationTargets(ctx, event)
	if errorValue != nil {
		log.Printf("calendar notification target resolve failed: %v", errorValue)
		return
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		log.Printf("calendar notification database open failed: %v", errorValue)
		return
	}
	defer database.Close()
	updatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	targetKeys := map[string]bool{}
	for _, target := range targets {
		targetKeys[target.Key] = true
		_, errorValue = database.ExecContext(ctx, `
INSERT INTO calendar_event_notifications (
	event_id, recipient_key, target_type, target_value, target_label, notify_at, status, sent_at, error, updated_at
) VALUES (?, ?, ?, ?, ?, ?, 'pending', '', '', ?)
ON CONFLICT(event_id, recipient_key) DO UPDATE SET
	target_type = excluded.target_type,
	target_value = excluded.target_value,
	target_label = excluded.target_label,
	notify_at = excluded.notify_at,
	status = 'pending',
	sent_at = '',
	error = '',
	updated_at = excluded.updated_at
WHERE calendar_event_notifications.status != 'sent'`,
			event.ID,
			target.Key,
			target.TargetType,
			firstNonEmpty(target.UserID, target.Key),
			target.Label,
			notifyAt.Format(time.RFC3339),
			updatedAt,
		)
		if errorValue != nil {
			log.Printf("calendar notification upsert failed: %v", errorValue)
			return
		}
	}
	service.cancelStaleCalendarNotifications(ctx, database, event.ID, targetKeys, updatedAt)
}

func calendarNotificationTime(event calendarEvent, now time.Time) (time.Time, bool) {
	startTime, errorValue := time.Parse(time.RFC3339, strings.TrimSpace(event.StartISO))
	if errorValue != nil || !startTime.After(now) {
		return time.Time{}, false
	}
	notifyAt := startTime.Add(-time.Duration(normalizeCalendarReminderLeadHours(event.ReminderLeadHours)) * time.Hour)
	if notifyAt.Before(now) {
		return now, true
	}
	return notifyAt, true
}

func (service *Service) calendarNotificationTargets(ctx context.Context, event calendarEvent) ([]calendarNotificationTarget, error) {
	people, hasPeopleLine := calendarPeopleFromDescription(event.Description)
	if !hasPeopleLine {
		return []calendarNotificationTarget{calendarAnnouncementsTarget()}, nil
	}
	users, errorValue := service.calendarMattermostUsers(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	targets := calendarTargetsForPeople(people, users)
	if len(targets) != len(people) {
		return []calendarNotificationTarget{calendarAnnouncementsTarget()}, nil
	}
	return targets, nil
}

func calendarAnnouncementsTarget() calendarNotificationTarget {
	return calendarNotificationTarget{
		TargetType: "announcements",
		Key:        "announcements:" + calendarAnnouncementsChannelName,
		Label:      calendarAnnouncementsChannelDisplayName,
	}
}

func calendarPeopleFromDescription(description string) ([]string, bool) {
	lines := strings.Split(strings.TrimSpace(description), "\n")
	if len(lines) == 0 {
		return nil, false
	}
	firstLine := strings.TrimSpace(lines[0])
	if firstLine == "" {
		return nil, false
	}
	if !strings.Contains(firstLine, ",") && strings.ContainsAny(firstLine, " \t") {
		return nil, false
	}
	people := normalizeCalendarPeople(strings.Split(firstLine, ","))
	return people, len(people) > 0
}

func (service *Service) calendarMattermostUsers(ctx context.Context) ([]mattermostUserRecord, error) {
	token, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	return service.activeMattermostUsers(ctx, token)
}

func calendarTargetsForPeople(people []string, users []mattermostUserRecord) []calendarNotificationTarget {
	targets := []calendarNotificationTarget{}
	seenUserIDs := map[string]bool{}
	for _, person := range people {
		user, found := calendarMattermostUserForPerson(person, users)
		if !found || seenUserIDs[user.ID] {
			continue
		}
		seenUserIDs[user.ID] = true
		targets = append(targets, calendarNotificationTarget{
			TargetType: "dm",
			Key:        "dm:" + user.ID,
			Label:      calendarMattermostUserDisplayName(user),
			UserID:     user.ID,
		})
	}
	return targets
}

func calendarMattermostUserForPerson(person string, users []mattermostUserRecord) (mattermostUserRecord, bool) {
	normalizedPerson := strings.ToLower(strings.TrimSpace(person))
	if normalizedPerson == "" {
		return mattermostUserRecord{}, false
	}
	for _, user := range users {
		for _, value := range calendarMattermostUserMatchValues(user) {
			if normalizedPerson == strings.ToLower(strings.TrimSpace(value)) {
				return user, true
			}
		}
	}
	return mattermostUserRecord{}, false
}

func calendarMattermostUserMatchValues(user mattermostUserRecord) []string {
	fullName := strings.TrimSpace(strings.Join([]string{user.FirstName, user.LastName}, " "))
	return uniqueNonEmpty([]string{
		user.ID,
		user.Email,
		user.Username,
		user.DisplayName,
		user.Nickname,
		user.FirstName,
		user.LastName,
		fullName,
	})
}

func calendarMattermostUserDisplayName(user mattermostUserRecord) string {
	return firstNonEmpty(user.Nickname, user.DisplayName, strings.TrimSpace(strings.Join([]string{user.FirstName, user.LastName}, " ")), user.Username, user.Email)
}

func (service *Service) cancelStaleCalendarNotifications(ctx context.Context, database *sql.DB, eventID string, targetKeys map[string]bool, updatedAt string) {
	rows, errorValue := database.QueryContext(ctx, "SELECT recipient_key FROM calendar_event_notifications WHERE event_id = ? AND status = 'pending'", eventID)
	if errorValue != nil {
		log.Printf("calendar stale notification query failed: %v", errorValue)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var recipientKey string
		if errorValue := rows.Scan(&recipientKey); errorValue != nil {
			log.Printf("calendar stale notification scan failed: %v", errorValue)
			return
		}
		if targetKeys[recipientKey] {
			continue
		}
		if _, errorValue := database.ExecContext(ctx, "UPDATE calendar_event_notifications SET status = 'canceled', updated_at = ? WHERE event_id = ? AND recipient_key = ? AND status = 'pending'", updatedAt, eventID, recipientKey); errorValue != nil {
			log.Printf("calendar stale notification cancel failed: %v", errorValue)
			return
		}
	}
	if errorValue := rows.Err(); errorValue != nil {
		log.Printf("calendar stale notification rows failed: %v", errorValue)
	}
}

func (service *Service) cancelCalendarNotifications(ctx context.Context, eventID string) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, "UPDATE calendar_event_notifications SET status = 'canceled', updated_at = ? WHERE event_id = ? AND status = 'pending'", time.Now().UTC().Format(time.RFC3339Nano), strings.TrimSpace(eventID))
	return errorValue
}

func (service *Service) processDueCalendarNotifications(ctx context.Context, now time.Time) {
	notifications, errorValue := service.readDueCalendarNotifications(ctx, now)
	if errorValue != nil {
		log.Printf("calendar notification read failed: %v", errorValue)
		return
	}
	for _, notification := range notifications {
		event, found, errorValue := service.readCalendarEventByID(ctx, notification.EventID)
		if errorValue != nil || !found {
			service.markCalendarNotificationError(ctx, notification, firstNonEmpty(errorString(errorValue), "event not found"))
			continue
		}
		if errorValue := service.sendCalendarNotification(ctx, notification, event); errorValue != nil {
			service.markCalendarNotificationError(ctx, notification, errorValue.Error())
			continue
		}
		service.markCalendarNotificationSent(ctx, notification)
	}
}

func (service *Service) readDueCalendarNotifications(ctx context.Context, now time.Time) ([]calendarNotification, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT event_id, recipient_key, target_type, target_value, target_label, notify_at, status
FROM calendar_event_notifications
WHERE status = 'pending' AND notify_at <= ?
ORDER BY notify_at`, now.UTC().Format(time.RFC3339))
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	notifications := []calendarNotification{}
	for rows.Next() {
		var notification calendarNotification
		if errorValue := rows.Scan(&notification.EventID, &notification.RecipientKey, &notification.TargetType, &notification.TargetValue, &notification.TargetLabel, &notification.NotifyAt, &notification.Status); errorValue != nil {
			return nil, errorValue
		}
		notifications = append(notifications, notification)
	}
	return notifications, rows.Err()
}

func (service *Service) sendCalendarNotification(ctx context.Context, notification calendarNotification, event calendarEvent) error {
	token, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return errorValue
	}
	switch notification.TargetType {
	case "dm":
		channelID, errorValue := service.ensureMattermostBotDirectChannelID(ctx, token, notification.TargetValue)
		if errorValue != nil {
			return errorValue
		}
		return service.postCalendarMattermostNotification(ctx, token, channelID, notification.TargetType, event)
	default:
		teamRecord, errorValue := service.ensureMattermostTeam(ctx, token)
		if errorValue != nil {
			return errorValue
		}
		channelID, errorValue := service.ensureMattermostPublicChannel(ctx, token, teamRecord.ID, calendarAnnouncementsChannelName, calendarAnnouncementsChannelDisplayName)
		if errorValue != nil {
			return errorValue
		}
		return service.postCalendarMattermostNotification(ctx, token, channelID, notification.TargetType, event)
	}
}

func (service *Service) ensureMattermostBotDirectChannelID(ctx context.Context, token string, userID string) (string, error) {
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return "", fmt.Errorf("Mattermost user ID is required")
	}
	botRecord, found, errorValue := service.findMattermostUserByUsername(ctx, token, "internkim")
	if errorValue != nil {
		return "", errorValue
	}
	if !found || botRecord.ID == "" || botRecord.DeleteAt != 0 || botRecord.ID == normalizedUserID {
		return "", fmt.Errorf("InternKim bot user is not available")
	}
	body := []string{normalizedUserID, botRecord.ID}
	var channelRecord mattermostChannelRecord
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/channels/direct", token, body, &channelRecord); errorValue != nil && !isMattermostBadRequest(errorValue) {
		return "", errorValue
	}
	if errorValue := service.showMattermostDirectChannel(ctx, token, normalizedUserID, botRecord.ID); errorValue != nil {
		return "", errorValue
	}
	if strings.TrimSpace(channelRecord.ID) != "" {
		return channelRecord.ID, nil
	}
	return "", fmt.Errorf("Mattermost direct channel was not created")
}

func (service *Service) postCalendarMattermostNotification(ctx context.Context, token string, channelID string, targetType string, event calendarEvent) error {
	body := map[string]any{
		"channel_id": channelID,
		"message":    calendarMattermostNotificationMessage(event, targetType),
		"props":      map[string]any{"internkim_calendar_notification": true, "calendar_event_id": event.ID},
	}
	return service.mattermostRequest(ctx, http.MethodPost, "/api/v4/posts", token, body, nil)
}

func calendarMattermostNotificationMessage(event calendarEvent, targetType string) string {
	lines := []string{
		"Calendar reminder",
		"**" + event.Title + "**",
		"Time: " + calendarNotificationTimeText(event),
	}
	if strings.TrimSpace(event.Location) != "" {
		lines = append(lines, "Location: "+strings.TrimSpace(event.Location))
	}
	if note := calendarNotificationNoteText(event.Description, targetType); note != "" {
		lines = append(lines, "Note: "+note)
	}
	lines = append(lines, "[Calendar 열기](/calendar/)")
	return strings.Join(lines, "\n")
}

func calendarNotificationTimeText(event calendarEvent) string {
	startTime, startError := time.Parse(time.RFC3339, event.StartISO)
	endTime, endError := time.Parse(time.RFC3339, event.EndISO)
	if startError != nil || endError != nil {
		return strings.TrimSpace(event.StartISO + " - " + event.EndISO)
	}
	location := time.UTC
	if loadedLocation, errorValue := time.LoadLocation(strings.TrimSpace(event.TimeZone)); errorValue == nil {
		location = loadedLocation
	}
	if event.IsAllDay {
		return startTime.In(location).Format("2006-01-02") + " all day"
	}
	return startTime.In(location).Format("2006-01-02 15:04") + " - " + endTime.In(location).Format("15:04 MST")
}

func calendarNotificationNoteText(description string, targetType string) string {
	lines := strings.Split(strings.TrimSpace(description), "\n")
	if len(lines) == 0 {
		return ""
	}
	if targetType == "dm" && len(lines) > 1 {
		return strings.TrimSpace(strings.Join(lines[1:], "\n"))
	}
	if targetType == "dm" {
		return ""
	}
	return strings.TrimSpace(description)
}

func (service *Service) markCalendarNotificationSent(ctx context.Context, notification calendarNotification) {
	service.updateCalendarNotificationStatus(ctx, notification, "sent", time.Now().UTC().Format(time.RFC3339Nano), "")
}

func (service *Service) markCalendarNotificationError(ctx context.Context, notification calendarNotification, message string) {
	service.updateCalendarNotificationStatus(ctx, notification, "pending", "", message)
}

func (service *Service) updateCalendarNotificationStatus(ctx context.Context, notification calendarNotification, status string, sentAt string, message string) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		log.Printf("calendar notification status open failed: %v", errorValue)
		return
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, "UPDATE calendar_event_notifications SET status = ?, sent_at = ?, error = ?, updated_at = ? WHERE event_id = ? AND recipient_key = ?", status, sentAt, strings.TrimSpace(message), time.Now().UTC().Format(time.RFC3339Nano), notification.EventID, notification.RecipientKey)
	if errorValue != nil {
		log.Printf("calendar notification status update failed: %v", errorValue)
	}
}

func errorString(errorValue error) string {
	if errorValue == nil {
		return ""
	}
	return errorValue.Error()
}

func boolToSQLiteInteger(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (service *Service) ensureCalendarICSToken(ctx context.Context) (string, error) {
	token, errorValue := service.readCalendarSetting(ctx, calendarSettingsICSKey)
	if errorValue != nil {
		return "", errorValue
	}
	if strings.TrimSpace(token) != "" {
		return token, nil
	}
	return service.writeCalendarICSToken(ctx, randomHex(32))
}

func (service *Service) writeCalendarICSToken(ctx context.Context, token string) (string, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return "", errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, "INSERT INTO calendar_settings(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", calendarSettingsICSKey, token)
	return token, errorValue
}

func (service *Service) readCalendarSetting(ctx context.Context, key string) (string, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return "", errorValue
	}
	defer database.Close()
	var value string
	errorValue = database.QueryRowContext(ctx, "SELECT value FROM calendar_settings WHERE key = ?", key).Scan(&value)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return "", nil
	}
	return value, errorValue
}

func (service *Service) isValidCalendarICSToken(ctx context.Context, token string) bool {
	expectedToken, errorValue := service.readCalendarSetting(ctx, calendarSettingsICSKey)
	return errorValue == nil && expectedToken != "" && expectedToken == token
}

func (service *Service) calendarExternalBaseURL(request *http.Request) string {
	if deviceURL := strings.TrimRight(strings.TrimSpace(readTrimmedFile(service.Configuration.DeviceURLPath)), "/"); deviceURL != "" {
		return deviceURL
	}
	scheme := firstNonEmpty(request.Header.Get("X-Forwarded-Proto"), "https")
	if isLocalRequest(request) {
		scheme = "http"
	}
	return scheme + "://" + request.Host
}

func calendarDAVSubscriptionURL(baseURL string, token string) string {
	parsedURL, errorValue := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if errorValue != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return strings.TrimRight(strings.TrimSpace(baseURL), "/") + calendarCollectionPath
	}
	parsedURL.User = url.UserPassword(calendarDAVUsername, token)
	parsedURL.Path = calendarCollectionPath
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""
	return parsedURL.String()
}

func buildCalendarFeed(events []calendarEvent) (*ical.Calendar, error) {
	calendar := newCalendarFeedDocument()
	for _, event := range events {
		eventCalendar, errorValue := calendarObjectForEvent(event)
		if errorValue != nil {
			return nil, errorValue
		}
		calendar.Children = append(calendar.Children, eventCalendar.Events()[0].Component)
	}
	return calendar, nil
}

func encodeCalendarObject(event calendarEvent) (string, error) {
	calendar, errorValue := calendarObjectForEvent(event)
	if errorValue != nil {
		return "", errorValue
	}
	var buffer bytes.Buffer
	if errorValue := ical.NewEncoder(&buffer).Encode(calendar); errorValue != nil {
		return "", errorValue
	}
	return buffer.String(), nil
}

func calendarObjectForEvent(event calendarEvent) (*ical.Calendar, error) {
	startTime, errorValue := time.Parse(time.RFC3339, event.StartISO)
	if errorValue != nil {
		return nil, errorValue
	}
	endTime, errorValue := time.Parse(time.RFC3339, event.EndISO)
	if errorValue != nil {
		return nil, errorValue
	}
	calendar := newCalendarDocument()
	calendarEventValue := ical.NewEvent()
	calendarEventValue.Props.SetText(ical.PropUID, event.UID)
	calendarEventValue.Props.SetDateTime(ical.PropDateTimeStamp, time.Now().UTC())
	calendarEventValue.Props.SetText(ical.PropSummary, event.Title)
	setOptionalCalendarText(calendarEventValue, ical.PropDescription, event.Description)
	setOptionalCalendarText(calendarEventValue, ical.PropLocation, event.Location)
	calendarEventValue.Props.SetText(ical.PropColor, event.Color)
	if event.IsAllDay {
		calendarEventValue.Props.SetDate(ical.PropDateTimeStart, startTime)
		calendarEventValue.Props.SetDate(ical.PropDateTimeEnd, endTime)
	} else {
		calendarEventValue.Props.SetDateTime(ical.PropDateTimeStart, startTime.UTC())
		calendarEventValue.Props.SetDateTime(ical.PropDateTimeEnd, endTime.UTC())
	}
	calendar.Children = append(calendar.Children, calendarEventValue.Component)
	return calendar, nil
}

func newCalendarDocument() *ical.Calendar {
	calendar := ical.NewCalendar()
	calendar.Props.SetText(ical.PropVersion, "2.0")
	calendar.Props.SetText(ical.PropProductID, calendarProductID)
	calendar.Props.SetText(ical.PropCalendarScale, "GREGORIAN")
	calendar.Props.SetText("X-WR-CALNAME", calendarName)
	calendar.Props.SetText("X-WR-TIMEZONE", "UTC")
	return calendar
}

func newCalendarFeedDocument() *ical.Calendar {
	calendar := newCalendarDocument()
	calendar.Props.SetText(ical.PropMethod, "PUBLISH")
	return calendar
}

func setOptionalCalendarText(event *ical.Event, propertyName string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	event.Props.SetText(propertyName, strings.TrimSpace(value))
}

func calendarEventFromCalendarObject(path string, calendar *ical.Calendar, actorEmail string) (calendarEvent, error) {
	eventValues := calendar.Events()
	if len(eventValues) == 0 {
		return calendarEvent{}, errors.New("VEVENT is required")
	}
	eventValue := eventValues[0]
	uid, errorValue := eventValue.Props.Text(ical.PropUID)
	if errorValue != nil || strings.TrimSpace(uid) == "" {
		return calendarEvent{}, errors.New("VEVENT UID is required")
	}
	startTime, errorValue := eventValue.DateTimeStart(time.UTC)
	if errorValue != nil {
		return calendarEvent{}, errorValue
	}
	endTime, errorValue := eventValue.DateTimeEnd(time.UTC)
	if errorValue != nil {
		return calendarEvent{}, errorValue
	}
	title, _ := eventValue.Props.Text(ical.PropSummary)
	description, _ := eventValue.Props.Text(ical.PropDescription)
	location, _ := eventValue.Props.Text(ical.PropLocation)
	color, _ := eventValue.Props.Text(ical.PropColor)
	startProperty := eventValue.Props.Get(ical.PropDateTimeStart)
	rawICS, errorValue := encodeExistingCalendar(calendar)
	if errorValue != nil {
		return calendarEvent{}, errorValue
	}
	id := strings.TrimSuffix(pathpkg.Base(path), ".ics")
	if id == "" || id == "." || id == "/" {
		id = strings.TrimSuffix(uid, "@internkim")
	}
	return calendarEvent{
		ID:                id,
		UID:               uid,
		Title:             firstNonEmpty(strings.TrimSpace(title), "Untitled event"),
		Description:       strings.TrimSpace(description),
		Location:          strings.TrimSpace(location),
		StartISO:          startTime.UTC().Format(time.RFC3339),
		EndISO:            endTime.UTC().Format(time.RFC3339),
		TimeZone:          "UTC",
		IsAllDay:          startProperty != nil && startProperty.ValueType() == ical.ValueDate,
		Color:             firstNonEmpty(strings.TrimSpace(color), "#2563eb"),
		ReminderLeadHours: calendarDefaultReminderLeadHours,
		CreatedByEmail:    actorEmail,
		RawICS:            rawICS,
	}, nil
}

func encodeExistingCalendar(calendar *ical.Calendar) (string, error) {
	var buffer bytes.Buffer
	if errorValue := ical.NewEncoder(&buffer).Encode(calendar); errorValue != nil {
		return "", errorValue
	}
	return buffer.String(), nil
}

func calendarObjectFromEvent(event calendarEvent) (caldav.CalendarObject, error) {
	rawICS := event.RawICS
	if strings.TrimSpace(rawICS) == "" {
		encodedICS, errorValue := encodeCalendarObject(event)
		if errorValue != nil {
			return caldav.CalendarObject{}, errorValue
		}
		rawICS = encodedICS
	}
	calendar, errorValue := decodeCalendarObject(rawICS)
	if errorValue != nil {
		return caldav.CalendarObject{}, errorValue
	}
	modTime, errorValue := time.Parse(time.RFC3339Nano, event.UpdatedAt)
	if errorValue != nil {
		modTime = time.Now().UTC()
	}
	return caldav.CalendarObject{
		Path:          calendarCollectionPath + event.ID + ".ics",
		ModTime:       modTime,
		ContentLength: int64(len(rawICS)),
		ETag:          calendarEventETag(event),
		Data:          calendar,
	}, nil
}

func decodeCalendarObject(rawICS string) (*ical.Calendar, error) {
	decoder := ical.NewDecoder(strings.NewReader(rawICS))
	calendar, errorValue := decoder.Decode()
	if errorValue != nil && !errors.Is(errorValue, io.EOF) {
		return nil, errorValue
	}
	return calendar, nil
}

func calendarEventETag(event calendarEvent) string {
	return event.ID + "-" + strings.ReplaceAll(event.UpdatedAt, `"`, "")
}

func checkCalendarPutPreconditions(options *caldav.PutCalendarObjectOptions, existing calendarEvent, exists bool) error {
	if options == nil {
		return nil
	}
	if options.IfNoneMatch.IsWildcard() && exists {
		return webdav.NewHTTPError(http.StatusPreconditionFailed, errors.New("calendar object already exists"))
	}
	if !options.IfMatch.IsSet() {
		return nil
	}
	if options.IfMatch.IsWildcard() {
		if !exists {
			return webdav.NewHTTPError(http.StatusPreconditionFailed, errors.New("calendar object does not exist"))
		}
		return nil
	}
	if !exists {
		return webdav.NewHTTPError(http.StatusPreconditionFailed, errors.New("calendar object does not exist"))
	}
	match, errorValue := options.IfMatch.MatchETag(calendarEventETag(existing))
	if errorValue != nil {
		return webdav.NewHTTPError(http.StatusBadRequest, errorValue)
	}
	if !match {
		return webdav.NewHTTPError(http.StatusPreconditionFailed, errors.New("etag mismatch"))
	}
	return nil
}

func calendarUIDFromObjectPath(path string) string {
	return strings.TrimSuffix(pathpkg.Base(path), ".ics")
}

func (backend calendarDAVBackend) CurrentUserPrincipal(ctx context.Context) (string, error) {
	return calendarPrincipalPath, nil
}

func (backend calendarDAVBackend) CalendarHomeSetPath(ctx context.Context) (string, error) {
	return calendarHomeSetPath, nil
}

func (backend calendarDAVBackend) CreateCalendar(ctx context.Context, calendar *caldav.Calendar) error {
	return nil
}

func (backend calendarDAVBackend) ListCalendars(ctx context.Context) ([]caldav.Calendar, error) {
	return []caldav.Calendar{calendarDAVCollection()}, nil
}

func (backend calendarDAVBackend) GetCalendar(ctx context.Context, path string) (*caldav.Calendar, error) {
	normalizedPath := strings.TrimSuffix(path, "/") + "/"
	if normalizedPath != calendarCollectionPath {
		return nil, fmt.Errorf("calendar not found")
	}
	calendar := calendarDAVCollection()
	return &calendar, nil
}

func calendarDAVCollection() caldav.Calendar {
	return caldav.Calendar{
		Path:                  calendarCollectionPath,
		Name:                  calendarName,
		Description:           "Shared Work calendar",
		MaxResourceSize:       1024 * 1024,
		SupportedComponentSet: []string{ical.CompEvent},
	}
}

func (backend calendarDAVBackend) GetCalendarObject(ctx context.Context, path string, request *caldav.CalendarCompRequest) (*caldav.CalendarObject, error) {
	uid := calendarUIDFromObjectPath(path)
	event, found, errorValue := backend.service.readCalendarEventByID(ctx, uid)
	if errorValue != nil {
		return nil, errorValue
	}
	if !found {
		return nil, fmt.Errorf("calendar object not found")
	}
	object, errorValue := calendarObjectFromEvent(event)
	if errorValue != nil {
		return nil, errorValue
	}
	return &object, nil
}

func (backend calendarDAVBackend) ListCalendarObjects(ctx context.Context, path string, request *caldav.CalendarCompRequest) ([]caldav.CalendarObject, error) {
	events, errorValue := backend.service.readCalendarEvents(ctx, time.Time{}, time.Time{})
	if errorValue != nil {
		return nil, errorValue
	}
	return calendarObjectsFromEvents(events)
}

func (backend calendarDAVBackend) QueryCalendarObjects(ctx context.Context, path string, query *caldav.CalendarQuery) ([]caldav.CalendarObject, error) {
	startTime, endTime := calendarQueryRange(query)
	events, errorValue := backend.service.readCalendarEvents(ctx, startTime, endTime)
	if errorValue != nil {
		return nil, errorValue
	}
	return calendarObjectsFromEvents(events)
}

func (backend calendarDAVBackend) PutCalendarObject(ctx context.Context, path string, calendar *ical.Calendar, options *caldav.PutCalendarObjectOptions) (*caldav.CalendarObject, error) {
	if _, _, errorValue := caldav.ValidateCalendarObject(calendar); errorValue != nil {
		return nil, errorValue
	}
	event, errorValue := calendarEventFromCalendarObject(path, calendar, "")
	if errorValue != nil {
		return nil, errorValue
	}
	existingEvent, existingFound, errorValue := backend.service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := checkCalendarPutPreconditions(options, existingEvent, existingFound); errorValue != nil {
		return nil, errorValue
	}
	if existingFound {
		event.CreatedByEmail = existingEvent.CreatedByEmail
		event.MattermostPostID = existingEvent.MattermostPostID
	}
	if errorValue := backend.service.writeCalendarEvent(ctx, event); errorValue != nil {
		return nil, errorValue
	}
	writtenEvent, found, errorValue := backend.service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil {
		return nil, errorValue
	}
	if !found {
		return nil, fmt.Errorf("calendar object was not persisted")
	}
	object, errorValue := calendarObjectFromEvent(writtenEvent)
	if errorValue != nil {
		return nil, errorValue
	}
	return &object, nil
}

func (backend calendarDAVBackend) DeleteCalendarObject(ctx context.Context, path string) error {
	return backend.service.softDeleteCalendarEvent(ctx, calendarUIDFromObjectPath(path))
}

func calendarObjectsFromEvents(events []calendarEvent) ([]caldav.CalendarObject, error) {
	objects := make([]caldav.CalendarObject, 0, len(events))
	for _, event := range events {
		object, errorValue := calendarObjectFromEvent(event)
		if errorValue != nil {
			return nil, errorValue
		}
		objects = append(objects, object)
	}
	return objects, nil
}

func calendarQueryRange(query *caldav.CalendarQuery) (time.Time, time.Time) {
	if query == nil {
		return time.Time{}, time.Time{}
	}
	return calendarCompFilterRange(query.CompFilter)
}

func calendarCompFilterRange(filter caldav.CompFilter) (time.Time, time.Time) {
	if !filter.Start.IsZero() || !filter.End.IsZero() {
		return filter.Start.UTC(), filter.End.UTC()
	}
	for _, childFilter := range filter.Comps {
		startTime, endTime := calendarCompFilterRange(childFilter)
		if !startTime.IsZero() || !endTime.IsZero() {
			return startTime, endTime
		}
	}
	return time.Time{}, time.Time{}
}

var _ caldav.Backend = calendarDAVBackend{}
var _ webdav.UserPrincipalBackend = calendarDAVBackend{}

type calendarRawProperty struct {
	XMLName  xml.Name
	InnerXML string `xml:",innerxml"`
}

type calendarPropPatchProp struct {
	Properties []calendarRawProperty `xml:",any"`
}

type calendarPropPatchSet struct {
	XMLName xml.Name              `xml:"DAV: set"`
	Prop    calendarPropPatchProp `xml:"DAV: prop"`
}

type calendarPropPatchRemove struct {
	XMLName xml.Name              `xml:"DAV: remove"`
	Prop    calendarPropPatchProp `xml:"DAV: prop"`
}

type calendarPropertyUpdate struct {
	XMLName xml.Name                  `xml:"DAV: propertyupdate"`
	Set     []calendarPropPatchSet    `xml:"DAV: set"`
	Remove  []calendarPropPatchRemove `xml:"DAV: remove"`
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

func (service *Service) handleCalendarPropPatch(responseWriter http.ResponseWriter, request *http.Request) {
	body, errorValue := io.ReadAll(request.Body)
	if errorValue != nil {
		http.Error(responseWriter, "read body failed", http.StatusBadRequest)
		return
	}
	var update calendarPropertyUpdate
	if errorValue := xml.Unmarshal(body, &update); errorValue != nil {
		http.Error(responseWriter, "invalid propertyupdate xml", http.StatusBadRequest)
		return
	}
	type propResult struct {
		name   xml.Name
		status int
	}
	results := []propResult{}
	for _, set := range update.Set {
		for _, property := range set.Prop.Properties {
			value := strings.TrimSpace(property.InnerXML)
			if errorValue := service.writeCalendarProperty(request.Context(), request.URL.Path, property.XMLName.Space, property.XMLName.Local, value); errorValue != nil {
				results = append(results, propResult{property.XMLName, http.StatusInternalServerError})
				continue
			}
			results = append(results, propResult{property.XMLName, http.StatusOK})
		}
	}
	for _, remove := range update.Remove {
		for _, property := range remove.Prop.Properties {
			_ = service.deleteCalendarProperty(request.Context(), request.URL.Path, property.XMLName.Space, property.XMLName.Local)
			results = append(results, propResult{property.XMLName, http.StatusOK})
		}
	}
	var buffer bytes.Buffer
	buffer.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	buffer.WriteString(`<D:multistatus xmlns:D="DAV:">`)
	buffer.WriteString(`<D:response><D:href>`)
	xml.EscapeText(&buffer, []byte(request.URL.Path))
	buffer.WriteString(`</D:href>`)
	for _, result := range results {
		buffer.WriteString(`<D:propstat><D:prop>`)
		writeCalendarEmptyXMLElement(&buffer, result.name)
		buffer.WriteString(`</D:prop><D:status>HTTP/1.1 `)
		buffer.WriteString(strconv.Itoa(result.status))
		buffer.WriteString(` `)
		buffer.WriteString(http.StatusText(result.status))
		buffer.WriteString(`</D:status></D:propstat>`)
	}
	buffer.WriteString(`</D:response></D:multistatus>`)
	responseWriter.Header().Set("Content-Type", "application/xml; charset=utf-8")
	responseWriter.WriteHeader(http.StatusMultiStatus)
	_, _ = responseWriter.Write(buffer.Bytes())
}

func writeCalendarEmptyXMLElement(buffer *bytes.Buffer, name xml.Name) {
	if name.Space == "" {
		buffer.WriteString(`<`)
		buffer.WriteString(name.Local)
		buffer.WriteString(`/>`)
		return
	}
	buffer.WriteString(`<X:`)
	buffer.WriteString(name.Local)
	buffer.WriteString(` xmlns:X="`)
	xml.EscapeText(buffer, []byte(name.Space))
	buffer.WriteString(`"/>`)
}

func (service *Service) handleCalendarPropFind(responseWriter http.ResponseWriter, request *http.Request) {
	recorder := httptest.NewRecorder()
	service.invokeCalendarDAVHandler(recorder, request)
	if recorder.Code != http.StatusMultiStatus {
		copyCalendarRecorderResponse(responseWriter, recorder)
		return
	}
	properties, errorValue := service.readCalendarProperties(request.Context(), calendarCollectionPath)
	if errorValue != nil || len(properties) == 0 {
		copyCalendarRecorderResponse(responseWriter, recorder)
		return
	}
	augmented, errorValue := injectCalendarPropertiesIntoMultistatus(recorder.Body.Bytes(), calendarCollectionPath, properties)
	if errorValue != nil {
		copyCalendarRecorderResponse(responseWriter, recorder)
		return
	}
	for key, values := range recorder.Header() {
		if strings.EqualFold(key, "Content-Length") {
			continue
		}
		for _, value := range values {
			responseWriter.Header().Add(key, value)
		}
	}
	responseWriter.Header().Set("Content-Length", strconv.Itoa(len(augmented)))
	responseWriter.WriteHeader(http.StatusMultiStatus)
	_, _ = responseWriter.Write(augmented)
}

func copyCalendarRecorderResponse(responseWriter http.ResponseWriter, recorder *httptest.ResponseRecorder) {
	for key, values := range recorder.Header() {
		for _, value := range values {
			responseWriter.Header().Add(key, value)
		}
	}
	responseWriter.WriteHeader(recorder.Code)
	_, _ = responseWriter.Write(recorder.Body.Bytes())
}

func injectCalendarPropertiesIntoMultistatus(body []byte, targetPath string, properties []calendarStoredProperty) ([]byte, error) {
	if len(properties) == 0 {
		return body, nil
	}
	hrefMarker := []byte("<href>" + targetPath + "</href>")
	hrefIndex := bytes.Index(body, hrefMarker)
	if hrefIndex < 0 {
		alternate := []byte("<href>" + strings.TrimSuffix(targetPath, "/") + "</href>")
		hrefIndex = bytes.Index(body, alternate)
		if hrefIndex < 0 {
			return body, nil
		}
	}
	propCloseIndex := bytes.Index(body[hrefIndex:], []byte("</prop>"))
	if propCloseIndex < 0 {
		return body, nil
	}
	insertionPoint := hrefIndex + propCloseIndex
	var injection bytes.Buffer
	for _, property := range properties {
		writeCalendarPropertyDefaultNamespace(&injection, property)
	}
	result := make([]byte, 0, len(body)+injection.Len())
	result = append(result, body[:insertionPoint]...)
	result = append(result, injection.Bytes()...)
	result = append(result, body[insertionPoint:]...)
	return result, nil
}

func writeCalendarPropertyDefaultNamespace(buffer *bytes.Buffer, property calendarStoredProperty) {
	if property.XMLName.Space == "" || property.XMLName.Space == "DAV:" {
		buffer.WriteString(`<`)
		buffer.WriteString(property.XMLName.Local)
		if property.XMLName.Space == "DAV:" {
			buffer.WriteString(` xmlns="DAV:"`)
		}
	} else {
		buffer.WriteString(`<`)
		buffer.WriteString(property.XMLName.Local)
		buffer.WriteString(` xmlns="`)
		xml.EscapeText(buffer, []byte(property.XMLName.Space))
		buffer.WriteString(`"`)
	}
	if property.Value == "" {
		buffer.WriteString(`/>`)
		return
	}
	buffer.WriteString(`>`)
	xml.EscapeText(buffer, []byte(property.Value))
	buffer.WriteString(`</`)
	buffer.WriteString(property.XMLName.Local)
	buffer.WriteString(`>`)
}
