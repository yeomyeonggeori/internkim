package setup

import (
	"errors"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/locallm"
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
		"ask.confirm",
		"capability_tool_names",
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
		"loop,ro,noload",
		"/workspace/.blueclaw/runtime/current/bin/blueclaw",
		"rootfs-init-missing-marker",
		"rootfs-blueclaw-init-root-launch",
		"rootfs-passwd-missing-blueclaw-user",
		"rootfs-group-missing-blueclaw-group",
		"rootfs-mount-failed",
		"blueclaw-payload-launch",
		"rootfs-marp-missing",
		"rootfs-bun-missing",
		"rootfs-bunx-missing",
		"rootfs-uv-missing",
		"rootfs-builtin-skills-python-missing",
		"rootfs-bc-missing",
		"rootfs-chromium-missing",
		"rootfs-posix-helper-missing",
		"rootfs-posix-helper-mode-drift",
		"rootfs-posix-helper-fs-capability-missing",
		"capabilities",
		"blueclaw-posix-helper-preflight",
		"posix helper is not executable by blueclaw",
		"blueclaw-posix-sync",
	} {
		if !strings.Contains(command, expectedFragment) {
			t.Fatalf("expected rootfs binary contract check to contain %q", expectedFragment)
		}
	}
}

func TestFirstAdminHealthCheckUsesCanonicalStatePath(t *testing.T) {
	context := &Context{SSH: stringMatchingBoardConnection{}}
	failedChecks := []string{}

	checkFirstAdminBootstrap(context, &failedChecks)

	if len(failedChecks) != 0 {
		t.Fatalf("expected canonical bootstrap path check to pass, got %+v", failedChecks)
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

func TestSimulationServicesDoNotInstallLlamaCppUnits(t *testing.T) {
	command := serviceUnitInstallCommand(&Context{BoardType: BoardSimulation})

	for _, unexpectedValue := range []string{
		locallm.LlamaCppServiceName,
		locallm.LlamaCppEmbeddingServiceName,
		locallm.LlamaCppBinaryPath,
		locallm.LlamaCppEmbeddingModelPath,
	} {
		if strings.Contains(command, unexpectedValue) {
			t.Fatalf("expected simulation service command to exclude %q, got:\n%s", unexpectedValue, command)
		}
	}
}

func TestJetsonServicesInstallLlamaCppUnits(t *testing.T) {
	command := serviceUnitInstallCommand(&Context{BoardType: BoardJetsonOrinNano})

	for _, expectedValue := range []string{
		locallm.LlamaCppServiceName,
		locallm.LlamaCppEmbeddingServiceName,
		locallm.LlamaCppBinaryPath,
		locallm.LlamaCppEmbeddingModelPath,
	} {
		if !strings.Contains(command, expectedValue) {
			t.Fatalf("expected Jetson service command to include %q, got:\n%s", expectedValue, command)
		}
	}
}

func TestServiceHealthReportChecksAllServicesInOneCommand(t *testing.T) {
	command := blueclawServiceHealthReportCommand(&Context{BoardType: BoardJetsonOrinNano})

	for _, expectedValue := range []string{
		"printf '%s=' 'blueclaw'",
		"systemctl is-active blueclaw",
		"printf '%s=' 'capabilityd'",
		"systemctl is-active internkim-capabilityd",
		"printf '%s=' 'admind'",
		"systemctl is-active internkim-admind",
		"printf '%s=' 'blueclawHealth'",
		"curl --max-time 5 -fsS http://127.0.0.1:8080/admin/api/health",
		"printf '%s=' 'capabilitydHealth'",
		"curl --max-time 5 -fsS --unix-socket /run/internkim/capability.sock",
		"printf '%s=' 'embedding'",
		locallm.LlamaCppEmbeddingServiceName,
	} {
		if !strings.Contains(command, expectedValue) {
			t.Fatalf("expected health report command to include %q, got:\n%s", expectedValue, command)
		}
	}
}

type serviceHealthFailureBoardConnection struct{}

type stringMatchingBoardConnection struct{}

func (connection stringMatchingBoardConnection) Run(command string) string {
	if strings.Contains(command, "/root/.internkim/state/admin/first-admin-bootstrap.json") {
		return "ok"
	}
	return ""
}

func (connection stringMatchingBoardConnection) SCP(localPath, remotePath string) error {
	return nil
}

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
