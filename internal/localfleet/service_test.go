package localfleet

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/blueclawworkspace"
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
		"(nohup sshpass -p admin",
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
