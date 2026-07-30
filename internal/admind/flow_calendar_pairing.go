package admind

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"
)

const calendarPairedTaskAllDayHours = 8

func flowSizeForDurationHours(definitions flowDefinitions, durationHours float64) string {
	sizes := append([]flowSizeDefinition{}, definitions.Sizes...)
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
		return flowStatusPlanned
	}
	if end.Before(now) {
		return flowStatusCompleted
	}
	return flowStatusPlanned
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

func (service *Service) pairedFlowTaskForCalendarEvent(request *http.Request, event calendarEvent) (flowTask, error) {
	definitions, errorValue := service.readFlowDefinitions(request.Context())
	if errorValue != nil {
		return flowTask{}, errorValue
	}
	members := service.flowMembers(request)
	owner := calendarEventOwnerMember(members, event, service.flowActorEmail(request))
	startDate, endDate := calendarEventDateKeys(event)
	now := flowDateNow()
	task := flowTask{
		OwnerID:         owner.ID,
		OwnerName:       owner.Name,
		ParticipantIDs:  calendarEventParticipantMemberIDs(members, event, owner.ID),
		Business:        defaultFlowTaskBusiness(definitions),
		Type:            defaultFlowTaskType(definitions),
		Content:         strings.TrimSpace(event.Title),
		Goal:            strings.TrimSpace(event.Description),
		Size:            flowSizeForDurationHours(definitions, calendarEventDurationHours(event)),
		Status:          calendarPairedTaskStatus(event, now),
		StartDate:       startDate,
		EndDate:         endDate,
		WeekCode:        weekCodeForDate(dateOrNow(endDate, now)),
		CalendarEventID: event.ID,
	}
	return task, nil
}

func defaultFlowTaskType(definitions flowDefinitions) string {
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

func calendarEventOwnerMember(members []flowMember, event calendarEvent, actorEmail string) flowMember {
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
	return flowMember{}
}

func calendarEventParticipantMemberIDs(members []flowMember, event calendarEvent, ownerID string) []string {
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

func (service *Service) createPairedFlowTaskForCalendarEvent(request *http.Request, event calendarEvent) {
	if strings.TrimSpace(event.ID) == "" || strings.TrimSpace(event.Title) == "" {
		return
	}
	_, found, errorValue := service.readFlowTaskByCalendarEventID(request.Context(), event.ID)
	if errorValue != nil || found {
		return
	}
	task, errorValue := service.pairedFlowTaskForCalendarEvent(request, event)
	if errorValue != nil {
		return
	}
	if _, errorValue := service.writeFlowTaskAtStatusEnd(request.Context(), task); errorValue != nil {
		return
	}
}

func (service *Service) deletePairedFlowTaskForCalendarEvent(ctx context.Context, eventID string) {
	task, found, errorValue := service.readFlowTaskByCalendarEventID(ctx, eventID)
	if errorValue != nil || !found {
		return
	}
	_ = service.deleteFlowTaskByID(ctx, task.ID)
}

func (service *Service) deletePairedCalendarEventForFlowTask(ctx context.Context, task flowTask) {
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

func (service *Service) createPairedCalendarEventForFlowTask(request *http.Request, task flowTask, payload flowTaskWriteRequest) string {
	eventRequest := calendarEventWriteRequest{
		Title:       task.Content,
		Description: task.Goal,
		Location:    strings.TrimSpace(payload.EventLocation),
		StartISO:    strings.TrimSpace(payload.EventStartISO),
		EndISO:      strings.TrimSpace(payload.EventEndISO),
		IsAllDay:    payload.EventAllDay,
	}
	if eventRequest.StartISO == "" || eventRequest.EndISO == "" {
		location, _ := service.workspaceTimeLocation()
		eventRequest = allDayCalendarRequestForTaskDates(eventRequest, task, location)
	}
	eventRequest.Participants = calendarParticipantIdentitiesForMemberIDs(service.flowMembers(request), task.ParticipantIDs)
	event, errorValue := service.normalizeCalendarEventWriteRequest(request, eventRequest, "")
	if errorValue != nil {
		return ""
	}
	if errorValue := service.writeCalendarEvent(request.Context(), event); errorValue != nil {
		return ""
	}
	return event.ID
}

func calendarParticipantIdentitiesForMemberIDs(members []flowMember, memberIDs []string) []calendarParticipantIdentity {
	memberByID := map[string]flowMember{}
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

func allDayCalendarRequestForTaskDates(eventRequest calendarEventWriteRequest, task flowTask, location *time.Location) calendarEventWriteRequest {
	startDate := dateOrNow(task.StartDate, flowDateNow())
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
