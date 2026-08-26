package admind

import "strings"

// The device's words for a status and the central plane's. The canonical list is
// host/relay/flow-task-as-task.ts, which the relay uses for the same crossing;
// TestTheStatusWordsMatchTheRelay reads it and fails when the two drift.
// The company app names these too, and TestTheStatusWordsAgreeWithTheCompanyApp
// reads its list rather than trusting this one to have kept up.
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

func centralFlowNote(task flowTask) string {
	return ""
}

func flowMemberIdentifier(record adminUserMutation) string {
	return stableFlowID(strings.ToLower(strings.TrimSpace(record.Email)))
}

// One word each way, so the reverse is the inverse and there is no second map to
// keep in step by hand.
var deviceStatusOfCentralStatus = invertStatusWords()

func invertStatusWords() map[string]string {
	inverted := map[string]string{}
	for deviceStatus, centralStatus := range centralStatusOfDeviceStatus {
		inverted[centralStatus] = deviceStatus
	}
	return inverted
}

func deviceFlowStatus(centralStatus string) string {
	if deviceStatus, known := deviceStatusOfCentralStatus[strings.TrimSpace(centralStatus)]; known {
		return deviceStatus
	}
	return "예정"
}
