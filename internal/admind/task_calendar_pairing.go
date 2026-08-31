package admind

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"
)

const calendarPairedTaskAllDayHours = 8

func taskSizeForDurationHours(definitions taskDefinitions, durationHours float64) string {
	sizes := append([]taskSizeDefinition{}, definitions.Sizes...)
	if len(sizes) == 0 {
		return ""
	}
	sort.SliceStable(sizes, func(left, right int) bool { return sizes[left].MaxHours < sizes[right].MaxHours })
	for _, size := range sizes {
		if durationHours <= float64(size.MaxHours) {
			return size.Name
		}
	}
	return sizes[len(sizes)-1].Name
}

func calendarEventDurationHours(event calendarEvent) float64 {
	start, startError := time.Parse(time.RFC3339, event.StartISO)
	end, endError := time.Parse(time.RFC3339, event.EndISO)
	if startError != nil || endError != nil {
		return calendarPairedTaskAllDayHours
	}
	if event.IsAllDay {
		days := end.Sub(start).Hours() / 24
		if days < 1 {
			days = 1
		}
		return days * calendarPairedTaskAllDayHours
	}
	hours := end.Sub(start).Hours()
	if hours <= 0 {
		return calendarPairedTaskAllDayHours
	}
	return hours
}

func calendarPairedTaskStatus(event calendarEvent, now time.Time) string {
	end, errorValue := time.Parse(time.RFC3339, event.EndISO)
	if errorValue != nil {
		return taskStatusPlanned
	}
	if end.Before(now) {
		return taskStatusCompleted
	}
	return taskStatusPlanned
}

func calendarEventDateKeys(event calendarEvent) (string, string) {
	start, startError := time.Parse(time.RFC3339, event.StartISO)
	end, endError := time.Parse(time.RFC3339, event.EndISO)
	if startError != nil {
		return "", ""
	}
	startKey := start.Format("2006-01-02")
	if endError != nil {
		return startKey, startKey
	}
	if event.IsAllDay {
		end = end.Add(-time.Second)
	}
	endKey := end.Format("2006-01-02")
	if endKey < startKey {
		endKey = startKey
	}
	return startKey, endKey
}

func (service *Service) pairedTaskForCalendarEvent(request *http.Request, event calendarEvent) (Task, error) {
	definitions, errorValue := service.readTaskDefinitions(request.Context())
	if errorValue != nil {
		return Task{}, errorValue
	}
	members := service.taskMembers(request)
	owner := calendarEventOwnerMember(members, event, service.taskActorEmail(request))
	startDate, endDate := calendarEventDateKeys(event)
	now := taskDateNow()
	task := Task{
		OwnerID:         owner.ID,
		OwnerName:       owner.Name,
		ParticipantIDs:  calendarEventParticipantMemberIDs(members, event, owner.ID),
		Business:        defaultTaskBusiness(definitions),
		Type:            defaultTaskType(definitions),
		Content:         strings.TrimSpace(event.Title),
		Size:            taskSizeForDurationHours(definitions, calendarEventDurationHours(event)),
		Status:          calendarPairedTaskStatus(event, now),
		StartDate:       startDate,
		EndDate:         endDate,
		WeekCode:        weekCodeForDate(dateOrNow(endDate, now)),
		CalendarEventID: event.ID,
	}
	return task, nil
}

func defaultTaskType(definitions taskDefinitions) string {
	if len(definitions.Types) == 0 {
		return ""
	}
	return strings.TrimSpace(definitions.Types[0])
}

func dateOrNow(dateKey string, now time.Time) time.Time {
	parsed, errorValue := time.Parse("2006-01-02", strings.TrimSpace(dateKey))
	if errorValue != nil {
		return now
	}
	return parsed
}

func calendarEventOwnerMember(members []taskMember, event calendarEvent, actorEmail string) taskMember {
	for _, candidateEmail := range []string{event.CreatedByEmail, actorEmail} {
		normalizedEmail := strings.ToLower(strings.TrimSpace(candidateEmail))
		if normalizedEmail == "" {
			continue
		}
		for _, member := range members {
			if member.Email == normalizedEmail {
				return member
			}
		}
	}
	if len(members) > 0 {
		return members[0]
	}
	return taskMember{}
}

func calendarEventParticipantMemberIDs(members []taskMember, event calendarEvent, ownerID string) []string {
	memberIDByEmail := map[string]string{}
	for _, member := range members {
		memberIDByEmail[member.Email] = member.ID
	}
	participantIDs := []string{}
	if ownerID != "" {
		participantIDs = append(participantIDs, ownerID)
	}
	for _, participant := range event.Participants {
		memberID, found := memberIDByEmail[strings.ToLower(strings.TrimSpace(participant.Email))]
		if !found || memberID == ownerID {
			continue
		}
		participantIDs = append(participantIDs, memberID)
	}
	return participantIDs
}

// The company holds an event as the one task row it is (is_event), and its
// board reads exclude events, so an event there has no paired task — a local
// one would be invisible to a company device's board reads.
func (service *Service) createPairedTaskForCalendarEvent(request *http.Request, event calendarEvent) bool {
	if service.centralPlane() != nil {
		return false
	}
	if strings.TrimSpace(event.ID) == "" || strings.TrimSpace(event.Title) == "" {
		return false
	}
	_, found, errorValue := service.readTaskByCalendarEventID(request.Context(), event.ID)
	if errorValue != nil || found {
		return false
	}
	task, errorValue := service.pairedTaskForCalendarEvent(request, event)
	if errorValue != nil {
		return false
	}
	if _, errorValue := service.writeTaskAtStatusEnd(request.Context(), task); errorValue != nil {
		return false
	}
	return true
}

func (service *Service) deletePairedTaskForCalendarEvent(ctx context.Context, eventID string) {
	if service.centralPlane() != nil {
		return
	}
	task, found, errorValue := service.readTaskByCalendarEventID(ctx, eventID)
	if errorValue != nil || !found {
		return
	}
	_ = service.deleteTaskByID(ctx, task.ID)
}

func (service *Service) deletePairedCalendarEventForTask(ctx context.Context, task Task) {
	if service.centralPlane() != nil {
		return
	}
	eventID := strings.TrimSpace(task.CalendarEventID)
	if eventID == "" {
		return
	}
	event, found, errorValue := service.readCalendarEventByID(ctx, eventID)
	if errorValue != nil || !found {
		return
	}
	_ = service.softDeleteCalendarEventIfCurrentVersion(ctx, eventID, event.UpdatedAt)
}

func (service *Service) createPairedCalendarEventForTask(request *http.Request, task Task, payload taskWriteRequest) string {
	if service.centralPlane() != nil {
		return ""
	}
	eventRequest := calendarEventWriteRequest{
		Title:       task.Content,
		Description: "",
		Location:    strings.TrimSpace(payload.EventLocation),
		StartISO:    strings.TrimSpace(payload.EventStartISO),
		EndISO:      strings.TrimSpace(payload.EventEndISO),
		IsAllDay:    payload.EventAllDay,
	}
	if eventRequest.StartISO == "" || eventRequest.EndISO == "" {
		location, _ := service.workspaceTimeLocation()
		eventRequest = allDayCalendarRequestForTaskDates(eventRequest, task, location)
	}
	eventRequest.Participants = calendarParticipantIdentitiesForMemberIDs(service.taskMembers(request), task.ParticipantIDs)
	event, errorValue := service.normalizeCalendarEventWriteRequest(request, eventRequest, "")
	if errorValue != nil {
		return ""
	}
	if errorValue := service.writeCalendarEvent(request.Context(), event); errorValue != nil {
		return ""
	}
	return event.ID
}

func calendarParticipantIdentitiesForMemberIDs(members []taskMember, memberIDs []string) []calendarParticipantIdentity {
	memberByID := map[string]taskMember{}
	for _, member := range members {
		memberByID[member.ID] = member
	}
	identities := []calendarParticipantIdentity{}
	for _, memberID := range memberIDs {
		member, found := memberByID[memberID]
		if !found {
			continue
		}
		identities = append(identities, calendarParticipantIdentity{PersonID: member.ID, Name: member.Name, Email: member.Email})
	}
	return identities
}

func allDayCalendarRequestForTaskDates(eventRequest calendarEventWriteRequest, task Task, location *time.Location) calendarEventWriteRequest {
	startDate := dateOrNow(task.StartDate, taskDateNow())
	endDate := dateOrNow(firstNonEmpty(task.EndDate, task.StartDate), startDate)
	if endDate.Before(startDate) {
		endDate = startDate
	}
	start := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, location)
	end := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 0, 0, 0, 0, location).AddDate(0, 0, 1)
	eventRequest.StartISO = start.Format(time.RFC3339)
	eventRequest.EndISO = end.Format(time.RFC3339)
	eventRequest.IsAllDay = true
	return eventRequest
}
