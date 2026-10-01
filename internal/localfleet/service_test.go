package localfleet

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/blueclawworkspace"
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
	if service.options.AdminHostPort != DefaultAdminHostPort {
		t.Fatalf("admin port = %d", service.options.AdminHostPort)
	}
}

func TestEphemeralServiceUsesRunScopedStateAndPorts(t *testing.T) {
	service, errorValue := NewService(Options{
		RepositoryRootPath: "/repo",
		ExecutablePath:     "/repo/internkim",
		RunID:              "Test Run 1",
		AdminHostPort:      19080,
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
	if service.adminHostURL() != "http://127.0.0.1:19080" {
		t.Fatalf("admin URL = %s", service.adminHostURL())
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
		"prepare-container-kernel",
		"prepare-local-fleet-embedding",
		"make build",
		"setup --board lab",
		"sudo bash '/mnt/shared/workspace/lab/scripts/configure-local-embedding.sh'",
		"--admin-email local-fleet-admin@internkim.test",
		"verify api",
		"verify-personal-settings.ts",
	} {
		if !strings.Contains(joinedPlans, expectedFragment) {
			t.Fatalf("expected %q in plans:\n%s", expectedFragment, joinedPlans)
		}
	}
}

func TestCompanyBrowserVerificationUsesManagedCentralPlane(t *testing.T) {
	service, errorValue := NewService(Options{
		RepositoryRootPath: "/repo",
		ExecutablePath:     "/repo/internkim",
		StateRootPath:      "/repo/.local/local-fleet/runs/browser-check",
		CompanyAppPort:     5197,
		AdminHostPort:      19080,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	plan := service.companyBrowserVerificationPlan()
	if plan.Name != "bun" {
		t.Fatalf("browser executable = %q", plan.Name)
	}
	if plan.DirectoryPath != "/repo" {
		t.Fatalf("browser working directory = %q", plan.DirectoryPath)
	}
	expectedArguments := []string{
		"run", filepath.Join("/repo", "tools", "verify-personal-settings.ts"),
		"--state-root", "/repo/.local/local-fleet/runs/browser-check",
		"--app-port", "5197",
		"--admin-port", "19080",
		"--chatd-url", blueclaw.ChatdEndpoint,
		"--config", "/repo/.local/local-fleet/runs/browser-check/config.json",
	}
	if strings.Join(plan.Arguments, " ") != strings.Join(expectedArguments, " ") {
		t.Fatalf("browser arguments = %q, want %q", strings.Join(plan.Arguments, " "), strings.Join(expectedArguments, " "))
	}
	personalSettingsPlans := service.personalSettingsScenarioPlans()
	personalSettingsPlan := personalSettingsPlans[len(personalSettingsPlans)-1]
	if strings.Join(personalSettingsPlan.Arguments, " ") != strings.Join(plan.Arguments, " ") {
		t.Fatalf("personal settings arguments = %q, want shared browser verification arguments %q", strings.Join(personalSettingsPlan.Arguments, " "), strings.Join(plan.Arguments, " "))
	}
}

func TestLearningSettingsScenarioUsesDedicatedSettingsScript(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	plans := service.learningSettingsScenarioPlans()
	joined := joinedPlanArguments(plans)
	if !strings.Contains(joined, "scenario-learning-settings.sh") {
		t.Fatalf("learning settings scenario did not use its dedicated script:\n%s", joined)
	}
	if strings.Contains(joined, "scenario-workspace-persistence.sh") {
		t.Fatalf("learning settings scenario reused workspace persistence:\n%s", joined)
	}
}

func TestTaskHistoryRetryScenarioRunsDatabaseAndRuntimeAcceptance(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	plans := service.taskHistoryRetryScenarioPlans()
	joined := joinedPlanArguments(plans)
	if !strings.Contains(joined, "scenario-task-history-retry.py") {
		t.Fatalf("task history retry scenario did not use its dedicated script:\n%s", joined)
	}
	for _, binary := range []string{"task-history-retry-postgres.test", "task-history-retry-runtime.test", "task-history-retry-admind.test"} {
		if !strings.Contains(joined, binary) {
			t.Fatalf("task history retry scenario did not build %s:\n%s", binary, joined)
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

func TestUpPlanCanSkipWebForScenarioOutputTests(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim", IsEphemeral: true})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	joinedPlans := joinedPlanArguments(service.upPlans(true))
	if !strings.Contains(joinedPlans, "--skip wifi,local-llm,web") {
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
	if !strings.Contains(forcedSetupPlan, "--skip wifi,local-llm,web,blueclaw-runtime-base") {
		t.Fatalf("expected forced reusable setup to skip the ensured runtime base:\n%s", forcedSetupPlan)
	}
}

func TestReusableUpPlanHonorsExplicitRuntimeBaseSkip(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	plans := service.upPlansThroughSetup(true, []string{"blueclaw-runtime-base"})
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
	plan := service.blueclawLabScenarioScriptPlan("buzz-direct-message")
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
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	command := service.startTunnelCommand()
	for _, expectedFragment := range []string{
		"-L '127.0.0.1:19080:127.0.0.1:18080'",
		"nc -z 127.0.0.1 19080",
	} {
		if !strings.Contains(command, expectedFragment) {
			t.Fatalf("expected %q in tunnel command:\n%s", expectedFragment, command)
		}
	}
	if strings.Contains(command, "8065") {
		t.Fatalf("expected no messenger port forward:\n%s", command)
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

func TestVirtualSessionScenarioRunsLinuxVirtualSession(t *testing.T) {
	repositoryRootPath := t.TempDir()
	pluginPath := filepath.Join(repositoryRootPath, ".dependency", "sample-plugin")
	if errorValue := os.MkdirAll(filepath.Join(pluginPath, "skills"), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	manifest := []byte(`{"$schema":"` + blueclawworkspace.PluginSchemaURL + `","name":"sample-plugin","version":"0.0.1"}`)
	if errorValue := os.WriteFile(filepath.Join(pluginPath, "plugin.json"), manifest, 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}
	service, errorValue := NewService(Options{
		RepositoryRootPath: repositoryRootPath,
		ExecutablePath:     "/repo/internkim",
		RunID:              "virtual-session",
		IsEphemeral:        true,
		AdminHostPort:      19080,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	plans := service.virtualSessionScenarioPlans("dm_send_confirm_acceptance")
	joinedPlans := joinedPlanArguments(plans)
	for _, expectedFragment := range []string{
		"vm-up --config",
		"provision-blueclaw-dev-session.sh",
		"virtual-session",
		"pkg/capabilityprotocol/generated/capability-tools.json",
		"--scenario' 'dm_send_confirm_acceptance",
		".artifacts/local-fleet/virtual-session/dm-send-confirm-acceptance",
	} {
		if !strings.Contains(joinedPlans, expectedFragment) {
			t.Fatalf("expected %q in plans:\n%s", expectedFragment, joinedPlans)
		}
	}
	if strings.Contains(joinedPlans, "setup --board lab") {
		t.Fatalf("virtual session scenario should not run setup:\n%s", joinedPlans)
	}
	virtualPlan := plans[len(plans)-1]
	virtualCommand := strings.Join(virtualPlan.Arguments, " ")
	for _, expectedFragment := range []string{
		"BLUECLAW_SCENARIO_CAPABILITY_CATALOG",
		"/mnt/shared/workspace/pkg/capabilityprotocol/generated/capability-tools.json",
		"BLUECLAW_SCENARIO_SKILL_ROOTS",
		"/mnt/shared/workspace/.dependency/",
	} {
		if !strings.Contains(virtualCommand, expectedFragment) {
			t.Fatalf("virtual session command must carry guest scenario environment %q:\n%s", expectedFragment, virtualCommand)
		}
	}
}

func TestEphemeralCleanupRemovesVirtualMachineAndKeepsEvidenceState(t *testing.T) {
	service, errorValue := NewService(Options{
		RepositoryRootPath: "/repo",
		ExecutablePath:     "/repo/internkim",
		StateRootPath:      "/repo/.local/local-fleet/runs/run-1",
		VirtualMachineName: "internkim-e2e-run-1",
		IsEphemeral:        true,
		AdminHostPort:      19080,
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

func messengerArtifactFixture(t *testing.T, recordedRevision string) (string, string) {
	t.Helper()
	repositoryRootPath := t.TempDir()
	blueclawPath := filepath.Join(repositoryRootPath, ".dependency", "blueclaw")
	if errorValue := os.MkdirAll(blueclawPath, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, arguments := range [][]string{
		{"init", "--quiet"},
		{"commit", "--quiet", "--allow-empty", "-m", "chatd"},
	} {
		command := exec.Command("git", arguments...)
		command.Dir = blueclawPath
		command.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
		if output, errorValue := command.CombinedOutput(); errorValue != nil {
			t.Fatalf("git %v: %v: %s", arguments, errorValue, output)
		}
	}
	pointerCommand := exec.Command("git", "rev-parse", "HEAD")
	pointerCommand.Dir = blueclawPath
	pointer, errorValue := pointerCommand.Output()
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	artifactPath := filepath.Join(repositoryRootPath, ".dependency", "buzz-relay")
	if errorValue := os.MkdirAll(artifactPath, 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	if recordedRevision == matchingChatdRevision {
		recordedRevision = strings.TrimSpace(string(pointer))
	}
	if recordedRevision != "" {
		if errorValue := os.WriteFile(filepath.Join(artifactPath, "CHATD_REVISION"), []byte(recordedRevision+"\n"), 0o644); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	return repositoryRootPath, strings.TrimSpace(string(pointer))
}

const matchingChatdRevision = "<the blueclaw pointer>"

func runMessengerArtifactCheck(t *testing.T, repositoryRootPath string) (error, string) {
	t.Helper()
	service, errorValue := NewService(Options{RepositoryRootPath: repositoryRootPath, ExecutablePath: filepath.Join(repositoryRootPath, "internkim")})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	command := exec.Command("/bin/sh", "-c", service.checkMessengerArtifactCommand())
	output, runError := command.CombinedOutput()
	return runError, string(output)
}

func TestAFleetRunAcceptsAChatdBuiltFromTheBlueclawPointer(t *testing.T) {
	repositoryRootPath, _ := messengerArtifactFixture(t, matchingChatdRevision)

	runError, output := runMessengerArtifactCheck(t, repositoryRootPath)

	if runError != nil {
		t.Fatalf("a chatd built from the pointer must be accepted, got %v: %s", runError, output)
	}
}

func TestAFleetRunRefusesAChatdBuiltFromAnotherRevision(t *testing.T) {
	repositoryRootPath, pointer := messengerArtifactFixture(t, "0123456789012345678901234567890123456789")

	runError, output := runMessengerArtifactCheck(t, repositoryRootPath)

	if runError == nil {
		t.Fatal("a chatd built from another revision would install a messenger the checkout never wrote")
	}
	if !strings.Contains(output, "0123456789012345678901234567890123456789") || !strings.Contains(output, pointer) {
		t.Fatalf("the refusal must name both revisions, got %s", output)
	}
	if !strings.Contains(output, "make prepare-buzz-relay") {
		t.Fatalf("the refusal must say how to fix it, got %s", output)
	}
}

func TestAFleetRunRefusesAChatdThatRecordsNoRevision(t *testing.T) {
	repositoryRootPath, _ := messengerArtifactFixture(t, "")

	runError, output := runMessengerArtifactCheck(t, repositoryRootPath)

	if runError == nil {
		t.Fatal("a copied artifact with no CHATD_REVISION is exactly the one that shipped a chatd with no health route")
	}
	if !strings.Contains(output, "make prepare-buzz-relay") {
		t.Fatalf("the refusal must say how to fix it, got %s", output)
	}
}

func TestUpPlansCheckTheMessengerArtifactBeforeBuildingAnything(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	plans := service.upPlansThroughSetup(true, nil)
	checkIndex := -1
	buildIndex := -1
	for index, plan := range plans {
		joined := strings.Join(plan.Arguments, " ")
		if strings.Contains(joined, "CHATD_REVISION") {
			checkIndex = index
		}
		if plan.Name == "make" {
			buildIndex = index
		}
	}
	if checkIndex < 0 {
		t.Fatal("a fleet run must check the messenger binary it would install")
	}
	if buildIndex < 0 || checkIndex > buildIndex {
		t.Fatalf("the check must come before the run spends ten minutes, got check %d build %d", checkIndex, buildIndex)
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

func TestScenarioRefusesStaleRuntimeBaseBeforeProvisioning(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: t.TempDir(), ExecutablePath: "/bin/echo"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	logger := &recordingLogger{}
	errorValue = service.Run(context.Background(), logger, JobRequest{
		Action:   ActionRunScenario,
		Scenario: "workspace-persistence",
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "make prepare-blueclaw-runtime-base") {
		t.Fatalf("expected runtime-base preflight failure, got %v", errorValue)
	}
	if len(logger.lines) != 0 {
		t.Fatalf("provisioning started before runtime-base preflight: %v", logger.lines)
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

func TestBuzzAttachmentScenarioRunsTheBuzzScript(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim", IsEphemeral: true})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	joinedPlans := joinedPlanArguments(service.buzzAttachmentScenarioPlans())

	if !strings.Contains(joinedPlans, "lab/scripts/scenario-buzz-attachment.sh") {
		t.Fatalf("expected the buzz scenario script in plans:\n%s", joinedPlans)
	}
}

func TestBuzzDirectMessageScenarioRunsTheBuzzScript(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim", IsEphemeral: true})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	joinedPlans := joinedPlanArguments(service.buzzDirectMessageScenarioPlans())

	if !strings.Contains(joinedPlans, "lab/scripts/scenario-buzz-direct-message.sh") {
		t.Fatalf("expected the buzz direct message script in plans:\n%s", joinedPlans)
	}
}

func TestBuzzInboundMentionScenarioRunsTheBuzzScript(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim", IsEphemeral: true})
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	joinedPlans := joinedPlanArguments(service.buzzInboundMentionScenarioPlans())

	if !strings.Contains(joinedPlans, "lab/scripts/scenario-buzz-inbound-mention.sh") {
		t.Fatalf("expected the buzz inbound mention script in plans:\n%s", joinedPlans)
	}
}

func TestEachFleetAsksForAKeyInItsOwnName(t *testing.T) {
	ephemeral, errorValue := NewService(Options{
		RepositoryRootPath: "/repo",
		ExecutablePath:     "/repo/internkim",
		RunID:              "test-run-1",
		IsEphemeral:        true,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	shared, errorValue := NewService(Options{
		RepositoryRootPath: "/repo",
		ExecutablePath:     "/repo/internkim",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	ephemeralPlan := joinedPlanArguments([]CommandPlan{ephemeral.startCentralPlanePlan()})
	sharedPlan := joinedPlanArguments([]CommandPlan{shared.startCentralPlanePlan()})
	if !strings.Contains(ephemeralPlan, "--agent-name internkim-e2e-test-run-1") {
		t.Fatalf("a run's plane must issue the key in that run's name:\n%s", ephemeralPlan)
	}
	if !strings.Contains(sharedPlan, "--agent-name "+DefaultVirtualMachineName) {
		t.Fatalf("the shared fleet must issue the key in its own name:\n%s", sharedPlan)
	}
	if !strings.Contains(ephemeralPlan, "--admin-port "+strconv.Itoa(ephemeral.options.AdminHostPort)) {
		t.Fatalf("the ephemeral plane must target its configured admind port:\n%s", ephemeralPlan)
	}
	if !strings.Contains(sharedPlan, "--admin-port "+strconv.Itoa(shared.options.AdminHostPort)) {
		t.Fatalf("the shared plane must target its configured admind port:\n%s", sharedPlan)
	}
	if ephemeralPlan == sharedPlan {
		t.Fatal("two fleets asking for the same name take each other's key away")
	}
}
