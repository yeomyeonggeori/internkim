package admind

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// The relay still translates the Korean statuses of historic device exports;
// cleanTaskStatus must read those the same way, or one border crossing keeps a
// meaning the other lost.
func TestTheStatusWordsMatchTheRelay(t *testing.T) {
	source, errorValue := os.ReadFile(filepath.FromSlash("../../host/relay/task-as-task.ts"))
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
		if cleanTaskStatus(deviceStatus) != centralStatus {
			t.Fatalf("the relay reads %s as %s and this reads it as %s; two crossings of the same border have to agree",
				deviceStatus, centralStatus, cleanTaskStatus(deviceStatus))
		}
	}
}
