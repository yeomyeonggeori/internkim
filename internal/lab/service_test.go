package lab

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	homeDirectoryPath, errorValue := os.MkdirTemp("", "ik-lab-test-home-")
	if errorValue != nil {
		panic(errorValue)
	}
	previousHomeDirectoryPath := os.Getenv("HOME")
	_ = os.Setenv("HOME", homeDirectoryPath)
	exitCode := m.Run()
	if previousHomeDirectoryPath == "" {
		_ = os.Unsetenv("HOME")
	} else {
		_ = os.Setenv("HOME", previousHomeDirectoryPath)
	}
	_ = os.RemoveAll(homeDirectoryPath)
	os.Exit(exitCode)
}

type fakeCommandRunner struct {
	runCommands    []ExecutableCommand
	startCommands  []ExecutableCommand
	outputCommands []ExecutableCommand
	outputValue    string
	outputValues   []string
	outputError    error
	runError       error
	runErrors      []error
	startError     error
}

func (fakeCommandRunner *fakeCommandRunner) Run(ctx context.Context, executableCommand ExecutableCommand) error {
	_ = ctx
	fakeCommandRunner.runCommands = append(fakeCommandRunner.runCommands, executableCommand)
	if len(fakeCommandRunner.runErrors) > 0 {
		runError := fakeCommandRunner.runErrors[0]
		fakeCommandRunner.runErrors = fakeCommandRunner.runErrors[1:]
		return runError
	}
	return fakeCommandRunner.runError
}

func (fakeCommandRunner *fakeCommandRunner) Start(ctx context.Context, executableCommand ExecutableCommand) error {
	_ = ctx
	fakeCommandRunner.startCommands = append(fakeCommandRunner.startCommands, executableCommand)
	return fakeCommandRunner.startError
}

func (fakeCommandRunner *fakeCommandRunner) Output(ctx context.Context, executableCommand ExecutableCommand) (string, error) {
	_ = ctx
	fakeCommandRunner.outputCommands = append(fakeCommandRunner.outputCommands, executableCommand)
	if fakeCommandRunner.outputError != nil {
		return "", fakeCommandRunner.outputError
	}
	if len(fakeCommandRunner.outputValues) > 0 {
		outputValue := fakeCommandRunner.outputValues[0]
		fakeCommandRunner.outputValues = fakeCommandRunner.outputValues[1:]
		return outputValue, nil
	}
	return fakeCommandRunner.outputValue, nil
}

func TestImageBuildUsesCloneAndSetCommands(t *testing.T) {
	commandRunner := &fakeCommandRunner{}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue := service.ImageBuild(context.Background())
	if errorValue != nil {
		t.Fatalf("expected image build to succeed: %v", errorValue)
	}
	if len(commandRunner.runCommands) != 2 {
		t.Fatalf("expected image build to emit 2 commands, got %d", len(commandRunner.runCommands))
	}
	if commandRunner.runCommands[0].ExecutableName != "tart" {
		t.Fatalf("expected tart clone command, got %q", commandRunner.runCommands[0].ExecutableName)
	}
	if commandRunner.runCommands[0].Arguments[0] != "clone" {
		t.Fatalf("expected clone subcommand, got %q", commandRunner.runCommands[0].Arguments[0])
	}
	if commandRunner.runCommands[1].Arguments[0] != "set" {
		t.Fatalf("expected set subcommand, got %q", commandRunner.runCommands[1].Arguments[0])
	}
}

func TestVirtualMachineUpSkipsImageBuildWhenVirtualMachineExists(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValue:  "10.0.0.5\n",
		outputValues: []string{"internkim-lab\n", `[{"Name":"internkim-lab","Running":true}]`},
	}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue := service.VirtualMachineUp(context.Background())
	if errorValue != nil {
		t.Fatalf("expected vm up to succeed: %v", errorValue)
	}
	if len(commandRunner.runCommands) != 0 {
		t.Fatalf("expected existing vm to skip image build, got %d run commands", len(commandRunner.runCommands))
	}
	if len(commandRunner.startCommands) != 0 {
		t.Fatalf("expected running vm to skip tart run, got %d start commands", len(commandRunner.startCommands))
	}
}

func TestVirtualMachineUpCreatesMissingVirtualMachine(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValues: []string{"", `[{"Name":"internkim-lab","Running":false}]`, "10.0.0.5\n"},
	}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue := service.VirtualMachineUp(context.Background())
	if errorValue != nil {
		t.Fatalf("expected vm up to succeed: %v", errorValue)
	}
	if len(commandRunner.runCommands) != 2 {
		t.Fatalf("expected clone and set commands, got %d", len(commandRunner.runCommands))
	}
	if commandRunner.runCommands[0].Arguments[0] != "clone" {
		t.Fatalf("expected clone command, got %v", commandRunner.runCommands[0].Arguments)
	}
	if commandRunner.runCommands[1].Arguments[0] != "set" {
		t.Fatalf("expected set command, got %v", commandRunner.runCommands[1].Arguments)
	}
	if len(commandRunner.startCommands) != 1 {
		t.Fatalf("expected one vm start command, got %d", len(commandRunner.startCommands))
	}
	if !strings.Contains(strings.Join(commandRunner.startCommands[0].Arguments, " "), "--nested") {
		t.Fatalf("expected nested vm run command, got %v", commandRunner.startCommands[0].Arguments)
	}
	if !strings.Contains(strings.Join(commandRunner.startCommands[0].Arguments, " "), "--dir=workspace:/repo") {
		t.Fatalf("expected repository workspace mount, got %v", commandRunner.startCommands[0].Arguments)
	}
	if commandRunner.startCommands[0].DetachedLogPath == "" {
		t.Fatalf("expected VM start command to use detached log path")
	}
}

func TestVirtualMachineUpRemovesStaleControlSocket(t *testing.T) {
	homeDirectoryPath, errorValue := os.MkdirTemp("/tmp", "ik-lab-home-")
	if errorValue != nil {
		t.Fatalf("expected test home directory: %v", errorValue)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(homeDirectoryPath)
	})
	t.Setenv("HOME", homeDirectoryPath)

	controlSocketDirectoryPath := filepath.Join(homeDirectoryPath, ".tart", "vms", "internkim-lab")
	if errorValue := os.MkdirAll(controlSocketDirectoryPath, 0o755); errorValue != nil {
		t.Fatalf("expected test control socket directory: %v", errorValue)
	}
	controlSocketPath := filepath.Join(controlSocketDirectoryPath, "control.sock")
	if errorValue := os.WriteFile(controlSocketPath, []byte("stale"), 0o600); errorValue != nil {
		t.Fatalf("expected stale control socket fixture: %v", errorValue)
	}

	commandRunner := &fakeCommandRunner{
		outputValues: []string{"internkim-lab\n", `[{"Name":"internkim-lab","Running":false}]`, "10.0.0.5\n"},
	}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue = service.VirtualMachineUp(context.Background())
	if errorValue != nil {
		t.Fatalf("expected vm up to succeed: %v", errorValue)
	}
	if _, errorValue := os.Lstat(controlSocketPath); !os.IsNotExist(errorValue) {
		t.Fatalf("expected stale control socket to be removed, got %v", errorValue)
	}
}

func TestVirtualMachineUpReturnsRunFailureBeforeIPAddressTimeout(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValues: []string{"", `[{"Name":"internkim-lab","Running":false}]`},
		startError:   errors.New("tart run failed"),
	}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue := service.VirtualMachineUp(context.Background())
	if errorValue == nil {
		t.Fatalf("expected vm up to fail")
	}
	if !strings.Contains(errorValue.Error(), "tart run") {
		t.Fatalf("expected tart run command in error, got %q", errorValue.Error())
	}
	if strings.Contains(errorValue.Error(), "ip address") {
		t.Fatalf("expected run failure before ip timeout, got %q", errorValue.Error())
	}
}

func TestProvisionUbuntuUsesRemoteScriptExecution(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValue:  "10.0.0.5\n",
		outputValues: []string{"internkim-lab\n", `[{"Name":"internkim-lab","Running":true}]`},
	}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue := service.ProvisionUbuntu(context.Background())
	if errorValue != nil {
		t.Fatalf("expected ubuntu provision to succeed: %v", errorValue)
	}
	if len(commandRunner.runCommands) != 4 {
		t.Fatalf("expected ssh readiness, writable checks, and remote script execution, got %d", len(commandRunner.runCommands))
	}
	if commandRunner.runCommands[0].ExecutableName != "/repo/bin/sshpass" {
		t.Fatalf("expected repo sshpass invocation, got %q", commandRunner.runCommands[0].ExecutableName)
	}
	if !strings.Contains(strings.Join(commandRunner.runCommands[0].Arguments, " "), "ConnectTimeout=5") {
		t.Fatalf("expected ssh readiness check before provisioning, got %v", commandRunner.runCommands[0].Arguments)
	}
	if !strings.Contains(strings.Join(commandRunner.runCommands[1].Arguments, " "), "internkim-write-check") {
		t.Fatalf("expected writable root check before provisioning, got %v", commandRunner.runCommands[1].Arguments)
	}
	if !strings.HasSuffix(commandRunner.runCommands[2].StandardInputPath, "lab/scripts/provision-ubuntu.sh") {
		t.Fatalf("expected provision script path, got %q", commandRunner.runCommands[2].StandardInputPath)
	}
	if !strings.Contains(strings.Join(commandRunner.runCommands[3].Arguments, " "), "internkim-write-check") {
		t.Fatalf("expected writable root check after provisioning, got %v", commandRunner.runCommands[3].Arguments)
	}
}

func TestProvisionUbuntuRestartsReadOnlyVirtualMachineOnce(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValue: "10.0.0.5\n",
		outputValues: []string{
			"internkim-lab\n",
			`[{"Name":"internkim-lab","Running":true}]`,
			"10.0.0.5\n",
			"10.0.0.5\n",
			"10.0.0.5\n",
			"internkim-lab\n",
			`[{"Name":"internkim-lab","Running":false}]`,
			"10.0.0.5\n",
			"10.0.0.5\n",
			"10.0.0.5\n",
			"10.0.0.5\n",
			"10.0.0.5\n",
		},
		runErrors: []error{
			nil,
			errors.New("read-only file system"),
			nil,
			nil,
			nil,
			nil,
			nil,
		},
	}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue := service.ProvisionUbuntu(context.Background())
	if errorValue != nil {
		t.Fatalf("expected ubuntu provision to recover after VM restart: %v", errorValue)
	}
	if len(commandRunner.startCommands) != 1 {
		t.Fatalf("expected one VM restart, got %d", len(commandRunner.startCommands))
	}
	if len(commandRunner.runCommands) < 5 {
		t.Fatalf("expected restart and provision commands, got %d", len(commandRunner.runCommands))
	}
	if commandRunner.runCommands[2].Arguments[0] != "stop" {
		t.Fatalf("expected VM stop after read-only check, got %v", commandRunner.runCommands[2].Arguments)
	}
}

func TestRuntimeBuilderCheckUsesDedicatedBuilderScript(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValues: []string{
			`[{"Name":"internkim-lab","Running":true}]`,
			"10.0.0.5\n",
			"10.0.0.5\n",
			"10.0.0.5\n",
		},
	}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue := service.RuntimeBuilderCheck(context.Background())
	if errorValue != nil {
		t.Fatalf("expected runtime builder check to succeed: %v", errorValue)
	}
	if len(commandRunner.runCommands) != 2 {
		t.Fatalf("expected ssh readiness and builder check script, got %d", len(commandRunner.runCommands))
	}
	checkCommand := commandRunner.runCommands[1]
	if !strings.HasSuffix(checkCommand.StandardInputPath, "lab/scripts/check-blueclaw-runtime-builder.sh") {
		t.Fatalf("expected builder check script, got %q", checkCommand.StandardInputPath)
	}
}

func TestVirtualMachineSSHUsesConfiguredPasswordAuthentication(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValue: "10.0.0.5\n",
	}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue := service.VirtualMachineSSH(context.Background(), []string{"cd /mnt/shared && true"})
	if errorValue != nil {
		t.Fatalf("expected vm ssh to succeed: %v", errorValue)
	}
	if len(commandRunner.runCommands) != 1 {
		t.Fatalf("expected one ssh command, got %d", len(commandRunner.runCommands))
	}
	command := commandRunner.runCommands[0]
	if command.ExecutableName != "/repo/bin/sshpass" {
		t.Fatalf("expected vm ssh to use repo sshpass, got %q", command.ExecutableName)
	}
	joinedArguments := strings.Join(command.Arguments, " ")
	for _, expectedFragment := range []string{"-p admin", "ssh", "StrictHostKeyChecking=no", "admin@10.0.0.5", "cd /mnt/shared && true"} {
		if !strings.Contains(joinedArguments, expectedFragment) {
			t.Fatalf("expected vm ssh arguments to contain %q, got %v", expectedFragment, command.Arguments)
		}
	}
}

func TestVirtualMachineDiagnosticsIncludesRecoveryCommands(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValues: []string{`[{"Name":"internkim-lab","Running":false}]`, ""},
	}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	diagnostics := service.VirtualMachineDiagnostics(context.Background())
	for _, expectedFragment := range []string{"Tart VM diagnostics", "internkim-lab", "internkim-lab-tart.log", "internkim lab vm-down", "--builder ssh"} {
		if !strings.Contains(diagnostics, expectedFragment) {
			t.Fatalf("expected diagnostics to contain %q, got:\n%s", expectedFragment, diagnostics)
		}
	}
}

func TestSetupUsesCurrentExecutableWithHostOverride(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValue:  "10.0.0.5\n",
		outputValues: []string{"internkim-lab\n", `[{"Name":"internkim-lab","Running":true}]`},
	}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue := service.Setup(context.Background(), "/repo/internkim", nil)
	if errorValue != nil {
		t.Fatalf("expected setup to succeed: %v", errorValue)
	}
	if len(commandRunner.runCommands) != 5 {
		t.Fatalf("expected provision and setup commands, got %d", len(commandRunner.runCommands))
	}
	setupCommand := commandRunner.runCommands[4]
	if setupCommand.ExecutableName != "/repo/internkim" {
		t.Fatalf("expected setup command to use current executable, got %q", setupCommand.ExecutableName)
	}
	if strings.Join(setupCommand.Arguments, " ") != "setup --board lab --ssh --host 10.0.0.5 --user admin --password admin --skip wifi" {
		t.Fatalf("unexpected setup arguments: %v", setupCommand.Arguments)
	}
	if setupCommand.EnvironmentVariables["INTERNKIM_BLUECLAW_USE_LOCAL"] != "1" {
		t.Fatalf("expected simulation setup to build local Blueclaw changes, got %v", setupCommand.EnvironmentVariables)
	}
}

func TestSetupPassesSelectorArgumentsWithoutDefaultForce(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValue:  "10.0.0.5\n",
		outputValues: []string{"internkim-lab\n", `[{"Name":"internkim-lab","Running":true}]`},
	}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue := service.Setup(context.Background(), "/repo/internkim", []string{"--only", "mattermost"})
	if errorValue != nil {
		t.Fatalf("expected setup to succeed: %v", errorValue)
	}

	setupCommand := commandRunner.runCommands[4]
	if strings.Join(setupCommand.Arguments, " ") != "setup --board lab --ssh --host 10.0.0.5 --user admin --password admin --only mattermost" {
		t.Fatalf("unexpected setup arguments: %v", setupCommand.Arguments)
	}
}

func TestSetupSimulationUsesSimulationTargetState(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValue:  "10.0.0.5\n",
		outputValues: []string{"internkim-lab\n", `[{"Name":"internkim-lab","Running":true}]`},
	}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue := service.SetupSimulation(context.Background(), "/repo/internkim", []string{"--only", "services"})
	if errorValue != nil {
		t.Fatalf("expected simulation setup to succeed: %v", errorValue)
	}

	setupCommand := commandRunner.runCommands[4]
	if strings.Join(setupCommand.Arguments, " ") != "setup --board sim --ssh --host 10.0.0.5 --user admin --password admin --only services" {
		t.Fatalf("unexpected setup arguments: %v", setupCommand.Arguments)
	}
}

func TestScenarioEndToEndRunsSetupAndScenarios(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValue:  "10.0.0.5\n",
		outputValues: []string{"internkim-lab\n", `[{"Name":"internkim-lab","Running":true}]`},
	}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue := service.ScenarioEndToEnd(context.Background(), "/repo/internkim", nil)
	if errorValue != nil {
		t.Fatalf("expected end-to-end scenario to succeed: %v", errorValue)
	}
	if len(commandRunner.runCommands) != 9 {
		t.Fatalf("expected provision, setup, and four default scenario commands, got %d", len(commandRunner.runCommands))
	}
}

func TestScenarioSimulationEndToEndUsesSimulationTargetState(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValue:  "10.0.0.5\n",
		outputValues: []string{"internkim-lab\n", `[{"Name":"internkim-lab","Running":true}]`},
	}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue := service.ScenarioSimulationEndToEnd(context.Background(), "/repo/internkim", []string{"--only", "services"})
	if errorValue != nil {
		t.Fatalf("expected simulation scenario to succeed: %v", errorValue)
	}

	setupCommand := commandRunner.runCommands[4]
	if strings.Join(setupCommand.Arguments, " ") != "setup --board sim --ssh --host 10.0.0.5 --user admin --password admin --only services" {
		t.Fatalf("unexpected setup arguments: %v", setupCommand.Arguments)
	}
}

func TestPrintSimulationPlanDoesNotCreateOrStartMissingVirtualMachine(t *testing.T) {
	commandRunner := &fakeCommandRunner{}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue := service.PrintSimulationPlan(context.Background(), "/repo/internkim", []string{"--plan"})
	if errorValue != nil {
		t.Fatalf("expected simulation plan to succeed: %v", errorValue)
	}
	if len(commandRunner.runCommands) != 0 {
		t.Fatalf("expected dry run to skip clone and setup commands, got %d", len(commandRunner.runCommands))
	}
	if len(commandRunner.startCommands) != 0 {
		t.Fatalf("expected dry run to skip vm start commands, got %d", len(commandRunner.startCommands))
	}
}

func buildTestConfiguration() Configuration {
	return applyDefaultConfiguration(Configuration{
		Host: HostConfiguration{
			Mode: "single-mac",
			Companion: CompanionConfiguration{
				ListenAddress:   "127.0.0.1:7780",
				CallbackBaseURL: "http://127.0.0.1:7780/callback",
			},
		},
		VirtualMachine: VirtualMachineConfiguration{
			Tart: TartConfiguration{
				Name:          "internkim-lab",
				Image:         "ghcr.io/cirruslabs/ubuntu:latest",
				NestedEnabled: true,
				CPUCount:      6,
				MemoryMiB:     8192,
				DiskGiB:       80,
			},
			Mattermost: MattermostConfiguration{
				ListenAddress: "127.0.0.1:8065",
			},
			SharedWorkspacePath: "/Users/test/workspace",
			MountDirectoryPath:  "/mnt/shared",
			SSHUsername:         "admin",
			SSHPassword:         "admin",
		},
		Firecracker: FirecrackerConfiguration{
			BinaryPath:         "/usr/local/bin/firecracker",
			KernelImagePath:    "/opt/blueclaw/vmlinux",
			RootfsImagePath:    "/opt/blueclaw/rootfs.ext4",
			WorkspaceImagePath: "/opt/blueclaw/workspace.ext4",
			VSockCID:           52,
		},
	})
}
