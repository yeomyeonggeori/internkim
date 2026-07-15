package admind

import (
	"testing"
	"time"
)

func TestFlowSummaryDependencyKeysForWeek(t *testing.T) {
	weekStart := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	keys := flowSummaryDependencyKeysForWeek("26W28", weekStart)
	if keys.RequestedWeek != (flowSummarySourceKey{Kind: flowSummarySourceWeek, Key: "26W28"}) {
		t.Fatalf("requested week = %+v", keys.RequestedWeek)
	}
	if keys.PreviousWeek.Key != "26W27" || keys.CurrentMonth.Key != "2026-07" || keys.PreviousMonth.Key != "2026-06" {
		t.Fatalf("dependency keys = %+v", keys)
	}
	if keys.Definitions != (flowSummarySourceKey{Kind: flowSummarySourceDefinitions, Key: "global"}) {
		t.Fatalf("definitions = %+v", keys.Definitions)
	}
}

func TestFlowSummaryMemberFingerprintIgnoresOrderAndPresentationFields(t *testing.T) {
	left, errorValue := flowSummaryMemberFingerprint([]flowMember{
		{ID: "two", Name: "Two", Email: "two@example.com", Role: "admin"},
		{ID: "one", Name: "One", Email: "one@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	right, errorValue := flowSummaryMemberFingerprint([]flowMember{
		{ID: "one", Name: "One", Email: "changed@example.com", Role: "member"},
		{ID: "two", Name: "Two", Email: "two@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if left != right {
		t.Fatalf("fingerprints differ: %q != %q", left, right)
	}
}

func TestFlowSummaryMemberFingerprintChangesWithAlignedIdentity(t *testing.T) {
	left, errorValue := flowSummaryMemberFingerprint([]flowMember{{ID: "one", Name: "One"}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	right, errorValue := flowSummaryMemberFingerprint([]flowMember{{ID: "one", Name: "Changed"}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if left == right {
		t.Fatalf("fingerprints matched: %q", left)
	}
}
