package admind

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
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav/caldav"
)

const (
	calendarProductID                = "-//InternKim//Shared Calendar//EN"
	calendarName                     = "Work"
	calendarDAVUsername              = "internkim"
	calendarPrincipalPath            = "/calendar/dav/team/"
	calendarHomeSetPath              = "/calendar/dav/team/calendars/"
	calendarServerXMLNamespace       = "http://calendarserver.org/ns/"
	calendarGetCTagLocalName         = "getctag"
	calendarCollectionPath           = "/calendar/dav/team/calendars/internkim/"
	calendarSettingsICSKey           = "ics_token"
	calendarDefaultReminderLeadHours = 24
	calendarAnnouncementsChannelName = "announcements"
)

type calendarEvent struct {
	ID                string                `json:"id"`
	UID               string                `json:"uid"`
	Title             string                `json:"title"`
	Description       string                `json:"description"`
	Location          string                `json:"location"`
	StartISO          string                `json:"startISO"`
	EndISO            string                `json:"endISO"`
	TimeZone          string                `json:"timeZone"`
	IsAllDay          bool                  `json:"isAllDay"`
	Color             string                `json:"color"`
	People            []string              `json:"people"`
	Participants      []calendarParticipant `json:"participants,omitempty"`
	ReminderLeadHours int                   `json:"reminderLeadHours"`
	CreatedByEmail    string                `json:"createdByEmail"`
	CreatedByName     string                `json:"createdByName"`
	CreatedByImage    string                `json:"createdByImage,omitempty"`
	UpdatedByEmail    string                `json:"updatedByEmail,omitempty"`
	UpdatedByName     string                `json:"updatedByName,omitempty"`
	UpdatedByImage    string                `json:"updatedByImage,omitempty"`
	UpdatedByAt       string                `json:"updatedByAt,omitempty"`
	UpdatedAt         string                `json:"updatedAt"`
	MattermostPostID  string                `json:"mattermostPostID,omitempty"`
	RemoteSource      string                `json:"remoteSource,omitempty"`
	RemoteETag        string                `json:"remoteETag,omitempty"`
	RemoteHref        string                `json:"remoteHref,omitempty"`
	RemoteModifiedAt  string                `json:"-"`
	RawICS            string                `json:"-"`
}

type calendarEventWriteRequest struct {
	EventID           string                        `json:"eventID"`
	Title             string                        `json:"title"`
	Description       string                        `json:"description"`
	Location          string                        `json:"location"`
	StartISO          string                        `json:"startISO"`
	EndISO            string                        `json:"endISO"`
	TimeZone          string                        `json:"timeZone"`
	IsAllDay          bool                          `json:"isAllDay"`
	Color             string                        `json:"color"`
	People            calendarPeopleInput           `json:"people"`
	Participants      []calendarParticipantIdentity `json:"participants"`
	ReminderLeadHours int                           `json:"reminderLeadHours"`
	AllowDuplicate    bool                          `json:"allowDuplicate"`
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

type calendarRemoteSyncResponse struct {
	Changed             bool `json:"changed"`
	PullAttempted       bool `json:"pullAttempted"`
	PullSkippedByCache  bool `json:"pullSkippedByCache"`
	SyncSkippedByLease  bool `json:"syncSkippedByLease"`
	PullCacheTTLSeconds int  `json:"pullCacheTTLSeconds"`
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
	path := strings.TrimPrefix(request.URL.Path, "/calendar/api")
	if !service.authorizeCalendarAPIRequest(request, path) {
		http.Error(responseWriter, "calendar access required", http.StatusForbidden)
		return
	}
	switch {
	case request.Method == http.MethodGet && path == "/events":
		service.listCalendarEvents(responseWriter, request)
	case request.Method == http.MethodGet && path == "/participants":
		service.listCalendarParticipants(responseWriter, request)
	case request.Method == http.MethodGet && isCalendarParticipantImageAPIPath(path):
		service.serveCalendarParticipantImage(responseWriter, request, path)
	case request.Method == http.MethodGet && isCalendarActorImageAPIPath(path):
		service.serveCalendarActorImage(responseWriter, request, path)
	case request.Method == http.MethodPost && path == "/events":
		service.createCalendarEvent(responseWriter, request)
	case request.Method == http.MethodPut && strings.HasPrefix(path, "/events/"):
		service.updateCalendarEvent(responseWriter, request, strings.TrimPrefix(path, "/events/"))
	case request.Method == http.MethodDelete && strings.HasPrefix(path, "/events/"):
		service.deleteCalendarEvent(responseWriter, request, strings.TrimPrefix(path, "/events/"))
	case request.Method == http.MethodGet && path == "/sync":
		service.writeCalendarSync(responseWriter, request)
	case request.Method == http.MethodPost && path == "/remote-sync":
		service.runCalendarRemoteSync(responseWriter, request)
	case request.Method == http.MethodGet && path == "/account-status":
		service.serveCalendarAccountStatus(responseWriter, request)
	case request.Method == http.MethodGet && path == "/google-calendars":
		service.serveGoogleCalendarList(responseWriter, request)
	case request.Method == http.MethodPost && path == "/google-calendars/selection":
		service.selectGoogleCalendar(responseWriter, request)
	case request.Method == http.MethodPost && path == "/google-oauth-client":
		service.uploadGoogleOAuthClient(responseWriter, request)
	case request.Method == http.MethodGet && path == "/conflicts":
		service.serveCalendarConflicts(responseWriter, request)
	case request.Method == http.MethodPost && strings.HasPrefix(path, "/conflicts/") && strings.HasSuffix(path, "/dismiss"):
		service.dismissCalendarConflictRequest(responseWriter, request, path)
	case request.Method == http.MethodPost && path == "/ics-token":
		service.rotateCalendarICSToken(responseWriter, request)
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) authorizeCalendarAPIRequest(request *http.Request, path string) bool {
	if path == "/participants" {
		return isLocalRequest(request) || service.authorizeWebStaffRequest(request)
	}
	if isCalendarParticipantImageAPIPath(path) {
		return isLocalRequest(request) || service.authorizeWebStaffRequest(request)
	}
	if isCalendarActorImageAPIPath(path) {
		return isLocalRequest(request) || service.authorizeWebStaffRequest(request)
	}
	return service.authorizeCalendarRequest(request)
}

func isCalendarParticipantImageAPIPath(path string) bool {
	return strings.HasPrefix(path, "/participants/") && strings.HasSuffix(path, "/image")
}

func isCalendarActorImageAPIPath(path string) bool {
	return strings.HasPrefix(path, "/events/") && strings.HasSuffix(path, "/actor-image")
}

func (service *Service) authorizeCalendarRequest(request *http.Request) bool {
	if isLocalRequest(request) {
		return true
	}
	if service.authorizeCalendarTokenRequest(request) {
		return true
	}
	return service.authorizeWebStaffRequest(request)
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
	query := request.URL.Query()
	hasExplicitRange := strings.TrimSpace(query.Get("startISO")) != "" && strings.TrimSpace(query.Get("endISO")) != ""
	startTime, endTime, errorValue := parseCalendarRange(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if startTime.IsZero() && endTime.IsZero() && query.Get("window") == "upcoming" {
		startTime, endTime = service.upcomingCalendarWindow(time.Now())
	}
	var events []calendarEvent
	if hasExplicitRange {
		events, errorValue = service.readCalendarEventWindow(request.Context(), startTime, endTime)
	} else {
		events, errorValue = service.readCalendarEvents(request.Context(), startTime, endTime)
	}
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	events = service.calendarEventsWithParticipantImages(request, events)
	service.writeJSON(responseWriter, calendarEventsResponse{Events: service.calendarEventsWithActorProfiles(request.Context(), events)})
}

func (service *Service) createCalendarEvent(responseWriter http.ResponseWriter, request *http.Request) {
	event, allowDuplicate, errorValue := service.decodeCalendarEventWriteRequest(request, "")
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if !allowDuplicate {
		if candidates, errorValue := service.findDuplicateCalendarCandidates(request.Context(), event); errorValue == nil && len(candidates) > 0 {
			responseWriter.WriteHeader(http.StatusOK)
			service.writeJSON(responseWriter, map[string]any{
				"status":     "duplicate_candidate",
				"candidates": service.calendarEventsWithParticipantImages(request, candidates),
			})
			return
		}
	}
	if errorValue := service.writeCalendarEvent(request.Context(), event); errorValue != nil {
		if writeCalendarTargetUnavailableError(responseWriter, errorValue) {
			return
		}
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	responseWriter.WriteHeader(http.StatusCreated)
	event = service.calendarEventWithParticipantImages(request, event)
	service.writeJSON(responseWriter, service.calendarEventWithActorProfiles(request.Context(), event))
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
	event, _, errorValue := service.decodeCalendarEventWriteRequest(request, existingEvent.ID)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if !hasCalendarEventUserEditableChanges(existingEvent, event) {
		existingEvent = service.calendarEventWithParticipantImages(request, existingEvent)
		service.writeJSON(responseWriter, service.calendarEventWithActorProfiles(request.Context(), existingEvent))
		return
	}
	event.UID = existingEvent.UID
	event.CreatedByEmail = existingEvent.CreatedByEmail
	event.CreatedByName = existingEvent.CreatedByName
	event.UpdatedByEmail, event.UpdatedByName = service.webStaffActorIdentity(request)
	event.UpdatedByAt = time.Now().UTC().Format(time.RFC3339Nano)
	event.MattermostPostID = existingEvent.MattermostPostID
	event.RemoteSource = existingEvent.RemoteSource
	event.RemoteETag = existingEvent.RemoteETag
	event.RemoteHref = existingEvent.RemoteHref
	regeneratedRawICS, errorValue := encodeCalendarObject(event)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	event.RawICS = regeneratedRawICS
	if errorValue := service.writeCalendarEvent(request.Context(), event); errorValue != nil {
		if writeCalendarTargetUnavailableError(responseWriter, errorValue) {
			return
		}
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	event = service.calendarEventWithParticipantImages(request, event)
	service.writeJSON(responseWriter, service.calendarEventWithActorProfiles(request.Context(), event))
}

func (service *Service) deleteCalendarEvent(responseWriter http.ResponseWriter, request *http.Request, eventID string) {
	if errorValue := service.softDeleteCalendarEvent(request.Context(), eventID); errorValue != nil {
		if errors.Is(errorValue, sql.ErrNoRows) {
			http.NotFound(responseWriter, request)
			return
		}
		if writeCalendarTargetUnavailableError(responseWriter, errorValue) {
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

func (service *Service) runCalendarRemoteSync(responseWriter http.ResponseWriter, request *http.Request) {
	syncStartedAt := time.Now().UTC()
	decision, errorValue := service.acquireCalendarRemoteSync(request.Context(), syncStartedAt)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if !decision.Acquired {
		service.writeJSON(responseWriter, calendarRemoteSyncResponse{
			PullSkippedByCache:  decision.SkippedByCache,
			SyncSkippedByLease:  decision.SkippedByLease,
			PullCacheTTLSeconds: int(calendarRemoteSyncSuccessCacheDuration.Seconds()),
		})
		return
	}
	result := service.runCalendarUserSyncCycle(request.Context())
	if errorValue := service.finishCalendarRemoteSync(request.Context(), decision, time.Now().UTC(), result.Succeeded()); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, calendarRemoteSyncResponse{
		Changed:             result.Changed,
		PullAttempted:       result.PullAttempted,
		PullSkippedByCache:  result.PullSkippedByCache,
		PullCacheTTLSeconds: int(calendarRemoteSyncSuccessCacheDuration.Seconds()),
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
		body, errorValue := io.ReadAll(request.Body)
		if errorValue == nil {
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

func writeCalendarSyncCollectionUnsupported(responseWriter http.ResponseWriter) {
	responseWriter.Header().Set("Content-Type", "application/xml; charset=utf-8")
	responseWriter.WriteHeader(http.StatusForbidden)
	_, _ = responseWriter.Write([]byte(
		`<?xml version="1.0" encoding="utf-8"?>` +
			`<D:error xmlns:D="DAV:"><D:supported-report/></D:error>`,
	))
}

func (service *Service) findDuplicateCalendarCandidates(ctx context.Context, event calendarEvent) ([]calendarEvent, error) {
	newStart, errorValue := time.Parse(time.RFC3339, event.StartISO)
	if errorValue != nil {
		return nil, errorValue
	}
	existingEvents, errorValue := service.readCalendarEvents(ctx, newStart.Add(-time.Second), newStart.Add(time.Second))
	if errorValue != nil {
		return nil, errorValue
	}
	newPeopleKey := calendarPeopleSetKey(event.People)
	candidates := []calendarEvent{}
	for _, existingEvent := range existingEvents {
		existingStart, parseError := time.Parse(time.RFC3339, existingEvent.StartISO)
		if parseError != nil || !existingStart.Equal(newStart) {
			continue
		}
		if calendarPeopleSetKey(existingEvent.People) != newPeopleKey {
			continue
		}
		candidates = append(candidates, existingEvent)
	}
	return candidates, nil
}

func calendarPeopleSetKey(people []string) string {
	normalizedPeople := []string{}
	seenPerson := map[string]bool{}
	for _, person := range people {
		normalizedPerson := strings.ToLower(strings.TrimSpace(person))
		if normalizedPerson == "" || seenPerson[normalizedPerson] {
			continue
		}
		seenPerson[normalizedPerson] = true
		normalizedPeople = append(normalizedPeople, normalizedPerson)
	}
	sort.Strings(normalizedPeople)
	return strings.Join(normalizedPeople, "\x00")
}

func (service *Service) decodeCalendarEventWriteRequest(request *http.Request, eventID string) (calendarEvent, bool, error) {
	var payload calendarEventWriteRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		return calendarEvent{}, false, errorValue
	}
	event, errorValue := service.normalizeCalendarEventWriteRequest(request, payload, eventID)
	return event, payload.AllowDuplicate, errorValue
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
	participants := calendarParticipantsFromIdentities(payload.Participants)
	if len(participants) == 0 && len(people) > 0 {
		participants = calendarParticipantsFromPeople(people)
	}
	createdByEmail, createdByName := service.webStaffActorIdentity(request)
	_, workspaceTimeZone := service.workspaceTimeLocation()
	event := calendarEvent{
		ID:                id,
		UID:               id + "@internkim",
		Title:             title,
		Description:       strings.TrimSpace(payload.Description),
		Location:          strings.TrimSpace(payload.Location),
		StartISO:          startTime.UTC().Format(time.RFC3339),
		EndISO:            endTime.UTC().Format(time.RFC3339),
		TimeZone:          firstNonEmpty(strings.TrimSpace(payload.TimeZone), workspaceTimeZone),
		IsAllDay:          payload.IsAllDay,
		Color:             firstNonEmpty(strings.TrimSpace(payload.Color), "#2563eb"),
		People:            people,
		Participants:      participants,
		ReminderLeadHours: normalizeCalendarReminderLeadHours(payload.ReminderLeadHours),
		CreatedByEmail:    createdByEmail,
		CreatedByName:     createdByName,
	}
	rawICS, errorValue := encodeCalendarObject(event)
	if errorValue != nil {
		return calendarEvent{}, errorValue
	}
	event.RawICS = rawICS
	return event, nil
}

func (service *Service) webStaffActorIdentity(request *http.Request) (string, string) {
	if actorEmail := authenticatedCallerEmail(request); actorEmail != "" {
		return actorEmail, actorEmail
	}
	cookieHeader := mattermostSessionCookieHeader(request)
	if cookieHeader == "" {
		return "", ""
	}
	userRecord, ok := service.mattermostSessionUser(request, cookieHeader)
	if !ok {
		return "", ""
	}
	actorEmail := strings.ToLower(strings.TrimSpace(userRecord.Email))
	return actorEmail, firstNonEmpty(mattermostDisplayName(userRecord), actorEmail)
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

func normalizeCalendarReminderLeadHours(value int) int {
	switch value {
	case 1, 2, 3, 6, 12, 24, 48:
		return value
	default:
		return calendarDefaultReminderLeadHours
	}
}

func (service *Service) upcomingCalendarWindow(now time.Time) (time.Time, time.Time) {
	location, _ := service.workspaceTimeLocation()
	local := now.In(location)
	startOfToday := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	return startOfToday.UTC(), startOfToday.AddDate(0, 0, 7).UTC()
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
