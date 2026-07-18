package admind

import (
	"context"
	"testing"
)

func TestCalendarRemoteEventStateRoundTrip(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	expected := calendarRemoteEventState{
		AccountID:         "account-1",
		CalendarURL:       "/calendars/user@example.com/events/",
		EventUID:          "event-1@example.com",
		RemoteModifiedAt:  "2026-07-15T01:00:00Z",
		LastSeenAt:        "2026-07-15T01:10:00Z",
		MissingDetectedAt: "2026-07-15T01:20:00Z",
	}
	if errorValue := service.upsertCalendarRemoteEventState(ctx, expected); errorValue != nil {
		t.Fatal(errorValue)
	}
	actual, found, errorValue := service.readCalendarRemoteEventState(ctx, expected.AccountID, expected.CalendarURL, expected.EventUID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found {
		t.Fatal("remote event state not found")
	}
	if actual != expected {
		t.Fatalf("state mismatch: got %+v want %+v", actual, expected)
	}
}
