package admind

import (
	"encoding/json"
	"testing"
)

func TestCalendarParticipantsKeepDuplicateNamesWithDifferentIdentity(t *testing.T) {
	participants := normalizeCalendarParticipants([]calendarParticipant{
		{PersonID: "person-left", Name: "김여명", Email: "left@example.com"},
		{PersonID: "person-right", Name: "김여명", Email: "right@example.com"},
	})

	if len(participants) != 2 {
		t.Fatalf("participants = %+v", participants)
	}
}

func TestCalendarParticipantsDropClientImage(t *testing.T) {
	participants := normalizeCalendarParticipants([]calendarParticipant{
		{PersonID: "person-dongha", Name: "이샘플", Email: "dongha@example.com", Image: "https://example.com/profile.png"},
	})

	if len(participants) != 1 || participants[0].Image != "" {
		t.Fatalf("participants = %+v", participants)
	}
}

func TestCalendarEventWriteRequestIgnoresParticipantImage(t *testing.T) {
	var payload calendarEventWriteRequest
	if errorValue := json.Unmarshal([]byte(`{"participants":[{"personID":"person-dongha","name":"이샘플","email":"dongha@example.com","image":"https://example.com/profile.png"}]}`), &payload); errorValue != nil {
		t.Fatal(errorValue)
	}

	participants := calendarParticipantsFromIdentities(payload.Participants)
	if len(participants) != 1 || participants[0].Image != "" {
		t.Fatalf("participants = %+v", participants)
	}
}
