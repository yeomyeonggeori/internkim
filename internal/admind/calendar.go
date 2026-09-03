package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const calendarDefaultReminderLeadHours = 24

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
	People            []string              `json:"people"`
	Participants      []calendarParticipant `json:"participants,omitempty"`
	ReminderLeadHours int                   `json:"reminderLeadHours"`
	CreatedByEmail    string                `json:"createdByEmail"`
	CreatedByName     string                `json:"createdByName"`
	UpdatedAt         string                `json:"updatedAt"`
}

type calendarEventWriteRequest struct {
	Title        string                        `json:"title"`
	Description  string                        `json:"description"`
	Location     string                        `json:"location"`
	StartISO     string                        `json:"startISO"`
	EndISO       string                        `json:"endISO"`
	TimeZone     string                        `json:"timeZone"`
	IsAllDay     bool                          `json:"isAllDay"`
	People       calendarPeopleInput           `json:"people"`
	Participants []calendarParticipantIdentity `json:"participants"`

	ReminderLeadHours int `json:"reminderLeadHours"`
	// The company keeps a reminder in minutes, which is what a caller that speaks
	// the row's own vocabulary sends.
	NotifyMinutesBefore int `json:"notifyMinutesBefore"`
	// Somebody asked somebody else to be somewhere. The company stamps who asked
	// from the status, so this is what it is told.
	IsRequestedOfSomebodyElse bool   `json:"isRequestedOfSomebodyElse"`
	ExpectedUpdatedAt         string `json:"expectedUpdatedAt"`
}

type calendarEventsResponse struct {
	Events []calendarEvent `json:"events"`
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
	escapedPath := strings.TrimPrefix(request.URL.EscapedPath(), "/calendar/api")
	if !service.authorizeCalendarAPIRequest(request, path) {
		http.Error(responseWriter, "calendar access required", http.StatusForbidden)
		return
	}
	switch {
	case request.Method == http.MethodGet && path == "/events":
		service.listCalendarEvents(responseWriter, request)
	case request.Method == http.MethodGet && path == "/holidays":
		service.serveCalendarHolidays(responseWriter, request)
	case request.Method == http.MethodGet && path == "/participants":
		service.listCalendarParticipants(responseWriter, request)
	case request.Method == http.MethodGet && strings.HasPrefix(escapedPath, "/events/"):
		service.getCalendarEventFromAPIPath(responseWriter, request, escapedPath)
	case request.Method == http.MethodPost && path == "/events":
		service.createCalendarEvent(responseWriter, request)
	case request.Method == http.MethodPut && strings.HasPrefix(escapedPath, "/events/"):
		service.updateCalendarEventFromAPIPath(responseWriter, request, escapedPath)
	case request.Method == http.MethodDelete && strings.HasPrefix(escapedPath, "/events/"):
		service.deleteCalendarEventFromAPIPath(responseWriter, request, escapedPath)
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) authorizeCalendarAPIRequest(request *http.Request, path string) bool {
	if path == "/participants" {
		return isLocalRequest(request) || service.authorizeWebMemberRequest(request)
	}
	return service.authorizeCalendarRequest(request)
}

func (service *Service) authorizeCalendarRequest(request *http.Request) bool {
	return isLocalRequest(request) || service.authorizeWebMemberRequest(request)
}

func (service *Service) listCalendarEvents(responseWriter http.ResponseWriter, request *http.Request) {
	startTime, endTime, errorValue := parseCalendarRange(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if startTime.IsZero() && endTime.IsZero() && request.URL.Query().Get("window") == "upcoming" {
		startTime, endTime = service.upcomingCalendarWindow(time.Now())
	}
	events, answered, readError := service.centralCalendarEvents(request, startTime, endTime)
	if !answered {
		writeCalendarBelongsToTheCompany(responseWriter)
		return
	}
	if readError != nil {
		writeCalendarCentralError(responseWriter, request, "", readError)
		return
	}
	service.writeJSON(responseWriter, calendarEventsResponse{
		Events: calendarEventsWithNormalizedParticipants(events),
	})
}

func (service *Service) createCalendarEvent(responseWriter http.ResponseWriter, request *http.Request) {
	event, payload, errorValue := service.decodeCalendarEventWriteRequest(request, "")
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	saved, answered, saveError := service.saveCentralCalendarEvent(request, event, "", "", payload.IsRequestedOfSomebodyElse)
	if !answered {
		writeCalendarBelongsToTheCompany(responseWriter)
		return
	}
	if writeCalendarCentralError(responseWriter, request, event.ID, saveError) {
		return
	}
	responseWriter.WriteHeader(http.StatusCreated)
	service.writeJSON(responseWriter, calendarEventWithNormalizedParticipants(saved))
}

func (service *Service) updateCalendarEvent(responseWriter http.ResponseWriter, request *http.Request, eventID string) {
	event, payload, errorValue := service.decodeCalendarEventWriteRequest(request, eventID)
	if errorValue != nil {
		writeCalendarErrorCode(responseWriter, http.StatusBadRequest, calendarMutationInvalidRequestErrorCode)
		return
	}
	// The company holds the event, so the device has no row to read first, and the
	// version the writer claims is whatever they sent rather than one read here.
	saved, answered, saveError := service.saveCentralCalendarEvent(
		request, event, eventID, strings.TrimSpace(payload.ExpectedUpdatedAt), payload.IsRequestedOfSomebodyElse)
	if !answered {
		writeCalendarBelongsToTheCompany(responseWriter)
		return
	}
	if writeCalendarCentralError(responseWriter, request, eventID, saveError) {
		return
	}
	service.writeJSON(responseWriter, calendarEventWithNormalizedParticipants(saved))
}

func (service *Service) deleteCalendarEvent(responseWriter http.ResponseWriter, request *http.Request, eventID string) {
	answered, removeError := service.removeCentralCalendarEvent(request, eventID)
	if !answered {
		writeCalendarBelongsToTheCompany(responseWriter)
		return
	}
	if writeCalendarCentralError(responseWriter, request, eventID, removeError) {
		return
	}
	responseWriter.WriteHeader(http.StatusNoContent)
}

func (service *Service) decodeCalendarEventWriteRequest(request *http.Request, eventID string) (calendarEvent, calendarEventWriteRequest, error) {
	var payload calendarEventWriteRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		return calendarEvent{}, calendarEventWriteRequest{}, errorValue
	}
	// A caller that speaks the company's vocabulary sends minutes; this API still
	// answers in whole hours.
	if payload.NotifyMinutesBefore > 0 && payload.ReminderLeadHours == 0 {
		payload.ReminderLeadHours = calendarReminderLeadHours(payload.NotifyMinutesBefore)
	}
	event, errorValue := service.normalizeCalendarEventWriteRequest(request, payload, eventID)
	return event, payload, errorValue
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
		id = randomHex(16)
	}
	people := normalizeCalendarPeople([]string(payload.People))
	participants := calendarParticipantsFromIdentities(payload.Participants)
	if len(participants) == 0 && len(people) > 0 {
		participants = calendarParticipantsFromPeople(people)
	}
	createdByEmail, createdByName := service.webMemberActorIdentity(request)
	_, workspaceTimeZone := service.workspaceTimeLocation()
	return calendarEvent{
		ID:                id,
		UID:               id + "@internkim",
		Title:             title,
		Description:       strings.TrimSpace(payload.Description),
		Location:          strings.TrimSpace(payload.Location),
		StartISO:          startTime.UTC().Format(time.RFC3339),
		EndISO:            endTime.UTC().Format(time.RFC3339),
		TimeZone:          firstNonEmpty(strings.TrimSpace(payload.TimeZone), workspaceTimeZone),
		IsAllDay:          payload.IsAllDay,
		People:            people,
		Participants:      participants,
		ReminderLeadHours: normalizeCalendarReminderLeadHours(payload.ReminderLeadHours),
		CreatedByEmail:    createdByEmail,
		CreatedByName:     createdByName,
	}, nil
}

func (service *Service) webMemberActorIdentity(request *http.Request) (string, string) {
	actorEmail := service.webActorEmail(request)
	if actorEmail == "" {
		return "", ""
	}
	return actorEmail, service.displayNameForEmail(request.Context(), actorEmail)
}

func (service *Service) displayNameForEmail(ctx context.Context, email string) string {
	for _, record := range service.blueclawPolicyUserRecords(ctx) {
		if strings.EqualFold(strings.TrimSpace(record.Email), email) && strings.TrimSpace(record.Name) != "" {
			return strings.TrimSpace(record.Name)
		}
	}
	return email
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
