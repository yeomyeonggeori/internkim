package admind

import "time"

func newLocalTestCalendarEvent(idSuffix string, title string) calendarEvent {
	start := time.Now().UTC().Add(3 * time.Hour).Truncate(time.Second)
	end := start.Add(time.Hour)
	return calendarEvent{
		ID:                idSuffix,
		UID:               idSuffix + "@internkim",
		Title:             title,
		StartISO:          start.Format(time.RFC3339),
		EndISO:            end.Format(time.RFC3339),
		TimeZone:          "UTC",
		Color:             "#2563eb",
		ReminderLeadHours: 24,
		CreatedByEmail:    "admin@example.com",
	}
}
