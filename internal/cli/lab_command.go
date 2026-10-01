package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	internkimlab "github.com/yeomyeonggeori/internkim/internal/lab"
)

type hostDependency struct {
	name        string
	purpose     string
	installHint string
}

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
	if len(arguments) == 0 || strings.HasPrefix(arguments[0], "-") {
		return errors.New("usage: internkim lab <vm-up|vm-ip|vm-ssh|vm-down|status> [--config path] [--timeout duration]")
	}
	subcommand := arguments[0]
	configurationPath := internkimlab.DefaultConfigurationPath(repositoryRootPath)
	timeout := 45 * time.Minute
	flagSet := flag.NewFlagSet("lab", flag.ContinueOnError)
	flagSet.StringVar(&configurationPath, "config", configurationPath, "Path to lab configuration JSON")
	flagSet.DurationVar(&timeout, "timeout", timeout, "Timeout for lab operations")
	if errorValue := flagSet.Parse(arguments[1:]); errorValue != nil {
		return errorValue
	}
	configuration, errorValue := internkimlab.LoadConfiguration(configurationPath)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := ensureLabDependencies(configuration); errorValue != nil {
		return errorValue
	}
	service := internkimlab.NewService(configuration, internkimlab.OperatingSystemCommandRunner{}, repositoryRootPath)
	contextValue, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return runLabSubcommand(contextValue, service, subcommand, flagSet.Args())
}

func runLabSubcommand(contextValue context.Context, service internkimlab.Service, subcommand string, remainingArguments []string) error {
	switch subcommand {
	case "vm-up":
		return service.VirtualMachineUp(contextValue)
	case "vm-ip":
		return printLabAnswer(service.VirtualMachineIPAddress(contextValue))
	case "vm-down":
		return service.VirtualMachineDown(contextValue)
	case "vm-ssh":
		return service.VirtualMachineSSH(contextValue, remainingArguments)
	case "status":
		return printLabAnswer(service.VirtualMachineStatus(contextValue))
	default:
		return fmt.Errorf("unknown lab subcommand: %s", subcommand)
	}
}

func printLabAnswer(answer string, errorValue error) error {
	if errorValue != nil {
		return errorValue
	}
	fmt.Println(answer)
	return nil
}

func labDependencies(configuration internkimlab.Configuration) []hostDependency {
	return []hostDependency{
		{
			name:        configuration.VirtualMachine.Container.BinaryPath,
			purpose:     "the Linux guest dev plane runs in",
			installHint: "install the container CLI from https://github.com/apple/container/releases",
		},
		{
			name:        "sshpass",
			purpose:     "password SSH into that guest",
			installHint: "brew install sshpass, or apt install sshpass",
		},
	}
}

func ensureLabDependencies(configuration internkimlab.Configuration) error {
	var message strings.Builder
	for _, dependency := range labDependencies(configuration) {
		if _, errorValue := exec.LookPath(dependency.name); errorValue != nil {
			message.WriteString(fmt.Sprintf("  - %s (%s)\n    install: %s\n", dependency.name, dependency.purpose, dependency.installHint))
		}
	}
	if message.Len() == 0 {
		return nil
	}
	return errors.New("missing host dependencies:\n" + message.String())
}
