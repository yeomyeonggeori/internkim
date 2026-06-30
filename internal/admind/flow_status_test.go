package admind

import "testing"

func TestCleanFlowStatusNormalizesNaturalVariants(t *testing.T) {
	cases := map[string]string{
		"진행 중":       flowStatusInProgress,
		"진행중":        flowStatusInProgress,
		"in progress": flowStatusInProgress,
		"in_progress": flowStatusInProgress,
		"진행":         flowStatusInProgress,
		"완료":         flowStatusCompleted,
		"done":       flowStatusCompleted,
		"예정":         flowStatusPlanned,
		"planned":    flowStatusPlanned,
		"보류":         flowStatusPaused,
		"취소":         flowStatusStopped,
		"cancelled":  flowStatusStopped,
	}
	for input, want := range cases {
		if got := cleanFlowStatus(input); got != want {
			t.Errorf("cleanFlowStatus(%q) = %q, want %q", input, got, want)
		}
	}
	for _, status := range flowStatusOptions() {
		if !isAllowedFlowStatus(status) {
			t.Errorf("canonical status %q should be allowed", status)
		}
	}
	if !isAllowedFlowStatus("진행 중") {
		t.Error("natural variant '진행 중' should normalize to an allowed status")
	}
}
