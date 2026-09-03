package admind

import (
	"testing"
)

func TestCalendarParticipantsKeepDuplicateNamesWithDifferentIdentity(t *testing.T) {
	participants := normalizeCalendarParticipants([]calendarParticipant{
		{PersonID: "person-left", Name: "김예시", Email: "left@example.com"},
		{PersonID: "person-right", Name: "김예시", Email: "right@example.com"},
	})

	if len(participants) != 2 {
		t.Fatalf("participants = %+v", participants)
	}
}
