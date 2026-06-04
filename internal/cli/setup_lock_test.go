package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAcquireSetupLockFailsFastWithExistingRun(t *testing.T) {
	stateDirectory := t.TempDir()
	firstHandle, errorValue := acquireSetupLock(stateDirectory, setupLockDocument{
		Command:       "internkim setup --only admind",
		SelectedSteps: "--only=admind",
		TargetHost:    "0.ssh.example.com",
		TargetURL:     "https://device.example",
	}, false)
	if errorValue != nil {
		t.Fatalf("first lock failed: %v", errorValue)
	}
	defer firstHandle.release()

	_, errorValue = acquireSetupLock(stateDirectory, setupLockDocument{}, false)
	if errorValue == nil {
		t.Fatal("expected second lock to fail")
	}
	for _, expectedText := range []string{"setup is already running", "--only=admind", "0.ssh.example.com", "https://device.example"} {
		if !strings.Contains(errorValue.Error(), expectedText) {
			t.Fatalf("expected lock error to contain %q, got %v", expectedText, errorValue)
		}
	}
}

func TestAcquireSetupLockRemovesStalePID(t *testing.T) {
	stateDirectory := t.TempDir()
	lockPath := filepath.Join(stateDirectory, setupLockFileName)
	writeSetupLockForTest(t, lockPath, `{"pid":999999,"command":"old setup","startedAt":"2026-01-01T00:00:00Z"}`)

	handle, errorValue := acquireSetupLock(stateDirectory, setupLockDocument{Command: "new setup"}, false)
	if errorValue != nil {
		t.Fatalf("expected stale lock to be removed: %v", errorValue)
	}
	defer handle.release()

	document, errorValue := readSetupLockDocument(lockPath)
	if errorValue != nil {
		t.Fatalf("read lock: %v", errorValue)
	}
	if document.Command != "new setup" || document.StartedAt.IsZero() || time.Since(document.StartedAt) > time.Minute {
		t.Fatalf("unexpected replacement lock document: %+v", document)
	}
}

func writeSetupLockForTest(t *testing.T, path string, content string) {
	t.Helper()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(path, []byte(content), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
}
