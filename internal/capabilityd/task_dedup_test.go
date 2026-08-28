package capabilityd

import "testing"

func TestMergeTaskAddInputIntoDuplicateAppliesNewValues(t *testing.T) {
	duplicate := taskForTool{ID: "t1", Content: "결산 확인", EndDate: ""}

	merged, hasNewValues := mergeTaskAddInputIntoDuplicate(duplicate, taskAddInput{Title: "결산 확인", EndsAt: "2026-07-24"})

	if !hasNewValues || merged.EndDate != "2026-07-24" {
		t.Fatalf("expected the new endDate to merge into the duplicate, got %+v hasNewValues=%v", merged, hasNewValues)
	}

	_, hasNewValues = mergeTaskAddInputIntoDuplicate(merged, taskAddInput{Title: "결산 확인", EndsAt: "2026-07-24"})
	if hasNewValues {
		t.Fatal("expected an identical re-add to report no new values")
	}
}

func TestResolveTaskHintPrefersRequesterOwnedTitleMatch(t *testing.T) {
	tasks := []taskForTool{
		{ID: "t1", OwnerID: "other", Content: "결산 확인"},
		{ID: "t2", OwnerID: "me", Content: "결산 확인"},
	}

	task, failure := resolveTaskHint("결산 확인", "me", tasks)
	if failure != nil || task.ID != "t2" {
		t.Fatalf("expected the requester-owned task to resolve, got %+v failure=%+v", task, failure)
	}

	_, failure = resolveTaskHint("결산 확인", "", tasks)
	if failure == nil {
		t.Fatal("expected ambiguity without a requester scope")
	}

	ambiguous := append(tasks, taskForTool{ID: "t3", OwnerID: "me", Content: "결산 확인"})
	_, failure = resolveTaskHint("결산 확인", "me", ambiguous)
	if failure == nil {
		t.Fatal("expected ambiguity when the requester owns multiple matches")
	}
}
