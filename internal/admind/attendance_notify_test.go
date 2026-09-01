package admind

import "testing"

func TestOnlyClockingIsWorthTellingTheCompany(t *testing.T) {
	for kind, wanted := range map[string]string{
		attendanceKindClockIn:  "이샘플 출근",
		attendanceKindClockOut: "이샘플 퇴근",
	} {
		title, told := attendanceNotifyTitle(attendanceEvent{Kind: kind, DisplayName: "이샘플"})
		if !told || title != wanted {
			t.Fatalf("%s = %q,%v; want %q,true", kind, title, told, wanted)
		}
	}

	for _, kind := range []string{"leave", "correction", ""} {
		if title, told := attendanceNotifyTitle(attendanceEvent{Kind: kind}); told {
			t.Fatalf("%q should be silent, got %q", kind, title)
		}
	}
}

func TestTheTitleFallsBackToWhateverNamesThePerson(t *testing.T) {
	title, _ := attendanceNotifyTitle(attendanceEvent{Kind: attendanceKindClockIn, MattermostUsername: "member1"})
	if title != "member1 출근" {
		t.Fatalf("title = %q", title)
	}
	title, _ = attendanceNotifyTitle(attendanceEvent{Kind: attendanceKindClockIn, Email: "member1@example.com"})
	if title != "member1@example.com 출근" {
		t.Fatalf("title = %q", title)
	}
}

func TestWhereSomebodyClockedInLeadsTheBody(t *testing.T) {
	if body := attendanceNotifyBody(attendanceEvent{LocationName: "본사", LocalTime: "09:02"}); body != "본사 · 09:02" {
		t.Fatalf("body = %q", body)
	}
	if body := attendanceNotifyBody(attendanceEvent{LocalTime: "09:02"}); body != "09:02" {
		t.Fatalf("a place nobody recorded leaves the time alone, got %q", body)
	}
	if body := attendanceNotifyBody(attendanceEvent{}); body != "" {
		t.Fatalf("body = %q", body)
	}
}

func TestALeaveRequestSaysWhichDaysItCovers(t *testing.T) {
	body := leaveNotifyBody(attendanceLeaveRequestRecord{LeaveTypeName: "연차", StartDate: "2026-09-01", EndDate: "2026-09-03"})
	if body != "연차 2026-09-01 ~ 2026-09-03" {
		t.Fatalf("body = %q", body)
	}
	body = leaveNotifyBody(attendanceLeaveRequestRecord{LeaveTypeName: "연차", StartDate: "2026-09-01", EndDate: "2026-09-01"})
	if body != "연차 2026-09-01" {
		t.Fatalf("body = %q", body)
	}
}

func TestWhoIsToldIsDecidedByTheDirectory(t *testing.T) {
	directory := []adminUserMutation{
		{Email: "admin@example.com", Role: adminUserRoleAdmin},
		{Email: "ops@example.com", Role: adminUserRoleOperationsAdmin},
		{Email: "member@example.com", Role: "member"},
		{Email: "", Role: adminUserRoleAdmin},
	}

	administrators := attendanceNotifyRecipients(directory, func(record adminUserMutation) bool {
		return record.Role == adminUserRoleAdmin || record.Role == adminUserRoleOperationsAdmin
	})
	if len(administrators) != 2 {
		t.Fatalf("a request waits on whoever gets to it first, so every administrator is told: %+v", administrators)
	}

	everyoneElse := attendanceNotifyRecipients(directory, func(record adminUserMutation) bool {
		return record.Email != "member@example.com"
	})
	if len(everyoneElse) != 2 {
		t.Fatalf("everyoneElse = %+v", everyoneElse)
	}
	for _, address := range everyoneElse {
		if address == "member@example.com" {
			t.Fatal("the person who clocked must not be told about themselves")
		}
	}
}

// Somebody added since the company left Mattermost has no account there, and
// telling the company about attendance is not a thing to skip them for.
func TestSomebodyWithNoMessengerAccountIsStillTold(t *testing.T) {
	told := attendanceNotifyRecipients([]adminUserMutation{
		{Email: "New1@Example.com", Role: "member"},
	}, func(adminUserMutation) bool { return true })
	if len(told) != 1 || told[0] != "new1@example.com" {
		t.Fatalf("told = %+v", told)
	}
}
