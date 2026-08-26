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

func TestEverybodyIsNobodyInParticular(t *testing.T) {
	service := serviceWithDirectoryPeople(t, []directoryPerson{
		{MemberID: "member-lee", Email: "lee@dawn.kim", Name: "동하 이"},
	})

	for _, everyone := range []string{"전체", "all", "@all"} {
		prepared, _, hasFailure := service.prepareCalendarEventWriteInput(
			context.Background(),
			calendarEventWriteInput{Title: "전사 워크숍", People: []string{everyone}},
			capabilities.ToolInvokeContext{RequesterEmail: "lee@dawn.kim", RequesterName: "동하 이", ResponseLanguage: "ko"},
			true,
		)
		if hasFailure {
			t.Fatalf("%q is not a person to look up", everyone)
		}
		if len(prepared.Participants) != 0 || len(prepared.People) != 0 {
			t.Fatalf("%q left an attendee list: people=%+v participants=%+v", everyone, prepared.People, prepared.Participants)
		}
	}
}
