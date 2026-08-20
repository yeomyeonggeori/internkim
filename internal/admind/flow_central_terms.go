package admind

import "strings"

// The device's words for a status and the central plane's. The canonical list is
// host/relay/flow-task-as-task.ts, which the relay uses for the same crossing;
// TestTheStatusWordsMatchTheRelay reads it and fails when the two drift.
var centralStatusOfDeviceStatus = map[string]string{
	"요청":   "requested",
	"예정":   "todo",
	"진행":   "in_progress",
	"완료":   "done",
	"일시정지": "paused",
	"기각":   "rejected",
	"중단":   "cancelled",
}

func centralFlowStatus(deviceStatus string) string {
	if central, known := centralStatusOfDeviceStatus[strings.TrimSpace(deviceStatus)]; known {
		return central
	}
	return "todo"
}

// The central plane keeps one note where the device keeps a goal and a reason for
// asking, so they are joined the way the relay joins them.
func centralFlowNote(task flowTask) string {
	lines := []string{}
	if goal := strings.TrimSpace(task.Goal); goal != "" {
		lines = append(lines, "목표: "+goal)
	}
	if reason := strings.TrimSpace(task.RequestReason); reason != "" {
		lines = append(lines, reason)
	}
	return strings.Join(lines, "\n")
}

func flowMemberIdentifier(record adminUserMutation) string {
	return stableFlowID(strings.ToLower(strings.TrimSpace(record.Email)))
}
