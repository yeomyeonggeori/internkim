package admind

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCalendarEventParticipantsFallbackToLegacyPeopleLine(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	startTime := time.Now().UTC().Add(2 * time.Hour)
	endTime := startTime.Add(time.Hour)
	event := calendarEvent{
		ID:                "legacy-participants",
		UID:               "legacy-participants@internkim",
		Title:             "Legacy participant sync",
		Description:       "이샘플, 김예시\nBring agenda",
		StartISO:          startTime.Format(time.RFC3339),
		EndISO:            endTime.Format(time.RFC3339),
		TimeZone:          "UTC",
		Color:             "#2563eb",
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
	if strings.Join(calendarParticipantNames(stored.Participants), "|") != "이샘플|김예시" {
		t.Fatalf("participants = %+v", stored.Participants)
	}
}

func TestCalendarRelatedParticipantsIncludeCreatorWithoutMutatingParticipants(t *testing.T) {
	event := calendarEvent{
		Participants: []calendarParticipant{
			{PersonID: "person-dongha", Name: "이샘플", Email: "dongha@example.com"},
		},
		CreatedByName:  "김예시",
		CreatedByEmail: "yeomyeong@example.com",
	}

	relatedParticipants := calendarEventRelatedParticipants(event)

	if len(relatedParticipants) != 2 {
		t.Fatalf("related participants = %+v", relatedParticipants)
	}
	if relatedParticipants[0].Name != "이샘플" || relatedParticipants[1].Name != "김예시" {
		t.Fatalf("related participants order = %+v", relatedParticipants)
	}
	if len(event.Participants) != 1 || event.Participants[0].Name != "이샘플" {
		t.Fatalf("event participants mutated = %+v", event.Participants)
	}
}

func TestCalendarRelatedParticipantsDeduplicateCreator(t *testing.T) {
	event := calendarEvent{
		Participants: []calendarParticipant{
			{PersonID: "person-yeomyeong", Name: "김예시", Email: "yeomyeong@example.com"},
		},
		CreatedByName:  "김예시",
		CreatedByEmail: "yeomyeong@example.com",
	}

	relatedParticipants := calendarEventRelatedParticipants(event)

	if len(relatedParticipants) != 1 || relatedParticipants[0].Name != "김예시" {
		t.Fatalf("related participants = %+v", relatedParticipants)
	}
}
