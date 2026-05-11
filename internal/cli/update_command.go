package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type updateCommandRunner func(directoryPath string, name string, arguments ...string) error

var runUpdateCommand updateCommandRunner = runStreamingUpdateCommand
var resolveUpdateExecutablePath = currentExecutablePath

func runUpdateArguments(arguments []string) error {
	if hasCommandArgument(arguments, "--help") || hasCommandArgument(arguments, "-h") {
		printUpdateUsage()
		return nil
	}
	setupSlice, errorValue := updateSetupSlice(arguments)
	if errorValue != nil {
		return errorValue
	}
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	if !hasCommandArgument(arguments, "--plan") {
		if errorValue := runUpdateCommand(repositoryRootPath, "make", "build"); errorValue != nil {
			return errorValue
		}
	}
	executablePath, errorValue := resolveUpdateExecutablePath()
	if errorValue != nil {
		return errorValue
	}
	setupArguments := updateSetupArguments(setupSlice, arguments)
	return runUpdateCommand(repositoryRootPath, executablePath, setupArguments...)
}

func updateSetupSlice(arguments []string) (string, error) {
	selectedModes := 0
	for _, flag := range []string{"--all", "--web", "--binaries"} {
		if hasCommandArgument(arguments, flag) {
			selectedModes++
		}
	}
	if selectedModes > 1 {
		return "", errors.New("choose only one of --all, --web, or --binaries")
	}
	if hasCommandArgument(arguments, "--web") {
		return "admin-web", nil
	}
	if hasCommandArgument(arguments, "--binaries") {
		return "binaries,services", nil
	}
	return "admin-web,binaries,services", nil
}

func updateSetupArguments(setupSlice string, arguments []string) []string {
	setupArguments := []string{"setup", "--only", setupSlice, "--force"}
	for _, argument := range arguments {
		if updateSpecificArgument(argument) {
			continue
		}
		setupArguments = append(setupArguments, argument)
	}
	return setupArguments
}

func updateSpecificArgument(argument string) bool {
	switch argument {
	case "--all", "--web", "--binaries", "--help", "-h":
		return true
	default:
		return strings.HasPrefix(argument, "--all=") || strings.HasPrefix(argument, "--web=") || strings.HasPrefix(argument, "--binaries=")
	}
}

func runStreamingUpdateCommand(directoryPath string, name string, arguments ...string) error {
	command := exec.Command(name, arguments...)
	command.Dir = directoryPath
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Stdin = os.Stdin
	if errorValue := command.Run(); errorValue != nil {
		return fmt.Errorf("%s %s failed: %w", name, strings.Join(arguments, " "), errorValue)
	}
	return nil
}

func printUpdateUsage() {
	fmt.Println("Usage: internkim update [--all|--web|--binaries] [--plan] [target options]")
	fmt.Println("Examples:")
	fmt.Println("  internkim update")
	fmt.Println("  internkim update --web")
	fmt.Println("  internkim update --binaries --host 192.168.1.50")
	fmt.Println("  internkim update --plan")
}
