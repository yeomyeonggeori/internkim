package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	internkimlab "gitlab.com/eastriver/internkim/internal/lab"
	setup "gitlab.com/eastriver/internkim/internal/provisioning/steps"
)

var runSimCommand updateCommandRunner = runStreamingUpdateCommand
var runSimSetup = runSetupSimulationArguments
var runSimLabTarget = runLabArgumentsForTarget
var resolveSimulationVirtualMachineIPAddress = resolveLabVirtualMachineIPAddress

func runSimArguments(arguments []string) error {
	command := "gate"
	commandArguments := arguments
	if len(arguments) > 0 && !strings.HasPrefix(arguments[0], "-") {
		command = arguments[0]
		commandArguments = arguments[1:]
	}
	switch command {
	case "gate":
		return runSimGateArguments(commandArguments)
	case "cleanup":
		return runSimCleanupArguments(commandArguments)
	case "reset":
		if errorValue := runSimCleanupArguments(commandArguments); errorValue != nil {
			return errorValue
		}
		return runSimGateArguments(nil)
	case "stop":
		return runSimLabTarget([]string{"vm-down"}, commandTargetBoardSimulation)
	case "ssh":
		return runSimLabTarget(append([]string{"vm-ssh"}, commandArguments...), commandTargetBoardSimulation)
	case "status":
		return runSimLabTarget([]string{"status"}, commandTargetBoardSimulation)
	case "help":
		printSimUsage()
		return nil
	default:
		return fmt.Errorf("unknown sim command %q", command)
	}
}

func runSimGateArguments(arguments []string) error {
	if hasCommandArgument(arguments, "--help") || hasCommandArgument(arguments, "-h") {
		printSimUsage()
		return nil
	}
	if errorValue := validateSimulationStateIsolation(); errorValue != nil {
		return errorValue
	}
	setupArguments := simGateSetupArguments(arguments)
	if hasCommandArgument(arguments, "--plan") {
		return runSimSetup(setupArguments)
	}
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	if errorValue := runSimCommand(repositoryRootPath, "make", "build"); errorValue != nil {
		return errorValue
	}
	return runSimSetup(setupArguments)
}

func runSimCleanupArguments(arguments []string) error {
	if hasCommandArgument(arguments, "--help") || hasCommandArgument(arguments, "-h") {
		printSimUsage()
		return nil
	}
	if !hasCommandArgument(arguments, "--local-only") {
		if errorValue := validateSimulationStateIsolation(); errorValue != nil {
			return errorValue
		}
		if errorValue := cleanupSimulationRemoteRegistration(); errorValue != nil {
			return errorValue
		}
	}
	if errorValue := runSimLabTarget([]string{"vm-down"}, commandTargetBoardSimulation); errorValue != nil {
		return errorValue
	}
	if hasCommandArgument(arguments, "--keep-state") {
		return nil
	}
	return removeSimulationLocalState()
}

func simGateSetupArguments(arguments []string) []string {
	setupArguments := filterSimGateSetupArguments(arguments)
	if !hasCommandArgument(setupArguments, "--force") && !hasCommandArgument(setupArguments, "--force-all") {
		setupArguments = append(setupArguments, "--force")
	}
	if !hasCommandArgument(setupArguments, "--verify") && !hasCommandArgument(setupArguments, "--verify-browser") {
		setupArguments = append(setupArguments, "--verify-browser")
	}
	return setupArguments
}

func filterSimGateSetupArguments(arguments []string) []string {
	filteredArguments := []string{}
	for _, argument := range arguments {
		switch argument {
		case "--help", "-h", "--keep-state":
			continue
		default:
			filteredArguments = append(filteredArguments, argument)
		}
	}
	return filteredArguments
}

func validateSimulationStateIsolation() error {
	simulationStateDir := simulationStateDirectoryPath()
	physicalStateDir := physicalStateDirectoryPath()
	for _, key := range simulationIsolationStateKeys() {
		simulationValue := strings.TrimSpace(loadState(simulationStateDir, key))
		physicalValue := strings.TrimSpace(loadState(physicalStateDir, key))
		if simulationValue != "" && physicalValue != "" && simulationValue == physicalValue {
			return fmt.Errorf("simulation state shares %s with the physical device; run `internkim sim cleanup --keep-state` and inspect %s", key, simulationStateDir)
		}
	}
	return nil
}

func markSimulationState(repositoryRootPath string) {
	stateDir := simulationStateDirectoryPath()
	saveState(stateDir, "target_kind", "simulation")
	saveState(stateDir, "simulation_repository", repositoryRootPath)
	saveState(stateDir, "simulation_updated_at", time.Now().UTC().Format(time.RFC3339))
}

func removeSimulationLocalState() error {
	stateDir := simulationStateDirectoryPath()
	if strings.TrimSpace(stateDir) == "" || stateDir == physicalStateDirectoryPath() {
		return errors.New("refusing to remove an invalid simulation state directory")
	}
	return os.RemoveAll(stateDir)
}

func cleanupSimulationRemoteRegistration() error {
	stateDir := simulationStateDirectoryPath()
	fleetID := strings.TrimSpace(loadState(stateDir, "fleet_id"))
	fleetSecret := strings.TrimSpace(loadState(stateDir, "fleet_secret"))
	if fleetID == "" || fleetSecret == "" {
		return nil
	}
	configuration := loadConfig()
	if strings.TrimSpace(configuration.RegisterSecret) == "" {
		return errors.New("INTERNKIM_REGISTER_SECRET is required to clean up registered simulation resources; pass --local-only to remove local sim state only")
	}
	body, errorValue := json.Marshal(map[string]string{
		"fleet_id":     fleetID,
		"fleet_secret": fleetSecret,
	})
	if errorValue != nil {
		return errorValue
	}
	requestURL := strings.TrimRight(configuration.APIBaseURL, "/") + "/api/register"
	request, errorValue := http.NewRequest(http.MethodDelete, requestURL, bytes.NewReader(body))
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Authorization", "Bearer "+configuration.RegisterSecret)
	request.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 30 * time.Second}
	response, errorValue := client.Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("simulation registration cleanup returned HTTP %d", response.StatusCode)
}

func stopSimulationBeforePhysicalDeploy() error {
	return runSimLabTarget([]string{"vm-down"}, commandTargetBoardSimulation)
}

func simulationStateDirectoryPath() string {
	return commandTargetStateDir(internkimHomeDir(), setup.BoardSimulation)
}

func physicalStateDirectoryPath() string {
	return commandTargetStateDir(internkimHomeDir(), setup.BoardJetsonOrinNano)
}

func simulationIsolationStateKeys() []string {
	return []string{"fleet_id", "device_url", "tunnel_token", "node_tunnel_token", "ssh_hostname"}
}

func printSimUsage() {
	fmt.Println("Usage: internkim sim [gate|status|ssh|stop|cleanup] [options]")
	fmt.Println("Examples:")
	fmt.Println("  internkim sim gate")
	fmt.Println("  internkim sim gate --plan")
	fmt.Println("  internkim sim cleanup")
	fmt.Println("  internkim sim cleanup --local-only")
}

func simulationHostTarget(repositoryRootPath string, target commandTarget) commandTarget {
	if target.mode != commandTargetModeSimulation || strings.TrimSpace(target.host) != "" {
		return target
	}
	host := resolveSimulationVirtualMachineIPAddress()
	if host == "" {
		return target
	}
	target.host = host
	configurationPath := filepath.Join(repositoryRootPath, "lab", "config.example.json")
	if configuration, errorValue := internkimlab.LoadConfiguration(configurationPath); errorValue == nil {
		target.sshUser = configuration.VirtualMachine.SSHUsername
		target.sshPassword = configuration.VirtualMachine.SSHPassword
	}
	return target
}
