package localfleet

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	scanner := bufio.NewScanner(pipe)
	buffer := make([]byte, 0, 1024*64)
	scanner.Buffer(buffer, 1024*1024)
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			logger.Info(line)
		}
	}
}

func (service Service) upPlans() []CommandPlan {
	return []CommandPlan{
		service.labCommand("vm-up"),
		service.shellPlan("start localhost tunnel", service.startTunnelCommand()),
		service.shellPlan("mount shared workspace", service.mountSharedWorkspaceCommand()),
		service.command("make", "build"),
		service.shellPlan("setup local fleet", service.setupCommand()),
	}
}

func (service Service) downPlans() []CommandPlan {
	return []CommandPlan{
		service.shellPlan("stop localhost tunnel", service.stopTunnelCommand()),
		service.labCommand("vm-down"),
	}
}

func (service Service) resetPlans() []CommandPlan {
	return []CommandPlan{
		service.shellPlan("reset local fleet", service.resetCommand()),
	}
}

func (service Service) predeployGatePlans() []CommandPlan {
	return append(service.upPlans(),
		service.shellPlan("verify api", service.verifyCommand("api")),
		service.shellPlan("verify mattermost", service.verifyCommand("mattermost")),
		service.shellPlan("verify browser", service.verifyCommand("browser --local")),
	)
}

func (service Service) mattermostScenarioPlans() []CommandPlan {
	return append(service.upPlans(), service.labCommand("scenario-mattermost"))
}

func (service Service) webBackedScenarioPlans(scenario string) []CommandPlan {
	return append(service.upPlans(), service.shellPlan("run "+scenario, service.verifyCommand("browser --local")))
}

func (service Service) baseRegressionPlans(base string, scenario string) []CommandPlan {
	worktreePath := filepath.Join(service.options.StateRootPath, "worktrees", "base-"+safeName(base))
	return []CommandPlan{
		service.shellPlan("prepare base worktree", fmt.Sprintf("rm -rf %s && git worktree add --detach %s %s", quoteShell(worktreePath), quoteShell(worktreePath), quoteShell(base))),
		service.shellPlan("run base scenario", fmt.Sprintf("cd %s && %s dev fleet run --scenario %s", quoteShell(worktreePath), quoteShell(service.options.ExecutablePath), quoteShell(scenario))),
	}
}

func (service Service) labCommand(arguments ...string) CommandPlan {
	commandArguments := append([]string{"lab"}, arguments...)
	commandArguments = append(commandArguments, "--config", service.configurationPath())
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

func (service Service) mountSharedWorkspaceCommand() string {
	return strings.Join([]string{
		quoteShell(service.options.ExecutablePath) + " lab vm-ssh --config " + quoteShell(service.configurationPath()) + " " + quoteShell("sudo mkdir -p /mnt/shared && (mountpoint -q /mnt/shared || sudo mount -t virtiofs com.apple.virtio-fs.automount /mnt/shared)"),
	}, " && ")
}

func (service Service) setupCommand() string {
	hostCommand := quoteShell(service.options.ExecutablePath) + " lab vm-ip --config " + quoteShell(service.configurationPath())
	return strings.Join([]string{
		"host=$(" + hostCommand + ")",
		"test -n \"$host\"",
		quoteShell(service.options.ExecutablePath) + " setup --board lab --ssh --host \"$host\" --user admin --password admin --skip wifi,local-llm,cloudflare-access,tunnel,google,slack",
	}, " && ")
}

func (service Service) startTunnelCommand() string {
	hostCommand := quoteShell(service.options.ExecutablePath) + " lab vm-ip --config " + quoteShell(service.configurationPath())
	sshpassPath := quoteShell(filepath.Join(service.options.RepositoryRootPath, "bin", "sshpass"))
	pidPath := quoteShell(service.tunnelPIDPath())
	logPath := quoteShell(service.tunnelLogPath())
	return strings.Join([]string{
		"host=$(" + hostCommand + ")",
		"test -n \"$host\"",
		"if [ -s " + pidPath + " ] && kill -0 \"$(cat " + pidPath + ")\" 2>/dev/null; then exit 0; fi",
		sshpassPath + " -p admin ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ExitOnForwardFailure=yes -N -L 127.0.0.1:18080:127.0.0.1:18080 -L 127.0.0.1:8065:127.0.0.1:8065 admin@\"$host\" > " + logPath + " 2>&1 & echo $! > " + pidPath,
		"sleep 1",
		"kill -0 \"$(cat " + pidPath + ")\"",
	}, " && ")
}

func (service Service) stopTunnelCommand() string {
	pidPath := quoteShell(service.tunnelPIDPath())
	return strings.Join([]string{
		"if [ -s " + pidPath + " ]; then kill \"$(cat " + pidPath + ")\" 2>/dev/null || true; fi",
		"rm -f " + pidPath,
	}, " && ")
}

func (service Service) verifyCommand(kind string) string {
	hostCommand := quoteShell(service.options.ExecutablePath) + " lab vm-ip --config " + quoteShell(service.configurationPath())
	return strings.Join([]string{
		"host=$(" + hostCommand + ")",
		"test -n \"$host\"",
		quoteShell(service.options.ExecutablePath) + " verify " + kind + " --board lab --host \"$host\" --user admin --password admin",
	}, " && ")
}

func (service Service) resetCommand() string {
	hostCommand := quoteShell(service.options.ExecutablePath) + " lab vm-ip --config " + quoteShell(service.configurationPath())
	return strings.Join([]string{
		"host=$(" + hostCommand + " 2>/dev/null || true)",
		"if [ -n \"$host\" ]; then " + quoteShell(service.options.ExecutablePath) + " reset blueclaw-history --board lab --host \"$host\" --user admin --password admin --confirm lab || true; fi",
		"rm -rf " + quoteShell(service.leasesPath()),
	}, " && ")
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

func safeName(value string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-")
	return replacer.Replace(strings.TrimSpace(value))
}
