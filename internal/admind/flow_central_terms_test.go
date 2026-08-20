package admind

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestTheStatusWordsMatchTheRelay(t *testing.T) {
	source, errorValue := os.ReadFile(filepath.FromSlash("../../host/relay/flow-task-as-task.ts"))
	if errorValue != nil {
		t.Skipf("the relay's copy is unavailable: %v", errorValue)
	}
	declaration := regexp.MustCompile(`(?s)const statusOfDevice: Record<string, string> = \{(.*?)\}`).FindSubmatch(source)
	if declaration == nil {
		t.Fatal("the relay no longer declares the statuses this reads")
	}

	relayStatuses := map[string]string{}
	for _, pair := range regexp.MustCompile(`(\S+): '([a-z_]+)'`).FindAllSubmatch(declaration[1], -1) {
		relayStatuses[string(pair[1])] = string(pair[2])
	}
	if len(relayStatuses) == 0 {
		t.Fatal("the relay's statuses parsed empty")
	}

	for deviceStatus, centralStatus := range relayStatuses {
		if centralFlowStatus(deviceStatus) != centralStatus {
			t.Fatalf("the relay sends %s as %s and this sends it as %s; two crossings of the same border have to agree",
				deviceStatus, centralStatus, centralFlowStatus(deviceStatus))
		}
	}
	for deviceStatus := range centralStatusOfDeviceStatus {
		if _, known := relayStatuses[deviceStatus]; !known {
			t.Fatalf("%s is mapped here and nowhere in the relay", deviceStatus)
		}
	}
}

func TestAGoalAndAReasonBecomeOneNote(t *testing.T) {
	note := centralFlowNote(flowTask{Goal: "출시", RequestReason: "고객 요청"})

	if note != "목표: 출시\n고객 요청" {
		t.Fatalf("note = %q", note)
	}
}

func TestATaskWithNeitherHasNoNote(t *testing.T) {
	if note := centralFlowNote(flowTask{}); note != "" {
		t.Fatalf("an empty note is stored as nothing, not as blank lines: %q", note)
	}
}
