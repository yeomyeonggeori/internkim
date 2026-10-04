package browser

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestBrowserProfileIsWritableByItsProcessUser(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("changing the browser process user requires root")
	}
	directory := browserPermissionDirectory(t)
	stateDirectory := filepath.Join(directory, "state")
	membersDirectory := filepath.Join(stateDirectory, "members")
	if errorValue := os.MkdirAll(membersDirectory, 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	memberDirectory := filepath.Join(membersDirectory, "sample")
	executablePath := filepath.Join(directory, "browser")
	script := "#!/bin/sh\nwhile [ $# -gt 0 ]; do\n  if [ \"$1\" = --profile-dir ]; then shift; profile=$1; fi\n  shift\ndone\nprintf 'profile writable' > \"$profile/probe\" || exit 13\nprintf 'profile writable\\n' >&2\nexit 23\n"
	if errorValue := os.WriteFile(executablePath, []byte(script), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue := LaunchDeviceBrowserProcess(context.Background(), DeviceBrowserLaunch{
		ExecutablePath: executablePath, Port: availableBrowserPort(t), MemberDirectory: memberDirectory,
		ProfileDirectory: filepath.Join(memberDirectory, "profile"), CacheDirectory: filepath.Join(memberDirectory, "cache"),
		LogPath: filepath.Join(memberDirectory, "moli.log"), UserName: "nobody",
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "exit status 23; profile writable") {
		t.Fatalf("browser could not write its profile after changing users: %v", errorValue)
	}
	owner, errorValue := deviceBrowserOwnerOf("nobody")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	assertBrowserPathOwner(t, stateDirectory, owner.userID)
	assertBrowserPathOwner(t, membersDirectory, owner.userID)
	assertBrowserPathOwner(t, filepath.Join(memberDirectory, "profile", "probe"), owner.userID)
	information, errorValue := os.Stat(membersDirectory)
	if errorValue != nil || information.Mode().Perm() != 0o700 {
		t.Fatalf("members directory lost its private permissions: %v, %v", information, errorValue)
	}
}

func browserPermissionDirectory(t *testing.T) string {
	t.Helper()
	directory, errorValue := os.MkdirTemp("", "internkim-browser-permission-")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	if errorValue := os.Chmod(directory, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	return directory
}

func assertBrowserPathOwner(t *testing.T, path string, expectedUserID uint32) {
	t.Helper()
	information, errorValue := os.Stat(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	attributes, ok := information.Sys().(*syscall.Stat_t)
	if !ok || attributes.Uid != expectedUserID {
		t.Fatalf("%s is not owned by browser user %d: %v", path, expectedUserID, information.Sys())
	}
}
