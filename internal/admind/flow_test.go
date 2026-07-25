package admind

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFlowStaffActorWithoutDeviceAuthIsolatesToTenantAdmin(t *testing.T) {
	service := NewService(Configuration{AdminEmailPath: writeTestFile(t, "admin03@example.test")})

	if !service.isFlowStaffActor(context.Background(), "admin03@example.test") {
		t.Fatal("tenant's own seed admin must keep flow staff access without device auth")
	}
	if service.isFlowStaffActor(context.Background(), "admin10@example.test") {
		t.Fatal("foreign tenant account must not gain flow staff access without device auth")
	}
}

func TestFlowTasksWithMatchingOwnerAndDates(t *testing.T) {
	tasks := []flowTask{
		{ID: "same-owner-and-date", OwnerID: "owner-1", StartDate: "2026-05-07", EndDate: "2026-05-07", Content: "10분 회의"},
		{ID: "different-owner", OwnerID: "owner-2", StartDate: "2026-05-07", EndDate: "2026-05-07", Content: "10분 회의"},
		{ID: "different-start", OwnerID: "owner-1", StartDate: "2026-05-08", EndDate: "2026-05-07", Content: "10분 회의"},
		{ID: "different-end", OwnerID: "owner-1", StartDate: "2026-05-07", EndDate: "2026-05-08", Content: "10분 회의"},
	}

	matches := flowTasksWithMatchingOwnerAndDates(tasks, flowTask{
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
	emptyPolicyServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write([]byte(`{"people":[]}`))
	}))
	defer emptyPolicyServer.Close()
	service := NewService(Configuration{BlueclawBaseURL: emptyPolicyServer.URL})
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

func TestBuildFlowMetricsCountsCompletedDistanceAndKeepsLegacyScoreAliases(t *testing.T) {
	metrics := buildFlowMetrics([]flowTask{
		{
			ParticipantNames: []string{"김철수", "이영희"},
			Business:         "여명거리",
			Type:             "기능",
			Size:             "M",
			Status:           "완료",
			EndDate:          "2026-06-02",
		},
		{
			ParticipantNames: []string{"김철수"},
			Business:         "여명거리",
			Type:             "개선",
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

func TestBuildFlowMemberScoresUsesSpreadsheetWeights(t *testing.T) {
	members := []flowMember{
		{ID: "member-a", Name: "김철수"},
		{ID: "member-b", Name: "이영희"},
	}
	definitions := flowDefinitions{
		Sizes: []flowSizeDefinition{
			{Name: "S", DistanceKM: 2},
			{Name: "M", DistanceKM: 5},
		},
	}
	weekStart := time.Date(2026, time.June, 1, 0, 0, 0, 0, flowDateLocation())
	scoreDetails := buildFlowMemberScoreDetails([]flowTask{
		{
			ParticipantIDs:   []string{"member-a"},
			ParticipantNames: []string{"김철수"},
			Size:             "S",
			Status:           "완료",
			EndDate:          "2026-06-01",
		},
		{
			ParticipantIDs:   []string{"member-b"},
			ParticipantNames: []string{"이영희"},
			Size:             "M",
			Status:           "진행",
			EndDate:          "2026-06-01",
		},
	}, members, definitions, weekStart)
	scores := currentFlowMemberScores(scoreDetails)

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

func TestWeightedFlowScoreUsesRecentFivePeriodBaseline(t *testing.T) {
	score := weightedFlowScore([]int{4, 3, 2, 1, 0})

	if score < 107.69 || score > 107.70 {
		t.Fatalf("score = %.2f, want 107.69", score)
	}
}

func TestBuildFlowMemberScoresKeepsDuplicateNamesSeparate(t *testing.T) {
	members := []flowMember{
		{ID: "member-a", Name: "김철수"},
		{ID: "member-b", Name: "김철수"},
	}
	definitions := flowDefinitions{
		Sizes: []flowSizeDefinition{
			{Name: "S", DistanceKM: 2},
		},
	}
	weekStart := time.Date(2026, time.June, 1, 0, 0, 0, 0, flowDateLocation())
	scores := buildFlowMemberScores([]flowTask{
		{
			ParticipantIDs:   []string{"member-a"},
			ParticipantNames: []string{"김철수"},
			Size:             "S",
			Status:           "완료",
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
