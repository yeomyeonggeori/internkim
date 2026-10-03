package localfleet

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/blueclawworkspace"
)

func (service Service) runPlans(contextValue context.Context, logger Logger, plans []CommandPlan) error {
	if errorValue := service.EnsureConfiguration(); errorValue != nil {
		return errorValue
	}
	for _, plan := range plans {
		if errorValue := service.runPlan(contextValue, logger, plan); errorValue != nil {
			service.writeLastResult("failed: " + errorValue.Error())
			return errorValue
		}
	}
	service.writeLastResult("succeeded")
	return nil
}

func (service Service) runCleanupPlans(contextValue context.Context, logger Logger, plans []CommandPlan) error {
	var cleanupErrors []string
	for _, plan := range plans {
		if errorValue := service.runPlan(contextValue, logger, plan); errorValue != nil {
			cleanupErrors = append(cleanupErrors, errorValue.Error())
		}
	}
	if len(cleanupErrors) > 0 {
		return errors.New(strings.Join(cleanupErrors, "; "))
	}
	return nil
}

func (service Service) runPlan(contextValue context.Context, logger Logger, plan CommandPlan) error {
	logger.Info("$ " + strings.Join(append([]string{plan.Name}, plan.Arguments...), " "))
	command := exec.CommandContext(contextValue, plan.Name, plan.Arguments...)
	command.Dir = plan.DirectoryPath
	command.Env = plan.Environment
	outputPipe, errorValue := command.StdoutPipe()
	if errorValue != nil {
		return errorValue
	}
	errorPipe, errorValue := command.StderrPipe()
	if errorValue != nil {
		return errorValue
	}
	if errorValue := command.Start(); errorValue != nil {
		return errorValue
	}
	done := make(chan struct{}, 2)
	go scanPlanOutput(outputPipe, logger, done)
	go scanPlanOutput(errorPipe, logger, done)
	<-done
	<-done
	return command.Wait()
}

func scanPlanOutput(pipe interface{ Read([]byte) (int, error) }, logger Logger, done chan struct{}) {
	defer func() { done <- struct{}{} }()
	reader := bufio.NewReaderSize(pipe, 1024*64)
	for {
		fragment, errorValue := reader.ReadSlice('\n')
		if text := strings.TrimSpace(string(fragment)); text != "" {
			logger.Info(text)
		}
		if errorValue == nil {
			continue
		}
		if errors.Is(errorValue, bufio.ErrBufferFull) {
			continue
		}
		return
	}
}

func (service Service) virtualSessionScenarioPlans(scenario string) []CommandPlan {
	return []CommandPlan{
		service.prepareContainerKernelPlan(),
		service.labCommand("vm-up"),
		service.blueclawDevSessionPreparePlan(scenario),
		service.shellPlan("check shared workspace", service.checkSharedWorkspaceCommand()),
		service.virtualSessionPlan(scenario),
	}
}

func (service Service) prepareContainerKernelPlan() CommandPlan {
	return service.command(filepath.Join(service.options.RepositoryRootPath, "tools", "prepare-container-kernel"))
}

func (service Service) workspaceOwnershipScenarioPlans() []CommandPlan {
	return []CommandPlan{
		service.command("env", "GOOS=linux", "GOARCH=arm64", "CGO_ENABLED=0", "go", "-C", ".dependency/blueclaw", "build",
			"-o", "../../.artifacts/workspace-ownership/blueclaw-posix-helper", "./cmd/blueclaw-posix-helper"),
		service.prepareContainerKernelPlan(),
		service.labCommand("vm-up"),
		service.shellPlan("check shared workspace", service.checkSharedWorkspaceCommand()),
		service.labCommand("vm-ssh", "sudo bash /mnt/shared/workspace/lab/scripts/scenario-workspace-ownership.sh /mnt/shared/workspace"),
	}
}

func (service Service) downPlans() []CommandPlan {
	return []CommandPlan{
		service.shellPlan("stop localhost tunnel", service.stopTunnelCommand()),
		service.shellPlan("stop the company app", service.stopCentralPlaneCommand()),
		service.labCommand("vm-down"),
	}
}

func (service Service) ephemeralCleanupPlans() []CommandPlan {
	return []CommandPlan{
		service.shellPlan("stop localhost tunnel", service.stopTunnelCommand()),
		service.shellPlan("stop the company app", service.stopCentralPlaneCommand()),
		service.shellPlan("remove ephemeral VM", service.removeVirtualMachineCommand()),
	}
}

func (service Service) labCommand(arguments ...string) CommandPlan {
	commandArguments := []string{"lab"}
	if len(arguments) > 0 {
		commandArguments = append(commandArguments, arguments[0], "--config", service.configurationPath())
		commandArguments = append(commandArguments, arguments[1:]...)
	} else {
		commandArguments = append(commandArguments, "--config", service.configurationPath())
	}
	return CommandPlan{
		DirectoryPath: service.options.RepositoryRootPath,
		Name:          service.options.ExecutablePath,
		Arguments:     commandArguments,
		Environment:   os.Environ(),
	}
}

func (service Service) command(name string, arguments ...string) CommandPlan {
	return CommandPlan{
		DirectoryPath: service.options.RepositoryRootPath,
		Name:          name,
		Arguments:     arguments,
		Environment:   os.Environ(),
	}
}

func (service Service) shellPlan(label string, command string) CommandPlan {
	return CommandPlan{
		DirectoryPath: service.options.RepositoryRootPath,
		Name:          "/bin/sh",
		Arguments:     []string{"-c", command},
		Environment:   os.Environ(),
	}
}

func (service Service) checkSharedWorkspaceCommand() string {
	command := quoteShell(service.options.ExecutablePath) + " lab vm-ssh --config " + quoteShell(service.configurationPath()) + " " + quoteShell("test -d /mnt/shared/workspace")
	return "for attempt in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do " + command + " && exit 0; sleep 2; done; " + command
}

func (service Service) startTunnelCommand() string {
	hostCommand := quoteShell(service.options.ExecutablePath) + " lab vm-ip --config " + quoteShell(service.configurationPath())
	pidPath := quoteShell(service.tunnelPIDPath())
	logPath := quoteShell(service.tunnelLogPath())
	adminForward := fmt.Sprintf("127.0.0.1:%d:127.0.0.1:18080", service.options.AdminHostPort)
	forwardHealthCheck := fmt.Sprintf("nc -z 127.0.0.1 %d", service.options.AdminHostPort)
	// The device belongs to a company, and the company lives on this machine, so
	// the same session carries the app and the record back the other way.
	appReverseForward := fmt.Sprintf("127.0.0.1:%d:127.0.0.1:%d", service.options.CompanyAppPort, service.options.CompanyAppPort)
	recordReverseForward := fmt.Sprintf("127.0.0.1:%d:127.0.0.1:%d", localRecordPort, localRecordPort)
	return strings.Join([]string{
		"host=$(" + hostCommand + ")",
		"test -n \"$host\"",
		"if [ -s " + pidPath + " ] && kill -0 \"$(cat " + pidPath + ")\" 2>/dev/null && " + forwardHealthCheck + "; then exit 0; fi",
		"if [ -s " + pidPath + " ]; then kill \"$(cat " + pidPath + ")\" 2>/dev/null || true; fi",
		"for attempt in 1 2 3 4 5 6 7 8 9 10; do rm -f " + pidPath + "; (nohup sshpass -p admin ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=password -o PubkeyAuthentication=no -o ExitOnForwardFailure=yes -N -L " + quoteShell(adminForward) + " -R " + quoteShell(appReverseForward) + " -R " + quoteShell(recordReverseForward) + " admin@\"$host\" > " + logPath + " 2>&1 < /dev/null & echo $! > " + pidPath + "); sleep 1; if [ -s " + pidPath + " ] && kill -0 \"$(cat " + pidPath + ")\" 2>/dev/null && " + forwardHealthCheck + "; then exit 0; fi; sleep 2; done; cat " + logPath + " 2>/dev/null || true; exit 1",
	}, " && ")
}

func (service Service) stopTunnelCommand() string {
	pidPath := quoteShell(service.tunnelPIDPath())
	return strings.Join([]string{
		"if [ -s " + pidPath + " ]; then kill \"$(cat " + pidPath + ")\" 2>/dev/null || true; fi",
		"rm -f " + pidPath,
	}, " && ")
}

func (service Service) reapOrphanedEphemeralContainersCommand() string {
	containerBinary := quoteShell("container")
	currentName := quoteShell(service.options.VirtualMachineName)
	listOrphans := containerBinary + ` ls -a 2>/dev/null | awk '$1 ~ /^internkim-e2e-/ && $5 == "stopped" { print $1 }'`
	return strings.Join([]string{
		"for orphan in $(" + listOrphans + ")",
		`do if [ "$orphan" != ` + currentName + ` ]; then ` + containerBinary + ` rm "$orphan" >/dev/null 2>&1 || true; fi`,
		"done",
	}, "; ")
}

func (service Service) removeVirtualMachineCommand() string {
	containerBinary := quoteShell("container")
	containerName := quoteShell(service.options.VirtualMachineName)
	return strings.Join([]string{
		containerBinary + " stop " + containerName + " >/dev/null 2>&1 || true",
		containerBinary + " rm " + containerName + " >/dev/null 2>&1 || true",
	}, "; ")
}

func (service Service) removeStateCommand() string {
	return "rm -rf " + quoteShell(service.options.StateRootPath)
}

func (service Service) blueclawDevSessionPreparePlan(scenario string) CommandPlan {
	workspacePath := "/mnt/shared/workspace"
	scriptPath := workspacePath + "/lab/scripts/provision-blueclaw-dev-session.sh"
	return service.labCommand("vm-ssh", "bash "+quoteShell(scriptPath)+" admin /mnt/shared "+quoteShell(virtualSessionNeedsBunValue(scenario)))
}

func (service Service) virtualSessionPlan(scenario string) CommandPlan {
	scenarioEnvironment, errorValue := service.virtualSessionEnvironment()
	if errorValue != nil {
		return service.labCommand("vm-ssh", "echo "+quoteShell(errorValue.Error())+" >&2; exit 1")
	}
	command := strings.Join([]string{
		"cd " + quoteShell("/mnt/shared/workspace/.dependency/blueclaw"),
		strings.Join(scenarioEnvironment, "; "),
		quoteShellArguments([]string{
			"go",
			"run",
			"./cmd/blueclaw-lab",
			"virtual-session",
			"--scenario",
			scenario,
			"--artifact-dir",
			service.virtualSessionArtifactDirectoryPath(scenario),
		}),
	}, " && ")
	return service.labCommand("vm-ssh", command)
}

func (service Service) virtualSessionEnvironment() ([]string, error) {
	skillRootPaths, errorValue := blueclawworkspace.SkillRootPaths(service.options.RepositoryRootPath)
	if errorValue != nil {
		return nil, errorValue
	}
	guestSkillRootPaths := make([]string, 0, len(skillRootPaths))
	for _, skillRootPath := range skillRootPaths {
		guestSkillRootPaths = append(guestSkillRootPaths, guestWorkspacePath(service.options.RepositoryRootPath, skillRootPath))
	}
	catalogPath := filepath.Join(service.options.RepositoryRootPath, "pkg", "capabilityprotocol", "generated", "capability-tools.json")
	return []string{
		"export BLUECLAW_SCENARIO_SKILL_ROOTS=" + quoteShell(strings.Join(guestSkillRootPaths, string(os.PathListSeparator))),
		"export BLUECLAW_SCENARIO_CAPABILITY_CATALOG=" + quoteShell(guestWorkspacePath(service.options.RepositoryRootPath, catalogPath)),
	}, nil
}

func guestWorkspacePath(repositoryRootPath string, hostPath string) string {
	relativePath, errorValue := filepath.Rel(repositoryRootPath, hostPath)
	if errorValue != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return hostPath
	}
	return filepath.Join("/mnt/shared/workspace", relativePath)
}

func (service Service) virtualSessionArtifactDirectoryPath(scenario string) string {
	runDirectoryName := firstNonEmpty(service.options.RunID, "default")
	return "/mnt/shared/workspace/.artifacts/local-fleet/" + safeIdentifier(runDirectoryName) + "/" + safeIdentifier(scenario)
}

func (service Service) logEphemeralContext(logger Logger) {
	logger.Info("ephemeral run: " + service.options.RunID)
	logger.Info("state: " + service.options.StateRootPath)
	logger.Info("evidence: " + service.hostArtifactDirectoryPath())
	logger.Info("vm: " + service.options.VirtualMachineName)
	logger.Info("admin URL: " + service.adminHostURL())
	logger.Info("cleanup vm: " + service.removeVirtualMachineCommand())
	logger.Info("cleanup all: " + service.manualCleanupCommand())
}

func (service Service) manualCleanupCommand() string {
	return service.removeVirtualMachineCommand() + "; " + service.removeStateCommand()
}

func (service Service) hostArtifactDirectoryPath() string {
	runDirectoryName := firstNonEmpty(service.options.RunID, "default")
	return filepath.Join(service.options.RepositoryRootPath, ".artifacts", "local-fleet", safeIdentifier(runDirectoryName))
}

func (service Service) configurationPath() string {
	return filepath.Join(service.options.StateRootPath, "config.json")
}

func (service Service) leasesPath() string {
	return filepath.Join(service.options.StateRootPath, "leases")
}

func (service Service) lastResultPath() string {
	return filepath.Join(service.options.StateRootPath, "last-result")
}

func (service Service) tunnelPIDPath() string {
	return filepath.Join(service.options.StateRootPath, "tunnel.pid")
}

func (service Service) tunnelLogPath() string {
	return filepath.Join(service.options.StateRootPath, "tunnel.log")
}

func (service Service) writeLastResult(value string) {
	_ = os.MkdirAll(service.options.StateRootPath, 0o700)
	_ = os.WriteFile(service.lastResultPath(), []byte(value+"\n"), 0o600)
}

func quoteShell(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func safeIdentifier(value string) string {
	normalizedValue := strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	previousWasDash := false
	for _, character := range normalizedValue {
		isLetter := character >= 'a' && character <= 'z'
		isDigit := character >= '0' && character <= '9'
		if isLetter || isDigit {
			builder.WriteRune(character)
			previousWasDash = false
			continue
		}
		if !previousWasDash {
			builder.WriteByte('-')
			previousWasDash = true
		}
	}
	identifier := strings.Trim(builder.String(), "-")
	if identifier == "" {
		return "run"
	}
	return identifier
}

func virtualSessionNeedsBunValue(scenario string) string {
	switch strings.ToLower(strings.TrimSpace(scenario)) {
	case "presentation", "presentation_local_multiturn_success":
		return "1"
	default:
		return "0"
	}
}

func quoteShellArguments(arguments []string) string {
	quotedArguments := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		quotedArguments = append(quotedArguments, quoteShell(argument))
	}
	return strings.Join(quotedArguments, " ")
}
