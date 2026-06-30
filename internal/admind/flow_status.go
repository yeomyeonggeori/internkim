package admind

import "strings"

const (
	flowStatusRequested  = "요청"
	flowStatusPlanned    = "예정"
	flowStatusInProgress = "진행"
	flowStatusCompleted  = "완료"
	flowStatusPaused     = "일시정지"
	flowStatusRejected   = "기각"
	flowStatusStopped    = "중단"
)

func defaultFlowStatus() string {
	return flowStatusPlanned
}

func flowStatusOptions() []string {
	return []string{
		flowStatusRequested,
		flowStatusPlanned,
		flowStatusInProgress,
		flowStatusCompleted,
		flowStatusPaused,
		flowStatusRejected,
		flowStatusStopped,
	}
}

func cleanFlowStatus(status string) string {
	trimmed := strings.TrimSpace(status)
	switch strings.ToLower(strings.ReplaceAll(trimmed, " ", "")) {
	case "진행", "진행중", "inprogress", "in_progress", "doing", "started", "active":
		return flowStatusInProgress
	case "예정", "planned", "todo", "scheduled", "upcoming":
		return flowStatusPlanned
	case "완료", "done", "completed", "complete", "finished":
		return flowStatusCompleted
	case "요청", "requested", "request":
		return flowStatusRequested
	case "일시정지", "보류", "paused", "pause", "onhold", "hold":
		return flowStatusPaused
	case "기각", "rejected", "reject", "denied":
		return flowStatusRejected
	case "중단", "취소", "stopped", "stop", "cancelled", "canceled", "abandoned":
		return flowStatusStopped
	}
	return trimmed
}

func isAllowedFlowStatus(status string) bool {
	return containsString(flowStatusOptions(), cleanFlowStatus(status))
}

func isFlowRequestedStatus(status string) bool {
	return cleanFlowStatus(status) == flowStatusRequested
}

func isFlowInProgressStatus(status string) bool {
	return cleanFlowStatus(status) == flowStatusInProgress
}

func isFlowCompletedStatus(status string) bool {
	return cleanFlowStatus(status) == flowStatusCompleted
}

func isFlowPlannedStatus(status string) bool {
	return cleanFlowStatus(status) == flowStatusPlanned
}

func isFlowPausedStatus(status string) bool {
	return cleanFlowStatus(status) == flowStatusPaused
}

func isFlowRejectedStatus(status string) bool {
	return cleanFlowStatus(status) == flowStatusRejected
}

func isFlowStoppedStatus(status string) bool {
	return cleanFlowStatus(status) == flowStatusStopped
}

func isFlowInactiveStatus(status string) bool {
	return isFlowRejectedStatus(status) || isFlowStoppedStatus(status)
}
