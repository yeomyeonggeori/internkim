package localfleet

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

type recordingLogger struct {
	lines []string
}

func (logger *recordingLogger) Info(message string) {
	logger.lines = append(logger.lines, message)
}

func TestServiceDefaultsToLocalFleetState(t *testing.T) {
	service, errorValue := NewService(Options{
		RepositoryRootPath: "/repo",
		ExecutablePath:     "/repo/internkim",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if service.options.VirtualMachineName != DefaultVirtualMachineName {
		t.Fatalf("virtual machine name = %q", service.options.VirtualMachineName)
	}
	expectedStatePath := filepath.Join("/repo", ".local", "local-fleet")
	if service.options.StateRootPath != expectedStatePath {
		t.Fatalf("state path = %q", service.options.StateRootPath)
	}
	if service.options.AdminHostPort != DefaultAdminHostPort || service.options.MattermostHostPort != DefaultMattermostHostPort {
		t.Fatalf("ports = %d/%d", service.options.AdminHostPort, service.options.MattermostHostPort)
	}
}

func TestEphemeralServiceUsesRunScopedStateAndPorts(t *testing.T) {
	service, errorValue := NewService(Options{
		RepositoryRootPath: "/repo",
		ExecutablePath:     "/repo/internkim",
		RunID:              "Test Run 1",
		AdminHostPort:      19080,
		MattermostHostPort: 19065,
		IsEphemeral:        true,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if service.options.RunID != "test-run-1" {
		t.Fatalf("run id = %q", service.options.RunID)
	}
	if service.options.VirtualMachineName != "internkim-e2e-test-run-1" {
		t.Fatalf("virtual machine name = %q", service.options.VirtualMachineName)
	}
	expectedStatePath := filepath.Join("/repo", ".local", "local-fleet", "runs", "test-run-1")
	if service.options.StateRootPath != expectedStatePath {
		t.Fatalf("state path = %q", service.options.StateRootPath)
	}
	if service.adminHostURL() != "http://127.0.0.1:19080" || service.mattermostHostURL() != "http://127.0.0.1:19065" {
		t.Fatalf("urls = %s %s", service.adminHostURL(), service.mattermostHostURL())
	}
}

func TestEphemeralCleanupContextSurvivesCanceledRun(t *testing.T) {
	runContext, cancelRun := context.WithCancel(context.Background())
	cancelRun()

	cleanupContext, cancelCleanup := newEphemeralCleanupContext(runContext)
	defer cancelCleanup()

	if cleanupContext.Err() != nil {
		t.Fatalf("cleanup context should survive canceled run context: %v", cleanupContext.Err())
	}
}

func TestPredeployGateUsesOneRecipePlan(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	plans := service.predeployGatePlans()
	joinedPlans := joinedPlanArguments(plans)
	for _, expectedFragment := range []string{
		"-L '127.0.0.1:18080:127.0.0.1:18080'",
		"-L '127.0.0.1:8065:127.0.0.1:8065'",
		"prepare-container-kernel",
		"prepare-local-fleet-embedding",
		"make build",
		"setup --board lab",
		"sudo bash '/mnt/shared/workspace/lab/scripts/configure-local-embedding.sh'",
		"configure-mattermost-test-settings.sh",
		"--admin-email local-fleet-admin@internkim.test",
		"verify api",
		"verify mattermost",
		"verify browser --local",
	} {
		if !strings.Contains(joinedPlans, expectedFragment) {
			t.Fatalf("expected %q in plans:\n%s", expectedFragment, joinedPlans)
		}
	}
}

func TestMattermostDirectMessageScenarioUsesVerifyGate(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	plans := service.mattermostDirectMessageScenarioPlans(false)
	joinedPlans := joinedPlanArguments(plans)
	if !strings.Contains(joinedPlans, "verify mattermost --direct-message-e2e") {
		t.Fatalf("expected direct-message verify gate in plans:\n%s", joinedPlans)
	}
}

func TestMattermostAskEphemeralScenarioUsesContainerSmoke(t *testing.T) {
	service, errorValue := NewService(Options{
		RepositoryRootPath: "/repo",
		ExecutablePath:     "/repo/internkim",
		VirtualMachineName: "internkim-e2e-ask",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	plans := service.mattermostAskEphemeralScenarioPlans()
	joinedPlans := joinedPlanArguments(plans)
	if !strings.Contains(joinedPlans, "/repo/lab/scripts/run-smoke-mattermost-ask-ephemeral-container.sh internkim-e2e-ask") {
		t.Fatalf("expected ask ephemeral smoke in plans:\n%s", joinedPlans)
	}
	if !strings.Contains(joinedPlans, "--wait-lock") {
		t.Fatalf("expected local fleet setup to wait for the shared setup lock:\n%s", joinedPlans)
	}
	for _, skippedStep := range []string{"web", "blueclaw-runtime-base", "skills", "blueclaw-config", "blueclaw-payload", "blueclaw-payload-direct", "openrouter", "staging", "services", "users-sync", "health"} {
		if !strings.Contains(joinedPlans, skippedStep) {
			t.Fatalf("expected ask scenario to skip %q:\n%s", skippedStep, joinedPlans)
		}
	}
}

func TestMattermostDocxAttachmentScenarioUsesPromptDownloadGate(t *testing.T) {
	service, errorValue := NewService(Options{
		RepositoryRootPath: "/repo",
		ExecutablePath:     "/repo/internkim",
		StateRootPath:      "/repo/.local/local-fleet/runs/docx",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	plans := service.mattermostDocxAttachmentScenarioPlans(false)
	joinedPlans := joinedPlanArguments(plans)
	for _, expectedFragment := range []string{
		"verify mattermost --prompt",
		"Local Fleet DOCX Attachment Test",
		"--expect-tool file.deliver",
		"--download-files-to '/repo/.local/local-fleet/runs/docx/downloads/mattermost-docx-attachment'",
		"--wait-for-completion",
	} {
		if !strings.Contains(joinedPlans, expectedFragment) {
			t.Fatalf("expected %q in plans:\n%s", expectedFragment, joinedPlans)
		}
	}
}

func TestUpPlanCanSkipWebForMattermostOutputTests(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim", IsEphemeral: true})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	joinedPlans := joinedPlanArguments(service.upPlans(true))
	if !strings.Contains(joinedPlans, "--skip wifi,local-llm,cloudflare-access,tunnel,google,slack,web") {
		t.Fatalf("expected test up plan to skip web:\n%s", joinedPlans)
	}
	if !strings.Contains(joinedPlans, "INTERNKIM_BLUECLAW_USE_LOCAL=1") {
		t.Fatalf("expected test up plan to use local Blueclaw checkout:\n%s", joinedPlans)
	}
	if strings.Contains(joinedPlans, blueclaw.BlueclawTestModelEnvironment) {
		t.Fatalf("expected test up plan to preserve tier model names:\n%s", joinedPlans)
	}
	if !strings.Contains(joinedPlans, blueclaw.BlueclawTestMaximumModelTierEnvironment+"='xlow'") {
		t.Fatalf("expected test up plan to cap models at xlow:\n%s", joinedPlans)
	}
	if !strings.Contains(joinedPlans, "setup --board lab --ssh --host \"$host\" --user admin --password admin --admin-email local-fleet-admin@internkim.test --wait-lock --force") {
		t.Fatalf("expected test up plan to force setup against the disposable VM:\n%s", joinedPlans)
	}
}

func TestUpPlanSkipsRuntimeBaseForReusableFleet(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	joinedPlans := joinedPlanArguments(service.upPlans(true))
	if !strings.Contains(joinedPlans, "--force") {
		t.Fatalf("expected reusable fleet setup to force small changed components:\n%s", joinedPlans)
	}
	if !strings.Contains(joinedPlans, "--skip wifi,local-llm,cloudflare-access,tunnel,google,slack,web,blueclaw-runtime-base") {
		t.Fatalf("expected reusable fleet setup to skip runtime base reinstall:\n%s", joinedPlans)
	}
}

func TestUpPlanCanUseRealModels(t *testing.T) {
	service, errorValue := NewService(Options{
		RepositoryRootPath:  "/repo",
		ExecutablePath:      "/repo/internkim",
		ShouldUseRealModels: true,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	joinedPlans := joinedPlanArguments(service.upPlans(true))
	if strings.Contains(joinedPlans, blueclaw.BlueclawTestModelEnvironment) || strings.Contains(joinedPlans, blueclaw.BlueclawTestModelTierEnvironment) {
		t.Fatalf("expected real model setup to omit test model selection:\n%s", joinedPlans)
	}
	if strings.Contains(joinedPlans, blueclaw.BlueclawTestMaximumModelTierEnvironment) {
		t.Fatalf("expected real model setup to omit model tier ceiling:\n%s", joinedPlans)
	}
}

func TestUpPlanCanSetMaximumModelTier(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim", MaximumModelTier: "high"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	joinedPlans := joinedPlanArguments(service.upPlans(true))
	if !strings.Contains(joinedPlans, blueclaw.BlueclawTestMaximumModelTierEnvironment+"='high'") {
		t.Fatalf("expected high maximum model tier:\n%s", joinedPlans)
	}
}

func TestUpPlanCanPassGenerationOptionsToSetup(t *testing.T) {
	service, errorValue := NewService(Options{
		RepositoryRootPath:    "/repo",
		ExecutablePath:        "/repo/internkim",
		GenerationSeed:        "41",
		GenerationTemperature: "0",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	joinedPlans := joinedPlanArguments(service.upPlans(true))
	for _, expectedFragment := range []string{
		"INTERNKIM_TEST_GENERATION_SEED='41'",
		"INTERNKIM_TEST_GENERATION_TEMPERATURE='0'",
		"INTERNKIM_TEST_GENERATION_TEMPERATURE='0' '/repo/internkim' setup --board lab",
	} {
		if !strings.Contains(joinedPlans, expectedFragment) {
			t.Fatalf("expected %q in plans:\n%s", expectedFragment, joinedPlans)
		}
	}
}

func TestScenarioPlanPassesConfigBeforeRemoteCommand(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	plan := service.blueclawLabScenarioScriptPlan("dm-recipient-resolve")
	configIndex := -1
	commandIndex := -1
	for index, argument := range plan.Arguments {
		if argument == "--config" {
			configIndex = index
		}
		if strings.HasPrefix(argument, "bash ") && commandIndex == -1 {
			commandIndex = index
		}
	}
	if configIndex == -1 || commandIndex == -1 {
		t.Fatalf("expected --config and remote command in plan: %v", plan.Arguments)
	}
	if configIndex > commandIndex {
		t.Fatalf("--config must precede the remote command so vm-ssh parses it: %v", plan.Arguments)
	}
}

func TestStartTunnelCommandKeepsSSHAliveAfterShellExit(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	command := service.startTunnelCommand()
	for _, expectedFragment := range []string{
		"(nohup '/repo/bin/sshpass'",
		"tunnel.log' 2>&1 < /dev/null & echo $! >",
		"tunnel.pid')",
	} {
		if !strings.Contains(command, expectedFragment) {
			t.Fatalf("expected %q in tunnel command:\n%s", expectedFragment, command)
		}
	}
}

func TestStartTunnelCommandUsesConfiguredHostPorts(t *testing.T) {
	service, errorValue := NewService(Options{
		RepositoryRootPath: "/repo",
		ExecutablePath:     "/repo/internkim",
		AdminHostPort:      19080,
		MattermostHostPort: 19065,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	command := service.startTunnelCommand()
	for _, expectedFragment := range []string{
		"-L '127.0.0.1:19080:127.0.0.1:18080'",
		"-L '127.0.0.1:19065:127.0.0.1:8065'",
	} {
		if !strings.Contains(command, expectedFragment) {
			t.Fatalf("expected %q in tunnel command:\n%s", expectedFragment, command)
		}
	}
}

func TestMattermostDirectMessageScenarioCanKeepArtifacts(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	plans := service.mattermostDirectMessageScenarioPlans(true)
	joinedPlans := joinedPlanArguments(plans)
	if !strings.Contains(joinedPlans, "verify mattermost --direct-message-e2e --keep") {
		t.Fatalf("expected direct-message verify keep gate in plans:\n%s", joinedPlans)
	}
}

func TestMattermostManualScenarioPreparesBrowserSession(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	joinedPlans := joinedPlanArguments(service.mattermostManualScenarioPlans())
	if !strings.Contains(joinedPlans, "/mnt/shared/workspace/lab/scripts/prepare-mattermost-manual-test.sh") {
		t.Fatalf("expected manual Mattermost preparation script in plans:\n%s", joinedPlans)
	}
}

func TestMattermostManualScenarioRequiresKeep(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	errorValue = service.RunScenario(context.Background(), &recordingLogger{}, "mattermost-manual", false, false)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "requires --keep") {
		t.Fatalf("expected manual Mattermost keep requirement, got %v", errorValue)
	}
}

func TestRealModelsIgnorePinnedTestModel(t *testing.T) {
	t.Setenv(blueclaw.BlueclawTestModelEnvironment, "test/pinned")
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim", ShouldUseRealModels: true})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	assignments := strings.Join(service.setupEnvironmentAssignments(), " ")
	if strings.Contains(assignments, blueclaw.BlueclawTestModelEnvironment) {
		t.Fatalf("expected real models to ignore pinned test model: %s", assignments)
	}
}

func TestWithoutMattermostScenarioRunsLinuxVirtualSession(t *testing.T) {
	service, errorValue := NewService(Options{
		RepositoryRootPath: "/repo",
		ExecutablePath:     "/repo/internkim",
		RunID:              "without-mm",
		IsEphemeral:        true,
		AdminHostPort:      19080,
		MattermostHostPort: 19065,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	plans := service.withoutMattermostScenarioPlans("dm_send_confirm_acceptance")
	joinedPlans := joinedPlanArguments(plans)
	for _, expectedFragment := range []string{
		"vm-up --config",
		"provision-blueclaw-dev-session.sh",
		"virtual-session",
		"--scenario' 'dm_send_confirm_acceptance",
		".artifacts/local-fleet/without-mm/dm-send-confirm-acceptance",
	} {
		if !strings.Contains(joinedPlans, expectedFragment) {
			t.Fatalf("expected %q in plans:\n%s", expectedFragment, joinedPlans)
		}
	}
	if strings.Contains(joinedPlans, "setup --board lab") || strings.Contains(joinedPlans, "verify mattermost") {
		t.Fatalf("without-mattermost scenario should not run setup or Mattermost verify:\n%s", joinedPlans)
	}
}

func TestSDKDHostTopologyScenarioRunsProvisionedLinuxGate(t *testing.T) {
	service, errorValue := NewService(Options{
		RepositoryRootPath: "/repo",
		ExecutablePath:     "/repo/internkim",
		RunID:              "sdkd-host-topology",
		IsEphemeral:        true,
		AdminHostPort:      19080,
		MattermostHostPort: 19065,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	plans := service.sdkdHostTopologyScenarioPlans()
	joinedPlans := joinedPlanArguments(plans)
	for _, expectedFragment := range []string{
		"vm-up --config",
		"make build",
		"setup --board lab",
		"--skip wifi,local-llm,cloudflare-access,tunnel,google,slack,web,mattermost",
		"sudo bash '/mnt/shared/workspace/lab/scripts/scenario-sdkd-host-topology.sh'",
	} {
		if !strings.Contains(joinedPlans, expectedFragment) {
			t.Fatalf("expected %q in plans:\n%s", expectedFragment, joinedPlans)
		}
	}
	if strings.Contains(joinedPlans, "virtual-session") || strings.Contains(joinedPlans, "verify mattermost") || strings.Contains(joinedPlans, "configure-mattermost-test-settings.sh") {
		t.Fatalf("SDKD topology should run against provisioned host services:\n%s", joinedPlans)
	}
	for _, plan := range plans {
		if strings.Contains(strings.Join(plan.Arguments, " "), "setup --board lab") {
			if !containsEnvironmentValue(plan.Environment, blueclaw.BlueclawSDKDModeEnvironment+"=authoritative") {
				t.Fatalf("expected authoritative SDKD setup environment, got %v", plan.Environment)
			}
			if containsEnvironmentName(plan.Environment, blueclaw.BlueclawTestModelTierEnvironment) {
				t.Fatalf("expected SDKD scenario to preserve production task level, got %v", plan.Environment)
			}
			if containsEnvironmentName(plan.Environment, blueclaw.BlueclawTestMaximumModelTierEnvironment) {
				t.Fatalf("expected SDKD scenario to omit the test maximum model tier for production task-level intent, got %v", plan.Environment)
			}
			if strings.Contains(strings.Join(plan.Arguments, " "), blueclaw.BlueclawTestMaximumModelTierEnvironment+"=") {
				t.Fatalf("expected SDKD setup command to omit the test maximum model tier for production task-level intent, got %v", plan.Arguments)
			}
			if !containsEnvironmentValue(plan.Environment, blueclaw.BlueclawAdminTaskDiagnosticEnvironment+"=true") {
				t.Fatalf("expected SDKD diagnostic preset environment, got %v", plan.Environment)
			}
		}
	}
}

func TestSDKDHostTopologyScriptVerifiesFallbackAndRecovery(t *testing.T) {
	scriptPath := filepath.Join("..", "..", "lab", "scripts", "scenario-sdkd-host-topology.sh")
	if output, errorValue := exec.Command("bash", "-n", scriptPath).CombinedOutput(); errorValue != nil {
		t.Fatalf("invalid SDKD topology script: %v: %s", errorValue, output)
	}
	document, errorValue := os.ReadFile(scriptPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	script := string(document)
	for _, expectedFragment := range []string{
		`.languageModel.defaultProvider == "sdkd"`,
		`exec sudo bash "$0" "$@"`,
		`conversation_id="sdkd-topology-$(cat /proc/sys/kernel/random/uuid)"`,
		`requester_person_id=`,
		`--arg requesterPersonID "$requester_person_id"`,
		`requesterPersonID:$requesterPersonID`,
		`conversationID:$conversationID`,
		`taskDecisionPreset:$taskDecisionPreset`,
		`task_decision_preset=${2-sdkd_topology}`,
		`workspace_runtime_config=/root/.blueclaw/workspace/.blueclaw/config/runtime.json`,
		`workspace_sync_source=$(mktemp -d)`,
		`blueclaw_process_pattern='[/]usr/local/bin/blueclaw-supervisor|[/]firecracker .*--api-sock /firecracker-api.socket'`,
		`stage_workspace_runtime_config() {`,
		`mkdir -p "$workspace_sync_source/.blueclaw/config"`,
		`cp "$workspace_runtime_source" "$workspace_sync_source/.blueclaw/config/runtime.json"`,
		`replace_host_runtime_config() {`,
		`mktemp "${runtime_config}.tmp.XXXXXX"`,
		`cp "$runtime_source" "$temporary_runtime_config"`,
		`mv "$temporary_runtime_config" "$runtime_config"`,
		`wait_for_blueclaw() {`,
		`http://127.0.0.1:8080/admin/api/health`,
		`sync_workspace_runtime_config() {`,
		`systemctl stop "$blueclaw_service_name" >/dev/null 2>&1 || true`,
		`if ! systemctl is-active --quiet "$blueclaw_service_name" && ! pgrep -f "$blueclaw_process_pattern" >/dev/null; then`,
		`systemctl kill "$blueclaw_service_name" --kill-who=all --signal=KILL >/dev/null 2>&1 || true`,
		`/usr/local/bin/blueclaw-supervisor sync-workspace --atomic --preserve-guest-state`,
		`--runtime "$runtime_config"`,
		`--source "$workspace_sync_source"`,
		`systemctl start "$blueclaw_service_name"`,
		`systemctl is-active "$blueclaw_service_name" 2>/dev/null`,
		`systemctl is-active "$blueclaw_service_name" 2>/dev/null
  wait_for_blueclaw`,
		`apply_runtime_config() {`,
		`stage_workspace_runtime_config "$workspace_runtime_source"`,
		`replace_host_runtime_config "$runtime_source"`,
		`apply_runtime_config "$runtime_config_backup" "$workspace_runtime_config_backup"`,
		`structuredSchemaNames = ((.languageModel.sdkd.structuredSchemaNames // []) + ["blueclaw_turn_router"] | unique)`,
		`jq -e '.languageModel.sdkd.structuredSchemaNames == ["blueclaw_agent_turn_action"]'`,
		`jq -e '.languageModel.sdkd.structuredSchemaNames == ["blueclaw_agent_turn_action"]' "$workspace_runtime_config"`,
		`enable_router_schema`,
		`http://127.0.0.1:8080/admin/api/policy`,
		`.people[0].personID | select(length > 0)`,
		`router_task_run_id=$(run_task 'Reply with exactly SDKD topology router ok.' '')`,
		`assert_guest_sdkd_router_transport "$router_task_run_id"`,
		`response_path=$(mktemp)`,
		`--connect-timeout 10 --max-time 300`,
		`task run response did not contain a completed task with a finish message`,
		`capability_socket_path=/run/internkim/capability.sock`,
		`chat_bridge_path=/_internkim/sdkd/v1/llm/chat`,
		`Authorization: Bearer $(cat "$auth_key_path")`,
		`--unix-socket "$capability_socket_path"`,
		`http://internkim-capability$chat_bridge_path`,
		`invalid_chat_request=`,
		`run_host_chat_bridge_request()`,
		`assert_host_chat_bridge_response 400 invalid_chat_completion_request false`,
		`assert_guest_sdkd_structured_transport "$authoritative_task_run_id" false`,
		`.taskRun.taskRunID`,
		`/admin/api/task/detail?taskRunID=`,
		`select(.schemaName == "blueclaw_agent_turn_action")`,
		`select(.isIntakePrecomputed == true)`,
		`select(.name == "agent.intake")`,
		`select(.schemaName == "blueclaw_turn_router")`,
		`all($intakes[]; .usedDeterministicFallback == false)`,
		`all($launches[]; (.isIntakePrecomputed // false) == false)`,
		`all($router_calls[]; (.usedFallback // false) == false)`,
		`all($calls[]; (.usedFallback // false) == false)`,
		`systemctl stop "$service_name"`,
		`assert_host_chat_bridge_response 503 sdkd_bridge_unavailable true`,
		`host chat bridge returned an unexpected error envelope`,
		`assert_guest_sdkd_structured_transport "$fallback_task_run_id" true`,
		`systemctl restart "$service_name"`,
		`assert_host_chat_bridge_response 400 invalid_chat_completion_request false`,
		`assert_guest_sdkd_structured_transport "$recovered_task_run_id" false`,
		`restore_runtime || true`,
		`trap restore_sdkd EXIT`,
	} {
		if !strings.Contains(script, expectedFragment) {
			t.Fatalf("expected %q in SDKD topology script", expectedFragment)
		}
	}
	for _, forbiddenFragment := range []string{
		"--relative-target",
		"--relative-target .blueclaw/config",
		"00000000-0000-0000-0000-000000000001",
	} {
		if strings.Contains(script, forbiddenFragment) {
			t.Fatalf("did not expect %q in SDKD topology script", forbiddenFragment)
		}
	}
	lastIndex := -1
	for _, expectedFragment := range []string{
		`stage_workspace_runtime_config() {`,
		`cp "$workspace_runtime_source" "$workspace_sync_source/.blueclaw/config/runtime.json"`,
		`replace_host_runtime_config() {`,
		`temporary_runtime_config=$(mktemp "${runtime_config}.tmp.XXXXXX")`,
		`cp "$runtime_source" "$temporary_runtime_config"`,
		`mv "$temporary_runtime_config" "$runtime_config"`,
		`systemctl stop "$blueclaw_service_name" >/dev/null 2>&1 || true`,
		`systemctl kill "$blueclaw_service_name" --kill-who=all --signal=KILL >/dev/null 2>&1 || true`,
		`/usr/local/bin/blueclaw-supervisor sync-workspace --atomic --preserve-guest-state`,
		`systemctl start "$blueclaw_service_name"`,
		`systemctl is-active "$blueclaw_service_name" 2>/dev/null`,
	} {
		fragmentIndex := strings.Index(script, expectedFragment)
		if fragmentIndex < 0 {
			t.Fatalf("expected %q in SDKD topology script", expectedFragment)
		}
		if fragmentIndex <= lastIndex {
			t.Fatalf("expected %q after previous workspace sync fragment", expectedFragment)
		}
		lastIndex = fragmentIndex
	}
	applyRuntimeConfigIndex := strings.Index(script, `apply_runtime_config() {`)
	if applyRuntimeConfigIndex < 0 {
		t.Fatal("expected apply_runtime_config function")
	}
	lastIndex = applyRuntimeConfigIndex
	for _, expectedFragment := range []string{
		`stage_workspace_runtime_config "$workspace_runtime_source"`,
		`replace_host_runtime_config "$runtime_source"`,
		`sync_workspace_runtime_config`,
	} {
		fragmentIndex := strings.Index(script[lastIndex:], expectedFragment)
		if fragmentIndex < 0 {
			t.Fatalf("expected %q in apply_runtime_config", expectedFragment)
		}
		lastIndex += fragmentIndex
	}
}

func containsEnvironmentValue(environment []string, expectedValue string) bool {
	for _, value := range environment {
		if value == expectedValue {
			return true
		}
	}
	return false
}

func containsEnvironmentName(environment []string, name string) bool {
	prefix := name + "="
	for _, value := range environment {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func TestEphemeralCleanupRemovesVirtualMachineAndKeepsEvidenceState(t *testing.T) {
	service, errorValue := NewService(Options{
		RepositoryRootPath: "/repo",
		ExecutablePath:     "/repo/internkim",
		StateRootPath:      "/repo/.local/local-fleet/runs/run-1",
		VirtualMachineName: "internkim-e2e-run-1",
		IsEphemeral:        true,
		AdminHostPort:      19080,
		MattermostHostPort: 19065,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	joinedPlans := joinedPlanArguments(service.ephemeralCleanupPlans())
	for _, expectedFragment := range []string{
		"'container' stop 'internkim-e2e-run-1'",
		"'container' rm 'internkim-e2e-run-1'",
	} {
		if !strings.Contains(joinedPlans, expectedFragment) {
			t.Fatalf("expected %q in cleanup plans:\n%s", expectedFragment, joinedPlans)
		}
	}
	if strings.Contains(joinedPlans, "rm -rf '/repo/.local/local-fleet/runs/run-1'") {
		t.Fatalf("cleanup plans should preserve run evidence state:\n%s", joinedPlans)
	}
	if !strings.Contains(service.manualCleanupCommand(), "rm -rf '/repo/.local/local-fleet/runs/run-1'") {
		t.Fatalf("manual full cleanup should still include state removal: %s", service.manualCleanupCommand())
	}
}

func TestCheckSharedWorkspaceCommandUsesBindMountedDirectory(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	command := service.checkSharedWorkspaceCommand()
	if !strings.Contains(command, "test -d /mnt/shared/workspace") {
		t.Fatalf("expected shared workspace check, got %s", command)
	}
	if strings.Contains(command, "virtiofs") || strings.Contains(command, "mountpoint") {
		t.Fatalf("expected no mount logic, got %s", command)
	}
}

func TestUnsupportedScenarioFails(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: t.TempDir(), ExecutablePath: "/bin/echo"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	errorValue = service.RunScenario(context.Background(), &recordingLogger{}, "unknown", false, false)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "unsupported local fleet scenario") {
		t.Fatalf("expected unsupported scenario error, got %v", errorValue)
	}
}

func TestStatusReportsConfigurationFailure(t *testing.T) {
	service, errorValue := NewService(Options{
		RepositoryRootPath: "/repo",
		ExecutablePath:     "/bin/echo",
		StateRootPath:      "/dev/null/local-fleet",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	status := service.Status(context.Background())
	if status.VirtualMachine.State != "failed" || !strings.Contains(status.VirtualMachine.Message, "configuration failed") {
		t.Fatalf("virtual machine status = %+v", status.VirtualMachine)
	}
	if status.MattermostURL != "http://127.0.0.1:8065" {
		t.Fatalf("mattermost URL = %q", status.MattermostURL)
	}
}

func TestLoopbackURLGuard(t *testing.T) {
	if !isLoopbackURL("http://127.0.0.1:8065/api/v4/system/ping") {
		t.Fatal("expected loopback URL to be accepted")
	}
	if isLoopbackURL("https://pilot-01.intern.kim") {
		t.Fatal("expected public URL to be rejected")
	}
}

func joinedPlanArguments(plans []CommandPlan) string {
	var lines []string
	for _, plan := range plans {
		lines = append(lines, strings.Join(append([]string{plan.Name}, plan.Arguments...), " "))
	}
	return strings.Join(lines, "\n")
}
