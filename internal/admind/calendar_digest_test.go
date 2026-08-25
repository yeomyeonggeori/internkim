package admind

import (
	"testing"
	"time"
)

func seoul(t *testing.T) *time.Location {
	t.Helper()
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return location
}

func TestADigestHoldsOnlyTheDaysThisPersonIsOn(t *testing.T) {
	location := seoul(t)
	events := []calendarEvent{
		{Title: "회의", StartISO: "2026-09-01T02:00:00Z", Participants: []calendarParticipant{{Email: "member1@example.com"}}},
		{Title: "남의 회의", StartISO: "2026-09-01T03:00:00Z", Participants: []calendarParticipant{{Email: "member2@example.com"}}},
		{Title: "내가 만든 것", StartISO: "2026-09-01T01:00:00Z", CreatedByEmail: "Member1@Example.com"},
	}

	entries := calendarDigestFor(events, "member1@example.com", location)
	if len(entries) != 2 {
		t.Fatalf("entries = %+v", entries)
	}
	if entries[0].Title != "내가 만든 것" || entries[0].At != "10:00" {
		t.Fatalf("first = %+v", entries[0])
	}
	if entries[1].At != "11:00" {
		t.Fatalf("second = %+v", entries[1])
	}
}

func TestNobodyIsMatchedByAnEmptyAddress(t *testing.T) {
	events := []calendarEvent{{Title: "회의", Participants: []calendarParticipant{{Email: ""}}}}
	if entries := calendarDigestFor(events, "", seoul(t)); len(entries) != 0 {
		t.Fatalf("entries = %+v", entries)
	}
	if entries := calendarDigestFor(events, "member1@example.com", seoul(t)); len(entries) != 0 {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestAnAllDayEventCarriesNoHour(t *testing.T) {
	location := seoul(t)
	events := []calendarEvent{
		{Title: "워크숍", IsAllDay: true, StartISO: "2026-09-01T00:00:00Z", CreatedByEmail: "member1@example.com"},
	}
	entries := calendarDigestFor(events, "member1@example.com", location)
	if len(entries) != 1 || entries[0].At != "" {
		t.Fatalf("entries = %+v", entries)
	}
	if body := calendarDigestBody(entries); body != "워크숍" {
		t.Fatalf("body = %q", body)
	}
}

func TestALongDayIsCutRatherThanSentWhole(t *testing.T) {
	entries := []calendarDigestEntry{
		{At: "09:00", Title: "하나"},
		{At: "10:00", Title: "둘"},
		{At: "11:00", Title: "셋"},
		{At: "12:00", Title: "넷"},
		{At: "13:00", Title: "다섯"},
	}
	body := calendarDigestBody(entries)
	if body != "09:00 하나\n10:00 둘\n11:00 셋\n외 2건" {
		t.Fatalf("body = %q", body)
	}
}

func TestADayThatFitsSaysNothingAboutWhatIsLeft(t *testing.T) {
	body := calendarDigestBody([]calendarDigestEntry{{At: "09:00", Title: "하나"}})
	if body != "09:00 하나" {
		t.Fatalf("body = %q", body)
	}
}

func TestOnlySomebodyWithAMattermostAccountAndAnAddressCanBeToldTheirDay(t *testing.T) {
	emails := calendarDigestEmails([]adminUserMutation{
		{MattermostUserID: "mm-1", Email: "member1@example.com"},
		{MattermostUserID: "mm-2", Email: "   "},
		{MattermostUserID: "", Email: "member3@example.com"},
	})
	if len(emails) != 1 || emails["mm-1"] != "member1@example.com" {
		t.Fatalf("emails = %+v", emails)
	}
}
