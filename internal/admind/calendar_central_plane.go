package admind

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

// The company holds the calendar, so a device reads it there and answers from
// what it read. Reading as the person who asked is what makes row level
// security the boundary: a colleague may see the company's events, and only an
// owner or an admin may change one.
func (service *Service) centralCalendarEvents(request *http.Request, startTime time.Time, endTime time.Time) ([]calendarEvent, bool) {
	requesterEmail := strings.ToLower(strings.TrimSpace(request.Header.Get("CF-Access-Authenticated-User-Email")))
	if requesterEmail == "" {
		return nil, false
	}
	client := service.centralPlane()
	if client == nil {
		return nil, false
	}
	events, errorValue := client.EventsBetween(request.Context(), "email", requesterEmail,
		calendarWindowBound(startTime), calendarWindowBound(endTime))
	if errorValue != nil {
		log.Printf("calendar stays on this device for %s: %v", requesterEmail, errorValue)
		return nil, false
	}
	return calendarEventsOfCompanyEvents(events, service.workspaceTimeZone().name), true
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
			ID:           event.CentralID,
			UID:          event.CentralID,
			Title:        event.Title,
			Description:  event.Note,
			Location:     event.Location,
			StartISO:     event.StartsAt,
			EndISO:       event.EndsAt,
			TimeZone:     timeZoneName,
			IsAllDay:     event.IsWholeDay,
			UpdatedAt:    event.UpdatedAt,
			People:       event.ParticipantMails,
			Participants: calendarParticipantsOfEmails(event.ParticipantMails),
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
func (service *Service) centralCalendarWriter(request *http.Request) (*centralplane.Client, string, bool) {
	requesterEmail := strings.ToLower(strings.TrimSpace(request.Header.Get("CF-Access-Authenticated-User-Email")))
	if requesterEmail == "" {
		return nil, "", false
	}
	client := service.centralPlane()
	if client == nil {
		return nil, "", false
	}
	return client, requesterEmail, true
}

func (service *Service) saveCentralCalendarEvent(request *http.Request, event calendarEvent, expectedUpdatedAt string) (calendarEvent, bool, error) {
	client, requesterEmail, canWrite := service.centralCalendarWriter(request)
	if !canWrite {
		return calendarEvent{}, false, nil
	}
	savedID, errorValue := client.SaveEvent(request.Context(), "email", requesterEmail, centralplane.Event{
		CentralID:         event.ID,
		Title:             event.Title,
		Note:              event.Description,
		Location:          event.Location,
		StartsAt:          event.StartISO,
		EndsAt:            event.EndISO,
		IsWholeDay:        event.IsAllDay,
		ParticipantMails:  calendarParticipantEmails(event),
		ExpectedUpdatedAt: expectedUpdatedAt,
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
	return saved, true, nil
}

func (service *Service) removeCentralCalendarEvent(request *http.Request, eventID string) (bool, error) {
	client, requesterEmail, canWrite := service.centralCalendarWriter(request)
	if !canWrite {
		return false, nil
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
