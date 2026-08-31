package admind

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

// The company holds the calendar, so a device reads it there and answers from
// what it read. Reading as the person who asked is what makes row level
// security the boundary: a colleague may see the company's events, and only an
// owner or an admin may change one.
// A company writes and reads every event as the person who asked, so a caller
// this device cannot name has no calendar to reach.
var errCalendarReaderUnnamed = errors.New("this call names nobody the company knows, and the company keeps the calendar")

// The company holds the calendar, so a read that fails is an error rather than
// a quiet fall back to the device's stale copy. Answering the old events as if
// they were current is worse than answering nothing: the next update is written
// against an event that has already moved.
func (service *Service) centralCalendarEvents(request *http.Request, startTime time.Time, endTime time.Time) ([]calendarEvent, bool, error) {
	client := service.centralPlane()
	if client == nil {
		return nil, false, nil
	}
	requesterEmail := strings.ToLower(strings.TrimSpace(request.Header.Get("CF-Access-Authenticated-User-Email")))
	if requesterEmail == "" {
		return nil, true, errCalendarReaderUnnamed
	}
	events, errorValue := client.EventsBetween(request.Context(), "email", requesterEmail,
		calendarWindowBound(startTime), calendarWindowBound(endTime))
	if errorValue != nil {
		return nil, true, errorValue
	}
	return calendarEventsOfCompanyEvents(events, service.workspaceTimeZone().name), true, nil
}

// The company holds the calendar, so reading one event by its identifier asks
// the company first. Only an event the company does not hold is read from this
// device, which is what keeps an update or a delete from failing to find an
// event the same API had just listed.
func (service *Service) centralCalendarEventByID(request *http.Request, eventID string) (calendarEvent, bool) {
	requesterEmail := strings.ToLower(strings.TrimSpace(request.Header.Get("CF-Access-Authenticated-User-Email")))
	companyEvent, held := service.companyHoldsCalendarEvent(request.Context(), requesterEmail, strings.TrimSpace(eventID))
	if !held {
		return calendarEvent{}, false
	}
	events := calendarEventsOfCompanyEvents([]centralplane.Event{companyEvent}, service.workspaceTimeZone().name)
	if len(events) != 1 {
		return calendarEvent{}, false
	}
	return events[0], true
}

func calendarWindowBound(moment time.Time) string {
	if moment.IsZero() {
		return ""
	}
	return moment.UTC().Format(time.RFC3339)
}

func calendarEventsOfCompanyEvents(events []centralplane.Event, timeZoneName string) []calendarEvent {
	converted := make([]calendarEvent, 0, len(events))
	for _, event := range events {
		converted = append(converted, calendarEvent{
			ID:                event.CentralID,
			UID:               event.CentralID,
			Title:             event.Title,
			Description:       event.Note,
			Location:          event.Location,
			StartISO:          event.StartsAt,
			EndISO:            event.EndsAt,
			TimeZone:          timeZoneName,
			IsAllDay:          event.IsWholeDay,
			ReminderLeadHours: event.NotifyMinutesBefore / 60,
			UpdatedAt:         event.UpdatedAt,
			People:            event.ParticipantMails,
			Participants:      calendarParticipantsOfEmails(event.ParticipantMails),
		})
	}
	return converted
}

func calendarParticipantsOfEmails(emails []string) []calendarParticipant {
	identities := make([]calendarParticipantIdentity, 0, len(emails))
	for _, email := range emails {
		identities = append(identities, calendarParticipantIdentity{Name: email, Email: email})
	}
	return calendarParticipantsFromIdentities(identities)
}

// The company holds the calendar, so a write goes there as the person who asked
// and the answer comes back from what the company saved.
func (service *Service) centralCalendarWriter(request *http.Request) (*centralplane.Client, string, bool, error) {
	client := service.centralPlane()
	if client == nil {
		return nil, "", false, nil
	}
	requesterEmail := strings.ToLower(strings.TrimSpace(request.Header.Get("CF-Access-Authenticated-User-Email")))
	if requesterEmail == "" {
		return nil, "", true, errCalendarReaderUnnamed
	}
	return client, requesterEmail, true, nil
}

// centralID is the company's own id for an event it already holds, and empty for
// one it does not. The device mints its own id before it knows whether the
// company will keep the event, and sending that as the company's id asks it to
// change a row nobody has.
func (service *Service) saveCentralCalendarEvent(request *http.Request, event calendarEvent, centralID string, expectedUpdatedAt string, isRequestedOfSomebodyElse bool) (calendarEvent, bool, error) {
	client, requesterEmail, answered, writerError := service.centralCalendarWriter(request)
	if !answered || writerError != nil {
		return calendarEvent{}, answered, writerError
	}
	savedID, errorValue := client.SaveEvent(request.Context(), "email", requesterEmail, centralplane.Event{
		CentralID:           centralID,
		Title:               event.Title,
		Note:                event.Description,
		Location:            event.Location,
		StartsAt:            event.StartISO,
		EndsAt:              event.EndISO,
		IsWholeDay:          event.IsAllDay,
		NotifyMinutesBefore: event.ReminderLeadHours * 60,
		Status:              requestedStatusWhenAskedOfSomebodyElse(isRequestedOfSomebodyElse),
		ParticipantMails:    calendarParticipantEmails(event),
		ExpectedUpdatedAt:   expectedUpdatedAt,
	})
	if errorValue != nil {
		if errors.Is(errorValue, centralplane.ErrEventVersionGone) {
			return calendarEvent{}, true, errCalendarEventVersionConflict
		}
		return calendarEvent{}, true, errorValue
	}
	saved := event
	saved.ID = savedID
	saved.UID = savedID
	saved.TimeZone = service.workspaceTimeZone().name
	if stored, found, readError := client.EventByID(request.Context(), "email", requesterEmail, savedID); readError == nil && found {
		saved.UpdatedAt = stored.UpdatedAt
	}
	return saved, true, nil
}

func (service *Service) removeCentralCalendarEvent(request *http.Request, eventID string) (bool, error) {
	client, requesterEmail, answered, writerError := service.centralCalendarWriter(request)
	if !answered || writerError != nil {
		return answered, writerError
	}
	return true, client.DeleteEvent(request.Context(), "email", requesterEmail, eventID)
}

func calendarParticipantEmails(event calendarEvent) []string {
	emails := make([]string, 0, len(event.Participants))
	for _, participant := range event.Participants {
		if email := strings.TrimSpace(participant.Email); email != "" {
			emails = append(emails, email)
		}
	}
	if len(emails) > 0 {
		return emails
	}
	return event.People
}

func writeCalendarCentralError(responseWriter http.ResponseWriter, request *http.Request, eventID string, errorValue error) bool {
	if errorValue == nil {
		return false
	}
	if writeCalendarEventVersionConflictError(responseWriter, errorValue) {
		return true
	}
	writeCalendarMutationInternalError(responseWriter, request, eventID, errorValue)
	return true
}

// The company holds the calendar, so an intent to delete is checked and later
// carried out there, on behalf of whoever asked for it.
func (service *Service) companyHoldsCalendarEvent(ctx context.Context, requesterEmail string, eventID string) (centralplane.Event, bool) {
	client := service.centralPlane()
	if client == nil || strings.TrimSpace(requesterEmail) == "" {
		return centralplane.Event{}, false
	}
	event, found, errorValue := client.EventByID(ctx, "email", requesterEmail, eventID)
	if errorValue != nil || !found {
		return centralplane.Event{}, false
	}
	return event, true
}

func (service *Service) removeCompanyCalendarEvent(ctx context.Context, requesterEmail string, eventID string) error {
	client := service.centralPlane()
	if client == nil || strings.TrimSpace(requesterEmail) == "" {
		return nil
	}
	return client.DeleteEvent(ctx, "email", requesterEmail, eventID)
}

// A task the company holds stands at requested until the person asked answers,
// and the company reads who asked from that. An event somebody files for
// themselves says nothing about a requester and keeps the default.
func requestedStatusWhenAskedOfSomebodyElse(isRequestedOfSomebodyElse bool) string {
	if isRequestedOfSomebodyElse {
		return "requested"
	}
	return ""
}
