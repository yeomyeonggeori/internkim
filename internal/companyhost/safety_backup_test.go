package companyhost

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnAttemptWithNoRecordedSafetyBackupTakesOne(t *testing.T) {
	relied, errorValue := safetyBackupToRelyOn("")

	if errorValue != nil || relied != "" {
		t.Fatalf("an attempt with no recorded safety backup relies on %q: %v", relied, errorValue)
	}
}

func TestAnAttemptWhoseRecordedSafetyBackupIsGoneTakesOne(t *testing.T) {
	relied, errorValue := safetyBackupToRelyOn(filepath.Join(t.TempDir(), "internkim-backup-20261118T090000Z.tar"))

	if errorValue != nil || relied != "" {
		t.Fatalf("an attempt relies on a safety backup that is gone: %q %v", relied, errorValue)
	}
}

func TestAnAttemptWhoseRecordedSafetyBackupIsThereReliesOnIt(t *testing.T) {
	kept := filepath.Join(t.TempDir(), "internkim-backup-20261118T090000Z.tar")
	if errorValue := os.WriteFile(kept, []byte("archive"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	relied, errorValue := safetyBackupToRelyOn(kept)

	if errorValue != nil || relied != kept {
		t.Fatalf("an attempt with %s still there relies on %q: %v", kept, relied, errorValue)
	}
}
