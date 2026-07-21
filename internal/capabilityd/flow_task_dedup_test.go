package capabilityd

import "testing"

func TestMergeFlowTaskAddInputIntoDuplicateAppliesNewValues(t *testing.T) {
	duplicate := flowTaskForTool{ID: "t1", Content: "결산 확인", EndDate: "", Goal: "기존 목표"}

	merged, hasNewValues := mergeFlowTaskAddInputIntoDuplicate(duplicate, flowTaskAddInput{Title: "결산 확인", EndDate: "2026-07-24"})

	if !hasNewValues || merged.EndDate != "2026-07-24" || merged.Goal != "기존 목표" {
		t.Fatalf("expected the new endDate to merge into the duplicate, got %+v hasNewValues=%v", merged, hasNewValues)
	}

	_, hasNewValues = mergeFlowTaskAddInputIntoDuplicate(merged, flowTaskAddInput{Title: "결산 확인", EndDate: "2026-07-24"})
	if hasNewValues {
		t.Fatal("expected an identical re-add to report no new values")
	}
}
