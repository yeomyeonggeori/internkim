package localfleet

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
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
}

func TestPredeployGateUsesOneRecipePlan(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	plans := service.predeployGatePlans()
	joinedPlans := joinedPlanArguments(plans)
	for _, expectedFragment := range []string{
		"-L 127.0.0.1:18080:127.0.0.1:18080",
		"-L 127.0.0.1:8065:127.0.0.1:8065",
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
	plans := service.mattermostDirectMessageScenarioPlans()
	joinedPlans := joinedPlanArguments(plans)
	if !strings.Contains(joinedPlans, "verify mattermost --direct-message-e2e") {
		t.Fatalf("expected direct-message verify gate in plans:\n%s", joinedPlans)
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

func TestUnsupportedScenarioFails(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: t.TempDir(), ExecutablePath: "/bin/echo"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	errorValue = service.RunScenario(context.Background(), &recordingLogger{}, "unknown")
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
