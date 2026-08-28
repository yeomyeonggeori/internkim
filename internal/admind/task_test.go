package admind

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTaskStaffActorWithoutDeviceAuthIsolatesToTenantAdmin(t *testing.T) {
	service := NewService(Configuration{AdminEmailPath: writeTestFile(t, "admin03@example.test")})

	if !service.isTaskStaffActor(context.Background(), "admin03@example.test") {
		t.Fatal("tenant's own seed admin must keep flow staff access without device auth")
	}
	if service.isTaskStaffActor(context.Background(), "admin10@example.test") {
		t.Fatal("foreign tenant account must not gain flow staff access without device auth")
	}
}

func TestTasksWithMatchingOwnerAndDates(t *testing.T) {
	tasks := []Task{
		{ID: "same-owner-and-date", OwnerID: "owner-1", StartDate: "2026-05-07", EndDate: "2026-05-07", Content: "10분 회의"},
		{ID: "different-owner", OwnerID: "owner-2", StartDate: "2026-05-07", EndDate: "2026-05-07", Content: "10분 회의"},
		{ID: "different-start", OwnerID: "owner-1", StartDate: "2026-05-08", EndDate: "2026-05-07", Content: "10분 회의"},
		{ID: "different-end", OwnerID: "owner-1", StartDate: "2026-05-07", EndDate: "2026-05-08", Content: "10분 회의"},
	}

	matches := tasksWithMatchingOwnerAndDates(tasks, Task{
		ID:        "candidate",
		OwnerID:   "owner-1",
		StartDate: "2026-05-07",
		EndDate:   "2026-05-07",
		Content:   "완전히 다른 표현이어도 owner와 날짜로 후보를 고른다",
	})

	if len(matches) != 1 || matches[0].ID != "same-owner-and-date" {
		t.Fatalf("matches = %+v", matches)
	}
}

func TestTaskDuplicateLLMRequestConstrainsDuplicateTaskIDToExistingTasks(t *testing.T) {
	request := taskDuplicateLLMRequest(Task{ID: "candidate"}, []Task{
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
	now := time.Date(2026, time.June, 11, 10, 0, 0, 0, taskDateLocation())

	dates := normalizeTaskDates(taskWriteRequest{}, "completed", now)

	if dates.StartDate != "2026-06-11" || dates.EndDate != "2026-06-11" || dates.WeekCode != "26W24" {
		t.Fatalf("dates = %+v", dates)
	}
}

func TestNormalizeTaskDatesPreservesExplicitCompletedDates(t *testing.T) {
	now := time.Date(2026, time.June, 11, 10, 0, 0, 0, taskDateLocation())

	dates := normalizeTaskDates(taskWriteRequest{StartDate: "2026-06-09", EndDate: "2026-06-10"}, "completed", now)

	if dates.StartDate != "2026-06-09" || dates.EndDate != "2026-06-10" || dates.WeekCode != "26W24" {
		t.Fatalf("dates = %+v", dates)
	}
}

func TestNormalizeTaskDatesLeavesPlannedWorkUndated(t *testing.T) {
	now := time.Date(2026, time.June, 11, 10, 0, 0, 0, taskDateLocation())

	dates := normalizeTaskDates(taskWriteRequest{}, "planned", now)

	if dates.StartDate != "" || dates.EndDate != "" || dates.WeekCode != "26W24" {
		t.Fatalf("dates = %+v", dates)
	}
}

func TestStatusCompletedWhenEnded(t *testing.T) {
	for _, testCase := range []struct {
		status  string
		endDate string
		want    string
	}{
		{status: "planned", endDate: "2026-06-11", want: "completed"},
		{status: "in_progress", endDate: "2026-06-10", want: "completed"},
		{status: "planned", endDate: "2026-06-12", want: "planned"},
		{status: "planned", endDate: "", want: "planned"},
		{status: "requested", endDate: "2026-06-10", want: "requested"},
		{status: "stopped", endDate: "2026-06-10", want: "stopped"},
	} {
		if got := statusCompletedWhenEnded(testCase.status, testCase.endDate, "2026-06-11"); got != testCase.want {
			t.Fatalf("statusCompletedWhenEnded(%q, %q) = %q, want %q", testCase.status, testCase.endDate, got, testCase.want)
		}
	}
}

func TestBuildTaskMetricsCountsCompletedDistanceAndKeepsLegacyScoreAliases(t *testing.T) {
	metrics := buildTaskMetrics([]Task{
		{
			ParticipantNames: []string{"김철수", "이영희"},
			Business:         "샘플거리",
			Type:             "기능",
			Size:             "M",
			Status:           "completed",
			EndDate:          "2026-06-02",
		},
		{
			ParticipantNames: []string{"김철수"},
			Business:         "샘플거리",
			Type:             "개선",
			Size:             "M",
			Status:           "completed",
		},
		{
			ParticipantNames: []string{"김철수"},
			Business:         "김인턴",
			Type:             "문서",
			Size:             "S",
			Status:           "in_progress",
		},
	}, taskDefinitions{
		Sizes: []taskSizeDefinition{
			{Name: "S", DistanceKM: 2},
			{Name: "M", DistanceKM: 5},
		},
	})

	if metrics.TotalDistance != 5 {
		t.Fatalf("total distance = %d, want 5", metrics.TotalDistance)
	}
	if metrics.TotalScore != metrics.TotalDistance {
		t.Fatalf("legacy total score = %d, want %d", metrics.TotalScore, metrics.TotalDistance)
	}
	if metrics.MemberDistances["김철수"] != 5 {
		t.Fatalf("김철수 distance = %d, want 5", metrics.MemberDistances["김철수"])
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

func TestBuildTaskMemberScoresUsesSpreadsheetWeights(t *testing.T) {
	members := []taskMember{
		{ID: "member-a", Name: "김철수"},
		{ID: "member-b", Name: "이영희"},
	}
	definitions := taskDefinitions{
		Sizes: []taskSizeDefinition{
			{Name: "S", DistanceKM: 2},
			{Name: "M", DistanceKM: 5},
		},
	}
	weekStart := time.Date(2026, time.June, 1, 0, 0, 0, 0, taskDateLocation())
	scoreDetails := buildTaskMemberScoreDetails([]Task{
		{
			ParticipantIDs:   []string{"member-a"},
			ParticipantNames: []string{"김철수"},
			Size:             "S",
			Status:           "completed",
			EndDate:          "2026-06-01",
		},
		{
			ParticipantIDs:   []string{"member-b"},
			ParticipantNames: []string{"이영희"},
			Size:             "M",
			Status:           "in_progress",
			EndDate:          "2026-06-01",
		},
	}, members, definitions, weekStart)
	scores := currentTaskMemberScores(scoreDetails)

	if scores["member-a"] != 115 {
		t.Fatalf("member-a score = %d, want 115", scores["member-a"])
	}
	if scoreDetails["member-a"].WeeklyScore != 115 || scoreDetails["member-a"].MonthlyScore != 115 || scoreDetails["member-a"].CurrentScore != 115 {
		t.Fatalf("member-a score detail = %+v", scoreDetails["member-a"])
	}
	if scores["member-b"] != 0 {
		t.Fatalf("member-b score = %d, want 0", scores["member-b"])
	}
}

func TestWeightedTaskScoreUsesRecentFivePeriodBaseline(t *testing.T) {
	score := weightedTaskScore([]int{4, 3, 2, 1, 0})

	if score < 107.69 || score > 107.70 {
		t.Fatalf("score = %.2f, want 107.69", score)
	}
}

func TestBuildTaskMemberScoresKeepsDuplicateNamesSeparate(t *testing.T) {
	members := []taskMember{
		{ID: "member-a", Name: "김철수"},
		{ID: "member-b", Name: "김철수"},
	}
	definitions := taskDefinitions{
		Sizes: []taskSizeDefinition{
			{Name: "S", DistanceKM: 2},
		},
	}
	weekStart := time.Date(2026, time.June, 1, 0, 0, 0, 0, taskDateLocation())
	scores := buildTaskMemberScores([]Task{
		{
			ParticipantIDs:   []string{"member-a"},
			ParticipantNames: []string{"김철수"},
			Size:             "S",
			Status:           "completed",
			EndDate:          "2026-06-01",
		},
	}, members, definitions, weekStart)

	if scores["member-a"] != 115 {
		t.Fatalf("member-a score = %d, want 115", scores["member-a"])
	}
	if scores["member-b"] != 0 {
		t.Fatalf("member-b score = %d, want 0", scores["member-b"])
	}
	if _, found := scores["김철수"]; found {
		t.Fatalf("score should not use duplicate member name key: %+v", scores)
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

func TestMembersFromUserRecordsPreservesOperationsAdminRole(t *testing.T) {
	members := membersFromUserRecords([]adminUserMutation{
		{Email: "operator@example.com", Name: "Operator", Role: "operationsAdmin"},
	})

	if len(members) != 1 {
		t.Fatalf("members = %d, want 1", len(members))
	}
	if members[0].Role != "operationsAdmin" {
		t.Fatalf("role = %q, want operationsAdmin", members[0].Role)
	}
}
