package capabilityd

import (
	"strings"
	"testing"
)

func TestAHintThatNamesPartOfOneTitleResolvesToIt(t *testing.T) {
	tasks := []flowTaskForTool{
		{ID: "task-1", Content: "8월 18일 상하이 edatec 미팅 후속"},
		{ID: "task-2", Content: "주간 결산 확인"},
	}

	task, failure := resolveFlowTaskHint("상하이 edatec 미팅", "", tasks)
	if failure != nil {
		t.Fatalf("expected the only task naming the meeting to resolve, got failure = %+v", failure)
	}
	if task.ID != "task-1" {
		t.Fatalf("resolved the wrong task: %+v", task)
	}
}

func TestAnExactTitleWinsOverALongerTitleThatContainsIt(t *testing.T) {
	tasks := []flowTaskForTool{
		{ID: "task-1", Content: "결산 확인 후속 정리"},
		{ID: "task-2", Content: "결산 확인"},
	}

	task, failure := resolveFlowTaskHint("결산 확인", "", tasks)
	if failure != nil {
		t.Fatalf("expected the exact title to resolve, got failure = %+v", failure)
	}
	if task.ID != "task-2" {
		t.Fatalf("the exact title must win over the one that merely contains it, got %+v", task)
	}
}

func TestAHintInsideSeveralTitlesOffersOnlyThoseTitles(t *testing.T) {
	tasks := []flowTaskForTool{
		{ID: "task-1", Content: "상하이 미팅 준비"},
		{ID: "task-2", Content: "상하이 미팅 후속"},
		{ID: "task-3", Content: "전혀 다른 업무"},
	}

	_, failure := resolveFlowTaskHint("상하이 미팅", "", tasks)
	if failure == nil {
		t.Fatal("expected two containing titles to stay ambiguous")
	}
	if len(failure.Candidates) != 2 {
		t.Fatalf("candidates must be the titles that matched, got %+v", failure.Candidates)
	}
	if !strings.Contains(failure.Message, "matched more than one") {
		t.Fatalf("an ambiguous hint must say so, got %q", failure.Message)
	}
}

func TestAHintThatMatchesNothingSaysNothingMatched(t *testing.T) {
	tasks := []flowTaskForTool{{ID: "task-1", Content: "휴가"}}

	_, failure := resolveFlowTaskHint("상하이 edatec 미팅", "", tasks)
	if failure == nil {
		t.Fatal("expected a hint naming nothing to fail")
	}
	if !strings.Contains(failure.Message, "no task matched") {
		t.Fatalf("the agent has to learn the target is absent, got %q", failure.Message)
	}
	if len(failure.Candidates) != 1 {
		t.Fatalf("candidates must still show what exists, got %+v", failure.Candidates)
	}
}

func TestACalendarHintNamingPartOfOneEventResolvesToIt(t *testing.T) {
	events := []calendarEventForTool{
		{EventID: "event-1", Title: "8월 18일 상하이 edatec 미팅"},
		{EventID: "event-2", Title: "휴가"},
	}

	eventID, failure := resolveCalendarEventHint("상하이 edatec 미팅", "", events)
	if failure != nil {
		t.Fatalf("expected the only event naming the meeting to resolve, got failure = %+v", failure)
	}
	if eventID != "event-1" {
		t.Fatalf("resolved the wrong event: %q", eventID)
	}
}

func TestACalendarHintThatMatchesNothingSaysNothingMatched(t *testing.T) {
	events := []calendarEventForTool{{EventID: "event-1", Title: "휴가"}}

	_, failure := resolveCalendarEventHint("상하이 edatec 미팅", "", events)
	if failure == nil {
		t.Fatal("expected a hint naming no event to fail")
	}
	if !strings.Contains(failure.Message, "no calendar event matched") {
		t.Fatalf("the agent has to learn the event is absent, got %q", failure.Message)
	}
}

func TestOwnershipStillBreaksATieBetweenIdenticalTitles(t *testing.T) {
	tasks := []flowTaskForTool{
		{ID: "task-1", OwnerID: "someone-else", Content: "결산 확인"},
		{ID: "task-2", OwnerID: "me", Content: "결산 확인"},
	}

	task, failure := resolveFlowTaskHint("결산 확인", "me", tasks)
	if failure != nil || task.ID != "task-2" {
		t.Fatalf("expected the requester-owned task to win, got %+v failure = %+v", task, failure)
	}
}

func TestIdenticalTitlesDoNotFallThroughToASearchOfEveryTitle(t *testing.T) {
	tasks := []flowTaskForTool{
		{ID: "task-1", Content: "결산 확인"},
		{ID: "task-2", Content: "결산 확인"},
		{ID: "task-3", Content: "결산 확인 후속"},
	}

	_, failure := resolveFlowTaskHint("결산 확인", "", tasks)
	if failure == nil {
		t.Fatal("expected two identical titles to stay ambiguous")
	}
	if len(failure.Candidates) != 2 {
		t.Fatalf("an ambiguous exact match offers the exact matches, not every title containing it, got %+v", failure.Candidates)
	}
}
