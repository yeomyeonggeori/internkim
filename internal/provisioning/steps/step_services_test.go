package setup

import (
	"errors"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
	"gitlab.com/eastriver/internkim/internal/runtime/locallm"
)

func TestBlueclawRuntimeContractCheckCatchesStaleAgentConfiguration(t *testing.T) {
	command := blueclawRuntimeContractCheckCommand()
	for _, expectedFragment := range []string{
		"defaultBudgetClass",
		"defaultTaskLevel",
		"firecrackerGuest",
		"runtime-profile-google-tool",
		"runtime-capability-google-tool",
		"file.deliver",
		"ask.confirm",
		"runtime-config-mirror-drift",
		"runtime-outbound-network-disabled",
		"runtime-outbound-network-cidr",
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
		"rootfs-$managed_executable-owner-drift",
		"rootfs-$managed_executable-mode-drift",
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
		"blueclaw-outbound-network",
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

func TestServicesSatisfiedRequiresGraphiti(t *testing.T) {
	context := &Context{
		Backend: BackendSSH,
		SSH:     serviceSatisfiedWithoutGraphitiBoardConnection{},
	}

	if StepServices.IsSatisfied(context) {
		t.Fatal("expected services not to be satisfied when graphiti memoryd is inactive")
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

func TestServicesInstallGraphitiMemoryd(t *testing.T) {
	command := serviceUnitInstallCommand(&Context{
		BoardType:    BoardJetsonOrinNano,
		PlannedSteps: map[string]bool{"local-llm": true},
	})

	for _, expectedValue := range []string{
		blueclaw.GraphitiMemorydServicePath,
		blueclaw.GraphitiMemorydPath,
		"systemctl enable " + blueclaw.GraphitiMemorydServiceName,
		"systemctl restart " + blueclaw.GraphitiMemorydServiceName,
	} {
		if !strings.Contains(command, expectedValue) {
			t.Fatalf("expected service command to include %q, got:\n%s", expectedValue, command)
		}
	}
	if strings.Contains(command, "disable "+blueclaw.GraphitiMemorydServiceName) {
		t.Fatalf("expected service command not to disable graphiti memoryd, got:\n%s", command)
	}
}

func TestServicesSkipGraphitiMemorydWithoutLocalLLM(t *testing.T) {
	command := serviceUnitInstallCommand(&Context{BoardType: BoardJetsonOrinNano})
	for _, unexpectedValue := range []string{
		blueclaw.GraphitiMemorydServicePath,
		blueclaw.GraphitiMemorydPath,
		"systemctl enable " + blueclaw.GraphitiMemorydServiceName,
		"systemctl restart " + blueclaw.GraphitiMemorydServiceName,
	} {
		if strings.Contains(command, unexpectedValue) {
			t.Fatalf("expected service command to exclude %q without local LLM, got:\n%s", unexpectedValue, command)
		}
	}
}

func TestBlueclawHostNetworkDependencyInstallCommandInstallsTapNATTools(t *testing.T) {
	command := blueclawHostNetworkDependencyInstallCommand()
	for _, expectedValue := range []string{"command -v ip", "command -v iptables", "command -v sysctl", "command -v jq", "apt-get install", "iproute2 iptables procps jq"} {
		if !strings.Contains(command, expectedValue) {
			t.Fatalf("expected host network dependency command to contain %q, got:\n%s", expectedValue, command)
		}
	}
}

func TestServicesReconcileMattermostSiteURLOnlyWhenMattermostIsPlanned(t *testing.T) {
	if shouldReconcileMattermostSiteURL(&Context{PlannedSteps: map[string]bool{"local-llm": true}}) {
		t.Fatal("expected non-Mattermost service plan to skip Mattermost SiteURL reconciliation")
	}
	if !shouldReconcileMattermostSiteURL(&Context{PlannedSteps: map[string]bool{"mattermost": true}}) {
		t.Fatal("expected Mattermost plan to reconcile Mattermost SiteURL")
	}
}

func TestMattermostSiteURLReconcileCommandRestartsOnlyWhenURLChanges(t *testing.T) {
	command := mattermostSiteURLReconcileCommand("https://device.example")
	for _, expectedValue := range []string{
		`current_url="$(jq -r '.ServiceSettings.SiteURL // .SiteURL // ""'`,
		`if [ "$current_url" != 'https://device.example' ]; then`,
		`jq --arg siteURL 'https://device.example'`,
		"systemctl restart mattermost",
	} {
		if !strings.Contains(command, expectedValue) {
			t.Fatalf("expected Mattermost SiteURL command to contain %q, got:\n%s", expectedValue, command)
		}
	}
}

func TestJetsonServicesDoNotInstallLlamaCppUnitsByDefault(t *testing.T) {
	command := serviceUnitInstallCommand(&Context{BoardType: BoardJetsonOrinNano})

	for _, unexpectedValue := range []string{
		locallm.LlamaCppServiceName,
		locallm.LlamaCppEmbeddingServiceName,
		locallm.LlamaCppBinaryPath,
		locallm.LlamaCppEmbeddingModelPath,
	} {
		if strings.Contains(command, unexpectedValue) {
			t.Fatalf("expected default Jetson service command to exclude %q, got:\n%s", unexpectedValue, command)
		}
	}
}

func TestJetsonServicesInstallLlamaCppUnitsWhenLocalLLMIsPlanned(t *testing.T) {
	command := serviceUnitInstallCommand(&Context{
		BoardType:    BoardJetsonOrinNano,
		PlannedSteps: map[string]bool{"local-llm": true},
	})

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

func TestCloudSharedServicesUseRemoteInferenceMode(t *testing.T) {
	command := serviceUnitInstallCommand(&Context{BoardType: BoardCloudShared})

	if !strings.Contains(command, "--local-inference-mode remote") {
		t.Fatalf("expected cloud-shared capabilityd service to use remote inference, got:\n%s", command)
	}
	for _, unexpectedValue := range []string{
		locallm.LlamaCppServiceName,
		locallm.LlamaCppEmbeddingServiceName,
		locallm.LlamaCppBinaryPath,
	} {
		if strings.Contains(command, unexpectedValue) {
			t.Fatalf("expected cloud-shared service command to exclude %q, got:\n%s", unexpectedValue, command)
		}
	}
}

func TestServiceHealthReportChecksAllServicesInOneCommand(t *testing.T) {
	command := blueclawServiceHealthReportCommand(&Context{
		BoardType:    BoardJetsonOrinNano,
		PlannedSteps: map[string]bool{"local-llm": true},
	})

	for _, expectedValue := range []string{
		"printf '%s=' 'blueclaw'",
		"systemctl is-active blueclaw",
		"printf '%s=' 'capabilityd'",
		"systemctl is-active internkim-capabilityd",
		"printf '%s=' 'admind'",
		"systemctl is-active internkim-admind",
		"printf '%s=' 'graphiti'",
		"systemctl is-active graphiti-memoryd",
		"printf '%s=' 'blueclawHealth'",
		"curl --max-time 15 -fsS http://127.0.0.1:8080/admin/api/health",
		"printf '%s=' 'capabilitydHealth'",
		"curl --max-time 5 -fsS --unix-socket /run/internkim/capability.sock",
		"printf '%s=' 'graphitiHealth'",
		"curl --max-time 5 -fsS http://127.0.0.1:7791/health | jq -e '.status == \"ok\"'",
		"printf '%s=' 'embedding'",
		locallm.LlamaCppEmbeddingServiceName,
	} {
		if !strings.Contains(command, expectedValue) {
			t.Fatalf("expected health report command to include %q, got:\n%s", expectedValue, command)
		}
	}
}

func TestBlueclawServicesHealthRequiresGraphiti(t *testing.T) {
	context := &Context{
		BoardType:    BoardJetsonOrinNano,
		PlannedSteps: map[string]bool{"local-llm": true},
		SSH: serviceHealthReportBoardConnection{
			report: strings.Join([]string{
				"blueclaw=active",
				"capabilityd=active",
				"admind=active",
				"graphiti=inactive",
				"blueclawHealth=ok",
				"capabilitydHealth=ok",
				"graphitiHealth=no",
			}, "\n"),
		},
	}

	if blueclawServicesAreHealthy(context) {
		t.Fatal("expected graphiti inactive report to fail health check")
	}
}

func TestBlueclawServicesHealthSkipsGraphitiWithoutLocalLLM(t *testing.T) {
	context := &Context{
		BoardType: BoardJetsonOrinNano,
		SSH: serviceHealthReportBoardConnection{
			report: strings.Join([]string{
				"blueclaw=active",
				"capabilityd=active",
				"admind=active",
				"blueclawHealth=ok",
				"capabilitydHealth=ok",
			}, "\n"),
		},
	}

	if !blueclawServicesAreHealthy(context) {
		t.Fatal("expected graphiti to be skipped without local LLM")
	}
}

func TestServiceHealthReportChecksLocalLLMWhenPlanned(t *testing.T) {
	command := blueclawServiceHealthReportCommand(&Context{
		BoardType:    BoardJetsonOrinNano,
		PlannedSteps: map[string]bool{"local-llm": true},
	})

	for _, expectedValue := range []string{
		"printf '%s=' 'embedding'",
		locallm.LlamaCppEmbeddingServiceName,
	} {
		if !strings.Contains(command, expectedValue) {
			t.Fatalf("expected local LLM health report command to include %q, got:\n%s", expectedValue, command)
		}
	}
}

type serviceHealthFailureBoardConnection struct{}

type serviceHealthReportBoardConnection struct {
	report string
}

type serviceSatisfiedWithoutGraphitiBoardConnection struct{}

type stringMatchingBoardConnection struct{}

func (connection serviceHealthReportBoardConnection) Run(command string) string {
	return connection.report
}

func (connection serviceHealthReportBoardConnection) SCP(localPath, remotePath string) error {
	return nil
}

func (connection serviceSatisfiedWithoutGraphitiBoardConnection) Run(command string) string {
	switch {
	case strings.Contains(command, "runtime_path ="):
		return "ok"
	case strings.Contains(command, "rootfs_path="):
		return "ok"
	case strings.Contains(command, "systemctl is-active graphiti-memoryd"):
		return "inactive"
	case strings.Contains(command, "systemctl is-active blueclaw"):
		return "active"
	case strings.Contains(command, "systemctl is-active internkim-capabilityd"):
		return "active"
	case strings.Contains(command, "systemctl is-active internkim-admind"):
		return "active"
	case strings.Contains(command, "systemctl is-active mattermost"):
		return "active"
	case strings.Contains(command, "curl --max-time 5 -fsS"):
		return "ok"
	default:
		return ""
	}
}

func (connection serviceSatisfiedWithoutGraphitiBoardConnection) SCP(localPath, remotePath string) error {
	return nil
}

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
	case strings.Contains(command, "systemctl is-active graphiti-memoryd"):
		return "inactive"
	case strings.Contains(command, "curl -fsS "):
		return "missing"
	default:
		return ""
	}
}

func (connection serviceHealthFailureBoardConnection) SCP(localPath, remotePath string) error {
	return nil
}
