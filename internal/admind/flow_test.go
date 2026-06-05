package admind

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFlowTasksWithMatchingDatesUsesStartAndEndDateOnly(t *testing.T) {
	tasks := []flowTask{
		{ID: "same-date", StartDate: "2026-05-07", EndDate: "2026-05-07", Content: "10분 회의"},
		{ID: "different-start", StartDate: "2026-05-08", EndDate: "2026-05-07", Content: "10분 회의"},
		{ID: "different-end", StartDate: "2026-05-07", EndDate: "2026-05-08", Content: "10분 회의"},
	}

	matches := flowTasksWithMatchingDates(tasks, flowTask{
		ID:        "candidate",
		StartDate: "2026-05-07",
		EndDate:   "2026-05-07",
		Content:   "완전히 다른 표현이어도 날짜로만 후보를 고른다",
	})

	if len(matches) != 1 || matches[0].ID != "same-date" {
		t.Fatalf("matches = %+v", matches)
	}
}

func TestFlowDuplicateLLMRequestConstrainsDuplicateTaskIDToExistingTasks(t *testing.T) {
	request := flowDuplicateLLMRequest(flowTask{ID: "candidate"}, []flowTask{
		{ID: "task-1", Content: "10분 회의"},
		{ID: "task-2", Content: "짧은 미팅"},
	})
	schema := request["structuredOutputSchema"].(map[string]any)["document"].(map[string]any)
	properties := schema["properties"].(map[string]any)
	duplicateTaskID := properties["duplicateTaskID"].(map[string]any)
	values := duplicateTaskID["enum"].([]string)

	if len(values) != 3 || values[0] != "" || values[1] != "task-1" || values[2] != "task-2" {
		t.Fatalf("duplicate task enum = %+v", values)
	}
}

func TestFlowMembersWithoutFleetCredentialsDoesNotReturnSeedMembers(t *testing.T) {
	service := NewService(Configuration{})
	request := httptest.NewRequest(http.MethodGet, "/flow/api/summary", nil)

	members := service.flowMembers(request)

	if len(members) != 0 {
		t.Fatalf("members = %+v", members)
	}
}

func TestNormalizeFlowTaskDatesSetsCompletedStartEndAndWeek(t *testing.T) {
	now := time.Date(2026, time.June, 11, 10, 0, 0, 0, flowDateLocation())

	dates := normalizeFlowTaskDates(flowTaskWriteRequest{}, "완료", now)

	if dates.StartDate != "2026-06-11" || dates.EndDate != "2026-06-11" || dates.WeekCode != "26W24" {
		t.Fatalf("dates = %+v", dates)
	}
}

func TestNormalizeFlowTaskDatesPreservesExplicitCompletedDates(t *testing.T) {
	now := time.Date(2026, time.June, 11, 10, 0, 0, 0, flowDateLocation())

	dates := normalizeFlowTaskDates(flowTaskWriteRequest{StartDate: "2026-06-09", EndDate: "2026-06-10"}, "완료", now)

	if dates.StartDate != "2026-06-09" || dates.EndDate != "2026-06-10" || dates.WeekCode != "26W24" {
		t.Fatalf("dates = %+v", dates)
	}
}

func TestNormalizeFlowTaskDatesSetsPlannedStartAndWeek(t *testing.T) {
	now := time.Date(2026, time.June, 11, 10, 0, 0, 0, flowDateLocation())

	dates := normalizeFlowTaskDates(flowTaskWriteRequest{}, "예정", now)

	if dates.StartDate != "2026-06-11" || dates.EndDate != "" || dates.WeekCode != "26W24" {
		t.Fatalf("dates = %+v", dates)
	}
}

func TestBuildFlowMetricsKeepsDistanceAndLegacyScoreAliases(t *testing.T) {
	metrics := buildFlowMetrics([]flowTask{
		{
			ParticipantNames: []string{"김철수", "이영희"},
			Business:         "여명거리",
			Type:             "기능",
			Size:             "M",
			Status:           "완료",
		},
		{
			ParticipantNames: []string{"김철수"},
			Business:         "김인턴",
			Type:             "문서",
			Size:             "S",
			Status:           "진행",
		},
	}, flowDefinitions{
		Sizes: []flowSizeDefinition{
			{Name: "S", DistanceKM: 2},
			{Name: "M", DistanceKM: 5},
		},
	})

	if metrics.TotalDistance != 6 {
		t.Fatalf("total distance = %d, want 6", metrics.TotalDistance)
	}
	if metrics.TotalScore != metrics.TotalDistance {
		t.Fatalf("legacy total score = %d, want %d", metrics.TotalScore, metrics.TotalDistance)
	}
	if metrics.MemberDistances["김철수"] != 6 {
		t.Fatalf("김철수 distance = %d, want 6", metrics.MemberDistances["김철수"])
	}
	if metrics.MemberScores["김철수"] != metrics.MemberDistances["김철수"] {
		t.Fatalf("김철수 legacy score = %d, want %d", metrics.MemberScores["김철수"], metrics.MemberDistances["김철수"])
	}
	if metrics.MemberDistances["이영희"] != 5 {
		t.Fatalf("이영희 distance = %d, want 5", metrics.MemberDistances["이영희"])
	}
	if metrics.MemberScores["이영희"] != metrics.MemberDistances["이영희"] {
		t.Fatalf("이영희 legacy score = %d, want %d", metrics.MemberScores["이영희"], metrics.MemberDistances["이영희"])
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
