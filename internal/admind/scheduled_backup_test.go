package admind

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPruneBackupDirectoryReclaimsDisk(t *testing.T) {
	backupDirectory := t.TempDir()
	write := func(name string, age time.Duration) {
		full := filepath.Join(backupDirectory, name)
		if errorValue := os.WriteFile(full, []byte("x"), 0o600); errorValue != nil {
			t.Fatal(errorValue)
		}
		if age > 0 {
			old := time.Now().Add(-age)
			_ = os.Chtimes(full, old, old)
		}
	}
	// encrypted bundles: keep newest 2
	write("internkim-backup-20260701T000000Z.ikbak", 0)
	write("internkim-backup-20260702T000000Z.ikbak", 0)
	write("internkim-backup-20260703T000000Z.ikbak", 0)
	// buzz sql snapshots: keep newest 2
	write("buzz-20260701T000000Z.sql", 0)
	write("buzz-20260702T000000Z.sql", 0)
	write("buzz-20260703T000000Z.sql", 0)
	// stale one-off deploy snapshot: always removed
	write("preserved-predeploy-20260101T000000Z.tar.gz", 0)
	// plain leftover from a failed encrypt: removed only when older than an hour
	write(".internkim-backup-20260704T000000Z.tar.gz", 2*time.Hour)
	// a plain leftover that may be in progress: kept
	write(".internkim-backup-20260705T000000Z.tar.gz", time.Minute)

	pruneBackupDirectory(backupDirectory, 2)

	exists := func(name string) bool {
		_, errorValue := os.Stat(filepath.Join(backupDirectory, name))
		return errorValue == nil
	}
	if exists("internkim-backup-20260701T000000Z.ikbak") {
		t.Fatal("oldest .ikbak should be pruned")
	}
	if !exists("internkim-backup-20260703T000000Z.ikbak") || !exists("internkim-backup-20260702T000000Z.ikbak") {
		t.Fatal("newest two .ikbak should be kept")
	}
	if exists("buzz-20260701T000000Z.sql") {
		t.Fatal("oldest buzz snapshot should be pruned")
	}
	if exists("preserved-predeploy-20260101T000000Z.tar.gz") {
		t.Fatal("stale preserved deploy snapshot should be removed")
	}
	if exists(".internkim-backup-20260704T000000Z.tar.gz") {
		t.Fatal("old plain leftover should be removed")
	}
	if !exists(".internkim-backup-20260705T000000Z.tar.gz") {
		t.Fatal("recent plain leftover (possibly in-progress) should be kept")
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
