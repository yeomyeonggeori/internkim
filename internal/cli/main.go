package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func currentExecutablePath() (string, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolvedPath, err := filepath.EvalSymlinks(executablePath); err == nil {
		executablePath = resolvedPath
	}
	return executablePath, nil
}

func resolveRepositoryRootPath() (string, error) {
	workingDirectoryPath, errorValue := os.Getwd()
	if errorValue != nil {
		return "", errorValue
	}

	searchPath := workingDirectoryPath
	for {
		if _, errorValue := os.Stat(filepath.Join(searchPath, "go.mod")); errorValue == nil {
			return searchPath, nil
		}
		parentPath := filepath.Dir(searchPath)
		if parentPath == searchPath {
			return "", errors.New("could not find repository root")
		}
		searchPath = parentPath
	}
}

func Main() {
	requestedProfile, arguments := splitVaultProfileArgument(os.Args[1:])
	os.Args = append(os.Args[:1], arguments...)
	reExecuteWithVaultEnvironment(requestedProfile)
	if len(os.Args) < 2 {
		printUsage()
		return
	}
	runNamedCommand(os.Args[1])
}

func runNamedCommand(name string) {
	switch name {
	case "release":
		runRelease()
	case "dev":
		runDev()
	case "lab":
		runLab()
	case "verify":
		runVerify()
	case "recover":
		runRecover()
	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: internkim <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  release  Build the company host packages and publish them")
	fmt.Println("  dev      Run blueclaw scenarios, or the company plane in a Linux guest")
	fmt.Println("  lab      Drive the Linux guest dev plane runs in")
	fmt.Println("  verify   Refuse a blueclaw pointer that moves backwards")
	fmt.Println("  recover  Ask the device for a signed recovery action (the Jetson cutover)")
}
