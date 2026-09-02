package admind

import (
	"context"
	"log"
	"strings"
	"time"
)

func (service *Service) linkedCalendarEventID(eventID string) string {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" || !service.linksGoToTheRecord() {
		return eventID
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	task, found, errorValue := service.readTaskByCalendarEventID(ctx, eventID)
	if errorValue != nil {
		log.Printf("the task paired with event %s is unreadable: %v", eventID, errorValue)
		return ""
	}
	if !found {
		return ""
	}
	return service.recordTaskIDCarrying(task.ID)
}
