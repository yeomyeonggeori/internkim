package admind

// 캘린더 HTTP API, 페이지 정적 자산 서빙, CalDAV 라우팅, 요청 정규화, ICS 토큰 설정.

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav/caldav"
)

const (
	calendarProductID                       = "-//InternKim//Shared Calendar//EN"
	calendarName                            = "Work"
	calendarDAVUsername                     = "internkim"
	calendarPrincipalPath                   = "/calendar/dav/team/"
	calendarHomeSetPath                     = "/calendar/dav/team/calendars/"
	calendarServerXMLNamespace              = "http://calendarserver.org/ns/"
	calendarGetCTagLocalName                = "getctag"
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

type calendarPeopleInput []string

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
	// normalizeCalendarEventWriteRequest 단계에서 RawICS 가 한 번 인코딩되지만,
	// 그 직후 event.UID 가 기존 값으로 되돌려지면 RawICS 안의 UID 와 컬럼 uid 가 어긋난다.
	// CalDAV 응답은 RawICS 를, ICS 피드는 event.UID 를 쓰므로 두 출력의 UID 가 달라지면서
	// 클라이언트 캐시에서 동일 이벤트가 중복으로 보이는 회귀가 발생한다 — 여기서 재인코딩으로 정합성 회복.
	regeneratedRawICS, errorValue := encodeCalendarObject(event)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	event.RawICS = regeneratedRawICS
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
				writeCalendarSyncCollectionUnsupported(responseWriter)
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

// writeCalendarSyncCollectionUnsupported는 sync-collection REPORT 요청을 RFC 3253 supported-report
// precondition 위반으로 거절한다. valid-sync-token 응답은 "토큰이 만료됐다"는 다른 의미여서
// 클라이언트가 토큰을 버리고 무한히 재시도할 위험이 있다 — supported-report가 정확한 unsupported 신호다.
// supported-report-set PROPFIND 응답(buildCalendarCollectionPropFindResponse)에도 sync-collection 은 포함되지 않는다.
func writeCalendarSyncCollectionUnsupported(responseWriter http.ResponseWriter) {
	responseWriter.Header().Set("Content-Type", "application/xml; charset=utf-8")
	responseWriter.WriteHeader(http.StatusForbidden)
	_, _ = responseWriter.Write([]byte(
		`<?xml version="1.0" encoding="utf-8"?>` +
			`<D:error xmlns:D="DAV:"><D:supported-report/></D:error>`,
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
