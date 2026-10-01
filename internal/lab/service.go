package lab

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const sharedWorkspaceDestinationPath = "/mnt/shared/workspace"

type containerListEntry struct {
	Configuration containerListEntryConfiguration `json:"configuration"`
	Status        containerListEntryStatus        `json:"status"`
	Networks      []containerListEntryNetwork     `json:"networks"`
}

type containerListEntryConfiguration struct {
	ID     string                    `json:"id"`
	Mounts []containerListEntryMount `json:"mounts"`
}

type containerListEntryMount struct {
	Destination string `json:"destination"`
	Source      string `json:"source"`
}

type containerListEntryStatus struct {
	State    string
	Networks []containerListEntryNetwork
}

type containerListEntryNetwork struct {
	IPv4Address string `json:"ipv4Address"`
}

func (status *containerListEntryStatus) UnmarshalJSON(data []byte) error {
	var state string
	if errorValue := json.Unmarshal(data, &state); errorValue == nil {
		status.State = state
		status.Networks = nil
		return nil
	}

	var document struct {
		State    string                      `json:"state"`
		Networks []containerListEntryNetwork `json:"networks"`
	}
	if errorValue := json.Unmarshal(data, &document); errorValue != nil {
		return errorValue
	}

	status.State = document.State
	status.Networks = document.Networks
	return nil
}

func (containerEntry containerListEntry) isRunning() bool {
	return containerEntry.Status.State == "running"
}

func (containerEntry containerListEntry) resolvedNetworks() []containerListEntryNetwork {
	if len(containerEntry.Status.Networks) > 0 {
		return containerEntry.Status.Networks
	}

	return containerEntry.Networks
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
	isCapabilityOptionSupported, errorValue := service.containerCreateSupportsCapabilityOption(ctx)
	if errorValue != nil {
		return errorValue
	}
	executableCommand := service.buildImageBuildCommand(isCapabilityOptionSupported)
	errorValue = service.commandRunner.Run(ctx, executableCommand)
	if errorValue != nil {
		return fmt.Errorf("run %q: %w", executableCommand.String(), errorValue)
	}

	return nil
}

func (service Service) containerCreateSupportsCapabilityOption(ctx context.Context) (bool, error) {
	helpCommand := ExecutableCommand{
		ExecutableName:       service.configuration.VirtualMachine.Container.BinaryPath,
		Arguments:            []string{"create", "--help"},
		WorkingDirectoryPath: service.repositoryRootPath,
	}
	helpOutput, errorValue := service.commandRunner.Output(ctx, helpCommand)
	if errorValue != nil {
		return false, fmt.Errorf("run %q: %w", helpCommand.String(), errorValue)
	}
	return strings.Contains(helpOutput, "--cap-add"), nil
}

func (service Service) VirtualMachineExists(ctx context.Context) (bool, error) {
	containerEntry, errorValue := service.findContainerListEntry(ctx)
	if errorValue != nil {
		return false, errorValue
	}

	return containerEntry != nil, nil
}

func (service Service) VirtualMachineRunning(ctx context.Context) (bool, error) {
	containerEntry, errorValue := service.findContainerListEntry(ctx)
	if errorValue != nil {
		return false, errorValue
	}
	if containerEntry == nil {
		return false, nil
	}

	return containerEntry.isRunning(), nil
}

func (service Service) findContainerListEntry(ctx context.Context) (*containerListEntry, error) {
	listCommand := service.buildContainerListCommand()
	output, errorValue := service.commandRunner.Output(ctx, listCommand)
	if errorValue != nil {
		return nil, fmt.Errorf("run %q: %w", listCommand.String(), errorValue)
	}

	var containerEntries []containerListEntry
	if errorValue := json.Unmarshal([]byte(output), &containerEntries); errorValue != nil {
		return nil, fmt.Errorf("parse container list: %w", errorValue)
	}

	for index := range containerEntries {
		if containerEntries[index].Configuration.ID == service.configuration.VirtualMachine.Container.Name {
			return &containerEntries[index], nil
		}
	}

	return nil, nil
}

func (service Service) EnsureVirtualMachineImage(ctx context.Context) error {
	fmt.Println("checking container")

	containerEntry, errorValue := service.findContainerListEntry(ctx)
	if errorValue != nil {
		return errorValue
	}
	if containerEntry != nil {
		if containerEntry.sharesWorkspaceAt(service.sharedWorkspacePath()) {
			return nil
		}
		// The shared builder outlives the checkout it was created for. One made
		// from a directory that has since been deleted cannot start at all, and
		// the failure names an invalid state rather than the stale mount.
		fmt.Printf("recreating container %q: it shares a different workspace\n", service.configuration.VirtualMachine.Container.Name)
		if errorValue := service.commandRunner.Run(ctx, service.buildContainerRemoveCommand()); errorValue != nil {
			return fmt.Errorf("remove container %q: %w", service.configuration.VirtualMachine.Container.Name, errorValue)
		}
	}

	fmt.Printf("creating container %q\n", service.configuration.VirtualMachine.Container.Name)
	if errorValue := service.ImageBuild(ctx); errorValue != nil {
		return fmt.Errorf("create container %q: %w", service.configuration.VirtualMachine.Container.Name, errorValue)
	}

	return nil
}

// A container the list reports without mounts predates this check; it is left
// alone rather than recreated on no evidence.
func (entry *containerListEntry) sharesWorkspaceAt(workspacePath string) bool {
	if len(entry.Configuration.Mounts) == 0 {
		return true
	}
	for _, mount := range entry.Configuration.Mounts {
		if mount.Destination != sharedWorkspaceDestinationPath {
			continue
		}
		return strings.TrimRight(mount.Source, "/") == strings.TrimRight(workspacePath, "/")
	}
	return true
}

func (service Service) buildContainerRemoveCommand() ExecutableCommand {
	return ExecutableCommand{
		ExecutableName:       service.configuration.VirtualMachine.Container.BinaryPath,
		Arguments:            []string{"rm", "--force", service.configuration.VirtualMachine.Container.Name},
		WorkingDirectoryPath: service.repositoryRootPath,
	}
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

	fmt.Printf("starting container %q\n", service.configuration.VirtualMachine.Container.Name)
	errorValue = service.commandRunner.Run(ctx, service.buildVirtualMachineUpCommand())
	if errorValue != nil {
		return fmt.Errorf("run %q: %w", service.buildVirtualMachineUpCommand().String(), errorValue)
	}

	fmt.Println("waiting for IP")
	if errorValue := service.waitForVirtualMachineIPAddress(ctx); errorValue != nil {
		return fmt.Errorf("wait for container %q IP address: %w\n%s", service.configuration.VirtualMachine.Container.Name, errorValue, service.VirtualMachineDiagnostics(ctx))
	}

	if errorValue := service.waitForVirtualMachineSSH(ctx); errorValue != nil {
		return fmt.Errorf("wait for container %q ssh: %w\n%s", service.configuration.VirtualMachine.Container.Name, errorValue, service.VirtualMachineDiagnostics(ctx))
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

	commandArguments := []string{
		"-p",
		service.configuration.VirtualMachine.SSHPassword,
		"ssh",
		"-o",
		"StrictHostKeyChecking=no",
		"-o",
		"UserKnownHostsFile=/dev/null",
		"-o",
		"LogLevel=ERROR",
		"-o",
		"ControlMaster=auto",
		"-o",
		"ControlPersist=600",
		"-o",
		"ControlPath=" + service.sshControlPath(),
		service.configuration.VirtualMachine.SSHUsername + "@" + virtualMachineIPAddress,
	}
	commandArguments = append(commandArguments, remoteArguments...)

	return service.commandRunner.Run(ctx, ExecutableCommand{
		ExecutableName:       "sshpass",
		Arguments:            commandArguments,
		WorkingDirectoryPath: service.repositoryRootPath,
	})
}

func (service Service) sshControlPath() string {
	identifier := sha256.Sum256([]byte(service.configuration.VirtualMachine.Container.Name))
	return filepath.Join(os.TempDir(), fmt.Sprintf("internkim-ssh-%x", identifier[:8]))
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

func (service Service) VirtualMachineDiagnostics(ctx context.Context) string {
	listOutput, listError := service.commandRunner.Output(ctx, service.buildContainerListCommand())
	bootLogOutput, bootLogError := service.commandRunner.Output(ctx, service.buildContainerBootLogCommand())

	var message strings.Builder
	message.WriteString("container diagnostics:\n")
	message.WriteString("  name: " + service.configuration.VirtualMachine.Container.Name + "\n")
	if listError == nil {
		message.WriteString("  container ls: " + strings.TrimSpace(listOutput) + "\n")
	} else {
		message.WriteString("  container ls error: " + listError.Error() + "\n")
	}
	if bootLogError != nil {
		message.WriteString("  boot log error: " + bootLogError.Error() + "\n")
	} else if strings.TrimSpace(bootLogOutput) != "" {
		message.WriteString("  boot log:\n    ")
		message.WriteString(strings.ReplaceAll(strings.TrimSpace(bootLogOutput), "\n", "\n    "))
		message.WriteString("\n")
	}
	message.WriteString("Recovery:\n")
	message.WriteString("  1. Run `internkim lab vm-down`.\n")
	message.WriteString("  2. Run `internkim lab vm-up` and confirm the container gets an IP.\n")
	message.WriteString("  3. If the container API server is down, run `container system start`, then retry `internkim lab vm-up`.\n")
	return message.String()
}

func (service Service) buildContainerListCommand() ExecutableCommand {
	return ExecutableCommand{
		ExecutableName: service.configuration.VirtualMachine.Container.BinaryPath,
		Arguments: []string{
			"ls",
			"-a",
			"--format",
			"json",
		},
		WorkingDirectoryPath: service.repositoryRootPath,
	}
}

func (service Service) buildContainerBootLogCommand() ExecutableCommand {
	return ExecutableCommand{
		ExecutableName: service.configuration.VirtualMachine.Container.BinaryPath,
		Arguments: []string{
			"logs",
			"--boot",
			"-n",
			"120",
			service.configuration.VirtualMachine.Container.Name,
		},
		WorkingDirectoryPath: service.repositoryRootPath,
	}
}

func (service Service) buildImageBuildCommand(isCapabilityOptionSupported bool) ExecutableCommand {
	arguments := []string{
		"create",
		"--name",
		service.configuration.VirtualMachine.Container.Name,
		"--cpus",
		formatInteger(service.configuration.VirtualMachine.Container.CPUCount),
		"--memory",
		formatInteger(service.configuration.VirtualMachine.Container.MemoryMiB) + "M",
		"--virtualization",
	}
	if isCapabilityOptionSupported {
		arguments = append(arguments, "--cap-add", "ALL")
	}
	arguments = append(arguments,
		"--tmpfs",
		"/run",
		"--tmpfs",
		"/run/lock",
		"--kernel",
		service.configuration.VirtualMachine.Container.KernelImagePath,
		"--volume",
		service.sharedWorkspacePath()+":"+sharedWorkspaceDestinationPath,
		service.configuration.VirtualMachine.Container.Image,
		"sh",
		"-c",
		service.buildBootstrapScript(),
	)
	return ExecutableCommand{
		ExecutableName:       service.configuration.VirtualMachine.Container.BinaryPath,
		Arguments:            arguments,
		WorkingDirectoryPath: service.repositoryRootPath,
	}
}

func (service Service) buildBootstrapScript() string {
	sshUsername := service.configuration.VirtualMachine.SSHUsername
	sshCredentials := sshUsername + ":" + service.configuration.VirtualMachine.SSHPassword
	return strings.Join([]string{
		"set -eu",
		"export DEBIAN_FRONTEND=noninteractive",
		"install_bootstrap_packages() {",
		"  for attempt in 1 2 3; do",
		"    if apt-get update && apt-get install -y systemd systemd-sysv openssh-server sudo rsync curl jq make; then return 0; fi",
		"    if [ \"$attempt\" -eq 3 ]; then return 1; fi",
		"    sleep $((attempt * 2))",
		"  done",
		"}",
		"if [ ! -x /lib/systemd/systemd ]; then install_bootstrap_packages; fi",
		"id " + sshUsername + " >/dev/null 2>&1 || useradd -m -s /bin/bash " + sshUsername,
		"echo '" + sshCredentials + "' | chpasswd",
		"printf '" + sshUsername + " ALL=(ALL) NOPASSWD:ALL\\n' > /etc/sudoers.d/" + sshUsername,
		"chmod 440 /etc/sudoers.d/" + sshUsername,
		"systemctl enable ssh >/dev/null 2>&1 || true",
		"if [ -L /etc/resolv.conf ]; then cp /etc/resolv.conf /etc/resolv.conf.static && rm /etc/resolv.conf && mv /etc/resolv.conf.static /etc/resolv.conf; fi",
		"ln -sf /dev/null /etc/systemd/system/systemd-resolved.service",
		"exec /lib/systemd/systemd",
	}, "\n")
}

func (service Service) buildVirtualMachineUpCommand() ExecutableCommand {
	return ExecutableCommand{
		ExecutableName: service.configuration.VirtualMachine.Container.BinaryPath,
		Arguments: []string{
			"start",
			service.configuration.VirtualMachine.Container.Name,
		},
		WorkingDirectoryPath: service.repositoryRootPath,
	}
}

func (service Service) buildVirtualMachineDownCommand() ExecutableCommand {
	return ExecutableCommand{
		ExecutableName: service.configuration.VirtualMachine.Container.BinaryPath,
		Arguments: []string{
			"stop",
			service.configuration.VirtualMachine.Container.Name,
		},
		WorkingDirectoryPath: service.repositoryRootPath,
	}
}

func (service Service) resolveVirtualMachineIPAddress(ctx context.Context) (string, error) {
	containerEntry, errorValue := service.findContainerListEntry(ctx)
	if errorValue != nil {
		return "", errorValue
	}
	if containerEntry == nil || !containerEntry.isRunning() {
		return "", nil
	}

	networks := containerEntry.resolvedNetworks()
	if len(networks) == 0 {
		return "", nil
	}

	return stripAddressSuffix(networks[0].IPv4Address), nil
}

func stripAddressSuffix(address string) string {
	trimmedAddress := strings.TrimSpace(address)
	if slashIndex := strings.IndexByte(trimmedAddress, '/'); slashIndex >= 0 {
		return trimmedAddress[:slashIndex]
	}

	return trimmedAddress
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
	for range 150 {
		if service.virtualMachineSSHReady(ctx) {
			return nil
		}
		isVirtualMachineRunning, errorValue := service.VirtualMachineRunning(ctx)
		if errorValue == nil && !isVirtualMachineRunning {
			return errors.New("virtual machine stopped before ssh became ready")
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
		ExecutableName: "sshpass",
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

func formatInteger(value int) string {
	return strconv.Itoa(value)
}

func applyRuntimeConfiguration(configuration Configuration, repositoryRootPath string) Configuration {
	if shouldUseRepositoryRootForSharedWorkspace(configuration.VirtualMachine.SharedWorkspacePath) {
		configuration.VirtualMachine.SharedWorkspacePath = repositoryRootPath
	}
	if strings.TrimSpace(configuration.VirtualMachine.Container.KernelImagePath) == "" {
		configuration.VirtualMachine.Container.KernelImagePath = filepath.Join(repositoryRootPath, ".dependency", "container-kernel", "Image-6.1.68-kvm")
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
