package admind

import "testing"

func TestPreferExplicitTaskValue(t *testing.T) {
	title := preferExplicitTaskValue(" 고객지원 분기 결산 누락 항목 확인 ", "분기 결산 누락 항목 확인")
	if title != "고객지원 분기 결산 누락 항목 확인" {
		t.Fatalf("title = %q", title)
	}
	endDate := preferExplicitTaskValue(" 2026-07-18 ", "2026-07-17")
	if endDate != "2026-07-18" {
		t.Fatalf("endDate = %q", endDate)
	}
}

func TestPreferExplicitTaskValueFallsBackToInference(t *testing.T) {
	value := preferExplicitTaskValue("  ", "추론 값")
	if value != "추론 값" {
		t.Fatalf("value = %q", value)
	}
}

func TestQuickTaskOwnerFollowsWhoTheNoteNames(t *testing.T) {
	requester := taskMember{ID: "lee", Email: "lee@example.com"}
	colleague := taskMember{ID: "kim", Email: "kim@example.com"}
	members := []taskMember{requester, colleague}

	if owner := quickTaskOwner(nil, members, requester); owner.ID != "lee" {
		t.Fatalf("a note naming nobody stays the requester's, got %q", owner.ID)
	}
	if owner := quickTaskOwner([]string{"kim", "lee"}, members, requester); owner.ID != "lee" {
		t.Fatalf("a note including the requester stays theirs, got %q", owner.ID)
	}
	if owner := quickTaskOwner([]string{"kim"}, members, requester); owner.ID != "kim" {
		t.Fatalf("a note naming only others hands the work to the first named, got %q", owner.ID)
	}
	if !shouldForceQuickTaskRequest(colleague, "lee@example.com") {
		t.Fatal("handing the work to someone else must take the request form")
	}
	if shouldForceQuickTaskRequest(requester, "lee@example.com") {
		t.Fatal("keeping the work must not take the request form")
	}
}
