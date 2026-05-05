package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const companionApplicationName = "Intern Kim Companion.app"

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
	fmt.Println("  upgrade   Build and install the local companion app")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  ./internkim companion upgrade")
	fmt.Println("  ./internkim companion upgrade --skip-build")
}

func runCompanionUpgrade(arguments []string) error {
	flags := flag.NewFlagSet("companion upgrade", flag.ContinueOnError)
	installPath := flags.String("install-path", defaultCompanionInstallPath(), "installed companion app path")
	skipBuild := flags.Bool("skip-build", false, "install the last built companion app without rebuilding")
	if errorValue := flags.Parse(arguments); errorValue != nil {
		return errorValue
	}
	if runtime.GOOS != "darwin" {
		return errors.New("companion app upgrade is currently only supported on macOS")
	}
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	if !*skipBuild {
		if errorValue := runCompanionUpgradeCommand(repositoryRootPath, "make", "build-companion-shell"); errorValue != nil {
			return errorValue
		}
	}
	bundlePath := companionBuiltApplicationPath(repositoryRootPath)
	if errorValue := verifyCompanionApplicationBundle(bundlePath); errorValue != nil {
		return errorValue
	}
	if errorValue := installCompanionApplication(bundlePath, *installPath); errorValue != nil {
		return errorValue
	}
	fmt.Println("Updated " + *installPath)
	fmt.Println("Restart Intern Kim Companion to use the new version.")
	return nil
}

func defaultCompanionInstallPath() string {
	return filepath.Join("/Applications", companionApplicationName)
}

func companionBuiltApplicationPath(repositoryRootPath string) string {
	return filepath.Join(repositoryRootPath, "companion", "src-tauri", "target", "debug", "bundle", "macos", companionApplicationName)
}

func verifyCompanionApplicationBundle(path string) error {
	executablePath := filepath.Join(path, "Contents", "MacOS", "internkim-companion-shell")
	if fileInfo, errorValue := os.Stat(executablePath); errorValue != nil {
		return errors.New("companion app bundle is missing; run without --skip-build first")
	} else if fileInfo.IsDir() {
		return errors.New("companion app executable path is a directory")
	}
	return nil
}

func installCompanionApplication(sourcePath string, targetPath string) error {
	if strings.TrimSpace(targetPath) == "" {
		return errors.New("install path is required")
	}
	return runCompanionUpgradeCommand("", "ditto", sourcePath, targetPath)
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
