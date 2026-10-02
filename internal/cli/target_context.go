package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/deployops"
	internkimlab "github.com/yeomyeonggeori/internkim/internal/lab"
	setup "github.com/yeomyeonggeori/internkim/internal/provisioning/steps"
)

type commandTargetMode string

const (
	commandTargetModePhysical   commandTargetMode = "physical"
	commandTargetModeLab        commandTargetMode = "lab"
	commandTargetModeSimulation commandTargetMode = "simulation"

	commandTargetBoardLab        = "lab"
	commandTargetBoardSimulation = setup.BoardSimulation
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
	fleetID        string
	fleetSecret    string
	useRemoteSSH   bool
}

type targetFlags struct {
	host          *string
	user          *string
	password      *string
	node          *string
	cloudflareSSH *bool
	board         *string
	simulation    *bool
}

func registerTargetFlags(flagSet *flag.FlagSet) targetFlags {
	return targetFlags{
		host:          flagSet.String("host", "", "Board host"),
		user:          flagSet.String("user", "", "SSH user"),
		password:      flagSet.String("password", "", "SSH password"),
		node:          flagSet.String("node", "", "Fleet node target"),
		cloudflareSSH: flagSet.Bool("remote-ssh", false, "Reach the device by its ssh hostname instead of probing the local network"),
		board:         flagSet.String("board", "", "Board target"),
		simulation:    flagSet.Bool("sim", false, "Use simulation target"),
	}
}

func (flags targetFlags) arguments() []string {
	return verifyTargetArguments(*flags.host, *flags.user, *flags.password, *flags.node, *flags.cloudflareSSH, *flags.board, *flags.simulation)
}

func (flags targetFlags) resolveVerifyTarget() (verifyTarget, error) {
	return resolveVerifyTarget(flags.arguments())
}

func resolveCommandTarget(arguments []string) commandTarget {
	boardType := commandArgumentValue(arguments, "--board", setup.BoardJetsonOrinNano)
	if hasCommandArgument(arguments, "--sim") {
		boardType = commandTargetBoardSimulation
	}
	mode := commandTargetModeForBoardType(boardType)
	baseStateDir := commandTargetStateDir(internkimHomeDir(), boardType)
	nodeID := commandTargetNodeID(arguments)
	stateDir := resolveCommandTargetStateDir(baseStateDir, nodeID, commandArgumentValue(arguments, "--host", "") != "")
	applyFleetTargetArguments(stateDir, baseStateDir, arguments)
	device := deviceTargetFor(mode, stateDir)
	sshUser, sshPassword := resolveSetupSSHCredentials(
		boardType,
		commandArgumentValue(arguments, "--user", ""),
		commandArgumentValue(arguments, "--password", ""),
	)
	return commandTarget{
		mode:           mode,
		boardType:      boardType,
		baseStateDir:   baseStateDir,
		stateDir:       stateDir,
		nodeID:         loadNodeID(stateDir),
		isNodeExplicit: nodeID != "",
		fleetRole:      loadState(stateDir, "fleet_role"),
		host:           commandArgumentValue(arguments, "--host", ""),
		sshUser:        sshUser,
		sshPassword:    sshPassword,
		deviceURL:      firstNonEmptyString(commandArgumentValue(arguments, "--device-url", ""), device.AdminURL),
		sshHostname:    device.SSHHostname,
		fleetID:        device.FleetID,
		fleetSecret:    device.FleetSecret,
		useRemoteSSH:   hasCommandArgument(arguments, "--remote-ssh"),
	}
}

func deviceTargetFor(mode commandTargetMode, stateDir string) deployops.Target {
	if mode == commandTargetModePhysical {
		device, _ := deployops.DeviceTargetFromEnvironment()
		return device
	}
	return deployops.Target{
		AdminURL:    loadState(stateDir, "device_url"),
		SSHHostname: loadState(stateDir, "ssh_hostname"),
		FleetID:     loadState(stateDir, "fleet_id"),
		FleetSecret: loadState(stateDir, "fleet_secret"),
	}
}

func (target commandTarget) fleetIdentity() (string, string, error) {
	if target.fleetID == "" || target.fleetSecret == "" {
		return "", "", fmt.Errorf("the vault names no %s and %s for this device; run it as `internkim @legacy …`",
			deployops.FleetIDVariable, deployops.FleetSecretVariable)
	}
	return target.fleetID, target.fleetSecret, nil
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
	if target.fleetID != "" {
		fmt.Printf("Fleet: %s\n", target.fleetID)
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

func commandTargetStateDirForNodeIdentity(baseStateDir string, nodeID string) string {
	normalizedNodeID := setupNodeIdentityName(nodeID)
	if normalizedNodeID == "" {
		return baseStateDir
	}
	if existingStateDir := commandTargetStateDirForAssignedNodeID(baseStateDir, normalizedNodeID); existingStateDir != "" {
		return existingStateDir
	}
	stateDir := filepath.Join(baseStateDir, "boards", normalizedNodeID)
	_ = os.MkdirAll(stateDir, 0o700)
	copySetupStateHints(baseStateDir, stateDir)
	copyFleetStateHints(baseStateDir, stateDir)
	if loadNodeID(stateDir) == "" {
		saveState(stateDir, "node_id", normalizedNodeID)
	}
	return stateDir
}

func commandTargetStateDirForAssignedNodeID(baseStateDir string, nodeID string) string {
	boardStateDirectories, errorValue := os.ReadDir(filepath.Join(baseStateDir, "boards"))
	if errorValue != nil {
		return ""
	}
	for _, boardStateDirectory := range boardStateDirectories {
		if !boardStateDirectory.IsDir() {
			continue
		}
		stateDir := filepath.Join(baseStateDir, "boards", boardStateDirectory.Name())
		if loadNodeID(stateDir) == nodeID {
			return stateDir
		}
	}
	return ""
}

func resolveCommandTargetStateDir(baseStateDir string, requestedNodeID string, hasExplicitHost bool) string {
	if strings.TrimSpace(requestedNodeID) != "" {
		return commandTargetStateDirForNodeIdentity(baseStateDir, requestedNodeID)
	}
	if hasExplicitHost {
		return baseStateDir
	}
	if defaultNodeID := loadState(baseStateDir, "default_node_id"); defaultNodeID != "" {
		return commandTargetStateDirForNodeIdentity(baseStateDir, defaultNodeID)
	}
	if activeNodeID := firstActiveFleetNodeID(baseStateDir); activeNodeID != "" {
		return commandTargetStateDirForNodeIdentity(baseStateDir, activeNodeID)
	}
	return baseStateDir
}

func commandTargetNodeID(arguments []string) string {
	nodeID := strings.TrimSpace(commandArgumentValue(arguments, "--node", ""))
	if nodeID == "" {
		return ""
	}
	if !isCommandTargetNodeNumber(nodeID) {
		fatal("--node must be a positive number such as 1, 2, or 3")
	}
	return nodeID
}

func setupNodeIdentityName(nodeID string) string {
	var builder strings.Builder
	for _, character := range strings.ToLower(strings.TrimSpace(nodeID)) {
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

func isCommandTargetNodeNumber(nodeID string) bool {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" || nodeID[0] == '0' {
		return false
	}
	for _, character := range nodeID {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
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

func applyFleetTargetArguments(stateDir string, baseStateDir string, arguments []string) {
	if fleetID := strings.TrimSpace(commandArgumentValue(arguments, "--fleet", "")); fleetID != "" {
		saveState(stateDir, "fleet_id", strings.ToLower(fleetID))
	}
	if fleetSecret := strings.TrimSpace(commandArgumentValue(arguments, "--fleet-secret", "")); fleetSecret != "" {
		saveState(stateDir, "fleet_secret", fleetSecret)
	}
	if loadState(stateDir, "fleet_id") == "" {
		if fleetID := loadState(baseStateDir, "fleet_id"); fleetID != "" {
			saveState(stateDir, "fleet_id", fleetID)
		}
	}
	if loadState(stateDir, "fleet_secret") == "" {
		if fleetSecret := loadState(baseStateDir, "fleet_secret"); fleetSecret != "" {
			saveState(stateDir, "fleet_secret", fleetSecret)
		}
	}
}

func copyFleetStateHints(sourceDir string, destinationDir string) {
	for _, key := range []string{
		"fleet_id",
		"fleet_secret",
		"device_url",
	} {
		if loadState(destinationDir, key) != "" {
			continue
		}
		if value := loadState(sourceDir, key); value != "" {
			saveState(destinationDir, key, value)
		}
	}
}
