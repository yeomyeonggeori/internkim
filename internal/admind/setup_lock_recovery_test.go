package admind

import (
	"strings"
	"testing"
)

func TestReleaseSetupLockRefusesWhileASetupRuns(t *testing.T) {
	command := releaseSetupLockCommand()

	if !strings.Contains(command, "/var/lock/internkim-setup.lock") {
		t.Fatal("expected the lock setup takes")
	}
	if !strings.Contains(command, `pgrep -af "internkim setup"`) {
		t.Fatal("a lock with a live holder must be left alone, or two setups write the device at once")
	}
	if strings.Index(command, `pgrep -af "internkim setup"`) > strings.Index(command, `rm -rf "$lock_directory"`) {
		t.Fatal("expected the running check before the removal")
	}
	if !strings.Contains(command, "cat \"$lock_directory/metadata.json\"") {
		t.Fatal("expected the lock's own record of who took it to be reported before it is cleared")
	}
}
