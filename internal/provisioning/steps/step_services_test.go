package setup

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
	"gitlab.com/eastriver/internkim/internal/runtime/locallm"
)

func TestServiceUnitDocumentsOmitsLlamaEnvironmentWhenLocalLLMIsNotPlanned(t *testing.T) {
	context := &Context{Backend: BackendSSH, BoardType: BoardJetsonOrinNano, PlannedSteps: map[string]bool{}}
	for _, service := range serviceUnitDocuments(context) {
		if service.path != blueclaw.LLMDServicePath {
			continue
		}
		for _, unexpectedValue := range []string{
			"Environment=BLUECLAW_LLMD_LLAMA_BASE_URL=",
			"Environment=BLUECLAW_LLMD_LLAMA_MODEL=",
			"Environment=BLUECLAW_LLMD_LLAMA_STRUCTURED_OUTPUTS_ENABLED=",
		} {
			if strings.Contains(service.document, unexpectedValue) {
				t.Fatalf("expected LLMD unit without a planned local-llm step to omit %q, got %s", unexpectedValue, service.document)
			}
		}
		return
	}
	t.Fatal("expected LLMD service unit to be present")
}

func TestServiceUnitDocumentsEmitsLlamaEnvironmentWhenLocalLLMIsPlanned(t *testing.T) {
	context := &Context{Backend: BackendSSH, BoardType: BoardJetsonOrinNano, PlannedSteps: map[string]bool{"local-llm": true}}
	for _, service := range serviceUnitDocuments(context) {
		if service.path != blueclaw.LLMDServicePath {
			continue
		}
		for _, expectedValue := range []string{
			"Environment=BLUECLAW_LLMD_LLAMA_BASE_URL=",
			"Environment=BLUECLAW_LLMD_LLAMA_MODEL=",
			"Environment=BLUECLAW_LLMD_LLAMA_STRUCTURED_OUTPUTS_ENABLED=true",
		} {
			if !strings.Contains(service.document, expectedValue) {
				t.Fatalf("expected LLMD unit with a planned local-llm step to contain %q, got %s", expectedValue, service.document)
			}
		}
		return
	}
	t.Fatal("expected LLMD service unit to be present")
}

func TestBlueclawRuntimeContractCheckCatchesStaleAgentConfiguration(t *testing.T) {
	command := blueclawRuntimeContractCheckCommand()
	for _, expectedFragment := range []string{
		"defaultBudgetClass",
		"defaultTaskLevel",
		"firecrackerGuest",
		"runtime-capability-google-tool",
		"runtime-config-mirror-drift",
		"runtime-outbound-network-disabled",
		"runtime-outbound-network-cidr",
	} {
		if !strings.Contains(command, expectedFragment) {
			t.Fatalf("expected runtime contract check to contain %q", expectedFragment)
		}
	}
	for _, staleFragment := range []string{"runtime-profile-missing-tools", "runtime-profile-google-tool", "mandatory_profile_tools"} {
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
		PlannedSteps: map[string]bool{"local-llm": true, "mattermost": true},
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
				"llmd=active",
				"admind=active",
				"graphiti=inactive",
				"blueclawHealth=ok",
				"capabilitydHealth=ok",
				"llmdHealth=ok",
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
				"llmd=active",
				"admind=active",
				"blueclawHealth=ok",
				"capabilitydHealth=ok",
				"llmdHealth=ok",
			}, "\n"),
		},
	}

	if !blueclawServicesAreHealthy(context) {
		t.Fatal("expected graphiti to be skipped without local LLM")
	}
}

func TestBlueclawServicesHealthSkipsMattermostCompositeHealthWithoutMattermost(t *testing.T) {
	context := &Context{
		BoardType:    BoardJetsonOrinNano,
		PlannedSteps: map[string]bool{},
		SSH: serviceHealthReportBoardConnection{
			report: strings.Join([]string{
				"blueclaw=active",
				"capabilityd=active",
				"llmd=active",
				"admind=active",
				"blueclawHealth=ok",
				"llmdHealth=ok",
			}, "\n"),
		},
	}

	if !blueclawServicesAreHealthy(context) {
		t.Fatal("expected Mattermost composite health to be skipped without Mattermost")
	}
}

func TestBlueclawServicesHealthRequiresMattermostCompositeHealthWhenPlanned(t *testing.T) {
	context := &Context{
		BoardType:    BoardJetsonOrinNano,
		PlannedSteps: map[string]bool{"mattermost": true},
		SSH: serviceHealthReportBoardConnection{
			report: strings.Join([]string{
				"blueclaw=active",
				"capabilityd=active",
				"llmd=active",
				"admind=active",
				"blueclawHealth=ok",
				"capabilitydHealth=no",
				"llmdHealth=ok",
			}, "\n"),
		},
	}

	if blueclawServicesAreHealthy(context) {
		t.Fatal("expected planned Mattermost composite health failure to fail service health")
	}
}

func TestServiceHealthReportSkipsMattermostCompositeHealthWithoutMattermost(t *testing.T) {
	command := blueclawServiceHealthReportCommand(&Context{
		BoardType:    BoardJetsonOrinNano,
		PlannedSteps: map[string]bool{},
	})

	if strings.Contains(command, "capabilitydHealth") {
		t.Fatalf("expected Mattermost composite health check to be omitted, got:\n%s", command)
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
	case strings.Contains(command, "systemctl is-active "+blueclaw.LLMDServiceName):
		return "active"
	case strings.Contains(command, blueclaw.LLMDHealthCheckCommand()):
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
