package admind

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func fileInode(t *testing.T, path string) uint64 {
	t.Helper()
	info, errorValue := os.Stat(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	status, isUnix := info.Sys().(*syscall.Stat_t)
	if !isUnix {
		t.Skip("inode numbers are not available here")
	}
	return uint64(status.Ino)
}

// A reader on the other side of the share opens this file by name while it is
// being replaced. Truncating it in place is what let that reader see half a
// document; a rename never does, because the old file stays whole until the new
// one takes its name.
func TestAShorterRosterReplacesTheFileRatherThanTruncatingIt(t *testing.T) {
	policyPath := filepath.Join(t.TempDir(), "policy.json")
	if errorValue := replaceWholeFile(policyPath, []byte(`{"people":["a","b","c"]}`)); errorValue != nil {
		t.Fatal(errorValue)
	}
	firstInode := fileInode(t, policyPath)

	if errorValue := replaceWholeFile(policyPath, []byte(`{"people":[]}`)); errorValue != nil {
		t.Fatal(errorValue)
	}

	if fileInode(t, policyPath) == firstInode {
		t.Error("the roster was rewritten in the same file, so a reader can still catch it truncated")
	}
	delivered, errorValue := os.ReadFile(policyPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(delivered) != `{"people":[]}` {
		t.Errorf("delivered = %s", delivered)
	}
}

func TestTheDeliveredRosterIsReadableByTheAgentAndLeavesNothingBehind(t *testing.T) {
	directory := t.TempDir()
	policyPath := filepath.Join(directory, "policy.json")
	if errorValue := replaceWholeFile(policyPath, []byte(`{"people":[]}`)); errorValue != nil {
		t.Fatal(errorValue)
	}

	info, errorValue := os.Stat(policyPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if info.Mode().Perm() != deliveredPolicyMode {
		t.Errorf("mode = %v, the agent reads this as a user that is not root", info.Mode().Perm())
	}
	entries, errorValue := os.ReadDir(directory)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(entries) != 1 {
		names := []string{}
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Errorf("directory holds %v; the replacement was left behind", names)
	}
}
