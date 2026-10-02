package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	internkimlab "github.com/yeomyeonggeori/internkim/internal/lab"
)

func runLab() {
	if errorValue := runLabArguments(os.Args[2:]); errorValue != nil {
		fatal(errorValue.Error())
	}
}

func runLabArguments(arguments []string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}

	configurationPath := internkimlab.DefaultConfigurationPath(repositoryRootPath)
	timeout := 45 * time.Minute
	flagSet := flag.NewFlagSet("lab", flag.ContinueOnError)
	flagSet.StringVar(&configurationPath, "config", configurationPath, "Path to lab configuration JSON")
	flagSet.DurationVar(&timeout, "timeout", timeout, "Timeout for lab operations")

	subcommand := "status"
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

	return runLabSubcommand(ctx, service, subcommand, flagSet.Args())
}

func runLabSubcommand(ctx context.Context, service internkimlab.Service, subcommand string, remainingArguments []string) error {
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
	case "status":
		status, errorValue := service.VirtualMachineStatus(ctx)
		if errorValue != nil {
			return errorValue
		}
		fmt.Println(status)
		return nil
	default:
		return fmt.Errorf("unknown lab subcommand: %s", subcommand)
	}
}
