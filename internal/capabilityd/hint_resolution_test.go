package capabilityd

import (
	"context"
	"strings"
	"testing"
)

func TestAHintThatNamesPartOfOneTitleResolvesToIt(t *testing.T) {
	tasks := []flowTaskForTool{
		{ID: "task-1", Content: "8월 18일 상하이 acme 미팅 후속"},
		{ID: "task-2", Content: "주간 결산 확인"},
	}

	task, failure := resolveFlowTaskHint("상하이 acme 미팅", "", tasks)
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

func TestAHintThatMatchesNothingSaysNothingMatchedAndNamesNothing(t *testing.T) {
	tasks := []flowTaskForTool{{ID: "task-1", Content: "휴가"}}

	_, failure := resolveFlowTaskHint("zzzz", "", tasks)
	if failure == nil {
		t.Fatal("expected a hint naming nothing to fail")
	}
	if !strings.Contains(failure.Message, "no task matched") {
		t.Fatalf("the agent has to learn the target is absent, got %q", failure.Message)
	}
	if len(failure.Candidates) != 0 {
		t.Fatalf("a task nothing was asked about is not a candidate, got %+v", failure.Candidates)
	}
}

func TestACalendarHintNamingPartOfOneEventResolvesToIt(t *testing.T) {
	events := []calendarEventForTool{
		{EventID: "event-1", Title: "8월 18일 상하이 acme 미팅"},
		{EventID: "event-2", Title: "휴가"},
	}

	event, failure := resolveCalendarEventHint("상하이 acme 미팅", "", events)
	if failure != nil {
		t.Fatalf("expected the only event naming the meeting to resolve, got failure = %+v", failure)
	}
	if event.EventID != "event-1" {
		t.Fatalf("resolved the wrong event: %q", event.EventID)
	}
}

func TestACalendarHintThatMatchesNothingSaysNothingMatched(t *testing.T) {
	events := []calendarEventForTool{{EventID: "event-1", Title: "휴가"}}

	_, failure := resolveCalendarEventHint("상하이 acme 미팅", "", events)
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

func TestATitleRememberedInPiecesIsProposedRatherThanGuessed(t *testing.T) {
	events := []calendarEventForTool{
		{EventID: "event-1", Title: "포틀랜드 출장 준비"},
		{EventID: "event-2", Title: "휴가"},
	}

	_, failure := resolveCalendarEventHint("포틀랜드 미팅", "", events)
	if failure == nil {
		t.Fatal("a title nothing contains must not resolve on its own")
	}
	if len(failure.Candidates) != 1 || failure.Candidates[0].EventID != "event-1" {
		t.Fatalf("the closest title has to be the one proposed, got %+v", failure.Candidates)
	}
	if !strings.Contains(failure.Message, "ask the user") || !strings.Contains(failure.Message, "none of them") {
		t.Fatalf("an approximation is the user's to confirm, got %q", failure.Message)
	}
}

func TestAnEventIDOneCharacterOffIsNeverProposed(t *testing.T) {
	events := []calendarEventForTool{{EventID: "dbc8fd43324b5771000a6639142d8dd6", Title: "휴가"}}

	_, failure := resolveCalendarEventHint("dbc8fd43324b5771000a6639142d8dd7", "", events)
	if failure == nil {
		t.Fatal("an identifier that is not an identifier must not resolve")
	}
	if len(failure.Candidates) != 0 {
		t.Fatalf("an identifier a character off was invented, not mistyped, got %+v", failure.Candidates)
	}
}

func TestAMistypedNameIsProposedRatherThanGuessed(t *testing.T) {
	members := []flowMemberForTool{
		{ID: "person-1", Name: "김예시", Email: "kimyesi@example.com", MattermostUsername: "kimyesi"},
		{ID: "person-2", Name: "박예시", Email: "parkyesi@example.com", MattermostUsername: "parkyesi"},
	}

	resolution := serviceWithDirectoryOf(t, members).resolveFlowOwnerHint(context.Background(), "김여영", members)
	if resolution.Failure == nil {
		t.Fatal("a name nobody has must not resolve on its own")
	}
	if resolution.Failure.ErrorCode != "flow_owner_approximate" {
		t.Fatalf("a name a character off is a typo, got %q", resolution.Failure.ErrorCode)
	}
	if len(resolution.Failure.Candidates) != 1 || resolution.Failure.Candidates[0].Name != "김예시" {
		t.Fatalf("only the near name belongs in the question, got %+v", resolution.Failure.Candidates)
	}
}

func TestAMistypedEmailDomainIsProposedAndAWrongLocalPartIsNot(t *testing.T) {
	members := []flowMemberForTool{
		{ID: "person-1", Name: "김예시", Email: "kimyesi@example.com"},
		{ID: "person-2", Name: "박예시", Email: "parkyesi@example.com"},
	}

	domainSlip := serviceWithDirectoryOf(t, members).resolveFlowOwnerHint(context.Background(), "iam@dawn.kin", members)
	if domainSlip.Failure == nil || domainSlip.Failure.ErrorCode != "flow_owner_approximate" {
		t.Fatalf("a domain everyone shares is a slip, got %+v", domainSlip.Failure)
	}
	if len(domainSlip.Failure.Candidates) != 1 || domainSlip.Failure.Candidates[0].Email != "kimyesi@example.com" {
		t.Fatalf("the local part says whose address it is, got %+v", domainSlip.Failure.Candidates)
	}

	otherPerson := serviceWithDirectoryOf(t, members).resolveFlowOwnerHint(context.Background(), "parkyesi@example.com", members)
	if otherPerson.Failure != nil || otherPerson.OwnerID != "person-2" {
		t.Fatalf("an address that exists is that person, got %+v", otherPerson)
	}
}

func TestAnExactNameOutranksEveryApproximation(t *testing.T) {
	members := []flowMemberForTool{
		{ID: "person-1", Name: "이샘플", Email: "sample@example.com"},
		{ID: "person-2", Name: "이샘풀", Email: "pool@example.com"},
	}

	resolution := serviceWithDirectoryOf(t, members).resolveFlowOwnerHint(context.Background(), "이샘플", members)
	if resolution.Failure != nil || resolution.OwnerID != "person-1" {
		t.Fatalf("an exact name is taken as given, got %+v", resolution)
	}
}
