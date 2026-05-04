package setup

import (
	"errors"
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

func TestBlueclawRootfsBaseContractCheckCatchesStaleBaseRuntime(t *testing.T) {
	command := blueclawRootfsBaseContractCheckCommand()
	for _, expectedFragment := range []string{
		"/opt/internkim/blueclaw-runtime/rootfs.ext4",
		"/workspace/.blueclaw/runtime/current/bin/blueclaw",
		"rootfs-init-missing-marker",
		"rootfs-blueclaw-init-root-launch",
		"rootfs-passwd-missing-blueclaw-user",
		"rootfs-group-missing-blueclaw-group",
		"blueclaw-payload-launch",
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

func TestServicesRunReturnsErrorWhenBlueclawHealthFails(t *testing.T) {
	previousAttempts := blueclawServiceHealthAttempts
	previousRetryDelay := blueclawServiceHealthRetryDelay
	blueclawServiceHealthAttempts = 1
	blueclawServiceHealthRetryDelay = 0
	defer func() {
		blueclawServiceHealthAttempts = previousAttempts
		blueclawServiceHealthRetryDelay = previousRetryDelay
	}()

	context := &Context{
		Backend: BackendSSH,
		SSH:     serviceHealthFailureBoardConnection{},
		Callbacks: Callbacks{
			LoadState: func(key string) string {
				return ""
			},
		},
	}

	errorValue := StepServices.Run(context)
	if !errors.Is(errorValue, errBlueclawHealthCheckFailed) {
		t.Fatalf("expected blueclaw health check failure, got %v", errorValue)
	}
}

type serviceHealthFailureBoardConnection struct{}

func (connection serviceHealthFailureBoardConnection) Run(command string) string {
	switch {
	case strings.Contains(command, "runtime_path ="):
		return "ok"
	case strings.Contains(command, "rootfs_path="):
		return "ok"
	case strings.Contains(command, "systemctl is-active blueclaw"):
		return "active"
	case strings.Contains(command, "systemctl is-active internkim-capabilityd"):
		return "active"
	case strings.Contains(command, "systemctl is-active internkim-admind"):
		return "active"
	case strings.Contains(command, "curl -fsS "):
		return "missing"
	default:
		return ""
	}
}

func (connection serviceHealthFailureBoardConnection) SCP(localPath, remotePath string) error {
	return nil
}
