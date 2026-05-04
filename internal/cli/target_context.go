package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	internkimlab "gitlab.com/eastriver/internkim/internal/lab"
	setup "gitlab.com/eastriver/internkim/internal/provisioning/steps"
)

type commandTargetMode string

const (
	commandTargetModePhysical   commandTargetMode = "physical"
	commandTargetModeLab        commandTargetMode = "lab"
	commandTargetModeSimulation commandTargetMode = "simulation"

	commandTargetBoardLab        = "lab"
	commandTargetBoardSimulation = "sim"
)

type commandTarget struct {
	mode        commandTargetMode
	boardType   string
	stateDir    string
	host        string
	sshUser     string
	sshPassword string
	deviceURL   string
}

func resolveCommandTarget(arguments []string) commandTarget {
	boardType := commandArgumentValue(arguments, "--board", setup.BoardJetsonOrinNano)
	if hasCommandArgument(arguments, "--sim") {
		boardType = commandTargetBoardSimulation
	}
	stateDir := commandTargetStateDir(internkimHomeDir(), boardType)
	sshUser, sshPassword := resolveSetupSSHCredentials(
		boardType,
		commandArgumentValue(arguments, "--user", ""),
		commandArgumentValue(arguments, "--password", ""),
	)
	return commandTarget{
		mode:        commandTargetModeForBoardType(boardType),
		boardType:   boardType,
		stateDir:    stateDir,
		host:        commandArgumentValue(arguments, "--host", ""),
		sshUser:     sshUser,
		sshPassword: sshPassword,
		deviceURL:   loadState(stateDir, "device_url"),
	}
}

func commandTargetModeForBoardType(boardType string) commandTargetMode {
	switch strings.TrimSpace(boardType) {
	case commandTargetBoardLab:
		return commandTargetModeLab
	case commandTargetBoardSimulation:
		return commandTargetModeSimulation
	default:
		return commandTargetModePhysical
	}
}

func resolveLabHostForCommandTarget(target commandTarget, repositoryRootPath string) commandTarget {
	if target.mode != commandTargetModeLab || strings.TrimSpace(target.host) != "" {
		return target
	}
	labHost := resolveLabVirtualMachineIPAddress()
	if labHost == "" {
		return target
	}
	configurationPath := internkimlab.DefaultConfigurationPath(repositoryRootPath)
	configuration, errorValue := internkimlab.LoadConfiguration(configurationPath)
	if errorValue != nil {
		target.host = labHost
		return target
	}
	target.host = labHost
	target.sshUser = configuration.VirtualMachine.SSHUsername
	target.sshPassword = configuration.VirtualMachine.SSHPassword
	return target
}

func printCommandTargetEvidence(target commandTarget) {
	fmt.Printf("Target: %s (%s)\n", target.boardType, target.mode)
	fmt.Printf("State: %s\n", target.stateDir)
	if strings.TrimSpace(target.host) != "" {
		fmt.Printf("Host: %s\n", target.host)
	}
	if strings.TrimSpace(target.deviceURL) != "" {
		fmt.Printf("URL: %s\n", target.deviceURL)
	}
}

func commandArgumentValue(arguments []string, name string, defaultValue string) string {
	for index, argument := range arguments {
		if argument == name && index+1 < len(arguments) {
			return strings.TrimSpace(arguments[index+1])
		}
		if strings.HasPrefix(argument, name+"=") {
			return strings.TrimSpace(strings.TrimPrefix(argument, name+"="))
		}
	}
	return defaultValue
}

func hasCommandArgument(arguments []string, name string) bool {
	for _, argument := range arguments {
		if argument == name || strings.HasPrefix(argument, name+"=") {
			return true
		}
	}
	return false
}

func commandTargetStateDir(baseStateDir string, boardType string) string {
	if commandTargetModeForBoardType(boardType) == commandTargetModeSimulation {
		stateDir := filepath.Join(baseStateDir, "simulations", setupStateName(boardType))
		_ = os.MkdirAll(stateDir, 0o700)
		return stateDir
	}
	return setupStateDir(baseStateDir, boardType)
}
