package admind

import "testing"

func TestCleanTaskStatusNormalizesNaturalVariants(t *testing.T) {
	cases := map[string]string{
		"진행 중":        taskStatusInProgress,
		"진행중":         taskStatusInProgress,
		"in progress": taskStatusInProgress,
		"in_progress": taskStatusInProgress,
		"진행":          taskStatusInProgress,
		"완료":          taskStatusCompleted,
		"done":        taskStatusCompleted,
		"예정":          taskStatusPlanned,
		"planned":     taskStatusPlanned,
		"보류":          taskStatusPaused,
		"취소":          taskStatusStopped,
		"cancelled":   taskStatusStopped,
	}
	for input, want := range cases {
		if got := cleanTaskStatus(input); got != want {
			t.Errorf("cleanTaskStatus(%q) = %q, want %q", input, got, want)
		}
	}
	for _, status := range taskStatusOptions() {
		if !isAllowedTaskStatus(status) {
			t.Errorf("canonical status %q should be allowed", status)
		}
	}
	if !isAllowedTaskStatus("진행 중") {
		t.Error("natural variant '진행 중' should normalize to an allowed status")
	}
}
