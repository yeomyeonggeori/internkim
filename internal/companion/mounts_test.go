package companion

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMountStorePersistsWithoutExposingLocalPathInSnapshot(t *testing.T) {
	rootPath := t.TempDir()
	storePath := filepath.Join(t.TempDir(), "mounts.json")
	store := NewMountStore(storePath)

	mount, errorValue := store.Create(rootPath, "Client Work")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.HasPrefix(mount.GuestPath, "/workspace/mounts/Client-Work-") {
		t.Fatalf("guest path = %q", mount.GuestPath)
	}
	if strings.Contains(mount.GuestPath, rootPath) {
		t.Fatalf("guest path exposed local path: %q", mount.GuestPath)
	}

	reloadedStore := NewMountStore(storePath)
	mounts := reloadedStore.List()
	if len(mounts) != 1 || mounts[0].MountID != mount.MountID {
		t.Fatalf("expected persisted mount, got %+v", mounts)
	}
}

func TestMountStoreReadsAndWritesInsideRoot(t *testing.T) {
	rootPath := t.TempDir()
	store := NewMountStore("")
	mount, errorValue := store.Create(rootPath, "work")
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if _, errorValue := store.WriteFile(mount.MountID, "notes/report.txt", []byte("hello")); errorValue != nil {
		t.Fatal(errorValue)
	}
	result, errorValue := store.ReadFile(mount.MountID, "notes/report.txt")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if result["contentBase64"] != base64.StdEncoding.EncodeToString([]byte("hello")) {
		t.Fatalf("unexpected read result: %+v", result)
	}
}

func TestMountStoreRejectsSymlinkEscape(t *testing.T) {
	rootPath := t.TempDir()
	outsidePath := t.TempDir()
	if errorValue := os.WriteFile(filepath.Join(outsidePath, "secret.txt"), []byte("secret"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.Symlink(outsidePath, filepath.Join(rootPath, "outside")); errorValue != nil {
		t.Skipf("symlink unavailable: %v", errorValue)
	}
	store := NewMountStore("")
	mount, errorValue := store.Create(rootPath, "work")
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if _, errorValue := store.ReadFile(mount.MountID, "outside/secret.txt"); errorValue == nil {
		t.Fatal("expected symlink escape to fail")
	}
}

func TestMountStorePauseBlocksFileOperationsAndResumeRestoresThem(t *testing.T) {
	rootPath := t.TempDir()
	store := NewMountStore("")
	mount, errorValue := store.Create(rootPath, "work")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := store.Pause(mount.MountID); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := store.WriteFile(mount.MountID, "blocked.txt", []byte("no")); errorValue == nil {
		t.Fatal("expected paused mount write to fail")
	}
	if _, errorValue := store.Resume(mount.MountID); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := store.WriteFile(mount.MountID, "allowed.txt", []byte("yes")); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestMountStoreLimitsModeChanges(t *testing.T) {
	rootPath := t.TempDir()
	store := NewMountStore("")
	mount, errorValue := store.Create(rootPath, "work")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := store.WriteFile(mount.MountID, "script.sh", []byte("echo ok")); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := store.ChangeMode(mount.MountID, "script.sh", 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := store.ChangeMode(mount.MountID, "script.sh", 0o777); errorValue == nil {
		t.Fatal("expected broad mode to fail")
	}
}
