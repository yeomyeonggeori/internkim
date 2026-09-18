package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const companionBinaryName = "internkim-companion"

func runCompanion() {
	if len(os.Args) < 3 || os.Args[2] == "--help" || os.Args[2] == "-h" {
		printCompanionUsage()
		return
	}
	switch os.Args[2] {
	case "upgrade", "update", "install":
		if errorValue := runCompanionUpgrade(os.Args[3:]); errorValue != nil {
			fatal(errorValue.Error())
		}
	default:
		printCompanionUsage()
	}
}

func printCompanionUsage() {
	fmt.Println("Usage: internkim companion <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  upgrade   Build the companion binary from this checkout and install it")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  ./internkim companion upgrade")
	fmt.Println("  ./internkim companion upgrade --install-path /usr/local/bin/internkim-companion")
}

func runCompanionUpgrade(arguments []string) error {
	flags := flag.NewFlagSet("companion upgrade", flag.ContinueOnError)
	installPath := flags.String("install-path", defaultCompanionInstallPath(), "installed companion binary path")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(*installPath) == "" {
		return errors.New("install path is required")
	}
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(*installPath), 0o755); errorValue != nil {
		return errorValue
	}
	if errorValue := runCompanionUpgradeCommand(repositoryRootPath, "go", "build", "-o", *installPath, "./cmd/internkim-companion"); errorValue != nil {
		return errorValue
	}
	fmt.Println("Installed " + *installPath)
	return restartCompanionServiceIfInstalled(*installPath)
}

func defaultCompanionInstallPath() string {
	homeDirectory, errorValue := os.UserHomeDir()
	if errorValue != nil {
		return companionBinaryName
	}
	return filepath.Join(homeDirectory, ".local", "bin", companionBinaryName)
}

func restartCompanionServiceIfInstalled(installPath string) error {
	if !isCompanionServiceInstalled(installPath) {
		fmt.Println("Run `" + installPath + " service install` to keep it running in the background.")
		return nil
	}
	return runCompanionUpgradeCommand("", installPath, "service", "restart")
}

func isCompanionServiceInstalled(installPath string) bool {
	status, errorValue := exec.Command(installPath, "service", "status").Output()
	return errorValue == nil && strings.HasPrefix(strings.TrimSpace(string(status)), "running")
}

func runCompanionUpgradeCommand(directoryPath string, name string, arguments ...string) error {
	command := exec.Command(name, arguments...)
	if strings.TrimSpace(directoryPath) != "" {
		command.Dir = directoryPath
	}
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Stdin = os.Stdin
	return command.Run()
}
