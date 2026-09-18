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
	"strconv"
	"strings"

	"gitlab.com/eastriver/internkim/internal/blueclawworkspace"
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
	return service.upPlansThroughSetup(skipWeb, nil)
}

// The guest sees one thing: this worktree, mounted at /mnt/shared/workspace. So a
// shared artifact that sync-worktree-local-state linked rather than copied points
// at a host path the guest has no idea about, and the run dies ten minutes in with
// a missing file. Say it in a second instead.
func (service Service) checkSharedArtifactsCommand() string {
	linkedArtifacts := []string{
		".dependency/blueclaw-runtime",
		".dependency/container-kernel",
		".dependency/local-fleet-embedding",
		".dependency/buzz-relay",
	}
	checks := make([]string, 0, len(linkedArtifacts))
	for _, artifact := range linkedArtifacts {
		checks = append(checks, "if [ -L "+quoteShell(artifact)+" ]; then linked=\"$linked "+artifact+"\"; fi")
	}
	return strings.Join([]string{
		"cd " + quoteShell(service.options.RepositoryRootPath),
		"linked=''",
		strings.Join(checks, "; "),
		"if [ -n \"$linked\" ]; then " +
			"echo \"the guest mounts this worktree and nothing else, so it cannot follow a link out of it:$linked\" >&2; " +
			"echo 'run tools/sync-worktree-local-state --copy <main worktree path> and try again' >&2; " +
			"exit 1; fi",
	}, " && ")
}

func (service Service) checkMessengerArtifactCommand() string {
	blueclawPath := filepath.Join(service.options.RepositoryRootPath, ".dependency", "blueclaw")
	chatdRevisionPath := filepath.Join(service.options.RepositoryRootPath, ".dependency", "buzz-relay", "CHATD_REVISION")
	advice := "echo 'run make prepare-buzz-relay and try again' >&2; exit 1; fi"
	return strings.Join([]string{
		"pointerRevision=\"$(git -C " + quoteShell(blueclawPath) + " rev-parse HEAD 2>/dev/null || true)\"",
		"if [ -z \"$pointerRevision\" ]; then echo 'the blueclaw submodule has no revision to build chatd from' >&2; exit 1; fi",
		"builtRevision=\"$(cat " + quoteShell(chatdRevisionPath) + " 2>/dev/null || true)\"",
		"if [ -z \"$builtRevision\" ]; then " +
			"echo 'the chatd in .dependency/buzz-relay records no revision, so it was never built for this checkout' >&2; " +
			advice,
		"if [ \"$builtRevision\" != \"$pointerRevision\" ]; then " +
			"echo \"the chatd in .dependency/buzz-relay was built from $builtRevision and .dependency/blueclaw points at $pointerRevision\" >&2; " +
			advice,
	}, " && ")
}

func (service Service) preparedFleetPlans() []CommandPlan {
	return []CommandPlan{
		service.startCentralPlanePlan(),
		service.labCommand("vm-up"),
		service.shellPlan("check shared workspace", service.checkSharedWorkspaceCommand()),
		service.shellPlan("start localhost tunnel", service.startTunnelCommand()),
		service.shellPlan("give the device its company", service.joinCentralPlaneCommand()),
	}
}

func (service Service) upPlansThroughSetup(skipWeb bool, additionalSkippedSteps []string) []CommandPlan {
	plans := []CommandPlan{
		service.shellPlan("check the guest can see the shared artifacts", service.checkSharedArtifactsCommand()),
		service.shellPlan("check the messenger binary was built for this checkout", service.checkMessengerArtifactCommand()),
		service.prepareContainerKernelPlan(),
		service.prepareLocalEmbeddingPlan(),
		service.startCentralPlanePlan(),
		service.labCommand("vm-up"),
		service.shellPlan("check shared workspace", service.checkSharedWorkspaceCommand()),
		service.shellPlan("start localhost tunnel", service.startTunnelCommand()),
		service.command("make", "build"),
	}
	if !service.options.IsEphemeral && !slices.Contains(additionalSkippedSteps, "blueclaw-runtime-base") {
		plans = append(plans, service.shellPlan("ensure reusable runtime base", service.ensureRuntimeBaseCommand()))
	}
	plans = append(plans,
		service.shellPlan("give the device its company", service.joinCentralPlaneCommand()),
		service.shellPlan("setup local fleet", service.setupCommand(skipWeb, additionalSkippedSteps...)),
		service.configureLocalEmbeddingPlan(),
	)
	return plans
}

func (service Service) prepareLocalEmbeddingPlan() CommandPlan {
	return service.command(filepath.Join(service.options.RepositoryRootPath, "tools", "prepare-local-fleet-embedding"))
}

func (service Service) configureLocalEmbeddingPlan() CommandPlan {
	scriptPath := "/mnt/shared/workspace/lab/scripts/configure-local-embedding.sh"
	return service.labCommand("vm-ssh", "sudo bash "+quoteShell(scriptPath))
}

func (service Service) upgradePathGatePlans(scenario string) []CommandPlan {
	plans := service.upPlans(false)
	plans = append(plans,
		service.labCommand("vm-ssh", "sudo bash "+quoteShell("/mnt/shared/workspace/lab/scripts/regress-fleet-state.sh")),
		service.upgradeReleaseApplyPlan(),
		service.shellPlan("verify api after upgrade", service.verifyCommand("api")),
		service.blueclawDevSessionPreparePlan(scenario),
		service.virtualSessionPlan(scenario),
	)
	return plans
}

func (service Service) upgradeReleaseApplyPlan() CommandPlan {
	return service.command(service.options.ExecutablePath, "deploy",
		"--components", "admind,capabilityd,blueclawPayload",
		"--board", "lab",
		"--device-url", service.adminHostURL(),
	)
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

func (service Service) resetPlans() []CommandPlan {
	return []CommandPlan{
		service.shellPlan("reset local fleet", service.resetCommand()),
	}
}

func (service Service) ephemeralCleanupPlans() []CommandPlan {
	return []CommandPlan{
		service.shellPlan("stop localhost tunnel", service.stopTunnelCommand()),
		service.shellPlan("stop the company app", service.stopCentralPlaneCommand()),
		service.shellPlan("remove ephemeral VM", service.removeVirtualMachineCommand()),
	}
}

func (service Service) predeployGatePlans() []CommandPlan {
	return append(service.upPlans(false),
		service.shellPlan("verify api", service.verifyCommand("api")),
		service.blueclawLabScenarioScriptPlan("buzz-direct-message"),
		service.companyBrowserVerificationPlan(),
	)
}

// The fleet already provisions the relay and chatd, so this one only needed a
// scenario to use them.
func (service Service) buzzAttachmentScenarioPlans() []CommandPlan {
	return append(service.upPlansThroughSetup(true, nil), service.blueclawLabScenarioScriptPlan("buzz-attachment"))
}

func (service Service) buzzDirectMessageScenarioPlans() []CommandPlan {
	return append(service.upPlansThroughSetup(true, nil), service.blueclawLabScenarioScriptPlan("buzz-direct-message"))
}

func (service Service) buzzInboundMentionScenarioPlans() []CommandPlan {
	return append(service.upPlansThroughSetup(true, nil), service.blueclawLabScenarioScriptPlan("buzz-inbound-mention"))
}

func (service Service) restartPolicySurvivalScenarioPlans() []CommandPlan {
	return append(service.upPlans(false), service.blueclawLabScenarioScriptPlan("restart-policy-survival"))
}

func (service Service) modelConfigurationUpgradeScenarioPlans() []CommandPlan {
	command := "printf '%s\\n' admin | sudo -S python3 /mnt/shared/workspace/lab/scripts/verify-model-configuration-upgrade.py /mnt/shared/workspace/.artifacts/model-configuration-upgrade.json"
	return append(service.upPlansThroughSetup(true, nil), service.labCommand("vm-ssh", command))
}

func (service Service) workspacePersistenceScenarioPlans() []CommandPlan {
	return append(service.upPlans(false), service.blueclawLabScenarioScriptPlan("workspace-persistence"))
}

func (service Service) learningSettingsScenarioPlans() []CommandPlan {
	scriptArguments := []string{
		"bash", "/mnt/shared/workspace/lab/scripts/scenario-learning-settings.sh",
		"admin", "127.0.0.1:8065", "/mnt/shared/workspace",
		service.virtualSessionArtifactDirectoryPath("learning-settings"),
	}
	return append(service.upPlans(true), service.labCommand("vm-ssh", quoteShellArguments(scriptArguments)))
}

func (service Service) morningBriefingScenarioPlans() []CommandPlan {
	arguments := []string{"sudo", "-S", "python3", "/mnt/shared/workspace/lab/scripts/scenario-morning-briefing.py", service.virtualSessionArtifactDirectoryPath("morning-briefing")}
	plans := []CommandPlan{
		service.shellPlan("build briefing provider regression tests", "GOOS=linux GOARCH=arm64 go test -c -o build/briefing-provider.test ./internal/capabilityd"),
		service.shellPlan("build morning briefing database tests", "cd .dependency/blueclaw && GOOS=linux GOARCH=arm64 go test -c -o ../../build/morning-briefing-postgres.test ./internal/store/postgres"),
		service.shellPlan("build morning briefing model tests", "cd .dependency/blueclaw && GOOS=linux GOARCH=arm64 go test -c -tags 'appliance llmeval' -o ../../build/morning-briefing-live.test ./internal/e2e"),
	}
	plans = append(plans, service.upPlans(true)...)
	return append(plans, service.labCommand("vm-ssh", "printf '%s\\n' admin | "+quoteShellArguments(arguments)))
}

func (service Service) firingSchedulesNothingScenarioPlans() []CommandPlan {
	arguments := []string{"sudo", "-S", "python3", "/mnt/shared/workspace/lab/scripts/scenario-firing-schedules-nothing.py", service.virtualSessionArtifactDirectoryPath("firing-schedules-nothing")}
	return append(service.upPlans(true), service.labCommand("vm-ssh", "printf '%s\\n' admin | "+quoteShellArguments(arguments)))
}

func (service Service) scheduleThroughTheCatalogScenarioPlans() []CommandPlan {
	arguments := []string{"sudo", "-S", "python3", "/mnt/shared/workspace/lab/scripts/scenario-schedule-through-the-catalog.py", service.virtualSessionArtifactDirectoryPath("schedule-through-the-catalog")}
	return append(service.upPlans(true), service.labCommand("vm-ssh", "printf '%s\\n' admin | "+quoteShellArguments(arguments)))
}

func (service Service) memoryStoreScenarioPlans() []CommandPlan {
	arguments := []string{"sudo", "-S", "python3", "/mnt/shared/workspace/lab/scripts/scenario-memory-store.py", service.virtualSessionArtifactDirectoryPath("memory-store")}
	plans := []CommandPlan{
		service.shellPlan("build memory database regression tests", "cd .dependency/blueclaw/.dependency/bluememo && GOOS=linux GOARCH=arm64 go test -c -o ../../../../build/memory-postgres.test ./postgres"),
		service.shellPlan("build memory host integration tests", "cd .dependency/blueclaw && GOOS=linux GOARCH=arm64 go test -c -o ../../build/memory-integration.test ./tests/integration"),
		service.shellPlan("build memory model regression tests", "cd .dependency/blueclaw && GOOS=linux GOARCH=arm64 go test -c -tags 'appliance llmeval' -o ../../build/memory-live.test ./internal/e2e"),
	}
	plans = append(plans, service.upPlans(true)...)
	return append(plans, service.labCommand("vm-ssh", "printf '%s\\n' admin | "+quoteShellArguments(arguments)))
}

func (service Service) webBackedScenarioPlans() []CommandPlan {
	return append(service.upPlans(false),
		service.shellPlan("verify api", service.verifyCommand("api")),
		service.companyBrowserVerificationPlan(),
	)
}

func (service Service) personalSettingsScenarioPlans() []CommandPlan {
	return append(service.upPlans(true), service.companyBrowserVerificationPlan())
}

func (service Service) companyBrowserVerificationPlan() CommandPlan {
	return service.command(
		"bun", "run", filepath.Join(service.options.RepositoryRootPath, "tools", "verify-personal-settings.ts"),
		"--state-root", service.options.StateRootPath,
		"--app-port", strconv.Itoa(service.options.CompanyAppPort),
		"--admin-port", strconv.Itoa(service.options.AdminHostPort),
		"--chatd-url", blueclaw.ChatdEndpoint,
		"--config", service.configurationPath(),
	)
}

func (service Service) taskHistoryRetryScenarioPlans() []CommandPlan {
	arguments := []string{"sudo", "-S", "python3", "/mnt/shared/workspace/lab/scripts/scenario-task-history-retry.py", service.virtualSessionArtifactDirectoryPath("task-history-retry")}
	return append(service.upPlans(true),
		service.shellPlan("build task retry database acceptance", "cd .dependency/blueclaw && GOOS=linux GOARCH=arm64 go test -c -o ../../build/task-history-retry-postgres.test ./internal/store/postgres"),
		service.shellPlan("build task retry runtime acceptance", "cd .dependency/blueclaw && GOOS=linux GOARCH=arm64 go test -c -o ../../build/task-history-retry-runtime.test ./internal/connectors"),
		service.shellPlan("build task retry proxy acceptance", "GOOS=linux GOARCH=arm64 go test -c -o build/task-history-retry-admind.test ./internal/admind"),
		service.labCommand("vm-ssh", "printf '%s\\n' admin | "+quoteShellArguments(arguments)),
	)
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
	skippedSteps := []string{"wifi", "local-llm"}
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
	return service.setupSSHCommand("--force --skip " + strings.Join(skippedSteps, ","))
}

func (service Service) ensureRuntimeBaseCommand() string {
	return service.setupSSHCommand("--only blueclaw-runtime-base --skip web")
}

func (service Service) setupSSHCommand(selectionArguments string) string {
	hostCommand := quoteShell(service.options.ExecutablePath) + " lab vm-ip --config " + quoteShell(service.configurationPath())
	setupCommandParts := append(service.setupEnvironmentAssignments(), quoteShell(service.options.ExecutablePath))
	return strings.Join([]string{
		"host=$(" + hostCommand + ")",
		"test -n \"$host\"",
		strings.Join(setupCommandParts, " ") + " setup --board lab --ssh --host \"$host\" --user admin --password admin --admin-email local-fleet-admin@internkim.test --wait-lock " + selectionArguments,
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
		"for attempt in 1 2 3 4 5 6 7 8 9 10; do rm -f " + pidPath + "; (nohup " + sshpassPath + " -p admin ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o PreferredAuthentications=password -o PubkeyAuthentication=no -o ExitOnForwardFailure=yes -N -L " + quoteShell(adminForward) + " -R " + quoteShell(appReverseForward) + " -R " + quoteShell(recordReverseForward) + " admin@\"$host\" > " + logPath + " 2>&1 < /dev/null & echo $! > " + pidPath + "); sleep 1; if [ -s " + pidPath + " ] && kill -0 \"$(cat " + pidPath + ")\" 2>/dev/null && " + forwardHealthCheck + "; then exit 0; fi; sleep 2; done; cat " + logPath + " 2>/dev/null || true; exit 1",
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
	case "presentation", "presentation_local_multiturn_success", "site_artifact_acceptance", "site_edit_redeploy_acceptance", "site_custom_structure_acceptance", "site_lifecycle_acceptance":
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
