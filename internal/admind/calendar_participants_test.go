package admind

import (
	"context"
	"testing"
	"time"
)

func TestCalendarEventParticipantsRoundTrip(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	startTime := time.Now().UTC().Add(2 * time.Hour)
	endTime := startTime.Add(time.Hour)
	event := calendarEvent{
		ID:          "participants-roundtrip",
		UID:         "participants-roundtrip@internkim",
		Title:       "Participant sync",
		Description: "Bring agenda",
		StartISO:    startTime.Format(time.RFC3339),
		EndISO:      endTime.Format(time.RFC3339),
		TimeZone:    "UTC",
		Color:       "#2563eb",
		Participants: []calendarParticipant{
			{PersonID: "person-dongha", Name: "이동하", Email: "dongha@example.com"},
			{PersonID: "person-yeomyeong", Name: "김여명", Email: "yeomyeong@example.com"},
		},
		ReminderLeadHours: 24,
		CreatedByEmail:    "admin@example.com",
	}

	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	stored, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found {
		t.Fatal("event not found after write")
	}
	if len(stored.Participants) != 2 {
		t.Fatalf("participants = %+v", stored.Participants)
	}
	if stored.Participants[0].PersonID != "person-dongha" || stored.Participants[1].PersonID != "person-yeomyeong" {
		t.Fatalf("participants order = %+v", stored.Participants)
	}

	stored.Participants = stored.Participants[:1]
	if errorValue := service.writeCalendarEvent(ctx, stored); errorValue != nil {
		t.Fatal(errorValue)
	}
	updated, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found {
		t.Fatal("event not found after update")
	}
	if len(updated.Participants) != 1 || updated.Participants[0].PersonID != "person-dongha" {
		t.Fatalf("updated participants = %+v", updated.Participants)
	}
}

func TestCalendarRemotePullPreservesStoredParticipants(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	startTime := time.Date(2026, 6, 1, 1, 0, 0, 0, time.UTC)
	endTime := startTime.Add(time.Hour)
	event := calendarEvent{
		ID:          "remote-participants-id",
		UID:         "remote-participants@internkim",
		Title:       "Local title",
		Description: "Local description",
		StartISO:    startTime.Format(time.RFC3339),
		EndISO:      endTime.Format(time.RFC3339),
		TimeZone:    "Asia/Seoul",
		Color:       "#2563eb",
		Participants: []calendarParticipant{
			{PersonID: "person-dongha", Name: "이동하", Email: "dongha@example.com"},
		},
		ReminderLeadHours: 24,
		CreatedByEmail:    "admin@example.com",
		RemoteSource:      remoteCalendarProviderGoogle,
		RemoteHref:        "/calendars/me/events/remote-participants.ics",
		RemoteETag:        `"etag-old"`,
	}
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	remoteEvent := event
	remoteEvent.Title = "Remote title"
	remoteEvent.Participants = nil
	remoteEvent.RemoteETag = `"etag-new"`
	account := remoteCalendarAccount{ID: "remote-account"}

	if errorValue := service.applyPulledRemoteEvent(ctx, account, event, true, remoteEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	stored, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found {
		t.Fatal("event not found after pull")
	}
	if stored.Title != "Remote title" {
		t.Fatalf("title = %q", stored.Title)
	}
	if len(stored.Participants) != 1 || stored.Participants[0].PersonID != "person-dongha" {
		t.Fatalf("participants = %+v", stored.Participants)
	}
}
