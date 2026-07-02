package blueclaw

import (
	"os"
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
		"/workspace/.blueclaw/runtime/current/bin/blueclaw",
		"mount -o remount,rw /",
		"configure_outbound_network",
		"ip addr replace 172.31.0.2/30 dev eth0",
		"ip route replace default via 172.31.0.1 dev eth0",
		"nameserver 1.1.1.1",
		"blueclaw-posix-helper sync",
		"check_blueclaw_posix_helper",
		"posix helper is not executable by blueclaw",
		"su -s /bin/bash blueclaw -c \"$blueclaw_binary",
		"su -s /bin/bash blueclaw -c 'INTERNKIM_CAPABILITY_ENDPOINT=",
	} {
		if !strings.Contains(document, expectedFragment) {
			t.Fatalf("expected guest init to contain %q", expectedFragment)
		}
	}
	if strings.Contains(document, "\n/usr/local/bin/blueclaw -runtime ") {
		t.Fatal("expected guest init not to launch blueclaw as root")
	}
}

func TestGuestInitCreatesResourceFirstWorkspaceLayout(t *testing.T) {
	document := readGuestInit(t)
	for _, expectedFragment := range []string{
		"/workspace/circles/staff",
		"/workspace/circles/c-level",
		"/workspace/circles/representative",
		"/workspace/circles/admin",
		"/workspace/circles/hr-compensation",
		"/workspace/private/people",
		"/workspace/shared/public",
		"chown -R blueclaw:blueclaw",
		"/workspace/circles",
		"/workspace/private",
		"/workspace/shared",
		"chmod 0711 /workspace/circles /workspace/private /workspace/private/people",
		"chmod 0755 /workspace/shared /workspace/shared/public",
		"shared/cache/dependencies",
		"shared/cache/dependencies/bun",
		"seed_blueclaw_bun_cache",
		"/opt/blueclaw/bun-cache",
	} {
		if !strings.Contains(document, expectedFragment) {
			t.Fatalf("expected guest init to contain %q", expectedFragment)
		}
	}
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
