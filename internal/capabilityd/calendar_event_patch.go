package capabilityd

func mergeCalendarEventUpdateInput(update calendarEventUpdateInput, current calendarEventForTool) calendarEventWriteInput {
	input := calendarEventWriteInput{
		EventID:             current.EventID,
		Title:               current.Title,
		Note:                current.Note,
		Location:            current.Location,
		StartsAt:            current.StartsAt,
		EndsAt:              current.EndsAt,
		IsWholeDay:          current.IsWholeDay,
		NotifyMinutesBefore: current.NotifyMinutesBefore,
	}
	if len(current.Participants) > 0 {
		input.Participants = current.Participants
	} else {
		input.People = current.People
	}
	applyCalendarEventTextPatch(update.Title, &input.Title)
	applyCalendarEventTextPatch(update.Note, &input.Note)
	applyCalendarEventTextPatch(update.Location, &input.Location)
	applyCalendarEventTextPatch(update.StartsAt, &input.StartsAt)
	applyCalendarEventTextPatch(update.EndsAt, &input.EndsAt)
	if update.IsWholeDay != nil {
		input.IsWholeDay = *update.IsWholeDay
	}
	if update.ParticipantPersonHints != nil {
		input.People = *update.ParticipantPersonHints
		input.Participants = nil
	}
	if update.NotifyMinutesBefore != nil {
		input.NotifyMinutesBefore = *update.NotifyMinutesBefore
	}
	return input
}

func applyCalendarEventTextPatch(value *string, target *string) {
	if value != nil {
		*target = *value
	}
}
