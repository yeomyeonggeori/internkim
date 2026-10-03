package admind

import "testing"

func TestCleanTaskStatusTranslatesOnlyStoredValues(t *testing.T) {
	stored := map[string]string{
		"요청":   taskStatusRequested,
		"예정":   taskStatusPlanned,
		"진행":   taskStatusInProgress,
		"완료":   taskStatusCompleted,
		"일시정지": taskStatusPaused,
		"기각":   taskStatusRejected,
		"중단":   taskStatusStopped,
	}
	for input, want := range stored {
		if got := cleanTaskStatus(input); got != want {
			t.Errorf("cleanTaskStatus(%q) = %q, want %q", input, got, want)
		}
	}
	for _, status := range taskStatusOptions() {
		if cleanTaskStatus(status) != status {
			t.Errorf("canonical status %q must pass through unchanged", status)
		}
		if !isAllowedTaskStatus(status) {
			t.Errorf("canonical status %q should be allowed", status)
		}
	}
	if isAllowedTaskStatus("doing") {
		t.Error("a natural-language variant is the model's to interpret, never this map's")
	}
}
