package setup

import (
	"strings"
	"testing"
)

func TestBlueclawRuntimeContractCheckCatchesStaleAgentConfiguration(t *testing.T) {
	command := blueclawRuntimeContractCheckCommand()
	for _, expectedFragment := range []string{
		"defaultBudgetClass",
		"defaultEffortLevel",
		"firecrackerGuest",
		"runtime-profile-google-tool",
		"runtime-capability-google-tool",
		"terminal.session",
		"browser_handoff.openURL",
		"approval.request",
		"runtime-config-mirror-drift",
	} {
		if !strings.Contains(command, expectedFragment) {
			t.Fatalf("expected runtime contract check to contain %q", expectedFragment)
		}
	}
}

func TestBlueclawRootfsBinaryContractCheckCatchesStaleGuestBinary(t *testing.T) {
	command := blueclawRootfsBinaryContractCheckCommand()
	for _, expectedFragment := range []string{
		"/opt/internkim/blueclaw-runtime/rootfs.ext4",
		"/usr/local/bin/blueclaw",
		"defaultEffortLevel",
		"agent.limit_stop",
		"defaultBudgetClass",
		"agent.budget_stop",
		"10분 예산",
		"rootfs-blueclaw-legacy-marker",
		"rootfs-init-missing-marker",
		"rootfs-blueclaw-init-root-launch",
		"rootfs-passwd-missing-blueclaw-user",
		"rootfs-group-missing-blueclaw-group",
		"blueclaw-non-root-launch",
		"rootfs-marp-missing",
		"rootfs-bun-missing",
		"rootfs-bunx-missing",
		"rootfs-chromium-missing",
	} {
		if !strings.Contains(command, expectedFragment) {
			t.Fatalf("expected rootfs binary contract check to contain %q", expectedFragment)
		}
	}
}
