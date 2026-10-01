package blueclaw

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGuestInitRunsBlueclawAsNonRootUser(t *testing.T) {
	document := readGuestInit(t)
	for _, expectedFragment := range []string{
		"ensure_blueclaw_account",
		"missing blueclaw user in rootfs",
		"missing blueclaw group in rootfs",
		"chown blueclaw:blueclaw /workspace /workspace/.blueclaw",
		"chown -R blueclaw:blueclaw",
		"chown -R postgres:postgres /workspace/.blueclaw/postgres",
		"chown postgres:blueclaw /workspace/.blueclaw/postgres",
		"/workspace/.blueclaw/tmp",
		"blueclaw_binary_path",
		"/delivery/runtime/current/bin/blueclaw",
		"mount -o remount,rw /",
		"configure_outbound_network",
		"ip addr replace 172.31.0.2/30 dev eth0",
		"ip route replace default via 172.31.0.1 dev eth0",
		"nameserver 1.1.1.1",
		"blueclaw-posix-helper sync",
		"check_blueclaw_posix_helper",
		"posix helper is not executable by blueclaw",
		"su -s /bin/bash blueclaw -c \"BLUECLAW_BUNDLED_SKILLS_PATH=/delivery/skills $blueclaw_binary",
		"-runtime /delivery/config/runtime.json",
	} {
		if !strings.Contains(document, expectedFragment) {
			t.Fatalf("expected guest init to contain %q", expectedFragment)
		}
	}
	if strings.Contains(document, "\n/usr/local/bin/blueclaw -runtime ") {
		t.Fatal("expected guest init not to launch blueclaw as root")
	}
}

func TestGuestInitDelegatesWorkspaceLayoutToPOSIXPolicy(t *testing.T) {
	document := readGuestInit(t)
	for _, expectedFragment := range []string{
		"blueclaw-posix-helper sync",
		"--policy /delivery/config/policy.json",
		"--workspace /workspace",
	} {
		if !strings.Contains(document, expectedFragment) {
			t.Fatalf("expected guest init to contain %q", expectedFragment)
		}
	}
}

func TestGuestInitPreservesUserWorkspaceOwnership(t *testing.T) {
	script := `set -eu
ensure_workspace_directory() { printf 'mkdir %s\n' "$*"; }
rmdir() { :; }
chmod() { printf 'chmod %s\n' "$*"; }
find() { printf 'find %s\n' "$*"; }
chown() { printf 'chown %s\n' "$*"; }
`
	script += guestInitFunction(t, "prepare_blueclaw_workspace") + "\nprepare_blueclaw_workspace\n"
	output, errorValue := exec.Command("bash", "-c", script).CombinedOutput()
	if errorValue != nil {
		t.Fatalf("prepare workspace: %v\n%s", errorValue, output)
	}
	for _, operation := range strings.Split(string(output), "\n") {
		for _, argument := range strings.Fields(operation) {
			if !strings.HasPrefix(argument, "/workspace") {
				continue
			}
			if argument == "/workspace" && !strings.Contains(operation, " -R ") {
				continue
			}
			if argument == "/workspace/.blueclaw" || strings.HasPrefix(argument, "/workspace/.blueclaw/") {
				continue
			}
			t.Errorf("boot must leave user workspace management to POSIX policy: %s", operation)
		}
	}
}

func TestGuestInitStopsWhenPOSIXPolicySynchronizationFails(t *testing.T) {
	workspacePath := t.TempDir()
	if errorValue := os.MkdirAll(filepath.Join(workspacePath, ".blueclaw", "logs"), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	function := strings.ReplaceAll(guestInitFunction(t, "sync_blueclaw_posix_policy"), "/workspace", workspacePath)
	script := "set -eu\n/usr/local/bin/blueclaw-posix-helper() { return 23; }\nlog_guest_init() { printf '%s\\n' \"$*\"; }\n"
	script += function + "\nsync_blueclaw_posix_policy\nprintf 'continued boot\\n'\n"
	output, errorValue := exec.Command("bash", "-c", script).CombinedOutput()
	if errorValue == nil || strings.Contains(string(output), "continued boot") {
		t.Fatalf("boot continued after failed POSIX synchronization: %s", output)
	}
	if !strings.Contains(string(output), "posix policy synchronization failed") {
		t.Fatalf("missing actionable boot failure: %s", output)
	}
}

func guestInitFunction(t *testing.T, name string) string {
	t.Helper()
	document := readGuestInit(t)
	start := strings.Index(document, name+"() {")
	if start < 0 {
		t.Fatalf("missing guest-init function %s", name)
	}
	end := strings.Index(document[start:], "\n}\n")
	if end < 0 {
		t.Fatalf("missing guest-init function boundary for %s", name)
	}
	return document[start : start+end+2]
}

func readGuestInit(t *testing.T) string {
	t.Helper()
	_, filePath, _, isOK := runtime.Caller(0)
	if !isOK {
		t.Fatal("expected caller path")
	}
	repositoryRootPath := filepath.Clean(filepath.Join(filepath.Dir(filePath), "..", "..", ".."))
	document, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, "assets", "blueclaw-runtime", "guest-init"))
	if errorValue != nil {
		t.Fatalf("expected guest init fixture: %v", errorValue)
	}
	return string(document)
}
