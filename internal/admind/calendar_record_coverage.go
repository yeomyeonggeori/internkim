package admind

import (
	"context"
	"log"
	"net/http"
	"time"
)

type calendarRecordCoverage struct {
	Events   int      `json:"events"`
	InRecord int      `json:"inRecord"`
	Missing  int      `json:"missing"`
	Examples []string `json:"examples"`
}

// A link to an event the record never took opens the day rather than the event.
// Whether that is a handful or most of the calendar decides whether it is worth
// carrying them across, so it is counted before anything is moved.
func (service *Service) handleCalendarRecordCoverage(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.authorizeInternalOrWebStaffRequest(request) {
		http.Error(responseWriter, "staff access required", http.StatusForbidden)
		return
	}
	coverage, errorValue := service.calendarCoverageOfTheRecord(request.Context())
	if errorValue != nil {
		log.Printf("calendar coverage failed: %v", errorValue)
		http.Error(responseWriter, "calendar_coverage_failed: "+errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, coverage)
}

func (service *Service) calendarCoverageOfTheRecord(ctx context.Context) (calendarRecordCoverage, error) {
	now := time.Now().UTC()
	events, errorValue := service.readCalendarEvents(ctx, now.AddDate(-2, 0, 0), now.AddDate(1, 0, 0))
	if errorValue != nil {
		return calendarRecordCoverage{}, errorValue
	}
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
	}
	return coverage, nil
}
