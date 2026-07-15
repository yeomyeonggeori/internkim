package lab

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

func TestProvisionUbuntuUsesRemoteScriptExecution(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValue: runningContainerListJSON,
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

func TestProvisionScriptsAcceptBindMountedWorkspace(t *testing.T) {
	relativeScriptPaths := []string{
		"lab/scripts/provision-ubuntu.sh",
		"lab/scripts/provision-blueclaw-dev-session.sh",
		"lab/scripts/provision-blueclaw-runtime-builder.sh",
		"lab/scripts/check-blueclaw-runtime-builder.sh",
		".dependency/blueclaw/lab/scripts/provision-ubuntu.sh",
	}
	for _, relativeScriptPath := range relativeScriptPaths {
		scriptContent := readRepositoryScript(t, relativeScriptPath)
		if !strings.Contains(scriptContent, `"$mount_directory_path/workspace"`) {
			t.Fatalf("expected %s to accept bind-mounted workspace", relativeScriptPath)
		}
		if strings.Contains(scriptContent, "mount | grep -q") {
			t.Fatalf("expected %s to use fixed-string mount checks", relativeScriptPath)
		}
	}
}

func TestProvisionUbuntuRestartsReadOnlyVirtualMachineOnce(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValue: runningContainerListJSON,
		outputValues: []string{
			runningContainerListJSON,
			runningContainerListJSON,
			runningContainerListJSON,
			runningContainerListJSON,
			runningContainerListJSON,
			stoppedContainerListJSON,
			stoppedContainerListJSON,
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
		t.Fatalf("expected ubuntu provision to recover after container restart: %v", errorValue)
	}
	if len(commandRunner.runCommands) < 5 {
		t.Fatalf("expected restart and provision commands, got %d", len(commandRunner.runCommands))
	}
	if commandRunner.runCommands[2].Arguments[0] != "stop" {
		t.Fatalf("expected container stop after read-only check, got %v", commandRunner.runCommands[2].Arguments)
	}
	startCommandCount := 0
	for _, command := range commandRunner.runCommands {
		if len(command.Arguments) > 0 && command.Arguments[0] == "start" {
			startCommandCount++
		}
	}
	if startCommandCount != 1 {
		t.Fatalf("expected one container restart, got %d", startCommandCount)
	}
}

func TestRuntimeBuilderPrepareAvoidsSharedWorkspaceSync(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValue: runningContainerListJSON,
	}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue := service.RuntimeBuilderPrepare(context.Background())
	if errorValue != nil {
		t.Fatalf("expected runtime builder prepare to succeed: %v", errorValue)
	}
	for _, command := range commandRunner.runCommands {
		if command.ExecutableName == "rsync" {
			t.Fatalf("expected runtime builder prepare to avoid shared workspace rsync, got %v", command.Arguments)
		}
	}
	joinedScripts := make([]string, 0, len(commandRunner.runCommands))
	for _, command := range commandRunner.runCommands {
		if command.StandardInputPath != "" {
			joinedScripts = append(joinedScripts, command.StandardInputPath)
		}
	}
	joinedScriptPaths := strings.Join(joinedScripts, " ")
	for _, expectedFragment := range []string{"provision-ubuntu.sh", "provision-blueclaw-runtime-builder.sh", "check-blueclaw-runtime-builder.sh"} {
		if !strings.Contains(joinedScriptPaths, expectedFragment) {
			t.Fatalf("expected runtime builder scripts to include %q, got %v", expectedFragment, joinedScripts)
		}
	}
}

func TestRuntimeBuilderCheckUsesDedicatedBuilderScript(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValue: runningContainerListJSON,
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
	if command.ExecutableName != "/repo/bin/sshpass" {
		t.Fatalf("expected vm ssh to use repo sshpass, got %q", command.ExecutableName)
	}
	joinedArguments := strings.Join(command.Arguments, " ")
	for _, expectedFragment := range []string{"-p admin", "ssh", "StrictHostKeyChecking=no", "LogLevel=ERROR", "ControlMaster=auto", "ControlPersist=600", "ControlPath=", "admin@192.168.65.10", "cd /mnt/shared && true"} {
		if !strings.Contains(joinedArguments, expectedFragment) {
			t.Fatalf("expected vm ssh arguments to contain %q, got %v", expectedFragment, command.Arguments)
		}
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

func TestSetupUsesCurrentExecutableWithHostOverride(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValue: runningContainerListJSON,
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
	if strings.Join(setupCommand.Arguments, " ") != "setup --board lab --ssh --host 192.168.65.10 --user admin --password admin --skip wifi" {
		t.Fatalf("unexpected setup arguments: %v", setupCommand.Arguments)
	}
	if setupCommand.EnvironmentVariables["INTERNKIM_BLUECLAW_USE_LOCAL"] != "1" {
		t.Fatalf("expected simulation setup to build local Blueclaw changes, got %v", setupCommand.EnvironmentVariables)
	}
}

func TestSetupPassesSelectorArgumentsWithoutDefaultForce(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValue: runningContainerListJSON,
	}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue := service.Setup(context.Background(), "/repo/internkim", []string{"--only", "mattermost"})
	if errorValue != nil {
		t.Fatalf("expected setup to succeed: %v", errorValue)
	}

	setupCommand := commandRunner.runCommands[4]
	if strings.Join(setupCommand.Arguments, " ") != "setup --board lab --ssh --host 192.168.65.10 --user admin --password admin --only mattermost" {
		t.Fatalf("unexpected setup arguments: %v", setupCommand.Arguments)
	}
}

func TestSetupSimulationUsesSimulationTargetState(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValue: runningContainerListJSON,
	}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue := service.SetupSimulation(context.Background(), "/repo/internkim", []string{"--only", "services"})
	if errorValue != nil {
		t.Fatalf("expected simulation setup to succeed: %v", errorValue)
	}

	setupCommand := commandRunner.runCommands[4]
	if strings.Join(setupCommand.Arguments, " ") != "setup --board sim --ssh --host 192.168.65.10 --user admin --password admin --only services" {
		t.Fatalf("unexpected setup arguments: %v", setupCommand.Arguments)
	}
}

func TestScenarioEndToEndRunsSetupAndScenarios(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValue: runningContainerListJSON,
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
		outputValue: runningContainerListJSON,
	}
	service := NewService(buildTestConfiguration(), commandRunner, "/repo")

	errorValue := service.ScenarioSimulationEndToEnd(context.Background(), "/repo/internkim", []string{"--only", "services"})
	if errorValue != nil {
		t.Fatalf("expected simulation scenario to succeed: %v", errorValue)
	}

	setupCommand := commandRunner.runCommands[4]
	if strings.Join(setupCommand.Arguments, " ") != "setup --board sim --ssh --host 192.168.65.10 --user admin --password admin --only services" {
		t.Fatalf("unexpected setup arguments: %v", setupCommand.Arguments)
	}
}

func TestPrintSimulationPlanDoesNotCreateOrStartMissingVirtualMachine(t *testing.T) {
	commandRunner := &fakeCommandRunner{
		outputValue: missingContainerListJSON,
	}
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

func readRepositoryScript(t *testing.T, relativeScriptPath string) string {
	t.Helper()
	scriptPath := filepath.Join("..", "..", relativeScriptPath)
	documentBytes, errorValue := os.ReadFile(scriptPath)
	if errorValue != nil {
		t.Fatalf("read %s: %v", relativeScriptPath, errorValue)
	}
	return string(documentBytes)
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
			Container: ContainerConfiguration{
				Name:      "internkim-lab",
				Image:     "ubuntu:24.04",
				CPUCount:  6,
				MemoryMiB: 8192,
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
