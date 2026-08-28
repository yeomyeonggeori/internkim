package admind

import "strings"

const (
	taskStatusRequested  = "requested"
	taskStatusPlanned    = "planned"
	taskStatusInProgress = "in_progress"
	taskStatusCompleted  = "completed"
	taskStatusPaused     = "paused"
	taskStatusRejected   = "rejected"
	taskStatusStopped    = "cancelled"
)

var koreanTaskStatusLabels = map[string]string{
	taskStatusRequested:  "요청",
	taskStatusPlanned:    "예정",
	taskStatusInProgress: "진행",
	taskStatusCompleted:  "완료",
	taskStatusPaused:     "일시정지",
	taskStatusRejected:   "기각",
	taskStatusStopped:    "중단",
}

func taskStatusLabel(status string) string {
	if label, known := koreanTaskStatusLabels[cleanTaskStatus(status)]; known {
		return label
	}
	return status
}

func defaultTaskStatus() string {
	return taskStatusPlanned
}

func taskStatusOptions() []string {
	return []string{
		taskStatusRequested,
		taskStatusPlanned,
		taskStatusInProgress,
		taskStatusCompleted,
		taskStatusPaused,
		taskStatusRejected,
		taskStatusStopped,
	}
}

// The exact enum values this store wrote before the English canonicalization.
// These are stored machine identifiers being translated, not text being
// interpreted; anything fuzzier than an exact stored value is the model's to
// judge, and the schemas it answers through only admit the canonical values.
var canonicalStatusOfStoredValue = map[string]string{
	"요청":   taskStatusRequested,
	"예정":   taskStatusPlanned,
	"진행":   taskStatusInProgress,
	"완료":   taskStatusCompleted,
	"일시정지": taskStatusPaused,
	"기각":   taskStatusRejected,
	"중단":   taskStatusStopped,
}

func cleanTaskStatus(status string) string {
	trimmed := strings.TrimSpace(status)
	if canonical, wasStored := canonicalStatusOfStoredValue[trimmed]; wasStored {
		return canonical
	}
	return trimmed
}

func isAllowedTaskStatus(status string) bool {
	return containsString(taskStatusOptions(), cleanTaskStatus(status))
}

func isTaskRequestedStatus(status string) bool {
	return cleanTaskStatus(status) == taskStatusRequested
}

func isTaskInProgressStatus(status string) bool {
	return cleanTaskStatus(status) == taskStatusInProgress
}

func isTaskCompletedStatus(status string) bool {
	return cleanTaskStatus(status) == taskStatusCompleted
}

func isTaskPlannedStatus(status string) bool {
	return cleanTaskStatus(status) == taskStatusPlanned
}

func isTaskPausedStatus(status string) bool {
	return cleanTaskStatus(status) == taskStatusPaused
}

func isTaskRejectedStatus(status string) bool {
	return cleanTaskStatus(status) == taskStatusRejected
}

func isTaskStoppedStatus(status string) bool {
	return cleanTaskStatus(status) == taskStatusStopped
}

func isTaskInactiveStatus(status string) bool {
	return isTaskRejectedStatus(status) || isTaskStoppedStatus(status)
}
