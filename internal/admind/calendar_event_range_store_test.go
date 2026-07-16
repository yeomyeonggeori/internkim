package admind

import (
	"context"
	"slices"
	"testing"
	"time"
)

func TestReadCalendarEventRowsPreservesFractionalRangeBoundaries(t *testing.T) {
	service := newCalendarTestService(t)
	rangeStart := time.Date(2026, time.July, 20, 0, 0, 0, 500_000_000, time.UTC)
	rangeEnd := rangeStart.Add(time.Second)
	boundaryOffset := time.Nanosecond
	events := []calendarEvent{
		calendarEventRangeStoreTestEvent("ends-at-start", rangeStart.Add(-time.Second), rangeStart),
		calendarEventRangeStoreTestEvent("overlaps-start", rangeStart.Add(-time.Second), rangeStart.Add(boundaryOffset)),
		calendarEventRangeStoreTestEvent("starts-before-end", rangeEnd.Add(-boundaryOffset), rangeEnd.Add(time.Second)),
		calendarEventRangeStoreTestEvent("starts-at-end", rangeEnd, rangeEnd.Add(time.Second)),
	}
	for _, event := range events {
		if errorValue := service.writeCalendarEventWithSource(context.Background(), event, calendarSourcePull); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	actualEvents, errorValue := readCalendarEventRows(context.Background(), database, rangeStart, rangeEnd)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	actualEventIDs := make([]string, 0, len(actualEvents))
	for _, event := range actualEvents {
		actualEventIDs = append(actualEventIDs, event.ID)
	}
	expectedEventIDs := []string{"overlaps-start", "starts-before-end"}
	if !slices.Equal(actualEventIDs, expectedEventIDs) {
		t.Fatalf("event IDs = %v, want %v", actualEventIDs, expectedEventIDs)
	}
}

func calendarEventRangeStoreTestEvent(eventID string, startTime time.Time, endTime time.Time) calendarEvent {
	return calendarEvent{
		ID:                eventID,
		UID:               eventID + "@example.test",
		Title:             eventID,
		StartISO:          startTime.Format(time.RFC3339Nano),
		EndISO:            endTime.Format(time.RFC3339Nano),
		TimeZone:          "UTC",
		Color:             "#2563eb",
		ReminderLeadHours: calendarDefaultReminderLeadHours,
		CreatedByEmail:    "staff@example.com",
	}
}
