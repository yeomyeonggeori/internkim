package lab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var errorUnsupportedHostMode = errors.New("unsupported host mode")

type tartVirtualMachineListEntry struct {
	Name    string `json:"Name"`
	Running bool   `json:"Running"`
}

type Service struct {
	configuration      Configuration
	commandRunner      CommandRunner
	repositoryRootPath string
}

func NewService(configuration Configuration, commandRunner CommandRunner, repositoryRootPath string) Service {
	return Service{
		configuration:      applyRuntimeConfiguration(configuration, repositoryRootPath),
		commandRunner:      commandRunner,
		repositoryRootPath: repositoryRootPath,
	}
}

func (service Service) ImageBuild(ctx context.Context) error {
	for _, executableCommand := range service.buildImageBuildCommands() {
		errorValue := service.commandRunner.Run(ctx, executableCommand)
		if errorValue != nil {
			return fmt.Errorf("run %q: %w", executableCommand.String(), errorValue)
		}
	}

	return nil
}

func (service Service) VirtualMachineExists(ctx context.Context) (bool, error) {
	output, errorValue := service.commandRunner.Output(ctx, service.buildVirtualMachineListCommand())
	if errorValue != nil {
		return false, fmt.Errorf("run %q: %w", service.buildVirtualMachineListCommand().String(), errorValue)
	}

	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == service.configuration.VirtualMachine.Tart.Name {
			return true, nil
		}
	}

	return false, nil
}

func (service Service) VirtualMachineRunning(ctx context.Context) (bool, error) {
	output, errorValue := service.commandRunner.Output(ctx, service.buildVirtualMachineListJSONCommand())
	if errorValue != nil {
		return false, fmt.Errorf("run %q: %w", service.buildVirtualMachineListJSONCommand().String(), errorValue)
	}

	var virtualMachines []tartVirtualMachineListEntry
	if errorValue := json.Unmarshal([]byte(output), &virtualMachines); errorValue != nil {
		return false, fmt.Errorf("parse Tart VM list: %w", errorValue)
	}

	for _, virtualMachine := range virtualMachines {
		if virtualMachine.Name == service.configuration.VirtualMachine.Tart.Name {
			return virtualMachine.Running, nil
		}
	}

	return false, nil
}

func (service Service) EnsureVirtualMachineImage(ctx context.Context) error {
	fmt.Println("checking Tart VM")

	hasVirtualMachine, errorValue := service.VirtualMachineExists(ctx)
	if errorValue != nil {
		return errorValue
	}
	if hasVirtualMachine {
		return nil
	}

	fmt.Printf("creating VM %q\n", service.configuration.VirtualMachine.Tart.Name)
	if errorValue := service.ImageBuild(ctx); errorValue != nil {
		return fmt.Errorf("create VM %q: %w", service.configuration.VirtualMachine.Tart.Name, errorValue)
	}

	return nil
}

func (service Service) VirtualMachineUp(ctx context.Context) error {
	if errorValue := service.EnsureVirtualMachineImage(ctx); errorValue != nil {
		return errorValue
	}

	isVirtualMachineRunning, errorValue := service.VirtualMachineRunning(ctx)
	if errorValue != nil {
		return errorValue
	}
	if isVirtualMachineRunning {
		virtualMachineIPAddress, errorValue := service.resolveVirtualMachineIPAddress(ctx)
		if errorValue == nil && strings.TrimSpace(virtualMachineIPAddress) != "" {
			return nil
		}
	}

	fmt.Printf("starting VM %q\n", service.configuration.VirtualMachine.Tart.Name)
	errorValue = service.commandRunner.Start(ctx, service.buildVirtualMachineUpCommand())
	if errorValue != nil {
		return fmt.Errorf("run %q: %w", service.buildVirtualMachineUpCommand().String(), errorValue)
	}

	fmt.Println("waiting for IP")
	if errorValue := service.waitForVirtualMachineIPAddress(ctx); errorValue != nil {
		return fmt.Errorf("wait for VM %q IP address: %w", service.configuration.VirtualMachine.Tart.Name, errorValue)
	}

	return nil
}

func (service Service) VirtualMachineDown(ctx context.Context) error {
	return service.commandRunner.Run(ctx, service.buildVirtualMachineDownCommand())
}

func (service Service) VirtualMachineSSH(ctx context.Context, remoteArguments []string) error {
	virtualMachineIPAddress, errorValue := service.resolveVirtualMachineIPAddress(ctx)
	if errorValue != nil {
		return errorValue
	}

	commandArguments := []string{service.configuration.VirtualMachine.SSHUsername + "@" + virtualMachineIPAddress}
	commandArguments = append(commandArguments, remoteArguments...)

	return service.commandRunner.Run(ctx, ExecutableCommand{
		ExecutableName:       "ssh",
		Arguments:            commandArguments,
		WorkingDirectoryPath: service.repositoryRootPath,
	})
}

func (service Service) VirtualMachineStatus(ctx context.Context) (string, error) {
	isVirtualMachineRunning, errorValue := service.VirtualMachineRunning(ctx)
	if errorValue != nil {
		return "", errorValue
	}
	if !isVirtualMachineRunning {
		return "stopped", nil
	}

	return "running", nil
}

func (service Service) VirtualMachineIPAddress(ctx context.Context) (string, error) {
	return service.resolveVirtualMachineIPAddress(ctx)
}

func (service Service) ProvisionUbuntu(ctx context.Context) error {
	if errorValue := service.VirtualMachineUp(ctx); errorValue != nil {
		return errorValue
	}
	if errorValue := service.waitForVirtualMachineSSH(ctx); errorValue != nil {
		return errorValue
	}

	fmt.Println("provisioning Ubuntu")
	return service.runRemoteScript(ctx, filepath.Join("lab", "scripts", "provision-ubuntu.sh"), []string{
		service.configuration.VirtualMachine.MountDirectoryPath,
	})
}

func (service Service) Setup(ctx context.Context, executablePath string, setupArguments []string) error {
	if errorValue := service.ProvisionUbuntu(ctx); errorValue != nil {
		return errorValue
	}

	fmt.Println("running setup")
	virtualMachineIPAddress, errorValue := service.resolveVirtualMachineIPAddress(ctx)
	if errorValue != nil {
		return errorValue
	}

	arguments := service.buildSetupArguments(virtualMachineIPAddress, setupArguments)

	return service.commandRunner.Run(ctx, ExecutableCommand{
		ExecutableName:       executablePath,
		Arguments:            arguments,
		WorkingDirectoryPath: service.repositoryRootPath,
		EnvironmentVariables: service.setupEnvironmentVariables(),
	})
}

func (service Service) ScenarioMattermost(ctx context.Context) error {
	return service.runRemoteScript(ctx, filepath.Join("lab", "scripts", "scenario-mattermost.sh"), []string{
		service.configuration.VirtualMachine.Mattermost.ListenAddress,
		service.configuration.VirtualMachine.MountDirectoryPath,
	})
}

func (service Service) ScenarioGoogle(ctx context.Context) error {
	return service.runRemoteScript(ctx, filepath.Join("lab", "scripts", "scenario-google.sh"), []string{
		service.configuration.VirtualMachine.MountDirectoryPath,
	})
}

func (service Service) ScenarioCloudflare(ctx context.Context) error {
	return service.runRemoteScript(ctx, filepath.Join("lab", "scripts", "scenario-cloudflare.sh"), []string{
		service.configuration.VirtualMachine.MountDirectoryPath,
	})
}

func (service Service) ScenarioEndToEnd(ctx context.Context, executablePath string, setupArguments []string) error {
	if errorValue := service.Setup(ctx, executablePath, setupArguments); errorValue != nil {
		return errorValue
	}
	if containsSetupSelector(setupArguments) {
		return nil
	}
	if errorValue := service.runRemoteScript(ctx, filepath.Join("lab", "scripts", "scenario-e2e.sh"), []string{
		service.configuration.VirtualMachine.MountDirectoryPath,
	}); errorValue != nil {
		return errorValue
	}
	if errorValue := service.ScenarioMattermost(ctx); errorValue != nil {
		return errorValue
	}
	if !service.shouldSkipSimulationStep("google", setupArguments) {
		if errorValue := service.ScenarioGoogle(ctx); errorValue != nil {
			return errorValue
		}
	}
	return service.ScenarioCloudflare(ctx)
}

func (service Service) PrintSimulationPlan(ctx context.Context, executablePath string, setupArguments []string) error {
	fmt.Println("simulation plan")

	hasVirtualMachine, errorValue := service.VirtualMachineExists(ctx)
	if errorValue != nil {
		return errorValue
	}
	if !hasVirtualMachine {
		fmt.Printf("VM %q: missing\n", service.configuration.VirtualMachine.Tart.Name)
		fmt.Println("inner setup plan: unavailable until the VM exists")
		return nil
	}

	fmt.Printf("VM %q: exists\n", service.configuration.VirtualMachine.Tart.Name)

	virtualMachineIPAddress, errorValue := service.resolveVirtualMachineIPAddress(ctx)
	if errorValue != nil || strings.TrimSpace(virtualMachineIPAddress) == "" {
		fmt.Println("inner setup plan: unavailable until the VM is running")
		return nil
	}

	return service.commandRunner.Run(ctx, ExecutableCommand{
		ExecutableName:       executablePath,
		Arguments:            service.buildSetupArguments(virtualMachineIPAddress, setupArguments),
		WorkingDirectoryPath: service.repositoryRootPath,
		EnvironmentVariables: service.setupEnvironmentVariables(),
	})
}

func (service Service) buildSetupArguments(virtualMachineIPAddress string, setupArguments []string) []string {
	arguments := []string{
		"setup",
		"--board",
		"lab",
		"--ssh",
		"--host",
		virtualMachineIPAddress,
		"--user",
		service.configuration.VirtualMachine.SSHUsername,
		"--password",
		service.configuration.VirtualMachine.SSHPassword,
	}
	arguments = append(arguments, setupArguments...)
	if defaultSkippedSteps := simulationDefaultSkippedSteps(setupArguments); len(defaultSkippedSteps) > 0 {
		arguments = append(arguments, "--skip", strings.Join(defaultSkippedSteps, ","))
	}
	return arguments
}

func (service Service) shouldSkipSimulationStep(stepName string, setupArguments []string) bool {
	for _, skippedStep := range simulationSkippedSteps(setupArguments) {
		if skippedStep == stepName {
			return true
		}
	}

	return false
}

func simulationSkippedSteps(setupArguments []string) []string {
	skippedSteps := simulationDefaultSkippedSteps(setupArguments)
	for index, argument := range setupArguments {
		if argument == "--skip" && index+1 < len(setupArguments) {
			skippedSteps = append(skippedSteps, strings.Split(setupArguments[index+1], ",")...)
			continue
		}
		if strings.HasPrefix(argument, "--skip=") {
			skippedSteps = append(skippedSteps, strings.Split(strings.TrimPrefix(argument, "--skip="), ",")...)
		}
	}

	var normalizedSkippedSteps []string
	for _, skippedStep := range skippedSteps {
		if normalizedSkippedStep := strings.TrimSpace(skippedStep); normalizedSkippedStep != "" {
			normalizedSkippedSteps = append(normalizedSkippedSteps, normalizedSkippedStep)
		}
	}

	return normalizedSkippedSteps
}

func simulationDefaultSkippedSteps(setupArguments []string) []string {
	if containsSetupSelector(setupArguments) {
		return nil
	}

	return []string{"wifi"}
}

func containsSetupForce(arguments []string) bool {
	for _, argument := range arguments {
		if argument == "--force" || argument == "--force-all" {
			return true
		}
	}
	return false
}

func containsSetupSelector(arguments []string) bool {
	for _, argument := range arguments {
		if argument == "--only" || strings.HasPrefix(argument, "--only=") ||
			argument == "--from" || strings.HasPrefix(argument, "--from=") ||
			argument == "--skip" || strings.HasPrefix(argument, "--skip=") {
			return true
		}
	}
	return false
}

func containsSetupPlan(arguments []string) bool {
	for _, argument := range arguments {
		if argument == "--plan" {
			return true
		}
	}

	return false
}

func (service Service) buildVirtualMachineListCommand() ExecutableCommand {
	return ExecutableCommand{
		ExecutableName: service.configuration.VirtualMachine.Tart.BinaryPath,
		Arguments: []string{
			"list",
			"--source",
			"local",
			"--quiet",
		},
		WorkingDirectoryPath: service.repositoryRootPath,
	}
}

func (service Service) buildVirtualMachineListJSONCommand() ExecutableCommand {
	return ExecutableCommand{
		ExecutableName: service.configuration.VirtualMachine.Tart.BinaryPath,
		Arguments: []string{
			"list",
			"--source",
			"local",
			"--format",
			"json",
		},
		WorkingDirectoryPath: service.repositoryRootPath,
	}
}

func (service Service) buildImageBuildCommands() []ExecutableCommand {
	return []ExecutableCommand{
		{
			ExecutableName: service.configuration.VirtualMachine.Tart.BinaryPath,
			Arguments: []string{
				"clone",
				service.configuration.VirtualMachine.Tart.Image,
				service.configuration.VirtualMachine.Tart.Name,
			},
			WorkingDirectoryPath: service.repositoryRootPath,
		},
		{
			ExecutableName: service.configuration.VirtualMachine.Tart.BinaryPath,
			Arguments: []string{
				"set",
				service.configuration.VirtualMachine.Tart.Name,
				"--cpu",
				formatInteger(service.configuration.VirtualMachine.Tart.CPUCount),
				"--memory",
				formatInteger(service.configuration.VirtualMachine.Tart.MemoryMiB),
				"--disk-size",
				formatInteger(service.configuration.VirtualMachine.Tart.DiskGiB),
			},
			WorkingDirectoryPath: service.repositoryRootPath,
		},
	}
}

func (service Service) buildVirtualMachineUpCommand() ExecutableCommand {
	commandArguments := []string{"run", "--no-graphics"}
	if service.configuration.VirtualMachine.Tart.NestedEnabled {
		commandArguments = append(commandArguments, "--nested")
	}
	commandArguments = append(commandArguments,
		"--dir=workspace:"+service.sharedWorkspacePath(),
		service.configuration.VirtualMachine.Tart.Name,
	)

	return ExecutableCommand{
		ExecutableName:       service.configuration.VirtualMachine.Tart.BinaryPath,
		Arguments:            commandArguments,
		WorkingDirectoryPath: service.repositoryRootPath,
		DetachedLogPath:      filepath.Join(os.TempDir(), "internkim-lab-tart.log"),
	}
}

func (service Service) buildVirtualMachineDownCommand() ExecutableCommand {
	return ExecutableCommand{
		ExecutableName: service.configuration.VirtualMachine.Tart.BinaryPath,
		Arguments: []string{
			"stop",
			service.configuration.VirtualMachine.Tart.Name,
		},
		WorkingDirectoryPath: service.repositoryRootPath,
	}
}

func (service Service) runRemoteScript(ctx context.Context, relativeScriptPath string, scriptArguments []string) error {
	virtualMachineIPAddress, errorValue := service.resolveVirtualMachineIPAddress(ctx)
	if errorValue != nil {
		return errorValue
	}

	remoteScriptArguments := append([]string{service.configuration.VirtualMachine.SSHPassword}, scriptArguments...)
	commandArguments := []string{
		"-p",
		service.configuration.VirtualMachine.SSHPassword,
		"ssh",
		"-o",
		"StrictHostKeyChecking=no",
		"-o",
		"UserKnownHostsFile=/dev/null",
		service.configuration.VirtualMachine.SSHUsername + "@" + virtualMachineIPAddress,
		service.remoteScriptCommand(relativeScriptPath, remoteScriptArguments),
	}

	return service.commandRunner.Run(ctx, ExecutableCommand{
		ExecutableName:       service.sshpassExecutablePath(),
		Arguments:            commandArguments,
		WorkingDirectoryPath: service.repositoryRootPath,
		StandardInputPath:    filepath.Join(service.repositoryRootPath, relativeScriptPath),
	})
}

func (service Service) remoteScriptCommand(relativeScriptPath string, scriptArguments []string) string {
	command := "bash -s -- " + shellEscapeArguments(scriptArguments)
	if service.configuration.VirtualMachine.SSHUsername == "root" {
		return command
	}
	if strings.HasSuffix(relativeScriptPath, "provision-ubuntu.sh") {
		return command
	}
	return "sudo " + command
}

func (service Service) sshpassExecutablePath() string {
	repositorySSHPath := filepath.Join(service.repositoryRootPath, "bin", "sshpass")
	return repositorySSHPath
}

func (service Service) resolveVirtualMachineIPAddress(ctx context.Context) (string, error) {
	output, errorValue := service.commandRunner.Output(ctx, ExecutableCommand{
		ExecutableName: service.configuration.VirtualMachine.Tart.BinaryPath,
		Arguments: []string{
			"ip",
			service.configuration.VirtualMachine.Tart.Name,
		},
		WorkingDirectoryPath: service.repositoryRootPath,
	})
	if errorValue != nil {
		return "", errorValue
	}

	return strings.TrimSpace(output), nil
}

func (service Service) waitForVirtualMachineIPAddress(ctx context.Context) error {
	for range 60 {
		virtualMachineIPAddress, errorValue := service.resolveVirtualMachineIPAddress(ctx)
		if errorValue == nil && virtualMachineIPAddress != "" {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	return errors.New("virtual machine did not report an ip address")
}

func (service Service) waitForVirtualMachineSSH(ctx context.Context) error {
	fmt.Println("waiting for SSH")
	for range 60 {
		if service.virtualMachineSSHReady(ctx) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	return errors.New("virtual machine ssh did not become ready")
}

func (service Service) virtualMachineSSHReady(ctx context.Context) bool {
	virtualMachineIPAddress, errorValue := service.resolveVirtualMachineIPAddress(ctx)
	if errorValue != nil || virtualMachineIPAddress == "" {
		return false
	}

	errorValue = service.commandRunner.Run(ctx, ExecutableCommand{
		ExecutableName: service.sshpassExecutablePath(),
		Arguments: []string{
			"-p",
			service.configuration.VirtualMachine.SSHPassword,
			"ssh",
			"-o",
			"StrictHostKeyChecking=no",
			"-o",
			"UserKnownHostsFile=/dev/null",
			"-o",
			"ConnectTimeout=5",
			service.configuration.VirtualMachine.SSHUsername + "@" + virtualMachineIPAddress,
			"true",
		},
		WorkingDirectoryPath: service.repositoryRootPath,
	})
	return errorValue == nil
}

func shellEscapeArguments(arguments []string) string {
	escapedArguments := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		escapedArguments = append(escapedArguments, "'"+strings.ReplaceAll(argument, "'", "'\"'\"'")+"'")
	}

	return strings.Join(escapedArguments, " ")
}

func formatInteger(value int) string {
	return strconv.Itoa(value)
}

func applyRuntimeConfiguration(configuration Configuration, repositoryRootPath string) Configuration {
	if shouldUseRepositoryRootForSharedWorkspace(configuration.VirtualMachine.SharedWorkspacePath) {
		configuration.VirtualMachine.SharedWorkspacePath = repositoryRootPath
	}

	return configuration
}

func shouldUseRepositoryRootForSharedWorkspace(sharedWorkspacePath string) bool {
	trimmedSharedWorkspacePath := strings.TrimSpace(sharedWorkspacePath)
	if trimmedSharedWorkspacePath == "" {
		return true
	}
	if strings.Contains(trimmedSharedWorkspacePath, "/Users/me/") {
		return true
	}

	_, errorValue := os.Stat(trimmedSharedWorkspacePath)
	return os.IsNotExist(errorValue)
}

func (service Service) sharedWorkspacePath() string {
	return service.configuration.VirtualMachine.SharedWorkspacePath
}

func (service Service) setupEnvironmentVariables() map[string]string {
	return nil
}
