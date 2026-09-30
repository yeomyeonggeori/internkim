package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	internkimlab "gitlab.com/eastriver/internkim/internal/lab"
)

func resolveLabVirtualMachineIPAddress() string {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return ""
	}

	configurationPath := internkimlab.DefaultConfigurationPath(repositoryRootPath)
	configuration, errorValue := internkimlab.LoadConfiguration(configurationPath)
	if errorValue != nil {
		return ""
	}

	service := internkimlab.NewService(
		configuration,
		internkimlab.OperatingSystemCommandRunner{},
		repositoryRootPath,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	virtualMachineIPAddress, errorValue := service.VirtualMachineIPAddress(ctx)
	if errorValue != nil {
		return ""
	}

	return strings.TrimSpace(virtualMachineIPAddress)
}

func runLab() {
	if errorValue := runLabArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runSetupSimulation(setupArguments []string) {
	if errorValue := runSetupSimulationArguments(setupArguments); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runSetupSimulationArguments(setupArguments []string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}

	configurationPath := internkimlab.DefaultConfigurationPath(repositoryRootPath)
	configuration, errorValue := internkimlab.LoadConfiguration(configurationPath)
	if errorValue != nil {
		return errorValue
	}

	if errorValue := ensureSimulationDependencies(configuration); errorValue != nil {
		return errorValue
	}

	service := internkimlab.NewService(
		configuration,
		internkimlab.OperatingSystemCommandRunner{},
		repositoryRootPath,
	)

	executablePath, errorValue := currentExecutablePath()
	if errorValue != nil {
		return errorValue
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	if containsSetupPlan(setupArguments) {
		if errorValue := service.PrintSimulationPlan(ctx, executablePath, setupArguments); errorValue != nil {
			return errorValue
		}
		return nil
	}

	filteredSetupArguments, shouldVerify, shouldVerifyBrowser := splitSimulationVerifyArguments(setupArguments)
	setupArguments = filteredSetupArguments

	if errorValue := validateSimulationStateIsolation(); errorValue != nil {
		return errorValue
	}
	markSimulationState(repositoryRootPath)
	if errorValue := service.SetupSimulation(ctx, executablePath, setupArguments); errorValue != nil {
		return errorValue
	}

	if !shouldVerify && !shouldVerifyBrowser {
		return nil
	}
	return verifySimulationTarget(ctx, service, configuration, shouldVerifyBrowser)
}

func verifySimulationTarget(ctx context.Context, service internkimlab.Service, configuration internkimlab.Configuration, shouldVerifyBrowser bool) error {
	virtualMachineIPAddress, errorValue := service.VirtualMachineIPAddress(ctx)
	if errorValue != nil {
		return errorValue
	}
	verifyArguments := []string{"api", "--board", commandTargetBoardSimulation, "--host", virtualMachineIPAddress, "--user", configuration.VirtualMachine.SSHUsername, "--password", configuration.VirtualMachine.SSHPassword}
	if errorValue := runVerifyArguments(verifyArguments); errorValue != nil {
		return errorValue
	}
	if shouldVerifyBrowser {
		if errorValue := runVerifyArguments([]string{"browser", "--local", "--board", commandTargetBoardSimulation, "--host", virtualMachineIPAddress, "--user", configuration.VirtualMachine.SSHUsername, "--password", configuration.VirtualMachine.SSHPassword}); errorValue != nil {
			return errorValue
		}
		if errorValue := runVerifyArguments([]string{"browser", "--public", "--board", commandTargetBoardSimulation, "--host", virtualMachineIPAddress, "--user", configuration.VirtualMachine.SSHUsername, "--password", configuration.VirtualMachine.SSHPassword}); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func splitSimulationVerifyArguments(arguments []string) ([]string, bool, bool) {
	var filteredArguments []string
	shouldVerify := false
	shouldVerifyBrowser := false
	for _, argument := range arguments {
		switch argument {
		case "--verify":
			shouldVerify = true
		case "--verify-browser":
			shouldVerify = true
			shouldVerifyBrowser = true
		default:
			filteredArguments = append(filteredArguments, argument)
		}
	}
	return filteredArguments, shouldVerify, shouldVerifyBrowser
}

func runLabArguments(arguments []string) error {
	return runLabArgumentsForTarget(arguments, commandTargetBoardLab)
}

func runLabArgumentsForTarget(arguments []string, boardType string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}

	configurationPath := internkimlab.DefaultConfigurationPath(repositoryRootPath)
	timeout := 45 * time.Minute
	flagSet := flag.NewFlagSet("lab", flag.ContinueOnError)
	flagSet.StringVar(&configurationPath, "config", configurationPath, "Path to lab configuration JSON")
	flagSet.DurationVar(&timeout, "timeout", timeout, "Timeout for lab operations")

	subcommand := "scenario-e2e"
	flagArguments := arguments
	if len(arguments) > 0 && !strings.HasPrefix(arguments[0], "-") {
		subcommand = arguments[0]
		flagArguments = arguments[1:]
	}

	if errorValue := flagSet.Parse(flagArguments); errorValue != nil {
		return errorValue
	}

	configuration, errorValue := internkimlab.LoadConfiguration(configurationPath)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := ensureSimulationDependencies(configuration); errorValue != nil {
		return errorValue
	}

	service := internkimlab.NewService(
		configuration,
		internkimlab.OperatingSystemCommandRunner{},
		repositoryRootPath,
	)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return runLabSubcommand(ctx, service, subcommand, boardType, flagSet.Args())
}

func runLabSubcommand(ctx context.Context, service internkimlab.Service, subcommand string, boardType string, remainingArguments []string) error {
	switch subcommand {
	case "image-build":
		return service.ImageBuild(ctx)
	case "vm-up":
		return service.VirtualMachineUp(ctx)
	case "vm-ip":
		virtualMachineIPAddress, errorValue := service.VirtualMachineIPAddress(ctx)
		if errorValue != nil {
			return errorValue
		}
		fmt.Println(virtualMachineIPAddress)
		return nil
	case "vm-down":
		return service.VirtualMachineDown(ctx)
	case "vm-ssh":
		return service.VirtualMachineSSH(ctx, remainingArguments)
	case "runtime-builder-prepare":
		return service.RuntimeBuilderPrepare(ctx)
	case "runtime-builder-check":
		return service.RuntimeBuilderCheck(ctx)
	case "runtime-builder-shell":
		return service.VirtualMachineSSH(ctx, []string{"cd /mnt/shared && exec ${SHELL:-/bin/bash} -l"})
	case "status":
		status, errorValue := service.VirtualMachineStatus(ctx)
		if errorValue != nil {
			return errorValue
		}
		fmt.Println(status)
		return nil
	case "setup":
		executablePath, errorValue := currentExecutablePath()
		if errorValue != nil {
			return errorValue
		}
		if boardType == commandTargetBoardSimulation {
			return service.SetupSimulation(ctx, executablePath, nil)
		}
		return service.Setup(ctx, executablePath, nil)
	case "scenario-cloudflare":
		return service.ScenarioCloudflare(ctx)
	case "scenario-e2e":
		executablePath, errorValue := currentExecutablePath()
		if errorValue != nil {
			return errorValue
		}
		if boardType == commandTargetBoardSimulation {
			return service.ScenarioSimulationEndToEnd(ctx, executablePath, nil)
		}
		return service.ScenarioEndToEnd(ctx, executablePath, nil)
	default:
		return fmt.Errorf("unknown lab subcommand: %s", subcommand)
	}
}

func getLocalSSHPubKey() string {
	// Try common public key locations
	home, _ := os.UserHomeDir()
	for _, name := range []string{"id_ed25519.pub", "id_rsa.pub", "id_ecdsa.pub"} {
		data, err := os.ReadFile(filepath.Join(home, ".ssh", name))
		if err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}
