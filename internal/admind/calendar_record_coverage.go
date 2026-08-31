package admind

import (
	"log"
	"net/http"
	"time"
)

type calendarRecordCoverage struct {
	Events   int      `json:"events"`
	InRecord int      `json:"inRecord"`
	Missing  int      `json:"missing"`
	Paired   int      `json:"paired"`
	Examples []string `json:"examples"`
}

// A link to an event the record never took opens the day rather than the event.
// Whether that is a handful or most of the calendar decides whether it is worth
// carrying them across, so it is counted before anything is moved.
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

// An event reaches the record through the task it is paired with, and the ones
// made before that pairing existed have none. Pairing them is what puts them
// where a link can name them; it also puts each on the board, which is what the
// count is for reading before the pairing runs.
func (service *Service) calendarCoverageOfTheRecord(request *http.Request) (calendarRecordCoverage, error) {
	ctx := request.Context()
	now := time.Now().UTC()
	events, errorValue := service.readCalendarEvents(ctx, now.AddDate(-2, 0, 0), now.AddDate(1, 0, 0))
	if errorValue != nil {
		return calendarRecordCoverage{}, errorValue
	}
	shouldPair := request.URL.Query().Get("pair") == "true"
	coverage := calendarRecordCoverage{Events: len(events), Examples: []string{}}
	for _, event := range events {
		if service.linkedCalendarEventID(event.ID) != "" {
			coverage.InRecord++
			continue
		}
		coverage.Missing++
		if len(coverage.Examples) < 5 {
			coverage.Examples = append(coverage.Examples, event.StartISO+" "+event.Title)
		}
		if shouldPair && service.pairedEventIntoTheRecord(request, event) {
			coverage.Paired++
		}
	}
	return coverage, nil
}

func (service *Service) pairedEventIntoTheRecord(request *http.Request, event calendarEvent) bool {
	if _, found, errorValue := service.readTaskByCalendarEventID(request.Context(), event.ID); errorValue != nil || found {
		return false
	}
	return service.createPairedTaskForCalendarEvent(request, event)
}
