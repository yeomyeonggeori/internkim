package admind

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTaskMembersWithoutFleetCredentialsDoesNotReturnSeedMembers(t *testing.T) {
	emptyPolicyServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write([]byte(`{"people":[]}`))
	}))
	defer emptyPolicyServer.Close()
	service := NewService(Configuration{BlueclawBaseURL: emptyPolicyServer.URL})
	request := httptest.NewRequest(http.MethodGet, "/flow/api/summary", nil)

	members := service.taskMembers(request)

	if len(members) != 0 {
		t.Fatalf("members = %+v", members)
	}
}

func TestNormalizeTaskDatesSetsCompletedStartEndAndWeek(t *testing.T) {
	now := time.Date(2026, time.June, 11, 10, 0, 0, 0, defaultCompanyLocation())

	dates := normalizeTaskDates(taskWriteRequest{}, "completed", now)

	if dates.StartDate != "2026-06-11" || dates.EndDate != "2026-06-11" || dates.WeekCode != "26W24" {
		t.Fatalf("dates = %+v", dates)
	}
}

func TestNormalizeTaskDatesPreservesExplicitCompletedDates(t *testing.T) {
	now := time.Date(2026, time.June, 11, 10, 0, 0, 0, defaultCompanyLocation())

	dates := normalizeTaskDates(taskWriteRequest{StartDate: "2026-06-09", EndDate: "2026-06-10"}, "completed", now)

	if dates.StartDate != "2026-06-09" || dates.EndDate != "2026-06-10" || dates.WeekCode != "26W24" {
		t.Fatalf("dates = %+v", dates)
	}
}

func TestNormalizeTaskDatesLeavesPlannedWorkUndated(t *testing.T) {
	now := time.Date(2026, time.June, 11, 10, 0, 0, 0, defaultCompanyLocation())

	dates := normalizeTaskDates(taskWriteRequest{}, "planned", now)

	if dates.StartDate != "" || dates.EndDate != "" || dates.WeekCode != "26W24" {
		t.Fatalf("dates = %+v", dates)
	}
}

func TestMembersFromUserRecordsSortsByHireDate(t *testing.T) {
	members := membersFromUserRecords([]adminUserMutation{
		{Email: "late@example.com", Name: "Late", HireDate: "2026-05-10", Role: "admin"},
		{Email: "unknown@example.com", Name: "Unknown", Role: "admin"},
		{Email: "early@example.com", Name: "Early", HireDate: "2024-01-03", Role: "member"},
		{Email: "same@example.com", Name: "Alpha", HireDate: "2026-05-10", Role: "member"},
	})

	got := []string{members[0].Email, members[1].Email, members[2].Email, members[3].Email}
	want := []string{"early@example.com", "same@example.com", "late@example.com", "unknown@example.com"}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("members order = %+v, want %+v", got, want)
		}
	}
}

func TestMembersFromUserRecordsCarriesTheRoleTheDirectoryGaveThem(t *testing.T) {
	members := membersFromUserRecords([]adminUserMutation{
		{Email: "colleague@example.com", Name: "이샘플", Role: adminUserRoleMember},
		{Email: "admin@example.com", Name: "박예시", Role: adminUserRoleAdmin},
	})

	if len(members) != 2 {
		t.Fatalf("members = %d, want 2", len(members))
	}
	roleByEmail := map[string]string{}
	for _, member := range members {
		roleByEmail[member.Email] = member.Role
	}
	if roleByEmail["colleague@example.com"] != adminUserRoleMember {
		t.Fatalf("colleague role = %q, want %q", roleByEmail["colleague@example.com"], adminUserRoleMember)
	}
	if roleByEmail["admin@example.com"] != adminUserRoleAdmin {
		t.Fatalf("admin role = %q, want %q", roleByEmail["admin@example.com"], adminUserRoleAdmin)
	}
}
