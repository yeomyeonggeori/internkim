package admind

import (
	"context"
	"log"
	"strings"
	"time"
)

func calendarPeopleFromDescription(description string) ([]string, bool) {
	lines := strings.Split(strings.TrimSpace(description), "\n")
	if len(lines) == 0 {
		return nil, false
	}
	firstLine := strings.TrimSpace(lines[0])
	if firstLine == "" {
		return nil, false
	}
	if !strings.Contains(firstLine, ",") && strings.ContainsAny(firstLine, " \t") {
		return nil, false
	}
	people := normalizeCalendarPeople(strings.Split(firstLine, ","))
	return people, len(people) > 0
}

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

func calendarPeopleIncludesAll(people []string) bool {
	for _, person := range people {
		normalizedPerson := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(person, "@")))
		if normalizedPerson == "all" || normalizedPerson == "전체" {
			return true
		}
	}
	return false
}
