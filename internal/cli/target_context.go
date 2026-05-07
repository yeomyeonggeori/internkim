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
	mode           commandTargetMode
	boardType      string
	baseStateDir   string
	stateDir       string
	nodeID         string
	isNodeExplicit bool
	fleetRole      string
	host           string
	sshUser        string
	sshPassword    string
	deviceURL      string
	sshHostname    string
	useRemoteSSH   bool
}

func resolveCommandTarget(arguments []string) commandTarget {
	boardType := commandArgumentValue(arguments, "--board", setup.BoardJetsonOrinNano)
	if hasCommandArgument(arguments, "--sim") {
		boardType = commandTargetBoardSimulation
	}
	baseStateDir := commandTargetStateDir(internkimHomeDir(), boardType)
	nodeID := commandTargetNodeID(arguments)
	stateDir := resolveCommandTargetStateDir(baseStateDir, nodeID, commandArgumentValue(arguments, "--host", "") != "")
	applyFleetTargetArguments(stateDir, baseStateDir, arguments)
	sshUser, sshPassword := resolveSetupSSHCredentials(
		boardType,
		commandArgumentValue(arguments, "--user", ""),
		commandArgumentValue(arguments, "--password", ""),
	)
	return commandTarget{
		mode:           commandTargetModeForBoardType(boardType),
		boardType:      boardType,
		baseStateDir:   baseStateDir,
		stateDir:       stateDir,
		nodeID:         loadState(stateDir, "board_id"),
		isNodeExplicit: nodeID != "",
		fleetRole:      loadState(stateDir, "fleet_role"),
		host:           commandArgumentValue(arguments, "--host", ""),
		sshUser:        sshUser,
		sshPassword:    sshPassword,
		deviceURL:      loadState(stateDir, "device_url"),
		sshHostname:    loadState(stateDir, "ssh_hostname"),
		useRemoteSSH:   hasCommandArgument(arguments, "--cloudflare-ssh"),
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
	if strings.TrimSpace(target.nodeID) != "" {
		fmt.Printf("Node: %s\n", target.nodeID)
	}
	if fleetID := strings.TrimSpace(loadState(target.stateDir, "device_id")); fleetID != "" {
		fmt.Printf("Fleet: %s\n", fleetID)
	}
	if strings.TrimSpace(target.fleetRole) != "" {
		fmt.Printf("Role: %s\n", target.fleetRole)
	}
	if strings.TrimSpace(target.host) != "" {
		fmt.Printf("Host: %s\n", target.host)
	}
	if target.useRemoteSSH {
		fmt.Printf("SSH: cloudflare\n")
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

func commandTargetStateDirForBoardIdentity(baseStateDir string, boardID string) string {
	normalizedBoardID := setupBoardIdentityName(boardID)
	if normalizedBoardID == "" {
		return baseStateDir
	}
	if existingStateDir := commandTargetStateDirForAssignedBoardID(baseStateDir, normalizedBoardID); existingStateDir != "" {
		return existingStateDir
	}
	stateDir := filepath.Join(baseStateDir, "boards", normalizedBoardID)
	_ = os.MkdirAll(stateDir, 0o700)
	copySetupStateHints(baseStateDir, stateDir)
	copyFleetStateHints(baseStateDir, stateDir)
	if loadState(stateDir, "board_id") == "" {
		saveState(stateDir, "board_id", normalizedBoardID)
	}
	return stateDir
}

func commandTargetStateDirForAssignedBoardID(baseStateDir string, boardID string) string {
	boardStateDirectories, errorValue := os.ReadDir(filepath.Join(baseStateDir, "boards"))
	if errorValue != nil {
		return ""
	}
	for _, boardStateDirectory := range boardStateDirectories {
		if !boardStateDirectory.IsDir() {
			continue
		}
		stateDir := filepath.Join(baseStateDir, "boards", boardStateDirectory.Name())
		if loadState(stateDir, "board_id") == boardID {
			return stateDir
		}
	}
	return ""
}

func resolveCommandTargetStateDir(baseStateDir string, requestedNodeID string, hasExplicitHost bool) string {
	if strings.TrimSpace(requestedNodeID) != "" {
		return commandTargetStateDirForBoardIdentity(baseStateDir, requestedNodeID)
	}
	if hasExplicitHost {
		return baseStateDir
	}
	if defaultNodeID := loadState(baseStateDir, "default_node_id"); defaultNodeID != "" {
		return commandTargetStateDirForBoardIdentity(baseStateDir, defaultNodeID)
	}
	if activeNodeID := firstActiveFleetNodeID(baseStateDir); activeNodeID != "" {
		return commandTargetStateDirForBoardIdentity(baseStateDir, activeNodeID)
	}
	return baseStateDir
}

func commandTargetNodeID(arguments []string) string {
	if nodeID := commandArgumentValue(arguments, "--node", ""); nodeID != "" {
		return nodeID
	}
	return commandArgumentValue(arguments, "--board-id", "")
}

func setupBoardIdentityName(boardID string) string {
	var builder strings.Builder
	for _, character := range strings.ToLower(strings.TrimSpace(boardID)) {
		switch {
		case character >= 'a' && character <= 'z':
			builder.WriteRune(character)
		case character >= '0' && character <= '9':
			builder.WriteRune(character)
		case character == '-':
			builder.WriteRune(character)
		default:
			builder.WriteRune('-')
		}
	}
	return strings.Trim(builder.String(), "-")
}

func firstActiveFleetNodeID(baseStateDir string) string {
	boardStateDirectories, errorValue := os.ReadDir(filepath.Join(baseStateDir, "boards"))
	if errorValue != nil {
		return ""
	}
	for _, boardStateDirectory := range boardStateDirectories {
		if !boardStateDirectory.IsDir() {
			continue
		}
		stateDir := filepath.Join(baseStateDir, "boards", boardStateDirectory.Name())
		if loadState(stateDir, "fleet_role") == "active" {
			return boardStateDirectory.Name()
		}
	}
	return ""
}

func activeFleetCommandTargets(baseTarget commandTarget) []commandTarget {
	return fleetCommandTargetsByRole(baseTarget, "active")
}

func allFleetCommandTargets(baseTarget commandTarget) []commandTarget {
	targets := fleetCommandTargetsByRole(baseTarget, "active")
	targets = append(targets, fleetCommandTargetsByRole(baseTarget, "pending")...)
	return targets
}

func fleetCommandTargetsByRole(baseTarget commandTarget, role string) []commandTarget {
	boardStateDirectories, errorValue := os.ReadDir(filepath.Join(baseTarget.baseStateDir, "boards"))
	if errorValue != nil {
		return nil
	}
	targets := []commandTarget{}
	for _, boardStateDirectory := range boardStateDirectories {
		if !boardStateDirectory.IsDir() {
			continue
		}
		stateDir := filepath.Join(baseTarget.baseStateDir, "boards", boardStateDirectory.Name())
		if loadState(stateDir, "fleet_role") != role {
			continue
		}
		target := baseTarget
		target.stateDir = stateDir
		target.nodeID = loadState(stateDir, "board_id")
		target.fleetRole = loadState(stateDir, "fleet_role")
		target.deviceURL = loadState(stateDir, "device_url")
		target.sshHostname = loadState(stateDir, "ssh_hostname")
		target.host = ""
		target.isNodeExplicit = true
		targets = append(targets, target)
	}
	return targets
}

func applyFleetTargetArguments(stateDir string, baseStateDir string, arguments []string) {
	if fleetID := strings.TrimSpace(commandArgumentValue(arguments, "--fleet", "")); fleetID != "" {
		saveState(stateDir, "device_id", strings.ToLower(fleetID))
	}
	fleetSecret := strings.TrimSpace(commandArgumentValue(arguments, "--fleet-secret", ""))
	if fleetSecret == "" {
		fleetSecret = strings.TrimSpace(os.Getenv("INTERNKIM_FLEET_SECRET"))
	}
	if fleetSecret != "" {
		saveState(stateDir, "device_secret", fleetSecret)
	}
	if loadState(stateDir, "device_id") == "" {
		if deviceID := loadState(baseStateDir, "device_id"); deviceID != "" {
			saveState(stateDir, "device_id", deviceID)
		}
	}
	if loadState(stateDir, "device_secret") == "" {
		if deviceSecret := loadState(baseStateDir, "device_secret"); deviceSecret != "" {
			saveState(stateDir, "device_secret", deviceSecret)
		}
	}
}

func copyFleetStateHints(sourceDir string, destinationDir string) {
	for _, key := range []string{"device_id", "device_secret", "device_url"} {
		if loadState(destinationDir, key) != "" {
			continue
		}
		if value := loadState(sourceDir, key); value != "" {
			saveState(destinationDir, key, value)
		}
	}
}
