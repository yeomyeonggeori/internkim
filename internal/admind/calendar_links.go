package admind

import (
	"strings"
)

func (service *Service) linkedCalendarEventID(eventID string) string {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" || !service.linksGoToTheRecord() {
		return eventID
	}
	return service.recordTaskIDCarrying(eventID)
}
