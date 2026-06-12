package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const defaultHostVirtualMachineName = "internkim-pilot-01"

type hostCommandInvocation struct {
	ExecutableName string
	Arguments      []string
	Stdin          io.Reader
	Stdout         io.Writer
	Stderr         io.Writer
	Environment    []string
}

type hostCommandExecutor interface {
	LookPath(name string) (string, error)
	CombinedOutput(invocation hostCommandInvocation) ([]byte, error)
	Run(invocation hostCommandInvocation) error
}

type operatingSystemHostCommandExecutor struct{}

func (executor operatingSystemHostCommandExecutor) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func (executor operatingSystemHostCommandExecutor) CombinedOutput(invocation hostCommandInvocation) ([]byte, error) {
	command := exec.Command(invocation.ExecutableName, invocation.Arguments...)
	command.Env = hostCommandEnvironment(invocation.Environment)
	command.Stdin = invocation.Stdin
	return command.CombinedOutput()
}

func (executor operatingSystemHostCommandExecutor) Run(invocation hostCommandInvocation) error {
	command := exec.Command(invocation.ExecutableName, invocation.Arguments...)
	command.Env = hostCommandEnvironment(invocation.Environment)
	command.Stdin = invocation.Stdin
	command.Stdout = invocation.Stdout
	command.Stderr = invocation.Stderr
	return command.Run()
}

func hostCommandEnvironment(environment []string) []string {
	if len(environment) == 0 {
		return os.Environ()
	}
	return append(os.Environ(), environment...)
}

func runHost() {
	if len(os.Args) < 3 || containsArg("--help") || containsArg("-h") {
		printHostUsage()
		return
	}
	exitCode, errorValue := executeHostCommand(os.Args[2], os.Args[3:], operatingSystemHostCommandExecutor{})
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func printHostUsage() {
	fmt.Println("Usage: internkim host <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  init      Create and bootstrap the tenant VM on this Mac")
	fmt.Println("  status    Show Mac container VM state and tenant runtime status")
	fmt.Println("  sync-cli  Cross-compile and install internkim CLI into the tenant VM")
	fmt.Println("  add-team  Provision a host-runtime tenant inside the tenant VM")
	fmt.Println("  remove-team  Remove a host-runtime tenant inside the tenant VM")
	fmt.Println("  console   Serve the local host tenant console")
}

func executeHostCommand(commandName string, arguments []string, executor hostCommandExecutor) (int, error) {
	switch commandName {
	case "init":
		return 0, executeHostInit(arguments, executor, os.Stdout, os.Stderr)
	case "status":
		return 0, executeHostStatus(arguments, executor, os.Stdout)
	case "sync-cli":
		return 0, executeHostSyncCLI(arguments, executor, os.Stdout, os.Stderr)
	case "add-team":
		return executeHostAddTeam(arguments, executor, os.Stdout, os.Stderr)
	case "remove-team":
		return executeHostRemoveTeam(arguments, executor, os.Stdout, os.Stderr)
	case "console":
		return 0, executeHostConsole(arguments, executor, os.Stdout)
	default:
		printHostUsage()
		return 0, nil
	}
}

type hostVirtualMachineOptions struct {
	VirtualMachineName string
}

func parseHostVirtualMachineOptions(commandName string, arguments []string) (hostVirtualMachineOptions, error) {
	flags := flag.NewFlagSet(commandName, flag.ContinueOnError)
	virtualMachineName := flags.String("vm", defaultHostVirtualMachineName, "container VM name")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return hostVirtualMachineOptions{}, errorValue
	}
	return hostVirtualMachineOptions{VirtualMachineName: strings.TrimSpace(*virtualMachineName)}, nil
}

func executeHostStatus(arguments []string, executor hostCommandExecutor, output io.Writer) error {
	options, errorValue := parseHostVirtualMachineOptions("host status", arguments)
	if errorValue != nil {
		return errorValue
	}
	containerPath, errorValue := resolveHostContainerPath(executor)
	if errorValue != nil {
		return errorValue
	}
	listOutput, errorValue := executor.CombinedOutput(hostContainerListInvocation(containerPath))
	if errorValue != nil {
		return fmt.Errorf("container list failed: %w", errorValue)
	}
	if errorValue := ensureHostVirtualMachineIsRunning(executor, containerPath, options.VirtualMachineName); errorValue != nil {
		return errorValue
	}
	tenants, errorValue := hostTenantSummaries(executor, containerPath, options.VirtualMachineName)
	if errorValue != nil {
		return errorValue
	}
	fmt.Fprintln(output, strings.TrimSpace(string(listOutput)))
	fmt.Fprintln(output)
	fmt.Fprintln(output, "TENANT\tPUBLIC_URL\tRUNNING")
	for _, tenant := range tenants {
		fmt.Fprintf(output, "%s\t%s\t%s\n", tenant.TenantID, tenant.PublicURL, tenant.RunningState)
	}
	return nil
}

func executeHostSyncCLI(arguments []string, executor hostCommandExecutor, output io.Writer, errorOutput io.Writer) error {
	options, errorValue := parseHostVirtualMachineOptions("host sync-cli", arguments)
	if errorValue != nil {
		return errorValue
	}
	return syncHostCLI(executor, options.VirtualMachineName, output, errorOutput)
}

type hostAddTeamOptions struct {
	TeamID             string
	VirtualMachineName string
	RemoteArguments    []string
}

type hostRemoveTeamOptions struct {
	TeamID             string
	VirtualMachineName string
	RemoteArguments    []string
}

func executeHostAddTeam(arguments []string, executor hostCommandExecutor, output io.Writer, errorOutput io.Writer) (int, error) {
	options, errorValue := parseHostAddTeamOptions(arguments)
	if errorValue != nil {
		return 1, errorValue
	}
	return runHostAddTeam(options, executor, output, errorOutput)
}

func runHostAddTeam(options hostAddTeamOptions, executor hostCommandExecutor, output io.Writer, errorOutput io.Writer) (int, error) {
	if errorValue := syncHostCLI(executor, options.VirtualMachineName, output, errorOutput); errorValue != nil {
		return 1, errorValue
	}
	containerPath, errorValue := resolveHostContainerPath(executor)
	if errorValue != nil {
		return 1, errorValue
	}
	if errorValue := ensureHostVirtualMachineIsRunning(executor, containerPath, options.VirtualMachineName); errorValue != nil {
		return 1, errorValue
	}
	invocation := hostAddTeamInvocation(containerPath, options)
	errorValue = executor.Run(hostCommandInvocation{
		ExecutableName: invocation.ExecutableName,
		Arguments:      invocation.Arguments,
		Stdout:         output,
		Stderr:         errorOutput,
	})
	if errorValue == nil {
		return 0, nil
	}
	if exitCode := hostCommandExitCode(errorValue); exitCode >= 0 {
		return exitCode, nil
	}
	return 1, errorValue
}

func executeHostRemoveTeam(arguments []string, executor hostCommandExecutor, output io.Writer, errorOutput io.Writer) (int, error) {
	options, errorValue := parseHostRemoveTeamOptions(arguments)
	if errorValue != nil {
		return 1, errorValue
	}
	return runHostRemoveTeam(options, executor, output, errorOutput)
}

func runHostRemoveTeam(options hostRemoveTeamOptions, executor hostCommandExecutor, output io.Writer, errorOutput io.Writer) (int, error) {
	if errorValue := syncHostCLI(executor, options.VirtualMachineName, output, errorOutput); errorValue != nil {
		return 1, errorValue
	}
	containerPath, errorValue := resolveHostContainerPath(executor)
	if errorValue != nil {
		return 1, errorValue
	}
	if errorValue := ensureHostVirtualMachineIsRunning(executor, containerPath, options.VirtualMachineName); errorValue != nil {
		return 1, errorValue
	}
	invocation := hostRemoveTeamInvocation(containerPath, options)
	errorValue = executor.Run(hostCommandInvocation{
		ExecutableName: invocation.ExecutableName,
		Arguments:      invocation.Arguments,
		Stdout:         output,
		Stderr:         errorOutput,
	})
	if errorValue == nil {
		return 0, nil
	}
	if exitCode := hostCommandExitCode(errorValue); exitCode >= 0 {
		return exitCode, nil
	}
	return 1, errorValue
}

func parseHostAddTeamOptions(arguments []string) (hostAddTeamOptions, error) {
	options := hostAddTeamOptions{VirtualMachineName: defaultHostVirtualMachineName}
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if argument == "--" {
			options.RemoteArguments = append(options.RemoteArguments, arguments[index+1:]...)
			break
		}
		key, value, hasInlineValue := strings.Cut(argument, "=")
		switch key {
		case "--team":
			parsedValue, nextIndex, errorValue := hostOptionValue(arguments, index, value, hasInlineValue)
			if errorValue != nil {
				return hostAddTeamOptions{}, errorValue
			}
			options.TeamID = strings.TrimSpace(parsedValue)
			index = nextIndex
		case "--display-name":
			parsedValue, nextIndex, errorValue := hostOptionValue(arguments, index, value, hasInlineValue)
			if errorValue != nil {
				return hostAddTeamOptions{}, errorValue
			}
			options.RemoteArguments = append(options.RemoteArguments, "--display-name", parsedValue)
			index = nextIndex
		case "--member":
			parsedValue, nextIndex, errorValue := hostOptionValue(arguments, index, value, hasInlineValue)
			if errorValue != nil {
				return hostAddTeamOptions{}, errorValue
			}
			options.RemoteArguments = append(options.RemoteArguments, "--member", parsedValue)
			index = nextIndex
		case "--vm":
			parsedValue, nextIndex, errorValue := hostOptionValue(arguments, index, value, hasInlineValue)
			if errorValue != nil {
				return hostAddTeamOptions{}, errorValue
			}
			options.VirtualMachineName = strings.TrimSpace(parsedValue)
			index = nextIndex
		default:
			options.RemoteArguments = append(options.RemoteArguments, argument)
		}
	}
	if options.TeamID == "" {
		return hostAddTeamOptions{}, errors.New("--team is required")
	}
	if options.VirtualMachineName == "" {
		return hostAddTeamOptions{}, errors.New("--vm is required")
	}
	return options, nil
}

func parseHostRemoveTeamOptions(arguments []string) (hostRemoveTeamOptions, error) {
	options := hostRemoveTeamOptions{VirtualMachineName: defaultHostVirtualMachineName}
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if argument == "--" {
			options.RemoteArguments = append(options.RemoteArguments, arguments[index+1:]...)
			break
		}
		key, value, hasInlineValue := strings.Cut(argument, "=")
		switch key {
		case "--team":
			parsedValue, nextIndex, errorValue := hostOptionValue(arguments, index, value, hasInlineValue)
			if errorValue != nil {
				return hostRemoveTeamOptions{}, errorValue
			}
			options.TeamID = strings.TrimSpace(parsedValue)
			index = nextIndex
		case "--vm":
			parsedValue, nextIndex, errorValue := hostOptionValue(arguments, index, value, hasInlineValue)
			if errorValue != nil {
				return hostRemoveTeamOptions{}, errorValue
			}
			options.VirtualMachineName = strings.TrimSpace(parsedValue)
			index = nextIndex
		default:
			options.RemoteArguments = append(options.RemoteArguments, argument)
		}
	}
	if options.TeamID == "" {
		return hostRemoveTeamOptions{}, errors.New("--team is required")
	}
	if options.VirtualMachineName == "" {
		return hostRemoveTeamOptions{}, errors.New("--vm is required")
	}
	return options, nil
}

func hostOptionValue(arguments []string, index int, inlineValue string, hasInlineValue bool) (string, int, error) {
	if hasInlineValue {
		if strings.TrimSpace(inlineValue) == "" {
			return "", index, errors.New(arguments[index] + " requires a value")
		}
		return inlineValue, index, nil
	}
	if index+1 >= len(arguments) || strings.HasPrefix(arguments[index+1], "--") {
		return "", index, errors.New(arguments[index] + " requires a value")
	}
	return arguments[index+1], index + 1, nil
}

func syncHostCLI(executor hostCommandExecutor, virtualMachineName string, output io.Writer, errorOutput io.Writer) error {
	containerPath, errorValue := resolveHostContainerPath(executor)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := ensureHostVirtualMachineIsRunning(executor, containerPath, virtualMachineName); errorValue != nil {
		return errorValue
	}
	temporaryFile, errorValue := os.CreateTemp("", "internkim-linux-arm64-*")
	if errorValue != nil {
		return errorValue
	}
	temporaryPath := temporaryFile.Name()
	if errorValue := temporaryFile.Close(); errorValue != nil {
		return errorValue
	}
	defer os.Remove(temporaryPath)
	if errorValue := executor.Run(hostBuildCLIInvocation(temporaryPath, output, errorOutput)); errorValue != nil {
		return fmt.Errorf("cross-compile internkim CLI failed: %w", errorValue)
	}
	binaryFile, errorValue := os.Open(filepath.Clean(temporaryPath))
	if errorValue != nil {
		return errorValue
	}
	defer binaryFile.Close()
	if errorValue := executor.Run(hostInstallCLIInvocation(containerPath, virtualMachineName, binaryFile, output, errorOutput)); errorValue != nil {
		return fmt.Errorf("install internkim CLI into VM failed: %w", errorValue)
	}
	return nil
}

func resolveHostContainerPath(executor hostCommandExecutor) (string, error) {
	containerPath, errorValue := executor.LookPath("container")
	if errorValue != nil {
		return "", errors.New("container CLI is required but was not found in PATH")
	}
	return containerPath, nil
}

func ensureHostVirtualMachineIsRunning(executor hostCommandExecutor, containerPath string, virtualMachineName string) error {
	_, errorValue := executor.CombinedOutput(hostContainerExecInvocation(containerPath, virtualMachineName, "true"))
	if errorValue != nil {
		return fmt.Errorf("VM %s is not running or container exec failed: %w", virtualMachineName, errorValue)
	}
	return nil
}

func hostTenantIDs(executor hostCommandExecutor, containerPath string, virtualMachineName string) ([]string, error) {
	output, errorValue := executor.CombinedOutput(hostContainerExecInvocation(containerPath, virtualMachineName, "ls", "-1", "/srv/internkim/tenants"))
	if errorValue != nil {
		return nil, fmt.Errorf("list tenants in VM failed: %w", errorValue)
	}
	tenantIDs := []string{}
	for _, line := range strings.Split(string(output), "\n") {
		tenantID := strings.TrimSpace(line)
		if tenantID != "" {
			tenantIDs = append(tenantIDs, tenantID)
		}
	}
	return tenantIDs, nil
}

type hostTenantStatusDocument struct {
	Manifest struct {
		TenantID           string `json:"tenantID"`
		PublicURL          string `json:"publicURL"`
		MattermostInstance struct {
			PublicURL string `json:"publicURL"`
		} `json:"mattermostInstance"`
	} `json:"manifest"`
}

type hostTenantSummary struct {
	TenantID     string `json:"tenantID"`
	PublicURL    string `json:"publicURL"`
	Running      bool   `json:"running"`
	RunningState string `json:"-"`
}

func listHostTenantSummaries(executor hostCommandExecutor, virtualMachineName string) ([]hostTenantSummary, error) {
	containerPath, errorValue := resolveHostContainerPath(executor)
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := ensureHostVirtualMachineIsRunning(executor, containerPath, virtualMachineName); errorValue != nil {
		return nil, errorValue
	}
	return hostTenantSummaries(executor, containerPath, virtualMachineName)
}

func hostTenantSummaries(executor hostCommandExecutor, containerPath string, virtualMachineName string) ([]hostTenantSummary, error) {
	tenantIDs, errorValue := hostTenantIDs(executor, containerPath, virtualMachineName)
	if errorValue != nil {
		return nil, errorValue
	}
	tenants := make([]hostTenantSummary, 0, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		tenant, errorValue := hostTenantSummaryForID(executor, containerPath, virtualMachineName, tenantID)
		if errorValue != nil {
			return nil, errorValue
		}
		tenants = append(tenants, tenant)
	}
	return tenants, nil
}

func hostTenantSummaryForID(executor hostCommandExecutor, containerPath string, virtualMachineName string, tenantID string) (hostTenantSummary, error) {
	status, errorValue := hostTenantStatus(executor, containerPath, virtualMachineName, tenantID)
	if errorValue != nil {
		return hostTenantSummary{}, errorValue
	}
	runningState := hostTenantRunningState(executor, containerPath, virtualMachineName, tenantID)
	return hostTenantSummary{
		TenantID:     status.Manifest.TenantID,
		PublicURL:    hostTenantPublicURL(status),
		Running:      runningState == "active",
		RunningState: runningState,
	}, nil
}

func hostTenantStatus(executor hostCommandExecutor, containerPath string, virtualMachineName string, tenantID string) (hostTenantStatusDocument, error) {
	output, errorValue := executor.CombinedOutput(hostContainerExecInvocation(containerPath, virtualMachineName, "internkim", "tenant", "status", "--tenant", tenantID))
	if errorValue != nil {
		return hostTenantStatusDocument{}, fmt.Errorf("tenant status failed for %s: %w", tenantID, errorValue)
	}
	var status hostTenantStatusDocument
	if errorValue := json.Unmarshal(output, &status); errorValue != nil {
		return hostTenantStatusDocument{}, errorValue
	}
	return status, nil
}

func hostTenantPublicURL(status hostTenantStatusDocument) string {
	if strings.TrimSpace(status.Manifest.PublicURL) != "" {
		return strings.TrimSpace(status.Manifest.PublicURL)
	}
	return strings.TrimSpace(status.Manifest.MattermostInstance.PublicURL)
}

func hostTenantRunningState(executor hostCommandExecutor, containerPath string, virtualMachineName string, tenantID string) string {
	output, errorValue := executor.CombinedOutput(hostContainerExecInvocation(containerPath, virtualMachineName, "systemctl", "is-active", "internkim-tenant-blueclaw-"+tenantID+".service"))
	state := strings.TrimSpace(string(output))
	if state != "" {
		return state
	}
	if errorValue != nil {
		return "inactive"
	}
	return "unknown"
}

func hostContainerListInvocation(containerPath string) hostCommandInvocation {
	return hostCommandInvocation{ExecutableName: containerPath, Arguments: []string{"list"}}
}

func hostContainerExecInvocation(containerPath string, virtualMachineName string, arguments ...string) hostCommandInvocation {
	commandArguments := append([]string{"exec", virtualMachineName}, arguments...)
	return hostCommandInvocation{ExecutableName: containerPath, Arguments: commandArguments}
}

func hostBuildCLIInvocation(outputPath string, output io.Writer, errorOutput io.Writer) hostCommandInvocation {
	return hostCommandInvocation{
		ExecutableName: "go",
		Arguments:      []string{"build", "-o", outputPath, "./cmd/internkim"},
		Stdout:         output,
		Stderr:         errorOutput,
		Environment:    []string{"GOOS=linux", "GOARCH=arm64"},
	}
}

func hostInstallCLIInvocation(containerPath string, virtualMachineName string, binaryInput io.Reader, output io.Writer, errorOutput io.Writer) hostCommandInvocation {
	return hostCommandInvocation{
		ExecutableName: containerPath,
		Arguments:      []string{"exec", "--interactive", virtualMachineName, "sh", "-c", "cat > /usr/local/bin/internkim && chmod 755 /usr/local/bin/internkim"},
		Stdin:          binaryInput,
		Stdout:         output,
		Stderr:         errorOutput,
	}
}

func hostAddTeamInvocation(containerPath string, options hostAddTeamOptions) hostCommandInvocation {
	remoteArguments := []string{
		"exec",
		options.VirtualMachineName,
		"internkim",
		"tenant",
		"provision",
		"--runtime",
		"host",
		"--tenant",
		options.TeamID,
	}
	remoteArguments = append(remoteArguments, options.RemoteArguments...)
	return hostCommandInvocation{ExecutableName: containerPath, Arguments: remoteArguments}
}

func hostRemoveTeamInvocation(containerPath string, options hostRemoveTeamOptions) hostCommandInvocation {
	remoteArguments := []string{
		"exec",
		options.VirtualMachineName,
		"internkim",
		"tenant",
		"remove",
		"--tenant",
		options.TeamID,
		"--confirm",
		options.TeamID,
	}
	remoteArguments = append(remoteArguments, options.RemoteArguments...)
	return hostCommandInvocation{ExecutableName: containerPath, Arguments: remoteArguments}
}

const hostVirtualMachineBootstrapScript = `set -eu
export DEBIAN_FRONTEND=noninteractive
if [ ! -x /lib/systemd/systemd ]; then
  updated=0
  for attempt in $(seq 1 60); do
    if apt-get update > /tmp/internkim-apt-update.log 2>&1 && ! grep -q 'Err:' /tmp/internkim-apt-update.log; then updated=1; break; fi
    sleep 5
  done
  cat /tmp/internkim-apt-update.log
  if [ "$updated" != 1 ]; then echo 'apt-get update kept failing' >&2; exit 1; fi
  apt-get install -y systemd systemd-sysv openssh-server sudo rsync curl jq make
fi
id admin >/dev/null 2>&1 || useradd -m -s /bin/bash admin
echo 'admin:admin' | chpasswd
printf 'admin ALL=(ALL) NOPASSWD:ALL\n' > /etc/sudoers.d/admin
chmod 440 /etc/sudoers.d/admin
systemctl enable ssh >/dev/null 2>&1 || true
if [ -L /etc/resolv.conf ]; then cp /etc/resolv.conf /etc/resolv.conf.static && rm /etc/resolv.conf && mv /etc/resolv.conf.static /etc/resolv.conf; fi
ln -sf /dev/null /etc/systemd/system/systemd-resolved.service
exec /lib/systemd/systemd`

type hostInitOptions struct {
	VirtualMachineName string
	CPUCount           string
	Memory             string
	Image              string
}

func parseHostInitOptions(arguments []string) (hostInitOptions, error) {
	flags := flag.NewFlagSet("host init", flag.ContinueOnError)
	virtualMachineName := flags.String("vm", defaultHostVirtualMachineName, "container VM name")
	cpuCount := flags.String("cpus", "4", "VM CPU count")
	memory := flags.String("memory", "6g", "VM memory size")
	image := flags.String("image", "docker.io/library/ubuntu:24.04", "VM base image")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return hostInitOptions{}, errorValue
	}
	return hostInitOptions{
		VirtualMachineName: strings.TrimSpace(*virtualMachineName),
		CPUCount:           strings.TrimSpace(*cpuCount),
		Memory:             strings.TrimSpace(*memory),
		Image:              strings.TrimSpace(*image),
	}, nil
}

func executeHostInit(arguments []string, executor hostCommandExecutor, output io.Writer, errorOutput io.Writer) error {
	options, errorValue := parseHostInitOptions(arguments)
	if errorValue != nil {
		return errorValue
	}
	containerPath, errorValue := resolveHostContainerPath(executor)
	if errorValue != nil {
		return errorValue
	}
	if ensureHostVirtualMachineIsRunning(executor, containerPath, options.VirtualMachineName) == nil {
		fmt.Fprintf(output, "VM %s is already running\n", options.VirtualMachineName)
		return finishHostInit(executor, options.VirtualMachineName, output, errorOutput)
	}
	if errorValue := executor.Run(hostCreateVirtualMachineInvocation(containerPath, options, output, errorOutput)); errorValue != nil {
		return fmt.Errorf("create VM %s failed: %w", options.VirtualMachineName, errorValue)
	}
	if errorValue := waitForHostVirtualMachine(executor, containerPath, options.VirtualMachineName); errorValue != nil {
		return errorValue
	}
	fmt.Fprintf(output, "VM %s is running\n", options.VirtualMachineName)
	return finishHostInit(executor, options.VirtualMachineName, output, errorOutput)
}

func finishHostInit(executor hostCommandExecutor, virtualMachineName string, output io.Writer, errorOutput io.Writer) error {
	if errorValue := syncHostCLI(executor, virtualMachineName, output, errorOutput); errorValue != nil {
		return errorValue
	}
	fmt.Fprintln(output, "internkim CLI installed into the VM")
	fmt.Fprintln(output, "Remaining manual steps before the first add-team:")
	fmt.Fprintln(output, "  1. Install tenant prerequisites in the VM: internkim binaries, /opt/internkim/blueclaw-runtime payload, /opt/mattermost, cloudflared")
	fmt.Fprintln(output, "  2. Place Cloudflare account/tunnel credentials for sync-cloudflare-tunnel")
	fmt.Fprintln(output, "  3. Run: internkim host add-team --team <id> --member <email:name[:password]>")
	return nil
}

func waitForHostVirtualMachine(executor hostCommandExecutor, containerPath string, virtualMachineName string) error {
	deadline := time.Now().Add(20 * time.Minute)
	for {
		output, errorValue := executor.CombinedOutput(hostContainerExecInvocation(containerPath, virtualMachineName, "systemctl", "is-system-running"))
		state := strings.TrimSpace(string(output))
		if errorValue == nil && (state == "running" || state == "degraded") {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("VM %s did not reach a running systemd state in time (last: %s); the bootstrap may still be running - check 'container logs %s' and re-run 'internkim host init' once the VM network is up", virtualMachineName, state, virtualMachineName)
		}
		time.Sleep(5 * time.Second)
	}
}

func hostCreateVirtualMachineInvocation(containerPath string, options hostInitOptions, output io.Writer, errorOutput io.Writer) hostCommandInvocation {
	return hostCommandInvocation{
		ExecutableName: containerPath,
		Arguments: []string{
			"run",
			"--detach",
			"--name", options.VirtualMachineName,
			"--cpus", options.CPUCount,
			"--memory", options.Memory,
			"--virtualization",
			options.Image,
			"sh", "-c", hostVirtualMachineBootstrapScript,
		},
		Stdout: output,
		Stderr: errorOutput,
	}
}

func hostCommandExitCode(errorValue error) int {
	type exitCoder interface {
		ExitCode() int
	}
	var exitError exitCoder
	if errors.As(errorValue, &exitError) {
		return exitError.ExitCode()
	}
	return -1
}
