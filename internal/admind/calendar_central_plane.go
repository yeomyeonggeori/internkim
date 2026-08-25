package admind

import (
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
