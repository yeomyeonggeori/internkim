package admind

import (
	"strings"
	"testing"

	blueclawruntime "gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func TestAChatdReleaseWritesTheUnitSetupWouldWrite(t *testing.T) {
	relayPublicURL := "wss://relay.example.test"

	unit, isKnown := releaseChatdUnit(relayPublicURL)

	if !isKnown {
		t.Fatal("a recorded relay address was treated as unknown")
	}
	if unit != blueclawruntime.ChatdServiceUnit(relayPublicURL) {
		t.Fatalf("the release writes a chatd unit setup would not:\n%s", unit)
	}
	if !strings.Contains(unit, "Environment=CHATD_STATE_DIRECTORY="+blueclawruntime.ChatdStateDirectoryPath) {
		t.Fatalf("the released unit gives chatd no state directory:\n%s", unit)
	}
}

func TestAChatdReleaseKeepsTheUnitWhenNoRelayAddressIsRecorded(t *testing.T) {
	if _, isKnown := releaseChatdUnit("  "); isKnown {
		t.Fatal("with no relay address the release would point chatd at the loopback relay instead of leaving the provisioned unit")
	}
}
