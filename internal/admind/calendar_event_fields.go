package admind

import (
	"strings"
	"time"
)

const (
	calendarFieldTitle             = "title"
	calendarFieldDescription       = "description"
	calendarFieldLocation          = "location"
	calendarFieldStart             = "start"
	calendarFieldEnd               = "end"
	calendarFieldTimeZone          = "timeZone"
	calendarFieldIsAllDay          = "isAllDay"
	calendarFieldColor             = "color"
	calendarFieldParticipants      = "participants"
	calendarFieldReminderLeadHours = "reminderLeadHours"
)

func eventUpdatedWithin(updatedAt string, now time.Time, window time.Duration) bool {
	trimmed := strings.TrimSpace(updatedAt)
	if trimmed == "" {
		return false
	}
	parsed, errorValue := time.Parse(time.RFC3339Nano, trimmed)
	if errorValue != nil {
		return false
	}
	return now.Sub(parsed) < window
}

func calendarAllUserEditableFields() []string {
	return []string{
		calendarFieldTitle,
		calendarFieldDescription,
		calendarFieldLocation,
		calendarFieldStart,
		calendarFieldEnd,
		calendarFieldTimeZone,
		calendarFieldIsAllDay,
		calendarFieldColor,
		calendarFieldParticipants,
		calendarFieldReminderLeadHours,
	}
}

func diffCalendarEventFields(previous calendarEvent, current calendarEvent) []string {
	if strings.TrimSpace(previous.ID) == "" {
		return calendarAllUserEditableFields()
	}
	fields := []string{}
	if previous.Title != current.Title {
		fields = append(fields, calendarFieldTitle)
	}
	if previous.Description != current.Description {
		fields = append(fields, calendarFieldDescription)
	}
	if previous.Location != current.Location {
		fields = append(fields, calendarFieldLocation)
	}
	if previous.StartISO != current.StartISO {
		fields = append(fields, calendarFieldStart)
	}
	if previous.EndISO != current.EndISO {
		fields = append(fields, calendarFieldEnd)
	}
	if previous.TimeZone != current.TimeZone {
		fields = append(fields, calendarFieldTimeZone)
	}
	if previous.IsAllDay != current.IsAllDay {
		fields = append(fields, calendarFieldIsAllDay)
	}
	if previous.Color != current.Color {
		fields = append(fields, calendarFieldColor)
	}
	if !calendarParticipantsEqual(previous.Participants, current.Participants) {
		fields = append(fields, calendarFieldParticipants)
	}
	if previous.ReminderLeadHours != current.ReminderLeadHours {
		fields = append(fields, calendarFieldReminderLeadHours)
	}
	return fields
}

func hasCalendarEventUserEditableChanges(previous calendarEvent, current calendarEvent) bool {
	return len(diffCalendarEventFields(previous, current)) > 0
}

func mergeCalendarEventChanges(remote calendarEvent, local calendarEvent, changedFields []string) calendarEvent {
	merged := remote
	for _, field := range changedFields {
		switch field {
		case calendarFieldTitle:
			merged.Title = local.Title
		case calendarFieldDescription:
			merged.Description = local.Description
		case calendarFieldLocation:
			merged.Location = local.Location
		case calendarFieldStart:
			merged.StartISO = local.StartISO
		case calendarFieldEnd:
			merged.EndISO = local.EndISO
		case calendarFieldTimeZone:
			merged.TimeZone = local.TimeZone
		case calendarFieldIsAllDay:
			merged.IsAllDay = local.IsAllDay
		case calendarFieldColor:
			merged.Color = local.Color
		case calendarFieldParticipants:
			merged.Participants = local.Participants
		case calendarFieldReminderLeadHours:
			merged.ReminderLeadHours = local.ReminderLeadHours
		}
	}
	return merged
}

func preserveCalendarInternalParticipants(merged calendarEvent, local calendarEvent, changedFields []string) calendarEvent {
	if calendarFieldListIncludes(changedFields, calendarFieldParticipants) {
		return merged
	}
	merged.Participants = local.Participants
	return merged
}

func calendarFieldListIncludes(fields []string, target string) bool {
	for _, field := range fields {
		if field == target {
			return true
		}
	}
	return false
}

func intersectCalendarFields(left []string, right []string) []string {
	if len(left) == 0 || len(right) == 0 {
		return nil
	}
	leftSet := map[string]struct{}{}
	for _, value := range left {
		leftSet[value] = struct{}{}
	}
	result := []string{}
	for _, value := range right {
		if _, ok := leftSet[value]; ok {
			result = append(result, value)
		}
	}
	return result
}

func excludeCalendarFields(fields []string, excluded []string) []string {
	result := []string{}
	for _, field := range fields {
		if !calendarFieldListIncludes(excluded, field) {
			result = append(result, field)
		}
	}
	return result
}
