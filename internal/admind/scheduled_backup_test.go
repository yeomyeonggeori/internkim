package admind

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPruneScheduledBackupsKeepsNewestBundles(t *testing.T) {
	backupDirectory := t.TempDir()
	for _, name := range []string{
		"internkim-backup-20260701T000000Z.ikbak",
		"internkim-backup-20260702T000000Z.ikbak",
		"internkim-backup-20260703T000000Z.ikbak",
	} {
		if errorValue := os.WriteFile(filepath.Join(backupDirectory, name), []byte("x"), 0o600); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	pruneScheduledBackups(backupDirectory, 2)
	entries, _ := os.ReadDir(backupDirectory)
	if len(entries) != 2 {
		t.Fatalf("expected 2 bundles, got %d", len(entries))
	}
	if _, errorValue := os.Stat(filepath.Join(backupDirectory, "internkim-backup-20260701T000000Z.ikbak")); !os.IsNotExist(errorValue) {
		t.Fatal("expected the oldest bundle to be pruned")
	}
}

func TestNewestScheduledBackupAgeTriggersWhenEmpty(t *testing.T) {
	if newestScheduledBackupAge(t.TempDir()) < scheduledBackupInterval {
		t.Fatal("empty directory should report a due backup")
	}
}

func TestReadOrCreateBackupPassphrasePersists(t *testing.T) {
	passphrasePath := filepath.Join(t.TempDir(), "secrets", "backup-passphrase")
	first, errorValue := readOrCreateBackupPassphrase(passphrasePath)
	if errorValue != nil || first == "" {
		t.Fatalf("expected passphrase, got %q error %v", first, errorValue)
	}
	second, errorValue := readOrCreateBackupPassphrase(passphrasePath)
	if errorValue != nil || second != first {
		t.Fatalf("expected stable passphrase, got %q vs %q error %v", first, second, errorValue)
	}
	information, _ := os.Stat(passphrasePath)
	if information.Mode().Perm() != 0o600 {
		t.Fatalf("expected 0600 passphrase file, got %v", information.Mode().Perm())
	}
}
