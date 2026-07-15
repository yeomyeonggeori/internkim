package localfleet

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
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

func (service Service) upPlans(skipWeb bool) []CommandPlan {
	return service.upPlansWithSkippedSetupSteps(skipWeb, nil)
}

func (service Service) upPlansWithSkippedSetupSteps(skipWeb bool, additionalSkippedSteps []string) []CommandPlan {
	return []CommandPlan{
		service.prepareContainerKernelPlan(),
		service.prepareLocalEmbeddingPlan(),
		service.labCommand("vm-up"),
		service.shellPlan("check shared workspace", service.checkSharedWorkspaceCommand()),
		service.shellPlan("start localhost tunnel", service.startTunnelCommand()),
		service.command("make", "build"),
		service.shellPlan("setup local fleet", service.setupCommand(skipWeb, additionalSkippedSteps...)),
		service.configureLocalEmbeddingPlan(),
		service.mattermostTestSettingsPlan(),
	}
}

func (service Service) prepareLocalEmbeddingPlan() CommandPlan {
	return service.command(filepath.Join(service.options.RepositoryRootPath, "tools", "prepare-local-fleet-embedding"))
}

func (service Service) configureLocalEmbeddingPlan() CommandPlan {
	scriptPath := "/mnt/shared/workspace/lab/scripts/configure-local-embedding.sh"
	return service.labCommand("vm-ssh", "sudo bash "+quoteShell(scriptPath))
}

func (service Service) mattermostTestSettingsPlan() CommandPlan {
	workspacePath := "/mnt/shared/workspace"
	scriptPath := workspacePath + "/lab/scripts/configure-mattermost-test-settings.sh"
	return service.labCommand("vm-ssh", "bash "+quoteShell(scriptPath)+" admin 127.0.0.1:8065")
}

func (service Service) withoutMattermostScenarioPlans(scenario string) []CommandPlan {
	return []CommandPlan{
		service.prepareContainerKernelPlan(),
		service.labCommand("vm-up"),
		service.blueclawDevSessionPreparePlan(scenario),
		service.shellPlan("check shared workspace", service.checkSharedWorkspaceCommand()),
		service.withoutMattermostVirtualSessionPlan(scenario),
	}
}

func (service Service) prepareContainerKernelPlan() CommandPlan {
	return service.command(filepath.Join(service.options.RepositoryRootPath, "tools", "prepare-container-kernel"))
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

func (service Service) ephemeralCleanupPlans() []CommandPlan {
	return []CommandPlan{
		service.shellPlan("stop localhost tunnel", service.stopTunnelCommand()),
		service.shellPlan("remove ephemeral VM", service.removeVirtualMachineCommand()),
	}
}

func (service Service) predeployGatePlans() []CommandPlan {
	return append(service.upPlans(false),
		service.shellPlan("verify api", service.verifyCommand("api")),
		service.shellPlan("verify mattermost", service.verifyCommand("mattermost")),
		service.blueclawLabScenarioScriptPlan("dm-recipient-resolve"),
		service.shellPlan("verify browser", service.verifyCommand("browser --local")),
	)
}

func (service Service) dmRecipientResolveScenarioPlans() []CommandPlan {
	return append(service.upPlans(false), service.blueclawLabScenarioScriptPlan("dm-recipient-resolve"))
}

func (service Service) mattermostScenarioPlans() []CommandPlan {
	return append(service.upPlans(false), service.labCommand("scenario-mattermost"))
}

func (service Service) mattermostDirectMessageScenarioPlans(keepArtifacts bool) []CommandPlan {
	verificationKind := "mattermost --direct-message-e2e"
	if keepArtifacts {
		verificationKind += " --keep"
	}
	return append(service.upPlans(false), service.shellPlan("verify direct message", service.verifyCommand(verificationKind)))
}

func (service Service) mattermostManualScenarioPlans() []CommandPlan {
	workspacePath := "/mnt/shared/workspace"
	scriptPath := workspacePath + "/lab/scripts/prepare-mattermost-manual-test.sh"
	return append(service.upPlans(false), service.labCommand("vm-ssh", "bash "+quoteShell(scriptPath)+" admin 127.0.0.1:8065"))
}

func (service Service) mattermostAskEphemeralScenarioPlans() []CommandPlan {
	scriptPath := filepath.Join(service.options.RepositoryRootPath, "lab", "scripts", "run-smoke-mattermost-ask-ephemeral-container.sh")
	skippedSteps := []string{
		"blueclaw-runtime-base",
		"skills",
		"blueclaw-config",
		"blueclaw-payload",
		"blueclaw-payload-direct",
		"openrouter",
		"staging",
		"services",
		"users-sync",
		"health",
	}
	return append(service.upPlansWithSkippedSetupSteps(true, skippedSteps), service.command(scriptPath, service.options.VirtualMachineName))
}

func (service Service) mattermostDocxAttachmentScenarioPlans(keepArtifacts bool) []CommandPlan {
	prompt := "간단한 테스트 보고서를 워드 파일(.docx)로 만들어서 첨부파일로 줘. 제목은 Local Fleet DOCX Attachment Test."
	downloadDirectory := filepath.Join(service.options.StateRootPath, "downloads", "mattermost-docx-attachment")
	verificationKind := strings.Join([]string{
		"mattermost",
		"--prompt " + quoteShell(prompt),
		"--expect-tool file.deliver",
		"--download-files-to " + quoteShell(downloadDirectory),
		"--wait-for-completion",
		"--timeout 480",
	}, " ")
	if keepArtifacts {
		verificationKind += " --keep"
	}
	return append(service.upPlans(false), service.shellPlan("verify docx attachment", service.verifyCommand(verificationKind)))
}

func (service Service) restartPolicySurvivalScenarioPlans() []CommandPlan {
	return append(service.upPlans(false), service.blueclawLabScenarioScriptPlan("restart-policy-survival"))
}

func (service Service) webBackedScenarioPlans(scenario string) []CommandPlan {
	return append(service.upPlans(false), service.shellPlan("run "+scenario, service.verifyCommand("browser --local")))
}

func (service Service) baseRegressionPlans(base string, scenario string) []CommandPlan {
	worktreePath := filepath.Join(service.options.StateRootPath, "worktrees", "base-"+safeName(base))
	return []CommandPlan{
		service.shellPlan("prepare base worktree", fmt.Sprintf("rm -rf %s && git worktree add --detach %s %s", quoteShell(worktreePath), quoteShell(worktreePath), quoteShell(base))),
		service.shellPlan("run base scenario", fmt.Sprintf("cd %s && %s dev fleet run --scenario %s", quoteShell(worktreePath), quoteShell(service.options.ExecutablePath), quoteShell(scenario))),
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

func (service Service) setupCommand(skipWeb bool, additionalSkippedSteps ...string) string {
	hostCommand := quoteShell(service.options.ExecutablePath) + " lab vm-ip --config " + quoteShell(service.configurationPath())
	skippedSteps := []string{"wifi", "local-llm", "cloudflare-access", "tunnel", "google", "slack"}
	if skipWeb {
		skippedSteps = append(skippedSteps, "web")
	}
	if !service.options.IsEphemeral {
		skippedSteps = append(skippedSteps, "blueclaw-runtime-base")
	}
	for _, skippedStep := range additionalSkippedSteps {
		if !slices.Contains(skippedSteps, skippedStep) {
			skippedSteps = append(skippedSteps, skippedStep)
		}
	}
	setupCommandParts := append(service.setupEnvironmentAssignments(), quoteShell(service.options.ExecutablePath))
	return strings.Join([]string{
		"host=$(" + hostCommand + ")",
		"test -n \"$host\"",
		strings.Join(setupCommandParts, " ") + " setup --board lab --ssh --host \"$host\" --user admin --password admin --admin-email local-fleet-admin@internkim.test --wait-lock --force --skip " + strings.Join(skippedSteps, ","),
	}, " && ")
}

func (service Service) setupEnvironmentAssignments() []string {
	assignments := []string{
		"INTERNKIM_BLUECLAW_USE_LOCAL=1",
		"INTERNKIM_SKIP_PAGES_DEPLOY_FOR_LAB=1",
	}
	if pinnedModelName := strings.TrimSpace(os.Getenv(blueclaw.BlueclawTestModelEnvironment)); !service.options.ShouldUseRealModels && pinnedModelName != "" {
		assignments = append(assignments, blueclaw.BlueclawTestModelEnvironment+"="+quoteShell(pinnedModelName))
	}
	if maximumModelTier := strings.TrimSpace(service.options.MaximumModelTier); maximumModelTier != "" {
		assignments = append(assignments, blueclaw.BlueclawTestMaximumModelTierEnvironment+"="+quoteShell(maximumModelTier))
	}
	if generationSeed := strings.TrimSpace(service.options.GenerationSeed); generationSeed != "" {
		assignments = append(assignments, "INTERNKIM_TEST_GENERATION_SEED="+quoteShell(generationSeed))
	}
	if generationTemperature := strings.TrimSpace(service.options.GenerationTemperature); generationTemperature != "" {
		assignments = append(assignments, "INTERNKIM_TEST_GENERATION_TEMPERATURE="+quoteShell(generationTemperature))
	}
	return assignments
}

func (service Service) startTunnelCommand() string {
	hostCommand := quoteShell(service.options.ExecutablePath) + " lab vm-ip --config " + quoteShell(service.configurationPath())
	sshpassPath := quoteShell(filepath.Join(service.options.RepositoryRootPath, "bin", "sshpass"))
	pidPath := quoteShell(service.tunnelPIDPath())
	logPath := quoteShell(service.tunnelLogPath())
	adminForward := fmt.Sprintf("127.0.0.1:%d:127.0.0.1:18080", service.options.AdminHostPort)
	mattermostForward := fmt.Sprintf("127.0.0.1:%d:127.0.0.1:8065", service.options.MattermostHostPort)
	return strings.Join([]string{
		"host=$(" + hostCommand + ")",
		"test -n \"$host\"",
		"if [ -s " + pidPath + " ] && kill -0 \"$(cat " + pidPath + ")\" 2>/dev/null; then exit 0; fi",
		"for attempt in 1 2 3 4 5 6 7 8 9 10; do rm -f " + pidPath + "; (nohup " + sshpassPath + " -p admin ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=password -o PubkeyAuthentication=no -o ExitOnForwardFailure=yes -N -L " + quoteShell(adminForward) + " -L " + quoteShell(mattermostForward) + " admin@\"$host\" > " + logPath + " 2>&1 < /dev/null & echo $! > " + pidPath + "); sleep 1; if [ -s " + pidPath + " ] && kill -0 \"$(cat " + pidPath + ")\" 2>/dev/null; then exit 0; fi; sleep 2; done; cat " + logPath + " 2>/dev/null || true; exit 1",
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

func (service Service) blueclawLabScenarioScriptPlan(scenario string) CommandPlan {
	workspacePath := "/mnt/shared/workspace"
	scriptPath := workspacePath + "/.dependency/blueclaw/lab/scripts/scenario-" + scenario + ".sh"
	return service.labCommand("vm-ssh", "bash "+quoteShell(scriptPath)+" admin 127.0.0.1:8065 "+workspacePath)
}

func (service Service) blueclawDevSessionPreparePlan(scenario string) CommandPlan {
	workspacePath := "/mnt/shared/workspace"
	scriptPath := workspacePath + "/lab/scripts/provision-blueclaw-dev-session.sh"
	return service.labCommand("vm-ssh", "bash "+quoteShell(scriptPath)+" admin /mnt/shared "+quoteShell(virtualSessionNeedsBunValue(scenario)))
}

func (service Service) withoutMattermostVirtualSessionPlan(scenario string) CommandPlan {
	command := strings.Join([]string{
		"cd " + quoteShell("/mnt/shared/workspace/.dependency/blueclaw"),
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

func (service Service) virtualSessionArtifactDirectoryPath(scenario string) string {
	runDirectoryName := firstNonEmpty(service.options.RunID, "default")
	return "/mnt/shared/workspace/.artifacts/local-fleet/" + safeIdentifier(runDirectoryName) + "/" + safeIdentifier(scenario)
}

func (service Service) logEphemeralContext(logger Logger, request JobRequest) {
	logger.Info("ephemeral run: " + service.options.RunID)
	logger.Info("state: " + service.options.StateRootPath)
	logger.Info("evidence: " + service.hostArtifactDirectoryPath())
	logger.Info("vm: " + service.options.VirtualMachineName)
	logger.Info("admin URL: " + service.adminHostURL())
	if !request.WithoutMattermost {
		logger.Info("Mattermost URL: " + service.mattermostHostURL())
	}
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

func safeName(value string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-")
	return replacer.Replace(strings.TrimSpace(value))
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
	case "slides", "slides_local_multiturn_success", "site", "site_artifact_acceptance", "site_prototype_acceptance", "site_edit_redeploy_acceptance", "site_lifecycle_acceptance":
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
