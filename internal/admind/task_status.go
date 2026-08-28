package admind

import "strings"

const (
	taskStatusRequested  = "요청"
	taskStatusPlanned    = "예정"
	taskStatusInProgress = "진행"
	taskStatusCompleted  = "완료"
	taskStatusPaused     = "일시정지"
	taskStatusRejected   = "기각"
	taskStatusStopped    = "중단"
)

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

func cleanTaskStatus(status string) string {
	trimmed := strings.TrimSpace(status)
	switch strings.ToLower(strings.ReplaceAll(trimmed, " ", "")) {
	case "진행", "진행중", "inprogress", "in_progress", "doing", "started", "active":
		return taskStatusInProgress
	case "예정", "planned", "todo", "scheduled", "upcoming":
		return taskStatusPlanned
	case "완료", "done", "completed", "complete", "finished":
		return taskStatusCompleted
	case "요청", "requested", "request":
		return taskStatusRequested
	case "일시정지", "보류", "paused", "pause", "onhold", "hold":
		return taskStatusPaused
	case "기각", "rejected", "reject", "denied":
		return taskStatusRejected
	case "중단", "취소", "stopped", "stop", "cancelled", "canceled", "abandoned":
		return taskStatusStopped
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
