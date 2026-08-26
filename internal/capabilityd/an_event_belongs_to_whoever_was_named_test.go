package capabilityd

import (
	"context"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestNamingSomebodyElseDoesNotAddThePersonAsking(t *testing.T) {
	service := serviceWithDirectoryPeople(t, []directoryPerson{
		{MemberID: "member-dawn", Email: "iam@dawn.kim", Name: "여명 김"},
		{MemberID: "member-lee", Email: "lee@dawn.kim", Name: "동하 이"},
	})

	prepared, _, hasFailure := service.prepareCalendarEventWriteInput(
		context.Background(),
		calendarEventWriteInput{Title: "모나 개발자 미팅", People: []string{"여명"}},
		capabilities.ToolInvokeContext{RequesterEmail: "lee@dawn.kim", RequesterName: "동하 이", ResponseLanguage: "ko"},
		true,
	)

	if hasFailure {
		t.Fatal("the company carries this person")
	}
	if len(prepared.Participants) != 1 || prepared.Participants[0].Email != "iam@dawn.kim" {
		t.Fatalf("participants = %+v", prepared.Participants)
	}
}

func TestNamingNobodyMeansThePersonAsking(t *testing.T) {
	service := serviceWithDirectoryPeople(t, []directoryPerson{
		{MemberID: "member-lee", Email: "lee@dawn.kim", Name: "동하 이"},
	})

	prepared, _, hasFailure := service.prepareCalendarEventWriteInput(
		context.Background(),
		calendarEventWriteInput{Title: "치과"},
		capabilities.ToolInvokeContext{RequesterEmail: "lee@dawn.kim", RequesterName: "동하 이", ResponseLanguage: "ko"},
		true,
	)

	if hasFailure {
		t.Fatal("an event with nobody named is the requester's own")
	}
	if len(prepared.Participants) != 1 {
		t.Fatalf("participants = %+v", prepared.Participants)
	}
}

// Whether an event is open to everyone is the model's to say before it calls.
// Reading it out of the attendee list meant a colleague actually called 전체
// could never be invited to anything.
func TestEverybodyIsNobodyInParticular(t *testing.T) {
	service := serviceWithDirectoryPeople(t, []directoryPerson{
		{MemberID: "member-lee", Email: "lee@dawn.kim", Name: "동하 이"},
	})

	prepared, _, hasFailure := service.prepareCalendarEventWriteInput(
		context.Background(),
		calendarEventWriteInput{Title: "전사 워크숍", EveryoneAttends: true},
		capabilities.ToolInvokeContext{RequesterEmail: "lee@dawn.kim", RequesterName: "동하 이", ResponseLanguage: "ko"},
		true,
	)
	if hasFailure {
		t.Fatal("an event open to everyone names nobody to look up")
	}
	if len(prepared.Participants) != 0 || len(prepared.People) != 0 {
		t.Fatalf("an all-hands event left an attendee list: people=%+v participants=%+v", prepared.People, prepared.Participants)
	}
}

func TestAColleagueCalledEveryoneIsStillAColleague(t *testing.T) {
	service := serviceWithDirectoryPeople(t, []directoryPerson{
		{MemberID: "member-jeonche", Email: "jeonche@dawn.kim", Name: "전체 김"},
	})

	prepared, _, hasFailure := service.prepareCalendarEventWriteInput(
		context.Background(),
		calendarEventWriteInput{Title: "면담", People: calendarToolPeopleInput{"전체"}},
		capabilities.ToolInvokeContext{RequesterEmail: "lee@dawn.kim", ResponseLanguage: "ko"},
		true,
	)
	if hasFailure {
		t.Fatal("a name is a name even when it reads like a word the runtime used to watch for")
	}
	if len(prepared.Participants) != 1 || prepared.Participants[0].Email != "jeonche@dawn.kim" {
		t.Fatalf("participants = %+v", prepared.Participants)
	}
}
