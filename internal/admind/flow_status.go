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
	return strings.TrimSpace(status)
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
