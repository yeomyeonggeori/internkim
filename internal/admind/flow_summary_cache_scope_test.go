package admind

import (
	"reflect"
	"testing"
	"time"
)

func TestFlowTaskSummarySourceKeysUsesWeekAndEndpointMonths(t *testing.T) {
	task := flowTask{ID: "task-1", WeekCode: "26W28", StartDate: "2026-06-30", EndDate: "2026-07-02"}
	keys, errorValue := flowTaskSummarySourceKeys(task)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	want := []flowSummarySourceKey{
		{Kind: flowSummarySourceMonth, Key: "2026-06"},
		{Kind: flowSummarySourceMonth, Key: "2026-07"},
		{Kind: flowSummarySourceWeek, Key: "26W28"},
	}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %+v, want %+v", keys, want)
	}
}

func TestFlowTaskSummarySourceKeysRejectsInvalidScope(t *testing.T) {
	for _, task := range []flowTask{
		{ID: "bad-week", WeekCode: "week", StartDate: "2026-07-01"},
		{ID: "bad-date", WeekCode: "26W28", StartDate: "July"},
	} {
		if _, errorValue := flowTaskSummarySourceKeys(task); errorValue == nil {
			t.Fatalf("task %+v did not fail", task)
		}
	}
}

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
