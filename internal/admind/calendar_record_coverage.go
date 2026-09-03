package admind

import (
	"context"
	"log"
	"net/http"
)

const calendarCarryRecoveryAction = "calendar-carry-into-the-record"

type calendarRecordCoverage struct {
	Events   int      `json:"events"`
	InRecord int      `json:"inRecord"`
	Missing  int      `json:"missing"`
	Carried  int      `json:"carried"`
	Refused  []string `json:"refused"`
	Examples []string `json:"examples"`
}

func (service *Service) handleCalendarRecordCoverage(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.authorizeInternalOrWebMemberRequest(request) {
		http.Error(responseWriter, "member access required", http.StatusForbidden)
		return
	}
	coverage, errorValue := service.calendarCoverageOfTheRecord(request)
	if errorValue != nil {
		log.Printf("calendar coverage failed: %v", errorValue)
		http.Error(responseWriter, "calendar_coverage_failed: "+errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, coverage)
}

func (service *Service) calendarCoverageOfTheRecord(request *http.Request) (calendarRecordCoverage, error) {
	events, errorValue := service.localCalendarEvents(request.Context())
	if errorValue != nil {
		return calendarRecordCoverage{}, errorValue
	}
	carried, errorValue := service.eventsAlreadyCarriedIntoTheRecord(request.Context())
	if errorValue != nil {
		return calendarRecordCoverage{}, errorValue
	}
	shouldCarry := request.URL.Query().Get("carry") == "true"
	coverage := calendarRecordCoverage{Events: len(events), Refused: []string{}, Examples: []string{}}
	for _, event := range events {
		if service.recordHoldsCalendarEvent(event.ID, carried) {
			coverage.InRecord++
			continue
		}
		coverage.Missing++
		if len(coverage.Examples) < 5 {
			coverage.Examples = append(coverage.Examples, calendarEventDescribedForCarry(event))
		}
		if !shouldCarry {
			continue
		}
		if service.carriedEventIntoTheRecord(request, event) {
			coverage.Carried++
			continue
		}
		coverage.Refused = append(coverage.Refused, calendarEventDescribedForCarry(event))
	}
	return coverage, nil
}

// The record refuses a row the past violated, and the row is reported as it was
// recorded rather than reshaped into something the device never held.
func (service *Service) carriedEventIntoTheRecord(request *http.Request, event calendarEvent) bool {
	saved, answered, errorValue := service.saveCentralCalendarEvent(request, event, "", "", false)
	if !answered {
		log.Printf("the calendar event %s stays on this device: it names no company to carry it to", event.ID)
		return false
	}
	if errorValue != nil {
		log.Printf("the record refused the calendar event %s as it happened: %v", event.ID, errorValue)
		return false
	}
	if errorValue := service.rememberEventCarriedIntoTheRecord(request.Context(), event.ID, saved.ID); errorValue != nil {
		log.Printf("the record took the calendar event %s as %s, but this device could not remember the link: %v",
			event.ID, saved.ID, errorValue)
		return false
	}
	return true
}

func calendarEventDescribedForCarry(event calendarEvent) string {
	return event.StartISO + " " + event.Title
}

func (service *Service) recordHoldsCalendarEvent(eventID string, carried map[string]string) bool {
	if carried[eventID] != "" {
		return true
	}
	return service.linkedCalendarEventID(eventID) != ""
}

// An event is held by the record only once the record has been asked and has
// answered. A device that names no company has asked nobody, so everything it
// holds is still only its own.
func (service *Service) calendarEventsTheRecordDoesNotHold(ctx context.Context) (int, error) {
	events, errorValue := service.localCalendarEvents(ctx)
	if errorValue != nil {
		return 0, errorValue
	}
	if len(events) == 0 {
		return 0, nil
	}
	if !service.belongsToACompany() {
		return len(events), nil
	}
	carried, errorValue := service.eventsAlreadyCarriedIntoTheRecord(ctx)
	if errorValue != nil {
		return 0, errorValue
	}
	uncovered := 0
	for _, event := range events {
		if !service.recordHoldsCalendarEvent(event.ID, carried) {
			uncovered++
		}
	}
	return uncovered, nil
}
