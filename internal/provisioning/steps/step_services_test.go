package setup

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
	"github.com/yeomyeonggeori/internkim/internal/runtime/locallm"
)

func TestBlueclawRuntimeContractCheckCatchesStaleAgentConfiguration(t *testing.T) {
	command := blueclawRuntimeContractCheckCommand()
	for _, expectedFragment := range []string{
		"defaultBudgetClass",
		"defaultTaskLevel",
		"virtualMachineGuest",
		"runtime-config-mirror-drift",
		"runtime-outbound-network-disabled",
		"runtime-outbound-network-cidr",
	} {
		if !strings.Contains(command, expectedFragment) {
			t.Fatalf("expected runtime contract check to contain %q", expectedFragment)
		}
	}
	for _, staleFragment := range []string{"runtime-profile-missing-tools", "mandatory_profile_tools"} {
		if strings.Contains(command, staleFragment) {
			t.Fatalf("expected runtime contract check to omit stale profile validation %q", staleFragment)
		}
	}
}

func TestBlueclawRuntimeContractCheckRejectsLegacyRuntimeFields(t *testing.T) {
	legacyPaths := [][]string{
		{"agent", "defaultBudgetClass"},
		{"languageModel", "backend"},
		{"languageModel", "capability", "backend"},
		{"capabilities", "backend"},
		{"languageModel", "openRouter", "apiKeyPath"},
		{"languageModel", "liteRTLM", "wrapperPath"},
		{"languageModel", "liteRTLM", "modelPath"},
		{"languageModel", "liteRTLM", "backend"},
		{"connectors", "mattermost", "botTokenPath"},
		{"connectors", "slack", "botTokenPath"},
		{"connectors", "slack", "signingSecretPath"},
	}

	for _, legacyPath := range legacyPaths {
		t.Run(strings.Join(legacyPath, "."), func(t *testing.T) {
			runtimeConfiguration := validRuntimeConfiguration(t)
			setRuntimeString(runtimeConfiguration, legacyPath, "legacy")
			if result := runRuntimeContractCheck(t, runtimeConfiguration); result != "legacy-runtime-config" {
				t.Fatalf("expected legacy runtime field %s to fail, got %q", strings.Join(legacyPath, "."), result)
			}
		})
	}
}

func TestBlueclawRuntimeContractCheckAllowsBackendSchemaProperties(t *testing.T) {
	runtimeConfiguration := validRuntimeConfiguration(t)
	capabilities := runtimeConfiguration["capabilities"].(map[string]any)
	toolDescriptors := capabilities["toolDescriptors"].([]any)
	toolDescriptors = append(toolDescriptors, map[string]any{
		"namespace": "llm",
		"outputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"backend": map[string]any{"type": "string"},
			},
		},
		"resultContract": map[string]any{
			"schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"backend": map[string]any{"type": "string"},
				},
			},
		},
	})
	capabilities["toolDescriptors"] = toolDescriptors

	if result := runRuntimeContractCheck(t, runtimeConfiguration); result != "ok" {
		t.Fatalf("expected schema properties named backend to pass, got %q", result)
	}
}

func TestBlueclawRuntimeContractCheckRejectsNonStringLegacyRuntimeField(t *testing.T) {
	runtimeConfiguration := validRuntimeConfiguration(t)
	capabilities := runtimeConfiguration["capabilities"].(map[string]any)
	capabilities["backend"] = map[string]any{"type": "string"}

	if result := runRuntimeContractCheck(t, runtimeConfiguration); result != "legacy-runtime-config" {
		t.Fatalf("expected non-string legacy runtime field to fail, got %q", result)
	}
}

func validRuntimeConfiguration(t *testing.T) map[string]any {
	t.Helper()

	document, errorValue := blueclaw.BlueclawRuntimeConfigDocumentWithOptions(blueclaw.RuntimeConfigOptions{})
	if errorValue != nil {
		t.Fatalf("generate runtime configuration: %v", errorValue)
	}
	runtimeConfiguration := map[string]any{}
	if errorValue := json.Unmarshal([]byte(document), &runtimeConfiguration); errorValue != nil {
		t.Fatalf("decode runtime configuration: %v", errorValue)
	}
	return runtimeConfiguration
}

func setRuntimeString(runtimeConfiguration map[string]any, path []string, value string) {
	parent := runtimeConfiguration
	for _, key := range path[:len(path)-1] {
		nested, isFound := parent[key].(map[string]any)
		if !isFound {
			nested = map[string]any{}
			parent[key] = nested
		}
		parent = nested
	}
	parent[path[len(path)-1]] = value
}

func runRuntimeContractCheck(t *testing.T, runtimeConfiguration map[string]any) string {
	t.Helper()

	document, errorValue := json.Marshal(runtimeConfiguration)
	if errorValue != nil {
		t.Fatalf("encode runtime configuration: %v", errorValue)
	}
	runtimePath := filepath.Join(t.TempDir(), "runtime.json")
	workspaceRuntimePath := filepath.Join(t.TempDir(), "workspace-runtime.json")
	for _, path := range []string{runtimePath, workspaceRuntimePath} {
		if errorValue := os.WriteFile(path, document, 0o600); errorValue != nil {
			t.Fatalf("write runtime configuration: %v", errorValue)
		}
	}
	command := strings.ReplaceAll(blueclawRuntimeContractCheckCommand(), "/root/.blueclaw/config/runtime.json", runtimePath)
	command = strings.ReplaceAll(command, "/root/.blueclaw/workspace/.blueclaw/config/runtime.json", workspaceRuntimePath)
	output, errorValue := exec.Command("sh", "-c", command).CombinedOutput()
	if errorValue != nil {
		t.Fatalf("run runtime contract check: %v: %s", errorValue, output)
	}
	return strings.TrimSpace(string(output))
}

func TestBlueclawRootfsBaseContractCheckCatchesStaleBaseRuntime(t *testing.T) {
	command := blueclawRootfsBaseContractCheckCommand()
	for _, expectedFragment := range []string{
		"/opt/internkim/blueclaw-runtime/rootfs.ext4",
		"loop,ro,noload",
		"/delivery/runtime/current/bin/blueclaw",
		"rootfs-init-missing-marker",
		"rootfs-blueclaw-init-root-launch",
		"rootfs-passwd-missing-blueclaw-user",
		"rootfs-group-missing-blueclaw-group",
		"rootfs-mount-failed",
		"blueclaw-payload-launch",
		"rootfs-bun-missing",
		"rootfs-bunx-missing",
		"rootfs-uv-missing",
		"rootfs-$managed_executable-owner-drift",
		"rootfs-$managed_executable-mode-drift",
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

func TestServicesInstallCommandRemovesLegacySDKDUnit(t *testing.T) {
	command := serviceUnitInstallCommand(&Context{BoardType: BoardJetsonOrinNano})

	for _, expectedValue := range []string{
		"systemctl stop blueclaw-sdkd 2>/dev/null || true",
		"systemctl disable blueclaw-sdkd 2>/dev/null || true",
		"rm -f /etc/systemd/system/blueclaw-sdkd.service",
	} {
		if !strings.Contains(command, expectedValue) {
			t.Fatalf("expected service install command to contain %q, got:\n%s", expectedValue, command)
		}
	}
}

func TestServicesInstallOnlyTheAgentAndModelUnits(t *testing.T) {
	context := &Context{
		BoardType:    BoardJetsonOrinNano,
		PlannedSteps: map[string]bool{"local-llm": true},
	}
	expectedPaths := []string{
		blueclaw.BlueclawServicePath,
		blueclaw.CapabilitydServicePath,
		blueclaw.AdmindServicePath,
		locallm.LlamaCppServicePath,
		locallm.LlamaCppEmbeddingServicePath,
	}

	installedPaths := []string{}
	for _, service := range serviceUnitDocuments(context) {
		installedPaths = append(installedPaths, service.path)
	}
	if strings.Join(installedPaths, "\n") != strings.Join(expectedPaths, "\n") {
		t.Fatalf("expected the memory store to live inside blueclaw with no sidecar unit, got %v", installedPaths)
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
		"printf '%s=' 'blueclawHealth'",
		"curl --max-time 15 -fsS http://127.0.0.1:8080/admin/api/health",
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

func TestBlueclawServicesHealthRequiresEmbeddingWithLocalLLM(t *testing.T) {
	context := &Context{
		BoardType:    BoardJetsonOrinNano,
		PlannedSteps: map[string]bool{"local-llm": true},
		SSH: serviceHealthReportBoardConnection{
			report: strings.Join([]string{
				"blueclaw=active",
				"capabilityd=active",
				"admind=active",
				"blueclawHealth=ok",
				"capabilitydHealth=ok",
				"embedding=inactive",
			}, "\n"),
		},
	}

	if blueclawServicesAreHealthy(context) {
		t.Fatal("expected an inactive embedding server to fail the health check")
	}
}

func TestBlueclawServicesHealthSkipsEmbeddingWithoutLocalLLM(t *testing.T) {
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
		t.Fatal("expected the embedding server to be skipped without local LLM")
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

func TestServicesSatisfiedRequiresCapabilitydToAnswer(t *testing.T) {
	context := &Context{
		Backend: BackendSSH,
		SSH:     capabilitydHealthBoardConnection{capabilitydHealth: "ok"},
	}
	if !StepServices.IsSatisfied(context) {
		t.Fatal("expected services to be satisfied when capabilityd answers")
	}

	silentContext := &Context{
		Backend: BackendSSH,
		SSH:     capabilitydHealthBoardConnection{capabilitydHealth: "no"},
	}
	if StepServices.IsSatisfied(silentContext) {
		t.Fatal("expected services not to be satisfied when capabilityd is active but does not answer")
	}
}

func TestServicesSatisfiedRequiresConfiguredCapabilitydProvidersToBeReady(t *testing.T) {
	context := &Context{
		Backend: BackendSSH,
		SSH:     capabilitydHealthBoardConnection{capabilitydHealth: "unready:chatd"},
	}

	if StepServices.IsSatisfied(context) {
		t.Fatal("expected services not to be satisfied when capabilityd reports an unready configured provider")
	}
}

func TestBlueclawServicesHealthNamesTheUnreadyCapabilitydProvider(t *testing.T) {
	context := &Context{BoardType: BoardJetsonOrinNano}
	report := map[string]string{
		"blueclaw":          "active",
		"capabilityd":       "active",
		"admind":            "active",
		"blueclawHealth":    "ok",
		"capabilitydHealth": "unready:chatd",
	}

	unhealthy := unhealthyServiceHealthEntries(context, report)
	if len(unhealthy) != 1 || unhealthy[0] != "capabilitydHealth=unready:chatd" {
		t.Fatalf("expected the unready provider to be named, got %v", unhealthy)
	}
}

func TestBlueclawServicesHealthRequiresCapabilitydHealth(t *testing.T) {
	context := &Context{
		BoardType: BoardJetsonOrinNano,
		SSH: serviceHealthReportBoardConnection{
			report: strings.Join([]string{
				"blueclaw=active",
				"capabilityd=active",
				"admind=active",
				"blueclawHealth=ok",
				"capabilitydHealth=no",
			}, "\n"),
		},
	}

	if blueclawServicesAreHealthy(context) {
		t.Fatal("expected an unanswering capabilityd to fail service health")
	}
}

func TestServiceHealthReportChecksCapabilitydHealthUnconditionally(t *testing.T) {
	command := blueclawServiceHealthReportCommand(&Context{BoardType: BoardJetsonOrinNano})

	for _, expectedValue := range []string{
		"printf '%s=' 'capabilitydHealth'",
		"curl --max-time 5 -fsS --unix-socket " + blueclaw.CapabilitySocketPath,
	} {
		if !strings.Contains(command, expectedValue) {
			t.Fatalf("expected health report command to include %q, got:\n%s", expectedValue, command)
		}
	}
}

type capabilitydHealthBoardConnection struct {
	capabilitydHealth string
}

func (connection capabilitydHealthBoardConnection) Run(command string) string {
	switch {
	case strings.Contains(command, "runtime_path ="):
		return "ok"
	case strings.Contains(command, "rootfs_path="):
		return "ok"
	case strings.Contains(command, blueclaw.RetiredLLMDServiceIsGoneCommand()):
		return "inactive"
	case strings.Contains(command, "cat "+blueclaw.CapabilitydServicePath):
		return "ExecStart=" + blueclaw.CapabilitydBinaryPath + " --chatd-endpoint " + blueclaw.ChatdEndpoint + " --chatd-platform buzz"
	case strings.Contains(command, blueclaw.CapabilitySocketPath):
		return connection.capabilitydHealth
	case strings.Contains(command, blueclaw.BlueclawHealthCheckURL()):
		return "ok"
	case strings.HasPrefix(command, "systemctl is-active "):
		return "active"
	default:
		return ""
	}
}

func (connection capabilitydHealthBoardConnection) SCP(localPath, remotePath string) error {
	return nil
}

type serviceHealthFailureBoardConnection struct{}

type serviceHealthReportBoardConnection struct {
	report string
}

type stringMatchingBoardConnection struct{}

func (connection serviceHealthReportBoardConnection) Run(command string) string {
	return connection.report
}

func (connection serviceHealthReportBoardConnection) SCP(localPath, remotePath string) error {
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
	case strings.Contains(command, "curl -fsS "):
		return "missing"
	default:
		return ""
	}
}

func (connection serviceHealthFailureBoardConnection) SCP(localPath, remotePath string) error {
	return nil
}

func TestNoServiceUnitInstallsTheRetiredLLMD(t *testing.T) {
	for _, boardType := range []string{BoardJetsonOrinNano, BoardSimulation} {
		context := &Context{Backend: BackendSSH, BoardType: boardType, PlannedSteps: map[string]bool{"local-llm": true}}
		for _, service := range serviceUnitDocuments(context) {
			if strings.Contains(service.path, "blueclaw-llmd") {
				t.Fatalf("%s still installs a unit at %s", boardType, service.path)
			}
		}
		for _, serviceName := range enabledServiceNames(context) {
			if serviceName == "blueclaw-llmd" {
				t.Fatalf("%s still enables blueclaw-llmd", boardType)
			}
		}
	}
}

func TestRetirementStopsDisablesAndRemovesLLMD(t *testing.T) {
	command := blueclaw.RetireLLMDLeftByEarlierReleasesCommand()
	for _, expectedFragment := range []string{
		"systemctl disable --now blueclaw-llmd",
		"rm -f /etc/systemd/system/blueclaw-llmd.service",
		"rm -f /usr/local/bin/blueclaw-llmd",
		"systemctl daemon-reload",
	} {
		if !strings.Contains(command, expectedFragment) {
			t.Fatalf("a deploy that leaves llmd running is the failure this guards; %q is missing from:\n%s", expectedFragment, command)
		}
	}
}
