package capabilityd

import (
	"context"
	"strings"
	"testing"
)

func TestAnAttendeeTheCompanyCarriesGoesOntoTheEventAsThemselves(t *testing.T) {
	service := serviceWithDirectoryPeople(t, []directoryPerson{
		{MemberID: "member-dawn", Email: "iam@dawn.kim", Name: "김표본"},
		{MemberID: "member-lee", Email: "lee@dawn.kim", Name: "이가명"},
	})

	participant, failure, hasFailure := service.calendarParticipantForPersonHint(context.Background(), "여명", "ko")

	if hasFailure {
		t.Fatalf("half a colleague's name is how people refer to each other, failure=%+v", failure)
	}
	if participant.Email != "iam@dawn.kim" || participant.PersonID != "member-dawn" {
		t.Fatalf("participant = %+v", participant)
	}
}

func TestAnAttendeeNobodyCanPlaceIsNotWrittenOntoTheEvent(t *testing.T) {
	service := serviceWithDirectoryPeople(t, []directoryPerson{
		{MemberID: "member-lee", Email: "lee@dawn.kim", Name: "이가명"},
	})

	participant, failure, hasFailure := service.calendarParticipantForPersonHint(context.Background(), "모나 개발자", "ko")

	if !hasFailure {
		t.Fatalf("a name carrying no address reaches the company as nobody, and the event keeps whoever filed it: %+v", participant)
	}
	if failure.ErrorCode != "recipient_not_found" || !strings.Contains(failure.Message, "모나 개발자") {
		t.Fatalf("failure = %+v", failure)
	}
}
