package admind

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
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

func TestRemovingFilesToleratesTheOnesAlreadyGone(t *testing.T) {
	present := filepath.Join(t.TempDir(), "present.conf")
	if errorValue := os.WriteFile(present, nil, 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}

	errorValue := removeFilesIfPresent([]string{present, present + ".absent"})

	if errorValue != nil {
		t.Fatalf("a file already gone is the common case: %v", errorValue)
	}
	if _, statError := os.Stat(present); !errors.Is(statError, fs.ErrNotExist) {
		t.Fatalf("the present file was left behind: %v", statError)
	}
}
