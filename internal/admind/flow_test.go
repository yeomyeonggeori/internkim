package admind

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
