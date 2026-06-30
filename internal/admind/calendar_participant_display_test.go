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
		Description:       "이샘플, 김표본\nBring agenda",
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
	if strings.Join(calendarParticipantNames(stored.Participants), "|") != "이샘플|김표본" {
		t.Fatalf("participants = %+v", stored.Participants)
	}
}

func TestCalendarNotificationTargetsPreferEventParticipants(t *testing.T) {
	event := calendarEvent{
		Description: "캘린더 메모\n이 줄은 일반 설명",
		Participants: []calendarParticipant{
			{PersonID: "person-gamyeong", Name: "이샘플", Email: "gamyeong@example.com"},
		},
	}
	users := []mattermostUserRecord{
		{ID: "user-1", Username: "gamyeong", Nickname: "이샘플", Email: "gamyeong@example.com"},
	}

	people, hasPeople := calendarNotificationPeople(event)
	targets := calendarTargetsForPeople(people, users)

	if !hasPeople || strings.Join(people, "|") != "이샘플" {
		t.Fatalf("people=%+v hasPeople=%v", people, hasPeople)
	}
	if len(targets) != 1 || targets[0].Key != "dm:user-1" {
		t.Fatalf("targets = %+v", targets)
	}
}

func TestCalendarRelatedParticipantsIncludeCreatorWithoutMutatingParticipants(t *testing.T) {
	event := calendarEvent{
		Participants: []calendarParticipant{
			{PersonID: "person-gamyeong", Name: "이샘플", Email: "gamyeong@example.com"},
		},
		CreatedByName:  "김표본",
		CreatedByEmail: "pyobon@example.com",
	}

	relatedParticipants := calendarEventRelatedParticipants(event)

	if len(relatedParticipants) != 2 {
		t.Fatalf("related participants = %+v", relatedParticipants)
	}
	if relatedParticipants[0].Name != "이샘플" || relatedParticipants[1].Name != "김표본" {
		t.Fatalf("related participants order = %+v", relatedParticipants)
	}
	if len(event.Participants) != 1 || event.Participants[0].Name != "이샘플" {
		t.Fatalf("event participants mutated = %+v", event.Participants)
	}
}

func TestCalendarRelatedParticipantsDeduplicateCreator(t *testing.T) {
	event := calendarEvent{
		Participants: []calendarParticipant{
			{PersonID: "person-pyobon", Name: "김표본", Email: "pyobon@example.com"},
		},
		CreatedByName:  "김표본",
		CreatedByEmail: "pyobon@example.com",
	}

	relatedParticipants := calendarEventRelatedParticipants(event)

	if len(relatedParticipants) != 1 || relatedParticipants[0].Name != "김표본" {
		t.Fatalf("related participants = %+v", relatedParticipants)
	}
}

func TestCalendarNotificationTargetsIncludeCreator(t *testing.T) {
	event := calendarEvent{
		Participants: []calendarParticipant{
			{PersonID: "person-gamyeong", Name: "이샘플", Email: "gamyeong@example.com"},
		},
		CreatedByName:  "김표본",
		CreatedByEmail: "pyobon@example.com",
	}
	users := []mattermostUserRecord{
		{ID: "user-1", Username: "gamyeong", Nickname: "이샘플", Email: "gamyeong@example.com"},
		{ID: "user-2", Username: "pyobon", Nickname: "김표본", Email: "pyobon@example.com"},
	}

	targets, resolved := calendarNotificationTargetsForUsers(event, users)

	if !resolved || len(targets) != 2 || targets[0].Key != "dm:user-1" || targets[1].Key != "dm:user-2" {
		t.Fatalf("targets = %+v resolved=%v", targets, resolved)
	}
}

func TestCalendarNotificationTargetsIgnoreUnmatchedCreator(t *testing.T) {
	event := calendarEvent{
		Participants: []calendarParticipant{
			{PersonID: "person-gamyeong", Name: "이샘플", Email: "gamyeong@example.com"},
		},
		CreatedByName:  "김표본",
		CreatedByEmail: "pyobon@example.com",
	}
	users := []mattermostUserRecord{
		{ID: "user-1", Username: "gamyeong", Nickname: "이샘플", Email: "gamyeong@example.com"},
	}

	targets, resolved := calendarNotificationTargetsForUsers(event, users)

	if !resolved || len(targets) != 1 || targets[0].Key != "dm:user-1" {
		t.Fatalf("targets = %+v resolved=%v", targets, resolved)
	}
}
