package admind

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

func diffCalendarEventFields(previous calendarEvent, current calendarEvent) []string {
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
