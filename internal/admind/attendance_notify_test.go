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

	// A correction or a leave day is not somebody arriving.
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
	// One day is one date, not a range that starts and ends together.
	body = leaveNotifyBody(attendanceLeaveRequestRecord{LeaveTypeName: "연차", StartDate: "2026-09-01", EndDate: "2026-09-01"})
	if body != "연차 2026-09-01" {
		t.Fatalf("body = %q", body)
	}
}

func TestWhoIsToldIsDecidedByTheDirectory(t *testing.T) {
	directory := []adminUserMutation{
		{MattermostUserID: "mm-admin", Role: adminUserRoleAdmin},
		{MattermostUserID: "mm-ops", Role: adminUserRoleOperationsAdmin},
		{MattermostUserID: "mm-staff", Role: "member"},
		{MattermostUserID: "", Role: adminUserRoleAdmin},
	}

	administrators := attendanceNotifyRecipients(directory, func(record adminUserMutation) bool {
		return record.Role == adminUserRoleAdmin || record.Role == adminUserRoleOperationsAdmin
	})
	if len(administrators) != 2 {
		t.Fatalf("a request waits on whoever gets to it first, so every administrator is told: %+v", administrators)
	}

	// The one who clocked was there when it happened.
	everyoneElse := attendanceNotifyRecipients(directory, func(record adminUserMutation) bool {
		return record.MattermostUserID != "mm-staff"
	})
	if len(everyoneElse) != 2 {
		t.Fatalf("everyoneElse = %+v", everyoneElse)
	}
	for _, externalID := range everyoneElse {
		if externalID == "mm-staff" {
			t.Fatal("the person who clocked must not be told about themselves")
		}
	}
}
