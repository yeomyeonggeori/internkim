package admind

import (
	"testing"
)

func TestCalendarParticipantsFromMembersNamesEachPerson(t *testing.T) {
	participants := calendarParticipantsFromMembers([]taskMember{
		{ID: "person-sample", Name: "이샘플", Email: "sample@example.com"},
	})

	if len(participants) != 1 || participants[0].PersonID != "person-sample" || participants[0].Name != "이샘플" {
		t.Fatalf("participants = %+v", participants)
	}
}
