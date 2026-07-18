package capabilityd

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func mergeCalendarEventUpdateInput(document json.RawMessage, current calendarEventForTool) (json.RawMessage, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return nil, fmt.Errorf("calendar event input is required")
	}
	patch := map[string]json.RawMessage{}
	if errorValue := json.Unmarshal(document, &patch); errorValue != nil {
		return nil, errorValue
	}
	currentInput := calendarEventWriteInput{
		EventID:           current.ID,
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
		currentInput.Participants = current.Participants
	} else {
		currentInput.People = current.People
	}
	mergedDocument, errorValue := json.Marshal(calendarEventWritePayload(currentInput))
	if errorValue != nil {
		return nil, errorValue
	}
	merged := map[string]json.RawMessage{}
	if errorValue := json.Unmarshal(mergedDocument, &merged); errorValue != nil {
		return nil, errorValue
	}
	for field, value := range patch {
		if isCalendarEventUpdatePatchField(field) {
			merged[field] = value
		}
	}
	if _, found := patch["people"]; found {
		delete(merged, "participants")
	}
	if _, found := patch["participants"]; found {
		delete(merged, "people")
	}
	eventID, errorValue := json.Marshal(current.ID)
	if errorValue != nil {
		return nil, errorValue
	}
	merged["eventID"] = eventID
	return json.Marshal(merged)
}

func isCalendarEventUpdatePatchField(field string) bool {
	switch field {
	case "title", "description", "location", "startISO", "endISO", "timeZone", "isAllDay", "color", "people", "participants", "reminderLeadHours", "includeRequester":
		return true
	default:
		return false
	}
}
