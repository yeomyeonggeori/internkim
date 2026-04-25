package lab

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeCommandRunner struct {
	runCommands    []ExecutableCommand
	startCommands  []ExecutableCommand
	outputCommands []ExecutableCommand
	outputValue    string
	outputValues   []string
	outputError    error
	runError       error
	startError     error
}

func (fakeCommandRunner *fakeCommandRunner) Run(ctx context.Context, executableCommand ExecutableCommand) error {
	_ = ctx
	fakeCommandRunner.runCommands = append(fakeCommandRunner.runCommands, executableCommand)
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
	if len(commandRunner.runCommands) != 1 {
		t.Fatalf("expected one remote script execution, got %d", len(commandRunner.runCommands))
	}
	if commandRunner.runCommands[0].ExecutableName != "/repo/bin/sshpass" {
		t.Fatalf("expected repo sshpass invocation, got %q", commandRunner.runCommands[0].ExecutableName)
	}
	if !strings.HasSuffix(commandRunner.runCommands[0].StandardInputPath, "lab/scripts/provision-ubuntu.sh") {
		t.Fatalf("expected provision script path, got %q", commandRunner.runCommands[0].StandardInputPath)
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
	if len(commandRunner.runCommands) != 2 {
		t.Fatalf("expected provision and setup commands, got %d", len(commandRunner.runCommands))
	}
	setupCommand := commandRunner.runCommands[1]
	if setupCommand.ExecutableName != "/repo/internkim" {
		t.Fatalf("expected setup command to use current executable, got %q", setupCommand.ExecutableName)
	}
	if strings.Join(setupCommand.Arguments, " ") != "setup --ssh --host 10.0.0.5 --user admin --password admin --skip wifi" {
		t.Fatalf("unexpected setup arguments: %v", setupCommand.Arguments)
	}
	if setupCommand.EnvironmentVariables != nil {
		t.Fatalf("expected simulation setup to inherit host environment without fake values")
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

	setupCommand := commandRunner.runCommands[1]
	if strings.Join(setupCommand.Arguments, " ") != "setup --ssh --host 10.0.0.5 --user admin --password admin --only mattermost" {
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
	if len(commandRunner.runCommands) != 6 {
		t.Fatalf("expected provision, setup, and four default scenario commands, got %d", len(commandRunner.runCommands))
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
