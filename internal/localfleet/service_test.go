package localfleet

import (
	"context"
	"os"
	"path/filepath"
	"sort"
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
		"verify browser --local",
	} {
		if !strings.Contains(joinedPlans, expectedFragment) {
			t.Fatalf("expected %q in plans:\n%s", expectedFragment, joinedPlans)
		}
	}
}

func TestLocalEmbeddingLibraryProbeConsumesCompleteLdconfigOutput(t *testing.T) {
	scriptPath := filepath.Join("..", "..", "lab", "scripts", "configure-local-embedding.sh")
	document, errorValue := os.ReadFile(scriptPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	script := string(document)
	if !strings.Contains(script, "ldconfig -p | grep -F 'libgomp.so.1' >/dev/null") {
		t.Fatal("local embedding script does not probe libgomp safely")
	}
}

func TestUpPlanCanSkipWebForMattermostOutputTests(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim", IsEphemeral: true})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	joinedPlans := joinedPlanArguments(service.upPlans(true))
	if !strings.Contains(joinedPlans, "--skip wifi,local-llm,slack,web") {
		t.Fatalf("expected test up plan to skip web:\n%s", joinedPlans)
	}
	if !strings.Contains(joinedPlans, "INTERNKIM_BLUECLAW_USE_LOCAL=1") {
		t.Fatalf("expected test up plan to use local Blueclaw checkout:\n%s", joinedPlans)
	}
	if strings.Contains(joinedPlans, blueclaw.BlueclawTestModelEnvironment) {
		t.Fatalf("expected test up plan to preserve tier model names:\n%s", joinedPlans)
	}
	if !strings.Contains(joinedPlans, blueclaw.BlueclawTestMaximumModelTierEnvironment+"='low'") {
		t.Fatalf("expected test up plan to cap models at low:\n%s", joinedPlans)
	}
	if !strings.Contains(joinedPlans, "setup --board lab --ssh --host \"$host\" --user admin --password admin --admin-email local-fleet-admin@internkim.test --wait-lock --force") {
		t.Fatalf("expected test up plan to force setup against the disposable VM:\n%s", joinedPlans)
	}
	if strings.Contains(joinedPlans, "--only blueclaw-runtime-base") {
		t.Fatalf("expected disposable fleet setup to install the runtime in its single setup pass:\n%s", joinedPlans)
	}
}

func TestReusableUpPlanEnsuresRuntimeBaseBeforeForcedSetup(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	plans := service.upPlans(true)
	runtimeBasePlanIndex := planArgumentIndex(plans, "--only blueclaw-runtime-base")
	forcedSetupPlanIndex := planArgumentIndex(plans, "--force --skip")
	if runtimeBasePlanIndex < 0 || forcedSetupPlanIndex < 0 || runtimeBasePlanIndex >= forcedSetupPlanIndex {
		t.Fatalf("expected runtime base ensure before forced setup:\n%s", joinedPlanArguments(plans))
	}
	runtimeBasePlan := strings.Join(plans[runtimeBasePlanIndex].Arguments, " ")
	if strings.Contains(runtimeBasePlan, "--force") {
		t.Fatalf("expected runtime base ensure to honor its satisfied check:\n%s", runtimeBasePlan)
	}
	forcedSetupPlan := strings.Join(plans[forcedSetupPlanIndex].Arguments, " ")
	if !strings.Contains(forcedSetupPlan, "--skip wifi,local-llm,slack,web,blueclaw-runtime-base") {
		t.Fatalf("expected forced reusable setup to skip the ensured runtime base:\n%s", forcedSetupPlan)
	}
}

func TestReusableUpPlanHonorsExplicitRuntimeBaseSkip(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	plans := service.upPlansWithSkippedSetupSteps(true, []string{"blueclaw-runtime-base"})
	if planArgumentIndex(plans, "--only blueclaw-runtime-base") >= 0 {
		t.Fatalf("expected explicit runtime base skip to omit the ensure pass:\n%s", joinedPlanArguments(plans))
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
		"nc -z 127.0.0.1 19080 && nc -z 127.0.0.1 19065",
	} {
		if !strings.Contains(command, expectedFragment) {
			t.Fatalf("expected %q in tunnel command:\n%s", expectedFragment, command)
		}
	}
}

func TestPreparedFleetPlansOnlyRestoreConnectivity(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	joinedPlans := joinedPlanArguments(service.preparedFleetPlans())
	for _, expectedText := range []string{"vm-up", "test -d /mnt/shared/workspace", "ExitOnForwardFailure=yes"} {
		if !strings.Contains(joinedPlans, expectedText) {
			t.Fatalf("prepared Fleet plans are missing %q:\n%s", expectedText, joinedPlans)
		}
	}
	for _, forbiddenText := range []string{"make build", "setup --board", "configure-local-embedding"} {
		if strings.Contains(joinedPlans, forbiddenText) {
			t.Fatalf("prepared Fleet plans contain %q:\n%s", forbiddenText, joinedPlans)
		}
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
	if strings.Contains(joinedPlans, "setup --board lab") {
		t.Fatalf("without-mattermost scenario should not run setup:\n%s", joinedPlans)
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

func TestScenarioNamesMatchTheRegistryEverySortedAndAccepted(t *testing.T) {
	names := ScenarioNames()
	if !sort.StringsAreSorted(names) {
		t.Fatalf("scenario names are not sorted: %v", names)
	}
	service, errorValue := NewService(Options{RepositoryRootPath: t.TempDir(), ExecutablePath: "/bin/echo"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	builders := service.scenarioPlanBuilders()
	if len(names) != len(builders) {
		t.Fatalf("ScenarioNames returned %d names, scenarioPlanBuilders has %d entries", len(names), len(builders))
	}
	for _, name := range names {
		if _, isKnown := builders[name]; !isKnown {
			t.Fatalf("ScenarioNames listed %q, which scenarioPlanBuilders does not accept", name)
		}
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
	if isLoopbackURL("https://pilot-01.example.test") {
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

func planArgumentIndex(plans []CommandPlan, expectedText string) int {
	for planIndex, plan := range plans {
		if strings.Contains(strings.Join(plan.Arguments, " "), expectedText) {
			return planIndex
		}
	}
	return -1
}

// Every other local fleet scenario drives Mattermost, which is not the
// messenger a company's messages travel over any more.
func TestBuzzAttachmentScenarioRunsTheBuzzScript(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim", IsEphemeral: true})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	joinedPlans := joinedPlanArguments(service.buzzAttachmentScenarioPlans())

	if !strings.Contains(joinedPlans, "lab/scripts/scenario-buzz-attachment.sh") {
		t.Fatalf("expected the buzz scenario script in plans:\n%s", joinedPlans)
	}
	if strings.Contains(joinedPlans, "mattermost") {
		t.Fatalf("expected the buzz scenario to reach nothing through mattermost:\n%s", joinedPlans)
	}
}

func TestBuzzDirectMessageScenarioReachesNothingThroughMattermost(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim", IsEphemeral: true})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	joinedPlans := joinedPlanArguments(service.buzzDirectMessageScenarioPlans())

	if !strings.Contains(joinedPlans, "lab/scripts/scenario-buzz-direct-message.sh") {
		t.Fatalf("expected the buzz direct message script in plans:\n%s", joinedPlans)
	}
	if strings.Contains(joinedPlans, "mattermost") {
		t.Fatalf("the company reads buzz, so this scenario must not need mattermost:\n%s", joinedPlans)
	}
}
