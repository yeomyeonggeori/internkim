package capabilityd

func mergeCalendarEventUpdateInput(update calendarEventUpdateInput, current calendarEventForTool) calendarEventWriteInput {
	input := calendarEventWriteInput{
		EventID:           current.EventID,
		Title:             current.Title,
		Description:       current.Description,
		Location:          current.Location,
		StartISO:          current.StartISO,
		EndISO:            current.EndISO,
		TimeZone:          current.TimeZone,
		IsAllDay:          current.IsAllDay,
		Color:             current.Color,
		ReminderLeadHours: current.ReminderLeadHours,
	}
	if len(current.Participants) > 0 {
		input.Participants = current.Participants
	} else {
		input.People = current.People
	}
	applyCalendarEventTextPatch(update.Title, &input.Title)
	applyCalendarEventTextPatch(update.Description, &input.Description)
	applyCalendarEventTextPatch(update.Location, &input.Location)
	applyCalendarEventTextPatch(update.StartISO, &input.StartISO)
	applyCalendarEventTextPatch(update.EndISO, &input.EndISO)
	applyCalendarEventTextPatch(update.TimeZone, &input.TimeZone)
	applyCalendarEventTextPatch(update.Color, &input.Color)
	if update.IsAllDay != nil {
		input.IsAllDay = *update.IsAllDay
	}
	if update.People != nil {
		input.People = *update.People
		input.Participants = nil
	}
	if update.ReminderLeadHours != nil {
		input.ReminderLeadHours = *update.ReminderLeadHours
	}
	if update.IncludeRequester != nil {
		input.IncludeRequester = update.IncludeRequester
	}
	return input
}

func applyCalendarEventTextPatch(value *string, target *string) {
	if value != nil {
		*target = *value
	}
}
