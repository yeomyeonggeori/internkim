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
	fmt.Println("  status    Show Mac container VM state and tenant runtime status")
	fmt.Println("  sync-cli  Cross-compile and install internkim CLI into the tenant VM")
	fmt.Println("  add-team  Provision a host-runtime tenant inside the tenant VM")
}

func executeHostCommand(commandName string, arguments []string, executor hostCommandExecutor) (int, error) {
	switch commandName {
	case "status":
		return 0, executeHostStatus(arguments, executor, os.Stdout)
	case "sync-cli":
		return 0, executeHostSyncCLI(arguments, executor, os.Stdout, os.Stderr)
	case "add-team":
		return executeHostAddTeam(arguments, executor, os.Stdout, os.Stderr)
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
	tenantIDs, errorValue := hostTenantIDs(executor, containerPath, options.VirtualMachineName)
	if errorValue != nil {
		return errorValue
	}
	fmt.Fprintln(output, strings.TrimSpace(string(listOutput)))
	fmt.Fprintln(output)
	fmt.Fprintln(output, "TENANT\tPUBLIC_URL\tRUNNING")
	for _, tenantID := range tenantIDs {
		status, errorValue := hostTenantStatus(executor, containerPath, options.VirtualMachineName, tenantID)
		if errorValue != nil {
			return errorValue
		}
		runningState := hostTenantRunningState(executor, containerPath, options.VirtualMachineName, tenantID)
		fmt.Fprintf(output, "%s\t%s\t%s\n", status.Manifest.TenantID, hostTenantPublicURL(status), runningState)
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

func executeHostAddTeam(arguments []string, executor hostCommandExecutor, output io.Writer, errorOutput io.Writer) (int, error) {
	options, errorValue := parseHostAddTeamOptions(arguments)
	if errorValue != nil {
		return 1, errorValue
	}
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
