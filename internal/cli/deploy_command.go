package cli

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/blueclawworkspace"
	setup "gitlab.com/eastriver/internkim/internal/provisioning/steps"
)

func runDeploy() {
	arguments := os.Args[2:]
	if hasCommandArgument(arguments, "--help") || hasCommandArgument(arguments, "-h") {
		fmt.Println(deployUsageText())
		os.Exit(0)
	}
	if errorValue := validateDeployArguments(arguments); errorValue != nil {
		fmt.Fprintf(os.Stderr, "deploy: %s\n\n%s\n", errorValue.Error(), deployUsageText())
		os.Exit(1)
	}
	if hasCommandArgument(arguments, "--legacy-ssh") {
		runDeployLegacySSH()
		return
	}
	if errorValue := runRegistryReleaseDeploy(arguments); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runDeployLegacySSH() {
	configuration := loadConfig()
	scriptDir, _ := os.Getwd()
	binDir := filepath.Join(scriptDir, "bin")
	sshpassBin := filepath.Join(binDir, "sshpass")
	boardBinDir := filepath.Join(scriptDir, "build", "board-bin")
	arguments := commandControlArguments(os.Args[2:])
	setupStepNames, hasSelectedSetupSteps, errorValue := legacySSHDeploySetupStepNames(arguments)
	if errorValue != nil {
		fatal(errorValue.Error())
	}

	targets := []commandTarget{resolveCommandTarget(arguments)}
	if hasCommandArgument(arguments, "--all-active") {
		targets = activeFleetCommandTargets(targets[0])
		if len(targets) == 0 {
			fatal("No active fleet nodes are known locally.")
		}
	}

	for _, target := range targets {
		if strings.TrimSpace(target.fleetRole) == "pending" {
			fatal("Refusing to deploy to pending node " + target.nodeID)
		}
		ssh, isRemote, errorValue := resolveDeviceSSHConnection(configuration, sshpassBin, target)
		if errorValue != nil {
			fatal(errorValue.Error())
		}
		target.host = ssh.host
		target.useRemoteSSH = isRemote
		printCommandTargetEvidence(target)
		if hasSelectedSetupSteps {
			runLegacySSHSetupDeploy(configuration, scriptDir, target, ssh, setupStepNames)
		} else {
			runDeployToBoard(scriptDir, boardBinDir, ssh)
		}
	}
	fmt.Println("Deploy complete.")
}

func legacySSHDeploySetupStepNames(arguments []string) ([]string, bool, error) {
	selectedComponentNames, errorValue := selectedReleaseComponentNames(arguments)
	if errorValue != nil {
		return nil, false, errorValue
	}
	if len(selectedComponentNames) == 0 {
		return nil, false, nil
	}

	componentMappings := []struct {
		componentName string
		stepName      string
	}{
		{componentName: "web", stepName: "web"},
		{componentName: "admind", stepName: "admind"},
		{componentName: "capabilityd", stepName: "capabilityd"},
		{componentName: "skills", stepName: "skills"},
		{componentName: "blueclawPayload", stepName: "blueclaw-payload-direct"},
	}
	mappedComponentNames := map[string]bool{}
	stepNames := []string{}
	for _, mapping := range componentMappings {
		if !selectedComponentNames[mapping.componentName] {
			continue
		}
		mappedComponentNames[mapping.componentName] = true
		stepNames = append(stepNames, mapping.stepName)
	}

	unsupportedComponentNames := []string{}
	for componentName := range selectedComponentNames {
		if !mappedComponentNames[componentName] {
			unsupportedComponentNames = append(unsupportedComponentNames, componentName)
		}
	}
	if len(unsupportedComponentNames) > 0 {
		sort.Strings(unsupportedComponentNames)
		return nil, true, fmt.Errorf("legacy SSH deploy does not support component(s): %s", strings.Join(unsupportedComponentNames, ", "))
	}
	return stepNames, true, nil
}

func runLegacySSHSetupDeploy(configuration config, scriptDir string, target commandTarget, ssh *sshClient, stepNames []string) {
	messenger := newMsg("ko")
	flowState := newSetupFlowState(
		messenger,
		configuration,
		collectSetupParameterValues(),
		target.stateDir,
		scriptDir,
		currentExecutableFingerprint(),
		ssh,
		false,
	)
	pipelineContext := &setup.Context{
		Backend:      setup.BackendSSH,
		Language:     messenger.lang,
		StateDir:     target.stateDir,
		ScriptDir:    scriptDir,
		BoardType:    target.boardType,
		BoardIP:      ssh.host,
		PublicURL:    target.deviceURL,
		RelayDomain:  relayDomainForTarget(target.stateDir),
		SetupCommand: commandLineForSetupLock(os.Args[2:]),
		SetupSteps:   strings.Join(stepNames, ","),
		SetupLockID:  randomHexString(12),
		Force:        true,
		HTTP:         &http.Client{Timeout: 30 * time.Second},
		Callbacks:    flowState.callbacks(),
		SSH:          sshBoardConnection{client: ssh},
	}
	selector := setup.Selector{Only: stepNames, Force: true}
	if errorValue := setupRegistryForBoard(target.boardType).Run(pipelineContext, selector); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runDeployToBoard(scriptDir string, boardBinDir string, ssh *sshClient) {
	boardTools := []string{"download"}

	fmt.Print("Checking skill dependencies... ")
	ssh.run(installSkillPythonDependenciesCommand())
	fmt.Println("ok")

	fmt.Print("Deploying skills... ")
	skillDirectories, skillsError := blueclawworkspace.SkillDirectories(scriptDir)
	if skillsError != nil {
		fatal(skillsError.Error())
	}
	if len(skillDirectories) > 0 {
		ssh.run("mkdir -p /root/.blueclaw/workspace/skills")
		for _, skillDirectory := range skillDirectories {
			remoteSkillDir := "/root/.blueclaw/workspace/skills/" + skillDirectory.Name
			ssh.run("rm -rf " + remoteSkillDir + " && mkdir -p " + remoteSkillDir)
			ssh.scpDir(skillDirectory.Path, remoteSkillDir)
		}
		ssh.run("chown -R blueclaw:blueclaw /root/.blueclaw/workspace/skills")
	}
	ssh.run(`chown -R blueclaw:blueclaw /root/.blueclaw/workspace/skills 2>/dev/null || true`)
	fmt.Println("ok")

	fmt.Print("Deploying workspace tools... ")
	toolsDirectoryPath := blueclawworkspace.ToolsPath(scriptDir)
	if _, err := os.Stat(toolsDirectoryPath); err == nil {
		remoteToolsDirectoryPath := "/root/.blueclaw/workspace/tools"
		ssh.run("rm -rf " + remoteToolsDirectoryPath + " && mkdir -p " + remoteToolsDirectoryPath)
		ssh.scpDir(toolsDirectoryPath, remoteToolsDirectoryPath)
		ssh.run("chown -R root:root " + remoteToolsDirectoryPath + " && chmod -R a+rX,go-w " + remoteToolsDirectoryPath)
	}
	fmt.Println("ok")

	for _, tool := range boardTools {
		fmt.Printf("Building %s... ", tool)
		cmd := exec.Command("go", "build", "-o", filepath.Join(boardBinDir, tool), "./cmd/"+tool+"/")
		cmd.Dir = scriptDir
		cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=arm64")
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Println("FAILED")
			fmt.Println(string(out))
			continue
		}
		fmt.Print("ok, deploying... ")
		ssh.scp(filepath.Join(boardBinDir, tool), "/usr/local/bin/"+tool)
		ssh.run("chmod +x /usr/local/bin/" + tool)
		ssh.run("mkdir -p /root/.blueclaw/workspace/bin /root/.blueclaw/workspace/downloads && cp /usr/local/bin/" + tool + " /root/.blueclaw/workspace/bin/ && chmod 755 /root/.blueclaw/workspace/bin /root/.blueclaw/workspace/bin/" + tool)
		fmt.Println("ok")
	}
}

func toolBinEntries(boardBinDir string, names []string) []struct{ local, remote, name string } {
	var entries []struct{ local, remote, name string }
	for _, name := range names {
		entries = append(entries, struct{ local, remote, name string }{
			filepath.Join(boardBinDir, name),
			"/usr/local/bin/" + name,
			name,
		})
	}
	return entries
}
