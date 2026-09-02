package admind

import (
	"reflect"
	"testing"
)

func editableCalendarEventForFieldDiff() calendarEvent {
	return calendarEvent{
		ID:                "field-diff",
		UID:               "field-diff@internkim",
		Title:             "Design review",
		Description:       "Bring the deck",
		Location:          "Studio",
		StartISO:          "2026-06-15T10:00:00Z",
		EndISO:            "2026-06-15T11:00:00Z",
		TimeZone:          "UTC",
		IsAllDay:          false,
		Color:             "#2563eb",
		ReminderLeadHours: 24,
		Participants:      []calendarParticipant{{PersonID: "person-1", Name: "이샘플"}},
	}
}

func TestDiffCalendarEventFieldsReturnsOnlyChanged(t *testing.T) {
	previous := editableCalendarEventForFieldDiff()
	current := previous
	current.Title = "Design review II"
	current.StartISO = "2026-06-15T10:30:00Z"

	changed := diffCalendarEventFields(previous, current)

	if !reflect.DeepEqual(changed, []string{calendarFieldTitle, calendarFieldStart}) {
		t.Fatalf("changed fields = %v, want title and start", changed)
	}
}

func TestDiffCalendarEventFieldsSeesEveryEditableFieldOnItsOwn(t *testing.T) {
	testCases := []struct {
		field string
		edit  func(*calendarEvent)
	}{
		{calendarFieldTitle, func(event *calendarEvent) { event.Title = "Another title" }},
		{calendarFieldDescription, func(event *calendarEvent) { event.Description = "Another agenda" }},
		{calendarFieldLocation, func(event *calendarEvent) { event.Location = "Room 3B" }},
		{calendarFieldStart, func(event *calendarEvent) { event.StartISO = "2026-06-15T09:00:00Z" }},
		{calendarFieldEnd, func(event *calendarEvent) { event.EndISO = "2026-06-15T12:00:00Z" }},
		{calendarFieldTimeZone, func(event *calendarEvent) { event.TimeZone = "Asia/Seoul" }},
		{calendarFieldIsAllDay, func(event *calendarEvent) { event.IsAllDay = true }},
		{calendarFieldColor, func(event *calendarEvent) { event.Color = "#dc2626" }},
		{calendarFieldReminderLeadHours, func(event *calendarEvent) { event.ReminderLeadHours = 1 }},
		{calendarFieldParticipants, func(event *calendarEvent) {
			event.Participants = []calendarParticipant{{PersonID: "person-2", Name: "박예시"}}
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.field, func(t *testing.T) {
			previous := editableCalendarEventForFieldDiff()
			current := previous
			testCase.edit(&current)

			changed := diffCalendarEventFields(previous, current)

			if !reflect.DeepEqual(changed, []string{testCase.field}) {
				t.Fatalf("changed fields = %v, want exactly %q", changed, testCase.field)
			}
			if !hasCalendarEventUserEditableChanges(previous, current) {
				t.Fatalf("a changed %s must reach the store, and this edit would be answered with the old event", testCase.field)
			}
		})
	}
}

func TestDiffCalendarEventFieldsSeesNoChangeInARepeatedSave(t *testing.T) {
	event := editableCalendarEventForFieldDiff()

	if changed := diffCalendarEventFields(event, event); len(changed) != 0 {
		t.Fatalf("changed fields = %v, want none", changed)
	}
	if hasCalendarEventUserEditableChanges(event, event) {
		t.Fatal("a repeated save writes nothing")
	}
}
