package localfleet

import (
	"context"
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
		"make build",
		"setup --board lab",
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
	if !strings.Contains(joinedPlans, blueclaw.BlueclawTestModelEnvironment+"='"+blueclaw.BlueclawTestModelName+"'") {
		t.Fatalf("expected test up plan to use the cheap test model:\n%s", joinedPlans)
	}
	if !strings.Contains(joinedPlans, "setup --board lab --ssh --host \"$host\" --user admin --password admin --admin-email local-fleet-admin@internkim.test --force") {
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
	if strings.Contains(joinedPlans, blueclaw.BlueclawTestModelEnvironment) {
		t.Fatalf("expected real model setup to omit test model override:\n%s", joinedPlans)
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
