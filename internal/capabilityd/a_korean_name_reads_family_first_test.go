package capabilityd

import (
	"context"
	"testing"
)

// A name is recorded given name first so every directory agrees on one order,
// and read family name first by a Korean reader. Handing the record back as
// written is how a colleague ends up addressed as 예시 김.
func TestAnAttendeeIsNamedTheWayTheAnswerIsRead(t *testing.T) {
	service := serviceWithDirectoryPeople(t, []directoryPerson{
		{MemberID: "member-dawn", Email: "kimyesi@example.com", Name: "예시 김"},
	})

	korean, _, hasFailure := service.calendarParticipantForPersonHint(context.Background(), "예시", "ko")
	if hasFailure || korean.Name != "김예시" {
		t.Fatalf("korean participant = %+v", korean)
	}

	english, _, hasFailure := service.calendarParticipantForPersonHint(context.Background(), "예시", "en")
	if hasFailure || english.Name != "예시 김" {
		t.Fatalf("english participant = %+v", english)
	}
}

func TestALatinNameIsNotRewrittenForAKoreanReader(t *testing.T) {
	service := serviceWithDirectoryPeople(t, []directoryPerson{
		{MemberID: "member-smith", Email: "smith@example.com", Name: "John Michael Smith"},
	})

	participant, _, hasFailure := service.calendarParticipantForPersonHint(context.Background(), "Michael", "ko")
	if hasFailure || participant.Name != "John Michael Smith" {
		t.Fatalf("participant = %+v", participant)
	}
}
