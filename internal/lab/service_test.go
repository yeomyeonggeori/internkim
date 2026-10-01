package lab

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

const runningContainerListJSON = `[{"configuration":{"id":"internkim-lab"},"status":"running","networks":[{"network":"default","ipv4Address":"192.168.65.10/24"}]}]`
const runningContainerListStatusObjectJSON = `[{"configuration":{"id":"internkim-lab"},"status":{"state":"running","networks":[{"network":"default","ipv4Address":"192.168.65.10/24"}]}}]`
const stoppedContainerListJSON = `[{"configuration":{"id":"internkim-lab"},"status":"stopped","networks":[]}]`
const missingContainerListJSON = `[]`

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

func TestImageBuildUsesContainerCreateCommand(t *testing.T) {
	commandRunner := &fakeCommandRunner{outputValue: "--cap-add"}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue := service.ImageBuild(context.Background())
	if errorValue != nil {
		t.Fatalf("expected image build to succeed: %v", errorValue)
	}
	if len(commandRunner.runCommands) != 1 {
		t.Fatalf("expected image build to emit 1 command, got %d", len(commandRunner.runCommands))
	}
	createCommand := commandRunner.runCommands[0]
	if createCommand.ExecutableName != "container" {
		t.Fatalf("expected container create command, got %q", createCommand.ExecutableName)
	}
	joinedArguments := strings.Join(createCommand.Arguments, " ")
	for _, expectedFragment := range []string{
		"create",
		"--name internkim-lab",
		"--cpus 6",
		"--memory 8192M",
		"--virtualization",
		"--cap-add ALL",
		"--tmpfs /run",
		"--tmpfs /run/lock",
		"--kernel /repo/.dependency/container-kernel/Image-6.1.68-kvm",
		"--volume /repo:/mnt/shared/workspace",
		"ubuntu:24.04",
	} {
		if !strings.Contains(joinedArguments, expectedFragment) {
			t.Fatalf("expected create arguments to contain %q, got %v", expectedFragment, createCommand.Arguments)
		}
	}
	bootstrapScript := createCommand.Arguments[len(createCommand.Arguments)-1]
	for _, expectedFragment := range []string{
		"for attempt in 1 2 3",
		"apt-get update && apt-get install -y systemd systemd-sysv openssh-server sudo rsync curl jq make",
		"if [ \"$attempt\" -eq 3 ]; then return 1; fi",
		"sleep $((attempt * 2))",
		"useradd -m -s /bin/bash admin",
		"echo 'admin:admin' | chpasswd",
		"exec /lib/systemd/systemd",
	} {
		if !strings.Contains(bootstrapScript, expectedFragment) {
			t.Fatalf("expected bootstrap script to contain %q, got %q", expectedFragment, bootstrapScript)
		}
	}
}

func TestImageBuildOmitsUnsupportedContainerCapabilityOption(t *testing.T) {
	commandRunner := &fakeCommandRunner{outputValue: "--tmpfs"}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue := service.ImageBuild(context.Background())
	if errorValue != nil {
		t.Fatalf("expected image build to succeed: %v", errorValue)
	}
	joinedArguments := strings.Join(commandRunner.runCommands[0].Arguments, " ")
	if strings.Contains(joinedArguments, "--cap-add") {
		t.Fatalf("unsupported capability option in create arguments: %v", commandRunner.runCommands[0].Arguments)
	}
	if !strings.Contains(joinedArguments, "--tmpfs /run --tmpfs /run/lock") {
		t.Fatalf("expected tmpfs mounts in create arguments: %v", commandRunner.runCommands[0].Arguments)
	}
}

func TestVirtualMachineUpSkipsImageBuildWhenVirtualMachineRunning(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValue: runningContainerListJSON,
	}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue := service.VirtualMachineUp(context.Background())
	if errorValue != nil {
		t.Fatalf("expected vm up to succeed: %v", errorValue)
	}
	if len(commandRunner.runCommands) != 0 {
		t.Fatalf("expected running container to skip create and start, got %d run commands", len(commandRunner.runCommands))
	}
}

func TestVirtualMachineUpParsesStatusObject(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValue: runningContainerListStatusObjectJSON,
	}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue := service.VirtualMachineUp(context.Background())
	if errorValue != nil {
		t.Fatalf("expected vm up to succeed: %v", errorValue)
	}
	if len(commandRunner.runCommands) != 0 {
		t.Fatalf("expected running container to skip create and start, got %d run commands", len(commandRunner.runCommands))
	}
}

func TestVirtualMachineUpCreatesMissingVirtualMachine(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValues: []string{missingContainerListJSON, "--cap-add", stoppedContainerListJSON, runningContainerListJSON},
		outputValue:  runningContainerListJSON,
	}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue := service.VirtualMachineUp(context.Background())
	if errorValue != nil {
		t.Fatalf("expected vm up to succeed: %v", errorValue)
	}
	if len(commandRunner.runCommands) != 3 {
		t.Fatalf("expected create, start, and ssh probe commands, got %d", len(commandRunner.runCommands))
	}
	if commandRunner.runCommands[0].Arguments[0] != "create" {
		t.Fatalf("expected create command, got %v", commandRunner.runCommands[0].Arguments)
	}
	startCommand := commandRunner.runCommands[1]
	if strings.Join(startCommand.Arguments, " ") != "start internkim-lab" {
		t.Fatalf("expected container start command, got %v", startCommand.Arguments)
	}
	if len(commandRunner.startCommands) != 0 {
		t.Fatalf("expected no detached start commands, got %d", len(commandRunner.startCommands))
	}
}

func TestVirtualMachineUpStartsStoppedVirtualMachine(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValues: []string{stoppedContainerListJSON, stoppedContainerListJSON, runningContainerListJSON},
		outputValue:  runningContainerListJSON,
	}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue := service.VirtualMachineUp(context.Background())
	if errorValue != nil {
		t.Fatalf("expected vm up to succeed: %v", errorValue)
	}
	if len(commandRunner.runCommands) != 2 {
		t.Fatalf("expected start and ssh probe commands for existing container, got %d", len(commandRunner.runCommands))
	}
	if strings.Join(commandRunner.runCommands[0].Arguments, " ") != "start internkim-lab" {
		t.Fatalf("expected container start command, got %v", commandRunner.runCommands[0].Arguments)
	}
}

func TestVirtualMachineUpReturnsRunFailureBeforeIPAddressTimeout(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValues: []string{missingContainerListJSON, "--cap-add", stoppedContainerListJSON},
		runErrors:    []error{nil, errors.New("container start failed")},
	}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue := service.VirtualMachineUp(context.Background())
	if errorValue == nil {
		t.Fatalf("expected vm up to fail")
	}
	if !strings.Contains(errorValue.Error(), "start internkim-lab") {
		t.Fatalf("expected container start command in error, got %q", errorValue.Error())
	}
	if strings.Contains(errorValue.Error(), "ip address") {
		t.Fatalf("expected run failure before ip timeout, got %q", errorValue.Error())
	}
}

func TestVirtualMachineSSHUsesConfiguredPasswordAuthentication(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValue: runningContainerListJSON,
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
	if command.ExecutableName != "sshpass" {
		t.Fatalf("expected vm ssh to use sshpass, got %q", command.ExecutableName)
	}
	joinedArguments := strings.Join(command.Arguments, " ")
	for _, expectedFragment := range []string{"-p admin", "ssh", "StrictHostKeyChecking=no", "LogLevel=ERROR", "ControlMaster=auto", "ControlPersist=600", "ControlPath=", "admin@192.168.65.10", "cd /mnt/shared && true"} {
		if !strings.Contains(joinedArguments, expectedFragment) {
			t.Fatalf("expected vm ssh arguments to contain %q, got %v", expectedFragment, command.Arguments)
		}
	}
}

func TestSSHControlPathIsStableAndIsolatedByContainer(t *testing.T) {
	configuration := buildTestConfiguration()
	firstService := NewService(configuration, &fakeCommandRunner{}, "/repo")
	secondService := NewService(configuration, &fakeCommandRunner{}, "/repo")
	configuration.VirtualMachine.Container.Name = "another-fleet"
	otherService := NewService(configuration, &fakeCommandRunner{}, "/repo")

	if firstService.sshControlPath() != secondService.sshControlPath() {
		t.Fatal("same container must reuse one SSH control path")
	}
	if firstService.sshControlPath() == otherService.sshControlPath() {
		t.Fatal("different containers must not share an SSH control path")
	}
	if len(firstService.sshControlPath()) >= 100 {
		t.Fatalf("SSH control path is too long: %s", firstService.sshControlPath())
	}
}

func TestVirtualMachineDiagnosticsIncludesRecoveryCommands(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValues: []string{stoppedContainerListJSON, "apt-get update failed\nnetwork unreachable"},
	}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	diagnostics := service.VirtualMachineDiagnostics(context.Background())
	for _, expectedFragment := range []string{"container diagnostics", "internkim-lab", "container ls", "boot log", "apt-get update failed", "network unreachable", "internkim lab vm-down", "container system start"} {
		if !strings.Contains(diagnostics, expectedFragment) {
			t.Fatalf("expected diagnostics to contain %q, got:\n%s", expectedFragment, diagnostics)
		}
	}
	if len(commandRunner.outputCommands) != 2 || strings.Join(commandRunner.outputCommands[1].Arguments, " ") != "logs --boot -n 120 internkim-lab" {
		t.Fatalf("expected bounded container boot log command, got %+v", commandRunner.outputCommands)
	}
}

func buildTestConfiguration() Configuration {
	return applyDefaultConfiguration(Configuration{
		Host: HostConfiguration{
			Mode: "single-mac",
		},
		VirtualMachine: VirtualMachineConfiguration{
			Container: ContainerConfiguration{
				Name:      "internkim-lab",
				Image:     "ubuntu:24.04",
				CPUCount:  6,
				MemoryMiB: 8192,
			},
			SharedWorkspacePath: "/Users/test/workspace",
			MountDirectoryPath:  "/mnt/shared",
			SSHUsername:         "admin",
			SSHPassword:         "admin",
		},
	})
}

// The shared builder container outlives the checkout it was created for. One
// left pointing at a directory that has since been deleted cannot start, and
// the container runtime reports an invalid state rather than the stale mount.
func TestABuilderSharingADifferentWorkspaceIsNotReused(t *testing.T) {
	entry := containerListEntry{Configuration: containerListEntryConfiguration{
		ID:     "internkim-lab",
		Mounts: []containerListEntryMount{{Destination: "/mnt/shared/workspace", Source: "/gone/scratchpad/deploy-main/"}},
	}}

	if entry.sharesWorkspaceAt("/repo") {
		t.Fatal("expected a builder mounted elsewhere to be recreated")
	}
	if !entry.sharesWorkspaceAt("/gone/scratchpad/deploy-main") {
		t.Fatal("expected a trailing slash to make no difference")
	}
}

func TestABuilderTheListReportsNoMountsForIsLeftAlone(t *testing.T) {
	entry := containerListEntry{Configuration: containerListEntryConfiguration{ID: "internkim-lab"}}

	if !entry.sharesWorkspaceAt("/repo") {
		t.Fatal("expected no evidence to mean no recreation")
	}
}
